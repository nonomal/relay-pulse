import { useTranslation } from 'react-i18next';

interface CategoryBadgeProps {
  category: 'commercial' | 'public';
  className?: string;
}

/**
 * 站点类型徽标组件
 * - commercial: 显示"商"（灰色）
 * - public: 显示"益"（蓝色）
 */
export function CategoryBadge({ category, className = '' }: CategoryBadgeProps) {
  const { t } = useTranslation();

  const isPublic = category === 'public';
  const labelKey = isPublic ? 'table.categoryShort.charity' : 'table.categoryShort.promoted';
  const tooltipKey = isPublic ? 'badges.public.tooltip' : 'badges.commercial.tooltip';

  const label = t(labelKey);
  const tooltip = t(tooltipKey);

  // 公益站：蓝色，商业站：灰色
  const colorClasses = isPublic
    ? 'bg-blue-500/20 text-blue-400'
    : 'bg-slate-500/20 text-slate-400';

  const tooltipLabelColor = isPublic ? 'text-blue-400' : 'text-slate-400';

  return (
    <span
      className={`relative group/category inline-flex items-center ${className}`}
      role="img"
      aria-label={`${label}: ${tooltip}`}
    >
      <span className={`px-1.5 py-0.5 text-[10px] font-medium ${colorClasses} rounded cursor-default select-none`}>
        {label}
      </span>
      {/* 延迟 tooltip - 悬停 700ms 后显示 */}
      <span className="absolute top-full left-0 mt-1 px-2 py-1 bg-slate-800 text-slate-200 text-xs rounded opacity-0 group-hover/category:opacity-100 pointer-events-none transition-opacity delay-700 whitespace-nowrap z-50">
        <span className={`font-medium ${tooltipLabelColor}`}>{label}</span>
        <span className="text-slate-400 ml-1">- {tooltip}</span>
      </span>
    </span>
  );
}
