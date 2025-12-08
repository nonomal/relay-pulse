/**
 * 主题切换器组件
 *
 * 功能：
 * - 显示当前主题图标
 * - 点击/悬浮展开主题列表
 * - 支持桌面端 hover 和移动端 click
 */

import { useState, useEffect, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { Sun, Moon, Palette } from 'lucide-react';
import { useTheme, type ThemeId } from '../hooks/useTheme';

/**
 * 获取主题图标
 */
function ThemeIcon({ themeId, size = 16 }: { themeId: ThemeId; size?: number }) {
  switch (themeId) {
    case 'default-dark':
      return <Moon size={size} />;
    case 'night-dark':
      return <Moon size={size} className="text-warning" />;
    case 'light-cool':
      return <Sun size={size} />;
    case 'light-warm':
      return <Sun size={size} className="text-warning" />;
    default:
      return <Palette size={size} />;
  }
}

export function ThemeSwitcher() {
  const { t } = useTranslation();
  const { theme, setTheme, themes } = useTheme();
  const [showMenu, setShowMenu] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  // 点击外部关闭菜单
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setShowMenu(false);
      }
    }

    if (showMenu) {
      document.addEventListener('mousedown', handleClickOutside);
      return () => document.removeEventListener('mousedown', handleClickOutside);
    }
  }, [showMenu]);

  // ESC 关闭菜单
  useEffect(() => {
    function handleEscape(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        setShowMenu(false);
      }
    }

    if (showMenu) {
      document.addEventListener('keydown', handleEscape);
      return () => document.removeEventListener('keydown', handleEscape);
    }
  }, [showMenu]);

  const handleThemeChange = (newTheme: ThemeId) => {
    setTheme(newTheme);
    setShowMenu(false);
  };

  return (
    <div ref={menuRef} className="relative group">
      {/* 触发按钮 - 仅图标 */}
      <button
        onClick={() => setShowMenu(!showMenu)}
        className="p-2 rounded-lg bg-elevated/50 hover:bg-muted/50 transition-all duration-200 focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none"
        aria-label={t('theme.switchTheme')}
        aria-expanded={showMenu}
        aria-haspopup="listbox"
      >
        <ThemeIcon themeId={theme} size={16} />
      </button>

      {/* 下拉菜单 - 桌面端 hover 显示，移动端 click 显示 */}
      <div
        className={`
          absolute top-full right-0 mt-1 z-50
          bg-elevated border border-default rounded-lg shadow-xl
          transition-all duration-200
          ${showMenu ? 'opacity-100 visible translate-y-0' : 'opacity-0 invisible -translate-y-2'}
          lg:group-hover:opacity-100 lg:group-hover:visible lg:group-hover:translate-y-0
        `}
        role="listbox"
        aria-label={t('theme.selectTheme')}
      >
        {themes.map((t_) => (
          <button
            key={t_.id}
            onClick={() => handleThemeChange(t_.id)}
            className={`
              w-full flex items-center justify-center p-2
              transition-colors cursor-pointer first:rounded-t-lg last:rounded-b-lg
              focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none
              ${theme === t_.id ? 'bg-accent/20 text-accent' : 'text-primary hover:bg-muted/50'}
            `}
            role="option"
            aria-selected={theme === t_.id}
            aria-label={t(t_.nameKey)}
          >
            <ThemeIcon themeId={t_.id} size={16} />
          </button>
        ))}
      </div>
    </div>
  );
}

export default ThemeSwitcher;
