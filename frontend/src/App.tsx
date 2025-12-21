import { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import { Server } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Helmet } from 'react-helmet-async';
import { Header } from './components/Header';
import { Controls } from './components/Controls';
import { StatusTable } from './components/StatusTable';
import { StatusCard } from './components/StatusCard';
import { Tooltip } from './components/Tooltip';
import { Footer } from './components/Footer';
import { EmptyFavorites } from './components/EmptyFavorites';
import { useMonitorData } from './hooks/useMonitorData';
import { useUrlState } from './hooks/useUrlState';
import { useFavorites } from './hooks/useFavorites';
import { createMediaQueryEffect } from './utils/mediaQuery';
import { trackPeriodChange, trackServiceFilter, trackEvent } from './utils/analytics';
import type { TooltipState, ProcessedMonitorData } from './types';

// localStorage key for time align preference
const STORAGE_KEY_TIME_ALIGN = 'relay-pulse-time-align';

function App() {
  const { t, i18n } = useTranslation();

  // 使用 URL 状态同步 Hook，支持收藏和分享
  const [urlState, urlActions] = useUrlState();
  const {
    timeRange,
    timeFilter,      // 每日时段过滤
    filterProvider,
    filterService,
    filterChannel,
    filterCategory,
    showFavoritesOnly,  // 仅显示收藏
    viewMode,
    sortConfig,
    isInitialSort,  // 是否为初始排序状态（用于赞助商置顶）
  } = urlState;

  // 移动端筛选抽屉状态（移到 App 层级，Header 和 Controls 共用）
  const [showFilterDrawer, setShowFilterDrawer] = useState(false);
  const {
    setTimeRange,
    setTimeFilter,   // 每日时段过滤
    setFilterProvider,
    setFilterService,
    setFilterChannel,
    setFilterCategory,
    setViewMode,
    setSortConfig,
    enterFavoritesMode,  // 进入收藏模式（保存快照）
    exitFavoritesMode,   // 退出收藏模式（恢复快照）
  } = urlActions;

  // 收藏管理 Hook
  const { favorites, isFavorite, toggleFavorite, count: favoritesCount } = useFavorites();

  // 时间对齐模式（使用 localStorage 持久化，不影响分享链接）
  const [timeAlign, setTimeAlignState] = useState<string>(() => {
    if (typeof window === 'undefined') return '';
    return localStorage.getItem(STORAGE_KEY_TIME_ALIGN) || '';
  });

  // 包装 setter 以同步到 localStorage
  const setTimeAlign = useCallback((align: string) => {
    setTimeAlignState(align);
    if (typeof window !== 'undefined') {
      if (align) {
        localStorage.setItem(STORAGE_KEY_TIME_ALIGN, align);
      } else {
        localStorage.removeItem(STORAGE_KEY_TIME_ALIGN);
      }
    }
    // 追踪时间对齐模式变化
    trackEvent('change_time_align', { align: align || 'dynamic' });
  }, []);

  // 移动端检测（< 960px）
  const [isMobile, setIsMobile] = useState(false);
  useEffect(() => {
    const cleanup = createMediaQueryEffect('tablet', setIsMobile);
    return cleanup;
  }, []);

  // 移动端强制使用 table 视图
  const effectiveViewMode = isMobile ? 'table' : viewMode;

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

  const { loading, error, data, stats, providers, slowLatencyMs, enableBadges, refetch } = useMonitorData({
    timeRange,
    timeAlign,
    timeFilter,
    filterService,
    filterProvider,
    filterChannel,
    filterCategory,
    sortConfig,
    isInitialSort,
  });

  // 统计激活的筛选器数量（用于移动端 Header 显示）
  const activeFiltersCount = [
    showFavoritesOnly,
    filterCategory.length > 0,
    providers.length > 0 && filterProvider.length > 0,
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
    if (filterProvider.length > 0) {
      filtered = filtered.filter(item => filterProvider.includes(item.providerId));
    }
    if (filterService.length > 0) {
      filtered = filtered.filter(item => filterService.includes(item.serviceType.toLowerCase()));
    }
    if (filterChannel.length > 0) {
      filtered = filtered.filter(item => item.channel && filterChannel.includes(item.channel));
    }
    if (filterCategory.length > 0) {
      filtered = filtered.filter(item => filterCategory.includes(item.category));
    }
    return filtered;
  }, [baseData, filterProvider, filterService, filterChannel, filterCategory]);

  // 动态 Provider 选项：基于 service + channel + category 筛选后的数据
  // 不包含 provider 筛选，避免选项自我消失
  const effectiveProviders = useMemo(() => {
    let filtered = baseData;
    if (filterService.length > 0) {
      filtered = filtered.filter(item => filterService.includes(item.serviceType.toLowerCase()));
    }
    if (filterChannel.length > 0) {
      filtered = filtered.filter(item => item.channel && filterChannel.includes(item.channel));
    }
    if (filterCategory.length > 0) {
      filtered = filtered.filter(item => filterCategory.includes(item.category));
    }
    const map = new Map<string, string>();
    filtered.forEach(item => {
      if (!map.has(item.providerId)) {
        map.set(item.providerId, item.providerName);
      }
    });
    return Array.from(map.entries())
      .sort((a, b) => a[1].localeCompare(b[1], 'zh-CN'))
      .map(([value, label]) => ({ value, label }));
  }, [baseData, filterService, filterChannel, filterCategory]);

  // 动态 Service 选项：基于 provider + channel + category 筛选后的数据
  const effectiveServices = useMemo(() => {
    let filtered = baseData;
    if (filterProvider.length > 0) {
      filtered = filtered.filter(item => filterProvider.includes(item.providerId));
    }
    if (filterChannel.length > 0) {
      filtered = filtered.filter(item => item.channel && filterChannel.includes(item.channel));
    }
    if (filterCategory.length > 0) {
      filtered = filtered.filter(item => filterCategory.includes(item.category));
    }
    const set = new Set<string>();
    filtered.forEach(item => set.add(item.serviceType.toLowerCase()));
    return Array.from(set).sort();
  }, [baseData, filterProvider, filterChannel, filterCategory]);

  // 动态 Channel 选项：基于 provider + service + category 筛选后的数据
  const effectiveChannels = useMemo(() => {
    let filtered = baseData;
    if (filterProvider.length > 0) {
      filtered = filtered.filter(item => filterProvider.includes(item.providerId));
    }
    if (filterService.length > 0) {
      filtered = filtered.filter(item => filterService.includes(item.serviceType.toLowerCase()));
    }
    if (filterCategory.length > 0) {
      filtered = filtered.filter(item => filterCategory.includes(item.category));
    }
    const set = new Set<string>();
    filtered.forEach(item => {
      if (item.channel) set.add(item.channel);
    });
    return Array.from(set).sort();
  }, [baseData, filterProvider, filterService, filterCategory]);

  // 动态 Category 选项：基于 provider + service + channel 筛选后的数据
  const effectiveCategories = useMemo(() => {
    let filtered = baseData;
    if (filterProvider.length > 0) {
      filtered = filtered.filter(item => filterProvider.includes(item.providerId));
    }
    if (filterService.length > 0) {
      filtered = filtered.filter(item => filterService.includes(item.serviceType.toLowerCase()));
    }
    if (filterChannel.length > 0) {
      filtered = filtered.filter(item => item.channel && filterChannel.includes(item.channel));
    }
    const set = new Set<string>();
    filtered.forEach(item => set.add(item.category));
    return Array.from(set).sort();
  }, [baseData, filterProvider, filterService, filterChannel]);

  // 收藏模式切换（使用事务性方法，保存/恢复筛选状态快照）
  const handleFavoritesModeChange = useCallback((enabled: boolean) => {
    if (enabled) {
      enterFavoritesMode();
    } else {
      exitFavoritesMode();
    }
  }, [enterFavoritesMode, exitFavoritesMode]);

  // 追踪时间范围变化
  useEffect(() => {
    trackPeriodChange(timeRange);
  }, [timeRange]);

  // 追踪服务筛选变化
  useEffect(() => {
    trackServiceFilter(
      filterProvider.length > 0 ? filterProvider.join(',') : undefined,
      filterService.length > 0 ? filterService.join(',') : undefined
    );
  }, [filterProvider, filterService]);

  // 追踪通道筛选变化
  useEffect(() => {
    if (filterChannel.length > 0) {
      trackEvent('filter_channel', { channel: filterChannel.join(',') });
    }
  }, [filterChannel]);

  // 追踪分类筛选变化
  useEffect(() => {
    if (filterCategory.length > 0) {
      trackEvent('filter_category', { category: filterCategory.join(',') });
    }
  }, [filterCategory]);

  // 追踪视图模式切换（使用实际显示的视图模式）
  useEffect(() => {
    trackEvent('change_view_mode', { mode: effectiveViewMode });
  }, [effectiveViewMode]);

  const handleSort = (key: string) => {
    let direction: 'asc' | 'desc' = 'desc';
    // 初始状态（置顶模式）下，首次点击任何排序都使用降序
    // 非初始状态下，点击同一字段切换升降序
    if (!isInitialSort && sortConfig.key === key && sortConfig.direction === 'desc') {
      direction = 'asc';
    }
    setSortConfig({ key, direction });
  };

  const handleBlockHover = useCallback((
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
  }, []);

  const handleBlockLeave = useCallback(() => {
    setTooltip((prev) => ({ ...prev, show: false }));
  }, []);

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
    trackEvent('manual_refresh');
    refetch(true); // 绕过浏览器缓存
  };

  return (
    <>
      {/* 动态更新 HTML meta 标签 */}
      <Helmet>
        <html lang={i18n.language} />
        <title>{t('meta.title')}</title>
        <meta name="description" content={t('meta.description')} />
      </Helmet>

      <div className="min-h-screen bg-page text-primary font-sans selection-accent">
        {/* 全局 Tooltip */}
        <Tooltip tooltip={tooltip} onClose={handleBlockLeave} slowLatencyMs={slowLatencyMs} timeRange={timeRange} />

        {/* 背景装饰 */}
        <div className="fixed top-0 left-0 w-full h-full overflow-hidden pointer-events-none z-0">
          <div className="absolute top-[-10%] right-[-10%] w-[600px] h-[600px] bg-accent/10 rounded-full blur-[120px]" />
          <div className="absolute bottom-[-10%] left-[-10%] w-[600px] h-[600px] bg-accent/10 rounded-full blur-[120px]" />
        </div>

        <div className="relative z-10 max-w-7xl mx-auto px-4 py-4 sm:py-6 sm:px-6 lg:px-8">
          {/* 头部 */}
          <Header
            stats={stats}
            onFilterClick={() => setShowFilterDrawer(true)}
            onRefresh={handleRefresh}
            loading={loading}
            refreshCooldown={refreshCooldown}
            activeFiltersCount={activeFiltersCount}
          />

          {/* 控制栏 */}
          <Controls
            filterProvider={filterProvider}
            filterService={filterService}
            filterChannel={filterChannel}
            filterCategory={filterCategory}
            showFavoritesOnly={showFavoritesOnly}
            favoritesCount={favoritesCount}
            timeRange={timeRange}
            timeAlign={timeAlign}
            timeFilter={timeFilter}
            viewMode={viewMode}
            loading={loading}
            channels={effectiveChannels}
            providers={effectiveProviders}
            effectiveServices={effectiveServices}
            effectiveCategories={effectiveCategories}
            isMobile={isMobile}
            showFilterDrawer={showFilterDrawer}
            onFilterDrawerClose={() => setShowFilterDrawer(false)}
            onProviderChange={setFilterProvider}
            onServiceChange={setFilterService}
            onChannelChange={setFilterChannel}
            onCategoryChange={setFilterCategory}
            onShowFavoritesOnlyChange={handleFavoritesModeChange}
            onTimeRangeChange={setTimeRange}
            onTimeAlignChange={setTimeAlign}
            onTimeFilterChange={setTimeFilter}
            onViewModeChange={setViewMode}
            onRefresh={handleRefresh}
            refreshCooldown={refreshCooldown}
          />

          {/* 内容区域 */}
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
            // 开启收藏筛选但无收藏时显示空状态
            <EmptyFavorites onClearFilter={exitFavoritesMode} />
          ) : (
            <>
              {effectiveViewMode === 'table' && (
                <StatusTable
                  data={filteredData}
                  sortConfig={sortConfig}
                  isInitialSort={isInitialSort}
                  timeRange={timeRange}
                  slowLatencyMs={slowLatencyMs}
                  enableBadges={enableBadges}
                  isFavorite={isFavorite}
                  onToggleFavorite={toggleFavorite}
                  onSort={handleSort}
                  onBlockHover={handleBlockHover}
                  onBlockLeave={handleBlockLeave}
                  onFilterProvider={(providerId) => setFilterProvider([providerId])}
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

          {/* 免责声明 */}
          <Footer />
        </div>
      </div>
    </>
  );
}

export default App;
