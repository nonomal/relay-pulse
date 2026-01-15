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
	"math/rand"
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
	Model     string            // 模型标识
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

// Probe 执行单次探测（支持可配置重试）
func (p *Prober) Probe(ctx context.Context, cfg *config.ServiceConfig) *ProbeResult {
	result := &ProbeResult{
		Provider:  cfg.Provider,
		Service:   cfg.Service,
		Channel:   cfg.Channel,
		Model:     cfg.Model,
		Timestamp: time.Now().Unix(),
		SubStatus: storage.SubStatusNone,
		HttpCode:  0, // 默认为 0，表示非 HTTP 错误
	}

	// 使用配置的超时时间包装 context
	// 兜底：防止 TimeoutDuration 未下发导致请求无期限挂起
	timeout := cfg.TimeoutDuration
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	// 获取对应 provider 的客户端（考虑代理配置）
	client, err := p.clientPool.GetClient(cfg.Provider, cfg.Proxy)
	if err != nil {
		result.Error = fmt.Errorf("获取 HTTP 客户端失败: %w", err)
		result.Status = 0
		result.SubStatus = storage.SubStatusNetworkError
		return result
	}

	// 重试配置：从 config 获取（已在 Normalize 阶段下发到 monitor 级别）
	maxAttempts := cfg.RetryCount + 1 // RetryCount 是额外重试次数，总尝试次数 = 1 + RetryCount
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	baseDelay := cfg.RetryBaseDelayDuration
	if baseDelay <= 0 {
		baseDelay = 200 * time.Millisecond
	}
	maxDelay := cfg.RetryMaxDelayDuration
	if maxDelay <= 0 {
		maxDelay = 2 * time.Second
	}
	jitter := cfg.RetryJitterValue
	if jitter < 0 {
		jitter = 0
	}
	if jitter > 1 {
		jitter = 1
	}

	// 累计延迟（所有 attempt 的延迟总和）
	var totalLatency int
	// 实际执行的 attempt 次数（用于最终日志）
	var actualAttempts int
	// 保存最后一次的响应体（用于最终诊断日志）
	var lastBodyBytes []byte

	// 重试循环（使用标签以便从 select 中正确跳出）
retryLoop:
	for attempt := 0; attempt < maxAttempts; attempt++ {
		actualAttempts = attempt + 1

		// 检查 context 是否已超时/取消
		if ctx.Err() != nil {
			// 超时或取消，不再重试
			if attempt == 0 {
				// 首次尝试就超时/取消，设置错误
				if errors.Is(ctx.Err(), context.DeadlineExceeded) {
					result.Error = fmt.Errorf("请求超时(%v): %w", timeout, ctx.Err())
				} else {
					result.Error = fmt.Errorf("请求取消: %w", ctx.Err())
				}
				result.Status = 0
				result.SubStatus = storage.SubStatusNetworkError
			}
			// 否则保留上一次尝试的结果
			break retryLoop
		}

		// 准备请求体（去除首尾空白，某些 API 对此敏感）
		reqBody := bytes.NewBuffer([]byte(strings.TrimSpace(cfg.Body)))
		req, err := http.NewRequestWithContext(ctx, cfg.Method, cfg.URL, reqBody)
		if err != nil {
			result.Error = fmt.Errorf("创建请求失败: %w", err)
			result.Status = 0
			result.SubStatus = storage.SubStatusNetworkError
			// 请求创建失败是配置问题，重试无意义
			break retryLoop
		}

		// 设置 Headers（已处理过占位符）
		for k, v := range cfg.Headers {
			req.Header.Set(k, v)
		}

		// 发送请求并计时
		start := time.Now()
		resp, err := client.Do(req)
		latency := int(time.Since(start).Milliseconds())
		totalLatency += latency

		if err != nil {
			// 极少数情况下 err != nil 但 resp != nil，需要关闭 body，避免资源泄漏
			drainAndClose(resp)

			// 超时/取消：不重试（总超时已到，继续重试无意义）
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				if errors.Is(err, context.DeadlineExceeded) {
					err = fmt.Errorf("请求超时(%v): %w", timeout, err)
				} else {
					err = fmt.Errorf("请求取消: %w", err)
				}
				logger.Error("probe", "请求失败（不重试）",
					"provider", cfg.Provider, "service", cfg.Service, "channel", cfg.Channel, "model", cfg.Model,
					"attempt", attempt+1, "max_attempts", maxAttempts, "error", err)
				result.Error = err
				result.Status = 0
				result.SubStatus = storage.SubStatusNetworkError
				result.Latency = totalLatency
				break retryLoop // 超时不重试
			}

			// 其他网络错误，设置结果并继续重试
			logger.Error("probe", "请求失败",
				"provider", cfg.Provider, "service", cfg.Service, "channel", cfg.Channel, "model", cfg.Model,
				"attempt", attempt+1, "max_attempts", maxAttempts, "error", err)
			result.Error = err
			result.Status = 0
			result.SubStatus = storage.SubStatusNetworkError
			result.Latency = totalLatency
			result.HttpCode = 0
			lastBodyBytes = nil // 网络错误无响应体

			// 检查是否需要重试
			if attempt+1 < maxAttempts {
				delay := computeRetryDelay(attempt, baseDelay, maxDelay, jitter)
				logger.Info("probe", "准备重试",
					"provider", cfg.Provider, "service", cfg.Service, "channel", cfg.Channel, "model", cfg.Model,
					"attempt", attempt+1, "next_attempt", attempt+2, "delay_ms", delay.Milliseconds())

				// 等待退避时间，同时监听 context
				select {
				case <-ctx.Done():
					// 超时，停止重试
					break retryLoop
				case <-time.After(delay):
					// 继续下一次重试
					continue
				}
			}
			continue
		}

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
					"provider", cfg.Provider, "service", cfg.Service, "channel", cfg.Channel, "model", cfg.Model, "error", readErr, "bytes", len(data))
			default:
				logger.Warn("probe", "读取响应体失败",
					"provider", cfg.Provider, "service", cfg.Service, "channel", cfg.Channel, "model", cfg.Model, "error", readErr)
			}

			// gzip 解压：当 Content-Encoding 包含 gzip 时，手动解压响应体
			// Go 的 http.Transport 在用户显式设置 Accept-Encoding 请求头时不会自动解压
			bodyBytes = decompressGzipIfNeeded(resp, bodyBytes, cfg.Provider, cfg.Service, cfg.Channel, cfg.Model)
		} else {
			_, _ = io.Copy(io.Discard, resp.Body)
		}
		_ = resp.Body.Close()

		// 保存响应体用于最终诊断
		lastBodyBytes = bodyBytes

		// 判定状态（先按 HTTP/延迟，再根据响应内容做二次判断）
		status, subStatus := p.determineStatus(resp.StatusCode, latency, cfg.SlowLatencyDuration)
		result.Status = status
		result.SubStatus = subStatus
		result.Status, result.SubStatus = evaluateStatus(result.Status, result.SubStatus, bodyBytes, cfg.SuccessContains)
		result.Latency = totalLatency
		result.Error = nil

		// 检查是否需要重试
		// 重试条件：status=0（红色）且非超时
		if result.Status == 0 && attempt+1 < maxAttempts {
			// 输出诊断信息（重试前输出）
			p.logFailedProbe(cfg, result, bodyBytes)

			delay := computeRetryDelay(attempt, baseDelay, maxDelay, jitter)
			logger.Info("probe", "探测失败，准备重试",
				"provider", cfg.Provider, "service", cfg.Service, "channel", cfg.Channel, "model", cfg.Model,
				"attempt", attempt+1, "next_attempt", attempt+2, "status", result.Status, "sub_status", result.SubStatus,
				"http_code", result.HttpCode, "delay_ms", delay.Milliseconds())

			// 等待退避时间，同时监听 context
			select {
			case <-ctx.Done():
				// 超时，停止重试，保留当前结果
				break retryLoop
			case <-time.After(delay):
				// 继续下一次重试
				continue
			}
		}

		// 成功（绿色/黄色）或已达最大重试次数，退出循环
		break retryLoop
	}

	// 最终诊断日志（仅在最终结果为红色时输出）
	if result.Status == 0 {
		// 输出诊断信息（使用保存的最后一次响应体）
		p.logFailedProbe(cfg, result, lastBodyBytes)

		logger.Warn("probe", "探测最终失败",
			"provider", cfg.Provider, "service", cfg.Service, "channel", cfg.Channel, "model", cfg.Model,
			"actual_attempts", actualAttempts, "max_attempts", maxAttempts,
			"total_latency_ms", result.Latency, "status", result.Status, "sub_status", result.SubStatus,
			"http_code", result.HttpCode)
	}

	// 日志（不打印敏感信息）
	logger.Info("probe", "探测完成",
		"provider", cfg.Provider, "service", cfg.Service, "channel", cfg.Channel, "model", cfg.Model,
		"code", result.HttpCode, "latency_ms", result.Latency, "status", result.Status, "sub_status", result.SubStatus)

	return result
}

