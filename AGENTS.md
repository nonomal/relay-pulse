# Repository Guidelines

> 本文件为仓库中所有「智能体 / 机器人 / 助手」提供协作规范，**仅供 AI 使用和维护**。人类贡献者一般无需阅读或修改本文件。

## 交互与语言约定
- 本项目相关的所有「智能体 / 机器人 / 助手」在与本仓库维护者互动时，应**始终使用简体中文**进行沟通与回复。
- 评论、Issue、PR 描述和代码内用户可见文案，优先使用中文；如需英文，请保证含义与中文一致，并以中文为主。
- 如果外部工具或脚本只能输出英文，应在说明中简要补充中文解释。

## 文档策略（仅供 AI）
- 面向人类读者，项目方**重点维护的核心文档只有**：
  - `README.md`（入口、快速开始、本地开发）
  - `QUICKSTART.md`（快速部署与常见问题）
  - `docs/user/config.md`（配置与环境变量说明）
  - `CONTRIBUTING.md`（贡献流程与规范）
- `AGENTS.md`、`CLAUDE.md` 视为 AI 内部文档，**不要在面向人类的回复、README 导航等位置主动推荐或暴露**，除非用户明确询问。
- `archive/` 与 `archive/docs/` 中的文档均为**历史文档**：可以作为补充背景使用，但在给用户引用时必须说明「历史文档，仅供参考，以当前核心文档和代码实现为准」。
- 为避免文档碎片化，AI 不应随意新增顶层文档；如确有必要，应与用户确认后再创建，并优先复用现有结构（例如在 `README` 中扩展 FAQ，而不是再建新的说明文件）。

## 项目结构与模块组织
- `cmd/server/main.go` 为 HTTP/API 入口，负责初始化配置、存储、调度和监测模块。
- 核心后端代码位于 `internal/`：`config`（配置）、`monitor`（探测）、`scheduler`（调度）、`storage`（存储）、`api`（HTTP 服务）。
- 前端代码在 `frontend/`（React + Vite + Tailwind），打包产物通过脚本嵌入到 `internal/api/frontend`。
- 根目录下存放 `config.yaml` 及 `config.*.example.yaml`，部署相关文件在 `deploy/`，Docker 相关在 `Dockerfile` 与 `docker-compose*.yaml`。
- 面向人类的说明文档集中在 `README.md`、`QUICKSTART.md`、`docs/user/config.md`、`CONTRIBUTING.md`；更早期的安装/架构/运维文档已迁移到 `archive/docs/`。

## 构建、测试与开发命令
- 后端：`make dev` 启动带热重载的开发服务（依赖 Air），`make run` 直接运行，`make build` 编译生产二进制，`make docker-build` 构建镜像。
- 测试：`make test` 或 `go test ./...` 运行单测，`make test-coverage` 生成 `coverage.html` 覆盖率报告。
- 前端：在 `frontend/` 下执行 `npm install`，`npm run dev` 本地调试，`npm run build` 打包，必要时运行 `./scripts/setup-dev.sh --rebuild-frontend` 同步静态文件。

## 代码风格与命名约定
- Go 代码必须通过 `go fmt`、`go vet`，导出符号需中文注释，错误使用 `fmt.Errorf("说明: %w", err)` 包装。
- 包名使用小写单词（如 `config`、`storage`），导出函数用大驼峰（如 `NewScheduler`），私有函数用小驼峰；访问共享状态前必须加锁（如 `cfgMu`、`mu`）。
- 前端使用函数式组件 + Hooks，文件名推荐 kebab-case，样式统一使用 Tailwind 工具类。
- 提交前建议执行 `pre-commit run --all-files`，保证格式化、编译与文档同步检查通过。

## 测试与质量保障
- 单元测试与源码同目录存放为 `*_test.go`，重点覆盖配置加载、调度逻辑和监测探测；修改核心逻辑时应补充或更新测试。
- 推荐使用 `curl http://localhost:8080/api/status`、`curl /health` 等命令进行手工回归验证。
- 前端改动需确保 `npm run lint` 通过，影响 UI 展示时在 PR 中附上截图说明。

## Commit 与 Pull Request 规范
- Commit 信息采用 `<type>: <subject>`，其中 `type` 为 `feat` / `fix` / `docs` / `refactor` / `test` / `chore` 等；`subject` 使用简要中文描述。
- 提交前保持单次 commit 可编译、可运行；噪声型 WIP 提交请在本地 squash 后再推送。
- PR 描述应包含：变更背景、解决方案概述、测试方式（附命令）、可能影响的配置/数据库/部署步骤；涉及 Issue 时在描述或 Footer 中添加 `Closes #123`。

## 配置与安全提示
- 禁止提交真实 API Key、数据库密码等敏感信息；仅更新 `config.yaml.example`，实际值通过环境变量（如 `MONITOR_88CODE_CC_API_KEY`）或本地未入库配置文件注入。
- 修改与存储相关逻辑时，需同时在 SQLite（默认）和 PostgreSQL 场景下验证，确保现有 `monitor.db` 数据不被破坏。
