# Linux.do 发布文案

> 最后更新：2025-01-21

---

## 标题

```
[开源] RelayPulse - 真·API级的LLM中转监控 | Go+React19实战 | 顺便聊聊AI结对开发
```

---

## 正文

### 前言：当 Uptime Kuma 都在骗你时

各位佬友好，我是 prehisle。

上周遇到个诡异的问题：**Uptime Kuma 一片绿，但跑代码全是 401/500**。

查了半天才发现：很多监控工具只检查 TCP 连通性或 HTTP 200，根本不知道 API 层面（比如认证、模型调用）是否正常。

**TCP 通了 ≠ API 能用。**

为了解决这个痛点（顺便守护写代码时的"心流"），我搓了个 **RelayPulse**：

- 🎯 **真实 API 探测**：每次检查都发起真实的 `/v1/chat/completions` 请求（消耗最小 Token）
- ⚡ **只有 LLM 真的吐字了，才算"可用"**
- 📊 **热力图可视化 + 可用率算法**：一眼看出哪家靠谱

**技术栈尝鲜：**
- **Backend**: Go 1.24 + Gin + fsnotify (配置热更新)
- **Frontend**: React 19 + Tailwind CSS v4 + Vite
- **Storage**: SQLite/PostgreSQL 双存储支持
- **Deploy**: Docker 容器化 (镜像仅 20MB)

---

### ✨ 在线演示

**传送门：** https://relaypulse.top

