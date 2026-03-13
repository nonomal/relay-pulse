import { describe, it, expect, beforeEach } from 'vitest';
import {
  availabilityToColor,
  availabilityToStyle,
  latencyToColor,
  sponsorLevelToBorderClass,
  sponsorLevelToCardBorderColor,
  sponsorLevelToPinnedBgClass,
  clearColorCache,
} from './color';

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
    expect(sponsorLevelToCardBorderColor('pulse')).toBe('hsl(152 76% 39% / 0.4)');
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
