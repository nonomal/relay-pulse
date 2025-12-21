import { useRef } from 'react';
import { useTranslation } from 'react-i18next';
import type { RiskBadge as RiskBadgeType } from '../../types';
import { useBadgeTooltip } from '../../hooks/useBadgeTooltip';
import { BadgeTooltip } from './BadgeTooltip';

interface RiskBadgeProps {
  risk: RiskBadgeType;
  className?: string;
  tooltipPlacement?: 'top' | 'bottom';
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
        className="fill-warning/80"
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
export function RiskBadge({ risk, className = '', tooltipPlacement = 'top' }: RiskBadgeProps) {
  const { t } = useTranslation();
  const triggerRef = useRef<HTMLSpanElement>(null);
  const { isOpen, position, handleMouseEnter, handleMouseLeave } = useBadgeTooltip(
    triggerRef,
    tooltipPlacement
  );

  const defaultTooltip = t('badges.risk.tooltip');
  const hasLink = Boolean(risk.discussionUrl);

  const content = (
    <span
      ref={triggerRef}
      onMouseEnter={handleMouseEnter}
      onMouseLeave={handleMouseLeave}
      className={`inline-flex items-center select-none ${hasLink ? 'cursor-pointer' : 'cursor-default'} ${className}`}
      role="img"
      aria-label={`${risk.label}: ${defaultTooltip}`}
    >
      <span className="inline-flex items-center justify-center">
        <RiskIcon />
      </span>
    </span>
  );

  const tooltipContent = (
    <BadgeTooltip isOpen={isOpen} position={position}>
      <span className="font-medium text-warning">{risk.label}</span>
      {hasLink && (
        <span className="text-secondary ml-1">- {t('badges.risk.clickToView')}</span>
      )}
    </BadgeTooltip>
  );

  // 如果有讨论链接，包裹为可点击链接
  if (hasLink) {
    return (
      <>
        <a
          href={risk.discussionUrl}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex hover:opacity-80 transition-opacity"
          onClick={(e) => e.stopPropagation()}
        >
          {content}
        </a>
        {tooltipContent}
      </>
    );
  }

  return (
    <>
      {content}
      {tooltipContent}
    </>
  );
}