![界面预览](https://github.com/prehisle/relay-pulse/raw/main/docs/screenshots/dashboard.png)

> *注：看到那几条灰色的？那是我刚才重启后端导致的间隙 😅*

---

### 🛠️ 核心功能

#### 1. 真实 API 探测（拒绝假活）

不同于普通监控的 Ping/TCP 检查，RelayPulse 会：

- **发起真实的 Chat Completion 请求**（带真实 API Key）
- **验证响应**：状态码、延迟、内容关键字（可选）
- **根据阈值判定健康度**：`slow_latency` 配置可调

**状态码系统：**
- 🟢 **绿色** (status=1)：HTTP 2xx + 快速响应
- 🟡 **黄色** (status=2)：HTTP 2xx 但延迟高 / 5xx 临时错误
- 🔴 **红色** (status=0)：连接失败 / 4xx 错误
- ⚫ **灰色** (status=3)：未配置 / 认证失败

**可用率计算：**
- 采用平均值法：`总可用率 = 平均(所有时间块的可用率)`
- 灰色状态（无数据）算作 100%，避免初期虚低
- 渐变色显示：< 60% 红色，60-80% 黄色，80-100% 绿色

#### 2. 配置热更新（无需重启）

修改 `config.yaml` 保存后，服务**自动重载**，调度器立即使用新配置。

**实现原理：**
- 基于 `fsnotify` 监听文件变化
- 先验证新配置，再原子性更新
- 调用注册的回调函数通知各组件

```bash
# 修改配置
vim config.yaml

# 观察日志
[Config] 检测到配置文件变更，正在重载...
[Config] 热更新成功！已加载 3 个监控任务
[Scheduler] 配置已更新，下次巡检将使用新配置
```

#### 3. 双存储支持

- **SQLite**（默认）：单机部署，零配置
- **PostgreSQL**：K8s/生产环境，支持多副本

通过环境变量即可切换：

```bash
export MONITOR_STORAGE_TYPE=postgres
export MONITOR_POSTGRES_HOST=postgres-service
./monitor
```

#### 4. 分级监控策略

- **公益站 (public)**：重点看连通性
- **商业站 (commercial)**：严格评估延迟和稳定性

配置中通过 `category` 字段区分：

```yaml
monitors:
  - provider: "example"
    service: "cc"
    category: "commercial"  # 或 "public"
    sponsor: "团队自有"
    url: "https://api.example.com/v1/chat/completions"
    # ...
```

---

### 🏗️ 架构设计

**后端分层架构：**
```
cmd/server/main.go          → 应用入口，依赖注入
internal/
├── config/                 → 配置管理（验证、热更新、环境变量）
├── storage/                → 存储抽象层（SQLite/PostgreSQL）
├── monitor/                → 监控引擎（HTTP 客户端池、探测逻辑）
├── scheduler/              → 任务调度（并发控制、防重复触发）
└── api/                    → HTTP API（Gin、历史查询、CORS）
```

**核心设计原则：**
- 基于接口的设计 (`storage.Storage`)
- 并发安全（RWMutex 保护共享状态）
- 优雅关闭（Context 传播确保资源清理）

**前端组件化：**
```
frontend/src/
├── components/            → UI 组件（StatusCard、HeatmapBlock、Footer）
├── hooks/                 → 自定义 Hooks（useMonitorData 封装 API 轮询）
├── types/                 → TypeScript 类型定义
└── utils/                 → 工具函数（可用率计算、颜色渐变）
```

---

### 🤖 AI 协作小彩蛋

这个项目是我和 **Claude Code + Codex** 结对开发的：

- **Claude Code**：负责架构建议、Go 并发模型、React 19 新特性指导
- **Codex**：负责代码补全和样板生成
- **人类（我）**：负责 Code Review、测试、核心逻辑决策、架构权衡

**有意思的地方：**

1. **React 19 刚出，文档很少**，Claude 凭推理能力搞定了 `useActionState`、并发渲染等新特性
2. **Go 的 Goroutine 池和热更新回调**，基本一遍过，逻辑非常清晰
3. **前端 Tailwind v4 + Cyberpunk 风格**，Codex 秒杀样板代码
4. **并发安全问题**（读写锁、调度器状态机），Claude 给出了严谨的实现方案

**人类的作用：**
- 把控产品方向（"真实API探测" 这个核心需求）
- Review 所有 AI 生成的代码（特别是并发、安全相关）
- 处理依赖冲突和版本兼容问题
- 写文档和测试用例

如果你对 **AI Pair Programming 的 Prompt 技巧**感兴趣，评论区可以聊聊（比如如何让 AI 理解复杂的架构约束）。

---

### 📊 数据展示

**热力图：**
- 24小时：每小时一个方块
- 7天/30天：每天一个方块
- 鼠标悬停查看详细信息（时间、状态、延迟、可用率）

**表格视图：**
- 当前状态 + 最新延迟
- 最近 24h 可用率
- 点击服务商名称跳转官网

**实时刷新：**
- 前端每 30 秒轮询一次
- 后端根据配置定时探测（默认 1 分钟）

---

### 🔒 安全设计

**API Key 管理：**
- 支持环境变量覆盖（推荐）：`MONITOR_<PROVIDER>_<SERVICE>_API_KEY`
- 配置文件中的 Key 通过 `.gitignore` 排除，不会提交到仓库
- 前端 API 响应中 `api_key` 字段始终不返回

**CORS 配置：**
- 生产环境仅允许 `https://relaypulse.top`
- 开发环境允许 `http://localhost:*`

**Git 安全扫描：**
- 使用 `gitleaks` 扫描敏感信息
- Pre-commit hooks 自动检查

---

### 📌 招募 & 互动

#### 需要测试 Key

目前收录的站点都是我**自费测试**（每次探测都消耗 Token）。

如果你：
- 是**服务商**，想验证自家服务质量
- 有**闲置 Key** 愿意贡献测试
- 想推荐新的 LLM 中转服务

欢迎联系我：

- 📧 **邮箱**：`prehisle@gmail.com`（推荐）
- 💬 **QQ**：18058344
- 🐙 **GitHub Issues**：https://github.com/prehisle/relay-pulse/issues/new?template=provider.md

⚠️ **安全提示：**
- **不要在 GitHub Issue 公开贴 Key！**
- Key 仅用于健康检查（发起 Chat Completion 请求）
- 不会保存到仓库，通过环境变量注入
- 可以随时撤销或更换

#### 开源地址

- **GitHub**: https://github.com/prehisle/relay-pulse
- **在线演示**: https://relaypulse.top
- **技术文档**: https://github.com/prehisle/relay-pulse/tree/main/docs

代码完全开源，欢迎：
- ⭐ **Star** 支持一下
- 🔀 **Fork** 自建节点
- 🐛 **提 Issue** 反馈问题
- 💡 **提 PR** 贡献代码

---

### 💬 互动话题

1. **你们监控 LLM API 时踩过哪些坑？**（比如延迟波动、突然失联、计费异常等）
2. **有没有更好的可用率计算方法？**（目前用的是平均值法）
3. **对 AI 辅助开发有什么想吐槽的？**（幻觉、版本不兼容、Prompt 技巧等）
4. **想自建节点？**（支持 Docker 一键部署，文档齐全）

评论区见 👇

---

### 📖 FAQ

**Q: 为什么不用现成的 Uptime Kuma / Statuspage？**
A: 因为它们只检查 HTTP 200 或 TCP 连通性，检测不到 API 层面的问题（如认证失败、模型不可用）。RelayPulse 发起真实的 Chat Completion 请求，只有 LLM 真的吐字了才算活着。

**Q: Key 如何安全保存？**
A: 推荐通过环境变量注入（`MONITOR_<PROVIDER>_<SERVICE>_API_KEY`），不会保存到配置文件。即使写在 `config.yaml`，该文件也已加入 `.gitignore`，不会提交到 Git。

**Q: 支持多地域监控吗？**
A: 目前单节点部署。多地域监控在 Roadmap 中，欢迎贡献境外节点或提供 VPS。

**Q: Go 1.24 是否必须？**
A: `go.mod` 要求 1.24，但理论上 1.22+ 即可编译运行。Docker 镜像已内置 1.24，无需本地安装。

**Q: React 19 稳定吗？**
A: 目前生产环境运行正常。前端使用的主要是 Hooks 和并发渲染特性，相对稳定。

**Q: 如何贡献新的服务商？**
A: 请通过 [GitHub Issues](https://github.com/prehisle/relay-pulse/issues/new?template=provider.md) 提交基本信息，Key 请私下发送到邮箱。

---

### 🎯 Roadmap

- [ ] 多地域节点支持（境外延迟监控）
- [ ] Webhook 告警（企业微信、钉钉、Telegram）
- [ ] 历史数据导出（CSV/JSON）
- [ ] 自定义探测脚本（支持更多 API 格式）
- [ ] 移动端 PWA 支持
- [ ] Affiliate 链接管理（返利/推广）

---

## 附：截图

*(发帖时插入以下截图)*

1. **仪表盘全景图**：展示热力图 + 可用率
2. **Tooltip 详情**：鼠标悬停显示的详细信息
3. **配置热更新日志**：展示修改 config.yaml 后的自动重载
4. **移动端适配**：响应式布局截图

---

## 发帖后的互动策略

1. **置顶回复准备 FAQ**，避免重复回答
2. **引导讨论**：
   - "有没有人遇到过 LLM API 假活的情况？"
   - "欢迎分享你的 Prompt 技巧"
3. **及时回复评论**，尤其是技术问题和合作意向
4. **准备 Demo 视频**（可选）：展示配置热更新、热力图交互

---

**最后更新：** 2025-01-21
**作者：** prehisle
**联系方式：** prehisle@gmail.com
