import { describe, it, expect, beforeEach } from 'vitest';
import {
  availabilityToColor,
  availabilityToStyle,
  latencyToColor,
  greenLatencyToColor,
  degradedLatencyToColor,
  heatmapBlockToStyle,
  darkenColor,
  lerpColorRgb,
  sponsorLevelToBorderClass,
  sponsorLevelToCardBorderColor,
  sponsorLevelToPinnedBgClass,
  clearColorCache,
} from './color';
import type { StatusCounts } from '../types';

// In 'node' environment, CSS variables are not available, so the functions
// fall back to DEFAULT_COLORS. Tests verify gradient logic with those defaults.

beforeEach(() => {
  clearColorCache();
});

describe('availabilityToColor', () => {
  it('returns gray for negative availability (no data)', () => {
    const color = availabilityToColor(-1);
    expect(color).toMatch(/^rgb\(/);
    // Gray fallback: rgb(148, 163, 184)
    expect(color).toBe('rgb(148, 163, 184)');
  });

  it('returns red at 0% availability', () => {
    const color = availabilityToColor(0);
    // At 0%, lerp(red, yellow, 0) = red
    expect(color).toBe('rgb(239, 68, 68)');
  });

  it('returns yellow at 60% availability', () => {
    const color = availabilityToColor(60);
    // At 60%, lerp(red, yellow, 1) = yellow
    expect(color).toBe('rgb(234, 179, 8)');
  });

  it('returns green at 100% availability', () => {
    const color = availabilityToColor(100);
    // At 100%, lerp(yellow, green, 1) = green
    expect(color).toBe('rgb(34, 197, 94)');
  });

  it('returns interpolated color at 30% availability', () => {
    const color = availabilityToColor(30);
    // lerp(red, yellow, 0.5) — midpoint between red and yellow
    expect(color).toMatch(/^rgb\(\d+, \d+, \d+\)$/);
    // Should not be pure red or pure yellow
    expect(color).not.toBe('rgb(239, 68, 68)');
    expect(color).not.toBe('rgb(234, 179, 8)');
  });

  it('returns interpolated color at 80% availability', () => {
    const color = availabilityToColor(80);
    // lerp(yellow, green, 0.5) — midpoint between yellow and green
    expect(color).toMatch(/^rgb\(\d+, \d+, \d+\)$/);
    expect(color).not.toBe('rgb(234, 179, 8)');
    expect(color).not.toBe('rgb(34, 197, 94)');
  });
});

describe('availabilityToStyle', () => {
  it('returns object with backgroundColor', () => {
    const style = availabilityToStyle(100);
    expect(style).toHaveProperty('backgroundColor');
    expect(style.backgroundColor).toMatch(/^rgb\(/);
  });
});

describe('latencyToColor', () => {
  it('returns gray for zero latency', () => {
    const color = latencyToColor(0, 1000);
    expect(color).toBe('rgb(148, 163, 184)');
  });

  it('returns gray for zero threshold', () => {
    const color = latencyToColor(100, 0);
    expect(color).toBe('rgb(148, 163, 184)');
  });

  it('returns green for latency < 30% of threshold', () => {
    // 200ms latency, 1000ms threshold → ratio 0.2 < 0.3 → pure green
    const color = latencyToColor(200, 1000);
    expect(color).toBe('rgb(34, 197, 94)');
  });

  it('returns green-to-yellow gradient for 30%-100% of threshold', () => {
    // 650ms latency, 1000ms threshold → ratio 0.65 → green-to-yellow lerp
    const color = latencyToColor(650, 1000);
    expect(color).toMatch(/^rgb\(\d+, \d+, \d+\)$/);
    expect(color).not.toBe('rgb(34, 197, 94)');   // not pure green
    expect(color).not.toBe('rgb(234, 179, 8)');    // not pure yellow
  });

  it('returns yellow at 100% of threshold', () => {
    // At ratio 1.0, lerp(green, yellow, 1.0) = yellow (edge: just barely enters yellow-to-red)
    // Actually ratio=1.0 falls into the 100%-200% bracket: lerp(yellow, red, 0)
    const color = latencyToColor(1000, 1000);
    expect(color).toBe('rgb(234, 179, 8)');
  });

  it('returns red at >= 200% of threshold', () => {
    const color = latencyToColor(2000, 1000);
    expect(color).toBe('rgb(239, 68, 68)');
  });

  it('returns red for very high latency', () => {
    const color = latencyToColor(10000, 1000);
    expect(color).toBe('rgb(239, 68, 68)');
  });
});

describe('sponsorLevelToBorderClass', () => {
  it('returns empty string for undefined', () => {
    expect(sponsorLevelToBorderClass()).toBe('');
  });

  it('returns empty string for empty string', () => {
    expect(sponsorLevelToBorderClass('')).toBe('');
  });

  it('returns beacon class', () => {
    expect(sponsorLevelToBorderClass('beacon')).toBe('border-l-2 border-sponsor-beacon');
  });

  it('returns backbone class', () => {
    expect(sponsorLevelToBorderClass('backbone')).toBe('border-l-2 border-sponsor-backbone');
  });

  it('returns core class', () => {
    expect(sponsorLevelToBorderClass('core')).toBe('border-l-2 border-sponsor-core');
  });

  it('returns public class', () => {
    expect(sponsorLevelToBorderClass('public')).toBe('border-l-2 border-sponsor-public');
  });

  it('returns empty for unknown level', () => {
    expect(sponsorLevelToBorderClass('unknown')).toBe('');
  });
});

describe('sponsorLevelToCardBorderColor', () => {
  it('returns undefined for no level', () => {
    expect(sponsorLevelToCardBorderColor()).toBeUndefined();
  });

  it('returns HSL for pulse', () => {
    expect(sponsorLevelToCardBorderColor('pulse')).toBe('hsl(32 94% 56% / 0.4)');
  });

  it('returns HSL for beacon', () => {
    expect(sponsorLevelToCardBorderColor('beacon')).toBe('hsl(152 76% 39% / 0.4)');
  });

  it('returns HSL for core', () => {
    expect(sponsorLevelToCardBorderColor('core')).toBe('hsl(43 96% 56% / 0.4)');
  });

  it('returns undefined for unknown level', () => {
    expect(sponsorLevelToCardBorderColor('unknown')).toBeUndefined();
  });
});

describe('sponsorLevelToPinnedBgClass', () => {
  it('returns empty for no level', () => {
    expect(sponsorLevelToPinnedBgClass()).toBe('');
  });

  it('returns beacon bg class', () => {
    expect(sponsorLevelToPinnedBgClass('beacon')).toBe('bg-sponsor-beacon');
  });

  it('returns backbone bg class', () => {
    expect(sponsorLevelToPinnedBgClass('backbone')).toBe('bg-sponsor-backbone');
  });

  it('returns core bg class', () => {
    expect(sponsorLevelToPinnedBgClass('core')).toBe('bg-sponsor-core');
  });

  it('returns empty for unknown level', () => {
    expect(sponsorLevelToPinnedBgClass('unknown')).toBe('');
  });
});

// ─── 90m 延迟渐变着色 ────────────────────────────────────────

// 默认颜色（node 环境无 CSS 变量时的 fallback）
const GREEN = { r: 34, g: 197, b: 94 };
const YELLOW = { r: 234, g: 179, b: 8 };
const RED = { r: 239, g: 68, b: 68 };

// 构造 HeatmapPoint 的辅助函数
const defaultCounts: StatusCounts = {
  available: 0, degraded: 0, unavailable: 0, missing: 0,
  slow_latency: 0, rate_limit: 0, server_error: 0, client_error: 0,
  auth_error: 0, invalid_request: 0, network_error: 0, response_timeout: 0,
  content_mismatch: 0,
};

function makePoint(overrides: {
  availability: number;
  latency?: number;
  slowLatencyMs?: number;
  status?: 'AVAILABLE' | 'DEGRADED' | 'UNAVAILABLE' | 'MISSING';
}) {
  return {
    index: 0,
    status: overrides.status ?? 'AVAILABLE' as const,
    timestamp: '12:00',
    timestampNum: 0,
    latency: overrides.latency ?? 0,
    availability: overrides.availability,
    statusCounts: { ...defaultCounts },
    slowLatencyMs: overrides.slowLatencyMs,
  };
}

describe('darkenColor', () => {
  it('darkens by 25%', () => {
    const result = darkenColor({ r: 100, g: 200, b: 80 }, 0.25);
    expect(result).toEqual({ r: 75, g: 150, b: 60 });
  });

  it('clamps amount to [0, 1]', () => {
    const full = darkenColor({ r: 100, g: 100, b: 100 }, 1);
    expect(full).toEqual({ r: 0, g: 0, b: 0 });
    const none = darkenColor({ r: 100, g: 100, b: 100 }, 0);
    expect(none).toEqual({ r: 100, g: 100, b: 100 });
  });
});

describe('lerpColorRgb', () => {
  it('returns c1 at t=0', () => {
    expect(lerpColorRgb({ r: 0, g: 0, b: 0 }, { r: 100, g: 100, b: 100 }, 0))
      .toEqual({ r: 0, g: 0, b: 0 });
  });

  it('returns c2 at t=1', () => {
    expect(lerpColorRgb({ r: 0, g: 0, b: 0 }, { r: 100, g: 100, b: 100 }, 1))
      .toEqual({ r: 100, g: 100, b: 100 });
  });

  it('clamps t > 1 to 1', () => {
    expect(lerpColorRgb({ r: 0, g: 0, b: 0 }, { r: 100, g: 100, b: 100 }, 2))
      .toEqual({ r: 100, g: 100, b: 100 });
  });
});

describe('greenLatencyToColor', () => {
  it('returns deep green for low latency (ratio < 0.2)', () => {
    // 100ms / 1000ms = 0.1 → deep green (green darkened 10%)
    const color = greenLatencyToColor(100, 1000);
    const deepGreen = darkenColor(GREEN, 0.10);
    expect(color).toBe(`rgb(${deepGreen.r}, ${deepGreen.g}, ${deepGreen.b})`);
  });

  it('returns interpolated color for medium latency (0.2 ≤ ratio < 1.0)', () => {
    const color = greenLatencyToColor(650, 1000);
    expect(color).toMatch(/^rgb\(\d+, \d+, \d+\)$/);
    // Should be between deep green and yellow-green
    const deepGreen = darkenColor(GREEN, 0.10);
    expect(color).not.toBe(`rgb(${deepGreen.r}, ${deepGreen.g}, ${deepGreen.b})`);
  });

  it('clamps at yellow-green for ratio ≥ 1.0', () => {
    const atOne = greenLatencyToColor(1000, 1000);
    const atTwo = greenLatencyToColor(2000, 1000);
    // Both should return the same clamped yellow-green
    expect(atOne).toBe(atTwo);
  });

  it('falls back to pure green for invalid inputs', () => {
    expect(greenLatencyToColor(0, 1000)).toBe(`rgb(${GREEN.r}, ${GREEN.g}, ${GREEN.b})`);
    expect(greenLatencyToColor(100, 0)).toBe(`rgb(${GREEN.r}, ${GREEN.g}, ${GREEN.b})`);
    expect(greenLatencyToColor(-1, 1000)).toBe(`rgb(${GREEN.r}, ${GREEN.g}, ${GREEN.b})`);
  });
});

describe('degradedLatencyToColor', () => {
  it('returns yellow-green for ratio ≤ 1.0', () => {
    const color = degradedLatencyToColor(1000, 1000);
    const yellowGreen = lerpColorRgb(YELLOW, GREEN, 0.2);
    expect(color).toBe(`rgb(${yellowGreen.r}, ${yellowGreen.g}, ${yellowGreen.b})`);
  });

  it('returns pure yellow at ratio = 2.0', () => {
    const color = degradedLatencyToColor(2000, 1000);
    expect(color).toBe(`rgb(${YELLOW.r}, ${YELLOW.g}, ${YELLOW.b})`);
  });

  it('clamps at orange-yellow for ratio ≥ 3.0', () => {
    const atThree = degradedLatencyToColor(3000, 1000);
    const atFive = degradedLatencyToColor(5000, 1000);
    expect(atThree).toBe(atFive);
    // Should be yellow mixed with red 30%
    const orangeYellow = lerpColorRgb(YELLOW, RED, 0.3);
    expect(atThree).toBe(`rgb(${orangeYellow.r}, ${orangeYellow.g}, ${orangeYellow.b})`);
  });

  it('interpolates between yellow-green and yellow for 1.0 < ratio < 2.0', () => {
    const color = degradedLatencyToColor(1500, 1000);
    expect(color).toMatch(/^rgb\(\d+, \d+, \d+\)$/);
    // Should differ from both endpoints
    const yellowGreen = lerpColorRgb(YELLOW, GREEN, 0.2);
    expect(color).not.toBe(`rgb(${yellowGreen.r}, ${yellowGreen.g}, ${yellowGreen.b})`);
    expect(color).not.toBe(`rgb(${YELLOW.r}, ${YELLOW.g}, ${YELLOW.b})`);
  });

  it('falls back to pure yellow for invalid inputs', () => {
    expect(degradedLatencyToColor(0, 1000)).toBe(`rgb(${YELLOW.r}, ${YELLOW.g}, ${YELLOW.b})`);
    expect(degradedLatencyToColor(100, 0)).toBe(`rgb(${YELLOW.r}, ${YELLOW.g}, ${YELLOW.b})`);
  });
});

describe('heatmapBlockToStyle', () => {
  it('uses availability coloring when useLatencyGradient is false', () => {
    const point = makePoint({ availability: 100, latency: 500, slowLatencyMs: 1000 });
    const style = heatmapBlockToStyle(point, false);
    // Should match availabilityToStyle(100)
    expect(style).toEqual(availabilityToStyle(100));
  });

  it('uses green latency gradient for green blocks (availability=100)', () => {
    const point = makePoint({ availability: 100, latency: 500, slowLatencyMs: 1000 });
    const style = heatmapBlockToStyle(point, true);
    expect(style.backgroundColor).toBe(greenLatencyToColor(500, 1000));
  });

  it('uses degraded latency gradient for yellow blocks (availability=70)', () => {
    const point = makePoint({ availability: 70, latency: 1200, slowLatencyMs: 1000, status: 'DEGRADED' });
    const style = heatmapBlockToStyle(point, true);
    expect(style.backgroundColor).toBe(degradedLatencyToColor(1200, 1000));
  });

  it('falls back to availability for red blocks even with gradient enabled', () => {
    const point = makePoint({ availability: 0, latency: 0, slowLatencyMs: 1000, status: 'UNAVAILABLE' });
    const style = heatmapBlockToStyle(point, true);
    expect(style).toEqual(availabilityToStyle(0));
  });

  it('falls back to availability for gray blocks (no data)', () => {
    const point = makePoint({ availability: -1, status: 'MISSING' });
    const style = heatmapBlockToStyle(point, true);
    expect(style).toEqual(availabilityToStyle(-1));
  });

  it('falls back to availability when slowLatencyMs is missing', () => {
    const point = makePoint({ availability: 100, latency: 500 });
    const style = heatmapBlockToStyle(point, true);
    expect(style).toEqual(availabilityToStyle(100));
  });

  it('falls back to availability when latency is zero', () => {
    const point = makePoint({ availability: 100, latency: 0, slowLatencyMs: 1000 });
    const style = heatmapBlockToStyle(point, true);
    expect(style).toEqual(availabilityToStyle(100));
  });
});
