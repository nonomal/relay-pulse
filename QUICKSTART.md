# 快速部署指南 ⚡

> **一键启动 LLM 服务可用性监测系统**

## 5 分钟快速部署

### 前置要求

- Docker 20.10+
- Docker Compose v2.0+

### 部署步骤

#### 1. 下载配置文件

```bash
# 创建项目目录
mkdir relay-pulse && cd relay-pulse

# 下载 docker-compose.yaml
curl -O https://raw.githubusercontent.com/prehisle/relay-pulse/main/docker-compose.yaml

# 下载配置模板
curl -O https://raw.githubusercontent.com/prehisle/relay-pulse/main/config.yaml.example
```

#### 2. 准备配置文件

```bash
# 创建配置目录并复制配置模板
mkdir -p config && cp config.yaml.example config/config.yaml

# 编辑配置（填入你的 API Key 和服务端点）
vim config/config.yaml
```

**最小配置示例**：

```yaml
interval: "1m"
slow_latency: "5s"

monitors:
  - provider: "openai"
    service: "cx"
    category: "commercial"           # 必填：商业站(commercial) 或 公益站(public)
    sponsor: "团队自有"              # 必填：提供 API Key 的赞助者
    base_url: "https://api.openai.com"
    template: "cx-codex-base"        # 引用 templates/cx-codex-base.json 模板
    api_key: "sk-your-api-key-here"
    model: "gpt-4.1"
```

#### 3. 一键启动

```bash
docker compose up -d
```

#### 4. 访问服务

- **Web 界面**: http://localhost:8080
- **API 端点**: http://localhost:8080/api/status
- **健康检查**: http://localhost:8080/health

完成！🎉

---

## 常用命令

```bash
# 查看运行状态
docker compose ps

# 查看实时日志
docker compose logs -f monitor

# 停止服务
docker compose down

# 重启服务
docker compose restart

# 更新到最新版本
docker compose pull
docker compose up -d
```

---

## 高级配置

### 使用环境变量（推荐生产环境）

**优点**：API Key 不写在配置文件中，更安全

#### 1. 创建环境变量文件

```bash
cat > .env <<'EOF'
MONITOR_OPENAI_GPT4_API_KEY=sk-your-real-api-key
MONITOR_ANTHROPIC_CLAUDE_API_KEY=sk-ant-your-key
EOF
```

#### 2. 配置文件中使用占位符

```yaml
monitors:
  - provider: "openai"
    service: "cx"
    category: "commercial"
    sponsor: "团队自有"
    base_url: "https://api.openai.com"
    template: "cx-codex-base"
    model: "gpt-4.1"
    # api_key 留空或不填，将从环境变量读取
```

#### 3. 启动时加载环境变量

```bash
docker compose --env-file .env up -d
```

**环境变量命名规则**：

支持两级和三级格式（三级含 channel，优先级更高）：

```
MONITOR_<PROVIDER>_<SERVICE>_<CHANNEL>_API_KEY   # 三级（channel 级，优先）
MONITOR_<PROVIDER>_<SERVICE>_API_KEY              # 二级（service 级）
```

- `<PROVIDER>`: 配置中的 provider 字段（大写，`-` 替换为 `_`）
- `<SERVICE>`: 配置中的 service 字段（大写，`-` 替换为 `_`）
- `<CHANNEL>`: 配置中的 channel 字段（大写，`-` 替换为 `_`）

**优先级**：`env_var_name` 自定义变量名 > channel 级 > service 级

**示例**：

| 配置 | 环境变量名 |
|------|-----------|
| `provider: "88code"`, `service: "cc"` | `MONITOR_88CODE_CC_API_KEY` |
| `provider: "88code"`, `service: "cc"`, `channel: "vip"` | `MONITOR_88CODE_CC_VIP_API_KEY` |
| `provider: "openai"`, `service: "gpt-4"` | `MONITOR_OPENAI_GPT4_API_KEY` |
| `provider: "anthropic"`, `service: "claude-3"` | `MONITOR_ANTHROPIC_CLAUDE3_API_KEY` |

---

