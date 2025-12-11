# PostgreSQL 部署指南

本文档介绍如何使用 PostgreSQL 作为后端数据库部署 RelayPulse。

## 适用场景

- ✅ 生产环境部署
- ✅ 需要高并发支持
- ✅ 多副本部署（Kubernetes）
- ✅ 需要专业数据库备份和恢复
- ✅ 需要数据库层面的监测和优化

## 快速开始

### 1. 准备配置文件

```bash
# 复制环境变量模板
cp .env.pg.example .env

# 编辑配置文件，修改数据库密码
vim .env

# 必改项：
# - POSTGRES_PASSWORD=你的强密码（建议 16 位以上）
```

### 2. 准备监测配置

确保 `config.yaml` 已配置好监测服务和 API Keys。

### 3. 启动服务

```bash
# 启动 PostgreSQL + RelayPulse
docker-compose -f docker-compose.pg.yaml up -d

# 查看启动日志
docker-compose -f docker-compose.pg.yaml logs -f

# 等待看到以下关键日志：
# - "database system is ready to accept connections" (PostgreSQL)
# - "postgres 存储已就绪" (RelayPulse)
# - "监测服务已启动" (RelayPulse)
```

### 4. 验证部署

```bash
# 健康检查
curl http://localhost/health
# 预期输出: {"status":"ok"}

# 查看监测数据
curl http://localhost/api/status | jq

# 访问 Web 界面
# http://your-server-ip
```

## 数据库管理

### 连接数据库

```bash
# 使用 psql 连接
docker exec -it relaypulse-postgres psql -U relaypulse -d relaypulse

# 常用命令：
# \dt              - 列出所有表
# \d probe_history - 查看表结构
# \q               - 退出
```

### 查看数据

```sql
-- 查看总记录数
SELECT COUNT(*) FROM probe_history;

-- 查看最新 10 条记录
SELECT provider, service, status, latency,
       to_timestamp(timestamp) as time
FROM probe_history
ORDER BY timestamp DESC
LIMIT 10;

-- 查看某个服务的可用率
SELECT
    provider,
    service,
    COUNT(*) as total,
    SUM(CASE WHEN status = 1 THEN 1 ELSE 0 END) as success,
    ROUND(SUM(CASE WHEN status = 1 THEN 1 ELSE 0 END) * 100.0 / COUNT(*), 2) as uptime_pct
FROM probe_history
WHERE timestamp > EXTRACT(epoch FROM NOW() - INTERVAL '24 hours')
GROUP BY provider, service
ORDER BY uptime_pct DESC;
```

### 备份数据库

```bash
# 备份数据库
docker exec relaypulse-postgres pg_dump -U relaypulse -d relaypulse > backup_$(date +%Y%m%d_%H%M%S).sql

# 恢复数据库
cat backup_20231128_120000.sql | docker exec -i relaypulse-postgres psql -U relaypulse -d relaypulse
```

### 清理旧数据

RelayPulse 会自动清理 30 天前的数据。如需手动清理：

```sql
-- 删除 7 天前的数据
DELETE FROM probe_history
WHERE timestamp < EXTRACT(epoch FROM NOW() - INTERVAL '7 days');

-- 清理后执行 VACUUM 回收空间
VACUUM ANALYZE probe_history;
```

## 性能优化

### 查看慢查询

```sql
-- 启用 pg_stat_statements 扩展
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- 查看最慢的 5 个查询
SELECT
    query,
    calls,
    mean_exec_time,
    total_exec_time
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 5;
```

### 查看索引使用情况

```sql
-- 查看索引扫描次数
SELECT
    schemaname,
    tablename,
    indexname,
    idx_scan,
    idx_tup_read,
    idx_tup_fetch
FROM pg_stat_user_indexes
WHERE tablename = 'probe_history';
```

### 数据库大小

```sql
-- 查看数据库总大小
SELECT pg_size_pretty(pg_database_size('relaypulse'));

-- 查看表大小
SELECT pg_size_pretty(pg_total_relation_size('probe_history'));

-- 查看索引大小
SELECT
    indexname,
    pg_size_pretty(pg_relation_size(indexrelid))
FROM pg_stat_user_indexes
WHERE tablename = 'probe_history';
```

## 常见问题

### 1. 容器启动失败，提示密码未设置

