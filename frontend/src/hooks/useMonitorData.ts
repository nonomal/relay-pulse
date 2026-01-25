import { useState, useEffect, useMemo, useCallback, useRef } from 'react';
import i18n from '../i18n';
import type {
  ApiResponseWithGroups,
  MonitorGroup,
  MonitorLayer,
  ProcessedMonitorData,
  SortConfig,
  StatusCounts,
  ProviderOption,
  ChannelOption,
  SponsorLevel,
  SponsorPinConfig,
  BoardFilter,
  MonitorResult,
} from '../types';
import { STATUS_MAP } from '../types';
import { API_BASE_URL, USE_MOCK_DATA } from '../constants';
import { fetchMockMonitorData } from '../utils/mockMonitor';
import { trackAPIPerformance, trackAPIError } from '../utils/analytics';
import { sortMonitorsWithPinning } from '../utils/sortMonitors';

// 请求节流间隔（毫秒）- 防止快速切换参数导致过多请求
const FETCH_THROTTLE_MS = 300;

// URL 二次校验函数
function validateUrl(url: string | undefined): string | null {
  if (!url || url.trim() === '') return null;
  try {
    new URL(url);
    return url;
  } catch {
    console.warn(`Invalid URL: ${url}`);
    return null;
  }
}

// Provider 名称标准化（用于筛选和去重）
function canonicalize(value?: string): string {
  return value?.trim().toLowerCase() ?? '';
}

// Provider 显示标签格式化（保留原始大小写，首字母大写）
function formatProviderLabel(value?: string): string {
  const trimmed = value?.trim();
  if (!trimmed) return i18n.t('common.unknownProvider');
  return trimmed.charAt(0).toUpperCase() + trimmed.slice(1);
}

// 自动轮询间隔（毫秒）- 与后端探测频率 interval: "1m" 保持一致
const POLL_INTERVAL_MS = 60_000;

// 有效的赞助商等级列表（运行时校验）
const SPONSOR_LEVELS: readonly SponsorLevel[] = ['basic', 'advanced', 'enterprise'];
const normalizeSponsorLevel = (level?: string): SponsorLevel | undefined =>
  SPONSOR_LEVELS.includes(level as SponsorLevel) ? (level as SponsorLevel) : undefined;

// 映射状态计数，提供默认值以向后兼容
const mapStatusCounts = (counts?: StatusCounts): StatusCounts => ({
  available: counts?.available ?? 0,
  degraded: counts?.degraded ?? 0,
  unavailable: counts?.unavailable ?? 0,
  missing: counts?.missing ?? 0,
  slow_latency: counts?.slow_latency ?? 0,
  rate_limit: counts?.rate_limit ?? 0,
  server_error: counts?.server_error ?? 0,
  client_error: counts?.client_error ?? 0,
  auth_error: counts?.auth_error ?? 0,
  invalid_request: counts?.invalid_request ?? 0,
  network_error: counts?.network_error ?? 0,
  content_mismatch: counts?.content_mismatch ?? 0,
  http_code_breakdown: counts?.http_code_breakdown, // 透传 HTTP 错误码细分
});

// 状态严重程度：0（红）> 2（黄）> 1（绿）> 3/-1（灰/缺失）
function statusSeverity(status: number): number {
  switch (status) {
    case 0: return 3; // 红色最严重
    case 2: return 2; // 黄色次之
    case 1: return 1; // 绿色
    default: return 0; // 灰色/缺失（包括 status=3 未配置和 status=-1 无数据）
  }
}

// 选择两个状态中较严重的一个
function pickWorstStatus(a: number, b: number): number {
  return statusSeverity(b) > statusSeverity(a) ? b : a;
}

