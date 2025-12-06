import { useCallback, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import type { ViewMode, SortConfig } from '../types';

/**
 * URL 查询参数与状态同步的配置
 *
 * 需要同步的状态：
 * - period: 时间范围（默认 24h）
 * - provider: 服务商筛选
 * - service: 服务筛选
 * - channel: 渠道筛选
 * - category: 分类筛选
 * - view: 视图模式（默认 table）
 * - sort: 排序配置（格式：key_direction，如 uptime_desc）
 */

interface UrlState {
  timeRange: string;
  filterProvider: string[];  // 多选服务商，空数组表示"全部"
  filterService: string[];   // 多选服务，空数组表示"全部"
  filterChannel: string[];   // 多选通道，空数组表示"全部"
  filterCategory: string[];  // 多选分类，空数组表示"全部"
  viewMode: ViewMode;
  sortConfig: SortConfig;
  isInitialSort: boolean;    // 是否为初始排序状态（URL 无 sort 参数）
}

interface UrlStateActions {
  setTimeRange: (value: string) => void;
  setFilterProvider: (value: string[]) => void;  // 多选服务商
  setFilterService: (value: string[]) => void;   // 多选服务
  setFilterChannel: (value: string[]) => void;   // 多选通道
  setFilterCategory: (value: string[]) => void;  // 多选分类
  setViewMode: (value: ViewMode) => void;
  setSortConfig: (value: SortConfig) => void;
}

// 默认值
const DEFAULTS = {
  timeRange: '24h',
  filterProvider: [] as string[],  // 空数组表示"全部"
  filterService: [] as string[],   // 空数组表示"全部"
  filterChannel: [] as string[],   // 空数组表示"全部"
  filterCategory: [] as string[],  // 空数组表示"全部"
  viewMode: 'table' as ViewMode,
  sortKey: 'uptime',
  sortDirection: 'desc' as const,
};

// 默认排序参数值（用于判断是否需要保留 URL 参数）
const DEFAULT_SORT_PARAM = `${DEFAULTS.sortKey}_${DEFAULTS.sortDirection}`;

// URL 参数名映射
const PARAM_KEYS = {
  timeRange: 'period',
  filterProvider: 'provider',
  filterService: 'service',
  filterChannel: 'channel',
  filterCategory: 'category',
  viewMode: 'view',
  sort: 'sort',
};

/**
 * 解析排序参数
 * 格式：key_direction，如 uptime_desc、latency_asc
 */
function parseSortParam(param: string | null): SortConfig {
  if (!param) {
    return { key: DEFAULTS.sortKey, direction: DEFAULTS.sortDirection };
  }

  const lastUnderscore = param.lastIndexOf('_');
  if (lastUnderscore === -1) {
    return { key: param, direction: DEFAULTS.sortDirection };
  }

  const key = param.substring(0, lastUnderscore);
  const direction = param.substring(lastUnderscore + 1);

  if (direction === 'asc' || direction === 'desc') {
    return { key, direction };
  }

  return { key: param, direction: DEFAULTS.sortDirection };
}

/**
 * 序列化排序配置为 URL 参数
 */
function serializeSortConfig(config: SortConfig): string {
  return `${config.key}_${config.direction}`;
}

/**
 * 双向同步 URL 查询参数和组件状态的 Hook
 *
 * 特性：
 * - 初始化时从 URL 恢复状态
 * - 状态变化时自动更新 URL
 * - 默认值不会出现在 URL 中（保持 URL 简洁）
 * - 使用 replace 模式避免污染浏览器历史
 */
export function useUrlState(): [UrlState, UrlStateActions] {
  const [searchParams, setSearchParams] = useSearchParams();
  // 会话态标记：用户是否在本次会话中手动点击过排序
  // 刷新页面后会重置为 false，允许置顶恢复
  const [hasManualSort, setHasManualSort] = useState(false);

  // 规范化函数：小写（用于 provider, service, category）
  const normalizeLower = useCallback((value: string) => value.trim().toLowerCase(), []);
  // 规范化函数：保留原始大小写（用于 channel，因为 channel 值来自后端数据）
  const normalizePreserveCase = useCallback((value: string) => value.trim(), []);

  // 从 URL 读取当前状态
  const state = useMemo<UrlState>(() => {
    // 验证 viewMode 参数，防止 URL 被篡改导致内容区空白
    const rawViewMode = searchParams.get(PARAM_KEYS.viewMode);
    const viewMode: ViewMode = (rawViewMode === 'table' || rawViewMode === 'grid')
      ? rawViewMode
      : DEFAULTS.viewMode;

    // 解析多选参数的通用函数（支持逗号分隔）
    // 向后兼容：过滤掉 'all'（旧版"全部"语义），去重并排序
    const parseArrayParam = (
      key: string,
      normalizer: (value: string) => string
    ): string[] => {
      const param = searchParams.get(key);
      if (!param) return [];
      return Array.from(new Set(
        param
          .split(',')
          .map(normalizer)
          .filter(s => s && s.toLowerCase() !== 'all')  // 过滤空值和旧的 'all'
      )).sort();
    };

    // 获取 sort 参数
    const rawSortParam = searchParams.get(PARAM_KEYS.sort);
    const hasSortParam = Boolean(rawSortParam && rawSortParam.trim());

    // 判断是否为初始排序状态
    // 用于赞助商置顶功能：初始状态启用置顶，用户点击排序后失效
    // - 本次会话未手动排序 且 URL 无 sort 参数 → 初始状态
    // - 刷新页面后 hasManualSort 重置为 false，若 URL 无 sort 参数则恢复置顶
    const isInitialSort = !hasManualSort && !hasSortParam;

    return {
      timeRange: searchParams.get(PARAM_KEYS.timeRange) || DEFAULTS.timeRange,
      filterProvider: parseArrayParam(PARAM_KEYS.filterProvider, normalizeLower),
      filterService: parseArrayParam(PARAM_KEYS.filterService, normalizeLower),
      filterChannel: parseArrayParam(PARAM_KEYS.filterChannel, normalizePreserveCase),
      filterCategory: parseArrayParam(PARAM_KEYS.filterCategory, normalizeLower),
      viewMode,
      sortConfig: parseSortParam(rawSortParam),
      isInitialSort,
    };
  }, [searchParams, normalizeLower, normalizePreserveCase, hasManualSort]);

  // 更新单个参数的通用函数
  const updateParam = useCallback((key: string, value: string, defaultValue: string) => {
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev);
      if (value === defaultValue) {
        next.delete(key);
      } else {
        next.set(key, value);
      }
      return next;
    }, { replace: true });
  }, [setSearchParams]);

  // 各个状态的 setter
  const setTimeRange = useCallback((value: string) => {
    updateParam(PARAM_KEYS.timeRange, value, DEFAULTS.timeRange);
  }, [updateParam]);

  // 多选数组参数的通用 setter
  const setArrayParam = useCallback((
    key: string,
    values: string[],
    normalizer: (value: string) => string
  ) => {
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev);
      // 规范化：去空、去重、排序，使用自定义大小写策略
      const normalized = Array.from(new Set(
        values
          .map(normalizer)
          .filter(v => v && v.toLowerCase() !== 'all')
      )).sort();

      if (normalized.length === 0) {
        // 空数组表示"全部"，移除参数
        next.delete(key);
      } else {
        next.set(key, normalized.join(','));
      }
      return next;
    }, { replace: true });
  }, [setSearchParams]);

  const setFilterProvider = useCallback((values: string[]) => {
    setArrayParam(PARAM_KEYS.filterProvider, values, normalizeLower);
  }, [setArrayParam, normalizeLower]);

  const setFilterService = useCallback((values: string[]) => {
    setArrayParam(PARAM_KEYS.filterService, values, normalizeLower);
  }, [setArrayParam, normalizeLower]);

  const setFilterChannel = useCallback((values: string[]) => {
    setArrayParam(PARAM_KEYS.filterChannel, values, normalizePreserveCase);
  }, [setArrayParam, normalizePreserveCase]);

  const setFilterCategory = useCallback((values: string[]) => {
    setArrayParam(PARAM_KEYS.filterCategory, values, normalizeLower);
  }, [setArrayParam, normalizeLower]);

  const setViewMode = useCallback((value: ViewMode) => {
    updateParam(PARAM_KEYS.viewMode, value, DEFAULTS.viewMode);
  }, [updateParam]);

  const setSortConfig = useCallback((config: SortConfig) => {
    // 标记用户已手动排序（本次会话内置顶失效）
    setHasManualSort(true);
    const serialized = serializeSortConfig(config);
    // 默认排序时移除 URL 参数，刷新后可恢复置顶
    // 非默认排序时保留参数
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev);
      if (serialized === DEFAULT_SORT_PARAM) {
        next.delete(PARAM_KEYS.sort);
      } else {
        next.set(PARAM_KEYS.sort, serialized);
      }
      return next;
    }, { replace: true });
  }, [setSearchParams]);

  const actions: UrlStateActions = {
    setTimeRange,
    setFilterProvider,
    setFilterService,
    setFilterChannel,
    setFilterCategory,
    setViewMode,
    setSortConfig,
  };

  return [state, actions];
}
