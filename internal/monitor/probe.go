package monitor

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"monitor/internal/config"
	"monitor/internal/logger"
	"monitor/internal/storage"
)

// ProbeResult 探测结果
type ProbeResult struct {
	Provider  string
	Service   string
	Channel   string
	Status    int               // 1=绿, 0=红, 2=黄
	SubStatus storage.SubStatus // 细分状态（黄色/红色原因）
	HttpCode  int               // HTTP 状态码（0 表示非 HTTP 错误）
	Latency   int               // ms
	Timestamp int64
	Error     error
}

// Prober 探测器
type Prober struct {
	clientPool *ClientPool
	storage    storage.Storage
}

// NewProber 创建探测器
func NewProber(storage storage.Storage) *Prober {
	return &Prober{
		clientPool: NewClientPool(),
		storage:    storage,
	}
}

// Probe 执行单次探测
func (p *Prober) Probe(ctx context.Context, cfg *config.ServiceConfig) *ProbeResult {
	result := &ProbeResult{
		Provider:  cfg.Provider,
		Service:   cfg.Service,
		Channel:   cfg.Channel,
		Timestamp: time.Now().Unix(),
		SubStatus: storage.SubStatusNone,
		HttpCode:  0, // 默认为 0，表示非 HTTP 错误
	}

	// 准备请求体（去除首尾空白，某些 API 对此敏感）
	reqBody := bytes.NewBuffer([]byte(strings.TrimSpace(cfg.Body)))
	req, err := http.NewRequestWithContext(ctx, cfg.Method, cfg.URL, reqBody)
	if err != nil {
		result.Error = fmt.Errorf("创建请求失败: %w", err)
		result.Status = 0
		result.SubStatus = storage.SubStatusNetworkError
		return result
	}

	// 设置Headers（已处理过占位符）
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	// 获取对应provider的客户端
	client := p.clientPool.GetClient(cfg.Provider)

	// 发送请求并计时
	start := time.Now()
	resp, err := client.Do(req)
	latency := int(time.Since(start).Milliseconds())
	result.Latency = latency

	if err != nil {
		logger.Error("probe", "请求失败",
			"provider", cfg.Provider, "service", cfg.Service, "channel", cfg.Channel, "error", err)
		result.Error = err
		result.Status = 0
		result.SubStatus = storage.SubStatusNetworkError
		return result
	}
	defer resp.Body.Close()

	// 记录 HTTP 状态码
	result.HttpCode = resp.StatusCode

	// 完整读取响应体（避免连接泄漏），在需要内容匹配时保留文本
	var bodyBytes []byte
	if cfg.SuccessContains != "" {
		data, readErr := io.ReadAll(resp.Body)
		switch {
		case readErr == nil:
			bodyBytes = data
		case isTolerableReadError(readErr):
			// 可容忍的传输错误（EOF、HTTP/2 流错误等），已读内容仍可用于匹配
			bodyBytes = data
			logger.Debug("probe", "读取响应体遇到可容忍错误，使用已读数据",
				"provider", cfg.Provider, "service", cfg.Service, "channel", cfg.Channel, "error", readErr, "bytes", len(data))
		default:
			logger.Warn("probe", "读取响应体失败",
				"provider", cfg.Provider, "service", cfg.Service, "channel", cfg.Channel, "error", readErr)
		}

		// gzip 解压：当 Content-Encoding 包含 gzip 时，手动解压响应体
		// Go 的 http.Transport 在用户显式设置 Accept-Encoding 请求头时不会自动解压
		bodyBytes = decompressGzipIfNeeded(resp, bodyBytes, cfg.Provider, cfg.Service, cfg.Channel)
	} else {
		_, _ = io.Copy(io.Discard, resp.Body)
	}

	// 判定状态（先按 HTTP/延迟，再根据响应内容做二次判断）
	status, subStatus := p.determineStatus(resp.StatusCode, latency, cfg.SlowLatencyDuration)
	result.Status = status
	result.SubStatus = subStatus
	result.Status, result.SubStatus = evaluateStatus(result.Status, result.SubStatus, bodyBytes, cfg.SuccessContains)

	// 当探测结果不可用时，输出一小段响应内容片段，便于排查（避免输出过长/敏感完整内容）
	if result.Status == 0 && len(bodyBytes) > 0 {
		snippet := strings.TrimSpace(aggregateResponseText(bodyBytes))
		if snippet != "" {
			const maxSnippetLen = 512 // 防止日志过长
			if len(snippet) > maxSnippetLen {
				snippet = snippet[:maxSnippetLen] + "... (truncated)"
			}
			logger.Warn("probe", "响应片段",
				"provider", cfg.Provider, "service", cfg.Service, "channel", cfg.Channel, "snippet", snippet)
		}
	}

	// 日志（不打印敏感信息）
	logger.Info("probe", "探测完成",
		"provider", cfg.Provider, "service", cfg.Service, "channel", cfg.Channel,
		"code", resp.StatusCode, "latency_ms", latency, "status", result.Status, "sub_status", result.SubStatus)

	return result
}