// 合并多个层的状态计数
function mergeStatusCounts(points: Array<{ status_counts?: StatusCounts }>): StatusCounts {
  const merged: StatusCounts = {
    available: 0,
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
  };

  points.forEach((p) => {
    const counts = p.status_counts;
    if (!counts) return;
    merged.available += counts.available ?? 0;
    merged.degraded += counts.degraded ?? 0;
    merged.unavailable += counts.unavailable ?? 0;
    merged.missing += counts.missing ?? 0;
    merged.slow_latency += counts.slow_latency ?? 0;
    merged.rate_limit += counts.rate_limit ?? 0;
    merged.server_error += counts.server_error ?? 0;
    merged.client_error += counts.client_error ?? 0;
    merged.auth_error += counts.auth_error ?? 0;
    merged.invalid_request += counts.invalid_request ?? 0;
    merged.network_error += counts.network_error ?? 0;
    merged.content_mismatch += counts.content_mismatch ?? 0;

    // 合并 http_code_breakdown
    if (counts.http_code_breakdown) {
      if (!merged.http_code_breakdown) {
        merged.http_code_breakdown = {};
      }
      Object.entries(counts.http_code_breakdown).forEach(([subStatus, codes]) => {
        if (!merged.http_code_breakdown![subStatus]) {
          merged.http_code_breakdown![subStatus] = {};
        }
        Object.entries(codes).forEach(([code, count]) => {
          const codeNum = parseInt(code, 10);
          merged.http_code_breakdown![subStatus][codeNum] =
            (merged.http_code_breakdown![subStatus][codeNum] ?? 0) + count;
        });
      });
    }
  });

  return merged;
}

type LayerTimelinePoint = NonNullable<MonitorLayer['timeline']>[number];

// 估算时间轴相邻点间隔（秒），用于推导时间戳对齐容差
function estimateTimelineStepSeconds(timeline: Array<{ timestamp: number }>): number {
  const deltas: number[] = [];
  for (let i = 1; i < timeline.length; i += 1) {
    const delta = Math.abs(timeline[i].timestamp - timeline[i - 1].timestamp);
    if (delta > 0) deltas.push(delta);
    if (deltas.length >= 10) break;
  }
  if (deltas.length === 0) return 60;
  deltas.sort((a, b) => a - b);
  return deltas[Math.floor(deltas.length / 2)];
}

// 在已按 timestamp 升序的 timeline 中，查找与目标时间戳最接近且在容差内的点（秒级）
function findClosestPointByTimestamp(
  sortedTimeline: LayerTimelinePoint[],
  targetTimestamp: number,
  toleranceSeconds: number
): LayerTimelinePoint | undefined {
  if (sortedTimeline.length === 0) return undefined;

  // 二分查找：找到第一个 timestamp >= targetTimestamp 的位置
  let left = 0;
  let right = sortedTimeline.length;
  while (left < right) {
    const mid = (left + right) >> 1;
    if (sortedTimeline[mid].timestamp < targetTimestamp) {
      left = mid + 1;
    } else {
      right = mid;
    }
  }

  // 候选点：当前位置和前一个位置
  const candidates: LayerTimelinePoint[] = [];
  if (left < sortedTimeline.length) candidates.push(sortedTimeline[left]);
  if (left - 1 >= 0) candidates.push(sortedTimeline[left - 1]);

  // 找到时间差最小的点
  let best: LayerTimelinePoint | undefined;
  let bestDiff = Number.POSITIVE_INFINITY;
  candidates.forEach((p) => {
    const diff = Math.abs(p.timestamp - targetTimestamp);
    if (diff < bestDiff) {
      bestDiff = diff;
      best = p;
    }
  });

  return best && bestDiff <= toleranceSeconds ? best : undefined;
}

