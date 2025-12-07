import { STATUS } from '../constants';
import type { ProcessedMonitorData, SortConfig, StatusKey, SponsorPinConfig, SponsorLevel } from '../types';
import { calculateBadgeScore, SPONSOR_WEIGHTS } from './badgeUtils';

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
  } else if (key === 'priceRatio') {
    return comparePriceRatio(a.priceRatio, b.priceRatio, direction);
  } else if (key === 'listedDays') {
    return compareListedDays(a.listedDays, b.listedDays, direction);
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
 * priceRatio 特殊排序：null 值始终排最后
 */
function comparePriceRatio(
  aRatio: number | null | undefined,
  bRatio: number | null | undefined,
  direction: 'asc' | 'desc'
): number {
  const aHasData = aRatio != null;
  const bHasData = bRatio != null;

  // null 值始终排最后
  if (aHasData && !bHasData) return -1;
  if (!aHasData && bHasData) return 1;
  if (!aHasData && !bHasData) return 0;

  // 两者都有数据，正常比较
  if (aRatio! < bRatio!) return direction === 'asc' ? -1 : 1;
  if (aRatio! > bRatio!) return direction === 'asc' ? 1 : -1;
  return 0;
}

/**
 * listedDays 特殊排序：null 值始终排最后
 */
function compareListedDays(
  aDays: number | null | undefined,
  bDays: number | null | undefined,
  direction: 'asc' | 'desc'
): number {
  const aHasData = aDays != null;
  const bHasData = bDays != null;

  // null 值始终排最后
  if (aHasData && !bHasData) return -1;
  if (!aHasData && bHasData) return 1;
  if (!aHasData && !bHasData) return 0;

  // 两者都有数据，正常比较
  if (aDays! < bDays!) return direction === 'asc' ? -1 : 1;
  if (aDays! > bDays!) return direction === 'asc' ? 1 : -1;
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

/**
 * 判断监控项是否满足置顶条件
 */
function meetsPinCriteria(
  item: ProcessedMonitorData,
  config: SponsorPinConfig
): boolean {
  // 必须有赞助级别
  if (!item.sponsorLevel) return false;

  // 有风险标记的不参与置顶
  if (item.risks?.length) return false;

  // 可用率必须达标（-1 表示无数据，不符合条件）
  if (item.uptime < 0 || item.uptime < config.min_uptime) return false;

  // 赞助级别必须达到最低要求
  const itemWeight = SPONSOR_WEIGHTS[item.sponsorLevel] || 0;
  const minWeight = SPONSOR_WEIGHTS[config.min_level as SponsorLevel] || 0;
  return itemWeight >= minWeight;
}

/**
 * 带置顶逻辑的排序函数
 *
 * 在页面初始加载时，将符合条件的赞助商置顶显示。
 * 用户点击任意排序按钮后，置顶失效，恢复正常排序。
 *
 * @param data 监控数据数组
 * @param sortConfig 用户排序配置
 * @param pinConfig 置顶配置（来自 API）
 * @param enablePinning 是否启用置顶（初始状态才启用）
 * @returns 排序后的数据，置顶项带 pinned: true 标记
 */
export function sortMonitorsWithPinning(
  data: ProcessedMonitorData[],
  sortConfig: SortConfig,
  pinConfig: SponsorPinConfig | null,
  enablePinning: boolean
): ProcessedMonitorData[] {
  // 复制数据避免修改原数组
  const items = [...data];

  // 置顶逻辑：配置存在、功能启用、且处于初始排序状态
  const shouldPin = pinConfig?.enabled && enablePinning && pinConfig.max_pinned > 0;

  if (!shouldPin) {
    // 不启用置顶：使用常规排序，清除所有 pinned 标记
    return sortMonitors(items, sortConfig).map(item => ({
      ...item,
      pinned: false,
    }));
  }

  // 1. 筛选符合置顶条件的项
  const pinnedCandidates = items.filter(item => meetsPinCriteria(item, pinConfig));

  // 2. 按赞助级别降序排序（同级别按可用率降序）
  pinnedCandidates.sort((a, b) => {
    const aWeight = SPONSOR_WEIGHTS[a.sponsorLevel!] || 0;
    const bWeight = SPONSOR_WEIGHTS[b.sponsorLevel!] || 0;
    if (aWeight !== bWeight) return bWeight - aWeight;
    // 同级别按可用率降序
    return b.uptime - a.uptime;
  });

  // 3. 取前 N 个
  const pinnedItems = pinnedCandidates.slice(0, pinConfig.max_pinned);
  const pinnedIds = new Set(pinnedItems.map(item => item.id));

  // 4. 其余项按可用率降序排序
  const remainingItems = items.filter(item => !pinnedIds.has(item.id));
  const sortedRemaining = sortMonitors(remainingItems, { key: 'uptime', direction: 'desc' });

  // 5. 合并结果，标记置顶项
  return [
    ...pinnedItems.map(item => ({ ...item, pinned: true })),
    ...sortedRemaining.map(item => ({ ...item, pinned: false })),
  ];
}
