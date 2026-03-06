import { describe, it, expect, vi, beforeEach } from 'vitest';
import { aggregateHeatmap, getAggregationFactor, resetMediaQueryCache } from './heatmapAggregator';
import type { ProcessedMonitorData } from '../types';

type HistoryPoint = ProcessedMonitorData['history'][number];

// Mock mediaQuery to avoid window dependency in node
vi.mock('./mediaQuery', () => ({
  BREAKPOINTS: { tablet: '(max-width: 960px)', mobile: '(max-width: 768px)' },
  addMediaQueryListener: vi.fn(() => vi.fn()),
}));

beforeEach(() => {
  resetMediaQueryCache();
});

function makePoint(index: number, timestampNum: number, overrides: Partial<HistoryPoint> = {}): HistoryPoint {
  return {
    index,
    status: 'AVAILABLE' as HistoryPoint['status'],
    timestamp: String(timestampNum),
    timestampNum,
    latency: 100,
    availability: 99.5,
    statusCounts: {
      available: 1,
      degraded: 0,
      unavailable: 0,
      missing: 0,
      slow_latency: 0,
      rate_limit: 0,
      server_error: 0,
      client_error: 0,
      auth_error: 0,
      invalid_request: 0,
      network_error: 0,
      content_mismatch: 0,
    },
    ...overrides,
  };
}

describe('aggregateHeatmap', () => {
  it('returns original points for 90m window (span <= 5400s)', () => {
    const now = Math.floor(Date.now() / 1000);
    const points = [
      makePoint(0, now - 3600),
      makePoint(1, now - 1800),
      makePoint(2, now),
    ];
    // 90m window: span = 3600s <= 5400s → no aggregation
    const result = aggregateHeatmap(points);
    expect(result).toBe(points); // same reference
  });

  it('returns original points when desktop (no matchMedia match)', () => {
    // In node env, getIsTablet() returns false (no window), so no aggregation
    const now = Math.floor(Date.now() / 1000);
    const points: HistoryPoint[] = [];
    for (let i = 0; i < 100; i++) {
      points.push(makePoint(i, now - (86400 - i * 864))); // spans ~24h
    }
    const result = aggregateHeatmap(points);
    expect(result).toBe(points); // no aggregation on desktop
  });

  it('returns original points when fewer than maxBlocks', () => {
    const now = Math.floor(Date.now() / 1000);
    const points = [
      makePoint(0, now - 86400),
      makePoint(1, now),
    ];
    const result = aggregateHeatmap(points, 50);
    expect(result).toBe(points);
  });

  it('handles empty array', () => {
    const result = aggregateHeatmap([]);
    expect(result).toEqual([]);
  });

  it('handles single point', () => {
    const point = makePoint(0, 1000);
    const result = aggregateHeatmap([point]);
    expect(result).toEqual([point]);
  });
});

describe('getAggregationFactor', () => {
  it('returns 1 on desktop (no matchMedia)', () => {
    // In node environment, getIsTablet() returns false
    expect(getAggregationFactor('24h')).toBe(1);
    expect(getAggregationFactor('7d')).toBe(1);
    expect(getAggregationFactor('30d')).toBe(1);
    expect(getAggregationFactor('90m')).toBe(1);
  });
});
