#!/usr/bin/env bash
#
# SQLite → PostgreSQL 数据迁移脚本（生产级）
# 用于生产环境从 SQLite 迁移到 PostgreSQL
#
# 使用方法：
#   PGPASSWORD=xxx ./scripts/migrate-sqlite-to-postgres.sh [options] <sqlite_db_path> <postgres_dsn>
#
# 示例：
#   # 交互式（推荐）
#   PGPASSWORD=monitor123 ./scripts/migrate-sqlite-to-postgres.sh \
#     monitor.db "host=localhost port=5432 user=monitor dbname=monitor sslmode=disable"
#
#   # 自动化（CI/CD）
#   PGPASSWORD=monitor123 ./scripts/migrate-sqlite-to-postgres.sh --force \
#     monitor.db "host=localhost port=5432 user=monitor dbname=monitor sslmode=disable"
#
# 环境变量：
#   PGPASSWORD - PostgreSQL 密码（避免命令行暴露）
#

set -euo pipefail

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 日志函数
log_info() { echo -e "${GREEN}[INFO]${NC} $(date '+%H:%M:%S') $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $(date '+%H:%M:%S') $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $(date '+%H:%M:%S') $1"; }
log_step() { echo -e "${BLUE}[STEP]${NC} $(date '+%H:%M:%S') $1"; }

# 计时器
start_time=$(date +%s)
step_start_time=$start_time

step_timer() {
    local now=$(date +%s)
    local elapsed=$((now - step_start_time))
    log_info "⏱️  耗时: ${elapsed}s"
    step_start_time=$now
}

# 跨平台时间戳转换函数（兼容 macOS 和 Linux）
epoch_to_date() {
    local epoch="$1"
    if date --version &>/dev/null 2>&1; then
        # GNU date (Linux)
        date -d "@$epoch" '+%Y-%m-%d %H:%M:%S'
    else
        # BSD date (macOS)
        date -r "$epoch" '+%Y-%m-%d %H:%M:%S'
    fi
}

# 清理 psql 输出（移除所有空白字符）
trim_output() {
    tr -d '[:space:]'
}

# 参数解析
FORCE=0
while [[ $# -gt 0 ]]; do
    case $1 in
        --force)
            FORCE=1
            shift
            ;;
        --help|-h)
            cat <<EOF
用法: PGPASSWORD=xxx $0 [options] <sqlite_db_path> <postgres_dsn>

选项:
  --force         自动确认所有提示（用于自动化）
  --help, -h      显示此帮助信息

参数:
  sqlite_db_path  SQLite 数据库文件路径
  postgres_dsn    PostgreSQL 连接字符串（不含密码）

环境变量:
  PGPASSWORD      PostgreSQL 密码（必需）

示例:
  PGPASSWORD=monitor123 $0 monitor.db \\
    "host=localhost port=5432 user=monitor dbname=monitor sslmode=disable"

  PGPASSWORD=monitor123 $0 --force monitor.db \\
    "host=localhost port=5432 user=monitor dbname=monitor sslmode=disable"
EOF
            exit 0
            ;;
        -*)
            log_error "未知选项: $1"
            exit 1
            ;;
        *)
            break
            ;;
    esac
done

SQLITE_DB="${1:-}"
PG_CONN="${2:-}"

# 参数验证
if [[ -z "$SQLITE_DB" ]] || [[ -z "$PG_CONN" ]]; then
    log_error "缺少必需参数"
    echo "用法: PGPASSWORD=xxx $0 [--force] <sqlite_db_path> <postgres_dsn>"
    echo "使用 --help 查看详细帮助"
    exit 1
fi

if [[ -z "${PGPASSWORD:-}" ]]; then
    log_error "缺少环境变量 PGPASSWORD"
    echo "请设置 PostgreSQL 密码："
    echo "  export PGPASSWORD=your_password"
    echo "或在命令行前添加："
    echo "  PGPASSWORD=your_password $0 ..."
    exit 1
fi

if [[ ! -f "$SQLITE_DB" ]]; then
    log_error "SQLite 数据库文件不存在: $SQLITE_DB"
    exit 1
fi

# 检查依赖
for cmd in sqlite3 psql; do
    if ! command -v $cmd &> /dev/null; then
        log_error "缺少依赖: $cmd"
        exit 1
    fi
done

log_info "开始迁移: $SQLITE_DB → PostgreSQL"
log_info "强制模式: $([ $FORCE -eq 1 ] && echo '是' || echo '否')"
echo ""

# ============================================
# 步骤 1: 统计 SQLite 数据
# ============================================
log_step "【1/9】统计 SQLite 数据"
SQLITE_COUNT=$(sqlite3 "$SQLITE_DB" "SELECT COUNT(*) FROM probe_history;" 2>/dev/null || echo "0")
log_info "SQLite 记录数: $SQLITE_COUNT"