// evaluateStatus 在基础状态上叠加响应内容匹配规则
// 对所有 2xx 响应（绿色和慢速黄色）进行内容校验
// 红色（已失败）和 429 黄色（非正常响应）不做校验。
// 为兼容流式响应，这里会尝试从 SSE/增量数据中聚合出最终文本再做匹配。
func evaluateStatus(baseStatus int, baseSubStatus storage.SubStatus, body []byte, successContains string) (int, storage.SubStatus) {
	if successContains == "" {
		return baseStatus, baseSubStatus
	}

	// 红色已是最差状态，不需要校验
	if baseStatus == 0 {
		return baseStatus, baseSubStatus
	}

	// 429 限流：响应体是错误信息，不做内容校验
	if baseStatus == 2 && baseSubStatus == storage.SubStatusRateLimit {
		return baseStatus, baseSubStatus
	}

	// 对 2xx 响应（绿色或慢速黄色）做内容校验
	text := aggregateResponseText(body)
	if strings.TrimSpace(text) == "" {
		// 没有响应内容，降级为红
		return 0, storage.SubStatusContentMismatch
	}

	if !strings.Contains(text, successContains) {
		// 未包含预期内容，认为请求语义失败
		return 0, storage.SubStatusContentMismatch
	}

	return baseStatus, baseSubStatus
}

// decompressGzipIfNeeded 检测并解压 gzip 压缩的响应体
// 当 Content-Encoding 包含 gzip 时进行解压，失败则保留原始数据
// 额外检测 gzip 魔术头（0x1f 0x8b）作为兜底，处理服务器漏写 Content-Encoding 的情况
func decompressGzipIfNeeded(resp *http.Response, data []byte, provider, service, channel string) []byte {
	if len(data) == 0 {
		return data
	}

	// 检查是否需要解压：Content-Encoding 声明 gzip 或数据以 gzip 魔术头开始
	contentEncoding := strings.ToLower(resp.Header.Get("Content-Encoding"))
	isGzipHeader := strings.Contains(contentEncoding, "gzip")
	isGzipMagic := len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b

	if !isGzipHeader && !isGzipMagic {
		return data
	}

	// 创建 gzip reader
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		logger.Warn("probe", "gzip 解压初始化失败，使用原始响应体",
			"provider", provider, "service", service, "channel", channel, "error", err)
		return data
	}
	defer gr.Close()

	// 读取解压后的数据
	decompressed, err := io.ReadAll(gr)
	if err != nil {
		logger.Warn("probe", "gzip 解压读取失败，使用原始响应体",
			"provider", provider, "service", service, "channel", channel, "error", err)
		return data
	}

	logger.Debug("probe", "gzip 解压成功",
		"provider", provider, "service", service, "channel", channel,
		"compressed_size", len(data), "decompressed_size", len(decompressed))

	return decompressed
}

// determineStatus 根据HTTP状态码和延迟判定监测状态
func (p *Prober) determineStatus(statusCode, latency int, slowLatency time.Duration) (int, storage.SubStatus) {
	// 2xx = 绿色
	if statusCode >= 200 && statusCode < 300 {
		// 如果延迟超过 slowLatency，降级为黄色
		if slowLatency > 0 && latency > int(slowLatency/time.Millisecond) {
			return 2, storage.SubStatusSlowLatency
		}
		return 1, storage.SubStatusNone
	}

	// 3xx = 绿色（重定向，通常由客户端自动处理，视为正常）
	if statusCode >= 300 && statusCode < 400 {
		return 1, storage.SubStatusNone
	}

	// 401/403 = 红色（认证/权限失败）
	if statusCode == 401 || statusCode == 403 {
		return 0, storage.SubStatusAuthError
	}

	// 400 = 红色（请求参数错误）
	if statusCode == 400 {
		return 0, storage.SubStatusInvalidRequest
	}

	// 429 = 红色（速率限制，视为不可用）
	if statusCode == 429 {
		return 0, storage.SubStatusRateLimit
	}

	// 5xx = 红色（服务器错误，视为不可用）
	if statusCode >= 500 {
		return 0, storage.SubStatusServerError
	}

	// 其他4xx = 红色（客户端错误）
	if statusCode >= 400 {
		return 0, storage.SubStatusClientError
	}

	// 1xx 和其他非标准状态码 = 红色（客户端错误，因为 LLM API 不应返回这些）
	if statusCode >= 100 && statusCode < 200 {
		return 0, storage.SubStatusClientError
	}

	// 完全异常的状态码（< 100 或无法识别）
	return 0, storage.SubStatusClientError
}

