package monitor

import (
	"bufio"
	"bytes"
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

	// 完整读取响应体（避免连接泄漏），在需要内容匹配时保留文本
	var bodyBytes []byte
	if cfg.SuccessContains != "" {
		data, readErr := io.ReadAll(resp.Body)
		switch {
		case readErr == nil:
			bodyBytes = data
		case errors.Is(readErr, io.ErrUnexpectedEOF) || errors.Is(readErr, io.EOF):
			// EOF/ErrUnexpectedEOF 通常表示传输提前结束（如 Content-Length 不匹配），但已读内容仍可用于匹配
			bodyBytes = data
			logger.Debug("probe", "读取响应体遇到 EOF，使用已读数据",
				"provider", cfg.Provider, "service", cfg.Service, "channel", cfg.Channel, "error", readErr, "bytes", len(data))
		default:
			logger.Warn("probe", "读取响应体失败",
				"provider", cfg.Provider, "service", cfg.Service, "channel", cfg.Channel, "error", readErr)
		}
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

// determineStatus 根据HTTP状态码和延迟判定监控状态
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
