# Makefile for LLM Monitor Service

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt

# Binary name
BINARY_NAME=monitor
BINARY_PATH=./$(BINARY_NAME)

# Main package
MAIN_PACKAGE=./cmd/server

# Build script
BUILD_SCRIPT=./scripts/build.sh
DOCKER_BUILD_SCRIPT=./scripts/docker-build.sh

# Air hot reload tool
# 优先使用 PATH 中的 air，如果没有则尝试 Go bin 目录
AIR_CMD=$(shell command -v air 2>/dev/null || echo "$(shell go env GOPATH)/bin/air")

# 配置文件（用于 run 命令）
# 注意：make dev 使用 air，配置文件固定为 config.yaml
# 如需自定义配置文件，请使用 make run 或直接运行 air
CONFIG ?= config.yaml

# 开发环境 CORS 配置（允许前端开发服务器访问）
MONITOR_CORS_ORIGINS ?= http://localhost:5173,http://127.0.0.1:5173,http://localhost:5174,http://127.0.0.1:5174,http://localhost:5175,http://127.0.0.1:5175,http://localhost:3000

.PHONY: help build run dev test fmt clean install-air release docker-build ci

# 默认目标：显示帮助
help:
	@echo "可用命令:"
	@echo "  make build         - 编译生产版本（注入版本信息）"
	@echo "  make release       - 发布构建（需指定 VERSION=vX.Y.Z）"
	@echo "  make docker-build  - 构建 Docker 镜像"
	@echo "  make run           - 直接运行（无热重载）"
	@echo "  make dev           - 开发模式（热重载，需要air）"
	@echo "  make test          - 运行测试"
	@echo "  make ci            - 本地模拟 CI 检查（lint + test）"
	@echo "  make fmt           - 格式化代码"
	@echo "  make clean         - 清理编译产物"
	@echo "  make install-air   - 安装air热重载工具"
	@echo ""
	@echo "开发环境已自动配置 CORS，允许前端开发服务器访问（端口 5173-5175, 3000）"

# 编译二进制（使用构建脚本，自动注入版本信息）
build:
	@if [ -f "$(BUILD_SCRIPT)" ]; then \
		bash $(BUILD_SCRIPT); \
	else \
		echo "警告: $(BUILD_SCRIPT) 不存在，使用简单构建"; \
		$(GOBUILD) -o $(BINARY_PATH) $(MAIN_PACKAGE); \
	fi

# 发布构建（需要指定版本号）
release:
	@if [ -z "$(VERSION)" ]; then \
		echo "错误: 请指定版本号"; \
		echo "用法: make release VERSION=v1.0.0"; \
		exit 1; \
	fi
	@echo "正在构建发布版本 $(VERSION)..."
	@VERSION=$(VERSION) bash $(BUILD_SCRIPT)

# 构建 Docker 镜像
docker-build:
	@if [ -f "$(DOCKER_BUILD_SCRIPT)" ]; then \
		bash $(DOCKER_BUILD_SCRIPT); \
	else \
		echo "错误: $(DOCKER_BUILD_SCRIPT) 不存在"; \
		exit 1; \
	fi

# 直接运行（不编译）
run:
	@echo "正在启动监测服务..."
	MONITOR_CORS_ORIGINS="$(MONITOR_CORS_ORIGINS)" $(GORUN) $(MAIN_PACKAGE)

# 开发模式（热重载）
dev:
	@if [ ! -f "$(AIR_CMD)" ] && [ -z "$$(command -v air 2>/dev/null)" ]; then \
		echo "错误: air 未安装"; \
		echo ""; \
		echo "请运行以下命令安装:"; \
		echo "  make install-air"; \
		echo ""; \
		echo "或手动安装:"; \
		echo "  go install github.com/air-verse/air@latest"; \
		exit 1; \
	fi
	@echo "正在启动开发服务（热重载）..."
	@echo "修改 .go 文件将自动重新编译"
	@if [ -f .env ]; then \
		echo "📋 加载 .env 文件..."; \
		set -a && . ./.env && set +a && \
		MONITOR_CORS_ORIGINS="$(MONITOR_CORS_ORIGINS)" $(AIR_CMD) -c .air.toml; \
	else \
		echo "⚠️  未找到 .env 文件，API Keys 可能无法加载"; \
		MONITOR_CORS_ORIGINS="$(MONITOR_CORS_ORIGINS)" $(AIR_CMD) -c .air.toml; \
	fi

# 运行测试
test:
	@echo "正在运行测试..."
	$(GOTEST) -v ./...

# 运行测试（带覆盖率）
test-coverage:
	@echo "正在运行测试并生成覆盖率..."
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "覆盖率报告已生成: coverage.html"

# 格式化代码
fmt:
	@echo "正在格式化代码..."
	$(GOFMT) ./...
	@echo "格式化完成"

# 清理编译产物
clean:
	@echo "正在清理..."
	@rm -f $(BINARY_PATH)
	@rm -rf tmp/
	@rm -f coverage.out coverage.html
	@echo "清理完成"

# 安装 air 热重载工具
install-air:
	@echo "正在安装 air..."
	$(GOCMD) install github.com/air-verse/air@latest
	@echo "安装完成！现在可以运行 'make dev'"

# 整理依赖
tidy:
	@echo "正在整理依赖..."
	$(GOMOD) tidy
	@echo "依赖整理完成"

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
	@cd frontend && npm run lint || echo "⚠ 前端 lint 有警告（不阻塞）"
	@echo ""
	@echo "=========================================="
	@echo "  CI 检查完成"
	@echo "=========================================="
