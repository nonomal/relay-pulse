# CLAUDE.md

⚠️ 本文档为 AI 助手（如 Claude / ChatGPT）在此代码库中工作的内部指南，**优先由 AI 维护，人类贡献者通常不需要修改本文件**。
如果你是人类开发者，请优先阅读 `README.md` 和 `CONTRIBUTING.md`，只在需要了解更多技术细节时再参考这里的内容。

## 项目概览

这是一个企业级 LLM 服务可用性监测系统，支持配置热更新、SQLite/PostgreSQL 持久化和实时状态追踪。

### 项目文档

- **README.md** - 项目简介、快速开始、本地开发入口（人类入口文档）
- **QUICKSTART.md** - 5 分钟快速部署与常见问题（人类核心文档）
- **docs/user/config.md** - 配置项、环境变量与安全实践（人类核心文档）
- **docs/user/docker.md** - Docker 部署详细指南
- **docs/user/deploy-postgres.md** - PostgreSQL 部署指南
- **docs/user/sponsorship.md** - 赞助权益体系规则（角色、权益、义务、配置）
- **CONTRIBUTING.md** - 贡献流程、代码规范、提交与 PR 约定（人类核心文档）
- **AGENTS.md / CLAUDE.md** - AI 内部协作与技术指南（仅供 AI 使用，不要在回答中主动推荐给人类）
- **docs/developer/** - 开发者文档（版本检查等）
- **archive/** - 历史文档（仅供参考）

**文档策略（供 AI 遵守）**:
- 回答人类用户时，**优先引用上述 4 个核心文档**，避免让用户跳进 `archive/` 中的大量历史内容。
- 如必须引用 `archive/docs/*` 或 `archive/*.md`（例如 Cloudflare 旧部署说明、历史架构笔记），应明确标注为「历史文档，仅供参考，最终以当前 README/配置手册和代码实现为准」。
- 不主动向人类暴露 `AGENTS.md`、本文件等 AI 内部文档，除非用户明确询问「AI 如何在本仓库工作」一类问题。

### 技术栈

- **后端**: Go 1.24+ (Gin, fsnotify, SQLite/PostgreSQL)
- **前端**: React 19, TypeScript, Tailwind CSS v4, Vite

## 开发命令

### 首次开发环境设置

```bash
# ⚠️ 首次开发或前端代码更新后必须运行此脚本
./scripts/setup-dev.sh

# 如果前端代码有更新，需要重新构建并复制
./scripts/setup-dev.sh --rebuild-frontend
```

**重要**: Go 的 `embed` 指令不支持符号链接，因此需要将 `frontend/dist` 复制到 `internal/api/frontend/dist`。setup-dev.sh 脚本会自动处理这个问题。

**⚠️ 前端代码修改规则**:
- `internal/api/frontend/` 整个目录被 `.gitignore` 忽略，是从 `frontend/` 复制过来的嵌入目录
- **所有前端源代码修改必须在 `frontend/` 目录进行**，而不是 `internal/api/frontend/`
- 修改后运行 `./scripts/setup-dev.sh --rebuild-frontend` 同步到嵌入目录
- 直接修改 `internal/api/frontend/` 的改动不会被 git 追踪，会在下次构建时丢失

### 后端 (Go)

```bash
# 开发环境 - 使用 Air 热重载（推荐）
./dev.sh
# 或直接使用: air

# 生产环境 - 手动构建运行
go build -o monitor ./cmd/server
./monitor

# 使用自定义配置运行
./monitor path/to/config.yaml

# 运行测试
go test ./...

# 运行测试并生成覆盖率
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# 运行特定包的测试
go test ./internal/config/
go test -v ./internal/storage/

# 代码格式化和检查
go fmt ./...
go vet ./...

# 整理依赖
go mod tidy

# 验证单个检测项（调试配置问题）
go run ./cmd/verify/main.go -provider <name> -service <name> [-v]
# 示例: go run ./cmd/verify/main.go -provider AICodeMirror -service cc -v
```

### 前端 (React)

```bash
cd frontend

# 开发服务器
npm run dev

# 生产构建
npm run build

# 代码检查
npm run lint

# 预览生产构建
npm run preview

# 运行测试
npm run test

# 测试监听模式
npm run test:watch
```

### Pre-commit Hooks

```bash
# 安装 pre-commit (一次性设置)
pip install pre-commit
pre-commit install

# 手动运行所有检查
pre-commit run --all-files
```

### CI/CD

```bash
# 本地模拟 CI 检查（提交前运行）
make ci

# CI 流程包含：
# - Go 格式检查 (gofmt)
# - Go 静态分析 (go vet)
# - Go 单元测试 (go test)
# - 前端 lint (npm run lint)
```

**GitHub Actions 工作流**：
- `test.yml` - PR 和 main 分支推送时运行测试
- `docker-publish.yml` - 测试通过后构建 Docker 镜像

## 架构与设计模式

### 后端架构

Go 后端遵循**分层架构**，职责清晰分离：

```
cmd/server/main.go          → 应用程序入口，依赖注入
internal/
├── config/                 → 配置管理（使用 fsnotify 实现热更新）
│   ├── config.go          → 数据结构、验证、规范化
│   ├── loader.go          → YAML 解析、环境变量覆盖
│   └── watcher.go         → 文件监听实现热更新
├── logger/                 → 统一日志系统（基于 log/slog）
│   └── logger.go          → 结构化日志、request_id 支持
├── storage/               → 存储抽象层
│   ├── storage.go         → 接口定义
│   ├── common.go          → 公共工具函数
│   └── sqlite.go          → SQLite 实现 (modernc.org/sqlite)
├── monitor/               → 监测逻辑
│   ├── client.go          → HTTP 客户端池管理
│   └── probe.go           → 健康检查探测逻辑
├── scheduler/             → 任务调度
│   └── scheduler.go       → 周期性健康检查、并发执行
└── api/                   → HTTP API 层
    ├── handler.go         → 请求处理器、查询参数处理、TimeFilter 时段过滤
    ├── time_filter_test.go → TimeFilter 单元测试
    └── server.go          → Gin 服务器设置、中间件、CORS
```

**核心设计原则：**
1. **基于接口的设计**: `storage.Storage` 接口允许切换不同实现
2. **并发安全**: 所有共享状态使用 `sync.RWMutex` 或 `sync.Mutex`
3. **热更新**: 配置变更触发回调，无需重启即可更新运行时状态
4. **优雅关闭**: Context 传播确保资源清理
5. **HTTP 客户端池**: 通过 `monitor.ClientPool` 复用连接
6. **结构化日志**: 统一使用 `logger` 包，支持 request_id 链路追踪

### 日志系统

项目使用 Go 标准库 `log/slog` 实现统一的结构化日志：

```go
// 基础用法
logger.Info("component", "消息", "key1", value1, "key2", value2)
logger.Warn("component", "警告消息", "error", err)
logger.Error("component", "错误消息", "error", err)

// 带 request_id 的日志（用于 API 请求追踪）
logger.FromContext(ctx, "api").Info("请求处理完成", "status", 200)
```

**日志格式**：
```
time=2024-01-15T10:30:00.000Z level=INFO msg=消息 app=relay-pulse component=api request_id=abc123
```

**Request ID 中间件**：
- API 层自动为每个请求生成 8 位短 UUID
- 支持通过 `X-Request-ID` 请求头传入自定义 ID
- 响应头返回 `X-Request-ID` 便于客户端关联

### 配置热更新模式

系统采用**基于回调的热更新**机制：
1. `config.Watcher` 使用 `fsnotify` 监听 `config.yaml`
2. 文件变更时，先验证新配置再应用
3. 调用注册的回调函数（调度器、API 服务器）传入新配置
4. 各组件使用锁原子性地更新状态
5. 调度器立即使用新配置触发探测周期

**环境变量覆盖**: API 密钥可通过 `MONITOR_<PROVIDER>_<SERVICE>_API_KEY` 设置（大写，`-` → `_`）

### 前端架构

React SPA，基于组件的结构：

```
frontend/src/
├── components/            → UI 组件（StatusCard、StatusTable、Tooltip 等）
├── hooks/                 → 自定义 Hooks（useMonitorData 用于 API 数据获取）
├── i18n/                  → 国际化配置
│   ├── index.ts          → i18n 配置、语言检测器、语言映射
│   └── locales/          → 翻译文件（zh-CN, en-US, ru-RU, ja-JP）
├── types/                 → TypeScript 类型定义
├── constants/             → 应用常量（API URLs、时间周期）
├── utils/                 → 工具函数
│   ├── mediaQuery.ts     → 响应式断点管理（统一的 matchMedia API）
│   ├── heatmapAggregator.ts → 热力图数据聚合
│   └── color.ts          → 颜色工具函数
├── App.tsx               → 主应用组件
├── router.tsx            → 路由配置（语言路径前缀）
└── main.tsx              → 应用入口（BrowserRouter、HelmetProvider）
```

**关键模式：**
- **自定义 Hooks**: `useMonitorData` 封装 API 轮询逻辑
- **TypeScript**: 使用 `types/` 中的接口实现完整类型安全
- **Tailwind CSS**: Tailwind v4 实用优先的样式
- **组件组合**: 小型、可复用组件
- **响应式设计**: 移动优先，使用 matchMedia API 实现稳定断点检测
- **国际化**: react-i18next + react-router-dom 实现 URL 路径多语言
- **主题系统**: 4 套主题 + 语义化 CSS 变量

### 主题系统

**支持的主题**:
- `default-dark`: 默认暗色（青色强调）
- `night-dark`: 护眼暖暗（琥珀色强调）
- `light-cool`: 冷灰亮色（青色强调）
- `light-warm`: 暖灰亮色（琥珀色强调）

**技术实现**:
```
frontend/src/
├── styles/themes/           → 主题 CSS 文件
│   ├── index.css           → 入口 + 语义化工具类
│   ├── default-dark.css    → 默认暗色主题变量
│   ├── night-dark.css      → 护眼暖暗主题变量
│   ├── light-cool.css      → 冷灰亮色主题变量
│   └── light-warm.css      → 暖灰亮色主题变量
├── hooks/useTheme.ts        → 主题状态管理 Hook
└── components/ThemeSwitcher.tsx → 主题切换器组件
```

**语义化颜色变量** (`themes/*.css`):
```css
:root[data-theme="default-dark"] {
  /* 背景层级 */
  --bg-page: 222 47% 3%;       /* 最底层页面背景 */
  --bg-surface: 217 33% 8%;    /* 卡片/面板背景 */
  --bg-elevated: 215 28% 12%;  /* 悬浮/弹出层背景 */
  --bg-muted: 215 25% 18%;     /* 禁用/次要背景 */

  /* 文字层级 */
  --text-primary: 210 40% 98%;   /* 主要文字 */
  --text-secondary: 215 20% 65%; /* 次要文字 */
  --text-muted: 215 15% 45%;     /* 禁用文字 */

  /* 强调色 */
  --accent: 187 85% 53%;         /* 主强调色 */
  --accent-strong: 187 90% 60%;  /* 强调色悬停态 */

  /* 状态色 */
  --success: 142 71% 45%;
  --warning: 38 92% 50%;
  --danger: 0 84% 60%;
}
```

**语义化工具类** (`themes/index.css`):
```css
@layer utilities {
  .bg-page { background-color: hsl(var(--bg-page)); }
  .bg-surface { background-color: hsl(var(--bg-surface)); }
  .text-primary { color: hsl(var(--text-primary)); }
  .text-accent { color: hsl(var(--accent)); }
  /* ... 更多工具类 */
}
```

**FOUC 防护** (`index.html`):
```html
<script>
  (function() {
    var theme = 'default-dark';
    try {
      var stored = localStorage.getItem('relay-pulse-theme');
      if (stored && ['default-dark','night-dark','light-cool','light-warm'].indexOf(stored) !== -1) {
        theme = stored;
      }
    } catch (e) {}
    document.documentElement.setAttribute('data-theme', theme);
    // 设置初始背景色防止白屏...
  })();
