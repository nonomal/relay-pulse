package selftest

import (
	"fmt"
	"os"
	"path/filepath"
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
// 该目录包含 cc_base.json 和 cx_base.json 等模板文件
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

// TestConfigBuilder is an interface for building test configurations
// Each test type implements this interface to generate its specific config
type TestConfigBuilder interface {
	Build(apiURL, apiKey string) (*config.ServiceConfig, error)
}

// TestType represents a type of self-test that can be performed
type TestType struct {
	ID          string            // Unique identifier (e.g., "cc", "cx", "embeddings")
	Name        string            // Display name
	Description string            // Description for UI
	Builder     TestConfigBuilder // Configuration builder
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

// ListTestTypes returns all registered test types
func ListTestTypes() []*TestType {
	var types []*TestType
	for _, t := range testTypeRegistry {
		types = append(types, t)
	}
	return types
}

// CCTestBuilder builds configuration for Claude Chat Completions (non-streaming)
// Headers and Body are aligned with cc-template in config.yaml
type CCTestBuilder struct{}

func (b *CCTestBuilder) Build(apiURL, apiKey string) (*config.ServiceConfig, error) {
	if apiURL == "" {
		return nil, fmt.Errorf("api_url is required")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("api_key is required")
	}

	// 动态读取请求体模板（与主站 data/cc_base.json 保持同步）
	body, err := loadBodyTemplate("cc_base.json")
	if err != nil {
		return nil, fmt.Errorf("failed to load CC body template: %w", err)
	}

	return &config.ServiceConfig{
		Provider: "selftest",
		Service:  "cc",
		URL:      apiURL,
		Method:   "POST",
		// Headers 对齐 config.yaml 中的 cc-template
		Headers: map[string]string{
			"Authorization":     "Bearer " + apiKey,
			"Content-Type":      "application/json",
			"User-Agent":        "claude-cli/2.0.45 (external, cli)",
			"Anthropic-Beta":    "interleaved-thinking-2025-05-14,context-management-2025-06-27,tool-examples-2025-10-29",
			"Anthropic-Version": "2023-06-01",
			"X-App":             "cli",
		},
		Body:                body,
		SuccessContains:     "isNewTopic", // 对齐 cc-template
		SlowLatencyDuration: 5 * time.Second,
	}, nil
}

// CXTestBuilder builds configuration for Codex (OpenAI Responses API)
// Headers and Body are aligned with cx-template in config.yaml
type CXTestBuilder struct{}

func (b *CXTestBuilder) Build(apiURL, apiKey string) (*config.ServiceConfig, error) {
	if apiURL == "" {
		return nil, fmt.Errorf("api_url is required")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("api_key is required")
	}

	// 动态读取请求体模板（与主站 data/cx_base.json 保持同步）
	body, err := loadBodyTemplate("cx_base.json")
	if err != nil {
		return nil, fmt.Errorf("failed to load CX body template: %w", err)
	}

	return &config.ServiceConfig{
		Provider: "selftest",
		Service:  "cx",
		URL:      apiURL,
		Method:   "POST",
		// Headers 对齐 config.yaml 中的 cx-template（使用 OpenAI Responses API）
		Headers: map[string]string{
			"Authorization": "Bearer " + apiKey,
			"Content-Type":  "application/json",
			"openai-beta":   "responses=experimental",
		},
		Body:                body,
		SuccessContains:     "pong", // 对齐 cx-template
		SlowLatencyDuration: 5 * time.Second,
	}, nil
}

// init registers built-in test types
func init() {
	RegisterTestType(&TestType{
		ID:          "cc",
		Name:        "Claude Code (cc)",
		Description: "",
		Builder:     &CCTestBuilder{},
	})

	RegisterTestType(&TestType{
		ID:          "cx",
		Name:        "Codex (cx)",
		Description: "",
		Builder:     &CXTestBuilder{},
	})

	// Future test types can be easily added:
	// RegisterTestType(&TestType{
	//     ID:          "embeddings",
	//     Name:        "Text Embeddings",
	//     Description: "文本嵌入向量生成测试",
	//     Builder:     &EmbeddingsTestBuilder{},
	// })
}
