import { useTranslation } from 'react-i18next';
import type { RiskBadge as RiskBadgeType } from '../../types';

interface RiskBadgeProps {
  risk: RiskBadgeType;
  className?: string;
}

/**
 * 风险徽标图标 - 警告三角形（黄色，缓和）
 */
function RiskIcon() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      {/* 警告三角形 */}
      <polygon
        points="12,3 22,21 2,21"
        className="fill-amber-500/80"
      />
      {/* 感叹号 */}
      <rect x="11" y="9" width="2" height="6" fill="white" rx="0.5" />
      <circle cx="12" cy="17.5" r="1" fill="white" />
    </svg>
  );
}

/**
 * 风险徽标组件
 * 显示警告三角图标，可附带链接指向讨论页面
 */
export function RiskBadge({ risk, className = '' }: RiskBadgeProps) {
  const { t } = useTranslation();
  const defaultTooltip = t('badges.risk.tooltip');
  const hasLink = Boolean(risk.discussionUrl);

  const content = (
    <span
      className={`relative group/risk inline-flex items-center select-none ${hasLink ? 'cursor-pointer' : 'cursor-default'} ${className}`}
      role="img"
      aria-label={`${risk.label}: ${defaultTooltip}`}
    >
      <RiskIcon />
      {/* 延迟 tooltip - 悬停 700ms 后显示 */}
      <span className="absolute top-full left-0 mt-1 px-2 py-1 bg-slate-800 text-slate-200 text-xs rounded opacity-0 group-hover/risk:opacity-100 pointer-events-none transition-opacity delay-700 whitespace-nowrap z-50">
        <span className="font-medium text-amber-400">{risk.label}</span>
        {hasLink && (
          <span className="text-slate-400 ml-1">- {t('badges.risk.clickToView')}</span>
        )}
      </span>
    </span>
  );

  // 如果有讨论链接，包裹为可点击链接
  if (hasLink) {
    return (
      <a
        href={risk.discussionUrl}
        target="_blank"
        rel="noopener noreferrer"
        className="inline-flex hover:opacity-80 transition-opacity"
        onClick={(e) => e.stopPropagation()}
      >
        {content}
      </a>
    );
  }

  return content;
}
