import { useState, useEffect, useCallback, useRef } from 'react';
import { Server } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Helmet } from 'react-helmet-async';
import { Header } from './components/Header';
import { Controls } from './components/Controls';
import { StatusTable } from './components/StatusTable';
import { StatusCard } from './components/StatusCard';
import { Tooltip } from './components/Tooltip';
import { Footer } from './components/Footer';
import { useMonitorData } from './hooks/useMonitorData';
import { useUrlState } from './hooks/useUrlState';
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
  } = urlActions;

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

  const { loading, error, data, stats, channels, providers, slowLatencyMs, refetch } = useMonitorData({
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
    filterCategory.length > 0,
    providers.length > 0 && filterProvider.length > 0,
    filterService.length > 0,
    filterChannel.length > 0,
  ].filter(Boolean).length;

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

      <div className="min-h-screen bg-slate-950 text-slate-200 font-sans selection:bg-cyan-500 selection:text-white">
        {/* 全局 Tooltip */}
        <Tooltip tooltip={tooltip} onClose={handleBlockLeave} slowLatencyMs={slowLatencyMs} timeRange={timeRange} />

        {/* 背景装饰 */}
        <div className="fixed top-0 left-0 w-full h-full overflow-hidden pointer-events-none z-0">
          <div className="absolute top-[-10%] right-[-10%] w-[600px] h-[600px] bg-blue-600/10 rounded-full blur-[120px]" />
          <div className="absolute bottom-[-10%] left-[-10%] w-[600px] h-[600px] bg-cyan-600/10 rounded-full blur-[120px]" />
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
            timeRange={timeRange}
            timeAlign={timeAlign}
            timeFilter={timeFilter}
            viewMode={viewMode}
            loading={loading}
            channels={channels}
            providers={providers}
            isMobile={isMobile}
            showFilterDrawer={showFilterDrawer}
            onFilterDrawerClose={() => setShowFilterDrawer(false)}
            onProviderChange={setFilterProvider}
            onServiceChange={setFilterService}
            onChannelChange={setFilterChannel}
            onCategoryChange={setFilterCategory}
            onTimeRangeChange={setTimeRange}
            onTimeAlignChange={setTimeAlign}
            onTimeFilterChange={setTimeFilter}
            onViewModeChange={setViewMode}
            onRefresh={handleRefresh}
            refreshCooldown={refreshCooldown}
          />

          {/* 内容区域 */}
          {error ? (
            <div className="flex flex-col items-center justify-center py-20 text-rose-400">
              <Server size={64} className="mb-4 opacity-20" />
              <p className="text-lg">{t('common.error', { message: error })}</p>
            </div>
          ) : loading && data.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-64 text-slate-500 gap-4">
              <div className="w-12 h-12 border-4 border-cyan-500/20 border-t-cyan-500 rounded-full animate-spin" />
              <p className="animate-pulse">{t('common.loading')}</p>
            </div>
          ) : data.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-20 text-slate-600">
              <Server size={64} className="mb-4 opacity-20" />
              <p className="text-lg">{t('common.noData')}</p>
            </div>
          ) : (
            <>
              {effectiveViewMode === 'table' && (
                <StatusTable
                  data={data}
                  sortConfig={sortConfig}
                  isInitialSort={isInitialSort}
                  timeRange={timeRange}
                  slowLatencyMs={slowLatencyMs}
                  onSort={handleSort}
                  onBlockHover={handleBlockHover}
                  onBlockLeave={handleBlockLeave}
                />
              )}

              {effectiveViewMode === 'grid' && (
                <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-6">
                  {data.map((item) => (
                    <StatusCard
                      key={item.id}
                      item={item}
                      timeRange={timeRange}
                      slowLatencyMs={slowLatencyMs}
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
