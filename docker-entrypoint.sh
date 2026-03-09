#!/bin/bash
set -e

# ============================================
# Docker Entrypoint Script
# 处理配置文件挂载逻辑和环境变量
# ============================================

CONFIG_FILE="/app/config.yaml"
MOUNTED_CONFIG="/config/config.yaml"
DEFAULT_CONFIG="/app/config.yaml.default"
TEMPLATES_DIR="/app/templates"
CONFIG_TEMPLATES_LINK="/config/templates"
ACTIVE_CONFIG="$CONFIG_FILE"

echo "[Entrypoint] 初始化监测服务..."

# 检查是否挂载了外部配置文件
if [ -f "$MOUNTED_CONFIG" ]; then
    echo "[Entrypoint] 检测到外部配置文件: $MOUNTED_CONFIG"
    echo "[Entrypoint] 使用挂载的配置文件(直接传递给服务以支持热重载)"
    ACTIVE_CONFIG="$MOUNTED_CONFIG"
    # 为配置文件中的 !include templates/xxx.json 创建软链接
    if [ ! -e "$CONFIG_TEMPLATES_LINK" ]; then
        ln -sfn "$TEMPLATES_DIR" "$CONFIG_TEMPLATES_LINK" 2>/dev/null || {
            echo "[Entrypoint] ⚠️ 无法创建 templates 软链接（配置目录可能为只读挂载）"
            echo "[Entrypoint]   如需 !include 功能，请移除配置目录的 :ro 标志或额外挂载: -v ./templates:/config/templates:ro"
        }
    fi
elif [ -f "$CONFIG_FILE" ]; then
    echo "[Entrypoint] 使用容器内配置文件: $CONFIG_FILE"
else
    echo "[Entrypoint] 未找到配置文件,使用默认配置"
    cp "$DEFAULT_CONFIG" "$CONFIG_FILE"
fi

# 打印环境变量配置提示
echo "[Entrypoint] 支持通过环境变量覆盖 API 密钥:"
echo "  格式: MONITOR_<PROVIDER>_<SERVICE>_API_KEY=sk-xxx"
echo "  示例: MONITOR_88CODE_CC_API_KEY=sk-real-key"

# 检查是否设置了环境变量覆盖
env | grep '^MONITOR_' > /dev/null && {
    echo "[Entrypoint] 检测到 MONITOR_* 环境变量,将覆盖配置文件中的 API 密钥"
} || {
    echo "[Entrypoint] 未检测到 MONITOR_* 环境变量"
}

echo "[Entrypoint] 启动监测服务..."
echo "----------------------------------------"

# 执行主程序（main.go 通过 os.Args[1] 读取配置文件路径，不需要 -config 标志）
exec /app/monitor "$ACTIVE_CONFIG"
