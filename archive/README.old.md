# LLM Service Monitor - 企业级监控服务

> **⚠️ 历史文档 / Deprecated** — Last verified: 2026-04-17
> 本文内容已过期，保留作为背景参考。现行文档请见仓库根 `README.md` 与 `docs/user/`。

生产级 LLM 服务可用性监控系统，支持热更新、真实历史数据持久化。

## 核心特性

✅ **配置驱动** - YAML 配置，支持环境变量覆盖
✅ **热更新** - 修改配置无需重启服务
✅ **多后端存储** - 支持 SQLite 和 PostgreSQL，灵活切换
✅ **云原生** - Kubernetes 友好，支持水平扩展
✅ **并发安全** - HTTP 客户端池复用，防重复触发
✅ **生产级质量** - 完整错误处理，优雅关闭

## 项目结构

```
monitor/
├── cmd/server/main.go          # 入口
├── internal/
│   ├── config/                 # 配置管理（验证、热更新、环境变量）
│   ├── storage/                # 存储层（SQLite/PostgreSQL 抽象）
│   │   ├── storage.go          # 存储接口定义
│   │   ├── factory.go          # 工厂模式
│   │   ├── sqlite.go           # SQLite 实现
│   │   └── postgres.go         # PostgreSQL 实现
│   ├── monitor/                # 监控引擎（HTTP 客户端池、探测）
│   ├── scheduler/              # 调度器（防重复、并发控制）
│   └── api/                    # API 层（gin、历史查询）
├── config.yaml                 # 配置文件
├── docker-compose.yaml         # Docker Compose（支持双后端）
└── Dockerfile                  # 多阶段构建
```

## 快速部署

### 🚀 Docker 一键启动（推荐）

```bash
# 1. 下载配置
curl -O https://raw.githubusercontent.com/prehisle/relay-pulse/main/docker-compose.yaml
curl -O https://raw.githubusercontent.com/prehisle/relay-pulse/main/config.yaml.example

# 2. 下载 templates 目录（包含 JSON 模板文件）
mkdir -p templates
curl -o templates/cc_base.json https://raw.githubusercontent.com/prehisle/relay-pulse/main/templates/cc_base.json
curl -o templates/cx_base.json https://raw.githubusercontent.com/prehisle/relay-pulse/main/templates/cx_base.json

# 3. 准备配置
cp config.yaml.example config.yaml
vim config.yaml  # 填入你的 API Key

# 4. 取消注释 templates 目录挂载（编辑 docker-compose.yaml）
# 找到这一行并取消注释:
#   - ./templates:/app/templates:ro

# 5. 一键启动
docker compose up -d

# 6. 访问服务
open http://localhost:8080
```

**详细教程**: 📖 [QUICKSTART.md](QUICKSTART.md)

---

### 📦 本地开发

#### 1. 安装依赖

```bash
go mod tidy
cd frontend && npm install
```

#### 2. 配置服务

复制示例配置：

```bash
cp config.yaml.example config.yaml
```

编辑 `config.yaml`，填入真实的 API Key 和必填字段：

```yaml
monitors:
  - provider: "88code"
    service: "cc"
    category: "commercial"       # 必填：commercial（推广站）或 public（公益站）
    sponsor: "团队自有"          # 必填：提供 API Key 的赞助者
    url: "https://api.88code.com/v1/chat/completions"
    method: "POST"
    api_key: "sk-your-real-key"  # 修改这里
    headers:
      Authorization: "Bearer {{API_KEY}}"
      Content-Type: "application/json"
    body: |
      {
        "model": "claude-3-opus",
        "messages": [{"role": "user", "content": "hi"}],
        "max_tokens": 1
      }
```

**⚠️ 配置迁移提示**：
- `category` 和 `sponsor` 为**必填字段**，缺失将导致启动失败
- 如果升级旧配置，请为每个 monitor 添加这两个字段
- 参考 `config.yaml.example` 查看完整示例

如果请求体较大，可将 JSON 放在 `templates/` 目录并在 `body` 中引用：

```yaml
body: "!include templates/cx_base.json"  # 路径必须位于 templates/ 下
```

### 3. 配置巡检间隔

可以在根级配置巡检频率（默认 1 分钟一次）：

