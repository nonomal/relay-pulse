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

// templatesDir 存储 templates/ 目录的路径（由 main.go 初始化时设置）
var (
	templatesDir     string
	templatesDirOnce sync.Once
)

// SetTemplatesDir 设置模板目录路径（应在 main.go 中调用）
// 该目录包含 cc-haiku-arith.json、cx-codex-arith.json 等模板文件
func SetTemplatesDir(dir string) {
	templatesDirOnce.Do(func() {
		templatesDir = dir
		logger.Info("selftest", "模板目录已设置", "path", templatesDir)
	})
}

// PayloadVariant 描述请求体模板的一个变体
type PayloadVariant struct {
	ID              string `json:"id"`                         // 变体标识，如 "cc-haiku-arith"
	Filename        string `json:"filename"`                   // 模板文件名，如 "cc-haiku-arith.json"
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

// testTypeRegistry 全局注册表，受 registryMu 保护
var (
	registryMu       sync.RWMutex
	testTypeRegistry = make(map[string]*TestType)
)

// RegisterTestType registers a new test type in the global registry
func RegisterTestType(t *TestType) {
	registryMu.Lock()
	testTypeRegistry[t.ID] = t
	registryMu.Unlock()
}

// GetTestType retrieves a test type by ID
func GetTestType(id string) (*TestType, bool) {
	registryMu.RLock()
	t, ok := testTypeRegistry[id]
	registryMu.RUnlock()
	return t, ok
}

// ListTestTypes returns all registered test types sorted by ID for stable output
func ListTestTypes() []*TestType {
	registryMu.RLock()
	types := make([]*TestType, 0, len(testTypeRegistry))
	for _, t := range testTypeRegistry {
		types = append(types, t)
	}
	registryMu.RUnlock()

	sort.Slice(types, func(i, j int) bool {
		return types[i].ID < types[j].ID
	})
	return types
}

// InitTemplates 扫描 templates/ 目录，按文件名约定（{service}-*.json）
// 动态填充已注册 TestType 的 Variants 和 DefaultVariant。
// 应在 SetTemplatesDir 之后、创建 TestJobManager 之前调用。
func InitTemplates(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("读取模板目录失败 %s: %w", dir, err)
	}

	// 按 service 前缀分组收集变体（匹配 {service}-*.json）
	grouped := make(map[string][]*PayloadVariant)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		filename := entry.Name()
		variantID := strings.TrimSuffix(filename, ".json")

		// 文件名约定：{service}-*.json；取第一个 '-' 之前的部分作为 service 前缀
		idx := strings.IndexByte(variantID, '-')
		if idx <= 0 {
			continue
		}
		service := variantID[:idx]

		grouped[service] = append(grouped[service], &PayloadVariant{
			ID:       variantID,
			Filename: filename,
		})
	}

	// 排序并设置 Order
	for _, variants := range grouped {
		sort.Slice(variants, func(i, j int) bool {
			return variants[i].ID < variants[j].ID
		})
		for i := range variants {
			variants[i].Order = i + 1
		}
	}

	// 原子替换注册表：构建新 map，一次性替换
	registryMu.Lock()
	next := make(map[string]*TestType, len(testTypeRegistry))
	totalVariants := 0
	for id, current := range testTypeRegistry {
		updated := *current // 浅拷贝元数据（Name/Builder 等不变）
		if variants, ok := grouped[id]; ok && len(variants) > 0 {
			updated.Variants = variants
			updated.DefaultVariant = variants[0].ID
			totalVariants += len(variants)
		} else {
			updated.Variants = nil
			updated.DefaultVariant = ""
		}
		next[id] = &updated
	}
	testTypeRegistry = next
	registryMu.Unlock()

	logger.Info("selftest", "自助测试模板已刷新",
		"templates_dir", dir, "variants", totalVariants)
	return nil
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

	// 模型元数据：variant > template
	model := variant.Model
	if model == "" {
		model = tmpl.Model
	}
	requestModel := tmpl.RequestModel

	// 从模板读取 slow_latency，兜底 5s
	slowLatency := 5 * time.Second
	if tmpl.SlowLatency != "" {
		if d, err := time.ParseDuration(tmpl.SlowLatency); err == nil && d > 0 {
			slowLatency = d
		}
	}

	// 从模板读取 timeout，兜底 10s
	timeout := 10 * time.Second
	if tmpl.Timeout != "" {
		if d, err := time.ParseDuration(tmpl.Timeout); err == nil && d > 0 {
			timeout = d
		}
	}

	return &config.ServiceConfig{
		Provider:            "selftest",
		Service:             b.Service,
		BaseURL:             apiURL,
		APIKey:              apiKey,
		Model:               model,
		RequestModel:        requestModel,
		URLPattern:          tmpl.URL,
		Method:              tmpl.Method,
		Headers:             headers,
		Body:                string(tmpl.BodyRaw),
		SuccessContains:     successContains,
		SlowLatencyDuration: slowLatency,
		TimeoutDuration:     timeout,
	}, nil
}

// init registers built-in test types with metadata only.
// Variants are populated later by InitTemplates after the templates directory is known.
func init() {
	RegisterTestType(&TestType{
		ID:      "cc",
		Name:    "Claude Code (cc)",
		Builder: &TemplateBuilder{Service: "cc"},
	})

	RegisterTestType(&TestType{
		ID:      "cx",
		Name:    "Codex (cx)",
		Builder: &TemplateBuilder{Service: "cx"},
	})

	RegisterTestType(&TestType{
		ID:      "gm",
		Name:    "Gemini (gm)",
		Builder: &TemplateBuilder{Service: "gm"},
	})
}