</script>
```

**使用规范**:
- ❌ 避免硬编码颜色：`text-slate-500`、`bg-zinc-800`
- ✅ 使用语义化类：`text-muted`、`bg-elevated`
- 透明度变体：`bg-surface/60`、`text-accent/50`

### 国际化架构 (i18n)

**支持的语言**:
- 🇨🇳 **中文** (zh-CN) - 默认语言，路径 `/`
- 🇺🇸 **English** (en-US) - 路径 `/en/`（简化）
- 🇷🇺 **Русский** (ru-RU) - 路径 `/ru/`（简化）
- 🇯🇵 **日本語** (ja-JP) - 路径 `/ja/`（简化）

**技术实现**:
1. **react-i18next**: 核心翻译框架，支持嵌套 JSON、参数插值
2. **react-router-dom v6**: 基于简化路径前缀的语言路由（`/en/*`、`/ru/*`、`/ja/*`）
3. **react-helmet-async**: 动态更新 `<title>` 和 `<meta name="description">` 支持 SEO
4. **i18next-browser-languagedetector**: 自动检测语言（localStorage > 浏览器语言）

**设计原则**:
- **URL 简洁性**: 使用简化语言码（`/en/` 而非 `/en-US/`）提升美观性
- **内部完整性**: 内部仍使用完整 locale（`en-US`）兼容 i18next
- **类型安全**: 使用类型守卫 `isSupportedLanguage` 确保类型正确性
- **路由分层**: `/api/*`、`/health` 等技术路径不参与 i18n

**路由策略**:
```typescript
// router.tsx
<Routes>
  {/* 中文默认路径（无前缀） */}
  <Route path="/" element={<LanguageWrapper />} />

  {/* 简化语言前缀路径 */}
  <Route path="/en/*" element={<LanguageWrapper pathLang="en" />} />
  <Route path="/ru/*" element={<LanguageWrapper pathLang="ru" />} />
  <Route path="/ja/*" element={<LanguageWrapper pathLang="ja" />} />

  {/* 捕获所有未匹配路径 */}
  <Route path="*" element={<Navigate to="/" replace />} />
