/**
 * 主题感知的颜色工具函数
 *
 * - 根据可用率计算渐变颜色
 * - 根据延迟计算渐变颜色
 * - 赞助商等级颜色映射
 *
 * 颜色值从 CSS 变量读取，支持主题切换
 */

import type { CSSProperties } from 'react';
import type { ProcessedMonitorData } from '../types';

interface RGB {
  r: number;
  g: number;
  b: number;
}

type HeatmapPoint = ProcessedMonitorData['history'][number];

// 默认颜色（用于 SSR 或变量未加载时的 fallback）
const DEFAULT_COLORS = {
  green: { r: 34, g: 197, b: 94 },   // #22c55e
  yellow: { r: 234, g: 179, b: 8 },  // #eab308
  red: { r: 239, g: 68, b: 68 },     // #ef4444
  gray: { r: 148, g: 163, b: 184 },  // #94a3b8
};

// 颜色缓存
let colorCache: Record<string, RGB> | null = null;

/**
 * HSL 字符串转 RGB 对象
 * 输入格式: "142 71% 45%" 或 "142 71 45"
 */
function hslToRgb(hslStr: string): RGB {
  const parts = hslStr.trim().split(/\s+/);
  if (parts.length < 3) {
    return DEFAULT_COLORS.gray;
  }

  const h = parseFloat(parts[0]) / 360;
  const s = parseFloat(parts[1].replace('%', '')) / 100;
  const l = parseFloat(parts[2].replace('%', '')) / 100;

  let r: number, g: number, b: number;

  if (s === 0) {
    r = g = b = l;
  } else {
    const hue2rgb = (p: number, q: number, t: number): number => {
      if (t < 0) t += 1;
      if (t > 1) t -= 1;
      if (t < 1 / 6) return p + (q - p) * 6 * t;
      if (t < 1 / 2) return q;
      if (t < 2 / 3) return p + (q - p) * (2 / 3 - t) * 6;
      return p;
    };

    const q = l < 0.5 ? l * (1 + s) : l + s - l * s;
    const p = 2 * l - q;
    r = hue2rgb(p, q, h + 1 / 3);
    g = hue2rgb(p, q, h);
    b = hue2rgb(p, q, h - 1 / 3);
  }

  return {
    r: Math.round(r * 255),
    g: Math.round(g * 255),
    b: Math.round(b * 255),
  };
}

/**
 * 从 CSS 变量读取颜色值并转换为 RGB
 */
function getCssVarAsRgb(varName: string, fallback: RGB): RGB {
  if (typeof window === 'undefined') {
    return fallback;
  }

  try {
    const value = getComputedStyle(document.documentElement)
      .getPropertyValue(varName)
      .trim();

    if (!value) {
      return fallback;
    }

    return hslToRgb(value);
  } catch {
    return fallback;
  }
}

/**
 * 获取主题颜色（带缓存）
 */
function getThemeColors(): Record<string, RGB> {
  if (colorCache) {
    return colorCache;
  }

  colorCache = {
    green: getCssVarAsRgb('--chart-green', DEFAULT_COLORS.green),
    yellow: getCssVarAsRgb('--chart-yellow', DEFAULT_COLORS.yellow),
    red: getCssVarAsRgb('--chart-red', DEFAULT_COLORS.red),
    gray: getCssVarAsRgb('--chart-gray', DEFAULT_COLORS.gray),
  };

  return colorCache;
}

/**
 * 清除颜色缓存（主题切换时调用）
 */
export function clearColorCache(): void {
  colorCache = null;
}

// 监听主题变化，自动清除缓存
if (typeof window !== 'undefined') {
  // 监听自定义主题变化事件
  window.addEventListener('theme-change', () => {
    clearColorCache();
  });

  // 监听 DOM 属性变化
  const observer = new MutationObserver((mutations) => {
    for (const mutation of mutations) {
      if (mutation.attributeName === 'data-theme') {
        clearColorCache();
        break;
      }
    }
  });

  observer.observe(document.documentElement, { attributes: true });
}

function clamp01(v: number): number {
  return Math.max(0, Math.min(1, v));
}

function rgbToCss(c: RGB): string {
  return `rgb(${c.r}, ${c.g}, ${c.b})`;
}

/**
 * 降低颜色亮度
 * @param amount 0~1，值越大越暗
 */
