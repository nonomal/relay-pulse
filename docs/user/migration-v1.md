# v1.0 迁移指南

> **适用对象**: 用户 | **最后更新**: 2026-01-29

本文档说明如何从 RelayPulse v0.x 升级到 v1.0。

## v1.0 新特性概览

### 用户系统

- **GitHub OAuth 登录**：支持通过 GitHub 账号登录
- **角色管理**：admin（管理员）和 user（普通用户）两种角色
- **会话管理**：支持滑动续期，7 天有效期

### 监测模板系统

- **服务定义**：预定义服务类型（Claude Code、Codex、Gemini 等）
- **模板配置**：每个服务可配置多个监测模板
- **多模型支持**：模板支持多模型配置，自动测试所有模型

### 用户自主提交

- **申请向导**：用户可自主提交监测项申请
- **自测功能**：提交前自动测试 API 可用性
- **审核流程**：管理员审核通过后自动创建监测项

### 架构变更

- **仅支持 PostgreSQL**：v1.0 移除了 SQLite 支持
- **新数据库表**：users、services、monitor_templates、monitors 等

## 升级前准备

### 1. 备份数据

```bash
# 备份 PostgreSQL 数据库
pg_dump -h localhost -U postgres relay_pulse > backup_v0.sql

# 备份配置文件
cp config.yaml config.yaml.backup
```

**注意**：v1.0 迁移工具仅支持 PostgreSQL。如果 v0.x 使用 SQLite，需要先手动导出数据或重新配置监测项。

### 2. 检查环境变量

v1.0 需要以下新环境变量：

```bash
# 必需：加密密钥（用于 API Key 加密）
CONFIG_ENCRYPTION_KEY=$(openssl rand -base64 32)

# 可选：GitHub OAuth（启用用户系统）
GITHUB_CLIENT_ID=your_client_id
GITHUB_CLIENT_SECRET=your_client_secret
GITHUB_CALLBACK_URL=https://your-domain.com/api/auth/github/callback
```

### 3. 确认 PostgreSQL 版本

v1.0 需要 PostgreSQL 12+，建议使用 PostgreSQL 14+。

## 升级步骤

### 方式一：使用迁移工具（推荐）

```bash
# 1. 下载最新版本
docker pull ghcr.io/prehisle/relay-pulse:v1.0

# 2. 运行迁移工具
docker run --rm \
  -e DATABASE_URL="postgres://user:pass@host:5432/relay_pulse?sslmode=disable" \
  ghcr.io/prehisle/relay-pulse:v1.0 \
  /app/migrate -init -migrate

# 3. 启动新版本
docker compose up -d
```

### 方式二：手动迁移

```bash
# 1. 停止服务
docker compose down

# 2. 初始化 v1.0 表结构
migrate -db "postgres://..." -init

# 3. 迁移数据
migrate -db "postgres://..." -migrate

# 4. 启动新版本
docker compose up -d
```

### 迁移工具选项

```bash
migrate [选项]

选项:
  -db string
        PostgreSQL 数据库连接 URL
        格式: postgres://user:password@host:port/dbname?sslmode=disable

  -init
        初始化 v1.0 数据库表结构

  -migrate
        执行 v0 到 v1 数据迁移

  -dry-run
        仅检查，不执行实际操作

示例:
  # 预览迁移
  migrate -db "postgres://..." -migrate -dry-run

  # 完整迁移
  migrate -db "postgres://..." -init -migrate
```

## 配置 GitHub OAuth

### 1. 创建 GitHub OAuth App

1. 访问 [GitHub Developer Settings](https://github.com/settings/developers)
2. 点击 "New OAuth App"
3. 填写信息：
   - Application name: `RelayPulse`
   - Homepage URL: `https://your-domain.com`
   - Authorization callback URL: `https://your-domain.com/api/auth/github/callback`
4. 保存 Client ID 和 Client Secret

### 2. 配置环境变量

```bash
GITHUB_CLIENT_ID=your_client_id
GITHUB_CLIENT_SECRET=your_client_secret
GITHUB_CALLBACK_URL=https://your-domain.com/api/auth/github/callback
```

### 3. 设置管理员

首个登录的用户默认为普通用户。设置管理员：

```sql
-- 将指定用户设为管理员
UPDATE users SET role = 'admin' WHERE username = 'your-github-username';
```

## 数据迁移说明

### 迁移内容

| 源表 | 目标表 | 说明 |
|------|--------|------|
| monitor_configs | monitors | 监测项配置（自动转换） |
| monitor_secrets | monitor_secrets | API Key（直接复制，需保持原 ID） |
| probe_history | probe_history | 探测记录（保留，不迁移） |
| status_events | status_events | 状态事件（保留，不迁移） |

**注意**：
- `probe_history` 和 `status_events` 表结构不变，数据保留在原位置
- `monitor_secrets` 的 API Key 使用 monitor_id 作为 AAD 加密，迁移时会复制到新表
- 如果 monitor_id 发生变化，需要重新设置 API Key

### 迁移映射

迁移工具会创建 `monitor_id_mapping` 表记录 ID 映射：

```sql
SELECT old_id, new_id, legacy_provider, legacy_service, legacy_channel
FROM monitor_id_mapping;
```

### 注意事项

1. **API Key 重新加密**：迁移时会使用新的 monitor ID 重新加密 API Key
2. **唯一性约束**：(provider, service, channel) 组合必须唯一
3. **幂等执行**：迁移脚本可重复执行，已迁移的数据会跳过

## 回滚方案

如需回滚到 v0.x：

```bash
# 1. 停止 v1.0 服务
docker compose down

# 2. 恢复数据库
psql -h localhost -U postgres relay_pulse < backup_v0.sql

# 3. 启动 v0.x 版本
docker pull ghcr.io/prehisle/relay-pulse:v0.x
docker compose up -d
```

## 常见问题

### Q: 迁移后监测项不显示？

检查 monitors 表是否有数据：
```sql
SELECT COUNT(*) FROM monitors WHERE enabled = true;
```

### Q: API Key 解密失败？

确保 `CONFIG_ENCRYPTION_KEY` 与迁移时使用的密钥一致。

### Q: GitHub 登录失败？

1. 检查 Callback URL 是否正确
2. 确认 Client ID 和 Secret 正确
3. 检查服务器时间是否准确

### Q: 如何批量设置管理员？

```sql
UPDATE users SET role = 'admin'
WHERE github_id IN (12345, 67890);
```

## 相关文档

- [配置手册](config.md)
- [PostgreSQL 部署](deploy-postgres.md)
- [Docker 部署](docker.md)