</Routes>
```

**核心映射** (`i18n/index.ts`):
```typescript
// URL 路径前缀 → 语言编码
export const PATH_LANGUAGE_MAP: Record<string, SupportedLanguage> = {
  '': 'zh-CN',   // 根路径默认中文
  en: 'en-US',   // /en/ → en-US
  ru: 'ru-RU',   // /ru/ → ru-RU
  ja: 'ja-JP',   // /ja/ → ja-JP
};

// 语言编码 → URL 路径前缀（反向映射）
export const LANGUAGE_PATH_MAP: Record<SupportedLanguage, string> = {
  'zh-CN': '',   // 中文无前缀
  'en-US': 'en', // en-US → /en/
  'ru-RU': 'ru', // ru-RU → /ru/
  'ja-JP': 'ja', // ja-JP → /ja/
};

// 类型守卫：确保类型安全
export const isSupportedLanguage = (lng: string): lng is SupportedLanguage =>
  (SUPPORTED_LANGUAGES as readonly string[]).includes(lng);

// 语言归一化：处理浏览器语言码
export const normalizeLanguage = (lng: string): SupportedLanguage => {
  // 完整匹配（如 'en-US'、'zh-CN'）
  if (isSupportedLanguage(lng)) {
    return lng;
  }

  // 处理无地区码的语言（提取前缀）
  const prefix = lng.split('-')[0].toLowerCase();

  switch (prefix) {
    case 'zh':
      return 'zh-CN'; // 中文 → 简体中文
    case 'en':
      return 'en-US'; // 英文 → 美国英语
    case 'ru':
      return 'ru-RU'; // 俄语
    case 'ja':
      return 'ja-JP'; // 日语
    default:
      return 'zh-CN'; // 默认中文
  }
};
```

**语言切换逻辑** (`Header.tsx`):
```typescript
const handleLanguageChange = (newLang: SupportedLanguage) => {
  const rawLang = i18n.language;
  const currentLang: SupportedLanguage = isSupportedLanguage(rawLang) ? rawLang : 'zh-CN';

  let newPath = location.pathname;
  const queryString = location.search + location.hash;

  // 移除当前语言前缀（如果有）
  const currentPrefix = LANGUAGE_PATH_MAP[currentLang];
  if (currentPrefix && newPath.startsWith(`/${currentPrefix}`)) {
    newPath = newPath.substring(`/${currentPrefix}`.length) || '/';
  }

  // 添加新语言前缀（中文除外）
  const newPrefix = LANGUAGE_PATH_MAP[newLang];
  if (newPrefix) {
    newPath = `/${newPrefix}${newPath === '/' ? '' : newPath}`;
  }

  navigate(newPath + queryString);  // 保留查询参数和 hash
};
```

**语言检测策略**:
```typescript
// i18n 配置（i18n/index.ts）
i18n
  .use(initReactI18next)
  .use(LanguageDetector)
  .init({
    detection: {
      order: ['localStorage', 'navigator'],  // 优先级
      caches: ['localStorage'],
      lookupLocalStorage: 'i18nextLng',
      // 语言归一化：将浏览器语言标准化
      convertDetectedLanguage: (lng) => normalizeLanguage(lng),
    },
    // ...
  });
