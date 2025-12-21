import { useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { useBadgeTooltip } from '../../hooks/useBadgeTooltip';
import { BadgeTooltip } from './BadgeTooltip';

interface PublicBadgeProps {
  className?: string;
  tooltipPlacement?: 'top' | 'bottom';
}

/**
 * 公益站徽标组件
 * 显示蓝色"益"标签，表示公益服务站
 */
export function PublicBadge({ className = '', tooltipPlacement = 'top' }: PublicBadgeProps) {
  const { t } = useTranslation();
  const triggerRef = useRef<HTMLSpanElement>(null);
  const { isOpen, position, handleMouseEnter, handleMouseLeave } = useBadgeTooltip(
    triggerRef,
    tooltipPlacement
  );

  const label = t('table.categoryShort.charity');
  const tooltip = t('badges.public.tooltip');

  return (
    <>
      <span
        ref={triggerRef}
        onMouseEnter={handleMouseEnter}
        onMouseLeave={handleMouseLeave}
        className={`inline-flex items-center ${className}`}
        role="img"
        aria-label={`${label}: ${tooltip}`}
      >
        <span className="px-1.5 py-0.5 text-[10px] font-medium bg-info/20 text-info rounded cursor-default select-none">
          {label}
        </span>
      </span>

      <BadgeTooltip isOpen={isOpen} position={position}>
        <span className="font-medium text-info">{label}</span>
        <span className="text-secondary ml-1">- {tooltip}</span>
      </BadgeTooltip>
    </>
  );
}