// logFailedProbe 输出探测失败的诊断信息
func (p *Prober) logFailedProbe(cfg *config.ServiceConfig, result *ProbeResult, bodyBytes []byte) {
	const maxSnippetLen = 512 // 防止日志过长

	// content_mismatch 特殊处理：即便响应体为空/仅空白，也输出诊断信息
	if result.SubStatus == storage.SubStatusContentMismatch {
		aggText := aggregateResponseText(bodyBytes)
		trimmed := strings.TrimSpace(aggText)

		if trimmed == "" {
			// body_bytes > 0 但 agg_len = 0 说明聚合器未能提取文本（如二进制/不识别格式）
			logger.Warn("probe", "内容校验失败：响应体为空或无法提取文本",
				"provider", cfg.Provider, "service", cfg.Service, "channel", cfg.Channel, "model", cfg.Model,
				"body_bytes", len(bodyBytes), "agg_len", len(aggText), "keyword_len", len(cfg.SuccessContains))
		} else {
			snippet := trimmed
			if len(snippet) > maxSnippetLen {
				snippet = snippet[:maxSnippetLen] + "... (truncated)"
			}
			logger.Warn("probe", "内容校验失败：未包含预期关键字",
				"provider", cfg.Provider, "service", cfg.Service, "channel", cfg.Channel, "model", cfg.Model,
				"body_bytes", len(bodyBytes), "keyword_len", len(cfg.SuccessContains), "snippet", snippet)
		}
	} else if len(bodyBytes) > 0 {
		// 其他红色状态：保持原有行为，在有响应体时输出片段
		snippet := strings.TrimSpace(aggregateResponseText(bodyBytes))
		if snippet != "" {
			if len(snippet) > maxSnippetLen {
				snippet = snippet[:maxSnippetLen] + "... (truncated)"
			}
			logger.Warn("probe", "响应片段",
				"provider", cfg.Provider, "service", cfg.Service, "channel", cfg.Channel, "model", cfg.Model, "snippet", snippet)
		}
	}
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
func decompressGzipIfNeeded(resp *http.Response, data []byte, provider, service, channel, model string) []byte {
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
			"provider", provider, "service", service, "channel", channel, "model", model, "error", err)
		return data
	}
	defer gr.Close()

	// 读取解压后的数据
	decompressed, err := io.ReadAll(gr)
	if err != nil {
		logger.Warn("probe", "gzip 解压读取失败，使用原始响应体",
			"provider", provider, "service", service, "channel", channel, "model", model, "error", err)
		return data
	}

	logger.Debug("probe", "gzip 解压成功",
		"provider", provider, "service", service, "channel", channel, "model", model,
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

	// 启发式检测 SSE 格式：
	// - 标准 SSE：同时包含 "event:" 和 "data:"
	// - Gemini SSE：只有 "data:" 行（以 "data:" 开头或包含 "\ndata:"）
	isSSE := bytes.Contains(body, []byte("event:")) && bytes.Contains(body, []byte("data:"))
	if !isSSE {
		isSSE = bytes.HasPrefix(body, []byte("data:")) || bytes.Contains(body, []byte("\ndata:"))
	}
	if isSSE {
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
// - OpenAI Chat: data: {"choices":[{"delta":{"content":"..."}}]}
// - OpenAI Responses API:
//   - event: response.output_text.delta + data: {"delta":"..."}（增量）
//   - event: response.output_text.done + data: {"text":"..."}（完整，兜底）
//   - Gemini API: data: {"candidates":[{"content":{"parts":[{"text":"..."}]}}]}
//     注意：Gemini SSE 没有 event: 行，只有 data: 行；流式响应中 text 可能被拆分
//
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

		// OpenAI Chat: {"choices":[{"delta":{"content":"..."}}]}
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

		// OpenAI Responses API: {"delta":"pong",...} - 顶层 delta 直接是字符串
		if delta, ok := obj["delta"].(string); ok {
			appendText(delta)
		}

		// OpenAI Responses API: {"text":"pong",...} - response.output_text.done 事件
		// 完整 text 通常已通过增量累积，这里仅作兜底（当 builder 为空时使用）
		if text, ok := obj["text"].(string); ok {
			if b.Len() == 0 {
				appendText(text)
			}
		}

		// Gemini API: {"candidates":[{"content":{"parts":[{"text":"..."}]}}]}
		// 流式响应中 text 可能被拆分到多个 chunk（如 "po" + "ng"），需要累积拼接
		if candidates, ok := obj["candidates"].([]any); ok {
			for _, c := range candidates {
				cm, ok := c.(map[string]any)
				if !ok {
					continue
				}
				content, ok := cm["content"].(map[string]any)
				if !ok {
					continue
				}
				parts, ok := content["parts"].([]any)
				if !ok {
					continue
				}
				for _, p := range parts {
					pm, ok := p.(map[string]any)
					if !ok {
						continue
					}
					if text, ok := pm["text"].(string); ok {
						appendText(text)
					}
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
// 返回保存后的记录（包含生成的 ID）和错误
func (p *Prober) SaveResult(result *ProbeResult) (*storage.ProbeRecord, error) {
	record := &storage.ProbeRecord{
		Provider:  result.Provider,
		Service:   result.Service,
		Channel:   result.Channel,
		Model:     result.Model,
		Status:    result.Status,
		SubStatus: result.SubStatus,
		HttpCode:  result.HttpCode,
		Latency:   result.Latency,
		Timestamp: result.Timestamp,
	}

	if err := p.storage.SaveRecord(record); err != nil {
		return nil, err
	}
	return record, nil
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

// computeRetryDelay 计算指数退避延迟
// 公式: delay = min(maxDelay, baseDelay * 2^retryIndex) * (1 ± jitter)
// retryIndex 从 0 开始（首次重试 retryIndex=0）
// 注意：最终结果会再次 cap 到 maxDelay，确保不超过上限
func computeRetryDelay(retryIndex int, baseDelay, maxDelay time.Duration, jitter float64) time.Duration {
	// 指数退避：baseDelay * 2^retryIndex
	delay := baseDelay
	for i := 0; i < retryIndex; i++ {
		delay *= 2
		if delay > maxDelay {
			delay = maxDelay
			break
		}
	}

	// 上限保护
	if delay > maxDelay {
		delay = maxDelay
	}

	// 应用抖动：delay * (1 ± jitter)
	if jitter > 0 {
		// 生成 [-jitter, +jitter] 范围的随机偏移
		jitterRange := float64(delay) * jitter
		offset := (rand.Float64()*2 - 1) * jitterRange // [-jitterRange, +jitterRange]
		delay = time.Duration(float64(delay) + offset)
	}

	// 确保延迟不为负
	if delay < 0 {
		delay = baseDelay
	}

	// 抖动后再次 cap 到 maxDelay，确保最终结果不超过上限
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}

// drainAndClose 排空响应体并关闭，便于连接复用
// 当重试时需要释放上一次响应的资源
func drainAndClose(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	// 限制读取量，防止恶意大响应阻塞
	const maxDrainBytes = 64 * 1024
	_, _ = io.CopyN(io.Discard, resp.Body, maxDrainBytes)
	_ = resp.Body.Close()
}
