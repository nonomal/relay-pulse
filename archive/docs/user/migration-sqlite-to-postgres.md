# SQLite → PostgreSQL 迁移指南

> **⚠️ 历史文档 / Deprecated** — Last verified: 2026-04-17
> 本文内容已过期，保留作为背景参考。现行文档请见仓库根 `README.md` 与 `docs/user/`。

> **Audience**: 运维人员 | **Last reviewed**: 2025-11-23

本文档介绍如何将生产环境从 SQLite 迁移到 PostgreSQL。

## 📋 迁移前检查

### 1. 确认当前环境

```bash
# 检查 SQLite 数据库位置
ls -lh monitor.db

# 统计数据量
sqlite3 monitor.db "SELECT COUNT(*) FROM probe_history;"

# 检查磁盘空间（需要至少 2 倍数据库大小）
df -h .
```

### 2. 准备 PostgreSQL

#### Docker Compose 环境

```bash
# 1. 启动 PostgreSQL 容器
docker compose up -d postgres

# 2. 验证连接
docker compose exec postgres psql -U monitor -d monitor -c "\dt"

# 应该看到 probe_history 表已创建
```

#### 独立 PostgreSQL

```bash
# 1. 创建数据库和用户
psql -U postgres <<EOF
CREATE DATABASE monitor;
CREATE USER monitor WITH PASSWORD 'your_secure_password';
GRANT ALL PRIVILEGES ON DATABASE monitor TO monitor;
\q
EOF

# 2. 验证连接
psql "host=localhost port=5432 user=monitor password=your_secure_password dbname=monitor sslmode=disable" -c "\dt"
```

## 🚀 迁移方案

### 方案 A：停机迁移（推荐，最安全）

**停机时间**：5-15 分钟（取决于数据量）

**适用场景**：
- 可以接受短暂停机
- 数据量 < 100 万条
- 生产环境首次迁移

#### 步骤 1：停止服务

```bash
# Docker Compose
docker compose stop monitor

# Systemd
sudo systemctl stop relay-pulse

# 二进制部署
kill $(cat monitor.pid)
```

#### 步骤 2：备份 SQLite

```bash
# 创建带时间戳的备份
cp monitor.db monitor.db.backup.$(date +%Y%m%d_%H%M%S)

# 验证备份
ls -lh monitor.db*
```

#### 步骤 3：执行迁移脚本

```bash
# 使用自动化脚本（推荐）
./scripts/migrate-sqlite-to-postgres.sh \
  monitor.db \
  "host=localhost port=5432 user=monitor password=monitor123 dbname=monitor sslmode=disable"

# 或使用 Docker Compose 中的 PostgreSQL
./scripts/migrate-sqlite-to-postgres.sh \
  monitor.db \
  "host=localhost port=5433 user=monitor password=monitor123 dbname=monitor sslmode=disable"
```

**脚本执行流程：**
1. 导出 SQLite 数据为 CSV
2. 验证数据完整性
3. 导入到 PostgreSQL
4. 重置序列（确保新记录 ID 不冲突）
5. 运行 ANALYZE 优化查询

#### 步骤 4：切换配置

##### Docker Compose

```yaml
# docker-compose.yaml
services:
  monitor:
    image: prehisle/relay-pulse:latest
    environment:
      - MONITOR_STORAGE_TYPE=postgres
      - MONITOR_POSTGRES_HOST=postgres
      - MONITOR_POSTGRES_PORT=5432
      - MONITOR_POSTGRES_USER=monitor
      - MONITOR_POSTGRES_PASSWORD=monitor123
      - MONITOR_POSTGRES_DBNAME=monitor
      - MONITOR_POSTGRES_SSLMODE=disable
```

```bash
# 启动 PostgreSQL 版本
docker compose up -d monitor
```

##### 配置文件

