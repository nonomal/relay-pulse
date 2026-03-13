.PHONY: dev dev-backend dev-frontend

# 一键启动前后端热重载开发环境
# 前端: Vite dev server (:5173) 带 HMR
# 后端: Air 热重载 (:8080)
# 开发时访问 http://localhost:5173
dev:
	@echo "Starting dev environment..."
	@echo "  Frontend: http://localhost:5173 (HMR)"
	@echo "  Backend:  http://localhost:8080 (API)"
	@trap 'kill 0' EXIT; \
	  $(MAKE) dev-backend & \
	  $(MAKE) dev-frontend & \
	  wait

dev-backend:
	$(HOME)/go/bin/air

dev-frontend:
	cd frontend && npm run dev
