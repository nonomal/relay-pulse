import { useState } from 'react';
import { Filter, RefreshCw, LayoutGrid, List, X, Clock, AlignStartVertical } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { getTimeRanges } from '../constants';
import type { ViewMode, ProviderOption } from '../types';

interface ControlsProps {
  filterProvider: string;
  filterService: string;
  filterChannel: string;
  filterCategory: string;
  timeRange: string;
  timeAlign: string; // 时间对齐模式：空=动态窗口, "hour"=整点对齐
  viewMode: ViewMode;
  loading: boolean;
  channels: string[];
  providers: ProviderOption[];  // 改为 ProviderOption[]
  showCategoryFilter?: boolean; // 是否显示分类筛选器，默认 true（用于服务商专属页面）
  refreshCooldown?: boolean; // 刷新冷却中，显示提示
  onProviderChange: (provider: string) => void;
  onServiceChange: (service: string) => void;
  onChannelChange: (channel: string) => void;
  onCategoryChange: (category: string) => void;
  onTimeRangeChange: (range: string) => void;
  onTimeAlignChange: (align: string) => void; // 切换时间对齐模式
  onViewModeChange: (mode: ViewMode) => void;
  onRefresh: () => void;
}

export function Controls({
  filterProvider,
  filterService,
  filterChannel,
  filterCategory,
  timeRange,
  timeAlign,
  viewMode,
  loading,
  channels,
  providers,
  showCategoryFilter = true,
  refreshCooldown = false,
  onProviderChange,
  onServiceChange,
  onChannelChange,
  onCategoryChange,
  onTimeRangeChange,
  onTimeAlignChange,
  onViewModeChange,
  onRefresh,
}: ControlsProps) {
  const { t } = useTranslation();
  const [showFilterDrawer, setShowFilterDrawer] = useState(false);

  // 统计激活的筛选器数量（仅计入可见的筛选器）
  const activeFiltersCount = [
    showCategoryFilter && filterCategory !== 'all',
    providers.length > 0 && filterProvider !== 'all',
    filterService !== 'all',
    filterChannel !== 'all',
  ].filter(Boolean).length;

  // 筛选器组件（桌面和移动端共用）
  const FilterSelects = () => (
    <>
      {/* Category 筛选器 - 可通过 showCategoryFilter 控制显示 */}
      {showCategoryFilter && (
        <select
          id="filter-category"
          name="filter-category"
          value={filterCategory}
          onChange={(e) => onCategoryChange(e.target.value)}
          className="bg-slate-800 text-slate-200 text-sm rounded-lg border border-slate-700 focus:ring-2 focus:ring-cyan-500 focus:border-transparent p-2 outline-none transition-all hover:bg-slate-750 w-full sm:w-auto"
        >
          <option value="all">{t('controls.filters.category')}</option>
          <option value="public">{t('controls.categories.charity')}</option>
          <option value="commercial">{t('controls.categories.promoted')}</option>
        </select>
      )}

      {/* Provider 筛选器 - 当 providers 为空时隐藏（用于服务商专属页面） */}
      {providers.length > 0 && (
        <select
          id="filter-provider"
          name="filter-provider"
          value={filterProvider}
          onChange={(e) => onProviderChange(e.target.value)}
          className="bg-slate-800 text-slate-200 text-sm rounded-lg border border-slate-700 focus:ring-2 focus:ring-cyan-500 focus:border-transparent p-2 outline-none transition-all hover:bg-slate-750 w-full sm:w-auto"
        >
          <option value="all">{t('controls.filters.provider')}</option>
          {providers.map(({ value, label }) => (
            <option key={value} value={value}>
              {label}
            </option>
          ))}
        </select>
      )}

      <select
        id="filter-service"
        name="filter-service"
        value={filterService}
        onChange={(e) => onServiceChange(e.target.value)}
        className="bg-slate-800 text-slate-200 text-sm rounded-lg border border-slate-700 focus:ring-2 focus:ring-cyan-500 focus:border-transparent p-2 outline-none transition-all hover:bg-slate-750 w-full sm:w-auto"
      >
        <option value="all">{t('controls.filters.service')}</option>
        <option value="cc">{t('controls.services.cc')}</option>
        <option value="cx">{t('controls.services.cx')}</option>
      </select>

      <select
        id="filter-channel"
        name="filter-channel"
        value={filterChannel}
        onChange={(e) => onChannelChange(e.target.value)}
        className="bg-slate-800 text-slate-200 text-sm rounded-lg border border-slate-700 focus:ring-2 focus:ring-cyan-500 focus:border-transparent p-2 outline-none transition-all hover:bg-slate-750 w-full sm:w-auto"
      >
        <option value="all">{t('controls.filters.channel')}</option>
        {channels.map((channel) => (
          <option key={channel} value={channel}>
            {channel}
          </option>
        ))}
      </select>
    </>
  );

  return (
    <>
      <div className="flex flex-col sm:flex-row gap-3 mb-4">
        {/* 筛选和视图控制 */}
        <div className="flex-1 flex flex-wrap gap-3 items-center bg-slate-900/40 p-3 rounded-2xl border border-slate-800/50 backdrop-blur-md">
          {/* 移动端：筛选按钮 */}
          <button
            onClick={() => setShowFilterDrawer(true)}
            className="sm:hidden flex items-center gap-2 px-3 py-2 bg-slate-800 text-slate-200 rounded-lg border border-slate-700 hover:bg-slate-750 transition-colors"
          >
            <Filter size={16} />
            <span className="text-sm font-medium">{t('controls.mobile.filterBtn')}</span>
            {activeFiltersCount > 0 && (
              <span className="px-1.5 py-0.5 bg-cyan-500 text-white text-xs rounded-full">
                {activeFiltersCount}
              </span>
            )}
          </button>

          {/* 桌面端：直接显示筛选器 */}
          <div className="hidden sm:flex items-center gap-2 text-slate-400 text-sm font-medium px-2">
            <Filter size={16} />
          </div>
          <div className="hidden sm:flex sm:flex-wrap gap-3 flex-1">
            <FilterSelects />
          </div>

          <div className="w-px h-8 bg-slate-700 mx-2 hidden sm:block"></div>

          {/* 视图切换（扩大触摸区域） */}
          <div className="flex bg-slate-800 rounded-lg p-1 border border-slate-700">
            <button
              onClick={() => onViewModeChange('table')}
              className={`p-2.5 rounded min-w-[44px] min-h-[44px] flex items-center justify-center ${
                viewMode === 'table'
                  ? 'bg-slate-700 text-cyan-400 shadow'
                  : 'text-slate-400 hover:text-slate-200'
              }`}
              title={t('controls.views.table')}
              aria-label={t('controls.views.switchToTable')}
            >
              <List size={18} />
            </button>
            <button
              onClick={() => onViewModeChange('grid')}
              className={`p-2.5 rounded min-w-[44px] min-h-[44px] flex items-center justify-center ${
                viewMode === 'grid'
                  ? 'bg-slate-700 text-cyan-400 shadow'
                  : 'text-slate-400 hover:text-slate-200'
              }`}
              title={t('controls.views.card')}
              aria-label={t('controls.views.switchToCard')}
            >
              <LayoutGrid size={18} />
            </button>
          </div>

          {/* 刷新按钮（扩大触摸区域） */}
          <div className="relative ml-auto">
            <button
              onClick={onRefresh}
              className="p-2.5 rounded-lg bg-cyan-500/10 text-cyan-400 hover:bg-cyan-500/20 transition-colors border border-cyan-500/20 group min-w-[44px] min-h-[44px] flex items-center justify-center cursor-pointer"
              title={t('common.refresh')}
              aria-label={t('common.refresh')}
            >
              <RefreshCw
                size={18}
                className={`transition-transform ${loading ? 'animate-spin' : 'group-hover:rotate-180'}`}
              />
            </button>
            {/* 冷却提示 */}
            {refreshCooldown && (
              <div className="absolute top-full left-1/2 -translate-x-1/2 mt-2 px-3 py-1.5 bg-slate-800 text-slate-300 text-xs rounded-lg whitespace-nowrap shadow-lg border border-slate-700 z-50">
                {t('common.refreshCooldown')}
              </div>
            )}
          </div>
        </div>

        {/* 时间范围选择（添加横向滚动） */}
        <div className="bg-slate-900/40 p-2 rounded-2xl border border-slate-800/50 backdrop-blur-md flex items-center gap-1 overflow-x-auto scrollbar-thin scrollbar-thumb-slate-700 scrollbar-track-transparent">
          {getTimeRanges(t).map((range) => (
            <button
              key={range.id}
              onClick={() => onTimeRangeChange(range.id)}
              className={`px-3 py-2 text-xs font-medium rounded-xl transition-all duration-200 whitespace-nowrap flex-shrink-0 ${
                timeRange === range.id
                  ? 'bg-gradient-to-br from-cyan-500 to-blue-600 text-white shadow-lg shadow-cyan-500/25'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800'
              }`}
            >
              {range.label}
            </button>
          ))}

          {/* 分隔线 */}
          <div className="w-px h-6 bg-slate-700 mx-1 flex-shrink-0" />

          {/* 时间对齐切换（仅在 24h 模式下显示） */}
          {timeRange === '24h' && (
            <div className="flex bg-slate-800 rounded-lg p-0.5 border border-slate-700 flex-shrink-0">
              <button
                onClick={() => onTimeAlignChange('')}
                className={`p-1.5 rounded flex items-center gap-1 text-xs transition-all ${
                  timeAlign === ''
                    ? 'bg-slate-700 text-cyan-400 shadow'
                    : 'text-slate-400 hover:text-slate-200'
                }`}
                title={t('controls.timeAlign.dynamicTitle')}
                aria-label={t('controls.timeAlign.dynamic')}
              >
                <Clock size={14} />
                <span className="hidden sm:inline">{t('controls.timeAlign.dynamic')}</span>
              </button>
              <button
                onClick={() => onTimeAlignChange('hour')}
                className={`p-1.5 rounded flex items-center gap-1 text-xs transition-all ${
                  timeAlign === 'hour'
                    ? 'bg-slate-700 text-cyan-400 shadow'
                    : 'text-slate-400 hover:text-slate-200'
                }`}
                title={t('controls.timeAlign.hourTitle')}
                aria-label={t('controls.timeAlign.hour')}
              >
                <AlignStartVertical size={14} />
                <span className="hidden sm:inline">{t('controls.timeAlign.hour')}</span>
              </button>
            </div>
          )}
        </div>
      </div>

      {/* 移动端筛选抽屉 */}
      {showFilterDrawer && (
        <div
          className="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm sm:hidden"
          onClick={() => setShowFilterDrawer(false)}
        >
          <div
            className="absolute bottom-0 left-0 right-0 bg-slate-900 border-t border-slate-800 rounded-t-2xl p-6 max-h-[80vh] overflow-y-auto"
            onClick={(e) => e.stopPropagation()}
          >
            {/* 抽屉头部 */}
            <div className="flex justify-between items-center mb-6">
              <div className="flex items-center gap-2">
                <Filter size={20} className="text-cyan-400" />
                <h3 className="text-lg font-semibold text-slate-100">{t('controls.mobile.filterTitle')}</h3>
                {activeFiltersCount > 0 && (
                  <span className="px-2 py-0.5 bg-cyan-500 text-white text-xs rounded-full">
                    {activeFiltersCount}
                  </span>
                )}
              </div>
              <button
                onClick={() => setShowFilterDrawer(false)}
                className="p-2 rounded-lg bg-slate-800 text-slate-400 hover:text-slate-200 transition-colors"
                aria-label={t('controls.mobile.closeFilter')}
              >
                <X size={20} />
              </button>
            </div>

            {/* 筛选器列表 */}
            <div className="flex flex-col gap-4">
              <div>
                <label className="block text-sm font-medium text-slate-400 mb-2">
                  {t('controls.filters.categoryLabel')}
                </label>
                <FilterSelects />
              </div>

              {/* 清空按钮 - 只清空可见的筛选器 */}
              {activeFiltersCount > 0 && (
                <button
                  onClick={() => {
                    if (showCategoryFilter) onCategoryChange('all');
                    if (providers.length > 0) onProviderChange('all');
                    onServiceChange('all');
                    onChannelChange('all');
                  }}
                  className="w-full py-3 bg-slate-800 text-slate-300 rounded-lg hover:bg-slate-750 transition-colors font-medium"
                >
                  {t('common.clear')}
                </button>
              )}

              {/* 应用按钮 */}
              <button
                onClick={() => setShowFilterDrawer(false)}
                className="w-full py-3 bg-gradient-to-br from-cyan-500 to-blue-600 text-white rounded-lg font-medium shadow-lg shadow-cyan-500/25 hover:shadow-cyan-500/40 transition-all"
              >
                {t('common.apply')}
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
