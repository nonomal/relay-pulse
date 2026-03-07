import { describe, it, expect } from 'vitest';
import type { ProcessedMonitorData, SponsorLevel } from '../types';
import {
  calculateBadgeScore,
  hasAnyBadge,
  hasAnyBadgeInList,
  CATEGORY_WEIGHTS,
  SPONSOR_WEIGHTS,
  RISK_WEIGHT,
} from './badgeUtils';

// Minimal mock factory
function mockItem(overrides: Partial<ProcessedMonitorData> = {}): ProcessedMonitorData {
  return {
    id: 'test-id',
    providerId: 'provider',
    providerSlug: 'provider',
    providerName: 'Provider',
    serviceType: 'cc',
    serviceName: 'CC',
    category: 'commercial',
    sponsor: '',
    board: 'hot',
    isMultiModel: false,
    history: [],
    currentStatus: { status: 'AVAILABLE', latency: 100, timestamp: Date.now() / 1000 },
    uptime: 99,
    ...overrides,
  } as ProcessedMonitorData;
}

describe('calculateBadgeScore', () => {
  it('returns 0 for commercial with no badges', () => {
    const item = mockItem({ category: 'commercial' });
    expect(calculateBadgeScore(item)).toBe(0);
  });

  it('adds category weight for public', () => {
    const item = mockItem({ category: 'public' });
    expect(calculateBadgeScore(item)).toBe(CATEGORY_WEIGHTS.public);
  });

  it('adds sponsor weight', () => {
    const item = mockItem({ sponsorLevel: 'core' as SponsorLevel });
    expect(calculateBadgeScore(item)).toBe(SPONSOR_WEIGHTS.core);
  });

  it('combines category and sponsor', () => {
    const item = mockItem({
      category: 'public',
      sponsorLevel: 'pulse' as SponsorLevel,
    });
    expect(calculateBadgeScore(item)).toBe(CATEGORY_WEIGHTS.public + SPONSOR_WEIGHTS.pulse);
  });

  it('subtracts risk weight', () => {
    const item = mockItem({
      risks: [{ label: 'risk1' }, { label: 'risk2' }] as ProcessedMonitorData['risks'],
    });
    expect(calculateBadgeScore(item)).toBe(2 * RISK_WEIGHT);
  });

  it('combines all badge types', () => {
    const item = mockItem({
      category: 'public',
      sponsorLevel: 'backbone' as SponsorLevel,
      risks: [{ label: 'r1' }] as ProcessedMonitorData['risks'],
    });
    const expected = CATEGORY_WEIGHTS.public + SPONSOR_WEIGHTS.backbone + RISK_WEIGHT;
    expect(calculateBadgeScore(item)).toBe(expected);
  });

  it('handles undefined risks', () => {
    const item = mockItem({ risks: undefined });
    expect(calculateBadgeScore(item)).toBe(0);
  });

  it('handles empty risks', () => {
    const item = mockItem({ risks: [] });
    expect(calculateBadgeScore(item)).toBe(0);
  });
});

describe('hasAnyBadge', () => {
  it('returns false when badges disabled', () => {
    const item = mockItem({ category: 'public' });
    expect(hasAnyBadge(item, { enableBadges: false })).toBe(false);
  });

  it('returns true for public category', () => {
    const item = mockItem({ category: 'public' });
    expect(hasAnyBadge(item)).toBe(true);
  });

  it('returns false for commercial with no other badges', () => {
    const item = mockItem({ category: 'commercial' });
    expect(hasAnyBadge(item)).toBe(false);
  });

  it('returns true when sponsor level is set', () => {
    const item = mockItem({ sponsorLevel: 'beacon' as SponsorLevel });
    expect(hasAnyBadge(item)).toBe(true);
  });

  it('returns false for sponsor when showSponsor is false', () => {
    const item = mockItem({ sponsorLevel: 'beacon' as SponsorLevel });
    expect(hasAnyBadge(item, { showSponsor: false })).toBe(false);
  });

  it('returns true when risks exist', () => {
    const item = mockItem({
      risks: [{ label: 'risk' }] as ProcessedMonitorData['risks'],
    });
    expect(hasAnyBadge(item)).toBe(true);
  });

  it('returns false for risks when showRisk is false', () => {
    const item = mockItem({
      risks: [{ label: 'risk' }] as ProcessedMonitorData['risks'],
    });
    expect(hasAnyBadge(item, { showRisk: false })).toBe(false);
  });

  it('returns true for frequency badge', () => {
    const item = mockItem({ intervalMs: 60000 });
    expect(hasAnyBadge(item)).toBe(true);
  });

  it('returns false for frequency when showFrequency is false', () => {
    const item = mockItem({ intervalMs: 60000 });
    expect(hasAnyBadge(item, { showFrequency: false })).toBe(false);
  });

  it('returns true for generic badges', () => {
    const item = mockItem({
      badges: [{ id: 'b1', kind: 'info', variant: 'default', weight: 0 }] as ProcessedMonitorData['badges'],
    });
    expect(hasAnyBadge(item)).toBe(true);
  });
});

describe('hasAnyBadgeInList', () => {
  it('returns false for empty list', () => {
    expect(hasAnyBadgeInList([])).toBe(false);
  });

  it('returns true if any item has badge', () => {
    const items = [
      mockItem({ category: 'commercial' }),
      mockItem({ category: 'public' }),
    ];
    expect(hasAnyBadgeInList(items)).toBe(true);
  });

  it('returns false if no items have badges', () => {
    const items = [
      mockItem({ category: 'commercial' }),
      mockItem({ category: 'commercial' }),
    ];
    expect(hasAnyBadgeInList(items)).toBe(false);
  });

  it('respects options', () => {
    const items = [
      mockItem({ category: 'public' }),
    ];
    expect(hasAnyBadgeInList(items, { showCategoryTag: false })).toBe(false);
  });
});