export function darkenColor(color: RGB, amount: number): RGB {
  const factor = 1 - clamp01(amount);
  return {
    r: Math.round(color.r * factor),
    g: Math.round(color.g * factor),
    b: Math.round(color.b * factor),
  };
}

/**
 * 线性插值两个 RGB 颜色，返回 RGB 对象
 */
export function lerpColorRgb(c1: RGB, c2: RGB, t: number): RGB {
  const ct = clamp01(t);
  return {
    r: Math.round(c1.r + (c2.r - c1.r) * ct),
    g: Math.round(c1.g + (c2.g - c1.g) * ct),
    b: Math.round(c1.b + (c2.b - c1.b) * ct),
  };
}

/**
 * 线性插值两个颜色（返回 CSS 字符串）
 */
function lerpColor(color1: RGB, color2: RGB, t: number): string {
  return rgbToCss(lerpColorRgb(color1, color2, t));
}

/**
 * 根据可用率返回背景颜色（CSS color string）
 *
 * 渐变逻辑：
 * - availability < 0 → 灰色（无数据）
 * - 0%-60% → 红到黄渐变
 * - 60%-100% → 黄到绿渐变
 */
export function availabilityToColor(availability: number): string {
  const colors = getThemeColors();

  // 无数据
  if (availability < 0) {
    return `rgb(${colors.gray.r}, ${colors.gray.g}, ${colors.gray.b})`;
  }

  // 0%-60% → 红到黄渐变
  if (availability <= 60) {
    const t = availability / 60;
    return lerpColor(colors.red, colors.yellow, t);
  }

  // 60%-100% → 黄到绿渐变
  const t = (availability - 60) / 40;
  return lerpColor(colors.yellow, colors.green, t);
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
 *
 * 渐变逻辑：
 * - latency <= 0 → 灰色（无数据）
 * - latency < 30% 阈值 → 绿色（优秀）
 * - 30%-100% 阈值 → 绿到黄渐变（良好）
 * - 100%-200% 阈值 → 黄到红渐变（较慢）
 * - >= 200% 阈值 → 红色（很慢）
 */
export function latencyToColor(latency: number, slowLatencyMs: number): string {
  const colors = getThemeColors();

  // 无数据或配置无效
  if (latency <= 0 || slowLatencyMs <= 0) {
    return `rgb(${colors.gray.r}, ${colors.gray.g}, ${colors.gray.b})`;
  }

  const ratio = latency / slowLatencyMs;

  // < 30% 阈值 → 绿色
  if (ratio < 0.3) {
    return `rgb(${colors.green.r}, ${colors.green.g}, ${colors.green.b})`;
  }

  // 30%-100% 阈值 → 绿到黄渐变
  if (ratio < 1) {
    const t = (ratio - 0.3) / 0.7;
    return lerpColor(colors.green, colors.yellow, t);
  }

  // 100%-200% 阈值 → 黄到红渐变
  if (ratio < 2) {
    const t = (ratio - 1) / 1;
    return lerpColor(colors.yellow, colors.red, t);
  }

  // >= 200% 阈值 → 红色
  return `rgb(${colors.red.r}, ${colors.red.g}, ${colors.red.b})`;
}

/**
 * 90m 绿色块延迟渐变：深绿 → 黄绿
 *
 * - ratio < 0.3       → 深绿（亮度降 25%）
 * - 0.3 ≤ ratio < 1.0 → 深绿 → 黄绿渐变
 * - ratio ≥ 1.0       → 钳位到黄绿
 */
export function greenLatencyToColor(latency: number, slowLatencyMs: number): string {
  const colors = getThemeColors();

  if (latency <= 0 || slowLatencyMs <= 0) {
    return rgbToCss(colors.green);
  }

  const ratio = latency / slowLatencyMs;
  const deepGreen = darkenColor(colors.green, 0.25);
  const yellowGreen = lerpColorRgb(colors.green, colors.yellow, 0.35);

  if (ratio < 0.3) {
    return rgbToCss(deepGreen);
  }

  if (ratio < 1) {
    return rgbToCss(lerpColorRgb(deepGreen, yellowGreen, (ratio - 0.3) / 0.7));
  }

  return rgbToCss(yellowGreen);
}

/**
 * 90m 黄色块延迟渐变：黄绿偏黄 → 纯黄 → 橙黄
 *
 * - ratio ≤ 1.0       → 黄绿偏黄（yellow mix green 20%）
 * - 1.0 < ratio < 2.0 → 黄绿偏黄 → 纯黄
 * - 2.0 ≤ ratio < 3.0 → 纯黄 → 橙黄
 * - ratio ≥ 3.0       → 钳位到橙黄（yellow mix red 30%）
 */
export function degradedLatencyToColor(latency: number, slowLatencyMs: number): string {
  const colors = getThemeColors();

  if (latency <= 0 || slowLatencyMs <= 0) {
    return rgbToCss(colors.yellow);
  }

  const ratio = latency / slowLatencyMs;
  const yellowGreen = lerpColorRgb(colors.yellow, colors.green, 0.2);
  const orangeYellow = lerpColorRgb(colors.yellow, colors.red, 0.3);

  if (ratio <= 1) {
    return rgbToCss(yellowGreen);
  }

  if (ratio < 2) {
    return rgbToCss(lerpColorRgb(yellowGreen, colors.yellow, ratio - 1));
  }

  if (ratio < 3) {
    return rgbToCss(lerpColorRgb(colors.yellow, orangeYellow, ratio - 2));
  }

  return rgbToCss(orangeYellow);
}

/**
 * 热力图块统一着色入口
 *
 * - useLatencyGradient=false → 直接按可用率着色
 * - useLatencyGradient=true  → 绿色/黄色块按延迟渐变，其余按可用率
 */
export function heatmapBlockToStyle(point: HeatmapPoint, useLatencyGradient = false): CSSProperties {
  if (!useLatencyGradient) {
    return availabilityToStyle(point.availability);
  }

  const slowMs = point.slowLatencyMs ?? 0;
  const hasValidLatency = point.latency > 0 && slowMs > 0;

  if (point.availability === 100 && hasValidLatency) {
    return { backgroundColor: greenLatencyToColor(point.latency, slowMs) };
  }

  if (point.availability === 70 && hasValidLatency) {
    return { backgroundColor: degradedLatencyToColor(point.latency, slowMs) };
  }

  return availabilityToStyle(point.availability);
}

/**
 * 根据赞助等级返回左边框 Tailwind 类名（用于表格行）
 * 使用固定颜色，不随主题变化（品牌一致性）
 */
export function sponsorLevelToBorderClass(level?: string): string {
  if (!level) return '';
  const BORDER_CLASSES: Record<string, string> = {
    public: 'border-l-2 border-sponsor-public',
    signal: 'border-l-2 border-sponsor-signal',
    pulse: 'border-l-2 border-sponsor-pulse',
    beacon: 'border-l-2 border-sponsor-beacon',
    backbone: 'border-l-2 border-sponsor-backbone',
    core: 'border-l-2 border-sponsor-core',
  };
  return BORDER_CLASSES[level] || '';
}

/**
 * 根据赞助等级返回卡片左边框颜色
 * 使用固定颜色值，不随主题变化（品牌一致性）
 */
export function sponsorLevelToCardBorderColor(level?: string): string | undefined {
  if (!level) return undefined;
  // 固定颜色：public=slate, signal=sky, pulse=emerald, beacon=emerald, backbone=amber, core=gold
  const BORDER_COLORS: Record<string, string> = {
    public: 'hsl(215 16% 55% / 0.35)',
    signal: 'hsl(199 89% 48% / 0.4)',
    pulse: 'hsl(152 76% 39% / 0.4)',
    beacon: 'hsl(152 76% 39% / 0.4)',
    backbone: 'hsl(38 92% 50% / 0.4)',
    core: 'hsl(43 96% 56% / 0.4)',
  };
  return BORDER_COLORS[level];
}

/**
 * 根据赞助等级返回置顶背景色（语义化 CSS 类名）
 * 使用固定颜色（5% 透明度），不随主题变化（品牌一致性）
 */
export function sponsorLevelToPinnedBgClass(level?: string): string {
  if (!level) return '';
  const BG_CLASSES: Record<string, string> = {
    public: 'bg-sponsor-public',
    signal: 'bg-sponsor-signal',
    pulse: 'bg-sponsor-pulse',
    beacon: 'bg-sponsor-beacon',
    backbone: 'bg-sponsor-backbone',
    core: 'bg-sponsor-core',
  };
  return BG_CLASSES[level] || '';
}
