package selftest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"monitor/internal/config"
)

// DefaultMaxResponseBytes 自助测试响应体读取上限（避免内存/带宽被滥用）
const DefaultMaxResponseBytes int64 = 10 << 20 // 10MB

// ProbeResult 自助测试探测结果（仅供 selftest 使用）
type ProbeResult struct {
	Status          int    // 1=绿, 0=红, 2=黄
	SubStatus       string // 细分状态（字符串，便于前端展示/排查）
	HTTPCode        int
	Latency         int    // ms
	ResponseSnippet string // 服务端响应片段（错误时便于排查）
	Err             error
}

// SelfTestProber 自助测试专用探测器（不复用 monitor.Prober，使用安全 HTTP 客户端）
type SelfTestProber struct {
	client       *http.Client
	maxBodyBytes int64
}

// NewSelfTestProber 创建自助测试探测器
func NewSelfTestProber(guard *SSRFGuard, maxBodyBytes int64) *SelfTestProber {
	if maxBodyBytes <= 0 {
		maxBodyBytes = DefaultMaxResponseBytes
	}
	return &SelfTestProber{
		client:       newSafeHTTPClient(guard),
		maxBodyBytes: maxBodyBytes,
	}
}

// Probe 执行一次自助测试探测（带响应体大小限制，且禁用重定向）
func (p *SelfTestProber) Probe(ctx context.Context, cfg *config.ServiceConfig) *ProbeResult {
	result := &ProbeResult{
		Status:    0,
		SubStatus: "none",
		HTTPCode:  0,
		Latency:   0,
	}

	reqBody := bytes.NewBuffer([]byte(strings.TrimSpace(cfg.Body)))
	req, err := http.NewRequestWithContext(ctx, cfg.Method, cfg.URL, reqBody)
	if err != nil {
		result.SubStatus = "invalid_request"
		result.Err = fmt.Errorf("创建请求失败: %w", err)
		return result
	}

	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := p.client.Do(req)
	latency := int(time.Since(start).Milliseconds())
	result.Latency = latency

	if err != nil {
		result.SubStatus = "network_error"
		result.Err = err
		return result
	}
	defer resp.Body.Close()

	result.HTTPCode = resp.StatusCode

	// 始终读取响应体（用于内容校验和错误排查）
	body, err := p.readBodyLimited(resp.Body, p.maxBodyBytes)
	if err != nil {
		result.SubStatus = "response_too_large"
		result.Err = err
		return result
	}

	status, sub := determineSelfTestStatus(resp.StatusCode, latency, cfg.SlowLatencyDuration)
	result.Status = status
	result.SubStatus = sub

	// 保存响应片段（用于错误排查，仅保存前 512 字符）
	if len(body) > 0 {
		snippet := strings.TrimSpace(string(body))
		const maxSnippetLen = 512
		if len(snippet) > maxSnippetLen {
			snippet = snippet[:maxSnippetLen] + "... (truncated)"
		}
		result.ResponseSnippet = snippet
	}

	// 内容校验：仅对非红状态做进一步判断（避免误把网络错误覆盖为内容不匹配）
	if result.Status != 0 && strings.TrimSpace(cfg.SuccessContains) != "" {
		if len(body) == 0 || !bytes.Contains(body, []byte(cfg.SuccessContains)) {
			result.Status = 0
			result.SubStatus = "content_mismatch"
			result.Err = fmt.Errorf("响应内容未包含预期关键字")
			return result
		}
	}

	return result
}

func (p *SelfTestProber) readBodyLimited(r io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		limit = DefaultMaxResponseBytes
	}
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return data, err
	}
	if int64(len(data)) > limit {
		return data[:limit], fmt.Errorf("响应体超过上限 %d bytes", limit)
	}
	return data, nil
}

func determineSelfTestStatus(statusCode, latency int, slowLatency time.Duration) (int, string) {
	// 2xx = 绿（或慢速黄）
	if statusCode >= 200 && statusCode < 300 {
		if slowLatency > 0 && latency > int(slowLatency/time.Millisecond) {
			return 2, "slow_latency"
		}
		return 1, "none"
	}

	// 3xx：重定向已被禁用，视为失败（避免 SSRF/绕过）
	if statusCode >= 300 && statusCode < 400 {
		return 0, "redirect_blocked"
	}

	// 401/403：鉴权失败
	if statusCode == 401 || statusCode == 403 {
		return 0, "auth_error"
	}

	// 400：请求错误
	if statusCode == 400 {
		return 0, "invalid_request"
	}

	// 429：被限流
	if statusCode == 429 {
		return 0, "rate_limited"
	}

	// 5xx：服务端错误
	if statusCode >= 500 {
		return 0, "server_error"
	}

	// 其他 4xx：客户端错误
	if statusCode >= 400 {
		return 0, "client_error"
	}

	// 兜底
	return 0, "unknown_error"
}
