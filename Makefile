# Makefile for LLM Monitor Service

# Go parameters
GOCMD=go
GOTEST=$(GOCMD) test

# Air hot reload tool
# 优先使用 PATH 中的 air，如果没有则尝试 Go bin 目录
AIR_CMD=$(shell command -v air 2>/dev/null || echo "$(shell go env GOPATH)/bin/air")

# 开发环境 CORS 配置（允许前端开发服务器访问）
MONITOR_CORS_ORIGINS ?= http://localhost:5173,http://127.0.0.1:5173,http://localhost:5174,http://127.0.0.1:5174,http://localhost:5175,http://127.0.0.1:5175,http://localhost:3000

# ============================================
# 本地 PostgreSQL 开发环境配置
# ============================================
PG_CONTAINER_NAME ?= relay-pulse-pg
PG_PORT ?= 5433
PG_USER ?= relaypulse
PG_PASSWORD ?= relaypulse123
PG_DATABASE ?= relaypulse
PG_IMAGE ?= postgres:15-alpine

# 备份目录
PG_BACKUP_DIR ?= .bak
PG_BACKUP_FILE ?= $(PG_BACKUP_DIR)/pg_backup_$(shell date +%Y%m%d_%H%M%S).dump

.PHONY: help dev dev-pg test ci
.PHONY: pg-up pg-down pg-status pg-shell pg-backup pg-restore pg-reset

# 默认目标：显示帮助
help:
	@echo "可用命令:"
	@echo ""
	@echo "  开发模式:"
	@echo "  make dev           - 后端开发模式（SQLite，热重载）"
	@echo "  make dev-pg        - 后端开发模式（本地 PG，热重载）"
	@echo ""
	@echo "  本地 PostgreSQL:"
	@echo "  make pg-up         - 启动本地 PG 容器 (端口 $(PG_PORT))"
	@echo "  make pg-down       - 停止本地 PG 容器"
	@echo "  make pg-status     - 查看 PG 容器状态"
	@echo "  make pg-shell      - 进入 PG 命令行"
	@echo "  make pg-backup     - 备份数据库到 $(PG_BACKUP_DIR)/"
	@echo "  make pg-restore F=<file> - 从备份恢复数据（需确认）"
	@echo "  make pg-reset      - 重置数据库（删除所有数据，需确认）"
	@echo ""
	@echo "  测试与质量:"
	@echo "  make test          - 运行测试"
	@echo "  make ci            - 本地模拟 CI 检查（lint + test）"
	@echo ""
	@echo "  本地 PG 配置: 容器=$(PG_CONTAINER_NAME) 端口=$(PG_PORT) 用户=$(PG_USER)"

# 开发模式（热重载）
dev:
	@if [ ! -f "$(AIR_CMD)" ] && [ -z "$$(command -v air 2>/dev/null)" ]; then \
		echo "错误: air 未安装"; \
		echo ""; \
		echo "请运行以下命令安装:"; \
		echo "  go install github.com/air-verse/air@latest"; \
		exit 1; \
	fi
	@echo "正在启动开发服务（热重载）..."
	@echo "修改 .go 文件将自动重新编译"
	@if [ -f .env ]; then \
		echo "📋 加载 .env 文件..."; \
		set -a && . ./.env && set +a && \
		MONITOR_STORAGE_TYPE=sqlite \
		MONITOR_CORS_ORIGINS="$(MONITOR_CORS_ORIGINS)" $(AIR_CMD) -c .air.toml; \
	else \
		echo "⚠️  未找到 .env 文件，API Keys 可能无法加载"; \
		MONITOR_CORS_ORIGINS="$(MONITOR_CORS_ORIGINS)" $(AIR_CMD) -c .air.toml; \
	fi

# 运行测试
test:
	@echo "正在运行测试..."
	$(GOTEST) -v ./...

# CI 检查（本地模拟 GitHub Actions 流程）
ci:
	@echo "=========================================="
	@echo "  本地 CI 检查"
	@echo "=========================================="
	@echo ""
	@echo ">> Go 格式检查..."
	@if [ -n "$$(gofmt -l .)" ]; then \
		echo "以下文件格式不正确:"; \
		gofmt -l .; \
		exit 1; \
	fi
	@echo "✓ Go 格式检查通过"
	@echo ""
	@echo ">> Go vet..."
	$(GOCMD) vet ./...
	@echo "✓ Go vet 通过"
	@echo ""
	@echo ">> Go 测试..."
	$(GOTEST) -v ./...
	@echo "✓ Go 测试通过"
	@echo ""
	@echo ">> 前端依赖安装..."
	@cd frontend && npm ci
	@echo ">> 前端 lint..."
	@cd frontend && npm run lint
	@echo "✓ 前端 lint 通过"
	@echo ""
	@echo ">> 前端测试..."
	@cd frontend && npm run test -- --run
	@echo "✓ 前端测试通过"
	@echo ""
	@echo "=========================================="
	@echo "  CI 检查完成"
	@echo "=========================================="

