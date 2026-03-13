import { describe, it, expect } from 'vitest';
import type { ProcessedMonitorData, Annotation } from '../types';
import {
  hasAnyAnnotation,
  hasAnyAnnotationInList,
  SPONSOR_WEIGHTS,
} from './annotationUtils';

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
    currentStatus: 'AVAILABLE',
    uptime: 99,
    ...overrides,
  } as ProcessedMonitorData;
}

function makeAnnotation(overrides: Partial<Annotation> = {}): Annotation {
  return {
    id: 'test',
    family: 'neutral',
    label: 'Test',
    priority: 0,
    origin: 'system',
    ...overrides,
  };
}

describe('hasAnyAnnotation', () => {
  it('returns false when annotations disabled', () => {
    const item = mockItem({
      annotations: [makeAnnotation({ id: 'public_service' })],
    });
    expect(hasAnyAnnotation(item, { enableAnnotations: false })).toBe(false);
  });

  it('returns true when item has annotations', () => {
    const item = mockItem({
      annotations: [makeAnnotation({ id: 'public_service', family: 'neutral' })],
    });
    expect(hasAnyAnnotation(item)).toBe(true);
  });

  it('returns false when item has no annotations', () => {
    const item = mockItem({ annotations: [] });
    expect(hasAnyAnnotation(item)).toBe(false);
  });

  it('returns false when annotations is undefined', () => {
    const item = mockItem({ annotations: undefined });
    expect(hasAnyAnnotation(item)).toBe(false);
  });

  it('defaults enableAnnotations to true', () => {
    const item = mockItem({
      annotations: [makeAnnotation({ id: 'sponsor_beacon', family: 'positive' })],
    });
    expect(hasAnyAnnotation(item)).toBe(true);
  });
});

describe('hasAnyAnnotationInList', () => {
  it('returns false for empty list', () => {
    expect(hasAnyAnnotationInList([])).toBe(false);
  });

  it('returns true if any item has annotations', () => {
    const items = [
      mockItem({ annotations: [] }),
      mockItem({ annotations: [makeAnnotation()] }),
    ];
    expect(hasAnyAnnotationInList(items)).toBe(true);
  });

  it('returns false if no items have annotations', () => {
    const items = [
      mockItem({ annotations: [] }),
      mockItem({ annotations: undefined }),
    ];
    expect(hasAnyAnnotationInList(items)).toBe(false);
  });

  it('respects enableAnnotations option', () => {
    const items = [
      mockItem({ annotations: [makeAnnotation()] }),
    ];
    expect(hasAnyAnnotationInList(items, { enableAnnotations: false })).toBe(false);
  });
});

describe('SPONSOR_WEIGHTS', () => {
  it('has correct hierarchy', () => {
    expect(SPONSOR_WEIGHTS.core).toBeGreaterThan(SPONSOR_WEIGHTS.backbone);
    expect(SPONSOR_WEIGHTS.backbone).toBeGreaterThan(SPONSOR_WEIGHTS.beacon);
    expect(SPONSOR_WEIGHTS.beacon).toBeGreaterThan(SPONSOR_WEIGHTS.pulse);
    expect(SPONSOR_WEIGHTS.pulse).toBeGreaterThan(SPONSOR_WEIGHTS.signal);
    expect(SPONSOR_WEIGHTS.signal).toBeGreaterThan(SPONSOR_WEIGHTS.public);
  });
});