// 从多个层构建综合时间轴（每个时间点取最差状态）
// 使用时间戳对齐而非索引对齐，解决多模型组因配置变更导致的 timeline 长度不一致问题
function buildCompositeTimelineFromLayers(
  layers: MonitorLayer[]
): Array<{
  status: number;
  time: string;
  timestamp: number;
  latency: number;
  availability: number;
  status_counts: StatusCounts;
}> {
  if (layers.length === 0) return [];

  // 使用父层（layer_order=0）作为基准时间轴，如果不存在则使用第一个层
  const baseLayer = layers.find((layer) => layer.layer_order === 0) ?? layers[0];
  const baseTimeline = baseLayer.timeline ?? [];
  if (baseTimeline.length === 0) return [];

  // 计算时间戳对齐容差：默认取步长一半
  // - 90m 模式常见步长约 180s（3 分钟），调度/网络抖动可能到几十秒；15s 上限会误判为缺失
  // - 钳制在 [10s, 120s]，避免跨桶误匹配
  const baseStepSeconds = estimateTimelineStepSeconds(baseTimeline);
  const toleranceSeconds = Math.max(10, Math.min(120, Math.floor(baseStepSeconds / 2)));

  // 预排序各层 timeline，便于按时间戳做二分查找
  // 优化：先检查是否已升序，避免不必要的排序（后端通常已按时间升序返回）
  const sortedLayerTimelines = layers.map((layer) => {
    const timeline = layer.timeline ?? [];
    // 检查是否已升序
    let isSorted = true;
    for (let i = 1; i < timeline.length && isSorted; i++) {
      if (timeline[i].timestamp < timeline[i - 1].timestamp) {
        isSorted = false;
      }
    }
    return {
      layer_order: layer.layer_order,
      timeline: isSorted ? timeline : timeline.slice().sort((a, b) => a.timestamp - b.timestamp),
    };
  });

  // 对每个时间点，计算所有层的综合状态
  return baseTimeline.map((basePoint) => {
    const targetTimestamp = basePoint.timestamp;

    // 按时间戳对齐：对每层取与 targetTimestamp 最接近且在容差内的点
    const points = sortedLayerTimelines
      .map(({ timeline }) => findClosestPointByTimestamp(timeline, targetTimestamp, toleranceSeconds))
      .filter((p): p is LayerTimelinePoint => Boolean(p));

    // 计算最差状态
    let worstStatus = -1;
    points.forEach((p) => {
      worstStatus = pickWorstStatus(worstStatus, p.status);
    });

    // 可用率：取所有层的最小值（最差层决定整体）
    const availabilities = points.map((p) => p.availability).filter((a) => a >= 0);
    const availability = availabilities.length > 0 ? Math.min(...availabilities) : -1;

    // 延迟：取所有层的最大值（最慢层决定整体）
    const latencies = points.map((p) => p.latency).filter((l) => l > 0);
    const latency = latencies.length > 0 ? Math.max(...latencies) : 0;

    // 合并状态计数
    const status_counts = mergeStatusCounts(points);

    return {
      status: worstStatus,
      time: basePoint.time,
      timestamp: basePoint.timestamp,
      latency,
      availability,
      status_counts,
    };
  });
}

// 计算可用率：仅统计有数据的时间块
function calculateUptime(points: Array<{ availability: number }>): number {
  const validPoints = points.filter((point) => point.availability >= 0);
  if (validPoints.length === 0) return -1;
  return parseFloat((
    validPoints.reduce((acc, point) => acc + point.availability, 0) / validPoints.length
  ).toFixed(2));
}

// 从 timeline 构建 history 数组
function buildHistoryFromTimeline(
  timeline: Array<{
    status: number;
    time: string;
    timestamp: number;
    latency: number;
    availability: number;
    status_counts?: StatusCounts;
  }>,
  slowLatencyMs: number,
  model?: string,
  layerOrder?: number
): ProcessedMonitorData['history'] {
  return (timeline || []).map((point, index) => ({
    index,
    status: STATUS_MAP[point.status] || 'UNAVAILABLE',
    timestamp: point.time,
    timestampNum: point.timestamp,
    latency: point.latency,
    availability: point.availability,
    statusCounts: mapStatusCounts(point.status_counts),
    slowLatencyMs,
    model,
    layerOrder,
  }));
}

// 选择最后检查时所用的层（优先父层，否则取时间戳最新的层）
function pickLastCheckLayer(layers: MonitorLayer[]): MonitorLayer | undefined {
  if (layers.length === 0) return undefined;
  // 优先选择父层（layer_order=0）
  const parent = layers.find((layer) => layer.layer_order === 0);
  if (parent) return parent;
  // 否则取时间戳最新的层
  return layers.reduce((latest, layer) => {
    const latestTs = latest.current_status?.timestamp ?? 0;
    const currentTs = layer.current_status?.timestamp ?? 0;
    return currentTs > latestTs ? layer : latest;
  }, layers[0]);
}

