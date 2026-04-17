# 架构概览

> **⚠️ 历史文档 / Deprecated** — Last verified: 2026-04-17
> 本文内容已过期，保留作为背景参考。现行文档请见仓库根 `README.md`、`CONTRIBUTING.md` 与 `docs/user/`。

> **Audience**: 开发者（贡献者） | **Last reviewed**: 2025-11-21

本文档介绍 Relay Pulse 的技术架构、核心设计和模块职责。

## 系统架构

```
┌─────────────────────────────────────────────────────────────┐
│                        Frontend (React)                      │
│  ┌─────────────┐  ┌──────────────┐  ┌───────────────────┐  │
│  │  Dashboard  │  │ StatusTable  │  │  TimelineChart    │  │
│  └──────┬──────┘  └──────┬───────┘  └────────┬──────────┘  │
│         └────────────────┴──────────────────┬─┘             │
│                                              │               │
│                      useMonitorData Hook     │               │
│                              │               │               │
└──────────────────────────────┼───────────────┼───────────────┘
                               │  HTTP         │
                               ▼               ▼
┌─────────────────────────────────────────────────────────────┐
│                 Backend (Go) - API Layer                     │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ internal/api/                                         │  │
│  │  - server.go      : Gin router, CORS, static files   │  │
│  │  - handler.go     : /api/status, query params        │  │
│  │  - frontend/dist  : Embedded React build (go:embed)  │  │
│  └────────────┬─────────────────────────────────────────┘  │
│               │                                             │
│               ▼                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ internal/scheduler/                                   │  │
│  │  - scheduler.go   : Periodic health checks,          │  │
│  │                     concurrency control,              │  │
│  │                     config hot-reload                 │  │
│  └────────────┬─────────────────────────────────────────┘  │
│               │                                             │
│               ▼                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ internal/monitor/                                     │  │
│  │  - probe.go       : HTTP health checks               │  │
│  │  - client.go      : HTTP client pool                 │  │
│  └────────────┬─────────────────────────────────────────┘  │
│               │                                             │
│               ▼                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ internal/storage/                                     │  │
│  │  - storage.go     : Storage interface                │  │
│  │  - factory.go     : Factory pattern                  │  │
│  │  - sqlite.go      : SQLite implementation            │  │
│  │  - postgres.go    : PostgreSQL implementation        │  │
│  └────────────┬─────────────────────────────────────────┘  │
│               │                                             │
└───────────────┼─────────────────────────────────────────────┘
                ▼
    ┌───────────────────────┐
    │   Database            │
    │  ┌─────────────────┐  │
    │  │ SQLite or       │  │
    │  │ PostgreSQL      │  │
    │  └─────────────────┘  │
    └───────────────────────┘
```

## 技术栈

