package selftest

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"monitor/internal/config"
	"monitor/internal/logger"
)

// dataDir 存储 data/ 目录的路径（由 main.go 初始化时设置）
var (
	dataDir     string
	dataDirOnce sync.Once
)

// SetDataDir 设置数据目录路径（应在 main.go 中调用）
// 该目录包含 cc-haiku-base.json、cx-codex-base.json 等模板文件
func SetDataDir(dir string) {
	dataDirOnce.Do(func() {
		dataDir = dir
		logger.Info("selftest", "数据目录已设置", "path", dataDir)
	})
}

// loadBodyTemplate 从 data/ 目录读取请求体模板文件
func loadBodyTemplate(filename string) (string, error) {
	if dataDir == "" {
		return "", fmt.Errorf("data directory not set, call SetDataDir first")
	}

	filePath := filepath.Join(dataDir, filename)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read body template %s: %w", filename, err)
	}

	return string(content), nil
}

// PayloadVariant 描述请求体模板的一个变体
type PayloadVariant struct {
	ID       string `json:"id"`       // 变体标识，如 "cc-haiku-base"
	Filename string `json:"filename"` // 模板文件名，如 "cc-haiku-base.json"
	Order    int    `json:"order"`    // UI 排序权重
}

// TestConfigBuilder is an interface for building test configurations
// Each test type implements this interface to generate its specific config
type TestConfigBuilder interface {
	Build(apiURL, apiKey string, variant *PayloadVariant) (*config.ServiceConfig, error)
}

// TestType represents a type of self-test that can be performed
type TestType struct {
	ID             string            // Unique identifier (e.g., "cc", "cx")
	Name           string            // Display name
	Description    string            // Description for UI
	DefaultVariant string            // 默认 payload 变体 ID
	Variants       []*PayloadVariant // 可选 payload 变体列表
	Builder        TestConfigBuilder // Configuration builder
}

// ResolveVariant 根据 variantID 解析 payload 变体；空 ID 回退到默认变体
func (t *TestType) ResolveVariant(variantID string) (*PayloadVariant, error) {
	id := strings.TrimSpace(variantID)
	if id == "" {
		id = t.DefaultVariant
	}
	if id == "" {
		return nil, fmt.Errorf("default payload variant not set for test type: %s", t.ID)
	}

	for _, v := range t.Variants {
		if v.ID == id {
			return v, nil
		}
	}

	return nil, &Error{
		Code:    ErrCodeUnknownVariant,
		Message: "不支持的 payload 变体",
		Err:     fmt.Errorf("unknown payload variant %q for test type %q", id, t.ID),
	}
}

// Global test type registry
var testTypeRegistry = make(map[string]*TestType)

// RegisterTestType registers a new test type in the global registry
func RegisterTestType(t *TestType) {
	testTypeRegistry[t.ID] = t
}

// GetTestType retrieves a test type by ID
func GetTestType(id string) (*TestType, bool) {
	t, ok := testTypeRegistry[id]
	return t, ok
}

// ListTestTypes returns all registered test types sorted by ID for stable output
func ListTestTypes() []*TestType {
	types := make([]*TestType, 0, len(testTypeRegistry))
	for _, t := range testTypeRegistry {
		types = append(types, t)
	}
	sort.Slice(types, func(i, j int) bool {
		return types[i].ID < types[j].ID
	})
	return types
}

// CCTestBuilder builds configuration for Claude Chat Completions (non-streaming)
type CCTestBuilder struct{}

func (b *CCTestBuilder) Build(apiURL, apiKey string, variant *PayloadVariant) (*config.ServiceConfig, error) {
	if apiURL == "" {
		return nil, fmt.Errorf("api_url is required")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("api_key is required")
	}
	if variant == nil || variant.Filename == "" {
		return nil, fmt.Errorf("payload variant is required")
	}

	body, err := loadBodyTemplate(variant.Filename)
	if err != nil {
		return nil, fmt.Errorf("failed to load CC body template: %w", err)
	}

	return &config.ServiceConfig{
		Provider: "selftest",
		Service:  "cc",
		URL:      apiURL,
		Method:   "POST",
		Headers: map[string]string{
			"Authorization":     "Bearer " + apiKey,
			"Content-Type":      "application/json",
			"User-Agent":        "claude-cli/2.0.45 (external, cli)",
			"Anthropic-Beta":    "interleaved-thinking-2025-05-14,context-management-2025-06-27,tool-examples-2025-10-29",
			"Anthropic-Version": "2023-06-01",
			"X-App":             "cli",
		},
		Body:                body,
		SuccessContains:     "isNewTopic",
		SlowLatencyDuration: 5 * time.Second,
	}, nil
}