// 将 legacy MonitorResult 转换为 ProcessedMonitorData（单层，isMultiModel=false）
function convertLegacyDataToProcessedData(
  item: MonitorResult,
  globalSlowLatencyMs: number
): ProcessedMonitorData {
  // 计算当前监测项的 slowLatencyMs（优先 per-monitor，否则用全局值）
  const itemSlowLatencyMs = item.slow_latency_ms ?? globalSlowLatencyMs;

  const history = buildHistoryFromTimeline(item.timeline, itemSlowLatencyMs);
  const uptime = calculateUptime(history);

  const currentStatus = item.current_status
    ? STATUS_MAP[item.current_status.status] || 'UNAVAILABLE'
    : 'MISSING';

  // 标准化 provider 名称
  const providerKey = canonicalize(item.provider);
  const providerLabel = item.provider_name || formatProviderLabel(item.provider);
  const serviceName = item.service_name || item.service;
  const channelName = item.channel_name || item.channel;

  return {
    id: `${providerKey || item.provider}-${item.service}-${item.channel || 'default'}`,
    providerId: providerKey || item.provider,
    providerSlug: item.provider_slug || canonicalize(item.provider),
    providerName: providerLabel,
    providerUrl: validateUrl(item.provider_url),
    serviceType: item.service,
    serviceName,
    category: item.category,
    sponsor: item.sponsor,
    sponsorUrl: validateUrl(item.sponsor_url),
    sponsorLevel: normalizeSponsorLevel(item.sponsor_level),
    risks: item.risks,
    badges: item.badges,
    priceMin: item.price_min ?? null,
    priceMax: item.price_max ?? null,
    listedDays: item.listed_days ?? null,
    channel: item.channel || undefined,
    channelName: channelName || undefined,
    board: item.board || 'hot',
    coldReason: item.cold_reason || undefined,
    probeUrl: item.probe_url,
    templateName: item.template_name,
    intervalMs: item.interval_ms ?? 0,
    slowLatencyMs: itemSlowLatencyMs,
    history,
    currentStatus,
    uptime,
    lastCheckTimestamp: item.current_status?.timestamp,
    lastCheckLatency: item.current_status?.latency,
    isMultiModel: false,  // Legacy data is single-layer
  };
}

// 将 MonitorGroup 转换为 ProcessedMonitorData（多层，isMultiModel=true）
function convertGroupToProcessedData(
  group: MonitorGroup,
  globalSlowLatencyMs: number
): ProcessedMonitorData {
  const itemSlowLatencyMs = group.slow_latency_ms ?? globalSlowLatencyMs;

  // 构建综合时间轴（每个时间点取所有层的最差状态）
  const compositeTimeline = buildCompositeTimelineFromLayers(group.layers);
  const history = compositeTimeline.length > 0
    ? buildHistoryFromTimeline(
        compositeTimeline,
        itemSlowLatencyMs,
        i18n.t('multiModel.composite'),  // 使用翻译的"综合"标记
        undefined // 综合状态无单一层序号
      )
    : [];

  // 组级可用率：取所有层可用率的最小值（最差层决定整体）
  const layerUptimes = group.layers.map((layer) =>
    calculateUptime(buildHistoryFromTimeline(layer.timeline, itemSlowLatencyMs))
  ).filter((u) => u >= 0);
  const uptime = layerUptimes.length > 0 ? Math.min(...layerUptimes) : -1;

  // 组级当前状态：直接使用后端计算的 current_status
  const currentStatus = STATUS_MAP[group.current_status] || 'MISSING';

  // 最后检查时间和延迟：优先父层，否则取最新层
  const lastCheckLayer = pickLastCheckLayer(group.layers);
  const lastCheckTimestamp = lastCheckLayer?.current_status?.timestamp;
  const lastCheckLatency = lastCheckLayer?.current_status?.latency;

  // 标准化 provider 名称
  const providerKey = canonicalize(group.provider);
  const providerLabel = group.provider_name || formatProviderLabel(group.provider);
  const serviceName = group.service_name || group.service;
  const channelName = group.channel_name || group.channel;

  return {
    id: `${providerKey || group.provider}-${group.service}-${group.channel || 'default'}`,
    providerId: providerKey || group.provider,
    providerSlug: group.provider_slug || canonicalize(group.provider),
    providerName: providerLabel,
    providerUrl: validateUrl(group.provider_url),
    serviceType: group.service,
    serviceName,
    category: group.category,
    sponsor: group.sponsor,
    sponsorUrl: validateUrl(group.sponsor_url),
    sponsorLevel: normalizeSponsorLevel(group.sponsor_level),
    risks: group.risks,
    badges: group.badges,
    priceMin: group.price_min ?? null,
    priceMax: group.price_max ?? null,
    listedDays: group.listed_days ?? null,
    channel: group.channel || undefined,
    channelName: channelName || undefined,
    board: group.board || 'hot',
    coldReason: group.cold_reason || undefined,
    probeUrl: group.probe_url,
    templateName: group.template_name,
    intervalMs: group.interval_ms ?? 0,
    slowLatencyMs: itemSlowLatencyMs,
    history,
    currentStatus,
    uptime,
    lastCheckTimestamp,
    lastCheckLatency,
    isMultiModel: group.layers.length > 1,   // 只有多于 1 层才是真正的多模型
    layers: group.layers, // 保留原始分层数据
  };
}