# ============================================
# 本地 PostgreSQL 开发环境命令
# ============================================
# 注意：本地 PG 仅绑定 127.0.0.1，不暴露到外网
# 如需修改 PG_PORT/PG_USER/PG_PASSWORD，需先 make pg-reset 再 make pg-up

# 启动本地 PostgreSQL 容器
pg-up:
	@echo "正在启动本地 PostgreSQL 容器..."
	@if docker ps -a --format '{{.Names}}' | grep -q "^$(PG_CONTAINER_NAME)$$"; then \
		if docker ps --format '{{.Names}}' | grep -q "^$(PG_CONTAINER_NAME)$$"; then \
			echo "✓ 容器 $(PG_CONTAINER_NAME) 已在运行"; \
			echo ""; \
			echo "实际端口映射:"; \
			docker port $(PG_CONTAINER_NAME); \
		else \
			echo "启动已存在的容器..."; \
			docker start $(PG_CONTAINER_NAME); \
			echo "等待 PostgreSQL 就绪..."; \
			for i in 1 2 3 4 5 6 7 8 9 10; do \
				if docker exec $(PG_CONTAINER_NAME) pg_isready -U $(PG_USER) >/dev/null 2>&1; then \
					echo "✓ 容器已启动"; \
					break; \
				fi; \
				sleep 1; \
			done; \
		fi \
	else \
		echo "创建新容器（仅绑定 127.0.0.1）..."; \
		docker run -d \
			--name $(PG_CONTAINER_NAME) \
			-e POSTGRES_USER=$(PG_USER) \
			-e POSTGRES_PASSWORD=$(PG_PASSWORD) \
			-e POSTGRES_DB=$(PG_DATABASE) \
			-p 127.0.0.1:$(PG_PORT):5432 \
			$(PG_IMAGE); \
		echo "等待 PostgreSQL 就绪..."; \
		for i in 1 2 3 4 5 6 7 8 9 10; do \
			if docker exec $(PG_CONTAINER_NAME) pg_isready -U $(PG_USER) >/dev/null 2>&1; then \
				echo "✓ 容器创建完成"; \
				break; \
			fi; \
			sleep 1; \
		done; \
	fi
	@echo ""
	@echo "连接信息:"
	@echo "  Host: 127.0.0.1 (仅本机可访问)"
	@echo "  Port: $(PG_PORT)"
	@echo "  User: $(PG_USER)"
	@echo "  Database: $(PG_DATABASE)"

# 停止本地 PostgreSQL 容器
pg-down:
	@echo "正在停止本地 PostgreSQL 容器..."
	@if docker ps --format '{{.Names}}' | grep -q "^$(PG_CONTAINER_NAME)$$"; then \
		docker stop $(PG_CONTAINER_NAME); \
		echo "✓ 容器已停止"; \
	else \
		echo "容器未在运行"; \
	fi

# 查看 PostgreSQL 容器状态
pg-status:
	@echo "PostgreSQL 容器状态:"
	@if docker ps -a --format '{{.Names}}' | grep -q "^$(PG_CONTAINER_NAME)$$"; then \
		docker ps -a --filter "name=$(PG_CONTAINER_NAME)" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"; \
	else \
		echo "  容器不存在，运行 'make pg-up' 创建"; \
	fi

# 进入 PostgreSQL 命令行
pg-shell:
	@if docker ps --format '{{.Names}}' | grep -q "^$(PG_CONTAINER_NAME)$$"; then \
		docker exec -it $(PG_CONTAINER_NAME) psql -U $(PG_USER) -d $(PG_DATABASE); \
	else \
		echo "错误: 容器未运行，请先执行 'make pg-up'"; \
		exit 1; \
	fi