## 数据持久化

### SQLite 数据库

数据自动保存在 Docker 命名卷 `relay-pulse-data` 中，重启容器不会丢失。

**查看数据卷**：

```bash
docker volume ls | grep relay-pulse
```

**备份数据库**：

```bash
docker compose exec monitor sh -c 'cp /app/monitor.db /app/data/monitor.db.backup'
docker cp relaypulse-monitor:/app/data/monitor.db.backup ./
```

**恢复数据库**：

```bash
docker cp ./monitor.db.backup relaypulse-monitor:/app/monitor.db
docker compose restart
```

---

## 配置热更新

修改配置文件后，**无需重启容器**，服务会自动检测并重载配置：

```bash
# 1. 编辑配置
vim config/config.yaml

# 2. 观察日志，等待配置重载提示
docker compose logs -f monitor

# 输出示例：
# [Config] 检测到配置文件变更，正在重载...
# [Config] 热更新成功！已加载 5 个监测任务
```

---

## 监测多个服务示例

```yaml
interval: "1m"
slow_latency: "5s"

monitors:
  # OpenAI GPT-4.1
  - provider: "openai"
    service: "cx"
    category: "commercial"
    sponsor: "团队自有"
    base_url: "https://api.openai.com"
    template: "cx-codex-base"
    api_key: "sk-openai-key"
    model: "gpt-4.1"

  # Anthropic Claude
  - provider: "anthropic"
    service: "cc"
    category: "commercial"
    sponsor: "团队自有"
    base_url: "https://api.anthropic.com"
    template: "cc-haiku-base"
    api_key: "sk-ant-key"
    model: "claude-haiku-4-20250514"

  # Google Gemini
  - provider: "google"
    service: "gm"
    category: "commercial"
    sponsor: "团队自有"
    base_url: "https://generativelanguage.googleapis.com"
    template: "gm-base"
    api_key: "your-google-api-key"
    model: "gemini-2.0-flash"
```

---

## 自定义端口

默认端口是 `8080`，如需修改：

```bash
# 编辑 docker-compose.yaml
vim docker-compose.yaml

# 修改 ports 部分
ports:
  - "3000:8080"  # 本地 3000 端口映射到容器 8080
```

---

## 故障排查

### 容器无法启动

```bash
# 查看详细日志
docker compose logs monitor

# 检查配置文件语法
docker compose config
```

### 配置文件找不到

确保 `config/config.yaml` 在 `docker-compose.yaml` 同目录下：

```bash
ls -la config/config.yaml docker-compose.yaml
```

### 数据库权限问题

```bash
# 检查容器内文件权限
docker compose exec monitor ls -la /app/
```

### 服务无法访问

```bash
# 检查容器状态
docker compose ps

# 检查端口占用
lsof -i :8080

# 测试健康检查
curl http://localhost:8080/health
```

---

## 卸载

```bash
# 停止并删除容器
docker compose down

# 同时删除数据卷（⚠️ 会丢失所有历史数据）
docker compose down -v

# 删除镜像
docker rmi ghcr.io/prehisle/relay-pulse:latest
```

---

## 生产部署建议

### 1. 使用 HTTPS（Cloudflare CDN）

生产环境推荐使用 Cloudflare 提供 HTTPS、CDN 和 DDoS 防护：

**步骤**：
1. 在 Cloudflare 添加 A 记录指向服务器 IP，开启代理（橙色云朵）
2. SSL/TLS 模式设置为 "灵活"（Flexible）
3. 配置页面规则缓存静态资源（`/assets/*`）
4. 配置服务器防火墙只允许 Cloudflare IP 访问 80 端口
5. 修改 `docker-compose.yaml` 端口映射为 `80:8080`

详细配置可参考 `archive/docs/deployment.md` 中的 "Cloudflare 配置" 章节（历史文档，仅供参考，以当前 README/配置手册为准）。

### 2. 资源限制

编辑 `docker-compose.yaml`，取消注释资源限制：

```yaml
deploy:
  resources:
    limits:
      cpus: '1'
      memory: 512M
    reservations:
      cpus: '0.5'
      memory: 256M
```