if [[ $SQLITE_COUNT -eq 0 ]]; then
    log_warn "SQLite 数据库为空，跳过迁移"
    exit 0
fi

# 获取 SQLite 数据范围
SQLITE_MIN_TS=$(sqlite3 "$SQLITE_DB" "SELECT MIN(timestamp) FROM probe_history;")
SQLITE_MAX_TS=$(sqlite3 "$SQLITE_DB" "SELECT MAX(timestamp) FROM probe_history;")
SQLITE_SUM_LATENCY=$(sqlite3 "$SQLITE_DB" "SELECT SUM(latency) FROM probe_history;")
log_info "时间范围: $(epoch_to_date $SQLITE_MIN_TS) ~ $(epoch_to_date $SQLITE_MAX_TS)"
log_info "延迟总和: ${SQLITE_SUM_LATENCY}ms (用于验证)"
step_timer
echo ""

# ============================================
# 步骤 2: 创建临时目录
# ============================================
log_step "【2/9】创建临时导出文件"
TEMP_DIR=$(mktemp -d)
EXPORT_FILE="$TEMP_DIR/probe_history_export.csv"
trap "rm -rf $TEMP_DIR" EXIT
log_info "临时目录: $TEMP_DIR"
step_timer
echo ""

# ============================================
# 步骤 3: 导出 SQLite 数据
# ============================================
log_step "【3/9】导出 SQLite 数据到 CSV"
sqlite3 "$SQLITE_DB" <<EOF
.mode csv
.headers off
.separator ","
.nullvalue ""
.output $EXPORT_FILE
SELECT id, provider, service, COALESCE(channel, ''), status, COALESCE(sub_status, ''), latency, timestamp
FROM probe_history
ORDER BY id;
.quit
EOF

# 验证导出文件
if [[ ! -f "$EXPORT_FILE" ]]; then
    log_error "导出文件创建失败"
    exit 1
fi

EXPORT_SIZE=$(du -h "$EXPORT_FILE" | cut -f1)
log_info "导出完成: $EXPORT_SIZE"
step_timer
echo ""

# ============================================
# 步骤 4: 检查 PostgreSQL 现有数据
# ============================================
log_step "【4/9】检查 PostgreSQL 现有数据"
PG_COUNT=$(psql "$PG_CONN" -v ON_ERROR_STOP=1 -t -c "SELECT COUNT(*) FROM probe_history;" | trim_output)
log_info "PostgreSQL 现有记录数: $PG_COUNT"

if [[ $PG_COUNT -gt 0 ]]; then
    log_warn "⚠️  PostgreSQL 已有 $PG_COUNT 条记录"
    if [[ $FORCE -eq 0 ]]; then
        read -p "是否清空 PostgreSQL 数据后重新导入? [y/N] " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log_error "取消迁移"
            exit 1
        fi
    else
        log_info "强制模式: 将清空 PostgreSQL 数据"
    fi
fi
step_timer
echo ""

# ============================================
# 步骤 5: 获取序列名（动态）
# ============================================
log_step "【5/9】获取 PostgreSQL 序列名"
SEQUENCE_NAME=$(psql "$PG_CONN" -v ON_ERROR_STOP=1 -t -c "SELECT pg_get_serial_sequence('probe_history', 'id');" | trim_output)
if [[ -z "$SEQUENCE_NAME" ]] || [[ "$SEQUENCE_NAME" == "null" ]]; then
    log_error "无法获取序列名，请检查表结构"
    exit 1
fi
log_info "序列名: $SEQUENCE_NAME"
step_timer
echo ""

# ============================================
# 步骤 6: 导入数据到 PostgreSQL（事务保护）
# ============================================
log_step "【6/9】导入数据到 PostgreSQL（事务保护）"
log_info "开始导入 $SQLITE_COUNT 条记录..."

psql "$PG_CONN" -v ON_ERROR_STOP=1 <<EOF
BEGIN;

-- 清空表（如果需要）
$([ $PG_COUNT -gt 0 ] && echo "TRUNCATE TABLE probe_history RESTART IDENTITY;")

-- 导入数据（明确 CSV 格式）
\copy probe_history (id, provider, service, channel, status, sub_status, latency, timestamp) FROM '$EXPORT_FILE' WITH (FORMAT csv, DELIMITER ',', QUOTE '"', ESCAPE '"', NULL '', HEADER false);

-- 重置序列
SELECT setval('$SEQUENCE_NAME', (SELECT COALESCE(MAX(id), 0) FROM probe_history));

-- 优化查询计划
ANALYZE probe_history;

COMMIT;
EOF

log_info "✅ 导入成功（事务已提交）"
step_timer
echo ""