```yaml
# config.yaml
storage:
  type: postgres
  postgres:
    host: localhost
    port: 5432
    user: monitor
    password: monitor123  # 建议使用环境变量
    dbname: monitor
    sslmode: disable
    max_open_conns: 25
    max_idle_conns: 5
    conn_max_lifetime: 5m
```

##### 环境变量

```bash
# .env 或 systemd unit
export MONITOR_STORAGE_TYPE=postgres
export MONITOR_POSTGRES_HOST=localhost
export MONITOR_POSTGRES_PORT=5432
export MONITOR_POSTGRES_USER=monitor
export MONITOR_POSTGRES_PASSWORD=monitor123
export MONITOR_POSTGRES_DBNAME=monitor
export MONITOR_POSTGRES_SSLMODE=require  # 生产环境建议启用 SSL

./monitor
```

#### 步骤 5：验证迁移

```bash
# 1. 检查服务启动日志
docker compose logs -f monitor
# 应该看到：
# [Storage] PostgreSQL storage initialized successfully
# [Config] 已加载 X 个监控任务

# 2. 检查 API 返回
curl http://localhost:8080/api/status | jq '.data[] | {provider, service, uptime: .current_status.uptime}'

# 3. 检查数据库连接
docker compose exec postgres psql -U monitor -d monitor -c "SELECT COUNT(*) FROM probe_history;"

# 4. 检查最新记录（验证写入正常）
docker compose exec postgres psql -U monitor -d monitor -c "SELECT * FROM probe_history ORDER BY timestamp DESC LIMIT 5;"
```

#### 步骤 6：清理（可选）

```bash
# 确认运行 1 周无问题后，归档 SQLite
mkdir -p archive
mv monitor.db.backup.* archive/

# 或直接删除
rm monitor.db.backup.*
```

---

### 方案 B：双写迁移（零停机，复杂）

**停机时间**：0 分钟

**适用场景**：
- 完全不能接受停机
- 数据量巨大（> 100 万条）
- 需要逐步验证

**⚠️ 注意**：当前代码不支持双写，需要修改代码实现。

#### 原理

```
┌─────────────┐
│  Scheduler  │
└──────┬──────┘
       │ 写入
       ▼
┌──────────────────┐
│ MultiStorage     │  ← 新增包装层
│ ├─ SQLite        │
│ └─ PostgreSQL    │
└──────────────────┘
```

#### 实现步骤

1. **修改代码**：创建 `MultiStorage` 包装器同时写入两个数据库
2. **启动双写**：运行一段时间（如 1 天）
3. **验证数据一致性**：对比两个数据库
4. **切换读取**：从 PostgreSQL 读取
5. **停止双写**：移除 SQLite 写入

**📝 代码示例**（需要实现）：

```go
// internal/storage/multi.go
type MultiStorage struct {
    primary   Storage  // PostgreSQL
    secondary Storage  // SQLite (只写不读)
}

func (m *MultiStorage) SaveProbeResult(ctx context.Context, result *ProbeResult) error {
    // 写入 PostgreSQL（主）
    if err := m.primary.SaveProbeResult(ctx, result); err != nil {
        return err
    }
    // 写入 SQLite（副）- 失败不影响主流程
    _ = m.secondary.SaveProbeResult(ctx, result)
    return nil
}
```

---

## 🛠️ 手动迁移（不使用脚本）

如果无法使用迁移脚本，可以手动执行：

### 步骤 1：导出 SQLite

```bash
# 方法 1：CSV 格式（推荐）
sqlite3 monitor.db <<EOF
.mode csv
.headers off
.output probe_history.csv
SELECT id, provider, service, COALESCE(channel, ''), status, COALESCE(sub_status, ''), latency, timestamp
FROM probe_history
ORDER BY id;
.quit
EOF

# 方法 2：SQL INSERT 语句
sqlite3 monitor.db <<EOF
.mode insert probe_history
.output probe_history.sql
SELECT * FROM probe_history;
.quit
EOF
```

### 步骤 2：导入 PostgreSQL

