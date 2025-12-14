#!/bin/bash
# 验证配置和环境变量是否正确工作

set -e  # 出错立即退出

echo "📋 加载 .env 文件..."
set -a
source .env
set +a

echo "✅ 环境变量已加载"
echo "📋 验证环境变量格式（显示前3个）:"
env | grep "^MONITOR_.*_API_KEY=" | head -3

echo ""
echo "📋 启动 monitor 程序（5秒后自动停止）..."
./monitor &
MONITOR_PID=$!

sleep 5

echo ""
echo "📋 停止 monitor 程序..."
kill $MONITOR_PID 2>/dev/null || true
wait $MONITOR_PID 2>/dev/null || true

echo "✅ 验证完成"
