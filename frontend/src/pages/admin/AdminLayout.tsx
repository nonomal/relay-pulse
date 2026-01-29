import { useState, useCallback } from 'react';
import { Outlet, NavLink, Navigate } from 'react-router-dom';
import { Helmet } from 'react-helmet-async';
import {
  LayoutDashboard,
  Monitor,
  Award,
  Settings,
  LogOut,
  Menu,
  X,
  FileText,
  Users,
  ClipboardList,
  Layers,
  Server,
} from 'lucide-react';
import { useAdminAuth } from '../../hooks/admin/useAdminAuth';
import type { AdminOutletContext } from '../../hooks/admin/useAdminContext';

/** 侧边栏导航项 */
const NAV_ITEMS = [
  { to: '/admin/monitors', icon: Monitor, label: '监测项管理' },
  { to: '/admin/applications', icon: ClipboardList, label: '申请管理' },
  { to: '/admin/users', icon: Users, label: '用户管理' },
  { to: '/admin/templates', icon: Layers, label: '模板管理' },
  { to: '/admin/services', icon: Server, label: '服务管理' },
  { to: '/admin/badges', icon: Award, label: 'Badge 管理' },
  { to: '/admin/settings', icon: Settings, label: '全局设置' },
  { to: '/admin/audits', icon: FileText, label: '审计日志' },
] as const;

/** 登录表单 */
function LoginForm({ onLogin }: { onLogin: (token: string) => void }) {
  const [token, setToken] = useState('');
  const [error, setError] = useState('');
  const [checking, setChecking] = useState(false);

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      const trimmed = token.trim();
      if (!trimmed) {
        setError('请输入 Token');
        return;
      }

      setChecking(true);
      setError('');

      // 验证 token 有效性：尝试调用一个轻量级 API
      try {
        const resp = await fetch('/api/admin/config/version', {
          headers: { 'X-Config-Token': trimmed },
        });
        if (resp.ok) {
          onLogin(trimmed);
        } else if (resp.status === 401) {
          setError('Token 无效');
        } else if (resp.status === 503) {
          setError('管理 API 未启用，请检查服务端配置');
        } else {
          setError(`验证失败 (${resp.status})`);
        }
      } catch {
        setError('网络连接失败');
      } finally {
        setChecking(false);
      }
    },
    [token, onLogin],
  );

  return (
    <div className="min-h-screen bg-page flex items-center justify-center px-4">
      <div className="w-full max-w-sm">
        <div className="bg-surface border border-muted rounded-lg p-8">
          <div className="text-center mb-6">
            <LayoutDashboard className="w-10 h-10 text-accent mx-auto mb-3" />
            <h1 className="text-xl font-bold text-primary">RelayPulse Admin</h1>
            <p className="text-sm text-secondary mt-1">配置管理后台</p>
          </div>

          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label htmlFor="admin-token" className="block text-sm font-medium text-secondary mb-1">
                Config Token
              </label>
              <input
                id="admin-token"
                type="password"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                placeholder="请输入管理 Token"
                autoFocus
                className="w-full px-3 py-2 bg-elevated border border-muted rounded-md text-primary placeholder:text-muted focus:outline-none focus:ring-2 focus:ring-accent/50 focus:border-accent"
              />
            </div>

            {error && (
              <p className="text-sm text-danger">{error}</p>
            )}

            <button
              type="submit"
              disabled={checking}
              className="w-full px-4 py-2 bg-accent text-white rounded-md font-medium hover:bg-accent-strong transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {checking ? '验证中...' : '登录'}
            </button>
          </form>
        </div>
      </div>
    </div>
  );
}

/** Admin 布局（侧边栏 + 内容区） */
export default function AdminLayout() {
  const { isAuthenticated, login, logout, adminFetch } = useAdminAuth();
  const [sidebarOpen, setSidebarOpen] = useState(false);

  if (!isAuthenticated) {
    return (
      <>
        <Helmet>
          <title>Login | RelayPulse Admin</title>
          <meta name="robots" content="noindex,nofollow" />
        </Helmet>
        <LoginForm onLogin={login} />
      </>
    );
  }

  return (
    <>
      <Helmet>
        <title>RelayPulse Admin</title>
        <meta name="robots" content="noindex,nofollow" />
      </Helmet>

      <div className="min-h-screen bg-page flex">
        {/* 移动端侧边栏遮罩 */}
        {sidebarOpen && (
          <div
            className="fixed inset-0 bg-black/50 z-40 lg:hidden"
            onClick={() => setSidebarOpen(false)}
          />
        )}

        {/* 侧边栏 */}
        <aside
          className={`
            fixed inset-y-0 left-0 z-50 w-60 bg-surface border-r border-muted
            transform transition-transform duration-200 ease-in-out
            lg:relative lg:translate-x-0
            ${sidebarOpen ? 'translate-x-0' : '-translate-x-full'}
          `}
        >
          <div className="flex flex-col h-full">
            {/* Logo */}
            <div className="flex items-center justify-between h-14 px-4 border-b border-muted">
              <NavLink to="/admin" className="flex items-center gap-2">
                <LayoutDashboard className="w-5 h-5 text-accent" />
                <span className="font-bold text-primary text-sm">RP Admin</span>
              </NavLink>
              <button
                onClick={() => setSidebarOpen(false)}
                className="lg:hidden p-1 text-secondary hover:text-primary"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {/* 导航 */}
            <nav className="flex-1 py-3 px-2 space-y-0.5 overflow-y-auto">
              {NAV_ITEMS.map(({ to, icon: Icon, label }) => (
                <NavLink
                  key={to}
                  to={to}
                  onClick={() => setSidebarOpen(false)}
                  className={({ isActive }) =>
                    `flex items-center gap-2.5 px-3 py-2 rounded-md text-sm transition-colors ${
                      isActive
                        ? 'bg-accent/15 text-accent font-medium'
                        : 'text-secondary hover:text-primary hover:bg-elevated'
                    }`
                  }
                >
                  <Icon className="w-4 h-4 shrink-0" />
                  {label}
                </NavLink>
              ))}
            </nav>

            {/* 底部操作 */}
            <div className="p-3 border-t border-muted">
              <button
                onClick={logout}
                className="flex items-center gap-2 w-full px-3 py-2 rounded-md text-sm text-secondary hover:text-danger hover:bg-elevated transition-colors"
              >
                <LogOut className="w-4 h-4" />
                退出登录
              </button>
            </div>
          </div>
        </aside>

        {/* 主内容区 */}
        <div className="flex-1 flex flex-col min-w-0">
          {/* 顶栏（移动端） */}
          <header className="h-14 bg-surface border-b border-muted flex items-center px-4 lg:hidden">
            <button
              onClick={() => setSidebarOpen(true)}
              className="p-1 text-secondary hover:text-primary"
            >
              <Menu className="w-5 h-5" />
            </button>
            <span className="ml-3 font-bold text-primary text-sm">RP Admin</span>
          </header>

          {/* 页面内容 */}
          <main className="flex-1 p-4 lg:p-6 overflow-auto">
            <Outlet context={{ adminFetch } satisfies AdminOutletContext} />
          </main>
        </div>
      </div>
    </>
  );
}

/** Admin 首页（重定向到监测项页） */
export function AdminIndexRedirect() {
  return <Navigate to="/admin/monitors" replace />;
}
