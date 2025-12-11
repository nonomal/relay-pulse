# Service Horizon 前端

LLM 服务监测系统的前端界面，使用 React + TypeScript + TailwindCSS 构建。

## 功能特性

- 🌐 **多语言支持**: 中文、English、Русский、日本語四语言切换
- 📊 **双视图模式**: 表格视图和卡片视图
- 🔍 **智能筛选**: 按服务商和服务类型筛选
- 📅 **时间范围**: 支持 24h、7d、30d
- 📈 **热力图**: GitHub 风格的状态历史展示
- 🎯 **实时统计**: 正常运行数和异常告警数
- 🔄 **排序功能**: 按服务商、服务类型、状态、可用率排序
- 💡 **悬浮提示**: 鼠标悬停显示详细信息
- 📱 **响应式设计**: 支持桌面端和移动端自适应

## 技术栈

- **框架**: React 19 + TypeScript
- **构建工具**: Vite
- **样式**: Tailwind CSS v4
- **图标**: lucide-react
- **HTTP**: Fetch API
- **国际化**: react-i18next + i18next
- **路由**: react-router-dom v6
- **SEO**: react-helmet-async

## 项目结构

```
frontend/
├── src/
│   ├── components/       # React 组件
│   │   ├── Header.tsx
│   │   ├── Controls.tsx
│   │   ├── StatusTable.tsx
│   │   ├── StatusCard.tsx
│   │   ├── StatusDot.tsx
│   │   ├── HeatmapBlock.tsx
│   │   ├── Tooltip.tsx
│   │   └── Footer.tsx
│   ├── hooks/           # 自定义 Hooks
│   │   └── useMonitorData.ts
│   ├── i18n/            # 国际化配置
│   │   ├── index.ts           # i18n 配置入口
│   │   └── locales/           # 翻译文件
│   │       ├── zh-CN.json     # 简体中文
│   │       ├── en-US.json     # 英语
│   │       ├── ru-RU.json     # 俄语
│   │       └── ja-JP.json     # 日语
│   ├── types/           # TypeScript 类型定义
│   │   └── index.ts
│   ├── constants/       # 常量配置
│   │   └── index.ts
│   ├── utils/           # 工具函数
│   │   ├── color.ts
│   │   ├── heatmapAggregator.ts
│   │   └── mediaQuery.ts
│   ├── App.tsx          # 主应用组件
│   ├── router.tsx       # 路由配置
│   ├── main.tsx         # 应用入口
│   └── index.css        # 全局样式
├── .env.development     # 开发环境变量
├── .env.production      # 生产环境变量
└── package.json
```

## 快速开始

### 安装依赖

```bash
npm install
```

### 开发模式

```bash
npm run dev
```

访问 http://localhost:5173

### 生产构建

```bash
npm run build
```

构建产物位于 `dist/` 目录

### 预览生产版本

```bash
npm run preview
```

## 环境变量

在 `.env.development` 或 `.env.production` 中配置：

```env
VITE_API_BASE_URL=http://localhost:8080
```

## API 对接

前端通过 `GET /api/status` 接口获取监测数据：

- 参数: `period` (24h/7d/30d), `provider` (服务商), `service` (服务类型)
- 返回: `{ meta: {...}, data: [{provider, service, current_status, timeline}] }`

## 开发说明

### 国际化 (i18n)

#### 支持的语言

- 🇨🇳 **中文** (zh-CN) - 默认语言，无路径前缀 `/`
- 🇺🇸 **English** (en-US) - 路径前缀 `/en-US/`
- 🇷🇺 **Русский** (ru-RU) - 路径前缀 `/ru-RU/`
- 🇯🇵 **日本語** (ja-JP) - 路径前缀 `/ja-JP/`

#### URL 路由规则

```
/                    → 根据浏览器语言自动检测（无语言前缀时）
/en-US/              → 英文
/ru-RU/              → 俄文
/ja-JP/              → 日文
```

**语言检测优先级**: URL 路径 > localStorage > 浏览器语言 > 默认中文

- 当访问 `/` 时，系统会根据浏览器语言自动选择合适的语言
- 如果检测到的语言不在支持列表中，则使用默认中文
- 语言切换时会保留当前页面的查询参数和路径

#### 添加新语言

1. 在 `src/i18n/locales/` 创建新翻译文件（如 `fr-FR.json`）
2. 复制现有翻译文件结构，翻译所有键值
3. 在 `src/i18n/index.ts` 中添加语言配置：

```typescript
import frFR from './locales/fr-FR.json';

export const LANGUAGE_NAMES: Record<string, { native: string; english: string; flag: string }> = {
  // ...
  'fr-FR': { native: 'Français', english: 'French', flag: '🇫🇷' },
};

export const SUPPORTED_LANGUAGES = ['zh-CN', 'en-US', 'ru-RU', 'ja-JP', 'fr-FR'] as const;

// 在 resources 中添加
resources: {
  // ...
  'fr-FR': { translation: frFR },
}
```

#### 修改翻译内容

编辑对应语言的 JSON 文件（`src/i18n/locales/*.json`），所有翻译文件结构必须保持一致。

#### 在组件中使用翻译

```typescript
import { useTranslation } from 'react-i18next';

function MyComponent() {
  const { t } = useTranslation();

  return (
    <div>
      <h1>{t('header.title')}</h1>
      <p>{t('common.loading')}</p>
      {/* 带参数的翻译 */}
      <span>{t('common.error', { message: 'Network timeout' })}</span>
    </div>
  );
}
```

#### 使用动态翻译常量

对于需要翻译的常量（如状态标签、时间范围），使用工厂函数：

```typescript
import { useTranslation } from 'react-i18next';
import { getStatusConfig, getTimeRanges } from '../constants';

function MyComponent() {
  const { t } = useTranslation();
  const STATUS = getStatusConfig(t);      // 动态状态配置
  const timeRanges = getTimeRanges(t);    // 动态时间范围

  return <span className={STATUS.AVAILABLE.text}>{STATUS.AVAILABLE.label}</span>;
}
```

### 添加新服务商

编辑 `src/constants/index.ts` 的 `PROVIDERS` 数组：

```typescript
export const PROVIDERS: Provider[] = [
  { id: 'new-provider', name: 'New Provider', services: ['cc', 'cx'] },
  // ...
];
```

### 修改时间范围

编辑 `src/constants/index.ts` 的 `TIME_RANGES` 数组：

```typescript
export const TIME_RANGES: TimeRange[] = [
  { id: '1h', label: '近1小时', points: 60, unit: 'hour' },
  // ...
];
```

## 浏览器支持

- Chrome/Edge (最新版)
- Firefox (最新版)
- Safari (最新版)

## License

MIT
