package generator

import (
	"os"
	"strings"
	"testing"
)

func TestGenerateConfig(t *testing.T) {
	monitors := []map[string]string{
		{
			"provider":         "openai",
			"service":          "cx",
			"category":         "commercial",
			"sponsor":          "team",
			"channel":          "standard",
			"board":            "hot",
			"base_url":         "https://api.openai.com",
			"url_pattern":      "{{BASE_URL}}/v1/chat/completions",
			"method":           "POST",
			"success_contains": "choices",
		},
	}

	config, err := GenerateConfig("1m", "5s", "10s", monitors)
	if err != nil {
		t.Fatalf("GenerateConfig 失败: %v", err)
	}

	// 验证配置包含必要的字段
	if !strings.Contains(config, "interval: \"1m\"") {
		t.Error("配置缺少 interval")
	}
	if !strings.Contains(config, "slow_latency: \"5s\"") {
		t.Error("配置缺少 slow_latency")
	}
	if !strings.Contains(config, "timeout: \"10s\"") {
		t.Error("配置缺少 timeout")
	}
	if !strings.Contains(config, "provider: \"openai\"") {
		t.Error("配置缺少 provider")
	}
	if !strings.Contains(config, "service: \"cx\"") {
		t.Error("配置缺少 service")
	}
	if !strings.Contains(config, "board: \"hot\"") {
		t.Error("配置缺少 board")
	}
	if !strings.Contains(config, "base_url: \"https://api.openai.com\"") {
		t.Error("配置缺少 base_url")
	}
	if strings.Contains(config, "    url:") {
		t.Error("配置不应包含旧格式的 url 字段")
	}
}

func TestGenerateFromTemplate(t *testing.T) {
	templates := []string{"openai", "anthropic", "custom"}

	for _, tmpl := range templates {
		config, err := GenerateFromTemplate(tmpl)
		if err != nil {
			t.Errorf("生成模板 %s 失败: %v", tmpl, err)
		}
		if config == "" {
			t.Errorf("模板 %s 为空", tmpl)
		}
		if !strings.Contains(config, "interval:") {
			t.Errorf("模板 %s 缺少 interval", tmpl)
		}
	}
}

func TestTemplateFormatNewStyle(t *testing.T) {
	// 验证使用新格式（template + base_url）的模板
	newStyleTemplates := map[string]struct {
		baseURL  string
		template string
	}{
		"openai":    {baseURL: "https://api.openai.com", template: "cx-codex-base"},
		"anthropic": {baseURL: "https://api.anthropic.com", template: "cc-haiku-base"},
		"gemini":    {baseURL: "https://generativelanguage.googleapis.com", template: "gm-base"},
	}

	for name, expect := range newStyleTemplates {
		config, err := GenerateFromTemplate(name)
		if err != nil {
			t.Fatalf("生成模板 %s 失败: %v", name, err)
		}
		if !strings.Contains(config, "base_url: \""+expect.baseURL+"\"") {
			t.Errorf("模板 %s 缺少 base_url: %s", name, expect.baseURL)
		}
		if !strings.Contains(config, "template: \""+expect.template+"\"") {
			t.Errorf("模板 %s 缺少 template: %s", name, expect.template)
		}
		// 新格式不应包含 url/method/headers/body
		if strings.Contains(config, "    url:") {
			t.Errorf("模板 %s 不应包含 url 字段（应使用 base_url + template）", name)
		}
		if strings.Contains(config, "    method:") {
			t.Errorf("模板 %s 不应包含 method 字段", name)
		}
	}
}

func TestTemplateFormatLegacyStyle(t *testing.T) {
	// 验证仍使用传统格式（暂无 data/ 模板）的配置
	legacyTemplates := []string{"cohere", "mistral", "custom"}

	for _, name := range legacyTemplates {
		config, err := GenerateFromTemplate(name)
		if err != nil {
			t.Fatalf("生成模板 %s 失败: %v", name, err)
		}
		if !strings.Contains(config, "base_url:") {
			t.Errorf("传统模板 %s 应包含 base_url 字段", name)
		}
		if !strings.Contains(config, "url_pattern:") {
			t.Errorf("传统模板 %s 应包含 url_pattern 字段", name)
		}
		if !strings.Contains(config, "method:") {
			t.Errorf("传统模板 %s 应包含 method 字段", name)
		}
	}
}

func TestMultiModelTemplateNewStyle(t *testing.T) {
	config, err := GenerateFromTemplate("multi-model")
	if err != nil {
		t.Fatalf("生成 multi-model 模板失败: %v", err)
	}
	if !strings.Contains(config, "base_url:") {
		t.Error("multi-model 父通道应包含 base_url")
	}
	if !strings.Contains(config, "template:") {
		t.Error("multi-model 父通道应包含 template")
	}
	if !strings.Contains(config, "parent:") {
		t.Error("multi-model 应包含子通道 parent 引用")
	}
	// 子通道不应有 body
	lines := strings.Split(config, "\n")
	inChild := false
	for _, line := range lines {
		if strings.Contains(line, "parent:") {
			inChild = true
		}
		if inChild && strings.TrimSpace(line) == "body: |" {
			t.Error("multi-model 子通道不应包含 body（由模板 {{MODEL}} 自动处理）")
		}
	}
}

func TestGenerateFromTemplateInvalid(t *testing.T) {
	_, err := GenerateFromTemplate("invalid")
	if err == nil {
		t.Error("应该返回错误")
	}
}

func TestWriteConfig(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "test_config_*.yaml")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	config := "test: value\n"
	err = WriteConfig(config, tmpfile.Name(), false)
	if err != nil {
		t.Fatalf("写入配置失败: %v", err)
	}

	// 验证文件内容
	content, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}
	if string(content) != config {
		t.Errorf("文件内容不匹配: %s != %s", string(content), config)
	}
}

func TestWriteConfigAppend(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "test_config_append_*.yaml")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	// 写入基础配置（带 monitors）
	base := `interval: "1m"
slow_latency: "5s"
timeout: "10s"

monitors:
  - provider: "base"
    service: "cc"
    category: "commercial"
    sponsor: "team"
    channel: "standard"
    board: "hot"
    base_url: "https://example.com"
    method: "GET"
`
	err = WriteConfig(base, tmpfile.Name(), false)
	if err != nil {
		t.Fatalf("写入基础配置失败: %v", err)
	}

	// 追加：使用模板（包含顶层 key），append=true 应只追加 monitors 列表项
	tmpl, err := GenerateFromTemplate("anthropic")
	if err != nil {
		t.Fatalf("GenerateFromTemplate 失败: %v", err)
	}
	err = WriteConfig(tmpl, tmpfile.Name(), true)
	if err != nil {
		t.Fatalf("追加配置失败: %v", err)
	}

	// 验证文件内容
	content, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}

	text := string(content)
	// 不应重复写入顶层 interval（仅应出现一次）
	intervalCount := strings.Count(text, "interval:")
	if intervalCount != 1 {
		t.Errorf("append 模式不应重复顶层 key（interval）: 出现 %d 次，期望 1 次", intervalCount)
	}
	// 应包含原有 + 追加的 monitors item
	if !strings.Contains(text, "provider: \"base\"") {
		t.Errorf("缺少基础 monitors item: %q", text)
	}
	if !strings.Contains(text, "provider: \"anthropic\"") {
		t.Errorf("缺少追加的 monitors item: %q", text)
	}
}
