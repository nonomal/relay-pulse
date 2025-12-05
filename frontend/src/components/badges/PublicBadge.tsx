import { useTranslation } from 'react-i18next';

interface PublicBadgeProps {
  className?: string;
}

/**
 * 公益站徽标组件
 * 显示蓝色"益"标签，表示公益服务站
 */
export function PublicBadge({ className = '' }: PublicBadgeProps) {
  const { t } = useTranslation();
  const label = t('table.categoryShort.charity');
  const tooltip = t('badges.public.tooltip');

  return (
    <span
      className={`relative group/public inline-flex items-center ${className}`}
      role="img"
      aria-label={`${label}: ${tooltip}`}
    >
      <span className="px-1.5 py-0.5 text-[10px] font-medium bg-blue-500/20 text-blue-400 rounded cursor-default select-none">
        {label}
      </span>
      {/* 延迟 tooltip - 悬停 700ms 后显示 */}
      <span className="absolute top-full left-0 mt-1 px-2 py-1 bg-slate-800 text-slate-200 text-xs rounded opacity-0 group-hover/public:opacity-100 pointer-events-none transition-opacity delay-700 whitespace-nowrap z-50">
        <span className="font-medium text-blue-400">{label}</span>
        <span className="text-slate-400 ml-1">- {tooltip}</span>
      </span>
    </span>
  );
}
