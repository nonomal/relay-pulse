/**
 * 收藏按钮组件
 *
 * 功能：
 * - 显示星星图标（空心/实心）
 * - 点击切换收藏状态
 * - 支持键盘操作和无障碍
 */

import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { Star } from 'lucide-react';

export interface FavoriteButtonProps {
  /** 是否已收藏 */
  isFavorite: boolean;
  /** 切换收藏状态的回调 */
  onToggle: () => void;
  /** 尺寸（像素） */
  size?: number;
  /** 是否显示为内联样式（无背景） */
  inline?: boolean;
  /** 自定义类名 */
  className?: string;
}

export function FavoriteButton({
  isFavorite,
  onToggle,
  size = 14,
  inline = false,
  className = '',
}: FavoriteButtonProps) {
  const { t } = useTranslation();

  const handleClick = useCallback(
    (e: React.MouseEvent) => {
      e.stopPropagation(); // 防止触发行点击事件
      onToggle();
    },
    [onToggle]
  );

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        e.stopPropagation();
        onToggle();
      }
    },
    [onToggle]
  );

  const label = isFavorite ? t('favorites.remove') : t('favorites.add');

  if (inline) {
    return (
      <button
        type="button"
        onClick={handleClick}
        onKeyDown={handleKeyDown}
        className={`
          inline-flex items-center justify-center
          transition-colors duration-150
          focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none
          rounded
          ${isFavorite
            ? 'text-warning hover:text-warning/80'
            : 'text-muted hover:text-secondary'
          }
          ${className}
        `}
        aria-label={label}
        aria-pressed={isFavorite}
        title={label}
      >
        <Star
          size={size}
          fill={isFavorite ? 'currentColor' : 'none'}
          strokeWidth={isFavorite ? 0 : 2}
        />
      </button>
    );
  }

  return (
    <button
      type="button"
      onClick={handleClick}
      onKeyDown={handleKeyDown}
      className={`
        p-1.5 rounded-md
        transition-all duration-150
        focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none
        ${isFavorite
          ? 'text-warning bg-warning/10 hover:bg-warning/20'
          : 'text-muted hover:text-secondary hover:bg-muted/30'
        }
        ${className}
      `}
      aria-label={label}
      aria-pressed={isFavorite}
      title={label}
    >
      <Star
        size={size}
        fill={isFavorite ? 'currentColor' : 'none'}
        strokeWidth={isFavorite ? 0 : 2}
      />
    </button>
  );
}

export default FavoriteButton;