### 3. 日志轮转

```yaml
logging:
  driver: "json-file"
  options:
    max-size: "10m"
    max-file: "3"
```

### 4. 定期备份数据库

```bash
# 添加到 crontab
0 2 * * * docker compose -f /path/to/docker-compose.yaml exec monitor sh -c 'cp /app/monitor.db /app/data/backup-$(date +\%Y\%m\%d).db'
```

---

## 新功能：备板系统 & 配置生成器

### 备板系统（Secondary Board）

RelayPulse 现在支持三层板块系统，用于管理不同生命周期的监测通道：

| 板块 | 说明 | 探测 | 适用场景 |
|------|------|------|----------|
| **主板 (hot)** | 活跃稳定的通道 | ✅ | 默认板块，稳定运行的服务 |
| **备板 (secondary)** | 观察期通道 | ✅ | 新上线通道、短期不稳定待观察 |
| **冷板 (cold)** | 归档通道 | ❌ | 长期不可用、已下线的历史通道 |

**启用备板功能**：

编辑 `config.yaml`，添加或修改：

```yaml
boards:
  enabled: true  # 启用板块功能
```

**配置示例**：

```yaml
monitors:
  # 主板：稳定运行的服务
  - provider: "openai"
    service: "gpt-4"
    board: "hot"
    # ...

  # 备板：新上线或观察期通道
  - provider: "newprovider"
    service: "api"
    board: "secondary"
    # ...

  # 冷板：已下线的历史通道
  - provider: "oldprovider"
    service: "api"
    board: "cold"
    cold_reason: "该渠道长期不稳定，已归档"
    # ...
```

**前端交互**：
- 控制栏显示板块下拉菜单（带图标）
- 支持 `?board=hot|secondary|cold|all` URL 参数
- 切换到冷板时显示提示信息

### 配置生成器（genconfig）

快速生成监测配置的 CLI 工具，支持 7 个预定义模板。

**列出所有模板**：

```bash
go run ./cmd/genconfig -list
```

**快速生成配置**：

```bash
# 生成 OpenAI 配置
go run ./cmd/genconfig -mode template -template openai -output config.yaml

# 生成 Anthropic 配置
go run ./cmd/genconfig -mode template -template anthropic -output config.yaml

# 生成 Gemini 配置
go run ./cmd/genconfig -mode template -template gemini -output config.yaml

# 生成多模型配置（父子关系示例）
go run ./cmd/genconfig -mode template -template multi-model -output config.yaml
```

**追加新的监测项**：

```bash
# 追加 Cohere 配置
go run ./cmd/genconfig -mode template -template cohere -output config.yaml -append
```

**交互式配置**：

```bash
go run ./cmd/genconfig -mode interactive
```

按照提示输入全局配置和监测项信息。

**可用模板**：
- `openai` - OpenAI GPT-4
- `anthropic` - Anthropic Claude
- `gemini` - Google Gemini
- `cohere` - Cohere
- `mistral` - Mistral AI
- `multi-model` - 多模型配置（父子关系）
- `custom` - 自定义 API

详细文档：[cmd/genconfig/README.md](cmd/genconfig/README.md)

---

## 自助测试（SelfTest）

RelayPulse 内置自助测试功能，允许用户在 Web 界面上临时测试 API 端点的连通性。

**启用方式**：在 `config.yaml` 中添加：

```yaml
selftest:
  enabled: true
```

启用后，访问 Web 界面的 `/selftest` 页面即可使用。详细配置参见 [配置手册](docs/user/config.md)。

---

## 更多文档

- **项目入口**: [README.md](README.md)
- **配置手册**: [docs/user/config.md](docs/user/config.md)
- **通知推送**: [notifier/README.md](notifier/README.md)（Telegram/QQ Bot）
- **贡献指南**: [CONTRIBUTING.md](CONTRIBUTING.md)

---

## 支持

- **GitHub Issues**: https://github.com/prehisle/relay-pulse/issues
- **文档**: https://github.com/prehisle/relay-pulse

**祝监测愉快！** 🚀
