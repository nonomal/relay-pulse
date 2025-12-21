import { useTranslation } from 'react-i18next';

interface CategoryBadgeProps {
  category: 'commercial' | 'public';
  className?: string;
}

/**
 * 站点类型徽标组件
 * - commercial: 不渲染（商业站是默认情况）
 * - public: 显示"益"（蓝色）
 */
export function CategoryBadge({ category, className = '' }: CategoryBadgeProps) {
  const { t } = useTranslation();

  // 商业站不渲染任何内容
  if (category !== 'public') {
    return null;
  }

  const label = t('table.categoryShort.charity');
  const tooltip = t('badges.public.tooltip');

  return (
    <span
      className={`relative group/category inline-flex items-center ${className}`}
      role="img"
      aria-label={`${label}: ${tooltip}`}
    >
      <span className="inline-flex items-center justify-center w-4 h-4 text-[10px] font-bold bg-info/20 text-info rounded cursor-default select-none">
        {label}
      </span>
      {/* 延迟 tooltip - 悬停 700ms 后显示 */}
      <span className="absolute top-full left-0 mt-1 px-2 py-1 bg-elevated text-primary text-xs rounded opacity-0 group-hover/category:opacity-100 pointer-events-none transition-opacity delay-700 whitespace-nowrap z-50">
        <span className="font-medium text-info">{label}</span>
        <span className="text-secondary ml-1">- {tooltip}</span>
      </span>
    </span>
  );
}
