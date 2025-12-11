# 版本检查指南

## 快速检查运行版本

### 方法 1: 查看启动日志

服务启动时会显示版本信息：

```bash
docker-compose logs monitor | head -10
```

输出示例：
```
🚀 Relay Pulse Monitor
📦 Version: e8acf6c-dirty
🔖 Git Commit: e8acf6c
🕐 Build Time: 2025-11-21 06:14:02 UTC
```

### 方法 2: 调用版本 API

```bash
curl http://localhost:8080/api/version
```

输出示例：
```json
{
  "version": "e8acf6c-dirty",
  "git_commit": "e8acf6c",
  "build_time": "2025-11-21 06:14:02 UTC"
}
```

### 方法 3: 浏览器开发者工具

1. 打开 http://localhost:8080
2. 按 F12 打开开发者工具
3. 在 Console 中输入：
```javascript
fetch('/api/version').then(r => r.json()).then(console.log)
```

## 版本信息说明

| 字段 | 说明 | 示例 |
|------|------|------|
| **version** | Git 描述版本 | `v1.0.0` 或 `e8acf6c-dirty` |
| **git_commit** | Git commit hash (短) | `e8acf6c` |
| **build_time** | 构建时间 (UTC) | `2025-11-21 06:14:02 UTC` |

### 版本格式

- **带 tag**: `v1.0.0` (git tag 版本号)
- **无 tag**: `e8acf6c` (commit hash 短格式)
- **有修改**: `e8acf6c-dirty` (本地有未提交修改)

## 确认是否使用最新版本

### 1. 检查最新 commit

在本地仓库：
```bash
git log -1 --oneline
```

### 2. 对比版本

如果服务器显示的 `git_commit` 与最新 commit 一致，说明使用的是最新版本。

### 3. 强制更新到最新版本

如果版本不一致，需要重新构建：

```bash
# 拉取最新代码
git pull origin main

# 重新构建镜像
docker-compose build --no-cache

# 重启服务
docker-compose up -d

# 验证新版本
docker-compose logs monitor | head -10
```

## 构建带版本信息的镜像

### 本地构建

使用提供的脚本自动注入版本信息：

```bash
# Go 二进制
./scripts/build.sh

# Docker 镜像
./scripts/docker-build.sh
```

### 手动构建

如果需要手动指定版本：

```bash
# 设置版本信息
export VERSION="v1.0.0"
export GIT_COMMIT=$(git rev-parse --short HEAD)
export BUILD_TIME=$(date -u '+%Y-%m-%d %H:%M:%S UTC')

# 构建
docker build \
  --build-arg VERSION="${VERSION}" \
  --build-arg GIT_COMMIT="${GIT_COMMIT}" \
  --build-arg BUILD_TIME="${BUILD_TIME}" \
  -t relay-pulse-monitor:${VERSION} \
  .
```

## 故障排查

### 问题：版本信息显示 "dev" / "unknown"

**原因**: 构建时未传递版本参数

**解决方案**: 使用 `scripts/build.sh` 或 `scripts/docker-build.sh` 构建

### 问题：版本信息与预期不符

**原因**: 使用了旧的镜像缓存

**解决方案**:
```bash
# 清除旧镜像
docker rmi relay-pulse-monitor

# 重新构建（不使用缓存）
docker-compose build --no-cache
docker-compose up -d
```

### 问题：本地和服务器版本不一致

**解决方案**:
1. 确认本地代码已推送: `git push origin main`
2. 服务器拉取最新代码: `git pull origin main`
3. 重新构建服务器镜像: `docker-compose build`
4. 重启服务: `docker-compose up -d`
5. 验证版本: `curl http://localhost:8080/api/version`

## 生产环境建议

1. **使用 Git tags** 标记版本:
   ```bash
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   ```

2. **构建时指定版本**:
   ```bash
   VERSION=v1.0.0 ./scripts/docker-build.sh
   ```

3. **定期检查版本**: 在监测告警中包含版本信息

4. **记录部署版本**: 在部署日志中记录 git_commit