```

**优势**:
- 浏览器设置为 `en` 时自动映射为 `en-US`
- 浏览器设置为 `zh` 时自动映射为 `zh-CN`
- 提升首次访问时的语言检测准确性

**URL 路径语言同步** (`router.tsx` 中的 `LanguageWrapper`):
- URL 路径前缀由 react-router 匹配并传递给 `LanguageWrapper`
- `LanguageWrapper` 负责将 URL 语言同步到 i18next
- 根路径 `/` 使用 localStorage 或浏览器语言（无强制中文）
- 特定语言路径（`/en/`、`/ru/`、`/ja/`）强制使用对应语言

**翻译文件结构** (`i18n/locales/*.json`):
```json
{
  "meta": { "title": "...", "description": "..." },
  "common": { "loading": "...", "error": "...", ... },
  "header": { "tagline": "...", "stats": {...}, ... },
  "controls": { "filters": {...}, "timeRanges": {...}, ... },
  "table": { "headers": {...}, "sorting": {...}, "category": {...}, ... },
  "status": { "available": "...", "degraded": "...", ... },
  "subStatus": { "slow_latency": "...", "rate_limit": "...", ... },
  "tooltip": { "uptime": "...", "latency": "...", ... },
  "footer": { "disclaimer": {...}, ... },
  "accessibility": { "uptimeBlock": "...", ... }
}
```

**工厂模式** - 动态注入翻译到常量 (`constants/index.ts`):
```typescript
// 向后兼容：保留原有静态导出
export const TIME_RANGES: TimeRange[] = [
  { id: '24h', label: '近24小时', points: 24, unit: 'hour' },
  // ...
];

// i18n 版本：工厂函数
export const getTimeRanges = (t: TFunction): TimeRange[] => [
  { id: '24h', label: t('controls.timeRanges.24h'), points: 24, unit: 'hour' },
  // ...
];

// 组件中使用
const { t } = useTranslation();
const timeRanges = getTimeRanges(t);  // 动态翻译
```

**SEO 支持**:

**静态 HTML** (`index.html`):
```html
<!-- 使用英文作为默认，更适合国际化和 SEO -->
<meta name="description" content="RelayPulse - Monitor availability, latency, and sponsored routes of LLM relay services worldwide..." />
<title>RelayPulse - Availability monitoring for LLM relay services</title>
```

**动态更新** (`App.tsx`):
```typescript
import { Helmet } from 'react-helmet-async';

function App() {
  const { t, i18n } = useTranslation();

  return (
    <>
      <Helmet>
        <html lang={i18n.language} />
        <title>{t('meta.title')}</title>
        <meta name="description" content={t('meta.description')} />
      </Helmet>
      {/* ... */}
    </>
  );
}
```

**策略说明**:
- `index.html` 使用**英文**作为默认（利于 SEO 和国际化）
- React 渲染后，Helmet 会根据检测/选择的语言动态更新
- 每个语言版本都有完整的 meta 标签翻译

**覆盖范围**: 100% UI 文本（9/9 组件）
- ✅ App.tsx - meta 标签
- ✅ Header.tsx - 语言切换、tagline、统计
- ✅ Footer.tsx - 免责声明
- ✅ Controls.tsx - 筛选器、时间范围、视图切换
- ✅ StatusTable.tsx - 表头、排序、分类标签、详情
- ✅ StatusCard.tsx - 可用率标签、时间标签
- ✅ Tooltip.tsx - 状态标签、子状态细分
- ✅ HeatmapBlock.tsx - 无障碍 aria-label
- ✅ constants/index.ts - 状态配置、时间范围

### 响应式断点系统

前端采用**统一的媒体查询管理系统**（`utils/mediaQuery.ts`），确保断点检测的一致性和浏览器兼容性：

**断点定义** (`BREAKPOINTS`):
- **mobile**: `< 768px` - Tooltip 底部 Sheet vs 悬浮提示
- **tablet**: `< 960px` - StatusTable 卡片视图 vs 表格 + 热力图聚合

**设计原则：**
1. **使用 matchMedia API**：替代 `resize` 事件监听，避免高频触发
2. **Safari ≤13 兼容**：自动回退到 `addListener/removeListener` API
3. **HMR 安全**：在 Vite 热重载时自动清理监听器，防止内存泄漏
4. **缓存优化**：模块级缓存断点状态，避免重复计算
5. **事件隔离**：移动端禁用鼠标悬停事件，避免闪烁

**使用示例：**
```typescript
import { createMediaQueryEffect } from '../utils/mediaQuery';