interface UseMonitorDataOptions {
  timeRange: string;
  timeAlign?: string;        // 时间对齐模式：空=动态滑动窗口, "hour"=整点对齐
  timeFilter?: string | null; // 每日时段过滤：null=全天, "09:00-17:00"=自定义
  board?: BoardFilter;       // 板块过滤：hot/secondary/cold/all（默认 hot）
  filterService: string[];   // 多选服务，空数组表示"全部"
  filterProvider: string[];  // 多选服务商，空数组表示"全部"
  filterChannel: string[];   // 多选通道，空数组表示"全部"
  filterCategory: string[];  // 多选分类，空数组表示"全部"
  sortConfig: SortConfig;
  isInitialSort: boolean;    // 是否为初始排序状态（用于赞助商置顶）
  autoRefresh?: boolean;     // 自动刷新开关，默认开启
}

export function useMonitorData({
  timeRange,
  timeAlign = '',
  timeFilter = null,
  board = 'hot',
  filterService,
  filterProvider,
  filterChannel,
  filterCategory,
  sortConfig,
  isInitialSort,
  autoRefresh = true,
}: UseMonitorDataOptions) {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [rawData, setRawData] = useState<ProcessedMonitorData[]>([]);
  const [reloadToken, setReloadToken] = useState(0);
  const skipCacheRef = useRef(false); // 使用 ref 避免触发 effect 重新执行
  const [slowLatencyMs, setSlowLatencyMs] = useState<number>(5000); // 默认 5 秒
  const [enableBadges, setEnableBadges] = useState<boolean>(true); // 徽标系统总开关（默认启用）
  const [boardsEnabled, setBoardsEnabled] = useState<boolean>(false); // 板块功能开关（默认禁用）
  const [boardsEnabledLoaded, setBoardsEnabledLoaded] = useState<boolean>(false); // 是否已从 API 获取板块开关状态
  const [sponsorPinConfig, setSponsorPinConfig] = useState<SponsorPinConfig | null>(null); // 赞助商置顶配置
  const [allMonitorIds, setAllMonitorIds] = useState<Set<string>>(new Set()); // 全量监控项 ID（用于清理无效收藏）
  const [allMonitorIdsSupported, setAllMonitorIdsSupported] = useState<boolean>(false); // 后端是否支持 all_monitor_ids

  // 统一的刷新触发器，供手动刷新与自动轮询复用
  // skipCache: 是否绕过浏览器缓存（手动刷新时应为 true）
  const triggerRefetch = useCallback((skipCache = false) => {
    setLoading(true);
    if (skipCache) {
      skipCacheRef.current = true; // 使用 ref 设置标志
    }
    setReloadToken((token) => token + 1);
  }, []);

  // 数据获取 - 支持双模式（Mock / API）
  // 使用 debounce 防止快速切换参数导致过多请求
  useEffect(() => {
    let isMounted = true;
    let debounceTimer: ReturnType<typeof setTimeout> | null = null;

    const fetchData = async () => {
      setLoading(true);
      setError(null);

      // 记录开始时间（在 try 外面，确保网络错误也能追踪性能）
      const startTime = USE_MOCK_DATA ? 0 : performance.now();

      try {
        let processed: ProcessedMonitorData[];

        if (USE_MOCK_DATA) {
          // 使用模拟数据 - 完全复刻 docs/front.jsx
          processed = await fetchMockMonitorData(timeRange);
          // Mock 数据模式：视为板块功能可用（便于本地调试）
          setBoardsEnabled(true);
          setBoardsEnabledLoaded(true);
        } else {
          // 使用真实 API
          // align 参数仅在 24h 模式下有效
          const alignParam = (timeAlign && timeRange === '24h') ? `&align=${encodeURIComponent(timeAlign)}` : '';
          // time_filter 参数仅在 7d/30d 模式下有效（90m/24h 关闭）
          const timeFilterParam =
            (timeFilter && timeRange !== '24h' && timeRange !== '90m')
              ? `&time_filter=${encodeURIComponent(timeFilter)}`
              : '';
          // board 参数：默认 hot
          const boardParam = `&board=${encodeURIComponent(board)}`;
          const url = `${API_BASE_URL}/api/status?period=${timeRange}${alignParam}${timeFilterParam}${boardParam}`;

          // 读取并重置 skipCache 标志
          const shouldSkipCache = skipCacheRef.current;
          if (shouldSkipCache) {
            skipCacheRef.current = false; // 立即重置，避免影响后续请求
          }

          // 手动刷新时绕过浏览器缓存
          const fetchOptions: RequestInit = shouldSkipCache ? { cache: 'no-store' } : {};
          const response = await fetch(url, fetchOptions);

          const duration = Math.round(performance.now() - startTime);

          if (!response.ok) {
            // 追踪 HTTP 错误（失败的性能和错误事件）
            trackAPIPerformance('/api/status', duration, false);
            trackAPIError('/api/status', `HTTP_${response.status}`, 'HTTP Error');
            throw new Error(`HTTP error! status: ${response.status}`);
          }

          const json: ApiResponseWithGroups = await response.json();

          // 追踪成功的 API 性能
          trackAPIPerformance('/api/status', duration, true);

          // 提取慢延迟阈值（用于延迟颜色渐变）
          if (json.meta.slow_latency_ms && json.meta.slow_latency_ms > 0) {
            setSlowLatencyMs(json.meta.slow_latency_ms);
          }

          // 提取徽标系统总开关（默认 true，兼容旧后端）
          setEnableBadges(json.meta.enable_badges !== false);

          // 提取赞助商置顶配置
          if (json.meta.sponsor_pin) {
            setSponsorPinConfig(json.meta.sponsor_pin);
          }

          // 提取板块功能开关（默认禁用，兼容旧后端）
          setBoardsEnabled(json.meta.boards?.enabled === true);
          setBoardsEnabledLoaded(true);

          // 提取全量监控项 ID（用于清理无效收藏，兼容旧后端）
          // 字段缺失时重置为空集，避免保留旧值导致误删
          if (Array.isArray(json.meta.all_monitor_ids)) {
            // 过滤非字符串元素并 trim，确保数据干净
            const validIds = json.meta.all_monitor_ids
              .filter((id): id is string => typeof id === 'string')
              .map((id) => id.trim())
              .filter((id) => id !== '');
            setAllMonitorIds(new Set(validIds));
            setAllMonitorIdsSupported(true); // 后端支持该字段
          } else {
            setAllMonitorIds(new Set());
            setAllMonitorIdsSupported(false); // 旧后端不支持
          }

          // 统一转换：合并 legacy data 和 groups
          const globalSlowLatencyMs = json.meta.slow_latency_ms ?? 5000;
          const legacy = (json.data || []).map((item) =>
            convertLegacyDataToProcessedData(item, globalSlowLatencyMs)
          );
          const groups = (Array.isArray(json.groups) ? json.groups : []).map((g) =>
            convertGroupToProcessedData(g, globalSlowLatencyMs)
          );
          processed = [...legacy, ...groups];
        }

        // 防止组件卸载后的状态更新
        if (!isMounted) return;
        setRawData(processed);
      } catch (err) {
        if (!isMounted) return;
        const errorMessage = err instanceof Error ? err.message : 'Unknown error';
        setError(errorMessage);

        // 只追踪真正的网络错误（fetch 失败、连接超时等）
        // HTTP 错误已经在上面追踪过了，避免重复
        if (!USE_MOCK_DATA && !errorMessage.startsWith('HTTP error!')) {
          const duration = Math.round(performance.now() - startTime);
          // 追踪网络错误的性能和错误事件
          trackAPIPerformance('/api/status', duration, false);
          trackAPIError('/api/status', 'NETWORK_ERROR', 'Network failure');
        }
      } finally {
        if (isMounted) {
          setLoading(false);
        }
      }
    };

    // 使用 debounce 延迟请求，防止快速切换参数
    // 注意：skipCacheRef 是 ref，不会触发 effect，直接在 fetchData 中读取
    debounceTimer = setTimeout(fetchData, FETCH_THROTTLE_MS);

    return () => {
      isMounted = false;
      if (debounceTimer) {
        clearTimeout(debounceTimer);
      }
    };
  }, [timeRange, timeAlign, timeFilter, board, reloadToken]);

  // 页面可见性驱动的自动轮询
  useEffect(() => {
    // SSR 环境保护
    if (typeof document === 'undefined') return;

    let intervalId: ReturnType<typeof setInterval> | null = null;

    const startPolling = () => {
      // 自动刷新关闭时不启动轮询
      if (!autoRefresh) return;
      if (document.visibilityState !== 'visible' || intervalId) return;
      intervalId = setInterval(triggerRefetch, POLL_INTERVAL_MS);
    };

    const stopPolling = () => {
      if (!intervalId) return;
      clearInterval(intervalId);
      intervalId = null;
    };

    const handleVisibilityChange = () => {
      if (document.visibilityState === 'visible') {
        // 仅在自动刷新开启时才触发刷新和启动轮询
        if (autoRefresh) {
          triggerRefetch(); // 页面重新可见时立即刷新
          startPolling();
        }
      } else {
        stopPolling();
      }
    };

    // 初始化：仅在页面可见且自动刷新开启时启动轮询
    if (autoRefresh) {
      startPolling();
    }
    document.addEventListener('visibilitychange', handleVisibilityChange);

    return () => {
      stopPolling();
      document.removeEventListener('visibilitychange', handleVisibilityChange);
    };
  }, [triggerRefetch, autoRefresh]);

  // 提取所有通道列表（去重并排序）
  // 返回 ChannelOption[] 格式，支持 label/value 分离
  const channels = useMemo<ChannelOption[]>(() => {
    const map = new Map<string, string>();  // value (channel) -> label (channelName)
    rawData.forEach((item) => {
      if (item.channel) {
        // 如果同一个 channel 有多个 channelName，保留第一个
        if (!map.has(item.channel)) {
          map.set(item.channel, item.channelName || item.channel);
        }
      }
    });
    return Array.from(map.entries())
      .sort((a, b) => a[1].localeCompare(b[1], 'zh-CN'))  // 按 label 排序
      .map(([value, label]) => ({ value, label }));
  }, [rawData]);

  // 提取所有服务商列表（去重并排序）
  // 返回 ProviderOption[] 格式，支持 label/value 分离
  const providers = useMemo<ProviderOption[]>(() => {
    const map = new Map<string, string>();  // value -> label
    rawData.forEach((item) => {
      if (item.providerId) {
        // 如果同一个 providerId 有多个 providerName，保留第一个
        if (!map.has(item.providerId)) {
          map.set(item.providerId, item.providerName);
        }
      }
    });
    return Array.from(map.entries())
      .sort((a, b) => a[1].localeCompare(b[1], 'zh-CN'))  // 按 label 排序
      .map(([value, label]) => ({ value, label }));
  }, [rawData]);

  // 数据过滤和排序
  const processedData = useMemo(() => {
    // 多选过滤：空数组表示"全部"
    const providerSet = filterProvider.length > 0 ? new Set(filterProvider) : null;
    const serviceSet = filterService.length > 0 ? new Set(filterService) : null;
    const channelSet = filterChannel.length > 0 ? new Set(filterChannel) : null;
    const categorySet = filterCategory.length > 0 ? new Set(filterCategory) : null;

    const filtered = rawData.filter((item) => {
      const matchService = serviceSet === null || serviceSet.has(item.serviceType.toLowerCase());
      const matchProvider = providerSet === null || providerSet.has(item.providerId);
      const matchChannel = channelSet === null || (item.channel && channelSet.has(item.channel));
      const matchCategory = categorySet === null || (item.category && categorySet.has(item.category));
      return matchService && matchProvider && matchChannel && matchCategory;
    });

    // 使用带置顶逻辑的排序函数
    return sortMonitorsWithPinning(filtered, sortConfig, sponsorPinConfig, isInitialSort);
  }, [rawData, filterService, filterProvider, filterChannel, filterCategory, sortConfig, sponsorPinConfig, isInitialSort]);

  // 统计数据
  const stats = useMemo(() => {
    const total = processedData.length;
    const healthy = processedData.filter((i) => i.currentStatus === 'AVAILABLE').length;
    const issues = total - healthy;
    return { total, healthy, issues };
  }, [processedData]);

  return {
    loading,
    error,
    data: processedData,
    rawData,  // 未过滤的原始数据，供 App.tsx 计算 effectiveXxx 使用
    stats,
    channels,
    providers,
    slowLatencyMs,
    enableBadges,
    boardsEnabled,  // 板块功能开关
    boardsEnabledLoaded,  // 是否已从 API 获取板块开关状态
    allMonitorIds,  // 全量监控项 ID（用于清理无效收藏）
    allMonitorIdsSupported, // 后端是否支持 all_monitor_ids（用于区分"空列表"和"不支持"）
    refetch: triggerRefetch,
  };
}
