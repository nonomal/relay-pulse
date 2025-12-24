import { useState, useCallback, useRef, useEffect, useMemo } from 'react';
import { useParams, useSearchParams } from 'react-router-dom';
import { Helmet } from 'react-helmet-async';
import { useTranslation } from 'react-i18next';
import { Server } from 'lucide-react';
import { useMonitorData } from '../hooks/useMonitorData';
import { useFavorites } from '../hooks/useFavorites';
import { Header } from '../components/Header';
import { Controls } from '../components/Controls';
import { StatusTable } from '../components/StatusTable';
import { StatusCard } from '../components/StatusCard';
import { Tooltip } from '../components/Tooltip';
import { Footer } from '../components/Footer';
import { EmptyFavorites } from '../components/EmptyFavorites';
import { createMediaQueryEffect } from '../utils/mediaQuery';
import type { ViewMode, SortConfig, TooltipState, ProcessedMonitorData } from '../types';

// localStorage key for time align preference (shared with App.tsx)
const STORAGE_KEY_TIME_ALIGN = 'relay-pulse-time-align';

// 获取 ProviderPage 专用的快照 key（按 provider slug 隔离）
const getSnapshotKey = (slug: string) =>
  `relay-pulse:v1:list-state:p/${encodeURIComponent(slug)}`;

// ProviderPage 快照数据结构（简化版，无 provider/category）
interface ProviderPageSnapshot {
  version: 1;
  filterService: string[];
  filterChannel: string[];
}

// Provider 名称规范化（小写、去空格）
function canonicalize(value?: string): string {
  return value?.trim().toLowerCase() ?? '';
}

/**
 * 服务商专属页面
 * URL: /p/:provider
 * 支持嵌入模式: ?embed=1
 */