**错误信息**:
```
WARNING: The POSTGRES_PASSWORD variable is not set. Defaulting to a blank string.
```

**解决方案**:
- 确保已创建 `.env` 文件（从 `.env.pg.example` 复制）
- 确保 `.env` 文件与 `docker-compose.pg.yaml` 在同一目录

### 2. Monitor 服务使用了 SQLite 而不是 PostgreSQL

**症状**: 日志显示 `✅ sqlite 存储已就绪`

**解决方案**:
```bash
# 检查环境变量是否正确传递
docker exec relaypulse-monitor env | grep STORAGE

# 应该看到:
# MONITOR_STORAGE_TYPE=postgres
```

### 3. 无法连接数据库

**错误信息**:
```
FATAL: role "monitor" does not exist
```

**解决方案**:
- 检查 `.env` 文件中的 `POSTGRES_USER` 是否为 `relaypulse`
- 删除数据卷重新初始化：
  ```bash
  docker-compose -f docker-compose.pg.yaml down -v
  docker-compose -f docker-compose.pg.yaml up -d
  ```

### 4. 表不存在

**错误信息**:
```
ERROR: relation "probe_history" does not exist
```

**解决方案**:
- 检查 monitor 服务日志，确认使用了 postgres 存储
- 手动创建表（临时方案）：
  ```bash
  docker exec -i relaypulse-postgres psql -U relaypulse -d relaypulse <<'EOF'
  CREATE TABLE IF NOT EXISTS probe_history (
      id BIGSERIAL PRIMARY KEY,
      provider TEXT NOT NULL,
      service TEXT NOT NULL,
      channel TEXT NOT NULL DEFAULT '',
      status INTEGER NOT NULL,
      sub_status TEXT NOT NULL DEFAULT '',
      latency INTEGER NOT NULL,
      timestamp BIGINT NOT NULL
  );
  CREATE INDEX IF NOT EXISTS idx_provider_service_channel_timestamp
  ON probe_history(provider, service, channel, timestamp DESC);
  EOF
  ```

## 生产环境建议

### 安全配置

1. **使用强密码**
   ```bash
   # 生成强密码
   openssl rand -base64 32
   ```

2. **启用 SSL**
   ```env
   MONITOR_POSTGRES_SSLMODE=require
   ```

3. **限制数据库端口暴露**
   - 在 `docker-compose.pg.yaml` 中移除 `ports: - "5432:5432"`
   - 仅允许 monitor 容器内部访问

4. **配置防火墙**
   ```bash
   # 仅允许内网访问
   ufw allow from 10.0.0.0/8 to any port 5432
   ```

### 监测告警

1. **PostgreSQL 连接数监测**
   ```sql
   SELECT count(*) FROM pg_stat_activity WHERE datname='relaypulse';
   ```

2. **数据库大小监测**
   ```sql
   SELECT pg_database_size('relaypulse') as size_bytes;
   ```

3. **慢查询告警**
   - 设置 `log_min_duration_statement = 1000` (记录超过 1 秒的查询)

### 备份策略

1. **定时备份**
   ```bash
   # 添加到 crontab
   0 2 * * * docker exec relaypulse-postgres pg_dump -U relaypulse -d relaypulse | gzip > /backup/relaypulse_$(date +\%Y\%m\%d).sql.gz
   ```

2. **保留策略**
   - 每日备份保留 7 天
   - 每周备份保留 4 周
   - 每月备份保留 12 个月

3. **验证备份**
   - 定期恢复备份到测试环境验证完整性

### 高可用配置

对于关键生产环境，建议使用：
- PostgreSQL 主从复制
- 使用云数据库服务（AWS RDS、阿里云 RDS 等）
- 配置自动故障转移

## 从 SQLite 迁移到 PostgreSQL

如需从现有 SQLite 部署迁移到 PostgreSQL，请参考项目根目录的迁移计划文档（由 AI 生成）。

简要步骤：
1. 备份现有 SQLite 数据库
2. 启动 PostgreSQL 服务
3. 使用迁移脚本 `scripts/migrate-sqlite-to-postgres.sh` 迁移数据
4. 切换到 PostgreSQL 配置
5. 验证数据完整性

## 相关文档

- [配置文件说明](./config.md)
- [Docker 部署指南](../QUICKSTART.md)
- [贡献指南](../../CONTRIBUTING.md)
