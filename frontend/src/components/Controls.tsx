import { useMemo } from 'react';
import { Filter, RefreshCw, LayoutGrid, List, X, Clock, AlignStartVertical } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { getTimeRanges } from '../constants';
import { MultiSelect } from './MultiSelect';
import { TimeFilterPicker } from './TimeFilterPicker';
import type { MultiSelectOption } from './MultiSelect';
import type { ViewMode, ProviderOption } from '../types';

interface ControlsProps {
  filterProvider: string[];  // 多选服务商，空数组表示"全部"
  filterService: string[];   // 多选服务，空数组表示"全部"
  filterChannel: string[];   // 多选通道，空数组表示"全部"
  filterCategory: string[];  // 多选分类，空数组表示"全部"
  timeRange: string;
  timeAlign: string;         // 时间对齐模式：空=动态窗口, "hour"=整点对齐
  timeFilter: string | null; // 每日时段过滤：null=全天, "09:00-17:00"=自定义
  viewMode: ViewMode;
  loading: boolean;
  channels: string[];
  providers: ProviderOption[];  // 改为 ProviderOption[]
  showCategoryFilter?: boolean; // 是否显示分类筛选器，默认 true（用于服务商专属页面）
  refreshCooldown?: boolean; // 刷新冷却中，显示提示
  isMobile?: boolean; // 是否为移动端，用于隐藏视图切换按钮
  showFilterDrawer?: boolean; // 移动端筛选抽屉是否显示（由 App 层级控制）
  onFilterDrawerClose?: () => void; // 关闭筛选抽屉回调
  onProviderChange: (providers: string[]) => void;  // 多选回调
  onServiceChange: (services: string[]) => void;    // 多选回调
  onChannelChange: (channels: string[]) => void;    // 多选回调
  onCategoryChange: (categories: string[]) => void; // 多选回调
  onTimeRangeChange: (range: string) => void;
  onTimeAlignChange: (align: string) => void;       // 切换时间对齐模式
  onTimeFilterChange: (filter: string | null) => void; // 切换每日时段过滤
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
  timeFilter,
  viewMode,
  loading,
  channels,
  providers,
  showCategoryFilter = true,
  refreshCooldown = false,
  isMobile = false,
  showFilterDrawer = false,
  onFilterDrawerClose,
  onProviderChange,
  onServiceChange,
  onChannelChange,
  onCategoryChange,
  onTimeRangeChange,
  onTimeAlignChange,
  onTimeFilterChange,
  onViewModeChange,
  onRefresh,
}: ControlsProps) {
  const { t } = useTranslation();

  // 服务选项（固定值）
  const serviceOptions = useMemo<MultiSelectOption[]>(() => [
    { value: 'cc', label: t('controls.services.cc') },
    { value: 'cx', label: t('controls.services.cx') },
  ], [t]);

  // 分类选项（固定值）
  const categoryOptions = useMemo<MultiSelectOption[]>(() => [
    { value: 'public', label: t('controls.categories.charity') },
    { value: 'commercial', label: t('controls.categories.promoted') },
  ], [t]);

  // 通道选项（动态值）
  const channelOptions = useMemo<MultiSelectOption[]>(() =>
    channels.map(channel => ({ value: channel, label: channel })),
  [channels]);

  // 统计激活的筛选器数量（仅计入可见的筛选器）
  const activeFiltersCount = [
    showCategoryFilter && filterCategory.length > 0,
    providers.length > 0 && filterProvider.length > 0,
    filterService.length > 0,
    filterChannel.length > 0,
  ].filter(Boolean).length;

  // 筛选器组件（桌面和移动端共用）
  const FilterSelects = () => (
    <>
      {/* Category 筛选器 - 可通过 showCategoryFilter 控制显示 */}
      {showCategoryFilter && (
        <MultiSelect
          value={filterCategory}
          options={categoryOptions}
          onChange={onCategoryChange}
          placeholder={t('controls.filters.category')}
          searchable={false}
        />
      )}

      {/* Provider 筛选器 - 当 providers 为空时隐藏（用于服务商专属页面） */}
      {providers.length > 0 && (
        <MultiSelect
          value={filterProvider}
          options={providers}
          onChange={onProviderChange}
          placeholder={t('controls.filters.provider')}
          searchable
        />
      )}

      {/* Service 筛选器 */}
      <MultiSelect
        value={filterService}
        options={serviceOptions}
        onChange={onServiceChange}
        placeholder={t('controls.filters.service')}
        searchable={false}
      />

      {/* Channel 筛选器 */}
      <MultiSelect
        value={filterChannel}
        options={channelOptions}
        onChange={onChannelChange}
        placeholder={t('controls.filters.channel')}
        searchable={channels.length > 5}
      />
    </>
  );

  return (
    <>
      <div className="flex flex-col lg:flex-row gap-2 lg:gap-3 mb-2 lg:mb-4 overflow-visible">
        {/* 筛选和视图控制（移动端隐藏，筛选/刷新已移到 Header） */}
        <div className="hidden lg:flex flex-1 flex-wrap gap-3 items-center bg-slate-900/40 p-3 rounded-2xl border border-slate-800/50 overflow-visible">
          {/* 桌面端：直接显示筛选器 */}
          <div className="flex items-center gap-2 text-slate-400 text-sm font-medium px-2">
            <Filter size={16} />
          </div>
          <div className="flex flex-wrap gap-3 flex-1 overflow-visible">
            {FilterSelects()}
          </div>

          <div className="w-px h-8 bg-slate-700 mx-2"></div>

          {/* 视图切换（仅桌面端显示） */}
          {!isMobile && (
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
          )}

          {/* 刷新按钮（桌面端显示，移动端已移到 Header） */}
          <div className="relative ml-auto hidden lg:block">
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

        {/* 时间范围选择 */}
        <div className="relative z-20 bg-slate-900/40 p-2 rounded-2xl border border-slate-800/50 backdrop-blur-md flex items-center gap-1 overflow-visible">
          {/* 时间对齐切换图标（附属于 24h，放在前面） */}
          <button
            onClick={() => {
              if (timeRange === '24h') {
                onTimeAlignChange(timeAlign === 'hour' ? '' : 'hour');
              }
            }}
            disabled={timeRange !== '24h'}
            title={timeRange === '24h'
              ? (timeAlign === 'hour' ? t('controls.timeAlign.hourTitle') : t('controls.timeAlign.dynamicTitle'))
              : undefined}
            className={`p-2 rounded-xl transition-all duration-200 flex-shrink-0 ${
              timeRange === '24h'
                ? 'text-cyan-400 hover:bg-slate-800 cursor-pointer'
                : 'text-slate-600 cursor-not-allowed'
            }`}
          >
            {timeAlign === 'hour' ? <AlignStartVertical size={16} /> : <Clock size={16} />}
          </button>

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

          {/* 时段筛选（仅 7d/30d 有效） */}
          <TimeFilterPicker
            value={timeFilter}
            disabled={timeRange === '24h'}
            onChange={onTimeFilterChange}
          />
        </div>
      </div>

      {/* 移动端筛选抽屉 */}
      {showFilterDrawer && (
        <div
          className="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm lg:hidden"
          onClick={() => onFilterDrawerClose?.()}
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
                onClick={() => onFilterDrawerClose?.()}
                className="p-2 rounded-lg bg-slate-800 text-slate-400 hover:text-slate-200 transition-colors"
                aria-label={t('controls.mobile.closeFilter')}
              >
                <X size={20} />
              </button>
            </div>

            {/* 筛选器列表 */}
            <div className="flex flex-col gap-4">
              {FilterSelects()}

              {/* 清空按钮 - 只清空可见的筛选器 */}
              {activeFiltersCount > 0 && (
                <button
                  onClick={() => {
                    if (showCategoryFilter) onCategoryChange([]);
                    if (providers.length > 0) onProviderChange([]);
                    onServiceChange([]);
                    onChannelChange([]);
                  }}
                  className="w-full py-3 bg-slate-800 text-slate-300 rounded-lg hover:bg-slate-750 transition-colors font-medium"
                >
                  {t('common.clear')}
                </button>
              )}

              {/* 应用按钮 */}
              <button
                onClick={() => onFilterDrawerClose?.()}
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