```yaml
interval: "1m"  # 支持 Go duration 格式，例如 "30s"、"1m"、"5m"
```

修改保存后，调度器会在下一轮自动使用新的间隔。

### 4. 运行服务

```bash
go run cmd/server/main.go
```

### 5. 测试 API

```bash
# 获取所有监控状态（24小时）
curl "http://localhost:8080/api/status"

# 获取 7 天历史
curl "http://localhost:8080/api/status?period=7d"

# 过滤特定 provider
curl "http://localhost:8080/api/status?provider=88code"

# 健康检查
curl "http://localhost:8080/health"
```

## 环境变量支持

可通过环境变量覆盖 API Key（更安全）：

```bash
export MONITOR_88CODE_CC_API_KEY="sk-real-key"
export MONITOR_DUCKCODING_CC_API_KEY="sk-duck-key"

go run cmd/server/main.go
```

命名规则：`MONITOR_<PROVIDER>_<SERVICE>_API_KEY`（大写，`-` 替换为 `_`）

## 数据库配置

系统支持 **SQLite** 和 **PostgreSQL** 两种存储后端，通过配置文件或环境变量灵活切换。

### SQLite（默认，单机部署）

适用于单机部署、开发环境和小规模监控。

```yaml
# config.yaml
storage:
  type: "sqlite"
  sqlite:
    path: "monitor.db"
```

**优点**：
- 零配置，开箱即用
- 无需额外服务依赖
- 适合快速启动和测试

**限制**：
- 不支持多副本部署
- Kubernetes 环境需要 StatefulSet + PV

### PostgreSQL（K8s/生产环境推荐）

适用于 Kubernetes 多副本部署、高可用场景。

```yaml
# config.yaml
storage:
  type: "postgres"
  postgres:
    host: "postgres-service"
    port: 5432
    user: "monitor"
    password: "secret"  # 建议使用环境变量
    database: "llm_monitor"
    sslmode: "disable"  # 生产环境建议 "require"
    max_open_conns: 25
    max_idle_conns: 5
    conn_max_lifetime: "1h"
```

**通过环境变量配置**（推荐）：

```bash
export MONITOR_STORAGE_TYPE=postgres
export MONITOR_POSTGRES_HOST=postgres-service
export MONITOR_POSTGRES_USER=monitor
export MONITOR_POSTGRES_PASSWORD=your_secure_password
export MONITOR_POSTGRES_DATABASE=llm_monitor

./monitor
```

**优点**：
- ✅ 支持水平扩展（多副本）
- ✅ 高可用和主从复制
- ✅ 完整的 ACID 事务
- ✅ 成熟的备份恢复方案
- ✅ 云原生数据库支持（AWS RDS、Google Cloud SQL 等）

**初始化 PostgreSQL**：

```sql
CREATE DATABASE llm_monitor;
CREATE USER monitor WITH PASSWORD 'your_password';
GRANT ALL PRIVILEGES ON DATABASE llm_monitor TO monitor;
```

系统会在首次启动时自动创建表结构和索引。

## 热更新

修改 `config.yaml` 后保存，服务会自动重载：

```bash
# 修改配置
vim config.yaml

# 观察日志
# [Config] 检测到配置文件变更，正在重载...
# [Config] 热更新成功！已加载 3 个监控任务
# [Scheduler] 配置已更新，下次巡检将使用新配置
```

如果配置错误，服务会保持旧配置并输出错误日志。

## API 响应格式

```json
{
  "meta": {
    "period": "24h",
    "count": 3
  },
  "data": [
    {
      "provider": "88code",
      "service": "cc",
      "category": "commercial",
      "sponsor": "团队自有",
      "channel": "vip-channel",
      "current_status": {
        "status": 1,
        "latency": 234,
        "timestamp": 1735559123
      },
      "timeline": [
        {
          "time": "14:30",
          "status": 1,
          "latency": 234
        }
      ]
    }
  ]
}
```

**字段说明**：
- `category`: 分类，`commercial`（推广站）或 `public`（公益站）
- `sponsor`: 赞助者名称
- `channel`: 业务通道标识（可选）

