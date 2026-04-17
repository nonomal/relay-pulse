# Archive - 历史文档

本目录包含项目开发过程中的历史文档，仅供参考。这些文档不再维护更新。

## 文档列表

- **IMPLEMENTATION.md** - 实现细节和技术决策
- **FINAL_REPORT.md** - 项目总结报告
- **CODE_REVIEW.md** - 代码审查记录

## 当前文档

请查看项目根目录的 [README.md](../README.md) 获取最新文档导航。维护者优先保证以下「核心文档」是最新的：

- **项目入口**: `README.md`
- **快速部署**: `QUICKSTART.md`
- **配置手册**: `docs/user/config.md`
- **贡献规范**: `CONTRIBUTING.md`

本目录下的 `archive/docs/` 中，保留了历史的安装指南、架构说明、运维手册等文档，仅供参考，**不再保证与最新代码完全一致**：

- `archive/docs/user/*.md` - 旧版安装与运维文档
- `archive/docs/developer/*.md` - 旧版架构、回忆清单、发布流程
- `archive/docs/deployment.md` - 历史部署说明
- `archive/docs/API_KEY_MIGRATION.md` / `DOCKER_TROUBLESHOOTING.md` - 一次性迁移/排障记录
- `archive/docs/community/linuxdo-post.md` - 早期社区发布文案

## 归档政策

本目录由维护者按以下规则管理，**AI 助手与贡献者不应从此处发起引用链路**：

- **入档时机**：文档覆盖的功能已下线、被重写、或仅为一次性交付物（PRD/Review/Report）且不再更新时，移入本目录。
- **入档后不再修订**：除批量补充过期标头、修正明显事实错误外，不做内容性更新。需要新内容时，请在现行 `docs/user/` 或 `CONTRIBUTING.md` 中重写。
- **不作为引路文档**：`README.md` / `QUICKSTART.md` / `docs/user/` 不应反向链接至本目录；如确需提及，必须明确标注「历史文档，仅供参考，以当前核心文档和代码实现为准」（与 `AGENTS.md` 的文档策略对齐）。
- **顶栏约定**：本目录下每份 `.md` 顶部应有 `> **⚠️ 历史文档 / Deprecated** — Last verified: YYYY-MM-DD` 标头，方便批量识别与审计。
