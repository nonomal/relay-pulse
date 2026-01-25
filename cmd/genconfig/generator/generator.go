package generator

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// TemplateRegistry 模板注册表
type TemplateRegistry struct {
	templates map[string]string
}

// NewTemplateRegistry 创建新的模板注册表
func NewTemplateRegistry() *TemplateRegistry {
	return &TemplateRegistry{
		templates: map[string]string{
			"openai":      openaiTemplate,
			"anthropic":   anthropicTemplate,
			"gemini":      geminiTemplate,
			"cohere":      cohereTemplate,
			"mistral":     mistralTemplate,
			"custom":      customTemplate,
			"multi-model": multiModelTemplate,
		},
	}
}

// GetTemplate 获取模板
func (tr *TemplateRegistry) GetTemplate(name string) (string, error) {
	template, ok := tr.templates[name]
	if !ok {
		return "", fmt.Errorf("未知的模板: %s", name)
	}
	return template, nil
}

// ListTemplates 列出所有可用模板
func (tr *TemplateRegistry) ListTemplates() []string {
	var names []string
	for name := range tr.templates {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

var (
	validBoards     = map[string]bool{"hot": true, "secondary": true, "cold": true}
	validCategories = map[string]bool{"commercial": true, "public": true}
	validMethods    = map[string]bool{"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true}
	// genconfig 生成的 service 必须与前端已知 serviceType 对齐
	validServices = map[string]bool{"cc": true, "cx": true, "gm": true}
)

func isValidEnum(value string, allowed map[string]bool) bool {
	_, ok := allowed[value]
	return ok
}

// quoteYAML 将字符串安全地编码为 YAML 双引号标量（ASCII-only escaping）
func quoteYAML(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch ch {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			// 控制字符做最小化转义，避免破坏 YAML 结构
			if ch < 0x20 {
				b.WriteString(fmt.Sprintf(`\x%02x`, ch))
			} else {
				b.WriteByte(ch)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// GenerateConfig 生成 YAML 配置（带枚举校验 + YAML 转义）
func GenerateConfig(interval, slowLatency, timeout string, monitors []map[string]string) (string, error) {
	var sb strings.Builder

	// 全局配置
	sb.WriteString("# RelayPulse 配置文件\n")
	sb.WriteString("# 由 genconfig 工具生成\n\n")
	sb.WriteString("# 全局配置\n")
	sb.WriteString(fmt.Sprintf("interval: %s\n", quoteYAML(interval)))
	sb.WriteString(fmt.Sprintf("slow_latency: %s\n", quoteYAML(slowLatency)))
	sb.WriteString(fmt.Sprintf("timeout: %s\n", quoteYAML(timeout)))
	sb.WriteString("\n# 板块配置\n")
	sb.WriteString("boards:\n")
	sb.WriteString("  enabled: true\n")
	sb.WriteString("\n# 存储配置\n")
	sb.WriteString("storage:\n")
	sb.WriteString("  type: \"sqlite\"\n") // 固定值无需转义
	sb.WriteString("  sqlite:\n")
	sb.WriteString("    path: \"monitor.db\"\n") // 固定值无需转义

	// 监测项
	sb.WriteString("\n# 监测项列表\n")
	sb.WriteString("monitors:\n")

	for i, monitor := range monitors {
		provider := strings.TrimSpace(monitor["provider"])
		service := strings.ToLower(strings.TrimSpace(monitor["service"]))
		category := strings.ToLower(strings.TrimSpace(monitor["category"]))
		sponsor := strings.TrimSpace(monitor["sponsor"])
		channel := strings.TrimSpace(monitor["channel"])
		board := strings.ToLower(strings.TrimSpace(monitor["board"]))
		url := strings.TrimSpace(monitor["url"])
		method := strings.ToUpper(strings.TrimSpace(monitor["method"]))
		successContains := strings.TrimSpace(monitor["success_contains"])

		if provider == "" {
			return "", fmt.Errorf("monitor[%d]: provider 不能为空", i)
		}
		if service == "" {
			return "", fmt.Errorf("monitor[%d]: service 不能为空", i)
		}
		if !isValidEnum(service, validServices) {
			return "", fmt.Errorf("monitor[%d]: service '%s' 无效，必须是 cc/cx/gm", i, service)
		}
		if category == "" {
			return "", fmt.Errorf("monitor[%d]: category 不能为空", i)
		}
		if !isValidEnum(category, validCategories) {
			return "", fmt.Errorf("monitor[%d]: category '%s' 无效，必须是 commercial/public", i, category)
		}
		if sponsor == "" {
			return "", fmt.Errorf("monitor[%d]: sponsor 不能为空", i)
		}
		if channel == "" {
			return "", fmt.Errorf("monitor[%d]: channel 不能为空", i)
		}
		if board == "" {
			board = "hot"
		}
		if !isValidEnum(board, validBoards) {
			return "", fmt.Errorf("monitor[%d]: board '%s' 无效，必须是 hot/secondary/cold", i, board)
		}
		if url == "" {
			return "", fmt.Errorf("monitor[%d]: url 不能为空", i)
		}
		if method == "" {
			method = "POST"
		}
		if !isValidEnum(method, validMethods) {
			return "", fmt.Errorf("monitor[%d]: method '%s' 无效，必须是 GET/POST/PUT/DELETE/PATCH", i, method)
		}

		sb.WriteString("  - provider: " + quoteYAML(provider) + "\n")
		sb.WriteString("    service: " + quoteYAML(service) + "\n")
		sb.WriteString("    category: " + quoteYAML(category) + "\n")
		sb.WriteString("    sponsor: " + quoteYAML(sponsor) + "\n")
		sb.WriteString("    channel: " + quoteYAML(channel) + "\n")
		sb.WriteString("    board: " + quoteYAML(board) + "\n")
		sb.WriteString("    url: " + quoteYAML(url) + "\n")
		sb.WriteString("    method: " + quoteYAML(method) + "\n")

		if successContains != "" {
			sb.WriteString("    success_contains: " + quoteYAML(successContains) + "\n")
		}

		sb.WriteString("    headers:\n")
		sb.WriteString("      Authorization: \"Bearer {{API_KEY}}\"\n")
		sb.WriteString("      Content-Type: \"application/json\"\n")
		sb.WriteString("    body: |\n")
		sb.WriteString("      {\"test\": true}\n")
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// GenerateFromTemplate 从模板生成配置
func GenerateFromTemplate(templateName string) (string, error) {
	registry := NewTemplateRegistry()
	return registry.GetTemplate(templateName)
}

// WriteConfig 写入配置到文件
func WriteConfig(config, filepath string, append bool) error {
	if !append {
		return os.WriteFile(filepath, []byte(config), 0644)
	}

	// append=true：monitors-only 追加，避免重复顶层 key 破坏 YAML 结构
	items, err := extractMonitorsOnlyItems(config)
	if err != nil {
		return err
	}

	existingBytes, err := os.ReadFile(filepath)
	if err != nil {
		// 文件不存在：退化为直接写入完整配置（更符合"生成新配置"预期）
		if os.IsNotExist(err) {
			return os.WriteFile(filepath, []byte(config), 0644)
		}
		return err
	}
	existing := string(existingBytes)

	_, end, ok := findMonitorsBlockEndOffset(existing)
	if !ok {
		// 文件里没有 monitors：直接追加一个新段
		var b strings.Builder
		b.WriteString(existing)
		if !strings.HasSuffix(existing, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("\nmonitors:\n")
		b.WriteString(items)
		return os.WriteFile(filepath, []byte(b.String()), 0644)
	}

	// 将新 items 插入到现有 monitors 段末尾（在下一个顶层 key 前）
	var b strings.Builder
	b.Grow(len(existing) + len(items) + 8)
	b.WriteString(existing[:end])
	if end > 0 && existing[end-1] != '\n' {
		b.WriteString("\n")
	}
	// items 已保证以 "  - " 开头且以 "\n" 结尾
	b.WriteString(items)
	b.WriteString(existing[end:])

	// 覆盖写回（避免在未知布局下纯追加导致插入点错误）
	f, err := os.OpenFile(filepath, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.WriteString(f, b.String())
	return err
}

func extractMonitorsOnlyItems(yamlText string) (string, error) {
	// 从生成的 YAML 中提取 monitors 列表项（不含顶层 key）
	// 假设：单文档 YAML、顶层 monitors: 段、两空格缩进列表项
	lines := strings.Split(yamlText, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "monitors:" && strings.TrimLeft(line, " \t") == line {
			start = i + 1
			break
		}
	}
	if start == -1 {
		return "", fmt.Errorf("无法在生成内容中找到 monitors: 段")
	}
	var out []string
	for i := start; i < len(lines); i++ {
		line := lines[i]
		trim := strings.TrimSpace(line)
		// 遇到下一个顶层 key 则停止（简单规则：无缩进 + 以冒号结尾/包含冒号）
		if trim != "" && strings.TrimLeft(line, " \t") == line && !strings.HasPrefix(trim, "#") && strings.Contains(trim, ":") {
			break
		}
		out = append(out, line)
	}
	// 去掉前后空行
	for len(out) > 0 && strings.TrimSpace(out[0]) == "" {
		out = out[1:]
	}
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	joined := strings.Join(out, "\n")
	if strings.TrimSpace(joined) == "" {
		return "", fmt.Errorf("monitors 段为空，无法追加")
	}
	// 必须包含至少一个列表项
	if !strings.Contains(joined, "\n  - ") && !strings.HasPrefix(joined, "  - ") {
		return "", fmt.Errorf("monitors 段不包含列表项（期望以 '  - ' 开头）")
	}
	if !strings.HasSuffix(joined, "\n") {
		joined += "\n"
	}
	return joined, nil
}

func findMonitorsBlockEndOffset(existing string) (startOffset int, endOffset int, ok bool) {
	// 在现有 YAML 中定位 monitors 段的起止位置（byte offset）
	// 假设：单文档 YAML、顶层 monitors: 段、两空格缩进列表项
	// 基于行扫描计算 byte offset，避免复杂 YAML 解析依赖
	offset := 0
	lines := strings.SplitAfter(existing, "\n")
	startOffset = -1
	endOffset = -1
	inMonitors := false

	for _, raw := range lines {
		line := strings.TrimSuffix(raw, "\n")
		trim := strings.TrimSpace(line)
		isTopLevel := trim != "" && strings.TrimLeft(line, " \t") == line && !strings.HasPrefix(trim, "#")

		if !inMonitors {
			if trim == "monitors:" && strings.TrimLeft(line, " \t") == line {
				inMonitors = true
				startOffset = offset
			}
			offset += len(raw)
			continue
		}

		// monitors 段结束：遇到下一个顶层 key
		if isTopLevel && strings.Contains(trim, ":") {
			endOffset = offset
			return startOffset, endOffset, true
		}

		offset += len(raw)
	}

	if inMonitors {
		return startOffset, len(existing), true
	}
	return 0, 0, false
}

const openaiTemplate = `# OpenAI 监测配置
interval: "1m"
slow_latency: "5s"
timeout: "10s"

boards:
  enabled: true

storage:
  type: "sqlite"
  sqlite:
    path: "monitor.db"

monitors:
  - provider: "openai"
    service: "cx"
    category: "commercial"
    sponsor: "团队"
    channel: "standard"
    board: "hot"
    url: "https://api.openai.com/v1/chat/completions"
    method: "POST"
    success_contains: "choices"
    headers:
      Authorization: "Bearer {{API_KEY}}"
      Content-Type: "application/json"
    body: |
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "hi"}],
        "max_tokens": 1
      }
`

const anthropicTemplate = `# Anthropic 监测配置
interval: "1m"
slow_latency: "5s"
timeout: "10s"

boards:
  enabled: true

storage:
  type: "sqlite"
  sqlite:
    path: "monitor.db"

monitors:
  - provider: "anthropic"
    service: "cc"
    category: "commercial"
    sponsor: "团队"
    channel: "standard"
    board: "hot"
    url: "https://api.anthropic.com/v1/messages"
    method: "POST"
    success_contains: "content"
    headers:
      x-api-key: "{{API_KEY}}"
      anthropic-version: "2023-06-01"
      Content-Type: "application/json"
    body: |
      {
        "model": "claude-3-opus-20240229",
        "messages": [{"role": "user", "content": "hi"}],
        "max_tokens": 1
      }
`

const customTemplate = `# 自定义 API 监测配置
interval: "1m"
slow_latency: "5s"
timeout: "10s"

boards:
  enabled: true

storage:
  type: "sqlite"
  sqlite:
    path: "monitor.db"

monitors:
  - provider: "custom-provider"
    service: "cx"
    category: "commercial"
    sponsor: "团队"
    channel: "standard"
    board: "hot"
    url: "https://api.example.com/health"
    method: "GET"
    success_contains: "ok"
    headers:
      Authorization: "Bearer {{API_KEY}}"
`

const geminiTemplate = `# Google Gemini 监测配置
interval: "1m"
slow_latency: "5s"
timeout: "10s"

boards:
  enabled: true

storage:
  type: "sqlite"
  sqlite:
    path: "monitor.db"

monitors:
  - provider: "google"
    service: "gm"
    category: "commercial"
    sponsor: "团队"
    channel: "standard"
    board: "hot"
    url: "https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:generateContent"
    method: "POST"
    success_contains: "candidates"
    headers:
      Content-Type: "application/json"
    body: |
      {
        "contents": [{"parts": [{"text": "hi"}]}]
      }
`

const cohereTemplate = `# Cohere 监测配置
interval: "1m"
slow_latency: "5s"
timeout: "10s"

boards:
  enabled: true

storage:
  type: "sqlite"
  sqlite:
    path: "monitor.db"

monitors:
  - provider: "cohere"
    service: "cx"
    category: "commercial"
    sponsor: "团队"
    channel: "standard"
    board: "hot"
    url: "https://api.cohere.ai/v1/generate"
    method: "POST"
    success_contains: "generations"
    headers:
      Authorization: "Bearer {{API_KEY}}"
      Content-Type: "application/json"
    body: |
      {
        "prompt": "hi",
        "max_tokens": 1
      }
`

const mistralTemplate = `# Mistral AI 监测配置
interval: "1m"
slow_latency: "5s"
timeout: "10s"

boards:
  enabled: true

storage:
  type: "sqlite"
  sqlite:
    path: "monitor.db"

monitors:
  - provider: "mistral"
    service: "cx"
    category: "commercial"
    sponsor: "团队"
    channel: "standard"
    board: "hot"
    url: "https://api.mistral.ai/v1/chat/completions"
    method: "POST"
    success_contains: "choices"
    headers:
      Authorization: "Bearer {{API_KEY}}"
      Content-Type: "application/json"
    body: |
      {
        "model": "mistral-small",
        "messages": [{"role": "user", "content": "hi"}],
        "max_tokens": 1
      }
`

const multiModelTemplate = `# 多模型监测配置（父子关系示例）
interval: "1m"
slow_latency: "5s"
timeout: "10s"

boards:
  enabled: true

storage:
  type: "sqlite"
  sqlite:
    path: "monitor.db"

monitors:
  # 父通道：定义公共配置
  - provider: "88code"
    service: "cc"
    channel: "vip"
    model: "claude-sonnet-4-20250514"
    category: "commercial"
    sponsor: "团队"
    sponsor_level: "advanced"
    board: "hot"
    url: "https://api.88code.com/v1/chat/completions"
    method: "POST"
    success_contains: "content"
    headers:
      Authorization: "Bearer {{API_KEY}}"
      Content-Type: "application/json"
    body: |
      {
        "model": "{{MODEL}}",
        "messages": [{"role": "user", "content": "hi"}],
        "max_tokens": 1
      }

  # 子通道：继承父配置，只需指定 model 和 parent
  - model: "claude-opus-4-20250514"
    parent: "88code/cc/vip"
    body: |
      {
        "model": "claude-opus-4-20250514",
        "messages": [{"role": "user", "content": "hi"}],
        "max_tokens": 1
      }

  - model: "claude-haiku-4-20250514"
    parent: "88code/cc/vip"
    body: |
      {
        "model": "claude-haiku-4-20250514",
        "messages": [{"role": "user", "content": "hi"}],
        "max_tokens": 1
      }
`
