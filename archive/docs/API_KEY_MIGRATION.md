# API 密钥环境变量迁移完成

## 迁移总结

✅ **已完成的工作：**

1. **代码修改**：修改 `internal/config/config.go` 中的 `ApplyEnvOverrides()` 方法，支持包含 channel 的环境变量格式
   - 新格式：`MONITOR_{PROVIDER}_{SERVICE}_{CHANNEL}_API_KEY`（优先）
   - 旧格式：`MONITOR_{PROVIDER}_{SERVICE}_API_KEY`（向后兼容）

2. **密钥提取**：提取了 53 个 API Key 到 `.env` 文件
   - 位置：`/Users/stoa/codes/relay-pulse/.env`
   - 格式示例：`MONITOR_88CODE_CC_VIP3_API_KEY=88_7a06...`

3. **配置清理**：从 `config.yaml` 中移除了所有 `api_key` 字段
   - 保留了所有 YAML anchors（`<<: *cc-template` 等）
   - 备份文件：`config.yaml.backup-with-keys`

4. **验证测试**：成功验证配置加载和环境变量覆盖
   - ✅ 53 个 monitors 全部加载成功
   - ✅ 环境变量正确覆盖 api_key
   - ✅ 探测功能正常运行

## 环境变量命名规则

### 命名格式
```bash
MONITOR_{PROVIDER}_{SERVICE}_{CHANNEL}_API_KEY
```

### 自定义环境变量名（可选）
如果自动生成的环境变量名冲突（例如channel名称中有特殊字符），可以在 `config.yaml` 中使用 `env_var_name` 字段手动指定：

```yaml
- provider: "duckcoding"
  service: "cc"
  channel: "cc专用"
  env_var_name: "MONITOR_DUCKCODING_CC_STANDARD_API_KEY"  # 自定义环境变量名
  <<: *cc-template

- provider: "duckcoding"
  service: "cc"
  channel: "cc专用-特价"
  env_var_name: "MONITOR_DUCKCODING_CC_DISCOUNT_API_KEY"  # 避免与上面冲突
  <<: *cc-template
```

### 转换规则
1. **大写**：所有字母转为大写
2. **特殊字符替换**：`-`、空格等特殊字符替换为 `_`
3. **连续下划线合并**：多个连续的 `_` 合并为一个
4. **去除首尾下划线**

### 命名示例
| Provider | Service | Channel | 环境变量名 |
|----------|---------|---------|-----------|
| 88code | cc | vip3 | `MONITOR_88CODE_CC_VIP3_API_KEY` |
| duckcoding | cx | cx专用 | `MONITOR_DUCKCODING_CX_CX专用_API_KEY` |
| duckcoding | cc | cc专用 | `MONITOR_DUCKCODING_CC_STANDARD_API_KEY` (自定义) |
| duckcoding | cc | cc专用-特价 | `MONITOR_DUCKCODING_CC_DISCOUNT_API_KEY` (自定义) |
| right | cx | free | `MONITOR_RIGHT_CX_FREE_API_KEY` |

## 使用方法

### 1. 本地开发

```bash
# 确保 .env 文件存在
ls -la .env

# 加载环境变量并运行
set -a && source .env && set +a && ./monitor
```

### 2. 使用 dev.sh（Air 热重载）

dev.sh 脚本会自动加载 `.env` 文件：

```bash
./dev.sh
```

### 3. Docker 部署

在 `docker-compose.yml` 中使用 `env_file`：

```yaml
services:
  monitor:
    image: relay-pulse:latest
    env_file:
      - .env
```

### 4. Systemd 服务

在 service 文件中使用 `EnvironmentFile`：

```ini
[Service]
EnvironmentFile=/path/to/relay-pulse/.env
ExecStart=/path/to/relay-pulse/monitor
```

## 安全注意事项

⚠️ **重要：**

1. **不要提交 .env 文件到 Git**
   - ✅ `.env` 已在 `.gitignore` 中
   - ⚠️ 请勿移除 `.gitignore` 中的 `.env` 规则

2. **备份 API 密钥**
   - 备份文件：`config.yaml.backup-with-keys`
   - 请妥善保管此备份文件
   - 考虑将备份文件移动到安全位置

3. **生产环境**
   - 使用环境变量管理工具（如 Vault、AWS Secrets Manager）
   - 定期轮换 API 密钥
   - 限制 `.env` 文件的读取权限：`chmod 600 .env`

## 文件清单

### 新增文件
- `.env` - API 密钥环境变量（⚠️ 不要提交）
- `scripts/extract-apikeys.py` - 密钥提取脚本
- `scripts/verify-env.sh` - 环境变量验证脚本
- `config.yaml.backup-with-keys` - 含密钥的配置备份（⚠️ 不要提交）
- `config.yaml.no-keys` - 不含密钥的配置文件（临时）

### 修改文件
- `config.yaml` - 已移除所有 api_key 字段
- `internal/config/config.go` - ApplyEnvOverrides() 支持 channel

### 可以删除的临时文件
```bash
rm config.yaml.no-keys  # 已应用到 config.yaml
```

## 回滚方法

如果需要回滚到旧版本（硬编码密钥）：

```bash
# 恢复旧配置
cp config.yaml.backup-with-keys config.yaml

# 还原代码（如果已提交）
git checkout HEAD~1 internal/config/config.go
```

## 下一步

准备提交代码：

```bash
# 1. 检查 git 状态
git status

# 2. 确认 .env 不在暂存区
git check-ignore .env  # 应该输出 .env

# 3. 添加修改的文件
git add config.yaml
git add internal/config/config.go
git add scripts/extract-apikeys.py

# 4. 提交
git commit -m "refactor(config): 将 API 密钥迁移到环境变量

- 修改 ApplyEnvOverrides 支持 MONITOR_{PROVIDER}_{SERVICE}_{CHANNEL}_API_KEY 格式
- 从 config.yaml 移除所有硬编码的 api_key 字段
- 新增 scripts/extract-apikeys.py 用于提取密钥到 .env
- 向后兼容旧的 MONITOR_{PROVIDER}_{SERVICE}_API_KEY 格式
- 53 个 monitors 全部验证通过

Closes #密钥安全性改进"
```

---

生成时间：2025-12-14
迁移人员：Claude Code Agent
