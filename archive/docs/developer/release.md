# 版本发布指南

> **⚠️ 历史文档 / Deprecated** — Last verified: 2026-04-17
> 本文内容已过期，保留作为背景参考。现行文档请见仓库根 `README.md`、`CONTRIBUTING.md` 与 `docs/user/`。

> **Last reviewed**: 2025-11-22

本文档说明如何发布新版本和管理 Docker 镜像。

## 版本号系统

项目使用 **Git tag** 作为版本号的单一真实来源，遵循 [Semantic Versioning](https://semver.org/)。

### 版本号格式

```
v<major>.<minor>.<patch>
例如: v0.1.0, v1.2.3
```

### 版本信息注入

版本信息在构建时自动注入到以下位置：

| 位置 | 说明 |
|------|------|
| Go 二进制 | 通过 ldflags 注入到 `internal/buildinfo` 包 |
| Docker 镜像 | 构建参数 + OCI 标签 |
| `/api/version` | API 端点返回版本信息 |
| 前端 Footer | 通过 API 获取并显示 |

## 发布新版本

### 快速流程

```bash
# 1. 确保代码已提交并推送
git status  # 确保工作区干净
git push origin main

# 2. 创建并推送 tag
git tag -a v0.2.0 -m "Release v0.2.0 - 功能描述"
git push origin v0.2.0

# 3. 等待 GitHub Actions 完成（约2-3分钟）
# 查看进度: https://github.com/prehisle/relay-pulse/actions
```

### 详细步骤

#### 1. 准备发布

```bash
# 确保在 main 分支
git checkout main
git pull origin main

# 确保工作区干净
git status
```

#### 2. 更新版本相关文件（可选）

如有需要，更新 CHANGELOG.md：

```markdown
## [0.2.0] - 2025-11-22

### Added
- 新功能描述

### Fixed
- 修复内容
```

#### 3. 创建 Tag

```bash
# 创建带注释的 tag
git tag -a v0.2.0 -m "Release v0.2.0

主要更新:
- 功能1
- 功能2
- 修复xxx
"

# 推送 tag
git push origin v0.2.0
```

#### 4. 验证发布

```bash
# 检查 GitHub Actions
open https://github.com/prehisle/relay-pulse/actions

# 检查 Docker 镜像
open https://github.com/prehisle/relay-pulse/pkgs/container/relay-pulse
```

## Docker 镜像

### 镜像仓库

```
ghcr.io/prehisle/relay-pulse
```

### CI/CD 触发规则

| 事件 | 构建的标签 |
|------|-----------|
| 推送 `main` 分支 | `latest` |
| 推送 `v*.*.*` tag | `0.1.0`, `0.1`, `0`, `latest` |
| 手动触发 | `latest` (可选多架构) |

### 镜像标签说明

| 标签 | 说明 |
|------|------|
| `latest` | 最新版本（跟随 main 分支或最新 tag） |
| `0.1.0` | 特定版本 |
| `0.1` | 0.1.x 最新版 |
| `0` | 0.x.x 最新版 |

### 拉取镜像

```bash
# 最新版
docker pull ghcr.io/prehisle/relay-pulse:latest

# 特定版本
docker pull ghcr.io/prehisle/relay-pulse:0.1.0
```

### 构建架构

- **默认**: `linux/amd64`（构建快，约2-3分钟）
- **多架构**: `linux/amd64` + `linux/arm64`（需手动触发，约10分钟）

#### 构建多架构镜像

1. 访问 [Actions 页面](https://github.com/prehisle/relay-pulse/actions/workflows/docker-publish.yml)
2. 点击 "Run workflow"
3. 勾选 "构建多架构镜像 (amd64 + arm64)"
4. 点击 "Run workflow"

## 本地构建

### 构建二进制

```bash
# 使用构建脚本（推荐，自动注入版本信息）
make build

# 或指定版本
make release VERSION=v0.2.0
```

### 构建 Docker 镜像

```bash
# 本地构建
bash scripts/docker-build.sh

# 构建并推送（需要先登录 ghcr.io）
bash scripts/docker-build.sh --push
```

### 版本信息查看

```bash
# 查看版本脚本输出
bash scripts/version.sh
# 输出:
# VERSION=v0.1.0
# GIT_COMMIT=abc1234
# BUILD_TIME=2025-11-22T12:00:00Z
# IMAGE_TAG=0.1.0

# 查看运行中服务的版本
curl http://localhost:8080/api/version
```

## 文件说明

| 文件 | 作用 |
|------|------|
| `scripts/version.sh` | 版本信息获取脚本 |
| `scripts/build.sh` | Go 二进制构建脚本 |
| `scripts/docker-build.sh` | Docker 镜像构建脚本 |
| `internal/buildinfo/buildinfo.go` | 版本信息 Go 包 |
| `.github/workflows/docker-publish.yml` | CI/CD 工作流 |

## 常见问题

### Q: 如何修改已推送的 tag？

```bash
# 删除本地和远程 tag
git tag -d v0.1.0
git push origin :refs/tags/v0.1.0

# 重新创建并推送
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
```

### Q: 为什么版本显示 `-dirty`？

说明工作区有未提交的修改。先提交所有修改：

```bash
git add .
git commit -m "your message"
```

### Q: 如何回滚到旧版本？

```bash
# Docker
docker pull ghcr.io/prehisle/relay-pulse:0.0.9
docker compose down && docker compose up -d

# 或在 docker-compose.yaml 中指定版本
# image: ghcr.io/prehisle/relay-pulse:0.0.9
```

### Q: GitHub Actions 构建失败怎么办？

1. 查看 Actions 日志定位错误
2. 修复代码后重新推送 tag：
   ```bash
   git tag -d v0.1.0
   git push origin :refs/tags/v0.1.0
   git tag -a v0.1.0 -m "v0.1.0"
   git push origin v0.1.0
   ```
