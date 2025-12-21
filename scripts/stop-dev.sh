#!/bin/bash
# 停止开发环境服务
# 使用方式: ./scripts/stop-dev.sh 或 make stop

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

# 停止进程的函数
stop_process() {
    local pid_file="$1"
    local name="$2"
    local color="$3"
    local stopped=false

    if [ -f "$pid_file" ]; then
        PID=$(cat "$pid_file")
        if kill -0 "$PID" 2>/dev/null; then
            printf "${color}[$name]${NC} 正在停止 (PID: $PID)...\n"
            kill -TERM "$PID" 2>/dev/null

            # 等待进程退出（最多 5 秒）
            for i in {1..10}; do
                if ! kill -0 "$PID" 2>/dev/null; then
                    printf "${color}[$name]${NC} 已停止\n"
                    rm -f "$pid_file"
                    stopped=true
                    break
                fi
                sleep 0.5
            done

            # 强制终止
            if [ "$stopped" = false ]; then
                printf "${YELLOW}[$name]${NC} 强制终止...\n"
                kill -KILL "$PID" 2>/dev/null || true
                rm -f "$pid_file"
                stopped=true
            fi
        else
            printf "${color}[$name]${NC} 进程已不存在，清理 PID 文件\n"
            rm -f "$pid_file"
        fi
    fi

    echo "$stopped"
}

# 通过端口停止（兜底方案）
stop_by_port() {
    local port="$1"
    local name="$2"

    # 检查 lsof 是否可用
    if ! command -v lsof &>/dev/null; then
        return 1
    fi

    local pids=$(lsof -ti ":$port" 2>/dev/null || true)
    if [ -n "$pids" ]; then
        printf "${YELLOW}通过端口 $port 清理残留 $name 进程...${NC}\n"
        echo "$pids" | xargs kill -TERM 2>/dev/null || true
        sleep 1
        # 检查是否还有残留
        pids=$(lsof -ti ":$port" 2>/dev/null || true)
        if [ -n "$pids" ]; then
            echo "$pids" | xargs kill -KILL 2>/dev/null || true
        fi
        return 0
    fi
    return 1
}

# 清理 FIFO 文件
cleanup_fifos() {
    rm -f "$PID_DIR/backend.fifo" "$PID_DIR/frontend.fifo" 2>/dev/null || true
}

main() {
    printf "${GREEN}========================================${NC}\n"
    printf "${GREEN}  停止开发服务${NC}\n"
    printf "${GREEN}========================================${NC}\n"
    echo ""

    local stopped_any=false

    # 1. 尝试通过 PID 文件停止
    if [ -f "$BACKEND_PID_FILE" ]; then
        result=$(stop_process "$BACKEND_PID_FILE" "api" "$GREEN")
        if [ "$result" = "true" ]; then
            stopped_any=true
        fi
    fi

    if [ -f "$FRONTEND_PID_FILE" ]; then
        result=$(stop_process "$FRONTEND_PID_FILE" "web" "$CYAN")
        if [ "$result" = "true" ]; then
            stopped_any=true
        fi
    fi

    # 2. 兜底：通过端口清理残留进程
    if stop_by_port 8080 "后端"; then
        stopped_any=true
    fi
    if stop_by_port 5173 "前端"; then
        stopped_any=true
    fi
    if stop_by_port 5174 "前端备用"; then
        stopped_any=true
    fi

    # 3. 清理 air 的临时文件和 FIFO
    if [ -d "tmp/air" ]; then
        rm -rf tmp/air
    fi
    cleanup_fifos

    echo ""
    if [ "$stopped_any" = true ]; then
        printf "${GREEN}开发服务已停止${NC}\n"
    else
        printf "${YELLOW}没有发现运行中的开发服务${NC}\n"
    fi
}

main "$@"
