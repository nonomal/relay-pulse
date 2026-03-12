package selftest

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"monitor/internal/config"
	"monitor/internal/logger"
)

// templatesDir 存储 templates/ 目录的路径（由 main.go 初始化时设置）
var (
	templatesDir     string
	templatesDirOnce sync.Once
)

// SetTemplatesDir 设置模板目录路径（应在 main.go 中调用）
// 该目录包含 cc-haiku-tiny.json、cx-codex-base.json 等模板文件
func SetTemplatesDir(dir string) {
	templatesDirOnce.Do(func() {
		templatesDir = dir
		logger.Info("selftest", "模板目录已设置", "path", templatesDir)
	})
}

// PayloadVariant 描述请求体模板的一个变体
type PayloadVariant struct {
	ID              string `json:"id"`                         // 变体标识，如 "cc-haiku-tiny"
	Filename        string `json:"filename"`                   // 模板文件名，如 "cc-haiku-tiny.json"
	Order           int    `json:"order"`                      // UI 排序权重
	Model           string `json:"model,omitempty"`            // 默认模型名（用于替换模板中的 {{MODEL}}）
	SuccessContains string `json:"success_contains,omitempty"` // 响应校验关键字（空则使用模板默认值）
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

// TemplateBuilder 从 templates/ 目录加载 JSON 模板构建测试配置
// 统一替代原来的 CCTestBuilder / CXTestBuilder / GMTestBuilder
type TemplateBuilder struct {
	Service string // 服务标识，如 "cc", "cx", "gm"
}

func (b *TemplateBuilder) Build(apiURL, apiKey string, variant *PayloadVariant) (*config.ServiceConfig, error) {
	if apiURL == "" {
		return nil, fmt.Errorf("api_url is required")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("api_key is required")
	}
	if variant == nil || variant.Filename == "" {
		return nil, fmt.Errorf("payload variant is required")
	}
	if templatesDir == "" {
		return nil, fmt.Errorf("templates directory not set, call SetTemplatesDir first")
	}

	tmpl, err := config.LoadProbeTemplate(filepath.Join(templatesDir, variant.Filename))
	if err != nil {
		return nil, fmt.Errorf("failed to load template %s: %w", variant.Filename, err)
	}

	// 从模板复制 headers（保留占位符，由 InjectVariables 在探测时替换）
	headers := make(map[string]string, len(tmpl.Headers))
	for k, v := range tmpl.Headers {
		headers[k] = v
	}

	successContains := tmpl.SuccessContains
	if variant.SuccessContains != "" {
		successContains = variant.SuccessContains
	}

	return &config.ServiceConfig{
		Provider:            "selftest",
		Service:             b.Service,
		BaseURL:             apiURL,
		APIKey:              apiKey,
		Model:               variant.Model,
		URLPattern:          tmpl.URL, // selftest: 使用模板 URL 模式（支持 {{BASE_URL}}/{{MODEL}} 等占位符）
		Method:              tmpl.Method,
		Headers:             headers,
		Body:                string(tmpl.BodyRaw),
		SuccessContains:     successContains,
		SlowLatencyDuration: 5 * time.Second,
	}, nil
}

// init registers built-in test types
func init() {
	ccVariants := []*PayloadVariant{
		{ID: "cc-haiku-tiny", Filename: "cc-haiku-tiny.json", Order: 1, Model: "claude-haiku-4-5-20251001"},
		{ID: "cc-opus-tiny", Filename: "cc-opus-tiny.json", Order: 2, Model: "claude-opus-4-5-20251101"},
		{ID: "cc-sonnet-tiny", Filename: "cc-sonnet-tiny.json", Order: 3, Model: "claude-sonnet-4-5-20250929"},
		{ID: "cc-arith", Filename: "cc-arith.json", Order: 10, Model: "claude-haiku-4-5-20251001"},
	}

	cxVariants := []*PayloadVariant{
		{ID: "cx-codex-base", Filename: "cx-codex-base.json", Order: 1, Model: "gpt-5-codex"},
		{ID: "cx-codexmax-base", Filename: "cx-codexmax-base.json", Order: 2, Model: "gpt-5.1-codex-max"},
		{ID: "cx-codexmini-base", Filename: "cx-codexmini-base.json", Order: 3, Model: "gpt-5.1-codex-mini"},
		{ID: "cx-arith", Filename: "cx-arith.json", Order: 10, Model: "gpt-5-codex"},
	}

	gmVariants := []*PayloadVariant{
		{ID: "gm-base", Filename: "gm-base.json", Order: 1, Model: "gemini-2.5-flash"},
		{ID: "gm-thinking", Filename: "gm-thinking.json", Order: 2, Model: "gemini-2.5-flash-thinking"},
		{ID: "gm-generate", Filename: "gm-generate.json", Order: 3, Model: "gemini-2.5-flash"},
		{ID: "gm-arith", Filename: "gm-arith.json", Order: 10, Model: "gemini-2.5-flash"},
	}

	RegisterTestType(&TestType{
		ID:             "cc",
		Name:           "Claude Code (cc)",
		Description:    "",
		DefaultVariant: "cc-haiku-tiny",
		Variants:       ccVariants,
		Builder:        &TemplateBuilder{Service: "cc"},
	})

	RegisterTestType(&TestType{
		ID:             "cx",
		Name:           "Codex (cx)",
		Description:    "",
		DefaultVariant: "cx-codex-base",
		Variants:       cxVariants,
		Builder:        &TemplateBuilder{Service: "cx"},
	})

	RegisterTestType(&TestType{
		ID:             "gm",
		Name:           "Gemini (gm)",
		Description:    "",
		DefaultVariant: "gm-base",
		Variants:       gmVariants,
		Builder:        &TemplateBuilder{Service: "gm"},
	})
}