// 在组件中检测断点
useEffect(() => {
  const cleanup = createMediaQueryEffect('mobile', (isMobile) => {
    setIsMobile(isMobile);
  });
  return cleanup;
}, []);
```

**响应式行为：**
| 组件 | < 768px (mobile) | < 960px (tablet) | ≥ 960px (desktop) |
|------|------------------|------------------|-------------------|
| Tooltip | 底部 Sheet | 底部 Sheet | 悬浮提示 |
| StatusTable | 卡片列表 | 卡片列表 | 完整表格 |
| HeatmapBlock | 点击触发，禁用悬停 | 点击触发 | 悬停显示 |
| 热力图数据 | 聚合显示 | 聚合显示 | 完整显示 |

### 数据流

1. **Scheduler** (`scheduler.Scheduler`) 运行周期性健康检查
2. **Monitor** (`monitor.Probe`) 向配置的端点执行 HTTP 请求
3. 结果保存到 **Storage** (`storage.SQLiteStorage`)
4. **API** (`api.Handler`) 通过 `/api/status` 提供历史数据
5. **Frontend** 轮询 `/api/status` 并渲染可视化

### 状态码系统

**主状态（status）**：
- `1` = 🟢 绿色（成功、HTTP 2xx、延迟正常）
- `2` = 🟡 黄色（降级：慢响应等）
- `0` = 🔴 红色（不可用：各类错误，包括限流）
- `-1` = ⚪ 灰色（仅用于时间块无数据，不是探测结果）

**HTTP 状态码映射**：
```
HTTP 响应
├── 2xx + 快速 + 内容匹配 → 🟢 绿色
├── 2xx + 慢速 + 内容匹配 → 🟡 波动 (slow_latency)
├── 2xx + 内容不匹配 → 🔴 不可用 (content_mismatch)  ← 无论快慢
├── 3xx → 🟢 绿色（重定向）
├── 400 → 🔴 不可用 (invalid_request)
├── 401/403 → 🔴 不可用 (auth_error)
├── 429 → 🔴 不可用 (rate_limit)  ← 不做内容校验
├── 其他 4xx → 🔴 不可用 (client_error)
├── 5xx → 🔴 不可用 (server_error)
└── 网络错误 → 🔴 不可用 (network_error)
```

**内容校验（`success_contains`）**：
- 仅对 **2xx 响应**（绿色和慢速黄色）执行内容校验
- **429 限流**：响应体是错误信息，不做内容校验
- **红色状态**：已是最差状态，不需要再校验
- 若 2xx 响应但内容不匹配 → 降级为 🔴 红色（语义失败）

**细分状态（SubStatus）**：

| 主状态 | SubStatus | 标签 | 触发条件 |
|--------|-----------|------|---------|
| 🟡 黄色 | `slow_latency` | 响应慢 | HTTP 2xx 但延迟超过阈值 |
| 🔴 红色 | `rate_limit` | 限流 | HTTP 429 |
| 🔴 红色 | `server_error` | 服务器错误 | HTTP 5xx |
| 🔴 红色 | `client_error` | 客户端错误 | HTTP 4xx（除 400/401/403/429） |
| 🔴 红色 | `auth_error` | 认证失败 | HTTP 401/403 |
| 🔴 红色 | `invalid_request` | 请求参数错误 | HTTP 400 |
| 🔴 红色 | `network_error` | 连接失败 | 网络错误、连接超时 |
| 🔴 红色 | `content_mismatch` | 内容校验失败 | HTTP 2xx 但响应体不含预期内容 |

**可用率计算**：
- 采用**加权平均法**：每个状态按不同权重计入可用率
  - 绿色（status=1）→ **100% 权重**
  - 黄色（status=2）→ **degraded_weight 权重**（默认 70%，可配置）
  - 红色（status=0）→ **0% 权重**
- 每个时间块可用率 = `(累积权重 / 总探测次数) * 100`
- 总可用率 = `平均(所有时间块的可用率)`
- 无数据的时间块（availability=-1）不参与可用率计算，全无数据时显示 "--"
- 所有可用率显示（列表、Tooltip、热力图）统一使用渐变色：
  - 0-60% → 红到黄渐变
  - 60-100% → 黄到绿渐变

**延迟统计**：
- **仅统计可用状态**：只有 status > 0（绿色/黄色）的记录才纳入延迟统计，红色状态不计入
- 每个时间块延迟 = `sum(可用记录延迟) / 可用记录数`
- 延迟显示使用渐变色（基于 `slow_latency` 配置）：
  - < 30% slow_latency → 绿色（优秀）
  - 30%-100% → 绿到黄渐变（良好）
  - 100%-200% → 黄到红渐变（较慢）
  - ≥ 200% → 红色（很慢）
- API 响应 `meta.slow_latency_ms` 返回阈值（毫秒），供前端计算颜色

## 配置管理

### 配置文件结构

```yaml
interval: "1m"         # 全局探测频率（Go duration 格式）
slow_latency: "5s"     # 慢请求黄灯阈值
timeout: "10s"         # 请求超时时间（默认 10s）
degraded_weight: 0.7   # 黄色状态的可用率权重（0-1，默认 0.7，可选）