### 后端
- **语言**: Go 1.24+
- **Web 框架**: [Gin](https://github.com/gin-gonic/gin)
- **配置**: [yaml.v3](https://github.com/go-yaml/yaml)
- **热更新**: [fsnotify](https://github.com/fsnotify/fsnotify)
- **数据库**: SQLite ([modernc.org/sqlite](https://gitlab.com/cznic/sqlite)) / PostgreSQL

### 前端
- **框架**: React 19
- **语言**: TypeScript
- **样式**: Tailwind CSS v4
- **构建**: Vite 7
- **状态**: Hooks (useState, useEffect)

## 核心设计模式

### 1. 分层架构

```
Presentation Layer  → API (Gin routes, handlers)
Business Logic      → Scheduler (periodic tasks, config reload)
Domain Logic        → Monitor (HTTP probes, client pool)
Data Layer          → Storage (interface abstraction)
```

### 2. 接口驱动设计

`storage.Storage` 接口允许灵活切换存储实现：

```go
type Storage interface {
    Init() error
    SaveProbeResult(ctx context.Context, result *ProbeResult) error
    GetHistory(ctx context.Context, query HistoryQuery) ([]ProbeResult, error)
    CleanOldRecords(daysToKeep int) error
    Close() error
}
```

### 3. 配置热更新机制

```
fsnotify 监听文件变更
     ↓
验证新配置（语法、必填字段）
     ↓
原子性更新（RWMutex 保护）
     ↓
触发回调（Scheduler, API Server）
     ↓
立即执行新配置的巡检
```

### 4. 并发安全

- `sync.RWMutex` 保护配置读写
- `sync.Once` 确保单例初始化
- HTTP client pool 复用连接
- Context 传播实现优雅关闭

### 5. 前端嵌入（go:embed）

```go
//go:embed frontend/dist
var frontendFS embed.FS

// 运行时从内存提供静态文件，无需外部依赖
```

## 模块详解

### cmd/server/main.go

**职责**：应用程序入口，依赖注入

**核心流程**：
1. 加载配置（`config.Loader`）
2. 初始化存储（`storage.New`）
3. 创建调度器（`scheduler.NewScheduler`）
4. 创建 API 服务器（`api.NewServer`）
5. 启动配置监听器（`config.NewWatcher`）
6. 启动定期清理任务
7. 监听中断信号，优雅关闭

### internal/config/

**职责**：配置管理、验证、热更新

#### config.go
- 数据结构定义（`AppConfig`, `ServiceConfig`）
- 配置验证（必填字段、枚举值）
- 配置规范化（默认值、占位符替换）

#### loader.go
- YAML 解析
- 环境变量覆盖（`MONITOR_*_API_KEY`）
- `!include` 文件引用支持

#### watcher.go
- 使用 `fsnotify` 监听配置文件
- 检测变更后验证新配置
- 调用回调函数通知订阅者

### internal/storage/

**职责**：数据持久化抽象层

#### storage.go
- 定义 `Storage` 接口
- 定义数据模型（`ProbeResult`, `HistoryQuery`）

#### factory.go
- 工厂模式创建存储实例
- 根据配置类型选择实现

#### sqlite.go
- SQLite 实现（WAL 模式）
- 自动创建表结构和索引
- 时间分组查询（小时/天）

#### postgres.go
- PostgreSQL 实现
- 连接池管理
- 支持多副本并发访问

### internal/monitor/

**职责**：HTTP 健康检查引擎

#### probe.go
- 执行 HTTP 请求
- 状态码判断（0=红, 1=绿, 2=黄, 3=灰）
- 延迟测量
- 语义验证（`success_contains`）

**状态码逻辑**：
```
HTTP 4xx/5xx 或网络错误        → 0 (红色，不可用)
HTTP 2xx + 延迟高              → 2 (黄色，波动)
HTTP 2xx + 延迟低 + 内容匹配   → 1 (绿色，可用)
HTTP 400/401/403               → 3 (灰色，无数据)
```

#### client.go
- HTTP client 池管理
- 超时配置
- 连接复用

### internal/scheduler/

**职责**：周期性任务调度

#### scheduler.go
- 使用 `time.Ticker` 定期触发
- 并发执行所有监控项（goroutine pool）
- 防重复触发（mutex 保护）
- 配置热更新支持
- 立即触发机制（`TriggerNow()`）

### internal/api/

**职责**：HTTP API 和静态文件服务

#### server.go
- Gin 路由配置
- CORS 中间件
- 静态文件服务（`go:embed`）
- SPA 路由回退（NoRoute handler）

**关键路由**：
- `GET /health` - 健康检查
- `GET /api/status` - 监控数据
- `GET /api/version` - 版本信息
- `GET /assets/*` - 前端静态资源
- `NoRoute` - SPA fallback

#### handler.go
- `/api/status` 实现
- 查询参数解析（`period`, `provider`, `service`）
- 数据聚合和格式化

## 数据流

### 1. 健康检查流程

```
Scheduler 启动定时器 (interval)
    ↓
并发执行所有 Monitor.Probe()
    ↓
发送 HTTP 请求，测量延迟
    ↓
判断状态码 (0/1/2/3)
    ↓
Storage.SaveProbeResult()
    ↓
写入数据库（SQLite/PostgreSQL）
```

### 2. API 查询流程

```
前端 useMonitorData Hook 定时轮询
    ↓
GET /api/status?period=24h
    ↓
Handler 解析查询参数
    ↓
Storage.GetHistory(query)
    ↓
数据库查询（时间分组）
    ↓
格式化 JSON 响应
    ↓
前端渲染时间线图表
```

### 3. 配置热更新流程

```
用户修改 config.yaml
    ↓
fsnotify 检测到文件变更
    ↓
Loader 重新加载配置
    ↓
验证新配置（语法、必填字段）
    ↓
调用回调函数
    ├─→ Scheduler.UpdateConfig()
    └─→ API Server.UpdateConfig()
    ↓
Scheduler.TriggerNow() 立即巡检
```

## 构建流程

### Docker 多阶段构建

```dockerfile
# Stage 1: Frontend Builder
FROM node:20-alpine
WORKDIR /build
COPY frontend/ ./
RUN npm ci && npm run build
# 输出: /build/dist/

# Stage 2: Backend Builder
FROM golang:1.24-alpine
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY internal/ cmd/ ./
COPY --from=frontend-builder /build/dist ./internal/api/frontend/dist
RUN go build -ldflags="..." -o monitor ./cmd/server
# 输出: /build/monitor (包含嵌入的前端)

# Stage 3: Runtime
FROM alpine:3.19
COPY --from=backend-builder /build/monitor /app/monitor
CMD ["/app/monitor"]
```

**关键点**：
1. 前端构建产物复制到 Go embed 目录
2. Go 编译时将 `frontend/dist` 嵌入二进制
3. 运行时无需外部文件，单一二进制包含所有资产

### 版本信息注入

```bash
go build -ldflags="
  -X main.Version=${VERSION}
  -X main.GitCommit=${GIT_COMMIT}
  -X main.BuildTime=${BUILD_TIME}
  -X monitor/internal/api.Version=${VERSION}
  -X monitor/internal/api.GitCommit=${GIT_COMMIT}
  -X monitor/internal/api.BuildTime=${BUILD_TIME}
" -o monitor ./cmd/server
```

## 测试策略

### 单元测试

```bash
go test ./internal/config/
go test ./internal/monitor/
go test ./internal/storage/
```

**覆盖的场景**：
- 配置验证（必填字段、枚举值、唯一性）
- HTTP 探测（状态码判断、延迟测量）
- 存储操作（CRUD、时间分组查询）

### 集成测试

```bash
# 启动服务
go run cmd/server/main.go

# 测试 API
curl http://localhost:8080/api/status
curl http://localhost:8080/health

# 测试配置热更新
vim config.yaml  # 修改配置
# 观察日志中的重载信息
```

## 性能优化

### 1. HTTP Client 池复用

```go
var httpClient = &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
    },
}
```

### 2. 并发健康检查

```go
var wg sync.WaitGroup
for _, monitor := range monitors {
    wg.Add(1)
    go func(m *config.ServiceConfig) {
        defer wg.Done()
        probe.Execute(m)
    }(monitor)
}
wg.Wait()
```

### 3. SQLite WAL 模式

允许写入时并发读取：
```sql
PRAGMA journal_mode=WAL;
```

### 4. 数据库索引

```sql
CREATE INDEX idx_provider_service ON probe_history(provider, service, timestamp);
```

### 5. 前端性能

- React.memo 避免不必要的重渲染
- useMemo 缓存计算结果
- Vite 代码分割和懒加载

## 安全考虑

### 1. API Key 管理

- 支持环境变量覆盖
- 占位符 `{{API_KEY}}` 运行时替换
- 日志中脱敏处理

### 2. CORS 配置

```go
allowedOrigins := []string{"https://relaypulse.top"}
if extraOrigins := os.Getenv("MONITOR_CORS_ORIGINS"); extraOrigins != "" {
    allowedOrigins = append(allowedOrigins, strings.Split(extraOrigins, ",")...)
}
```

### 3. 输入验证

- HTTP 方法枚举验证
- URL 格式检查
- Provider/Service 唯一性校验

### 4. SQL 注入防护

使用参数化查询：
```go
db.Query("SELECT * FROM probe_history WHERE provider = ?", provider)
```

## 常见陷阱

### 1. Docker 卷挂载覆盖二进制

❌ 错误：
```yaml
volumes:
  - relay-pulse-data:/app  # 覆盖整个 /app，包括二进制文件
```

✅ 正确：
```yaml
volumes:
  - relay-pulse-data:/data  # 只挂载数据目录
environment:
  - MONITOR_SQLITE_PATH=/data/monitor.db
```

### 2. 前端 API 硬编码

❌ 错误：
```typescript
const API_BASE_URL = 'http://localhost:8080';
```

✅ 正确：
```typescript
const API_BASE_URL = '';  // 使用相对路径
```

### 3. goroutine 泄漏

确保所有 goroutine 响应 Context 取消：
```go
for {
    select {
    case <-ctx.Done():
        return
    case <-ticker.C:
        // do work
    }
}
```

## 下一步

- [开发工作流](workflow.md) - 本地开发、测试、发布流程
- [贡献指南](../../CONTRIBUTING.md) - 代码规范、提交规范
