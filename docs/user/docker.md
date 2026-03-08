# Docker 部署指南（高级）

> 基础部署步骤请参见 [QUICKSTART.md](../../QUICKSTART.md)，本文档覆盖高级配置与技术细节。

## 架构说明

### 嵌入式部署

前端静态文件已**完全嵌入**到 Go 二进制文件中，无需 Nginx 反向代理：

```
┌─────────────────────────────┐
│   Docker Container (8080)   │
│  ┌───────────────────────┐  │
│  │   Go HTTP Server      │  │
│  │  ┌─────────────────┐  │  │
│  │  │  API Routes     │  │  │
│  │  │  /api/status    │  │  │
│  │  │  /health        │  │  │
│  │  └─────────────────┘  │  │
│  │  ┌─────────────────┐  │  │
│  │  │  Static Files   │  │  │
│  │  │  (Embedded)     │  │  │
│  │  │  /assets/*      │  │  │
│  │  │  /               │  │  │
│  │  └─────────────────┘  │  │
│  └───────────────────────┘  │
└─────────────────────────────┘
```

### 构建流程

```
1. Frontend Builder (Node.js)
   ├─ npm ci
   ├─ npm run build
   └─ 输出: dist/

2. Backend Builder (Go)
   ├─ 复制 dist/ 到 internal/api/frontend/dist
   ├─ go:embed 嵌入静态文件
   └─ 编译生成单个二进制文件

3. Runtime (Alpine)
   └─ 仅包含编译好的二进制文件
```

## 端口映射

默认映射 `80:8080`（本地 80 端口映射到容器内 8080 端口）。容器内服务始终监听 8080。

如需修改外部端口：

```yaml
ports:
  - "3000:8080"  # 本地 3000 映射到容器 8080
```

> **注意**: `docker-compose.pg.yaml`（PostgreSQL 部署）默认映射为 `8081:8080`。

## 配置说明

### 环境变量

在 `docker-compose.yaml` 中配置：

```yaml
environment:
  # 时区
  - TZ=Asia/Shanghai

  # API 密钥（覆盖 config.yaml）
  - MONITOR_88CODE_CC_API_KEY=sk-xxx
  - MONITOR_DUCKCODING_CC_API_KEY=sk-xxx
```

环境变量命名规则详见 [QUICKSTART.md](../../QUICKSTART.md) 或 [配置手册](config.md)。

### 数据持久化

- **SQLite 数据库**: 挂载卷 `relay-pulse-data`
- **配置文件**: `./config/config.yaml` → `/config/config.yaml`（目录挂载，支持热更新）
- **数据目录**: `./data` → `/app/data`

## 健康检查

容器自带健康检查：

```bash
# 查看健康状态
docker compose ps

# 手动健康检查
curl http://localhost/health
```

## 常用命令

```bash
# 进入容器
docker compose exec monitor sh

# 重新构建（清理缓存）
docker compose build --no-cache

# 清理并重建
docker compose down -v
docker compose up -d --build

# 更新配置（热更新，无需重启）
vim config/config.yaml
# 监测服务会自动检测配置变更并重载
```

## 故障排查

### 构建失败

```bash
# 查看构建日志
docker compose build

# 清理缓存重建
docker compose build --no-cache
```

### 服务无法访问

```bash
# 检查容器状态
docker compose ps

# 查看日志
docker compose logs -f monitor

# 检查端口占用（默认端口 80）
lsof -i :80
```

### 配置未生效

```bash
# 确认配置文件挂载
docker compose exec monitor cat /config/config.yaml

# 重启服务
docker compose restart
```

## 生产部署建议

### 1. 资源限制

```yaml
deploy:
  resources:
    limits:
      cpus: '0.5'
      memory: 256M
```

### 2. 日志驱动

```yaml
logging:
  driver: "json-file"
  options:
    max-size: "10m"
    max-file: "3"
```

### 3. 使用 .env 文件

```bash
# .env
MONITOR_88CODE_CC_API_KEY=sk-xxx
MONITOR_DUCKCODING_CC_API_KEY=sk-xxx
```

## 多环境部署

### SQLite（默认）

```bash
docker compose up -d
```

### PostgreSQL

```bash
# 1. 准备环境变量文件
cp .env.pg.example .env.pg
vim .env.pg  # 编辑数据库密码等配置

# 2. 启动 PostgreSQL 部署（端口 8081）
docker compose -f docker-compose.pg.yaml up -d

# 3. 访问服务
# Web 界面: http://localhost:8081
# API: http://localhost:8081/api/status
```

详细的 PostgreSQL 部署说明请参见 [PostgreSQL 部署指南](deploy-postgres.md)。

## 技术细节

### Go embed

使用 Go 1.16+ 的 `embed` 包：

```go
//go:embed frontend/dist
var frontendFS embed.FS
```

### 路由策略

- `/api/*` → API 处理器
- `/assets/*` → 静态资源（embed FS）
- `/*` → SPA 回退（index.html）

### 优势

- 单一二进制文件，部署简单
- 无需 Nginx，减少组件复杂度
- 更小的镜像体积
- 更快的启动速度
- 完整的 Go 生态工具支持