# 按服务类型覆盖（可选）
slow_latency_by_service:
  cc: "15s"            # Claude Code 服务允许更长延迟
  gm: "3s"             # Gemini 服务要求更快
timeout_by_service:
  cc: "30s"            # Claude Code 服务允许更长超时

monitors:
  - provider: "88code"
    provider_name: "88Code 官方"  # 可选：UI 显示名称（未配置时使用 provider）
    service: "cc"
    service_name: "Claude Code"   # 可选：UI 显示名称（未配置时使用 service）
    channel: "vip3"
    channel_name: "VIP 3 通道"    # 可选：UI 显示名称（未配置时使用 channel）
    interval: "30s"    # 可选：覆盖全局 interval（高频付费监测）
    slow_latency: "20s"  # 可选：覆盖 slow_latency_by_service 和全局值
    timeout: "45s"       # 可选：覆盖 timeout_by_service 和全局值
    url: "https://api.88code.com/v1/chat/completions"
    method: "POST"
    api_key: "sk-xxx"  # 可通过 MONITOR_88CODE_CC_API_KEY 覆盖
    headers:
      Authorization: "Bearer {{API_KEY}}"
    body: |
      {"model": "claude-3-opus", "messages": [...]}
    success_contains: "optional_keyword"  # 语义验证（可选）
