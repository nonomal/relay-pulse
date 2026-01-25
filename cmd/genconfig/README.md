# 配置生成器 (genconfig)

`genconfig` 是 RelayPulse 的配置生成工具，用于快速生成监测配置文件。

## 快速开始

### 列出所有可用模板

```bash
go run ./cmd/genconfig -list
```

### 模板快速生成

使用预定义模板快速生成配置：

```bash
# 生成 OpenAI 配置
go run ./cmd/genconfig -mode template -template openai

# 生成 Anthropic 配置
go run ./cmd/genconfig -mode template -template anthropic

# 生成 Google Gemini 配置
go run ./cmd/genconfig -mode template -template gemini

# 生成多模型配置（父子关系示例）
go run ./cmd/genconfig -mode template -template multi-model
```

### 保存到文件

```bash
# 保存到文件
go run ./cmd/genconfig -mode template -template openai -output config.yaml

# 追加到现有配置
go run ./cmd/genconfig -mode template -template anthropic -output config.yaml -append
```

### 交互式模式

交互式地创建配置（逐步输入各项参数）：

```bash
go run ./cmd/genconfig -mode interactive
```

交互式模式会引导你输入：
- 全局配置（巡检间隔、慢请求阈值、超时时间）
- 监测项配置（服务商、服务类型、通道、URL 等）

## 可用模板

### openai
生成 OpenAI GPT-4 监测配置，包含：
- 服务商：openai
- 服务类型：gpt-4
- 端点：https://api.openai.com/v1/chat/completions
- 关键字验证：choices

### anthropic
生成 Anthropic Claude 监测配置，包含：
- 服务商：anthropic
- 服务类型：claude
- 端点：https://api.anthropic.com/v1/messages
- 关键字验证：content

### gemini
生成 Google Gemini 监测配置，包含：
- 服务商：google
- 服务类型：gemini
- 端点：https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:generateContent
- 关键字验证：candidates

### cohere
生成 Cohere 监测配置，包含：
- 服务商：cohere
- 服务类型：generate
- 端点：https://api.cohere.ai/v1/generate
- 关键字验证：generations

### mistral
生成 Mistral AI 监测配置，包含：
- 服务商：mistral
- 服务类型：chat
- 端点：https://api.mistral.ai/v1/chat/completions
- 关键字验证：choices

### multi-model
生成多模型监测配置（父子关系示例），包含：
- 父通道：定义公共配置
- 子通道：继承父配置，只需指定 model 和 parent
- 适用于同一服务商的多个模型监测

### custom
生成自定义 API 监测配置模板，可根据需要修改。

## 命令行参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `-mode` | 生成模式：interactive 或 template | interactive |
| `-template` | 模板名称（仅在 mode=template 时使用） | - |
| `-output` | 输出文件路径（不指定则输出到 stdout） | - |
| `-append` | 追加到现有文件（仅在指定 output 时生效） | false |
| `-list` | 列出所有可用模板 | false |

## 使用示例

### 示例 1：快速生成 OpenAI 配置

```bash
go run ./cmd/genconfig -mode template -template openai -output config.yaml
```

### 示例 2：交互式创建多个监测项

```bash
go run ./cmd/genconfig -mode interactive
```

按照提示输入：
1. 全局配置（使用默认值或自定义）
2. 第一个监测项信息
3. 选择是否继续添加更多监测项

### 示例 3：追加新的监测项配置

```bash
# 先生成 OpenAI 配置
go run ./cmd/genconfig -mode template -template openai -output config.yaml

# 再追加 Anthropic 配置
go run ./cmd/genconfig -mode template -template anthropic -output config.yaml -append
```

### 示例 4：生成多模型配置

```bash
# 生成包含父子关系的多模型配置
go run ./cmd/genconfig -mode template -template multi-model -output config.yaml
```

## 生成的配置说明

生成的配置文件包含以下部分：

### 全局配置
```yaml
interval: "1m"           # 巡检间隔
slow_latency: "5s"       # 慢请求阈值
timeout: "10s"           # 请求超时时间
```

### 板块配置
```yaml
boards:
  enabled: true          # 启用板块功能
```

### 存储配置
```yaml
storage:
  type: "sqlite"
  sqlite:
    path: "monitor.db"
```

### 监测项
```yaml
monitors:
  - provider: "..."      # 服务商标识
    service: "..."       # 服务类型
    category: "..."      # 分类
    sponsor: "..."       # 赞助者
    channel: "..."       # 业务通道
    board: "..."         # 板块
    url: "..."           # 健康检查端点
    method: "..."        # HTTP 方法
    success_contains: "..." # 响应体关键字
    headers: {...}       # 请求头
    body: {...}          # 请求体
```

## 后续步骤

1. **配置 API Key**：
   ```bash
   export MONITOR_OPENAI_GPT4_API_KEY="sk-your-key"
   ```

2. **验证配置**：
   ```bash
   go run ./cmd/verify/main.go -provider openai -service gpt-4 -v
   ```

3. **启动监测**：
   ```bash
   go run ./cmd/server/main.go config.yaml
   ```

## 常见问题

### Q: 如何修改生成的配置？

A: 生成的配置文件是标准 YAML 格式，可以用任何文本编辑器修改。参考 [配置手册](../../docs/user/config.md) 了解所有配置选项。

### Q: 如何添加更多监测项？

A: 可以：
1. 使用 `-append` 参数追加新的模板配置
2. 手动编辑 YAML 文件添加新的 monitor 项
3. 再次运行 genconfig 并合并结果

### Q: 交互式模式支持哪些字段？

A: 当前支持以下字段：
- provider, service, category, sponsor
- channel, board, url, method
- success_contains

其他高级字段（如 proxy, interval 覆盖等）需要手动编辑配置文件。

### Q: 如何使用自定义请求体？

A: 生成的配置中 `body` 字段是简单的 JSON。对于复杂的请求体，可以：
1. 手动编辑 YAML 文件中的 body 字段
2. 使用 `!include` 引用外部文件：`body: "!include data/request.json"`

### Q: 如何使用多模型配置？

A: 多模型配置使用父子关系：
1. 定义一个父通道，包含公共配置
2. 定义多个子通道，只需指定 `model` 和 `parent` 字段
3. 子通道会自动继承父通道的所有配置

参考 `multi-model` 模板了解详细用法。

## 开发

### 添加新模板

在 `cmd/genconfig/generator/generator.go` 中添加新的模板常量，然后在 `NewTemplateRegistry()` 中注册：

```go
const myTemplate = `...`

func NewTemplateRegistry() *TemplateRegistry {
    return &TemplateRegistry{
        templates: map[string]string{
            "mytemplate": myTemplate,
        },
    }
}
```

### 扩展交互式模式

修改 `cmd/genconfig/main.go` 中的 `runInteractiveMode()` 函数，添加更多字段收集逻辑。

### 运行测试

```bash
go test ./cmd/genconfig/generator -v
```
