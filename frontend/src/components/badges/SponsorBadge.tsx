import { useRef } from 'react';
import { useTranslation } from 'react-i18next';
import type { SponsorLevel } from '../../types';
import { useBadgeTooltip } from '../../hooks/useBadgeTooltip';
import { BadgeTooltip } from './BadgeTooltip';

interface SponsorBadgeProps {
  level: SponsorLevel;
  className?: string;
  tooltipPlacement?: 'top' | 'bottom';
}

// 🔺 节点支持：正三角形（实心，指向上）
function BasicBadge() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <polygon
        points="12,4 4,18 20,18"
        className="fill-sponsor-basic"
      />
    </svg>
  );
}

// ⬢ 核心服务商：实心六边形
function AdvancedBadge() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <polygon
        points="12,2 21,7 21,17 12,22 3,17 3,7"
        className="fill-sponsor-advanced"
      />
    </svg>
  );
}

// 💠 全球伙伴：钻石形（带中心光点）
function EnterpriseBadge() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      {/* 外层菱形 */}
      <polygon
        points="12,2 22,12 12,22 2,12"
        className="fill-sponsor-enterprise"
      />
      {/* 中心光点 */}
      <circle
        cx="12"
        cy="12"
        r="3"
        fill="rgba(255,255,255,0.6)"
      />
    </svg>
  );
}

// 赞助商等级对应的徽章组件
const SPONSOR_BADGES: Record<SponsorLevel, React.FC> = {
  basic: BasicBadge,
  advanced: AdvancedBadge,
  enterprise: EnterpriseBadge,
};

/**
 * 赞助商徽章组件
 * 显示 SVG 图标，hover 700ms 后显示 tooltip（包含名称和描述）
 */
export function SponsorBadge({ level, className = '', tooltipPlacement = 'top' }: SponsorBadgeProps) {
  const { t } = useTranslation();
  const triggerRef = useRef<HTMLSpanElement>(null);
  const { isOpen, position, handleMouseEnter, handleMouseLeave } = useBadgeTooltip(
    triggerRef,
    tooltipPlacement
  );

  const BadgeIcon = SPONSOR_BADGES[level];
  const name = t(`badges.sponsor.${level}.name`);
  const tooltip = t(`badges.sponsor.${level}.tooltip`);

  return (
    <>
      <span
        ref={triggerRef}
        onMouseEnter={handleMouseEnter}
        onMouseLeave={handleMouseLeave}
        className={`inline-flex items-center cursor-default select-none ${className}`}
        role="img"
        aria-label={`${name}: ${tooltip}`}
      >
        <BadgeIcon />
      </span>

      <BadgeTooltip isOpen={isOpen} position={position}>
        <span className="font-medium">{name}</span>
        <span className="text-secondary ml-1">- {tooltip}</span>
      </BadgeTooltip>
    </>
  );
}