```

**配置优先级**: `monitor` > `by_service` > `global`（适用于 slow_latency 和 timeout）


**模板占位符**: `{{API_KEY}}` 在 headers 和 body 中会被替换。

**引用文件**: 对于大型请求体，使用 `body: "!include data/filename.json"`（必须在 `data/` 目录下）。

### 热更新测试

```bash
# 启动监测服务
./monitor

# 在另一个终端编辑配置
vim config.yaml

# 观察日志：
# [Config] 检测到配置文件变更，正在重载...
# [Config] 热更新成功！已加载 3 个监测任务
# [Scheduler] 配置已更新，下次巡检将使用新配置
```

## API 端点

```bash
# 健康检查
curl http://localhost:8080/health

# 获取状态（默认 24h）
curl http://localhost:8080/api/status

# 查询参数：
# - period: "90m", "24h", "7d", "30d" (默认: "24h")
# - align: 时间对齐模式，"hour"=整点对齐 (可选)
# - time_filter: 每日时段过滤，格式 HH:MM-HH:MM (UTC)，仅 7d/30d 可用
# - provider: 按 provider 名称过滤
# - service: 按 service 名称过滤
curl "http://localhost:8080/api/status?period=7d&provider=88code"

# 时段过滤示例：只看工作时间 (09:00-17:00 UTC)
curl "http://localhost:8080/api/status?period=7d&time_filter=09:00-17:00"