# ============================================
# 步骤 7: 验证数据完整性
# ============================================
log_step "【7/9】验证数据完整性"

# 基础验证：行数
PG_COUNT_AFTER=$(psql "$PG_CONN" -v ON_ERROR_STOP=1 -t -c "SELECT COUNT(*) FROM probe_history;" | trim_output)
log_info "PostgreSQL 导入后记录数: $PG_COUNT_AFTER"

if [[ $PG_COUNT_AFTER -ne $SQLITE_COUNT ]]; then
    log_error "❌ 数据不完整! 期望 $SQLITE_COUNT 行，实际 $PG_COUNT_AFTER 行"
    exit 1
fi

# 高级验证：时间范围和延迟总和
PG_MIN_TS=$(psql "$PG_CONN" -v ON_ERROR_STOP=1 -t -c "SELECT MIN(timestamp) FROM probe_history;" | trim_output)
PG_MAX_TS=$(psql "$PG_CONN" -v ON_ERROR_STOP=1 -t -c "SELECT MAX(timestamp) FROM probe_history;" | trim_output)
PG_SUM_LATENCY=$(psql "$PG_CONN" -v ON_ERROR_STOP=1 -t -c "SELECT SUM(latency) FROM probe_history;" | trim_output)

log_info "时间范围一致性:"
log_info "  SQLite:     $SQLITE_MIN_TS ~ $SQLITE_MAX_TS"
log_info "  PostgreSQL: $PG_MIN_TS ~ $PG_MAX_TS"

if [[ "$SQLITE_MIN_TS" != "$PG_MIN_TS" ]] || [[ "$SQLITE_MAX_TS" != "$PG_MAX_TS" ]]; then
    log_error "❌ 时间范围不一致！数据可能损坏"
    exit 1
fi

log_info "延迟总和一致性:"
log_info "  SQLite:     ${SQLITE_SUM_LATENCY}ms"
log_info "  PostgreSQL: ${PG_SUM_LATENCY}ms"

if [[ "$SQLITE_SUM_LATENCY" != "$PG_SUM_LATENCY" ]]; then
    log_error "❌ 延迟总和不一致！数据可能损坏"
    exit 1
fi

log_info "✅ 数据完整性验证通过"
step_timer
echo ""

# ============================================
# 步骤 8: 验证序列
# ============================================
log_step "【8/9】验证序列设置"
CURRENT_SEQ=$(psql "$PG_CONN" -v ON_ERROR_STOP=1 -t -c "SELECT last_value FROM $SEQUENCE_NAME;" | trim_output)
MAX_ID=$(psql "$PG_CONN" -v ON_ERROR_STOP=1 -t -c "SELECT MAX(id) FROM probe_history;" | trim_output)

log_info "序列当前值: $CURRENT_SEQ"
log_info "表最大 ID:   $MAX_ID"

if [[ $CURRENT_SEQ -ne $MAX_ID ]]; then
    log_error "❌ 序列值不正确! 这会导致插入冲突"
    exit 1
fi

log_info "✅ 序列设置正确"
step_timer
echo ""

# ============================================
# 步骤 9: 采样验证
# ============================================
log_step "【9/9】采样验证（前 3 条记录）"
log_info "SQLite:"
sqlite3 "$SQLITE_DB" <<EOF
.mode column
.headers on
SELECT * FROM probe_history ORDER BY id LIMIT 3;
EOF

echo ""
log_info "PostgreSQL:"
psql "$PG_CONN" -v ON_ERROR_STOP=1 -c "SELECT * FROM probe_history ORDER BY id LIMIT 3;"

step_timer
echo ""

# ============================================
# 总结
# ============================================
total_time=$(($(date +%s) - start_time))
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
log_info "✅ 迁移成功完成!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "📊 迁移统计:"
echo "  • 记录数:   $SQLITE_COUNT"
echo "  • 文件大小: $EXPORT_SIZE"
echo "  • 总耗时:   ${total_time}s"
echo ""
echo "📝 下一步操作:"
echo ""
echo "  1️⃣  修改 config.yaml 或环境变量，切换到 PostgreSQL:"
echo ""
echo "     storage:"
echo "       type: postgres"
echo "       postgres:"
echo "         host: localhost"
echo "         port: 5432"
echo "         user: monitor"
echo "         dbname: monitor"
echo ""
echo "  2️⃣  重启服务:"
echo "     docker compose up -d monitor"
echo ""
echo "  3️⃣  验证服务正常运行:"
echo "     curl http://localhost:8080/api/status"
echo "     curl http://localhost:8080/api/version"
echo ""
echo "  4️⃣  监测 7 天后，归档 SQLite 文件:"
echo "     mkdir -p archive"
echo "     mv $SQLITE_DB archive/"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
