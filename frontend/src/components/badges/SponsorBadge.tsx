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

// 🛡️ 公益链路：盾牌
function PublicBadge() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path
        d="M12 2L4 6v5c0 5.25 3.4 10.15 8 11.25C16.6 21.15 20 16.25 20 11V6L12 2z"
        className="fill-sponsor-public"
      />
    </svg>
  );
}

// · 信号链路：光点（小圆）
function SignalBadge() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <circle
        cx="12"
        cy="12"
        r="4"
        className="fill-sponsor-signal"
      />
    </svg>
  );
}

// ◆ 脉冲链路：小菱形
function PulseBadge() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <polygon
        points="12,4 19,12 12,20 5,12"
        className="fill-sponsor-pulse"
      />
    </svg>
  );
}

// 🔺 信标链路：正三角形（实心，指向上）
function BeaconBadge() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <polygon
        points="12,4 4,18 20,18"
        className="fill-sponsor-beacon"
      />
    </svg>
  );
}

// ⬢ 骨干链路：实心六边形
function BackboneBadge() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <polygon
        points="12,2 21,7 21,17 12,22 3,17 3,7"
        className="fill-sponsor-backbone"
      />
    </svg>
  );
}

// 💠 核心链路：钻石形（带中心光点）
function CoreBadge() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      {/* 外层菱形 */}
      <polygon
        points="12,2 22,12 12,22 2,12"
        className="fill-sponsor-core"
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

// 赞助等级对应的徽章组件
const SPONSOR_BADGES: Record<SponsorLevel, React.FC> = {
  public: PublicBadge,
  signal: SignalBadge,
  pulse: PulseBadge,
  beacon: BeaconBadge,
  backbone: BackboneBadge,
  core: CoreBadge,
};

/**
 * 赞助徽章组件
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
