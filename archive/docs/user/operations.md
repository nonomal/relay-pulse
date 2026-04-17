# 运维手册

> **⚠️ 历史文档 / Deprecated** — Last verified: 2026-04-17
> 本文内容已过期，保留作为背景参考。现行文档请见仓库根 `README.md` 与 `docs/user/`。

> **Audience**: 用户（运维人员） | **Last reviewed**: 2025-11-21

本文档介绍 Relay Pulse 的日常运维操作、健康检查、备份恢复和故障排查。

## 健康检查

### 服务状态检查

```bash
# 检查 HTTP 服务是否响应
curl http://localhost:8080/health
# 预期输出: {"status":"ok"}

# 检查 API 数据
curl http://localhost:8080/api/status | jq .

# 检查版本信息
curl http://localhost:8080/api/version
# 预期输出: {"version":"xxx","git_commit":"xxx","build_time":"xxx"}
```

### Docker 容器状态

```bash
# 查看容器状态
docker compose ps

# 查看实时日志
docker compose logs -f monitor

# 查看最近100行日志
docker compose logs --tail=100 monitor

# 检查容器资源使用
docker stats relaypulse-monitor
```

### 数据库检查

#### SQLite

```bash
# 检查数据库文件
ls -lh /data/monitor.db

# 查看数据量
sqlite3 /data/monitor.db "SELECT COUNT(*) FROM probe_history"

# 查看最近的记录
sqlite3 /data/monitor.db "SELECT * FROM probe_history ORDER BY timestamp DESC LIMIT 10"
```

#### PostgreSQL

```bash
# 连接数据库
docker compose exec postgres psql -U monitor -d llm_monitor

# 检查表
\dt

# 查看数据量
SELECT COUNT(*) FROM probe_history;

# 查看最近的记录
SELECT * FROM probe_history ORDER BY timestamp DESC LIMIT 10;
```

## 数据保留策略

Relay Pulse 每 24 小时自动执行一次 `CleanOldRecords(30)`，删除 `probe_history` 中超过 30 天的样本数据（适用于 SQLite 与 PostgreSQL）。

**查看执行情况**

```bash
docker compose logs monitor | grep "已清理"
```

**SQLite 手动清理**

```bash
docker compose exec monitor sqlite3 /data/monitor.db "DELETE FROM probe_history WHERE timestamp < strftime('%s','now','-30 day'); VACUUM;"
```

**PostgreSQL 手动清理**

```bash
docker compose exec postgres psql -U monitor -d llm_monitor -c "DELETE FROM probe_history WHERE timestamp < EXTRACT(EPOCH FROM NOW() - INTERVAL '30 days'); VACUUM;"
```

- 保留窗口目前固定为 30 天，如需不同策略请在 Issue 中反馈或在自定义构建中调整。

## 日志管理

### 查看日志

```bash
# 实时日志
docker compose logs -f

# 查看特定服务
docker compose logs -f monitor

# 查看最近的错误
docker compose logs --tail=50 monitor | grep ERROR

# 查看配置热更新日志
docker compose logs | grep "Config"
```

### 日志轮转

Docker Compose 配置日志轮转：

```yaml
services:
  monitor:
    logging:
      driver: "json-file"
      options:
        max-size: "10m"  # 单个日志文件最大 10MB
        max-file: "3"    # 保留最近 3 个日志文件
```

### 日志导出

```bash
# 导出所有日志
docker compose logs > relay-pulse-$(date +%Y%m%d).log

# 导出最近1小时日志
docker compose logs --since 1h > recent.log
```

## 备份与恢复

### SQLite 备份

#### 自动备份脚本

```bash
#!/bin/bash
# backup-sqlite.sh

BACKUP_DIR="/backups/relay-pulse"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="$BACKUP_DIR/monitor-$TIMESTAMP.db"

mkdir -p "$BACKUP_DIR"

# 从容器复制数据库
docker compose exec -T monitor cp /data/monitor.db /tmp/backup.db
docker cp relaypulse-monitor:/tmp/backup.db "$BACKUP_FILE"

# 压缩备份
gzip "$BACKUP_FILE"

# 保留最近7天的备份
find "$BACKUP_DIR" -name "monitor-*.db.gz" -mtime +7 -delete

echo "Backup completed: $BACKUP_FILE.gz"
```

#### 定时备份（Cron）

```bash
# 每天凌晨2点备份
0 2 * * * /opt/relay-pulse/backup-sqlite.sh >> /var/log/relay-pulse-backup.log 2>&1
```

#### 手动备份

```bash
# 备份
docker compose exec monitor cp /data/monitor.db /tmp/backup-$(date +%Y%m%d).db
docker cp relaypulse-monitor:/tmp/backup-$(date +%Y%m%d).db ./

# 恢复
docker cp ./backup-20250121.db relaypulse-monitor:/data/monitor.db
docker compose restart
```

### PostgreSQL 备份

#### pg_dump 备份

```bash
# 备份数据库
docker compose exec postgres pg_dump -U monitor -d llm_monitor \
  > backup-$(date +%Y%m%d).sql

# 压缩备份
gzip backup-$(date +%Y%m%d).sql
```

