import type { ReactNode } from 'react';
import { useAuth } from '../../hooks/useAuth';
import { useTranslation } from 'react-i18next';
import { GitHubLoginButton } from './GitHubLoginButton';

interface ProtectedRouteProps {
  children: ReactNode;
  requireAdmin?: boolean;
}

// 加载中占位组件
function LoadingSpinner() {
  return (
    <div className="flex items-center justify-center min-h-[400px]">
      <div className="animate-spin rounded-full h-8 w-8 border-2 border-accent border-t-transparent" />
    </div>
  );
}

// 未登录提示组件
function LoginPrompt() {
  const { t } = useTranslation();

  return (
    <div className="flex flex-col items-center justify-center min-h-[400px] gap-6 px-4">
      <div className="text-center">
        <h2 className="text-xl font-semibold text-primary mb-2">
          {t('auth.loginRequired', 'Login Required')}
        </h2>
        <p className="text-secondary max-w-md">
          {t('auth.loginRequiredDesc', 'Please sign in with your GitHub account to access this page.')}
        </p>
      </div>
      <GitHubLoginButton size="lg" />
    </div>
  );
}

// 无权限提示组件
function AccessDenied() {
  const { t } = useTranslation();

  return (
    <div className="flex flex-col items-center justify-center min-h-[400px] gap-4 px-4">
      <div className="w-16 h-16 rounded-full bg-danger/20 flex items-center justify-center">
        <svg
          className="w-8 h-8 text-danger"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
          />
        </svg>
      </div>
      <div className="text-center">
        <h2 className="text-xl font-semibold text-primary mb-2">
          {t('auth.accessDenied', 'Access Denied')}
        </h2>
        <p className="text-secondary max-w-md">
          {t('auth.accessDeniedDesc', 'You do not have permission to access this page. Please contact an administrator if you believe this is an error.')}
        </p>
      </div>
      <a
        href="/"
        className="mt-4 px-4 py-2 bg-surface hover:bg-elevated text-primary rounded-lg transition-colors"
      >
        {t('auth.backToHome', 'Back to Home')}
      </a>
    </div>
  );
}

// 账号被禁用提示组件
function AccountDisabled() {
  const { t } = useTranslation();
  const { logout } = useAuth();

  return (
    <div className="flex flex-col items-center justify-center min-h-[400px] gap-4 px-4">
      <div className="w-16 h-16 rounded-full bg-warning/20 flex items-center justify-center">
        <svg
          className="w-8 h-8 text-warning"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728A9 9 0 015.636 5.636m12.728 12.728L5.636 5.636"
          />
        </svg>
      </div>
      <div className="text-center">
        <h2 className="text-xl font-semibold text-primary mb-2">
          {t('auth.accountDisabled', 'Account Disabled')}
        </h2>
        <p className="text-secondary max-w-md">
          {t('auth.accountDisabledDesc', 'Your account has been disabled. Please contact an administrator for assistance.')}
        </p>
      </div>
      <button
        onClick={() => logout()}
        className="mt-4 px-4 py-2 bg-surface hover:bg-elevated text-primary rounded-lg transition-colors"
      >
        {t('auth.logout', 'Sign out')}
      </button>
    </div>
  );
}

export function ProtectedRoute({ children, requireAdmin = false }: ProtectedRouteProps) {
  const { isLoading, isAuthenticated, isAdmin, user } = useAuth();

  // 加载中
  if (isLoading) {
    return <LoadingSpinner />;
  }

  // 未登录
  if (!isAuthenticated) {
    return <LoginPrompt />;
  }

  // 账号被禁用
  if (user?.status === 'disabled') {
    return <AccountDisabled />;
  }

  // 需要管理员权限但不是管理员
  if (requireAdmin && !isAdmin) {
    return <AccessDenied />;
  }

  // 通过所有检查，渲染子组件
  return <>{children}</>;
}

export default ProtectedRoute;