**Status 说明**：
- `0` = 🔴 红色（服务不可用）
- `1` = 🟢 绿色（正常）
- `2` = 🟡 黄色（延迟高或临时错误）

## 高级特性

### 占位符替换

`{{API_KEY}}` 在 **headers 和 body** 中都会被替换：

```yaml
headers:
  Authorization: "Bearer {{API_KEY}}"
body: |
  {"api_key": "{{API_KEY}}", "model": "gpt-4"}
```

### 配置验证

服务启动时会验证：
- 必填字段（provider, service, url, method）
- Method 枚举（GET/POST/PUT/DELETE/PATCH）
- Provider+Service 唯一性

### 数据清理

自动清理 30 天前的历史数据（每天执行一次）。

### 优雅关闭

`Ctrl+C` 时会：
1. 停止调度器
2. 完成进行中的探测
3. 关闭 HTTP 服务器
4. 关闭数据库连接

## 生产部署建议

### 快速预览

- **域名**: `relaypulse.top`
- **仓库**: https://github.com/prehisle/relay-pulse.git
- **架构**: Cloudflare CDN/WAF → Go 服务（监听 8080，embed 静态资源 + API）→ SQLite/PostgreSQL

> 📖 **完整部署指南**：请查看 [docs/deployment.md](docs/deployment.md) 获取详细的生产环境部署步骤、安全加固、监控维护等内容。

### 部署前置准备

1. **配置文件**：
   ```bash
   cp config.yaml.example config.production.yaml
   cp deploy/relaypulse.env.example deploy/relaypulse.env
   ```

2. **前端环境变量**（`frontend/.env.production`）：
   ```bash
   VITE_API_BASE_URL=https://relaypulse.top
   VITE_USE_MOCK_DATA=false
   ```

3. **数据持久化目录**：
   ```bash
   mkdir -p monitor
   ```

### Docker 部署（推荐）

#### 方式一：使用 GitHub Container Registry 镜像

```bash
# 拉取最新镜像
docker pull ghcr.io/prehisle/relay-pulse:latest

# 使用 Docker Compose 启动（推荐）
docker compose --env-file deploy/relaypulse.env up -d monitor

# 或手动启动
docker run -d \
  --name relaypulse-monitor \
  -p 8080:8080 \
  -v $(pwd)/config.production.yaml:/config/config.yaml:ro \
  -v $(pwd)/monitor:/app/monitor-data \
  --env-file deploy/relaypulse.env \
  ghcr.io/prehisle/relay-pulse:latest
```

#### 方式二：本地构建镜像

```bash
# 构建镜像（多架构支持）
docker build -t relay-pulse:latest .

# 启动容器
docker run -d \
  --name relaypulse-monitor \
  -p 8080:8080 \
  -v $(pwd)/config.production.yaml:/config/config.yaml:ro \
  -v $(pwd)/monitor:/app/monitor-data \
  --env-file deploy/relaypulse.env \
  relay-pulse:latest
```

#### Docker Compose 部署

项目根目录已包含 `docker-compose.yaml`：

**常用操作**：
```bash
# SQLite 模式（默认）
docker compose --env-file deploy/relaypulse.env up -d monitor

# PostgreSQL 模式（需先取消注释 postgres 和 monitor-pg 配置）
docker compose --env-file deploy/relaypulse.env up -d postgres monitor-pg

# 查看日志
docker compose logs -f monitor        # SQLite 模式
docker compose logs -f monitor-pg     # PostgreSQL 模式

# 重启服务（配置更新后）
docker compose restart monitor

# 停止服务
docker compose down
```

#### PostgreSQL 模式部署

适用于 Kubernetes 或多副本部署场景：

```bash
# 1. 在 deploy/relaypulse.env 中设置:
#    MONITOR_STORAGE_TYPE=postgres
#    MONITOR_POSTGRES_HOST=postgres
#    MONITOR_POSTGRES_USER=monitor
#    MONITOR_POSTGRES_PASSWORD=your_secure_password
#    MONITOR_POSTGRES_DATABASE=llm_monitor

# 2. 启动 PostgreSQL 和监控服务
docker compose --env-file deploy/relaypulse.env up -d postgres monitor-pg

# 3. 验证连接
docker compose logs -f monitor-pg
# 输出应包含: ✅ postgres 存储已就绪

# 4. 查看数据库
docker compose exec postgres psql -U monitor -d llm_monitor -c "SELECT COUNT(*) FROM probe_history;"
```

