import { useMemo } from 'react';
import { Filter, RefreshCw, LayoutGrid, List, X, Clock, AlignStartVertical, Star } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { getTimeRanges } from '../constants';
import { MultiSelect } from './MultiSelect';
import { TimeFilterPicker } from './TimeFilterPicker';
import { SubscribeButton } from './SubscribeButton';
import type { MultiSelectOption } from './MultiSelect';
import type { ViewMode, ProviderOption } from '../types';

interface ControlsProps {
  filterProvider: string[];  // 多选服务商，空数组表示"全部"
  filterService: string[];   // 多选服务，空数组表示"全部"
  filterChannel: string[];   // 多选通道，空数组表示"全部"
  filterCategory: string[];  // 多选分类，空数组表示"全部"
  showFavoritesOnly: boolean; // 仅显示收藏
  favorites: Set<string>;     // 收藏项集合
  favoritesCount: number;     // 收藏数量
  timeRange: string;
  timeAlign: string;         // 时间对齐模式：空=动态窗口, "hour"=整点对齐
  timeFilter: string | null; // 每日时段过滤：null=全天, "09:00-17:00"=自定义
  viewMode: ViewMode;
  loading: boolean;
  channels: string[];
  providers: ProviderOption[];  // 改为 ProviderOption[]
  effectiveServices: string[];    // 动态服务选项（始终传递数组）
  effectiveCategories: string[];  // 动态分类选项（始终传递数组）
  showCategoryFilter?: boolean; // 是否显示分类筛选器，默认 true（用于服务商专属页面）
  refreshCooldown?: boolean; // 刷新冷却中，显示提示
  isMobile?: boolean; // 是否为移动端，用于隐藏视图切换按钮
  showFilterDrawer?: boolean; // 移动端筛选抽屉是否显示（由 App 层级控制）
  onFilterDrawerClose?: () => void; // 关闭筛选抽屉回调
  onProviderChange: (providers: string[]) => void;  // 多选回调
  onServiceChange: (services: string[]) => void;    // 多选回调
  onChannelChange: (channels: string[]) => void;    // 多选回调
  onCategoryChange: (categories: string[]) => void; // 多选回调
  onShowFavoritesOnlyChange: (value: boolean) => void; // 收藏筛选回调
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
  showFavoritesOnly,
  favorites,
  favoritesCount,
  timeRange,
  timeAlign,
  timeFilter,
  viewMode,
  loading,
  channels,
  providers,
  effectiveServices,
  effectiveCategories,
  showCategoryFilter = true,
  refreshCooldown = false,
  isMobile = false,
  showFilterDrawer = false,
  onFilterDrawerClose,
  onProviderChange,
  onServiceChange,
  onChannelChange,
  onCategoryChange,
  onShowFavoritesOnlyChange,
  onTimeRangeChange,
  onTimeAlignChange,
  onTimeFilterChange,
  onViewModeChange,
  onRefresh,
}: ControlsProps) {
  const { t } = useTranslation();

  // 服务选项（始终基于 effectiveServices 动态计算）
  const serviceOptions = useMemo<MultiSelectOption[]>(() => {
    const allOptions = [
      { value: 'cc', label: t('controls.services.cc') },
      { value: 'cx', label: t('controls.services.cx') },
    ];
    // 空数组表示无数据，显示全部选项作为回退
    if (effectiveServices.length === 0) return allOptions;
    return allOptions.filter(opt => effectiveServices.includes(opt.value));
  }, [t, effectiveServices]);

  // 分类选项（始终基于 effectiveCategories 动态计算）
  const categoryOptions = useMemo<MultiSelectOption[]>(() => {
    const allOptions = [
      { value: 'public', label: t('controls.categories.charity') },
      { value: 'commercial', label: t('controls.categories.promoted') },
    ];
    // 空数组表示无数据，显示全部选项作为回退
    if (effectiveCategories.length === 0) return allOptions;
    return allOptions.filter(opt => effectiveCategories.includes(opt.value));
  }, [t, effectiveCategories]);

  // 通道选项（动态值）
  const channelOptions = useMemo<MultiSelectOption[]>(() =>
    channels.map(channel => ({ value: channel, label: channel })),
  [channels]);

  // 统计激活的筛选器数量（仅计入可见的筛选器）
  const activeFiltersCount = [
    showFavoritesOnly,
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
      <div className="flex flex-col lg:flex-row gap-2 mb-2 lg:mb-3 overflow-visible">
        {/* 筛选和视图控制（移动端隐藏，筛选/刷新已移到 Header） */}
        <div className="hidden lg:flex flex-1 flex-wrap gap-2 items-center bg-surface/60 p-2 rounded-2xl overflow-visible">
          {/* 桌面端：直接显示筛选器 */}
          <div className="flex items-center gap-2 text-secondary text-sm font-medium px-1">
            <Filter size={16} />
          </div>
          <div className="flex flex-wrap gap-2 flex-1 overflow-visible">
            {FilterSelects()}
          </div>

          {/* 收藏筛选按钮 */}
          <button
            type="button"
            onClick={() => onShowFavoritesOnlyChange(!showFavoritesOnly)}
            className={`
              flex items-center gap-1.5 px-3 py-2 rounded-lg transition-all duration-200
              focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none
              ${showFavoritesOnly
                ? 'bg-accent/10 text-accent border border-accent/30'
                : 'bg-elevated/50 text-secondary hover:text-primary hover:bg-muted/50 border border-transparent'
              }
              ${!showFavoritesOnly && favoritesCount === 0 ? 'opacity-50 cursor-not-allowed' : ''}
            `}
            disabled={!showFavoritesOnly && favoritesCount === 0}
            title={showFavoritesOnly
              ? t('controls.favorites.exitMode')
              : (favoritesCount > 0
                ? t('controls.favorites.showOnly')
                : t('controls.favorites.noFavorites'))
            }
            aria-pressed={showFavoritesOnly}
          >
            <Star
              size={14}
              className={showFavoritesOnly ? 'text-warning' : ''}
              fill={showFavoritesOnly ? 'currentColor' : 'none'}
              strokeWidth={showFavoritesOnly ? 0 : 2}
            />
            {favoritesCount > 0 && (
              <span className="text-xs font-medium">{favoritesCount}</span>
            )}
          </button>

          {/* 订阅通知按钮（图标模式） */}
          <SubscribeButton favorites={favorites} iconOnly />

          <div className="w-px h-6 bg-muted mx-1"></div>

          {/* 视图切换（仅桌面端显示） */}
          {!isMobile && (
            <div className="flex bg-surface rounded-lg p-1 border border-default/50 shadow-sm">
              <button
                type="button"
                onClick={() => onViewModeChange('table')}
                className={`p-2.5 rounded min-w-[44px] min-h-[44px] flex items-center justify-center focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none ${
                  viewMode === 'table'
                    ? 'bg-muted text-accent shadow'
                    : 'text-secondary hover:text-primary'
                }`}
                title={t('controls.views.table')}
                aria-label={t('controls.views.switchToTable')}
              >
                <List size={18} />
              </button>
              <button
                type="button"
                onClick={() => onViewModeChange('grid')}
                className={`p-2.5 rounded min-w-[44px] min-h-[44px] flex items-center justify-center focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none ${
                  viewMode === 'grid'
                    ? 'bg-muted text-accent shadow'
                    : 'text-secondary hover:text-primary'
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
              type="button"
              onClick={onRefresh}
              className="p-2.5 rounded-lg bg-accent/10 text-accent hover:bg-accent/20 transition-colors border border-accent/20 group min-w-[44px] min-h-[44px] flex items-center justify-center cursor-pointer focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none"
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
              <div className="absolute top-full left-1/2 -translate-x-1/2 mt-2 px-3 py-1.5 bg-elevated text-secondary text-xs rounded-lg whitespace-nowrap shadow-lg border border-default z-50">
                {t('common.refreshCooldown')}
              </div>
            )}
          </div>
        </div>

        {/* 时间范围选择 */}
        <div className="relative z-20 bg-surface/40 p-2 rounded-2xl backdrop-blur-md flex items-center gap-1 overflow-visible">
          {/* 时间对齐切换图标（附属于 24h，放在前面） */}
          <button
            type="button"
            onClick={() => {
              if (timeRange === '24h') {
                onTimeAlignChange(timeAlign === 'hour' ? '' : 'hour');
              }
            }}
            disabled={timeRange !== '24h'}
            title={timeRange === '24h'
              ? (timeAlign === 'hour' ? t('controls.timeAlign.hourTitle') : t('controls.timeAlign.dynamicTitle'))
              : undefined}
            className={`p-2 rounded-xl transition-all duration-200 flex-shrink-0 focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none ${
              timeRange === '24h'
                ? 'text-accent hover:bg-elevated cursor-pointer'
                : 'text-muted cursor-not-allowed'
            }`}
          >
            {timeAlign === 'hour' ? <AlignStartVertical size={16} /> : <Clock size={16} />}
          </button>

          {getTimeRanges(t).map((range) => (
            <button
              type="button"
              key={range.id}
              onClick={() => onTimeRangeChange(range.id)}
              className={`px-3 py-2 text-xs font-medium rounded-xl transition-all duration-200 whitespace-nowrap flex-shrink-0 focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none ${
                timeRange === range.id
                  ? 'bg-gradient-button text-inverse shadow-lg shadow-accent/25'
                  : 'text-secondary hover:text-primary hover:bg-elevated'
              }`}
            >
              {range.label}
            </button>
          ))}

          {/* 时段筛选（仅 7d/30d 有效） */}
          <TimeFilterPicker
            value={timeFilter}
            disabled={timeRange === '24h' || timeRange === '90m'}
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
            className="absolute bottom-0 left-0 right-0 bg-surface border-t border-default rounded-t-2xl p-6 max-h-[80vh] overflow-y-auto"
            onClick={(e) => e.stopPropagation()}
          >
            {/* 抽屉头部 */}
            <div className="flex justify-between items-center mb-6">
              <div className="flex items-center gap-2">
                <Filter size={20} className="text-accent" />
                <h3 className="text-lg font-semibold text-primary">{t('controls.mobile.filterTitle')}</h3>
                {activeFiltersCount > 0 && (
                  <span className="px-2 py-0.5 bg-accent text-inverse text-xs rounded-full">
                    {activeFiltersCount}
                  </span>
                )}
              </div>
              <button
                type="button"
                onClick={() => onFilterDrawerClose?.()}
                className="p-2 rounded-lg bg-elevated text-secondary hover:text-primary transition-colors focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none"
                aria-label={t('controls.mobile.closeFilter')}
              >
                <X size={20} />
              </button>
            </div>

            {/* 筛选器列表 */}
            <div className="flex flex-col gap-4">
              {FilterSelects()}

              {/* 收藏筛选按钮（移动端） */}
              <button
                type="button"
                onClick={() => onShowFavoritesOnlyChange(!showFavoritesOnly)}
                className={`
                  flex items-center justify-center gap-2 w-full py-3 rounded-lg transition-all duration-200
                  focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none
                  ${showFavoritesOnly
                    ? 'bg-accent/10 text-accent border border-accent/30'
                    : 'bg-elevated text-secondary hover:text-primary border border-transparent'
                  }
                  ${!showFavoritesOnly && favoritesCount === 0 ? 'opacity-50 cursor-not-allowed' : ''}
                `}
                disabled={!showFavoritesOnly && favoritesCount === 0}
              >
                <Star
                  size={16}
                  className={showFavoritesOnly ? 'text-warning' : ''}
                  fill={showFavoritesOnly ? 'currentColor' : 'none'}
                  strokeWidth={showFavoritesOnly ? 0 : 2}
                />
                <span className="font-medium">
                  {t('controls.favorites.showOnly')}
                  {favoritesCount > 0 && ` (${favoritesCount})`}
                </span>
              </button>

              {/* 订阅通知按钮（移动端） */}
              <SubscribeButton favorites={favorites} className="w-full justify-center py-3" />

              {/* 清空按钮 - 只清空可见的筛选器 */}
              {activeFiltersCount > 0 && (
                <button
                  type="button"
                  onClick={() => {
                    onShowFavoritesOnlyChange(false);
                    if (showCategoryFilter) onCategoryChange([]);
                    if (providers.length > 0) onProviderChange([]);
                    onServiceChange([]);
                    onChannelChange([]);
                  }}
                  className="w-full py-3 bg-elevated text-secondary rounded-lg hover:bg-muted transition-colors font-medium focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none"
                >
                  {t('common.clear')}
                </button>
              )}

              {/* 应用按钮 */}
              <button
                type="button"
                onClick={() => onFilterDrawerClose?.()}
                className="w-full py-3 bg-gradient-button text-inverse rounded-lg font-medium shadow-lg shadow-accent/25 hover:shadow-accent/40 transition-all focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none"
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