```bash
# 方法 1：使用 \copy（推荐，快速）
psql "host=localhost port=5432 user=monitor password=monitor123 dbname=monitor sslmode=disable" <<EOF
\copy probe_history (id, provider, service, channel, status, sub_status, latency, timestamp) FROM 'probe_history.csv' WITH (FORMAT csv);
EOF

# 方法 2：执行 SQL 文件（慢）
psql "host=localhost port=5432 user=monitor password=monitor123 dbname=monitor sslmode=disable" -f probe_history.sql
```

### 步骤 3：重置序列

```bash
psql "host=localhost port=5432 user=monitor password=monitor123 dbname=monitor sslmode=disable" <<EOF
SELECT setval('probe_history_id_seq', (SELECT COALESCE(MAX(id), 0) FROM probe_history));
ANALYZE probe_history;
EOF
```

---

## 🔍 故障排查

### 问题 1：序列未重置导致 ID 冲突

**症状**：
```
ERROR: duplicate key value violates unique constraint "probe_history_pkey"
```

**解决**：
```bash
psql "$PG_CONN" -c "SELECT setval('probe_history_id_seq', (SELECT MAX(id) FROM probe_history));"
```

### 问题 2：数据量不一致

**排查**：
```bash
# SQLite
sqlite3 monitor.db "SELECT COUNT(*) FROM probe_history;"

# PostgreSQL
psql "$PG_CONN" -c "SELECT COUNT(*) FROM probe_history;"

# 检查时间范围
sqlite3 monitor.db "SELECT MIN(timestamp), MAX(timestamp) FROM probe_history;"
psql "$PG_CONN" -c "SELECT MIN(timestamp), MAX(timestamp) FROM probe_history;"
```

### 问题 3：PostgreSQL 连接失败

**排查**：
```bash
# 检查容器状态
docker compose ps postgres

# 检查端口
netstat -tuln | grep 5432

# 查看日志
docker compose logs postgres

# 测试连接
psql "host=localhost port=5432 user=monitor password=monitor123 dbname=monitor sslmode=disable" -c "SELECT 1;"
```

### 问题 4：导入数据后 channel 为空

**解决**：
启动服务后，会自动执行 `MigrateChannelData`，根据 `config.yaml` 中的 `channel` 配置补全历史数据。

**手动触发**：
```bash
# 重启服务即可
docker compose restart monitor
```

---

## 📊 性能对比

| 指标 | SQLite | PostgreSQL |
|------|--------|------------|
| 单机性能 | ⭐⭐⭐⭐⭐ 极快 | ⭐⭐⭐⭐ 快 |
| 并发写入 | ⭐⭐ 差（锁竞争） | ⭐⭐⭐⭐⭐ 优秀 |
| 水平扩展 | ❌ 不支持 | ✅ 支持多副本 |
| 高可用 | ❌ 不支持 | ✅ 支持主从/集群 |
| 运维成本 | ⭐⭐⭐⭐⭐ 零配置 | ⭐⭐⭐ 需要维护 |
| 适用场景 | < 10 监控项，单机 | > 20 监控项，K8s |

---

## ✅ 检查清单

迁移前：
- [ ] 已备份 SQLite 数据库
- [ ] 已准备 PostgreSQL 环境
- [ ] 已测试 PostgreSQL 连接
- [ ] 已停止监控服务

迁移中：
- [ ] 数据导出成功
- [ ] 数据导入完成
- [ ] 序列已重置
- [ ] 运行 ANALYZE

迁移后：
- [ ] 服务启动成功
- [ ] API 返回数据正常
- [ ] 新数据写入 PostgreSQL
- [ ] 前端显示正常
- [ ] 运行 7 天无异常

---

## 📚 相关文档

- [配置手册 - 存储后端](config.md#存储后端)
- [运维手册 - 备份恢复](operations.md#数据备份)
- [安装指南 - PostgreSQL 部署](install.md#postgresql-配置)