### Systemd 服务

```ini
[Unit]
Description=Relay Pulse Monitor
After=network.target

[Service]
Type=simple
User=monitor
WorkingDirectory=/opt/relay-pulse
EnvironmentFile=/etc/relay-pulse.env
ExecStart=/opt/relay-pulse/monitor /opt/relay-pulse/config/config.production.yaml
Restart=always
RestartSec=10
LimitNOFILE=4096

# 安全加固
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=/opt/relay-pulse/monitor

[Install]
WantedBy=multi-user.target
```

**启动服务**：
```bash
sudo systemctl daemon-reload
sudo systemctl enable relay-pulse.service
sudo systemctl start relay-pulse.service
sudo systemctl status relay-pulse.service
```

### 前端部署

```bash
# 构建前端
cd frontend
npm ci
npm run build

# 上传到服务器
rsync -av dist/ user@relaypulse.top:/var/www/relaypulse.top/dist/
```

**Cloudflare 配置说明**：

Go 服务通过 embed 直接提供所有静态资源和 API，无需单独的反向代理。生产环境使用 Cloudflare 提供 HTTPS、CDN 和安全防护。

详细配置步骤请查看 [docs/deployment.md](docs/deployment.md) 中的"Cloudflare 配置"章节。

### 安全提示

- ✅ 所有 API Key 使用环境变量，禁止提交到 Git
- ✅ `deploy/relaypulse.env` 必须加入 `.gitignore`
- ✅ 启用 HTTPS 和 HSTS（参见 `docs/deployment.md`）
- ✅ 配置 CORS 仅允许 `https://relaypulse.top`（参见 `internal/api/server.go`）
- ✅ PostgreSQL 使用 `sslmode=require`

### 部署验证清单

- [ ] `curl -I https://relaypulse.top/` 返回 200
- [ ] `curl https://relaypulse.top/api/status` 返回 JSON 数据
- [ ] 浏览器访问 `https://relaypulse.top` 显示仪表板
- [ ] 后端服务状态正常：`systemctl status relay-pulse` 或 `docker compose ps`
- [ ] 数据库有数据：`sqlite3 monitor/monitor.db 'SELECT COUNT(*) FROM probe_history;'`
- [ ] 配置热更新生效：修改 `config.production.yaml`，观察日志

## 技术栈

- **Web 框架**：gin
- **数据库**：SQLite (modernc.org/sqlite - 纯 Go)
- **配置**：yaml.v3
- **热更新**：fsnotify
- **CORS**：gin-contrib/cors

## 开发

### 开发模式（热重载）

推荐使用 [cosmtrek/air](https://github.com/cosmtrek/air) 进行本地开发，代码修改后自动重新编译和重启：

```bash
# 首次使用：安装 air
make install-air

# 启动开发服务（监听 .go 文件变化）
make dev
```

**工作原理**：
- 监听 `cmd/` 和 `internal/` 目录下的 `.go` 文件
- 文件变更后延迟 1 秒触发增量编译
- 自动重启后端服务
- 配置文件 `config.yaml` 仍由 `fsnotify` 热更新（互不干扰）

**可用命令**：
```bash
make help         # 查看所有可用命令
make build        # 编译生产版本
make run          # 直接运行（无热重载）
make dev          # 开发模式（需要air）
make test         # 运行测试
make fmt          # 格式化代码
make clean        # 清理临时文件
```

### 快速开始（无热重载）

```bash
# 安装 pre-commit
pip install pre-commit
pre-commit install

# 编译运行
go build -o monitor ./cmd/server
./monitor

# 或直接运行
make run
```

### 代码检查

```bash
# 手动运行所有检查
pre-commit run --all-files

# 单独检查
go fmt ./...
go vet ./...
go test ./...
```

### 详细指南

查看 [CONTRIBUTING.md](CONTRIBUTING.md) 获取完整的开发者指南，包括：

- 项目结构说明
- 代码规范
- 提交规范
- 常见问题

## 许可

MIT
