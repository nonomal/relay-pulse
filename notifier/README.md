# RelayPulse Notifier

Telegram 通知服务，用于订阅 RelayPulse 监测状态变更通知。

## 功能特性

- 通过 Telegram Bot 接收状态变更通知
- 支持一键从网页导入收藏列表
- 可配置的限流和重试机制
- 独立部署，与 RelayPulse 主服务解耦

## 快速开始

### Docker Compose 部署

1. 创建配置文件

```bash
cp config.yaml config.local.yaml
# 编辑 config.local.yaml 配置你的服务
```

2. 创建环境变量文件

```bash
cat > .env << 'EOF'
TELEGRAM_BOT_TOKEN=your_bot_token_here
RELAY_PULSE_API_TOKEN=your_api_token_here
RELAY_PULSE_EVENTS_URL=https://your-relay-pulse.com/api/events
EOF
```

3. 启动服务

```bash
docker compose up -d
```

4. 查看日志

```bash
docker compose logs -f notifier
```

### 直接运行二进制

```bash
# 编译
go build -o notifier ./cmd/notifier

# 运行
TELEGRAM_BOT_TOKEN=xxx ./notifier -config config.yaml
```

## 配置说明

```yaml
relay_pulse:
  events_url: "https://your-relay-pulse.com/api/events"
  api_token: ""                 # 必需，环境变量 RELAY_PULSE_API_TOKEN
  poll_interval: "5s"           # 轮询间隔

telegram:
  bot_token: ""                 # 必需，环境变量 TELEGRAM_BOT_TOKEN
  bot_username: "RelayPulseBot" # 用于生成 deeplink

database:
  driver: "sqlite"
  dsn: "file:data/notifier.db?_journal_mode=WAL"

api:
  addr: ":8081"                 # HTTP API 监听地址

limits:
  max_subscriptions_per_user: 20
  rate_limit_per_second: 25     # Telegram 限速
  max_retries: 3
  bind_token_ttl: "5m"
```

## 环境变量

| 变量名 | 说明 | 必需 |
|--------|------|------|
| `TELEGRAM_BOT_TOKEN` | Telegram Bot Token | 是 |
| `RELAY_PULSE_API_TOKEN` | RelayPulse Events API Token | 是 |
| `RELAY_PULSE_EVENTS_URL` | RelayPulse Events API URL | 否 |

## Bot 命令

| 命令 | 说明 |
|------|------|
| `/start` | 开始使用 / 导入收藏 |
| `/list` | 查看当前订阅 |
| `/add <provider> <service> [channel]` | 添加订阅 |
| `/remove <provider> <service> [channel]` | 移除订阅 |
| `/clear` | 清空所有订阅 |
| `/status` | 查看服务状态 |
| `/help` | 显示帮助 |

## API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/health` | GET | 健康检查 |
| `/api/bind-token` | POST | 创建绑定 token |
| `/api/bind-token/{token}` | GET | 获取并消费 token |

## 前端集成

在前端设置环境变量指向 notifier 服务：

```bash
# .env.production
VITE_NOTIFIER_API_URL=https://notifier.example.com
```

点击"订阅通知"按钮后，前端会：
1. 调用 `/api/bind-token` 创建临时 token
2. 打开 Telegram deeplink 跳转到 Bot
3. Bot 自动解析 token 并导入收藏列表

## 架构

```
┌─────────────────────────────────────────────────────────────────┐
│                    relay-pulse-notifier                          │
│                                                                  │
│  ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────────┐  │
│  │ Poller  │───▶│ Router  │───▶│ Sender  │───▶│  Telegram   │  │
│  │         │    │         │    │         │    │    API      │  │
│  └─────────┘    └─────────┘    └─────────┘    └─────────────┘  │
│       ↑              ↑              │                           │
│       │              │              ▼                           │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                       SQLite                             │   │
│  │  poll_cursor | telegram_chats | subscriptions |          │   │
│  │  bind_tokens | deliveries                                │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                  │
│  ┌─────────┐                                                    │
│  │   Bot   │◀────────────── Long Polling ──────────────────────│
│  └─────────┘                                                    │
└─────────────────────────────────────────────────────────────────┘
```

## 开发

```bash
# 安装依赖
go mod download

# 运行测试
go test ./...

# 构建
go build ./cmd/notifier
```

## 许可证

MIT