#### 恢复

```bash
# 解压备份
gunzip backup-20250121.sql.gz

# 恢复数据库
docker compose exec -T postgres psql -U monitor -d llm_monitor < backup-20250121.sql
```

#### 自动备份脚本

```bash
#!/bin/bash
# backup-postgres.sh

BACKUP_DIR="/backups/relay-pulse"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="$BACKUP_DIR/postgres-$TIMESTAMP.sql"

mkdir -p "$BACKUP_DIR"

# 导出数据库
docker compose exec -T postgres pg_dump -U monitor -d llm_monitor > "$BACKUP_FILE"

# 压缩
gzip "$BACKUP_FILE"

# 保留最近7天
find "$BACKUP_DIR" -name "postgres-*.sql.gz" -mtime +7 -delete

echo "Backup completed: $BACKUP_FILE.gz"
```

## 配置更新

### 热更新（无需重启）

```bash
# 修改配置文件
vi config.yaml

# 保存后，服务会自动重载
# 观察日志确认更新成功
docker compose logs -f | grep "Config"

# 预期日志:
# [Config] 检测到配置文件变更，正在重载...
# [Config] 热更新成功！已加载 X 个监控任务
# [Scheduler] 配置已更新，下次巡检将使用新配置
```

### 重启服务（存储配置变更）

```bash
# 修改存储配置后需要重启
docker compose restart

# 或完全重新部署
docker compose down
docker compose up -d
```

### 更新环境变量

```bash
# 修改环境变量文件
vi deploy/relaypulse.env

# 重新部署
docker compose down
docker compose --env-file deploy/relaypulse.env up -d
```

## 升级

### Docker 镜像升级

```bash
# 1. 备份数据库（重要！）
./backup-sqlite.sh

# 2. 拉取最新镜像
docker compose pull

# 3. 重启服务
docker compose up -d

# 4. 查看版本
curl http://localhost:8080/api/version

# 5. 验证服务
curl http://localhost:8080/health
```

### 回滚到旧版本

```bash
# 1. 停止当前服务
docker compose down

# 2. 指定镜像版本
docker pull ghcr.io/prehisle/relay-pulse:v1.2.0

# 3. 修改 docker-compose.yaml 指定版本
# image: ghcr.io/prehisle/relay-pulse:v1.2.0

# 4. 启动服务
docker compose up -d

# 5. 如果需要，恢复数据库备份
docker cp ./backup-20250121.db relaypulse-monitor:/data/monitor.db
docker compose restart
```

## 性能优化

### 资源限制

在 docker-compose.yaml 中配置：

```yaml
services:
  monitor:
    deploy:
      resources:
        limits:
          cpus: '1.0'      # 最多使用 1 个 CPU
          memory: 512M     # 最多使用 512MB 内存
        reservations:
          cpus: '0.5'      # 保证 0.5 个 CPU
          memory: 256M     # 保证 256MB 内存
```

### 数据库性能

#### SQLite 优化

```yaml
# 启用 WAL 模式（已默认开启）
storage:
  sqlite:
    path: "monitor.db?_journal_mode=WAL"
```

#### PostgreSQL 连接池

```yaml
storage:
  postgres:
    max_open_conns: 25     # 最大打开连接数
    max_idle_conns: 5      # 最大空闲连接数
    conn_max_lifetime: "1h" # 连接最大生命周期
```

### 巡检间隔调优

```yaml
# 根据监控项数量调整间隔
interval: "1m"   # 10个以下监控项
interval: "2m"   # 10-50个监控项
interval: "5m"   # 50个以上监控项
```

## 故障排查

### 问题1：静态资源返回 HTML（MIME 类型错误）

**症状**：
```
Failed to load module script: Expected a JavaScript module script
but the server responded with a MIME type of 'text/html'
```

**原因**：Docker 卷挂载 `relay-pulse-data:/app` 导致旧的二进制文件持续运行

**解决方案**：
```bash
# 1. 停止服务
docker compose down

# 2. 删除旧的卷（关键步骤！）
docker volume rm relay-pulse-data

# 3. 拉取最新镜像
docker compose pull

# 4. 重新启动
docker compose up -d

# 5. 验证修复
curl -I http://localhost:8080/assets/index-*.js | grep Content-Type
# 应该返回: Content-Type: text/javascript; charset=utf-8
```

**预防措施**：确保 docker-compose.yaml 使用正确的卷挂载：
```yaml
volumes:
  - relay-pulse-data:/data  # ✅ 只挂载数据目录
environment:
  - MONITOR_SQLITE_PATH=/data/monitor.db
```

### 问题2：CORS 跨域错误

**症状**：
```
Access to fetch at 'http://localhost:8080/api/status' from origin
'http://example.com:8080' has been blocked by CORS policy
```

**原因**：前端硬编码了 API 基础URL

**解决方案**：使用相对路径（已在最新版本修复）
```typescript
// frontend/src/constants/index.ts
export const API_BASE_URL = '';  // 使用相对路径
```

