# testenv - 本地测试环境

使用生产配置在本地运行 RelayPulse，用于开发调试和功能验证。

## 前置条件

- Go 1.24+
- Docker（仅 PostgreSQL 模式需要）

## 快速开始

```bash
# PostgreSQL 模式（默认，需要 Docker）
./start.sh up

# SQLite 模式（无需 Docker，最轻量）
./start.sh up --sqlite
```

启动后访问 http://localhost:8080 查看面板。`Ctrl+C` 停止 Monitor。

## 全部命令

| 命令 | 说明 |
|------|------|
| `./start.sh up` | PostgreSQL 模式启动（自动拉起 PG + 编译 + 运行） |
| `./start.sh up --sqlite` | SQLite 模式启动（自动编译 + 运行，数据存 `monitor.db`） |
| `./start.sh down` | 停止 PostgreSQL 容器（保留数据） |
| `./start.sh down -v` | 停止 PostgreSQL 并删除数据卷 |
| `./start.sh pg` | 仅启动 PostgreSQL |
| `./start.sh build` | 仅编译 Monitor 二进制 |
| `./start.sh status` | 查看 PostgreSQL、Monitor 进程和端口状态 |

## 目录结构

```
testenv/
├── start.sh              # 启停脚本
├── config.yaml           # 生产配置（从服务器拷贝）
├── .env                  # 环境变量与 API Keys
├── docker-compose.yaml   # PostgreSQL 容器定义
├── templates -> ../templates  # 软链接（首次 up 自动创建）
├── monitor               # 编译产物（git 忽略）
└── monitor.db            # SQLite 数据库（仅 --sqlite 模式）
```

## 两种模式对比

| | PostgreSQL | SQLite |
|---|---|---|
| 依赖 | Docker | 无 |
| 数据持久化 | Docker Volume | 本地 `monitor.db` 文件 |
| 与生产一致性 | 完全一致 | 部分功能差异（如 `enable_db_timeline_agg` 仅 PG 生效） |
| 适用场景 | 功能验证、性能测试 | 快速启动、前端调试 |

## 注意事项

- `.env` 包含真实 API Key，**请勿提交到 Git**
- `config.yaml` 中的归档目录 `output_dir: "/app/archive"` 是 Docker 路径，本地运行会报归档目录创建失败的 WARN，不影响核心功能
- 首轮探测完成前页面可能短暂显示加载状态，等待数据填充即可