# 备份数据库
pg-backup:
	@if ! docker ps --format '{{.Names}}' | grep -q "^$(PG_CONTAINER_NAME)$$"; then \
		echo "错误: 容器未运行，请先执行 'make pg-up'"; \
		exit 1; \
	fi
	@mkdir -p $(PG_BACKUP_DIR)
	@echo "正在备份数据库..."
	@docker exec $(PG_CONTAINER_NAME) pg_dump -U $(PG_USER) -d $(PG_DATABASE) -Fc > $(PG_BACKUP_FILE)
	@echo "✓ 备份完成: $(PG_BACKUP_FILE)"
	@echo ""
	@echo "现有备份:"
	@ls -lh $(PG_BACKUP_DIR)/pg_backup_*.dump 2>/dev/null || echo "  (无)"

# 从备份恢复数据（需要确认）
pg-restore:
	@if [ -z "$(F)" ]; then \
		echo "用法: make pg-restore F=<备份文件路径>"; \
		echo ""; \
		echo "现有备份:"; \
		ls -lh $(PG_BACKUP_DIR)/pg_backup_*.dump 2>/dev/null || echo "  (无)"; \
		exit 1; \
	fi
	@if [ ! -f "$(F)" ]; then \
		echo "错误: 备份文件不存在: $(F)"; \
		exit 1; \
	fi
	@if ! docker ps --format '{{.Names}}' | grep -q "^$(PG_CONTAINER_NAME)$$"; then \
		echo "错误: 容器未运行，请先执行 'make pg-up'"; \
		exit 1; \
	fi
	@echo "=========================================="
	@echo "  ⚠️  数据恢复确认"
	@echo "=========================================="
	@echo ""
	@echo "将从以下备份恢复数据:"
	@echo "  $(F)"
	@echo ""
	@echo "这将覆盖本地数据库中的现有数据！"
	@echo ""
	@printf "确认恢复? [y/N] " && read confirm && [ "$$confirm" = "y" ] || (echo "已取消"; exit 1)
	@echo "正在恢复数据..."
	@if docker exec -i $(PG_CONTAINER_NAME) pg_restore -U $(PG_USER) -d $(PG_DATABASE) --clean --if-exists --no-owner --no-privileges < $(F) 2>&1 | grep -v "^pg_restore: warning:" | grep -i "error" ; then \
		echo ""; \
		echo "⚠️  恢复过程中出现错误，请检查上方日志"; \
	else \
		echo ""; \
		echo "✓ 数据恢复完成"; \
	fi
	@echo ""
	@echo "数据表概览:"
	@docker exec $(PG_CONTAINER_NAME) psql -U $(PG_USER) -d $(PG_DATABASE) -c "\dt" || true

# 重置数据库（删除所有数据，需要确认）
pg-reset:
	@echo "=========================================="
	@echo "  ⚠️  数据库重置确认"
	@echo "=========================================="
	@echo ""
	@echo "这将删除本地 PostgreSQL 容器及其所有数据！"
	@echo ""
	@printf "确认重置? [y/N] " && read confirm && [ "$$confirm" = "y" ] || (echo "已取消"; exit 1)
	@if docker ps -a --format '{{.Names}}' | grep -q "^$(PG_CONTAINER_NAME)$$"; then \
		echo "停止并删除容器..."; \
		docker rm -f $(PG_CONTAINER_NAME); \
		echo "✓ 容器已删除"; \
	else \
		echo "容器不存在"; \
	fi
	@echo ""
	@echo "如需重新创建，请运行: make pg-up"

# 后端开发模式（本地 PostgreSQL）
dev-pg:
	@if [ ! -f "$(AIR_CMD)" ] && [ -z "$$(command -v air 2>/dev/null)" ]; then \
		echo "错误: air 未安装，请运行 'go install github.com/air-verse/air@latest'"; \
		exit 1; \
	fi
	@if ! docker ps --format '{{.Names}}' | grep -q "^$(PG_CONTAINER_NAME)$$"; then \
		echo "错误: 本地 PG 容器未运行，请先执行 'make pg-up'"; \
		exit 1; \
	fi
	@echo "=========================================="
	@echo "  本地 PostgreSQL 开发模式"
	@echo "=========================================="
	@echo ""
	@echo "📋 加载配置: .env + PG 覆盖"
	@echo ""
	@if [ -f .env ]; then \
		set -a && . ./.env && set +a; \
	fi && \
	MONITOR_STORAGE_TYPE=postgres \
	MONITOR_POSTGRES_HOST=localhost MONITOR_POSTGRES_PORT=$(PG_PORT) \
	GIN_MODE=debug \
	MONITOR_CORS_ORIGINS="$(MONITOR_CORS_ORIGINS)" $(AIR_CMD) -c .air.toml