### 问题3：ContainerConfig KeyError（Docker Compose V1）

**症状**：
```
KeyError: 'ContainerConfig'
```

**原因**：docker-compose v1 (1.29.2) 与新版 Docker 镜像格式不兼容

**解决方案A**：升级到 Docker Compose V2（推荐）
```bash
# 检查版本
docker compose version

# 安装 V2
sudo apt-get update
sudo apt-get install docker-compose-plugin

# 使用新命令
docker compose up -d  # 注意是空格，不是连字符
```

**解决方案B**：完全清理后重启
```bash
docker compose down
docker rmi ghcr.io/prehisle/relay-pulse
docker system prune -a
docker compose pull
docker compose up -d --force-recreate
```

### 问题4：配置文件未找到

**症状**：
```
open -config: no such file or directory
```

**解决方案**：升级到最新镜像（已修复）
```bash
docker pull ghcr.io/prehisle/relay-pulse:latest
docker compose up -d
```

### 问题5：数据库权限错误

**症状**：
```
unable to open database file
```

**解决方案**：
```bash
# 重新创建数据卷
docker compose down
docker volume rm relay-pulse-data
docker compose up -d

# 或检查权限
docker exec relaypulse-monitor ls -la /data/
docker exec relaypulse-monitor chown -R 1000:1000 /data/
```

### 问题6：端口冲突

**症状**：
```
port is already allocated
```

**解决方案**：
```bash
# 查看占用进程
sudo lsof -i :8080
sudo netstat -tulpn | grep 8080

# 修改端口
# 编辑 docker-compose.yaml
ports:
  - "8888:8080"  # 使用 8888 端口
```

### 问题7：版本信息不显示

**症状**：启动日志中没有版本号、Git commit

**原因**：Docker 卷挂载导致使用旧二进制

**解决方案**：同问题1，删除旧卷

### 问题8：热更新不生效

**检查清单**：
1. 配置文件语法正确（YAML 格式）
2. 必填字段完整
3. 查看日志中的错误信息

```bash
# 查看配置重载日志
docker compose logs | grep "Config"

# 手动触发重载（修改配置后保存）
touch config.yaml
```

## 监控告警

### Prometheus 集成（🔮 未来功能）

> 此功能仍在规划阶段，以下端口暴露示例仅供提前预留资源。

暴露 Prometheus metrics（可选，未来功能）：

```yaml
# 添加到 docker-compose.yaml
services:
  monitor:
    ports:
      - "8080:8080"
      - "9090:9090"  # Prometheus metrics
```

### 健康检查告警

```bash
#!/bin/bash
# health-check-alert.sh

ENDPOINT="http://localhost:8080/health"
WEBHOOK_URL="https://hooks.slack.com/services/YOUR/WEBHOOK/URL"

if ! curl -f -s "$ENDPOINT" > /dev/null; then
    curl -X POST "$WEBHOOK_URL" \
      -H 'Content-Type: application/json' \
      -d '{"text":"🚨 Relay Pulse 服务异常！"}'
    exit 1
fi
```

定时检查：
```bash
# 每5分钟检查一次
*/5 * * * * /opt/relay-pulse/health-check-alert.sh
```

## 安全加固

### 1. 最小权限运行

```yaml
services:
  monitor:
    user: "1000:1000"  # 非 root 用户
    read_only: true    # 只读根文件系统
    tmpfs:
      - /tmp
```

### 2. 网络隔离

```yaml
networks:
  relay-pulse-network:
    driver: bridge
    internal: true  # 内部网络，不暴露到外网
```

### 3. Secret 管理

```bash
# 使用 Docker Secrets
echo "sk-your-api-key" | docker secret create openai_api_key -

# 在 docker-compose.yaml 中引用
services:
  monitor:
    secrets:
      - openai_api_key
secrets:
  openai_api_key:
    external: true
```

### 4. 定期安全更新

```bash
# 每周检查更新
docker compose pull
docker compose up -d
```

## 常用运维命令速查

```bash
# 启动/停止/重启
docker compose up -d
docker compose down
docker compose restart

# 查看状态
docker compose ps
docker compose logs -f
docker stats relaypulse-monitor

# 更新
docker compose pull
docker compose up -d

# 备份
docker cp relaypulse-monitor:/data/monitor.db ./backup-$(date +%Y%m%d).db

# 进入容器
docker exec -it relaypulse-monitor sh

# 查看版本
curl http://localhost:8080/api/version

# 健康检查
curl http://localhost:8080/health
```

## 获取帮助

如果以上方案无法解决问题：

1. **查看完整日志**：
   ```bash
   docker compose logs > error.log
   ```

2. **提交 Issue**：https://github.com/prehisle/relay-pulse/issues

3. **包含以下信息**：
   - 操作系统和版本
   - Docker 版本：`docker version`
   - Docker Compose 版本：`docker compose version`
   - 错误日志
   - docker-compose.yaml 配置（脱敏后）

## 下一步

- [配置手册](config.md) - 详细配置说明
- [安装指南](install.md) - 安装和部署
- [API 规范](../reference/api.md) - REST API 详细文档
