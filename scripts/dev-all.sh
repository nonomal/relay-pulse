#!/bin/bash
# 一键启动前后端开发环境（热重载）
# 使用方式: ./scripts/dev-all.sh 或 make dev-all

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# PID 文件目录
PID_DIR=".dev"
BACKEND_PID_FILE="$PID_DIR/backend.pid"
FRONTEND_PID_FILE="$PID_DIR/frontend.pid"
BACKEND_FIFO="$PID_DIR/backend.fifo"
FRONTEND_FIFO="$PID_DIR/frontend.fifo"

# 确保目录存在
mkdir -p "$PID_DIR"

# 清理函数
cleanup() {
    # 防止重复清理
    trap - INT TERM EXIT

    echo ""
    printf "${YELLOW}正在停止开发服务...${NC}\n"

    # 停止后端
    if [ -f "$BACKEND_PID_FILE" ]; then
        PID=$(cat "$BACKEND_PID_FILE")
        if kill -0 "$PID" 2>/dev/null; then
            kill -TERM "$PID" 2>/dev/null || true
            # 等待进程退出
            for i in 1 2 3 4 5 6 7 8 9 10; do
                if ! kill -0 "$PID" 2>/dev/null; then
                    break
                fi
                sleep 0.3
            done
            # 强制杀死
            if kill -0 "$PID" 2>/dev/null; then
                kill -KILL "$PID" 2>/dev/null || true
            fi
            printf "${GREEN}[api]${NC} 已停止 (PID: $PID)\n"
        fi
        rm -f "$BACKEND_PID_FILE"
    fi

    # 停止前端
    if [ -f "$FRONTEND_PID_FILE" ]; then
        PID=$(cat "$FRONTEND_PID_FILE")
        if kill -0 "$PID" 2>/dev/null; then
            kill -TERM "$PID" 2>/dev/null || true
            for i in 1 2 3 4 5 6 7 8 9 10; do
                if ! kill -0 "$PID" 2>/dev/null; then
                    break
                fi
                sleep 0.3
            done
            if kill -0 "$PID" 2>/dev/null; then
                kill -KILL "$PID" 2>/dev/null || true
            fi
            printf "${CYAN}[web]${NC} 已停止 (PID: $PID)\n"
        fi
        rm -f "$FRONTEND_PID_FILE"
    fi

    # 清理 FIFO 文件
    rm -f "$BACKEND_FIFO" "$FRONTEND_FIFO" 2>/dev/null || true

    # 兜底：通过端口清理
    for port in 8080 5173 5174; do
        if command -v lsof &>/dev/null; then
            pids=$(lsof -ti ":$port" 2>/dev/null || true)
            if [ -n "$pids" ]; then
                printf "${YELLOW}清理端口 $port 残留进程...${NC}\n"
                echo "$pids" | xargs kill -TERM 2>/dev/null || true
                sleep 0.5
                pids=$(lsof -ti ":$port" 2>/dev/null || true)
                if [ -n "$pids" ]; then
                    echo "$pids" | xargs kill -KILL 2>/dev/null || true
                fi
            fi
        fi
    done

    printf "${GREEN}开发服务已全部停止${NC}\n"
}

# 捕获信号
trap cleanup INT TERM

# 检查是否已有服务运行（清理 stale PID 文件）
check_existing() {
    local has_existing=false

    if [ -f "$BACKEND_PID_FILE" ]; then
        PID=$(cat "$BACKEND_PID_FILE")
        if kill -0 "$PID" 2>/dev/null; then
            printf "${YELLOW}警告: 后端服务已在运行 (PID: $PID)${NC}\n"
            has_existing=true
        else
            # stale PID 文件，删除
            rm -f "$BACKEND_PID_FILE"
        fi
    fi

    if [ -f "$FRONTEND_PID_FILE" ]; then
        PID=$(cat "$FRONTEND_PID_FILE")
        if kill -0 "$PID" 2>/dev/null; then
            printf "${YELLOW}警告: 前端服务已在运行 (PID: $PID)${NC}\n"
            has_existing=true
        else
            rm -f "$FRONTEND_PID_FILE"
        fi
    fi

    if [ "$has_existing" = true ]; then
        printf "${YELLOW}请先运行 'make stop' 停止现有服务${NC}\n"
        exit 1
    fi
}

