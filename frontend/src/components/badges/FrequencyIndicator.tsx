import { useMemo, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { useBadgeTooltip } from '../../hooks/useBadgeTooltip';
import { BadgeTooltip } from './BadgeTooltip';

interface FrequencyIndicatorProps {
  intervalMs: number;  // 监测间隔（毫秒），30000 ~ 300000 (30s ~ 5min)
  className?: string;
  tooltipPlacement?: 'top' | 'bottom';
}

/**
 * 根据监测间隔计算透明度
 * 30s = 1.0（高频，最亮） → 5min = 0.3（低频，较暗）
 * 透明度差异更明显，便于区分不同频率
 */
function getFrequencyOpacity(intervalMs: number): number {
  const min = 30000;   // 30s
  const max = 300000;  // 5min

  const clamped = Math.min(max, Math.max(min, intervalMs));
  const ratio = (clamped - min) / (max - min);

  // 透明度：高频 1.0 → 低频 0.3
  return 1 - ratio * 0.7;
}

/**
 * 格式化间隔显示文本
 */
function formatInterval(intervalMs: number): string {
  const seconds = intervalMs / 1000;
  if (seconds < 60) {
    return `${seconds}s`;
  }
  const minutes = seconds / 60;
  if (Number.isInteger(minutes)) {
    return `${minutes}m`;
  }
  return `${minutes.toFixed(1)}m`;
}

/**
 * 频率图标 - 时钟+循环箭头样式，无动画
 * 使用 currentColor + opacity 实现颜色继承和透明度控制
 */
function FrequencyIcon({ opacity }: { opacity: number }) {
  return (
    <svg
      className="w-4 h-4 text-accent"
      viewBox="0 0 24 24"
      aria-hidden="true"
      focusable="false"
      style={{ opacity }}
    >
      {/* 时钟表盘 */}
      <circle
        cx="12"
        cy="12"
        r="9"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
      />
      {/* 时针 */}
      <line
        x1="12"
        y1="12"
        x2="12"
        y2="7"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
      />
      {/* 分针 */}
      <line
        x1="12"
        y1="12"
        x2="16"
        y2="12"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
      />
      {/* 循环箭头（右上角） */}
      <path
        d="M19,5 L19,9 L15,9"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

/**
 * 监测频率指示器
 * 显示心跳图标，透明度根据监测频率动态变化
 * - 高频（30s）：主题强调色，完全不透明
 * - 低频（5min）：主题强调色，70% 透明度
 * 无动画效果，通过 tooltip 显示具体间隔
 */
export function FrequencyIndicator({ intervalMs, className = '', tooltipPlacement = 'top' }: FrequencyIndicatorProps) {
  const { t } = useTranslation();
  const triggerRef = useRef<HTMLSpanElement>(null);
  const { isOpen, position, handleMouseEnter, handleMouseLeave } = useBadgeTooltip(
    triggerRef,
    tooltipPlacement
  );

  const opacity = useMemo(() => getFrequencyOpacity(intervalMs), [intervalMs]);
  const intervalText = useMemo(() => formatInterval(intervalMs), [intervalMs]);

  const tooltip = t('badges.frequency.tooltip', { interval: intervalText });

  return (
    <>
      <span
        ref={triggerRef}
        onMouseEnter={handleMouseEnter}
        onMouseLeave={handleMouseLeave}
        className={`inline-flex items-center cursor-default select-none ${className}`}
        role="img"
        aria-label={tooltip}
      >
        <FrequencyIcon opacity={opacity} />
      </span>

      <BadgeTooltip isOpen={isOpen} position={position}>
        {tooltip}
      </BadgeTooltip>
    </>
  );
}
