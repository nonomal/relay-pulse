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
    filterProvider,
    filterService,
    filterChannel,
    filterCategory,
    viewMode,
    sortConfig,
  } = urlState;
  const {
    setTimeRange,
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

  const [tooltip, setTooltip] = useState<TooltipState>({
    show: false,
    x: 0,
    y: 0,
    data: null,
  });

  const { loading, error, data, stats, channels, providers, slowLatencyMs, refetch } = useMonitorData({
    timeRange,
    timeAlign,
    filterService,
    filterProvider,
    filterChannel,
    filterCategory,
    sortConfig,
  });

  // 追踪时间范围变化
  useEffect(() => {
    trackPeriodChange(timeRange);
  }, [timeRange]);

  // 追踪服务筛选变化
  useEffect(() => {
    trackServiceFilter(
      filterProvider !== 'all' ? filterProvider : undefined,
      filterService !== 'all' ? filterService : undefined
    );
  }, [filterProvider, filterService]);

  // 追踪通道筛选变化
  useEffect(() => {
    if (filterChannel !== 'all') {
      trackEvent('filter_channel', { channel: filterChannel });
    }
  }, [filterChannel]);

  // 追踪分类筛选变化
  useEffect(() => {
    if (filterCategory !== 'all') {
      trackEvent('filter_category', { category: filterCategory });
    }
  }, [filterCategory]);

  // 追踪视图模式切换
  useEffect(() => {
    trackEvent('change_view_mode', { mode: viewMode });
  }, [viewMode]);

  const handleSort = (key: string) => {
    let direction: 'asc' | 'desc' = 'desc';
    if (sortConfig.key === key && sortConfig.direction === 'desc') {
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

  // 刷新冷却状态（5秒内重复刷新显示提示）
  const REFRESH_COOLDOWN_MS = 5000;
  const lastRefreshRef = useRef<number>(0);
  const [refreshCooldown, setRefreshCooldown] = useState(false);

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

      <div className="min-h-screen bg-slate-950 text-slate-200 font-sans selection:bg-cyan-500 selection:text-white overflow-x-hidden">
        {/* 全局 Tooltip */}
        <Tooltip tooltip={tooltip} onClose={handleBlockLeave} slowLatencyMs={slowLatencyMs} timeRange={timeRange} />

        {/* 背景装饰 */}
        <div className="fixed top-0 left-0 w-full h-full overflow-hidden pointer-events-none z-0">
          <div className="absolute top-[-10%] right-[-10%] w-[600px] h-[600px] bg-blue-600/10 rounded-full blur-[120px]" />
          <div className="absolute bottom-[-10%] left-[-10%] w-[600px] h-[600px] bg-cyan-600/10 rounded-full blur-[120px]" />
        </div>

        <div className="relative z-10 max-w-7xl mx-auto px-4 py-4 sm:py-6 sm:px-6 lg:px-8">
          {/* 头部 */}
          <Header stats={stats} />

          {/* 控制栏 */}
          <Controls
            filterProvider={filterProvider}
            filterService={filterService}
            filterChannel={filterChannel}
            filterCategory={filterCategory}
            timeRange={timeRange}
            timeAlign={timeAlign}
            viewMode={viewMode}
            loading={loading}
            channels={channels}
            providers={providers}
            onProviderChange={setFilterProvider}
            onServiceChange={setFilterService}
            onChannelChange={setFilterChannel}
            onCategoryChange={setFilterCategory}
            onTimeRangeChange={setTimeRange}
            onTimeAlignChange={setTimeAlign}
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
              {viewMode === 'table' && (
                <StatusTable
                  data={data}
                  sortConfig={sortConfig}
                  timeRange={timeRange}
                  slowLatencyMs={slowLatencyMs}
                  onSort={handleSort}
                  onBlockHover={handleBlockHover}
                  onBlockLeave={handleBlockLeave}
                />
              )}

              {viewMode === 'grid' && (
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
