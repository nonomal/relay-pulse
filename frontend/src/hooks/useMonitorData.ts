import { useState, useEffect, useMemo, useCallback } from 'react';
import i18n from '../i18n';
import type {
  ApiResponse,
  ProcessedMonitorData,
  SortConfig,
  StatusKey,
  StatusCounts,
  ProviderOption,
  SponsorLevel,
} from '../types';
import { API_BASE_URL, USE_MOCK_DATA, NO_DATA_AVAILABILITY } from '../constants';
import { fetchMockMonitorData } from '../utils/mockMonitor';
import { trackAPIPerformance, trackAPIError } from '../utils/analytics';
import { sortMonitors } from '../utils/sortMonitors';

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

// 导入 STATUS_MAP
const statusMap: Record<number, StatusKey> = {
  1: 'AVAILABLE',
  2: 'DEGRADED',
  0: 'UNAVAILABLE',
  3: 'MISSING',  // 未配置/认证失败
  '-1': 'MISSING',  // 缺失数据
};

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
});

interface UseMonitorDataOptions {
  timeRange: string;
  timeAlign?: string; // 时间对齐模式：空=动态滑动窗口, "hour"=整点对齐
  filterService: string[];   // 多选服务，空数组表示"全部"
  filterProvider: string[];  // 多选服务商，空数组表示"全部"
  filterChannel: string[];   // 多选通道，空数组表示"全部"
  filterCategory: string[];  // 多选分类，空数组表示"全部"
  sortConfig: SortConfig;
}