# 检查依赖
check_dependencies() {
    # 检查 air
    AIR_CMD=""
    if command -v air &>/dev/null; then
        AIR_CMD="air"
    elif [ -f "$(go env GOPATH)/bin/air" ]; then
        AIR_CMD="$(go env GOPATH)/bin/air"
    else
        printf "${RED}错误: air 未安装${NC}\n"
        echo "请运行: make install-air"
        exit 1
    fi

    # 检查 node_modules
    if [ ! -d "frontend/node_modules" ]; then
        printf "${YELLOW}前端依赖未安装，正在安装...${NC}\n"
        npm --prefix frontend install
    fi
}

# 日志前缀输出函数
prefix_output() {
    local prefix="$1"
    local color="$2"
    while IFS= read -r line || [ -n "$line" ]; do
        printf "${color}${prefix}${NC} %s\n" "$line"
    done
}

# 主函数
main() {
    printf "${GREEN}========================================${NC}\n"
    printf "${GREEN}  RelayPulse 开发环境${NC}\n"
    printf "${GREEN}========================================${NC}\n"
    echo ""

    check_existing
    check_dependencies

    # 加载 .env 文件
    if [ -f .env ]; then
        printf "${GREEN}加载 .env 文件...${NC}\n"
        set -a
        source .env
        set +a
    fi

    # 设置 CORS
    export MONITOR_CORS_ORIGINS="${MONITOR_CORS_ORIGINS:-http://localhost:5173,http://127.0.0.1:5173,http://localhost:5174,http://127.0.0.1:5174,http://localhost:5175,http://127.0.0.1:5175,http://localhost:3000}"

    echo ""
    printf "${GREEN}[api]${NC} 启动后端 (http://localhost:8080)...\n"
    printf "${CYAN}[web]${NC} 启动前端 (http://localhost:5173)...\n"
    echo ""
    printf "${YELLOW}按 Ctrl+C 停止所有服务${NC}\n"
    echo ""

    # 创建 tmp 目录
    mkdir -p tmp/air

    # 清理旧的 FIFO
    rm -f "$BACKEND_FIFO" "$FRONTEND_FIFO"

    # 创建 FIFO
    mkfifo "$BACKEND_FIFO"
    mkfifo "$FRONTEND_FIFO"

    # 启动后端（使用 exec 确保 $! 是真实进程 PID）
    (cd . && exec $AIR_CMD -c .air.toml) > "$BACKEND_FIFO" 2>&1 &
    BACKEND_PID=$!
    echo "$BACKEND_PID" > "$BACKEND_PID_FILE"

    # 异步读取后端日志
    prefix_output "[api]" "$GREEN" < "$BACKEND_FIFO" &

    # 启动前端（使用 exec 确保 $! 是真实进程 PID）
    (cd frontend && exec npm run dev) > "$FRONTEND_FIFO" 2>&1 &
    FRONTEND_PID=$!
    echo "$FRONTEND_PID" > "$FRONTEND_PID_FILE"

    # 异步读取前端日志
    prefix_output "[web]" "$CYAN" < "$FRONTEND_FIFO" &

    # 轮询检查进程状态（兼容 macOS bash 3.2，不使用 wait -n）
    while true; do
        # 检查后端进程
        if ! kill -0 "$BACKEND_PID" 2>/dev/null; then
            printf "${RED}[api]${NC} 后端进程异常退出\n"
            break
        fi

        # 检查前端进程
        if ! kill -0 "$FRONTEND_PID" 2>/dev/null; then
            printf "${RED}[web]${NC} 前端进程异常退出\n"
            break
        fi

        sleep 1
    done

    # 如果到这里说明有进程退出了，触发清理
    cleanup
}

main "$@"
