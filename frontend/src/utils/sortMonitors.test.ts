import { describe, expect, it } from 'vitest';
import { sortMonitors } from './sortMonitors';
import type { ProcessedMonitorData, SortConfig } from '../types';

// 创建测试用的 mock 数据
function createMockData(overrides: Partial<ProcessedMonitorData>): ProcessedMonitorData {
  return {
    id: 'test-id',
    providerId: 'test',
    providerSlug: 'test',
    providerName: 'Test',
    serviceType: 'cc',
    category: 'commercial',
    sponsor: 'Test Sponsor',
    history: [],
    currentStatus: 'AVAILABLE',
    uptime: 99.5,
    lastCheckLatency: 100,
    ...overrides,
  };
}

describe('sortMonitors', () => {
  describe('主排序', () => {
    it('按服务商名称升序排序', () => {
      const data = [
        createMockData({ id: '1', providerName: 'Charlie', lastCheckLatency: 100 }),
        createMockData({ id: '2', providerName: 'Alpha', lastCheckLatency: 200 }),
        createMockData({ id: '3', providerName: 'Bravo', lastCheckLatency: 150 }),
      ];
      const config: SortConfig = { key: 'providerName', direction: 'asc' };

      const result = sortMonitors(data, config);

      expect(result.map((d) => d.providerName)).toEqual(['Alpha', 'Bravo', 'Charlie']);
    });

    it('按服务商名称降序排序', () => {
      const data = [
        createMockData({ id: '1', providerName: 'Alpha' }),
        createMockData({ id: '2', providerName: 'Charlie' }),
        createMockData({ id: '3', providerName: 'Bravo' }),
      ];
      const config: SortConfig = { key: 'providerName', direction: 'desc' };

      const result = sortMonitors(data, config);

      expect(result.map((d) => d.providerName)).toEqual(['Charlie', 'Bravo', 'Alpha']);
    });

    it('按可用率降序排序', () => {
      const data = [
        createMockData({ id: '1', uptime: 80, lastCheckLatency: 100 }),
        createMockData({ id: '2', uptime: 99.9, lastCheckLatency: 200 }),
        createMockData({ id: '3', uptime: 95, lastCheckLatency: 150 }),
      ];
      const config: SortConfig = { key: 'uptime', direction: 'desc' };

      const result = sortMonitors(data, config);

      expect(result.map((d) => d.uptime)).toEqual([99.9, 95, 80]);
    });

    it('按可用率升序排序', () => {
      const data = [
        createMockData({ id: '1', uptime: 99.9 }),
        createMockData({ id: '2', uptime: 80 }),
        createMockData({ id: '3', uptime: 95 }),
      ];
      const config: SortConfig = { key: 'uptime', direction: 'asc' };

      const result = sortMonitors(data, config);

      expect(result.map((d) => d.uptime)).toEqual([80, 95, 99.9]);
    });

    it('按状态权重排序（AVAILABLE > DEGRADED > UNAVAILABLE）', () => {
      const data = [
        createMockData({ id: '1', currentStatus: 'DEGRADED', lastCheckLatency: 100 }),
        createMockData({ id: '2', currentStatus: 'AVAILABLE', lastCheckLatency: 200 }),
        createMockData({ id: '3', currentStatus: 'UNAVAILABLE', lastCheckLatency: 150 }),
      ];
      const config: SortConfig = { key: 'currentStatus', direction: 'desc' };

      const result = sortMonitors(data, config);

      expect(result.map((d) => d.currentStatus)).toEqual([
        'AVAILABLE',
        'DEGRADED',
        'UNAVAILABLE',
      ]);
    });
  });

  describe('uptime 特殊处理', () => {
    it('无数据（uptime < 0）始终排最后（降序）', () => {
      const data = [
        createMockData({ id: '1', uptime: -1, lastCheckLatency: 50 }),
        createMockData({ id: '2', uptime: 99, lastCheckLatency: 100 }),
        createMockData({ id: '3', uptime: 80, lastCheckLatency: 150 }),
      ];
      const config: SortConfig = { key: 'uptime', direction: 'desc' };

      const result = sortMonitors(data, config);

      expect(result.map((d) => d.uptime)).toEqual([99, 80, -1]);
    });

    it('无数据（uptime < 0）始终排最后（升序）', () => {
      const data = [
        createMockData({ id: '1', uptime: -1, lastCheckLatency: 50 }),
        createMockData({ id: '2', uptime: 99, lastCheckLatency: 100 }),
        createMockData({ id: '3', uptime: 80, lastCheckLatency: 150 }),
      ];
      const config: SortConfig = { key: 'uptime', direction: 'asc' };

      const result = sortMonitors(data, config);

      expect(result.map((d) => d.uptime)).toEqual([80, 99, -1]);
    });

    it('多个无数据记录保持相对顺序', () => {
      const data = [
        createMockData({ id: '1', uptime: -1, lastCheckLatency: 200 }),
        createMockData({ id: '2', uptime: 95, lastCheckLatency: 100 }),
        createMockData({ id: '3', uptime: -1, lastCheckLatency: 100 }),
      ];
      const config: SortConfig = { key: 'uptime', direction: 'desc' };

      const result = sortMonitors(data, config);

      // 95 排第一，两个 -1 按延迟二级排序
      expect(result.map((d) => d.id)).toEqual(['2', '3', '1']);
    });
  });

  describe('二级排序（延迟）', () => {
    it('可用率相等时，按延迟升序排序', () => {
      const data = [
        createMockData({ id: '1', uptime: 99, lastCheckLatency: 300 }),
        createMockData({ id: '2', uptime: 99, lastCheckLatency: 100 }),
        createMockData({ id: '3', uptime: 99, lastCheckLatency: 200 }),
      ];
      const config: SortConfig = { key: 'uptime', direction: 'desc' };

      const result = sortMonitors(data, config);

      expect(result.map((d) => d.lastCheckLatency)).toEqual([100, 200, 300]);
    });

    it('状态相等时，按延迟升序排序', () => {
      const data = [
        createMockData({ id: '1', currentStatus: 'AVAILABLE', lastCheckLatency: 500 }),
        createMockData({ id: '2', currentStatus: 'AVAILABLE', lastCheckLatency: 100 }),
        createMockData({ id: '3', currentStatus: 'AVAILABLE', lastCheckLatency: 250 }),
      ];
      const config: SortConfig = { key: 'currentStatus', direction: 'desc' };

      const result = sortMonitors(data, config);

      expect(result.map((d) => d.lastCheckLatency)).toEqual([100, 250, 500]);
    });

    it('延迟为 undefined 时排最后', () => {
      const data = [
        createMockData({ id: '1', uptime: 99, lastCheckLatency: undefined }),
        createMockData({ id: '2', uptime: 99, lastCheckLatency: 100 }),
        createMockData({ id: '3', uptime: 99, lastCheckLatency: 200 }),
      ];
      const config: SortConfig = { key: 'uptime', direction: 'desc' };

      const result = sortMonitors(data, config);

      expect(result.map((d) => d.id)).toEqual(['2', '3', '1']);
    });

    it('多个延迟为 undefined 时保持原顺序', () => {
      const data = [
        createMockData({ id: '1', uptime: 99, lastCheckLatency: undefined }),
        createMockData({ id: '2', uptime: 99, lastCheckLatency: undefined }),
        createMockData({ id: '3', uptime: 99, lastCheckLatency: 100 }),
      ];
      const config: SortConfig = { key: 'uptime', direction: 'desc' };

      const result = sortMonitors(data, config);

      // id=3 有延迟排第一，id=1 和 id=2 都无延迟，保持原顺序
      expect(result.map((d) => d.id)).toEqual(['3', '1', '2']);
    });
  });

  describe('不可变性', () => {
    it('不修改原数组', () => {
      const data = [
        createMockData({ id: '1', providerName: 'Charlie' }),
        createMockData({ id: '2', providerName: 'Alpha' }),
      ];
      const originalOrder = data.map((d) => d.id);
      const config: SortConfig = { key: 'providerName', direction: 'asc' };

      sortMonitors(data, config);

      expect(data.map((d) => d.id)).toEqual(originalOrder);
    });

    it('返回新数组', () => {
      const data = [createMockData({ id: '1' })];
      const config: SortConfig = { key: 'providerName', direction: 'asc' };

      const result = sortMonitors(data, config);

      expect(result).not.toBe(data);
    });
  });

  describe('边界情况', () => {
    it('空数组返回空数组', () => {
      const config: SortConfig = { key: 'uptime', direction: 'desc' };

      const result = sortMonitors([], config);

      expect(result).toEqual([]);
    });

    it('单元素数组直接返回', () => {
      const data = [createMockData({ id: '1' })];
      const config: SortConfig = { key: 'uptime', direction: 'desc' };

      const result = sortMonitors(data, config);

      expect(result).toHaveLength(1);
      expect(result[0].id).toBe('1');
    });

    it('空 key 时返回原数组副本', () => {
      const data = [
        createMockData({ id: '1' }),
        createMockData({ id: '2' }),
      ];
      const config: SortConfig = { key: '', direction: 'desc' };

      const result = sortMonitors(data, config);

      expect(result.map((d) => d.id)).toEqual(['1', '2']);
      expect(result).not.toBe(data);
    });
  });

  describe('徽标分数排序', () => {
    it('公益站比同等条件的商业站优先', () => {
      const data = [
        createMockData({ id: '1', category: 'commercial', lastCheckLatency: 100 }),
        createMockData({ id: '2', category: 'public', lastCheckLatency: 100 }),
      ];
      const config: SortConfig = { key: 'badgeScore', direction: 'desc' };

      const result = sortMonitors(data, config);

      expect(result.map((d) => d.id)).toEqual(['2', '1']); // 公益站优先
    });

    it('basic 赞助商的商业站优先于无赞助的公益站', () => {
      const data = [
        createMockData({ id: '1', category: 'public', sponsorLevel: undefined }),
        createMockData({ id: '2', category: 'commercial', sponsorLevel: 'basic' }),
      ];
      const config: SortConfig = { key: 'badgeScore', direction: 'desc' };

      const result = sortMonitors(data, config);

      // basic(20) > public(10)
      expect(result.map((d) => d.id)).toEqual(['2', '1']);
    });

    it('公益站 + basic 赞助商优先于商业站 + basic 赞助商', () => {
      const data = [
        createMockData({ id: '1', category: 'commercial', sponsorLevel: 'basic', lastCheckLatency: 100 }),
        createMockData({ id: '2', category: 'public', sponsorLevel: 'basic', lastCheckLatency: 100 }),
      ];
      const config: SortConfig = { key: 'badgeScore', direction: 'desc' };

      const result = sortMonitors(data, config);

      // public(10) + basic(20) = 30 > commercial(0) + basic(20) = 20
      expect(result.map((d) => d.id)).toEqual(['2', '1']);
    });
  });
});