export default function ProviderPage() {
  const { provider } = useParams<{ provider: string }>();
  const [searchParams] = useSearchParams();
  const { t, i18n } = useTranslation();

  // 嵌入模式检测
  const isEmbedMode = searchParams.get('embed') === '1';

  // 规范化 provider slug
  const normalizedProvider = canonicalize(provider);

  // 状态管理
  const [timeRange, setTimeRange] = useState('90m');
  const [timeFilter, setTimeFilter] = useState<string | null>(null);
  const [filterService, setFilterService] = useState<string[]>([]);
  const [filterChannel, setFilterChannel] = useState<string[]>([]);
  // filterCategory 在 Provider 页面固定为空数组（全部），不需要状态
  const [viewMode, setViewMode] = useState<ViewMode>('table');
  const [sortConfig, setSortConfig] = useState<SortConfig>({
    key: 'uptime',
    direction: 'desc',
  });

  // 时间对齐模式（使用 localStorage 持久化）
  const [timeAlign, setTimeAlignState] = useState<string>(() => {
    if (typeof window === 'undefined') return '';
    return localStorage.getItem(STORAGE_KEY_TIME_ALIGN) || '';
  });

  const setTimeAlign = useCallback((align: string) => {
    setTimeAlignState(align);
    if (typeof window !== 'undefined') {
      if (align) {
        localStorage.setItem(STORAGE_KEY_TIME_ALIGN, align);
      } else {
        localStorage.removeItem(STORAGE_KEY_TIME_ALIGN);
      }
    }
  }, []);

  const [tooltip, setTooltip] = useState<TooltipState>({
    show: false,
    x: 0,
    y: 0,
    data: null,
  });

  // 刷新冷却状态（5秒内重复刷新显示提示）
  const REFRESH_COOLDOWN_MS = 5000;
  const lastRefreshRef = useRef<number>(0);
  const [refreshCooldown, setRefreshCooldown] = useState(false);

  // 移动端筛选抽屉状态
  const [showFilterDrawer, setShowFilterDrawer] = useState(false);

  // 收藏管理 Hook
  const { favorites, isFavorite, toggleFavorite, count: favoritesCount } = useFavorites();
  const [showFavoritesOnly, setShowFavoritesOnly] = useState(false);

  // 移动端检测（< 960px）
  const [isMobile, setIsMobile] = useState(false);
  useEffect(() => {
    const cleanup = createMediaQueryEffect('tablet', setIsMobile);
    return cleanup;
  }, []);

  // 数据获取 - 先获取全部数据用于构建映射
  // Provider 页面不启用置顶功能（isInitialSort=false）
  const { data: allData, loading, error, stats, slowLatencyMs, enableBadges, refetch } = useMonitorData({
    timeRange,
    timeAlign,
    timeFilter,
    filterService,
    filterProvider: [], // 空数组表示全部
    filterChannel,
    filterCategory: [], // Provider页面不筛选分类，空数组表示全部
    sortConfig,
    isInitialSort: false, // Provider页面禁用置顶
  });

  // 构建 slug -> providerId 映射
  const slugToProviderId = new Map<string, string>();
  allData.forEach((item) => {
    slugToProviderId.set(item.providerSlug, item.providerId);
  });

  // 将 URL slug 映射回 providerId
  const realProviderId = slugToProviderId.get(normalizedProvider) || normalizedProvider;

  // 按 providerId 过滤数据
  const data = allData.filter((item) => item.providerId === realProviderId);

  // 统计激活的筛选器数量（用于移动端 Header 显示）
  // Provider 页面不显示 category 和 provider 筛选器
  const activeFiltersCount = [
    showFavoritesOnly,
    filterService.length > 0,
    filterChannel.length > 0,
  ].filter(Boolean).length;

  // 基础数据：应用收藏筛选后的数据（如适用）
  const baseData = useMemo(() => {
    if (!showFavoritesOnly) return data;
    return data.filter(item => favorites.has(item.id));
  }, [data, showFavoritesOnly, favorites]);

  // 最终过滤后的数据（应用所有筛选器）
  const filteredData = useMemo(() => {
    let filtered = baseData;
    if (filterService.length > 0) {
      filtered = filtered.filter(item => filterService.includes(item.serviceType.toLowerCase()));
    }
    if (filterChannel.length > 0) {
      filtered = filtered.filter(item => item.channel && filterChannel.includes(item.channel));
    }
    return filtered;
  }, [baseData, filterService, filterChannel]);

  // 动态 Service 选项：基于 channel 筛选后的数据
  const effectiveServices = useMemo(() => {
    let filtered = baseData;
    if (filterChannel.length > 0) {
      filtered = filtered.filter(item => item.channel && filterChannel.includes(item.channel));
    }
    const set = new Set<string>();
    filtered.forEach(item => set.add(item.serviceType.toLowerCase()));
    return Array.from(set).sort();
  }, [baseData, filterChannel]);

  // 动态 Channel 选项：基于 service 筛选后的数据
  const effectiveChannels = useMemo(() => {
    let filtered = baseData;
    if (filterService.length > 0) {
      filtered = filtered.filter(item => filterService.includes(item.serviceType.toLowerCase()));
    }
    const set = new Set<string>();
    filtered.forEach(item => {
      if (item.channel) set.add(item.channel);
    });
    return Array.from(set).sort();
  }, [baseData, filterService]);

  // 收藏模式切换（ProviderPage 独立快照管理）
  const handleFavoritesModeChange = useCallback((enabled: boolean) => {
    if (enabled) {
      // 防止重复进入：已在收藏模式时不重复保存快照
      if (showFavoritesOnly) return;

      // 保存快照
      const snapshot: ProviderPageSnapshot = {
        version: 1,
        filterService,
        filterChannel,
      };
      try {
        sessionStorage.setItem(getSnapshotKey(normalizedProvider), JSON.stringify(snapshot));
      } catch {
        // sessionStorage 不可用时静默失败
      }
      // 清空筛选器并启用收藏模式
      setFilterService([]);
      setFilterChannel([]);
      setShowFavoritesOnly(true);
    } else {
      // 恢复快照
      let snapshot: ProviderPageSnapshot | null = null;
      try {
        const raw = sessionStorage.getItem(getSnapshotKey(normalizedProvider));
        if (raw) {
          const parsed = JSON.parse(raw);
          // 校验快照结构
          if (parsed?.version === 1 &&
              Array.isArray(parsed.filterService) &&
              Array.isArray(parsed.filterChannel)) {
            snapshot = parsed;
          }
        }
      } catch {
        // 解析失败时使用默认值
      }
      // 无论成功与否都清理快照
      try {
        sessionStorage.removeItem(getSnapshotKey(normalizedProvider));
      } catch {
        // 静默失败
      }

      // 恢复筛选器
      if (snapshot) {
        setFilterService(snapshot.filterService);
        setFilterChannel(snapshot.filterChannel);
      } else {
        // 无快照时恢复为默认
        setFilterService([]);
        setFilterChannel([]);
      }
      setShowFavoritesOnly(false);
    }
  }, [normalizedProvider, filterService, filterChannel, showFavoritesOnly]);

  // 移动端强制使用 table 视图
  const effectiveViewMode = isMobile ? 'table' : viewMode;

  // 软 404 处理：只在 provider slug 真正不存在时返回 404
  // 避免网络错误或筛选条件导致的空数据被误判为 404
  const providerExists = allData.some((item) => item.providerSlug === normalizedProvider);

  if (!loading && !error && !providerExists) {
    return <ProviderNotFound providerSlug={provider || ''} isEmbedMode={isEmbedMode} />;
  }

  // 从数据中获取 provider 显示名称
  const providerDisplayName = data[0]?.providerName || provider || '';

  // Tooltip 处理
  const handleBlockHover = (
    e: React.MouseEvent<HTMLDivElement>,
    point: ProcessedMonitorData['history'][number]
  ) => {
    const rect = e.currentTarget.getBoundingClientRect();
    setTooltip({
      show: true,
      x: rect.left + rect.width / 2,
      y: rect.top - 10,
      data: point,
    });
  };

  const handleBlockLeave = () => {
    setTooltip((prev) => ({ ...prev, show: false }));
  };

  // 排序处理
  const handleSort = (key: string) => {
    setSortConfig((prevConfig) => ({
      key,
      direction:
        prevConfig.key === key && prevConfig.direction === 'asc'
          ? 'desc'
          : 'asc',
    }));
  };

  // 刷新处理
  const handleRefresh = () => {
    const now = Date.now();
    const elapsed = now - lastRefreshRef.current;

    if (elapsed < REFRESH_COOLDOWN_MS) {
      // 冷却中，显示提示
      setRefreshCooldown(true);
      setTimeout(() => setRefreshCooldown(false), 2000); // 提示显示 2 秒
      return;
    }

    lastRefreshRef.current = now;
    refetch(true); // 绕过浏览器缓存
  };

  return (
    <>
      <Helmet>
        <html lang={i18n.language} />
        <title>{t('provider.pageTitle', { name: providerDisplayName })}</title>
        <meta name="description" content={t('provider.pageDescription', { name: providerDisplayName })} />
      </Helmet>

      <div className="min-h-screen bg-page text-primary font-sans selection-accent overflow-x-hidden">
        {/* 全局 Tooltip */}
        <Tooltip tooltip={tooltip} onClose={handleBlockLeave} slowLatencyMs={slowLatencyMs} timeRange={timeRange} />

        {/* 背景装饰 */}
        {!isEmbedMode && (
          <div className="fixed top-0 left-0 w-full h-full overflow-hidden pointer-events-none z-0">
            <div className="absolute top-[-10%] right-[-10%] w-[600px] h-[600px] bg-accent/10 rounded-full blur-[120px]" />
            <div className="absolute bottom-[-10%] left-[-10%] w-[600px] h-[600px] bg-accent/10 rounded-full blur-[120px]" />
          </div>
        )}

        <div className="relative z-10 max-w-7xl mx-auto px-4 py-4 sm:py-6 sm:px-6 lg:px-8">
        {/* 完整模式：显示 Header */}
        {!isEmbedMode && (
          <Header
            stats={stats}
            onFilterClick={() => setShowFilterDrawer(true)}
            onRefresh={handleRefresh}
            loading={loading}
            refreshCooldown={refreshCooldown}
            activeFiltersCount={activeFiltersCount}
          />
        )}

        {/* 控制面板 - 隐藏 provider 和 category 筛选器，只显示当前 provider 的通道 */}
        <Controls
          timeRange={timeRange}
          timeAlign={timeAlign}
          timeFilter={timeFilter}
          filterService={filterService}
          filterProvider={[]}
          filterChannel={filterChannel}
          filterCategory={[]}
          showFavoritesOnly={showFavoritesOnly}
          favorites={favorites}
          favoritesCount={favoritesCount}
          viewMode={viewMode}
          loading={loading}
          providers={[]} // 空数组 → 隐藏 provider 筛选器
          channels={effectiveChannels} // 收藏筛选时只显示收藏项中的通道
          effectiveServices={effectiveServices}
          effectiveCategories={[]}  // ProviderPage 不显示分类筛选
          showCategoryFilter={false} // 隐藏分类筛选器
          isMobile={isMobile}
          showFilterDrawer={showFilterDrawer}
          onFilterDrawerClose={() => setShowFilterDrawer(false)}
          onTimeRangeChange={setTimeRange}
          onTimeAlignChange={setTimeAlign}
          onTimeFilterChange={setTimeFilter}
          onServiceChange={setFilterService}
          onProviderChange={() => {}} // 无操作
          onChannelChange={setFilterChannel}
          onCategoryChange={() => {}} // 无操作
          onShowFavoritesOnlyChange={handleFavoritesModeChange}
          onViewModeChange={setViewMode}
          onRefresh={handleRefresh}
          refreshCooldown={refreshCooldown}
        />

        {/* 主内容区域 - 移除 py-6 以减小与控制面板的间距 */}
        <main>
          {error ? (
            <div className="flex flex-col items-center justify-center py-20 text-danger">
              <Server size={64} className="mb-4 opacity-20" />
              <p className="text-lg">{t('common.error', { message: error })}</p>
            </div>
          ) : loading && data.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-64 text-muted gap-4">
              <div className="w-12 h-12 border-4 border-accent/20 rounded-full animate-spin" style={{ borderTopColor: 'hsl(var(--accent))' }} />
              <p className="animate-pulse">{t('common.loading')}</p>
            </div>
          ) : data.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-20 text-muted">
              <Server size={64} className="mb-4 opacity-20" />
              <p className="text-lg">{t('common.noData')}</p>
            </div>
          ) : showFavoritesOnly && filteredData.length === 0 ? (
            <EmptyFavorites onClearFilter={() => handleFavoritesModeChange(false)} />
          ) : (
            <>
              {effectiveViewMode === 'table' && (
                <StatusTable
                  data={filteredData}
                  sortConfig={sortConfig}
                  timeRange={timeRange}
                  slowLatencyMs={slowLatencyMs}
                  enableBadges={enableBadges}
                  showCategoryTag={false}
                  showProvider={!isEmbedMode}
                  showSponsor={false}
                  isFavorite={isFavorite}
                  onToggleFavorite={toggleFavorite}
                  onSort={handleSort}
                  onBlockHover={handleBlockHover}
                  onBlockLeave={handleBlockLeave}
                />
              )}

              {effectiveViewMode === 'grid' && (
                <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-6">
                  {filteredData.map((item) => (
                    <StatusCard
                      key={item.id}
                      item={item}
                      timeRange={timeRange}
                      slowLatencyMs={slowLatencyMs}
                      enableBadges={enableBadges}
                      showCategoryTag={false}
                      showProvider={!isEmbedMode}
                      isFavorite={isFavorite}
                      onToggleFavorite={toggleFavorite}
                      onBlockHover={handleBlockHover}
                      onBlockLeave={handleBlockLeave}
                    />
                  ))}
                </div>
              )}
            </>
          )}
        </main>

        {/* 完整模式：显示 Footer */}
        {!isEmbedMode && <Footer />}
        </div>
      </div>
    </>
  );
}

/**
 * 404 页面组件 - 服务商未找到
 */
interface ProviderNotFoundProps {
  providerSlug: string;
  isEmbedMode: boolean;
}

function ProviderNotFound({ providerSlug, isEmbedMode }: ProviderNotFoundProps) {
  const { t } = useTranslation();

  return (
    <>
      <Helmet>
        <title>{t('provider.notFoundTitle')}</title>
        <meta name="robots" content="noindex, nofollow" />
      </Helmet>

      <div className={`min-h-screen flex items-center justify-center ${isEmbedMode ? '' : 'bg-page'}`}>
        <div className="text-center px-4">
          <h1 className="text-6xl font-bold text-primary mb-4">404</h1>
          <p className="text-xl text-muted mb-8">
            {t('provider.notFoundMessage', { slug: providerSlug })}
          </p>
          {!isEmbedMode && (
            <a
              href="/"
              className="inline-block px-6 py-3 bg-elevated hover:bg-muted/50 text-primary rounded-lg transition-colors"
            >
              {t('provider.backToHome')}
            </a>
          )}
        </div>
      </div>
    </>
  );
}
