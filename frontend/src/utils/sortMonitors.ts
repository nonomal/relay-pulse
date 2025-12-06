import { STATUS } from '../constants';
import type { ProcessedMonitorData, SortConfig, StatusKey } from '../types';
import { calculateBadgeScore } from './badgeUtils';

/**
 * 对监控数据进行排序
 *
 * 排序规则：
 * 1. 按主排序字段排序（支持 asc/desc）
 * 2. 特殊字段处理：
 *    - badgeScore: 按徽标综合分数排序（公益站+10，赞助商正向，风险负向）
 *    - currentStatus: 按状态权重排序
 *    - uptime: uptime < 0 视为无数据，始终排最后
 * 3. 二级排序：主字段相等时，按 lastCheckLatency 升序（延迟低的优先）
 *
 * @param data 监控数据数组
 * @param sortConfig 排序配置
 * @returns 排序后的新数组（不修改原数组）
 */
export function sortMonitors(
  data: ProcessedMonitorData[],
  sortConfig: SortConfig
): ProcessedMonitorData[] {
  if (!sortConfig.key) {
    return [...data];
  }

  return [...data].sort((a, b) => {
    const comparison = comparePrimary(a, b, sortConfig);
    if (comparison !== 0) {
      return comparison;
    }
    // 二级排序：按延迟升序
    return compareLatency(a.lastCheckLatency, b.lastCheckLatency);
  });
}

/**
 * 主排序比较函数
 */
function comparePrimary(
  a: ProcessedMonitorData,
  b: ProcessedMonitorData,
  sortConfig: SortConfig
): number {
  const { key, direction } = sortConfig;

  let aValue: number | string;
  let bValue: number | string;

  // 特殊字段处理
  if (key === 'badgeScore') {
    // 徽标分数排序
    aValue = calculateBadgeScore(a);
    bValue = calculateBadgeScore(b);
  } else if (key === 'currentStatus') {
    aValue = STATUS[a.currentStatus as StatusKey]?.weight ?? 0;
    bValue = STATUS[b.currentStatus as StatusKey]?.weight ?? 0;
  } else if (key === 'uptime') {
    return compareUptime(a.uptime, b.uptime, direction);
  } else {
    aValue = a[key as keyof ProcessedMonitorData] as number | string;
    bValue = b[key as keyof ProcessedMonitorData] as number | string;
  }

  if (aValue < bValue) return direction === 'asc' ? -1 : 1;
  if (aValue > bValue) return direction === 'asc' ? 1 : -1;
  return 0;
}

/**
 * uptime 特殊排序：uptime < 0 视为无数据，始终排最后
 */
function compareUptime(
  aUptime: number,
  bUptime: number,
  direction: 'asc' | 'desc'
): number {
  const aHasData = aUptime >= 0;
  const bHasData = bUptime >= 0;

  // 无数据的始终排最后
  if (aHasData && !bHasData) return -1;
  if (!aHasData && bHasData) return 1;
  if (!aHasData && !bHasData) return 0;

  // 两者都有数据，正常比较
  if (aUptime < bUptime) return direction === 'asc' ? -1 : 1;
  if (aUptime > bUptime) return direction === 'asc' ? 1 : -1;
  return 0;
}

/**
 * 延迟二级排序：升序排列，undefined 排最后
 */
function compareLatency(
  aLatency: number | undefined,
  bLatency: number | undefined
): number {
  // 两者都无数据，保持原顺序
  if (aLatency === undefined && bLatency === undefined) return 0;
  // 无数据的排最后
  if (aLatency === undefined) return 1;
  if (bLatency === undefined) return -1;
  // 按延迟升序（低延迟优先）
  return aLatency - bLatency;
}
