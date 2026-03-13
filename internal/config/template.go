package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"monitor/internal/logger"
)

// ProbeTemplate 描述一次探测请求的完整模板（来自 templates/*.json）
type ProbeTemplate struct {
	Model           string            // 模型系列名（展示/DB 键）
	RequestModel    string            // 实际请求模型 ID（可选，为空时回退 Model）
	URL             string            // URL 模式，支持 {{BASE_URL}} 等占位符
	Method          string            // HTTP 方法
	Headers         map[string]string // 请求头，支持占位符
	BodyRaw         json.RawMessage   // body 原始 JSON 对象
	SuccessContains string            // 响应校验关键字，支持 {{EXPECTED_ANSWER}}
	SlowLatency     string            // 慢请求阈值（可选，如 "5s"）
	Timeout         string            // 超时时间（可选，如 "10s"）
}

// probeTemplateFile 是模板 JSON 文件的解析结构
type probeTemplateFile struct {
	Model        string            `json:"model"`
	RequestModel string            `json:"request_model"`
	URL          string            `json:"url"`
	Method       string            `json:"method"`
	Headers      map[string]string `json:"headers"`
	Body         json.RawMessage   `json:"body"`
	Response     struct {
		SuccessContains string `json:"success_contains"`
		SlowLatency     string `json:"slow_latency"`
		Timeout         string `json:"timeout"`
	} `json:"response"`
}

// LoadProbeTemplate 从 JSON 文件加载探测模板
func LoadProbeTemplate(filePath string) (*ProbeTemplate, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取模板文件失败 %s: %w", filePath, err)
	}

	var parsed probeTemplateFile
	if err := json.Unmarshal(content, &parsed); err != nil {
		return nil, fmt.Errorf("解析模板 JSON 失败 %s: %w", filePath, err)
	}

	tmpl := &ProbeTemplate{
		Model:           strings.TrimSpace(parsed.Model),
		RequestModel:    strings.TrimSpace(parsed.RequestModel),
		URL:             strings.TrimSpace(parsed.URL),
		Method:          strings.TrimSpace(parsed.Method),
		Headers:         parsed.Headers,
		BodyRaw:         parsed.Body,
		SuccessContains: strings.TrimSpace(parsed.Response.SuccessContains),
		SlowLatency:     strings.TrimSpace(parsed.Response.SlowLatency),
		Timeout:         strings.TrimSpace(parsed.Response.Timeout),
	}

	if tmpl.Method == "" {
		return nil, fmt.Errorf("模板 %s 未配置 method", filePath)
	}

	logger.Info("config", "模板加载完成", "path", filePath)
	return tmpl, nil
}
