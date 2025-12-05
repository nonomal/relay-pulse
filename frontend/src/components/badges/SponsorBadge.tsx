import { useTranslation } from 'react-i18next';
import type { SponsorLevel } from '../../types';

interface SponsorBadgeProps {
  level: SponsorLevel;
  className?: string;
}

// 🔺 节点支持：正三角形（实心，指向上）
function BasicBadge() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <polygon
        points="12,4 4,18 20,18"
        className="fill-emerald-500/80"
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
        className="fill-cyan-500/80"
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
        className="fill-amber-400"
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
export function SponsorBadge({ level, className = '' }: SponsorBadgeProps) {
  const { t } = useTranslation();
  const BadgeIcon = SPONSOR_BADGES[level];
  const name = t(`badges.sponsor.${level}.name`);
  const tooltip = t(`badges.sponsor.${level}.tooltip`);

  return (
    <span
      className={`relative group/sponsor inline-flex items-center cursor-default select-none ${className}`}
      role="img"
      aria-label={`${name}: ${tooltip}`}
    >
      <BadgeIcon />
      {/* 延迟 tooltip - 悬停 700ms 后显示，左对齐避免左侧裁剪 */}
      <span className="absolute top-full left-0 mt-1 px-2 py-1 bg-slate-800 text-slate-200 text-xs rounded opacity-0 group-hover/sponsor:opacity-100 pointer-events-none transition-opacity delay-700 whitespace-nowrap z-50">
        <span className="font-medium">{name}</span>
        <span className="text-slate-400 ml-1">- {tooltip}</span>
      </span>
    </span>
  );
}
