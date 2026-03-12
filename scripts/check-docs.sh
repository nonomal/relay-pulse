#!/bin/bash
# 文档同步检查脚本
# 确保代码变更与文档保持同步

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "检查文档同步状态..."

# 检查 README.md 是否存在
if [ ! -f "README.md" ]; then
    echo -e "${RED}错误: README.md 不存在${NC}"
    exit 1
fi

# 检查 API 文档是否包含所有端点
echo "检查 API 端点文档..."
API_ENDPOINTS=("/api/status" "/health")
for endpoint in "${API_ENDPOINTS[@]}"; do
    if ! grep -q "$endpoint" README.md; then
        echo -e "${YELLOW}警告: README.md 未包含端点 $endpoint 的文档${NC}"
    fi
done

# 检查配置示例是否与实际配置结构匹配
echo "检查配置文档..."
if [ -f "config.yaml.example" ]; then
    # 检查关键配置项
    REQUIRED_FIELDS=("provider" "service" "url" "api_key")
    for field in "${REQUIRED_FIELDS[@]}"; do
        if ! grep -q "$field" config.yaml.example; then
            echo -e "${RED}错误: config.yaml.example 缺少必要字段 $field${NC}"
            exit 1
        fi
    done
fi

# 检查代码中的新功能是否有对应文档
echo "检查功能文档..."

# 检查热更新功能文档
if grep -r "TriggerNow\|UpdateConfig" internal/ > /dev/null 2>&1; then
    if ! grep -qi "热更新\|hot.reload" README.md; then
        echo -e "${YELLOW}警告: 代码包含热更新功能，但 README.md 可能缺少相关文档${NC}"
    fi
fi

# 检查存储功能文档
if grep -r "SQLiteStorage" internal/ > /dev/null 2>&1; then
    if ! grep -qi "sqlite\|数据库\|database" README.md; then
        echo -e "${YELLOW}警告: 代码使用 SQLite 存储，但 README.md 可能缺少相关文档${NC}"
    fi
fi

echo -e "${GREEN}文档检查完成${NC}"
exit 0
