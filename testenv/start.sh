#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$SCRIPT_DIR"

# ── 颜色 ──
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; CYAN='\033[0;36m'; NC='\033[0m'
info()  { echo -e "${CYAN}[testenv]${NC} $*"; }
ok()    { echo -e "${GREEN}[testenv]${NC} $*"; }
warn()  { echo -e "${YELLOW}[testenv]${NC} $*"; }
err()   { echo -e "${RED}[testenv]${NC} $*" >&2; }

usage() {
  cat <<EOF
用法: $0 <command> [options]

命令:
  up              启动 PostgreSQL + 编译并运行 Monitor（前台，默认）
  up --sqlite     使用 SQLite 模式启动（无需 Docker）
  down            停止 PostgreSQL 容器
  down -v         停止 PostgreSQL 并删除数据卷
  pg              仅启动 PostgreSQL
  build           仅编译 Monitor 二进制
  status          查看运行状态
EOF
  exit 1
}

# ── 确保 templates/ 软链接存在 ──
ensure_templates_link() {
  if [ ! -e "$SCRIPT_DIR/templates" ]; then
    info "创建 templates/ 软链接 → $PROJECT_ROOT/templates"
    ln -s "$PROJECT_ROOT/templates" "$SCRIPT_DIR/templates"
  fi
}

# ── 解析 .env 为 env 命令参数 ──
# .env 含 bash 不合法的变量名（如带 . 的 key），不能用 source，
# 改为构建参数数组交给 env 命令，由 env 直接设入进程环境。
build_env_args() {
  ENV_ARGS=()
  local line key val
  while IFS= read -r line; do
    # 跳过空行和注释
    [[ -z "$line" || "$line" =~ ^[[:space:]]*# ]] && continue
    # 只取 KEY=VALUE 格式的行
    [[ "$line" =~ ^([A-Za-z_][A-Za-z0-9_.]*)[[:space:]]*=[[:space:]]*(.*) ]] || continue
    key="${BASH_REMATCH[1]}"
    val="${BASH_REMATCH[2]}"
    # 去除首尾引号
    val="${val#\"}" ; val="${val%\"}"
    val="${val#\'}" ; val="${val%\'}"
    # 展开 ${VAR} 引用（仅在已收集的变量中查找）
    while [[ "$val" =~ \$\{([A-Za-z_][A-Za-z0-9_]*)\} ]]; do
      local ref_key="${BASH_REMATCH[1]}"
      local ref_val=""
      for prev in "${ENV_ARGS[@]}"; do
        if [[ "$prev" == "${ref_key}="* ]]; then
          ref_val="${prev#*=}"
          break
        fi
      done
      val="${val/\$\{${ref_key}\}/${ref_val}}"
    done
    ENV_ARGS+=("${key}=${val}")
  done < "$SCRIPT_DIR/.env"
}

# ── PostgreSQL ──
pg_up() {
  info "启动 PostgreSQL …"
  docker compose up -d postgres 2>&1 | grep -v "is obsolete"
  info "等待 PostgreSQL 就绪 …"
  for i in $(seq 1 30); do
    if docker compose exec -T postgres pg_isready -U "${POSTGRES_USER:-relaypulse}" &>/dev/null; then
      ok "PostgreSQL 已就绪"
      return 0
    fi
    sleep 1
  done
  err "PostgreSQL 启动超时"; exit 1
}

pg_down() {
  info "停止 PostgreSQL …"
  docker compose down "$@" 2>&1 | grep -v "is obsolete"
  ok "已停止"
}

# ── 构建前端 ──
build_frontend() {
  info "构建前端 …"
  (cd "$PROJECT_ROOT/frontend" && npm run build)
  rm -rf "$PROJECT_ROOT/internal/api/frontend"
  cp -r "$PROJECT_ROOT/frontend" "$PROJECT_ROOT/internal/api/"
  ok "前端构建完成"
}

# ── 编译 ──
build_monitor() {
  build_frontend
  info "编译 Monitor → $SCRIPT_DIR/monitor"
  (cd "$PROJECT_ROOT" && go build -o "$SCRIPT_DIR/monitor" ./cmd/server)
  ok "编译完成"
}

# ── 启动（前台） ──
start_monitor() {
  info "启动 Monitor（Ctrl+C 停止）…"
  ok "Web UI:  http://localhost:8080"
  ok "API:     http://localhost:8080/api/status"
  ok "Health:  http://localhost:8080/health"
  echo ""
  exec env "${ENV_ARGS[@]}" "$SCRIPT_DIR/monitor" config.yaml
}

# ── up: PostgreSQL 模式 ──
cmd_up_pg() {
  ensure_templates_link
  pg_up
  build_monitor
  build_env_args
  # 覆盖 Docker 内部网络地址为 localhost
  ENV_ARGS+=("MONITOR_POSTGRES_HOST=localhost")
  ok "存储模式: PostgreSQL (localhost:5432)"
  start_monitor
}

# ── up --sqlite: SQLite 模式 ──
cmd_up_sqlite() {
  ensure_templates_link
  build_monitor
  build_env_args
  # 覆盖存储类型，移除 PG 相关变量
  local filtered=()
  for item in "${ENV_ARGS[@]}"; do
    case "$item" in
      MONITOR_POSTGRES_*|MONITOR_STORAGE_TYPE=*) ;;
      *) filtered+=("$item") ;;
    esac
  done
  ENV_ARGS=("${filtered[@]}" "MONITOR_STORAGE_TYPE=sqlite")
  ok "存储模式: SQLite (testenv/monitor.db)"
  start_monitor
}

cmd_status() {
  echo ""
  info "PostgreSQL:"
  docker compose ps postgres 2>&1 | grep -v "is obsolete" || warn "Docker Compose 未运行"
  echo ""
  info "Monitor 进程:"
  pgrep -fl "$SCRIPT_DIR/monitor" || warn "Monitor 未运行"
  echo ""
  info "端口占用:"
  lsof -i :5432 2>/dev/null | head -3 || warn "5432 未占用"
  lsof -i :8080 2>/dev/null | head -3 || warn "8080 未占用"
  echo ""
  info "SQLite 数据库:"
  if [ -f "$SCRIPT_DIR/monitor.db" ]; then
    ok "monitor.db ($(du -h "$SCRIPT_DIR/monitor.db" | cut -f1))"
  else
    warn "不存在"
  fi
}

# ── 入口 ──
[[ $# -lt 1 ]] && usage

case "$1" in
  up)
    if [[ "${2:-}" == "--sqlite" ]]; then
      cmd_up_sqlite
    else
      cmd_up_pg
    fi
    ;;
  down)    shift; pg_down "$@" ;;
  pg)      pg_up ;;
  build)   build_monitor ;;
  status)  cmd_status ;;
  *)       usage ;;
esac