// aggregateResponseText 将原始响应体整理为用于关键字匹配的文本。
// - 普通 JSON/文本：直接使用完整 body
// - SSE / 流式响应：尝试解析 data: 行中的增量内容并拼接
func aggregateResponseText(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	// 简单启发式：同时包含 "event:" 和 "data:" 时按 SSE 解析
	if bytes.Contains(body, []byte("event:")) && bytes.Contains(body, []byte("data:")) {
		if sseText := extractTextFromSSE(body); sseText != "" {
			return sseText
		}
	}

	// 回退到原始响应体
	return string(body)
}

// extractTextFromSSE 从 text/event-stream 风格的响应体中抽取语义文本。
// 当前支持的模式：
// - Anthropic: event: content_block_delta + data: {"delta":{"type":"text_delta","text":"..."}}
// - OpenAI:   data: {"choices":[{"delta":{"content":"..."}}]}
// 解析失败时会尽量回退到原始 data 文本。
func extractTextFromSSE(body []byte) string {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	// 提升单行上限，避免极端情况下行太长
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	var b strings.Builder

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}

		var obj map[string]any
		if err := json.Unmarshal([]byte(payload), &obj); err != nil {
			// 非 JSON data，直接拼接原始内容
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(payload)
			continue
		}

		appendText := func(s string) {
			if s == "" {
				return
			}
			b.WriteString(s)
		}

		// Anthropic: {"type":"content_block_delta", "delta":{"type":"text_delta","text":"..."}}
		if delta, ok := obj["delta"].(map[string]any); ok {
			if text, ok := delta["text"].(string); ok {
				appendText(text)
			}
		}

		// OpenAI: {"choices":[{"delta":{"content":"..."}}]}
		if choices, ok := obj["choices"].([]any); ok {
			for _, ch := range choices {
				chMap, ok := ch.(map[string]any)
				if !ok {
					continue
				}
				delta, ok := chMap["delta"].(map[string]any)
				if !ok {
					continue
				}
				if text, ok := delta["content"].(string); ok {
					appendText(text)
				}
			}
		}

		// 通用兜底：顶层 content / message 字段
		if content, ok := obj["content"].(string); ok {
			appendText(content)
		}
		if msg, ok := obj["message"].(string); ok {
			appendText(msg)
		}
	}

	if err := scanner.Err(); err != nil {
		// 扫描出错时，尽量返回已有内容；彻底失败则交由上层回退
	}

	return b.String()
}

// SaveResult 保存探测结果到存储
func (p *Prober) SaveResult(result *ProbeResult) error {
	record := &storage.ProbeRecord{
		Provider:  result.Provider,
		Service:   result.Service,
		Channel:   result.Channel,
		Status:    result.Status,
		SubStatus: result.SubStatus,
		HttpCode:  result.HttpCode,
		Latency:   result.Latency,
		Timestamp: result.Timestamp,
	}

	return p.storage.SaveRecord(record)
}

// Close 关闭探测器
func (p *Prober) Close() {
	p.clientPool.Close()
}

// MaskSensitiveInfo 脱敏敏感信息（用于日志）
func MaskSensitiveInfo(s string) string {
	if len(s) <= 8 {
		return "***"
	}
	// 只显示前4位和后4位
	return s[:4] + "***" + s[len(s)-4:]
}

// isTolerableReadError 判断是否为可容忍的响应体读取错误
// 部分中转服务器实现不严谨，可能在响应内容已完整返回后仍触发以下错误：
// - io.EOF / io.ErrUnexpectedEOF: Content-Length 不匹配或 chunked encoding 未正确终止
// - HTTP/2 stream error: 服务端发送 RST_STREAM 帧提前关闭流
// - HTTP/2 连接关闭: GOAWAY 帧或连接被关闭
// 这些情况下已读取的数据通常仍可用于内容匹配
func isTolerableReadError(err error) bool {
	if err == nil {
		return false
	}

	// 标准 EOF 错误
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	// HTTP/2 相关错误（通过错误消息匹配，因为 net/http2 未导出具体错误类型）
	errStr := err.Error()

	// HTTP/2 流错误：必须包含 "stream error:" 前缀，且带有常见错误码
	if strings.Contains(errStr, "stream error:") {
		// 常见的 HTTP/2 流错误码，这些通常表示服务端提前关闭而非真正的传输失败
		tolerableCodes := []string{
			"INTERNAL_ERROR",
			"CANCEL",
			"NO_ERROR",
			"PROTOCOL_ERROR",
			"REFUSED_STREAM",
		}
		for _, code := range tolerableCodes {
			if strings.Contains(errStr, code) {
				return true
			}
		}
	}

	// HTTP/2 连接级错误
	if strings.Contains(errStr, "GOAWAY") ||
		strings.Contains(errStr, "http2: response body closed") ||
		strings.Contains(errStr, "http2: server sent GOAWAY") {
		return true
	}

	return false
}
