import { useState, useRef, useEffect } from 'react';
import { useAuth } from '../../hooks/useAuth';
import { useTranslation } from 'react-i18next';

interface UserMenuProps {
  className?: string;
}

export function UserMenu({ className = '' }: UserMenuProps) {
  const { user, logout, isAdmin } = useAuth();
  const { t } = useTranslation();
  const [isOpen, setIsOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  // 点击外部关闭菜单
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    }

    if (isOpen) {
      document.addEventListener('mousedown', handleClickOutside);
      return () => document.removeEventListener('mousedown', handleClickOutside);
    }
  }, [isOpen]);

  // ESC 键关闭菜单
  useEffect(() => {
    function handleEscape(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        setIsOpen(false);
      }
    }

    if (isOpen) {
      document.addEventListener('keydown', handleEscape);
      return () => document.removeEventListener('keydown', handleEscape);
    }
  }, [isOpen]);

  if (!user) {
    return null;
  }

  const handleLogout = async () => {
    setIsOpen(false);
    await logout();
  };

  return (
    <div ref={menuRef} className={`relative ${className}`}>
      {/* 用户头像按钮 */}
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="flex items-center gap-2 p-1 rounded-full hover:bg-surface/60 transition-colors"
        aria-expanded={isOpen}
        aria-haspopup="true"
      >
        <img
          src={user.avatar_url}
          alt={user.username}
          className="w-8 h-8 rounded-full ring-2 ring-transparent hover:ring-accent/50 transition-all"
        />
        <span className="text-sm text-secondary hidden sm:inline">
          {user.username}
        </span>
        {/* 下拉箭头 */}
        <svg
          className={`w-4 h-4 text-muted transition-transform ${isOpen ? 'rotate-180' : ''}`}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
        </svg>
      </button>

      {/* 下拉菜单 */}
      {isOpen && (
        <div className="absolute right-0 mt-2 w-56 bg-elevated rounded-lg shadow-lg ring-1 ring-black/10 z-50">
          {/* 用户信息 */}
          <div className="px-4 py-3 border-b border-muted/20">
            <p className="text-sm font-medium text-primary">{user.username}</p>
            {user.email && (
              <p className="text-xs text-muted truncate">{user.email}</p>
            )}
            {isAdmin && (
              <span className="inline-block mt-1 px-2 py-0.5 text-xs font-medium bg-accent/20 text-accent rounded">
                {t('auth.admin', 'Admin')}
              </span>
            )}
          </div>

          {/* 菜单项 */}
          <div className="py-1">
            {/* 我的申请 */}
            <a
              href="/user/applications"
              className="block px-4 py-2 text-sm text-secondary hover:bg-surface hover:text-primary transition-colors"
              onClick={() => setIsOpen(false)}
            >
              {t('auth.myApplications', 'My Applications')}
            </a>

            {/* 管理后台（仅管理员） */}
            {isAdmin && (
              <a
                href="/admin"
                className="block px-4 py-2 text-sm text-secondary hover:bg-surface hover:text-primary transition-colors"
                onClick={() => setIsOpen(false)}
              >
                {t('auth.adminPanel', 'Admin Panel')}
              </a>
            )}

            {/* 分隔线 */}
            <div className="my-1 border-t border-muted/20" />

            {/* 登出 */}
            <button
              onClick={handleLogout}
              className="w-full text-left px-4 py-2 text-sm text-danger hover:bg-surface transition-colors"
            >
              {t('auth.logout', 'Sign out')}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

export default UserMenu;