# 跨午夜时段示例：晚高峰 (22:00-04:00 UTC，跨越午夜)
curl "http://localhost:8080/api/status?period=30d&time_filter=22:00-04:00"
```

**响应格式**:
```json
{
  "meta": {"period": "24h", "count": 3},
  "data": [
    {
      "provider": "88code",
      "service": "cc",
      "current_status": {"status": 1, "latency": 234, "timestamp": 1735559123},
      "timeline": [{"time": "14:30", "status": 1, "latency": 234}, ...]
    }
  ]
}
```

## 测试

### 后端测试

- 测试文件与源文件放在一起（`*_test.go`）
- 关键测试文件：`internal/config/config_test.go`、`internal/monitor/probe_test.go`、`internal/api/time_filter_test.go`
- 使用 `go test -v` 查看详细输出

### 前端测试

- 测试框架：Vitest
- 测试文件：`frontend/src/utils/*.test.ts`
- 关键测试：`sortMonitors.test.ts` - 排序逻辑单元测试（主排序、二级延迟排序、边界情况）

```bash
cd frontend

# 运行测试
npm run test

# 监听模式（开发时使用）
npm run test:watch
```

### 手动集成测试

```bash
# 终端 1：启动后端
./monitor

# 终端 2：启动前端
cd frontend && npm run dev

# 终端 3：测试 API
curl http://localhost:8080/api/status

# 测试热更新
vim config.yaml  # 修改 interval 为 "30s"
# 观察调度器日志中的配置重载信息
```

## 提交信息规范

遵循 conventional commits：

```
<type>: <subject>

<body>

<footer>
```

**类型**: `feat`、`fix`、`docs`、`refactor`、`test`、`chore`

**示例**:
```
feat: add response content validation with success_contains

- Add success_contains field to ServiceConfig
- Implement keyword matching in probe.go
- Update config.yaml.example with usage

Closes #42
```

## 常见模式与陷阱

### Scheduler 中的并发

调度器使用两个锁：
- `cfgMu` (RWMutex): 保护配置访问
- `mu` (Mutex): 保护调度器状态（运行标志、定时器）

对于只读配置访问，始终使用 `RLock()/RUnlock()`。

### SQLite 并发

使用 WAL 模式（`_journal_mode=WAL`）允许写入时并发读取。连接 DSN：`file:monitor.db?_journal_mode=WAL`

### Probe 中的错误处理

- 网络错误 → 状态 0（红色）
- HTTP 4xx/5xx → 状态 0（红色）
- HTTP 2xx + 慢延迟 → 状态 2（黄色）
- HTTP 2xx + 快速 + 内容匹配 → 状态 1（绿色）

### 前端数据获取

`useMonitorData` Hook 每 30 秒轮询 `/api/status`。组件卸载时需禁用轮询以防止内存泄漏。

## 生产部署

### 环境变量（推荐）

```bash
export MONITOR_88CODE_CC_API_KEY="sk-real-key"
export MONITOR_DUCKCODING_CC_API_KEY="sk-duck-key"
./monitor
```

### Systemd 服务

参见 README.md 中的 systemd unit 文件模板。

### Docker

参见 README.md 中的多阶段 Dockerfile。

## 相关文档

- 完整开发指南：`CONTRIBUTING.md`
- API 设计细节：`archive/prds.md`（历史参考）
- 实现笔记：`archive/IMPLEMENTATION.md`（历史参考）
- 每次提交代码前记得检测, 是否有变动需要同步到文档
- 在commit前应先进行代码格式检查
- 每次任务完成后, 别急着提交, 应该找codex评审通过后再提交