export function useMonitorData({
  timeRange,
  timeAlign = '',
  filterService,
  filterProvider,
  filterChannel,
  filterCategory,
  sortConfig,
}: UseMonitorDataOptions) {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [rawData, setRawData] = useState<ProcessedMonitorData[]>([]);
  const [reloadToken, setReloadToken] = useState(0);
  const [forceRefresh, setForceRefresh] = useState(false); // 手动刷新时绕过缓存
  const [slowLatencyMs, setSlowLatencyMs] = useState<number>(5000); // 默认 5 秒

  // 统一的刷新触发器，供手动刷新与自动轮询复用
  // skipCache: 是否绕过浏览器缓存（手动刷新时应为 true）
  const triggerRefetch = useCallback((skipCache = false) => {
    setLoading(true);
    if (skipCache) {
      setForceRefresh(true);
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
        } else {
          // 使用真实 API
          // align 参数仅在 24h 模式下有效
          const alignParam = (timeAlign && timeRange === '24h') ? `&align=${encodeURIComponent(timeAlign)}` : '';
          const url = `${API_BASE_URL}/api/status?period=${timeRange}${alignParam}`;

          // 手动刷新时绕过浏览器缓存
          const fetchOptions: RequestInit = forceRefresh ? { cache: 'no-store' } : {};
          const response = await fetch(url, fetchOptions);

          // 重置强制刷新标记
          if (forceRefresh) {
            setForceRefresh(false);
          }
          const duration = Math.round(performance.now() - startTime);

          if (!response.ok) {
            // 追踪 HTTP 错误（失败的性能和错误事件）
            trackAPIPerformance('/api/status', duration, false);
            trackAPIError('/api/status', `HTTP_${response.status}`, 'HTTP Error');
            throw new Error(`HTTP error! status: ${response.status}`);
          }

          const json: ApiResponse = await response.json();

          // 追踪成功的 API 性能
          trackAPIPerformance('/api/status', duration, true);

          // 提取慢延迟阈值（用于延迟颜色渐变）
          if (json.meta.slow_latency_ms && json.meta.slow_latency_ms > 0) {
            setSlowLatencyMs(json.meta.slow_latency_ms);
          }

          // 转换为前端数据格式
          processed = json.data.map((item) => {
            const history = item.timeline.map((point, index) => ({
              index,
              status: statusMap[point.status] || 'UNAVAILABLE',
              timestamp: point.time,
              timestampNum: point.timestamp,  // Unix 时间戳（秒）
              latency: point.latency,
              availability: point.availability,  // 可用率百分比
              statusCounts: mapStatusCounts(point.status_counts), // 映射状态计数
            }));

            const currentStatus = item.current_status
              ? statusMap[item.current_status.status] || 'UNAVAILABLE'
              : 'UNAVAILABLE';

            // 计算可用率：无数据时间块按 NO_DATA_AVAILABILITY (90%) 计入
            // - 避免新服务商因历史数据少而显示过高可用率
            // - 若全部时间块均无数据，返回 -1 由 UI 层展示为 "--"
            const hasAnyData = history.some(point => point.availability >= 0);
            const uptime = history.length > 0 && hasAnyData
              ? parseFloat((
                  history.reduce((acc, point) =>
                    acc + (point.availability >= 0 ? point.availability : NO_DATA_AVAILABILITY), 0
                  ) / history.length
                ).toFixed(2))
              : -1;

            // 标准化 provider 名称
            const providerKey = canonicalize(item.provider);
            const providerLabel = formatProviderLabel(item.provider);

            return {
              id: `${providerKey || item.provider}-${item.service}-${item.channel || 'default'}`,
              providerId: providerKey || item.provider,  // 规范化的 ID（小写）
              providerSlug: item.provider_slug || canonicalize(item.provider), // URL slug
              providerName: providerLabel,  // 格式化的显示名称
              providerUrl: validateUrl(item.provider_url),
              serviceType: item.service,
              category: item.category,
              sponsor: item.sponsor,
              sponsorUrl: validateUrl(item.sponsor_url),
              sponsorLevel: normalizeSponsorLevel(item.sponsor_level),
              channel: item.channel || undefined,
              history,
              currentStatus,
              uptime,
              lastCheckTimestamp: item.current_status?.timestamp,
              lastCheckLatency: item.current_status?.latency,
            };
          });
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
    // forceRefresh（手动刷新）时立即执行，不走 debounce
    if (forceRefresh) {
      fetchData();
    } else {
      debounceTimer = setTimeout(fetchData, FETCH_THROTTLE_MS);
    }

    return () => {
      isMounted = false;
      if (debounceTimer) {
        clearTimeout(debounceTimer);
      }
    };
  }, [timeRange, timeAlign, reloadToken, forceRefresh]);

  // 页面可见性驱动的自动轮询
  useEffect(() => {
    // SSR 环境保护
    if (typeof document === 'undefined') return;

    let intervalId: ReturnType<typeof setInterval> | null = null;

    const startPolling = () => {
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
        triggerRefetch(); // 页面重新可见时立即刷新
        startPolling();
      } else {
        stopPolling();
      }
    };

    // 初始化：仅在页面可见时启动轮询
    startPolling();
    document.addEventListener('visibilitychange', handleVisibilityChange);

    return () => {
      stopPolling();
      document.removeEventListener('visibilitychange', handleVisibilityChange);
    };
  }, [triggerRefetch]);

  // 提取所有通道列表（去重并排序）
  const channels = useMemo(() => {
    const set = new Set<string>();
    rawData.forEach((item) => {
      if (item.channel) {
        set.add(item.channel);
      }
    });
    return Array.from(set).sort();
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
      const matchService = serviceSet === null || serviceSet.has(item.serviceType);
      const matchProvider = providerSet === null || providerSet.has(item.providerId);
      const matchChannel = channelSet === null || (item.channel && channelSet.has(item.channel));
      const matchCategory = categorySet === null || (item.category && categorySet.has(item.category));
      return matchService && matchProvider && matchChannel && matchCategory;
    });

    return sortMonitors(filtered, sortConfig);
  }, [rawData, filterService, filterProvider, filterChannel, filterCategory, sortConfig]);

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
    stats,
    channels,
    providers,
    slowLatencyMs,
    refetch: triggerRefetch,
  };
}