// CXTestBuilder builds configuration for Codex (OpenAI Responses API)
type CXTestBuilder struct{}

func (b *CXTestBuilder) Build(apiURL, apiKey string, variant *PayloadVariant) (*config.ServiceConfig, error) {
	if apiURL == "" {
		return nil, fmt.Errorf("api_url is required")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("api_key is required")
	}
	if variant == nil || variant.Filename == "" {
		return nil, fmt.Errorf("payload variant is required")
	}

	body, err := loadBodyTemplate(variant.Filename)
	if err != nil {
		return nil, fmt.Errorf("failed to load CX body template: %w", err)
	}

	return &config.ServiceConfig{
		Provider: "selftest",
		Service:  "cx",
		URL:      apiURL,
		Method:   "POST",
		Headers: map[string]string{
			"Authorization": "Bearer " + apiKey,
			"Content-Type":  "application/json",
			"openai-beta":   "responses=experimental",
		},
		Body:                body,
		SuccessContains:     "pong",
		SlowLatencyDuration: 5 * time.Second,
	}, nil
}

// GMTestBuilder builds configuration for Gemini (v1beta streamGenerateContent)
type GMTestBuilder struct{}

func (b *GMTestBuilder) Build(apiURL, apiKey string, variant *PayloadVariant) (*config.ServiceConfig, error) {
	if apiURL == "" {
		return nil, fmt.Errorf("api_url is required")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("api_key is required")
	}
	if variant == nil || variant.Filename == "" {
		return nil, fmt.Errorf("payload variant is required")
	}

	body, err := loadBodyTemplate(variant.Filename)
	if err != nil {
		return nil, fmt.Errorf("failed to load GM body template: %w", err)
	}

	return &config.ServiceConfig{
		Provider: "selftest",
		Service:  "gm",
		URL:      apiURL,
		Method:   "POST",
		Headers: map[string]string{
			"User-Agent":        "GeminiCLI/0.22.2/gemini-3-pro-preview (darwin; arm64)",
			"x-goog-api-client": "google-genai-sdk/1.30.0 gl-node/v24.10.0",
			"Content-Type":      "application/json",
			"x-goog-api-key":    apiKey,
		},
		Body:                body,
		SuccessContains:     "pong",
		SlowLatencyDuration: 5 * time.Second,
	}, nil
}

// init registers built-in test types
func init() {
	ccVariants := []*PayloadVariant{
		{ID: "cc-haiku-base", Filename: "cc-haiku-base.json", Order: 1},
		{ID: "cc-haiku-tiny", Filename: "cc-haiku-tiny.json", Order: 2},
		{ID: "cc-opus-tiny", Filename: "cc-opus-tiny.json", Order: 3},
		{ID: "cc-sonnet-tiny", Filename: "cc-sonnet-tiny.json", Order: 4},
	}

	cxVariants := []*PayloadVariant{
		{ID: "cx-codex-base", Filename: "cx-codex-base.json", Order: 1},
		{ID: "cx-gpt52-base", Filename: "cx-gpt52-base.json", Order: 2},
		{ID: "cx-codexmax-base", Filename: "cx-codexmax-base.json", Order: 3},
		{ID: "cx-codexmini-base", Filename: "cx-codexmini-base.json", Order: 4},
	}

	gmVariants := []*PayloadVariant{
		{ID: "gm-base", Filename: "gm-base.json", Order: 1},
		{ID: "gm-thinking", Filename: "gm-thinking.json", Order: 2},
	}

	RegisterTestType(&TestType{
		ID:             "cc",
		Name:           "Claude Code (cc)",
		Description:    "",
		DefaultVariant: "cc-haiku-base",
		Variants:       ccVariants,
		Builder:        &CCTestBuilder{},
	})

	RegisterTestType(&TestType{
		ID:             "cx",
		Name:           "Codex (cx)",
		Description:    "",
		DefaultVariant: "cx-codex-base",
		Variants:       cxVariants,
		Builder:        &CXTestBuilder{},
	})

	RegisterTestType(&TestType{
		ID:             "gm",
		Name:           "Gemini (gm)",
		Description:    "",
		DefaultVariant: "gm-base",
		Variants:       gmVariants,
		Builder:        &GMTestBuilder{},
	})
}
