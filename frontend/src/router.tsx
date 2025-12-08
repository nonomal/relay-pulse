import { lazy, Suspense } from 'react';
import { Routes, Route, Navigate, Outlet } from 'react-router-dom';
import { useSyncLanguage } from './hooks/useSyncLanguage';

// 路由级代码分割：懒加载页面组件
const App = lazy(() => import('./App'));
const ProviderPage = lazy(() => import('./pages/ProviderPage'));

/**
 * 语言布局组件
 *
 * 职责：
 * 1. 接收固定的语言前缀（如 'en'、'ru'、'ja'）
 * 2. 使用 useSyncLanguage Hook 同步语言状态
 * 3. 使用 Outlet 渲染匹配的子路由（App 或 ProviderPage）
 */
interface LanguageLayoutProps {
  /** 语言前缀（如 'en'、'ru'、'ja'），无前缀则为 undefined */
  lang?: string;
}

function LanguageLayout({ lang }: LanguageLayoutProps) {
  useSyncLanguage(lang);
  return <Outlet />;
}

/**
 * 路由级加载占位符
 * 使用与主题一致的背景色和三点跳动动画，避免视觉跳跃
 * 注意：使用 CSS 变量以支持主题切换
 */
function RouterFallback() {
  return (
    <div
      style={{
        minHeight: '100vh',
        backgroundColor: 'hsl(var(--bg-page, 222 47% 4%))',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        gap: '6px',
      }}
    >
      {[0, 0.15, 0.3].map((delay, i) => (
        <div
          key={i}
          style={{
            width: '10px',
            height: '10px',
            borderRadius: '50%',
            background: 'var(--gradient-button, linear-gradient(135deg, #06b6d4, #3b82f6))',
            animation: `bounce 0.6s ease-in-out ${delay}s infinite alternate`,
          }}
        />
      ))}
      <style>{`
        @keyframes bounce {
          from { transform: translateY(0); opacity: 0.4; }
          to { transform: translateY(-12px); opacity: 1; }
        }
      `}</style>
    </div>
  );
}

/**
 * 应用路由配置
 *
 * 路由规则：
 * 1. 根路径 `/` 和 `/p/:provider` → 默认语言（中文，由 i18n 检测器决定）
 * 2. 明确的语言前缀路径：
 *    - `/en` 和 `/en/p/:provider` → 英文
 *    - `/ru` 和 `/ru/p/:provider` → 俄文
 *    - `/ja` 和 `/ja/p/:provider` → 日文
 * 3. 无效路径 → 重定向到根路径
 *
 * 嵌套路由结构：
 * - LanguageLayout 负责语言同步
 * - Outlet 渲染匹配的子路由（App 或 ProviderPage）
 *
 * 注意：
 * - 使用明确的路径前缀（/en、/ru、/ja）而非参数（:lang），避免与 /p/:provider 冲突
 * - `/api/*`、`/health` 等技术路径由后端处理，不会被前端路由拦截
 * - 所有内容页面（App、ProviderPage）自动获得 i18n 支持
 */
export default function AppRouter() {
  return (
    <Suspense fallback={<RouterFallback />}>
      <Routes>
        {/* 中文默认路径（无前缀） */}
        <Route element={<LanguageLayout />}>
          <Route index element={<App />} />
          <Route path="p/:provider" element={<ProviderPage />} />
        </Route>

        {/* 英文路径 */}
        <Route path="en" element={<LanguageLayout lang="en" />}>
          <Route index element={<App />} />
          <Route path="p/:provider" element={<ProviderPage />} />
        </Route>

        {/* 俄文路径 */}
        <Route path="ru" element={<LanguageLayout lang="ru" />}>
          <Route index element={<App />} />
          <Route path="p/:provider" element={<ProviderPage />} />
        </Route>

        {/* 日文路径 */}
        <Route path="ja" element={<LanguageLayout lang="ja" />}>
          <Route index element={<App />} />
          <Route path="p/:provider" element={<ProviderPage />} />
        </Route>

        {/* 捕获所有未匹配路径，重定向到根 */}
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </Suspense>
  );
}
