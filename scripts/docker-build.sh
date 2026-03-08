#!/bin/bash
# Docker 构建脚本 - 注入版本信息

set -e

# 解析参数
PUSH=false
REGISTRY="ghcr.io/prehisle"

while [[ $# -gt 0 ]]; do
  case $1 in
    --push)
      PUSH=true
      shift
      ;;
    --registry=*)
      REGISTRY="${1#*=}"
      shift
      ;;
    *)
      echo "未知参数: $1"
      echo "用法: $0 [--push] [--registry=ghcr.io/prehisle]"
      exit 1
      ;;
  esac
done

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 加载统一的版本信息
source "${SCRIPT_DIR}/version.sh"

echo "🐳 构建 Docker 镜像"
echo "📦 Version: $VERSION"
echo "🔖 Git Commit: $GIT_COMMIT"
echo "🕐 Build Time: $BUILD_TIME"
echo "🏷️  Registry: $REGISTRY"
echo ""

# 构建 Docker 镜像
docker build \
  --build-arg VERSION="${VERSION}" \
  --build-arg GIT_COMMIT="${GIT_COMMIT}" \
  --build-arg BUILD_TIME="${BUILD_TIME}" \
  -t relay-pulse-monitor:${IMAGE_TAG} \
  -t relay-pulse-monitor:latest \
  -t ${REGISTRY}/relay-pulse:${IMAGE_TAG} \
  -t ${REGISTRY}/relay-pulse:latest \
  .

echo ""
echo "✅ Docker 镜像构建完成"
echo "   本地镜像:"
echo "     - relay-pulse-monitor:${IMAGE_TAG}"
echo "     - relay-pulse-monitor:latest"
echo "   远程镜像标签:"
echo "     - ${REGISTRY}/relay-pulse:${IMAGE_TAG}"
echo "     - ${REGISTRY}/relay-pulse:latest"
echo ""
echo "镜像信息:"
echo "   Version: ${VERSION}"
echo "   Commit: ${GIT_COMMIT}"
echo "   Built: ${BUILD_TIME}"
echo ""

# 推送到远程仓库
if [ "$PUSH" = true ]; then
  echo "📤 推送镜像到 ${REGISTRY}..."
  docker push ${REGISTRY}/relay-pulse:${IMAGE_TAG}
  docker push ${REGISTRY}/relay-pulse:latest
  echo ""
  echo "✅ 镜像已推送到 GitHub Packages"
  echo "   查看: https://github.com/prehisle/relay-pulse/pkgs/container/relay-pulse"
else
  echo "💡 如需推送到 GitHub Packages，请运行:"
  echo "   $0 --push"
  echo ""
  echo "💡 推送前请先登录 GitHub Container Registry:"
  echo "   echo \$GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin"
fi

echo ""
echo "运行方式:"
echo "  docker run -p 8080:8080 -v ./config:/config relay-pulse-monitor:latest"
