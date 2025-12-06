/**
 * 根据可用率计算渐变颜色
 * - 60%以下 → 红色
 * - 60%-80% → 红到黄渐变
 * - 80%-100% → 黄到绿渐变
 * - -1（无数据）→ 灰色
 */

import type { CSSProperties } from 'react';

// 颜色常量
const RED = { r: 239, g: 68, b: 68 };     // #ef4444
const YELLOW = { r: 234, g: 179, b: 8 };   // #eab308
const GREEN = { r: 34, g: 197, b: 94 };    // #22c55e
const GRAY = { r: 148, g: 163, b: 184 };   // #94a3b8

interface RGB {
  r: number;
  g: number;
  b: number;
}

/**
 * 线性插值两个颜色
 */
function lerpColor(color1: RGB, color2: RGB, t: number): string {
  const r = Math.round(color1.r + (color2.r - color1.r) * t);
  const g = Math.round(color1.g + (color2.g - color1.g) * t);
  const b = Math.round(color1.b + (color2.b - color1.b) * t);
  return `rgb(${r}, ${g}, ${b})`;
}

/**
 * 根据可用率返回背景颜色（CSS color string）
 */
export function availabilityToColor(availability: number): string {
  // 无数据
  if (availability < 0) {
    return `rgb(${GRAY.r}, ${GRAY.g}, ${GRAY.b})`;
  }

  // 60%以下 → 红色
  if (availability < 60) {
    return `rgb(${RED.r}, ${RED.g}, ${RED.b})`;
  }

  // 60%-80% → 红到黄渐变
  if (availability < 80) {
    const t = (availability - 60) / 20;
    return lerpColor(RED, YELLOW, t);
  }

  // 80%-100% → 黄到绿渐变
  const t = (availability - 80) / 20;
  return lerpColor(YELLOW, GREEN, t);
}

/**
 * 根据可用率返回 Tailwind 兼容的 style 对象
 */
export function availabilityToStyle(availability: number): CSSProperties {
  return {
    backgroundColor: availabilityToColor(availability),
  };
}

/**
 * 根据延迟计算渐变颜色
 * - 延迟越低越好（与可用率相反）
 * - 基于 slow_latency 阈值进行相对渐变
 *
 * 渐变逻辑：
 * - latency <= 0 → 灰色（无数据）
 * - latency < 30% 阈值 → 绿色（优秀）
 * - 30%-100% 阈值 → 绿到黄渐变（良好）
 * - 100%-200% 阈值 → 黄到红渐变（较慢）
 * - >= 200% 阈值 → 红色（很慢）
 */
export function latencyToColor(latency: number, slowLatencyMs: number): string {
  // 无数据或配置无效
  if (latency <= 0 || slowLatencyMs <= 0) {
    return `rgb(${GRAY.r}, ${GRAY.g}, ${GRAY.b})`;
  }

  const ratio = latency / slowLatencyMs;

  // < 30% 阈值 → 绿色
  if (ratio < 0.3) {
    return `rgb(${GREEN.r}, ${GREEN.g}, ${GREEN.b})`;
  }

  // 30%-100% 阈值 → 绿到黄渐变
  if (ratio < 1) {
    const t = (ratio - 0.3) / 0.7;
    return lerpColor(GREEN, YELLOW, t);
  }

  // 100%-200% 阈值 → 黄到红渐变
  if (ratio < 2) {
    const t = (ratio - 1) / 1;
    return lerpColor(YELLOW, RED, t);
  }

  // >= 200% 阈值 → 红色
  return `rgb(${RED.r}, ${RED.g}, ${RED.b})`;
}

/**
 * 根据赞助商等级返回左边框 Tailwind 类名（用于表格行）
 * 颜色与 SponsorBadge 徽章一致
 */
export function sponsorLevelToBorderClass(level?: string): string {
  if (!level) return '';
  const BORDER_CLASSES: Record<string, string> = {
    basic: 'border-l-2 border-l-emerald-500/40',
    advanced: 'border-l-2 border-l-cyan-500/40',
    enterprise: 'border-l-2 border-l-amber-400/40',
  };
  return BORDER_CLASSES[level] || '';
}

// 卡片视图左边框颜色映射（返回内联样式对象）
// 与表格视图保持一致：40% 透明度
const CARD_BORDER_COLORS: Record<string, string> = {
  basic: 'rgba(16, 185, 129, 0.4)',    // emerald-500/40
  advanced: 'rgba(6, 182, 212, 0.4)',   // cyan-500/40
  enterprise: 'rgba(251, 191, 36, 0.4)', // amber-400/40
};

/**
 * 根据赞助商等级返回卡片左边框颜色（CSS color string）
 * 颜色与 SponsorBadge 徽章一致，用于卡片视图内联样式
 */
export function sponsorLevelToCardBorderColor(level?: string): string | undefined {
  if (!level) return undefined;
  return CARD_BORDER_COLORS[level];
}

/**
 * 根据赞助商等级返回置顶背景色（Tailwind class）
 * 使用 5% 透明度，颜色与徽章一致
 */
export function sponsorLevelToPinnedBgClass(level?: string): string {
  if (!level) return '';
  const BG_CLASSES: Record<string, string> = {
    basic: 'bg-emerald-500/[0.05]',
    advanced: 'bg-cyan-500/[0.05]',
    enterprise: 'bg-amber-400/[0.05]',
  };
  return BG_CLASSES[level] || '';
}
