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
 *    - latency: 不可用状态的延迟不参与排序，排最后（无二级排序）
 * 3. 二级排序：主字段相等时，按 lastCheckLatency 升序（延迟主排序除外）
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
    // 延迟主排序时不使用二级排序（UNAVAILABLE 的延迟不参与排序）
    if (sortConfig.key === 'latency') {
      return 0;
    }
    // 其他字段：二级排序按延迟升序
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
    return comparePriceRatio(a.priceMin, a.priceMax, b.priceMin, b.priceMax, direction);
  } else if (key === 'listedDays') {
    return compareListedDays(a.listedDays, b.listedDays, direction);
  } else if (key === 'latency') {
    return compareLatencyPrimary(a, b, direction);
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
 * 计算价格排序用的代表值（上限优先）
 * 用户心理：关心"最多付多少"，按上限排序更保护用户
 */
function getPriceValue(
  priceMin: number | null | undefined,
  priceMax: number | null | undefined
): number | null {
  // 优先使用上限（用户最坏情况）
  if (priceMax != null) return priceMax;
  if (priceMin != null) return priceMin;
  return null;
}

/**
 * priceRatio 特殊排序：null 值始终排最后，按上限比较
 */
function comparePriceRatio(
  aMin: number | null | undefined,
  aMax: number | null | undefined,
  bMin: number | null | undefined,
  bMax: number | null | undefined,
  direction: 'asc' | 'desc'
): number {
  const aValue = getPriceValue(aMin, aMax);
  const bValue = getPriceValue(bMin, bMax);

  const aHasData = aValue != null;
  const bHasData = bValue != null;

  // null 值始终排最后
  if (aHasData && !bHasData) return -1;
  if (!aHasData && bHasData) return 1;
  if (!aHasData && !bHasData) return 0;

  // 两者都有数据，正常比较
  if (aValue! < bValue!) return direction === 'asc' ? -1 : 1;
  if (aValue! > bValue!) return direction === 'asc' ? 1 : -1;
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
 * 延迟主排序：不可用状态的延迟不参与排序，排最后
 *
 * 优先级（升序时）：
 * 1. 可用状态 + 有延迟值 → 按延迟排序
 * 2. 可用状态 + 无延迟值 → 排在有延迟的后面
 * 3. UNAVAILABLE 状态 → 始终排最后（无论延迟值）
 */
function compareLatencyPrimary(
  a: ProcessedMonitorData,
  b: ProcessedMonitorData,
  direction: 'asc' | 'desc'
): number {
  const aIsUnavailable = a.currentStatus === 'UNAVAILABLE';
  const bIsUnavailable = b.currentStatus === 'UNAVAILABLE';

  // UNAVAILABLE 状态始终排最后（优先级最低）
  if (aIsUnavailable && !bIsUnavailable) return 1;
  if (!aIsUnavailable && bIsUnavailable) return -1;
  if (aIsUnavailable && bIsUnavailable) return 0; // 两者都是 UNAVAILABLE，保持原顺序

  // 此时两者都是可用状态，判断是否有延迟值
  const aHasLatency = a.lastCheckLatency !== undefined;
  const bHasLatency = b.lastCheckLatency !== undefined;

  // 无延迟值排在有延迟值的后面
  if (aHasLatency && !bHasLatency) return -1;
  if (!aHasLatency && bHasLatency) return 1;
  if (!aHasLatency && !bHasLatency) return 0; // 两者都无延迟，保持原顺序

  // 两者都有延迟值，按延迟比较
  if (a.lastCheckLatency! < b.lastCheckLatency!) return direction === 'asc' ? -1 : 1;
  if (a.lastCheckLatency! > b.lastCheckLatency!) return direction === 'asc' ? 1 : -1;
  return 0;
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
 * 计算单个赞助商的置顶配额
 *
 * 配额规则：
 * - enterprise（顶级）：最多 service_count 个通道
 * - advanced（高级）：最多 max(1, service_count - 1) 个通道
 * - basic（基础）：最多 1 个通道
 */
function getSponsorQuota(sponsorLevel: SponsorLevel, serviceCount: number): number {
  const safeServiceCount = Math.max(1, serviceCount);
  switch (sponsorLevel) {
    case 'enterprise':
      return safeServiceCount;
    case 'advanced':
      return Math.max(1, safeServiceCount - 1);
    case 'basic':
    default:
      return 1;
  }
}

/**
 * 规范化 provider 标识（用于置顶配额分组）
 * 按 provider 分组计算配额，而非 sponsor 字段
 */
function normalizeProviderKey(item: ProcessedMonitorData): string {
  return (item.providerId || '').trim().toLowerCase();
}

/**
 * 带置顶逻辑的排序函数
 *
 * 在页面初始加载时，将符合条件的赞助商置顶显示。
 * 用户点击任意排序按钮后，置顶失效，恢复正常排序。
 *
 * 置顶配额规则（按 provider 计算）：
 * - enterprise（顶级）：最多 service_count 个通道
 * - advanced（高级）：最多 max(1, service_count - 1) 个通道
 * - basic（基础）：最多 1 个通道
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

  // 固定配置值：服务数量（用于按 provider 计算配额；缺失时回退到 3 以兼容旧后端）
  const serviceCount = pinConfig?.service_count ?? 3;

  if (!shouldPin) {
    // 不启用置顶：使用常规排序，清除所有 pinned 标记
    return sortMonitors(items, sortConfig).map(item => ({
      ...item,
      pinned: false,
    }));
  }

  // 1. 筛选符合置顶条件的项
  const pinnedCandidates = items.filter(item => meetsPinCriteria(item, pinConfig));

  // 2. 构建每个 provider 的最高等级 Map（配额按 provider 最高等级计算，而非通道等级）
  const providerHighestLevel = new Map<string, SponsorLevel>();
  for (const item of pinnedCandidates) {
    const providerKey = normalizeProviderKey(item);
    if (!providerKey) continue;

    const currentHighest = providerHighestLevel.get(providerKey);
    if (!currentHighest) {
      providerHighestLevel.set(providerKey, item.sponsorLevel!);
    } else {
      // 比较权重，保留更高等级
      const currentWeight = SPONSOR_WEIGHTS[currentHighest] || 0;
      const newWeight = SPONSOR_WEIGHTS[item.sponsorLevel!] || 0;
      if (newWeight > currentWeight) {
        providerHighestLevel.set(providerKey, item.sponsorLevel!);
      }
    }
  }

  // 3. 候选项全局排序：赞助级别 > 可用率 > 延迟
  pinnedCandidates.sort((a, b) => {
    const aWeight = SPONSOR_WEIGHTS[a.sponsorLevel!] || 0;
    const bWeight = SPONSOR_WEIGHTS[b.sponsorLevel!] || 0;
    if (aWeight !== bWeight) return bWeight - aWeight;
    // 同级别按可用率降序
    const uptimeDiff = b.uptime - a.uptime;
    if (uptimeDiff !== 0) return uptimeDiff;
    // 同级别 + 同可用率：按延迟升序（低延迟优先）
    return compareLatency(a.lastCheckLatency, b.lastCheckLatency);
  });

  // 4. 按 provider 分组计算配额并选择置顶项
  //    同时保留 provider + service 去重规则
  const pinnedItems: ProcessedMonitorData[] = [];
  const pinnedByProvider = new Map<string, number>(); // 每个 provider 已置顶数量
  const pinnedProviderService = new Set<string>(); // provider+service 去重

  for (const item of pinnedCandidates) {
    // 全局截断
    if (pinnedItems.length >= pinConfig.max_pinned) break;

    // 获取 provider 标识用于配额计算
    const providerKey = normalizeProviderKey(item);
    if (!providerKey) continue;

    // 使用 provider 的最高等级计算配额（而非当前通道等级）
    const sponsorLevel = providerHighestLevel.get(providerKey);
    if (!sponsorLevel) continue; // 防御性检查：Map 中应该有该 provider

    const quota = getSponsorQuota(sponsorLevel, serviceCount);

    // 检查配额限制
    const used = pinnedByProvider.get(providerKey) || 0;
    if (used >= quota) continue;

    // 检查 provider + service 去重
    const providerServiceKey = `${item.providerId}|${item.serviceType}`;
    if (pinnedProviderService.has(providerServiceKey)) continue;

    // 通过所有检查，加入置顶列表
    pinnedItems.push(item);
    pinnedByProvider.set(providerKey, used + 1);
    pinnedProviderService.add(providerServiceKey);
  }

  const pinnedIds = new Set(pinnedItems.map(item => item.id));

  // 5. 其余项按可用率降序排序
  const remainingItems = items.filter(item => !pinnedIds.has(item.id));
  const sortedRemaining = sortMonitors(remainingItems, { key: 'uptime', direction: 'desc' });

  // 6. 合并结果，标记置顶项
  return [
    ...pinnedItems.map(item => ({ ...item, pinned: true })),
    ...sortedRemaining.map(item => ({ ...item, pinned: false })),
  ];
}
