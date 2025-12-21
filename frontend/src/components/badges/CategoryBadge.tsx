import { useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { useBadgeTooltip } from '../../hooks/useBadgeTooltip';
import { BadgeTooltip } from './BadgeTooltip';

interface CategoryBadgeProps {
  category: 'commercial' | 'public';
  className?: string;
  tooltipPlacement?: 'top' | 'bottom';
}

/**
 * 站点类型徽标组件
 * - commercial: 不渲染（商业站是默认情况）
 * - public: 显示"益"（蓝色）
 */
export function CategoryBadge({ category, className = '', tooltipPlacement = 'top' }: CategoryBadgeProps) {
  const { t } = useTranslation();
  const triggerRef = useRef<HTMLSpanElement>(null);
  const { isOpen, position, handleMouseEnter, handleMouseLeave } = useBadgeTooltip(
    triggerRef,
    tooltipPlacement
  );

  // 商业站不渲染任何内容
  if (category !== 'public') {
    return null;
  }

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
        <span className="inline-flex items-center justify-center w-4 h-4 text-[10px] font-bold bg-info/20 text-info rounded cursor-default select-none">
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
