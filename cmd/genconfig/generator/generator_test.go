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
			"url":              "https://api.openai.com/v1/chat/completions",
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
    url: "https://example.com"
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
