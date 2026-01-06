import { useState, useEffect, useMemo, memo } from 'react';
import { FixedSizeList as List, type ListChildComponentProps } from 'react-window';
import { ArrowUpDown, ArrowUp, ArrowDown, Zap, Shield, Filter } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { StatusDot } from './StatusDot';
import { HeatmapBlock } from './HeatmapBlock';
import { LayeredHeatmapBlock } from './LayeredHeatmapBlock';
import { MultiModelIndicator } from './MultiModelIndicator';
import { ExternalLink } from './ExternalLink';
import { BadgeCell } from './badges';
import { FavoriteButton } from './FavoriteButton';
import { getStatusConfig, getTimeRanges } from '../constants';
import { availabilityToColor, latencyToColor, sponsorLevelToBorderClass, sponsorLevelToCardBorderColor, sponsorLevelToPinnedBgClass } from '../utils/color';
import { aggregateHeatmap } from '../utils/heatmapAggregator';
import { createMediaQueryEffect } from '../utils/mediaQuery';
import { hasAnyBadge, hasAnyBadgeInList } from '../utils/badgeUtils';
import { formatPriceRatioStructured } from '../utils/format';
import { getServiceIconComponent } from './ServiceIcon';
import type { ProcessedMonitorData, SortConfig } from '../types';

type HistoryPoint = ProcessedMonitorData['history'][number];

// 虚拟滚动常量
const MOBILE_ROW_HEIGHT = 160;  // 移动端卡片高度（约 150px 内容 + 10px 间距）
const MOBILE_MAX_HEIGHT = 800;  // 移动端列表最大高度

// ServiceIcon 模块级缓存，避免重复调用 getServiceIconComponent
const serviceIconCache = new Map<string, ReturnType<typeof getServiceIconComponent>>();
const getCachedServiceIcon = (serviceType: string) => {
  if (!serviceIconCache.has(serviceType)) {
    serviceIconCache.set(serviceType, getServiceIconComponent(serviceType));
  }
  return serviceIconCache.get(serviceType);
};

// 通道单元格组件（带自定义 CSS tooltip，替代原生 title 属性）
interface ChannelCellProps {
  channel?: string;
  probeUrl?: string;
  templateName?: string;
  className?: string;
}

function ChannelCell({ channel, probeUrl, templateName, className = '' }: ChannelCellProps) {
  const { t } = useTranslation();
  const hasTooltip = !!(probeUrl || templateName);

  // 无 tooltip 时直接显示文本
  if (!hasTooltip) {
    return <span className={className}>{channel || '-'}</span>;
  }

  // 统一向上弹出，避免被页脚遮挡
  // 注意：不使用 mb-1 间隙，避免鼠标移入 tooltip 时触发区失去 hover 导致闪烁
  const tooltipPositionClass = 'bottom-full left-0';

  return (
    <span className={`relative group/channel inline-flex items-center cursor-help ${className}`}>
      <span className="truncate">{channel || '-'}</span>
      {/* CSS tooltip - 悬停后显示，支持鼠标移入复制内容 */}
      {/* pointer-events-none 防止不可见时拦截鼠标事件，hover 时启用 */}
      <span
        className={`absolute ${tooltipPositionClass} px-2 py-1.5 bg-elevated border border-default text-xs rounded-lg shadow-lg opacity-0 pointer-events-none group-hover/channel:opacity-100 group-hover/channel:pointer-events-auto transition-opacity delay-150 z-50 select-text cursor-text md:min-w-[20rem] max-w-[90vw] md:max-w-2xl`}
      >
        <span className="flex flex-col gap-1">
          {probeUrl && (
            <span className="flex flex-col">
              <span className="text-muted text-[10px]">{t('table.channelTooltip.probeUrl')}</span>
              <span className="text-primary font-mono text-[11px] break-all">{probeUrl}</span>
            </span>
          )}
          {templateName && (
            <span className="flex flex-col">
              <span className="text-muted text-[10px]">{t('table.channelTooltip.template')}</span>
              <span className="text-primary font-mono text-[11px] break-all">{templateName}</span>
            </span>
          )}
        </span>
      </span>
    </span>
  );
}

interface StatusTableProps {
  data: ProcessedMonitorData[];
  sortConfig: SortConfig;
  isInitialSort?: boolean;   // 是否为初始排序状态（控制高亮显示）
  timeRange: string;
  slowLatencyMs: number;
  enableBadges?: boolean;      // 徽标系统总开关，默认 true
  showCategoryTag?: boolean; // 是否显示分类标签（推荐/公益），默认 true
  showProvider?: boolean;    // 是否显示服务商名称，默认 true
  showSponsor?: boolean;     // 是否显示赞助者信息，默认 true
  isFavorite: (id: string) => boolean;  // 检查是否已收藏
  onToggleFavorite: (id: string) => void; // 切换收藏状态
  onSort: (key: string) => void;
  onBlockHover: (e: React.MouseEvent<HTMLDivElement>, point: HistoryPoint) => void;
  onBlockLeave: () => void;
  onFilterProvider?: (providerId: string) => void; // 按服务商筛选
}

// 移动端卡片列表项组件
function MobileListItem({
  item,
  slowLatencyMs,
  enableBadges = true,
  showCategoryTag = true,
  showProvider = true,
  showSponsor = true,
  isFavorite,
  onToggleFavorite,
  onBlockHover,
  onBlockLeave,
}: {
  item: ProcessedMonitorData;
  slowLatencyMs: number;
  enableBadges?: boolean;
  showCategoryTag?: boolean;
  showProvider?: boolean;
  showSponsor?: boolean;
  isFavorite: boolean;
  onToggleFavorite: () => void;
  onBlockHover: (e: React.MouseEvent<HTMLDivElement>, point: HistoryPoint) => void;
  onBlockLeave: () => void;
}) {
  const { t, i18n } = useTranslation();
  const STATUS = getStatusConfig(t);
  const ServiceIcon = getCachedServiceIcon(item.serviceType);

  // 聚合热力图数据
  const aggregatedHistory = useMemo(
    () => aggregateHeatmap(item.history, 30),
    [item.history]
  );

  // 检查是否有徽标需要显示
  const hasItemBadges = hasAnyBadge(item, { enableBadges, showCategoryTag, showSponsor, showRisk: true });

  // 卡片左边框颜色（仅基于赞助级别，置顶改用背景色）
  const borderColor = sponsorLevelToCardBorderColor(item.sponsorLevel);

  // 是否显示左边框（仅基于赞助级别）
  const hasLeftBorder = !!item.sponsorLevel;

  // 置顶项使用对应徽标颜色的极淡背景色
  const pinnedBgClass = item.pinned ? sponsorLevelToPinnedBgClass(item.sponsorLevel) : '';
  const baseBgClass = pinnedBgClass || 'bg-surface/60';

  // 卡片最小高度 = 行高(160) - 行间距(8) = 152px
  // 确保所有卡片高度一致，避免虚拟列表中间距不均
  const cardMinHeight = 152;

  return (
    <div
      className={`${baseBgClass} border border-default rounded-r-xl ${hasLeftBorder ? 'rounded-l-sm border-l-2' : 'rounded-l-xl'} p-3 space-y-2`}
      style={{
        ...(borderColor ? { borderLeftColor: borderColor } : {}),
        minHeight: cardMinHeight,
      }}
    >
      {/* 徽标行 - 仅在有徽标时显示 */}
      {hasItemBadges && (
        <BadgeCell
          item={item}
          showCategoryTag={showCategoryTag}
          showSponsor={showSponsor}
          showRisk={true}
        />
      )}

      {/* 主要信息行 */}
      <div className="flex items-start justify-between gap-2">
        <div className="flex items-center gap-2 min-w-0 flex-1">
          {/* 服务图标 */}
          <div className="w-8 h-8 flex-shrink-0 rounded-lg bg-elevated flex items-center justify-center border border-default text-primary">
            {ServiceIcon ? (
              <ServiceIcon className="w-4 h-4" />
            ) : item.serviceType === 'cc' ? (
              <Zap className="text-service-cc" size={14} />
            ) : (
              <Shield className="text-service-cx" size={14} />
            )}
          </div>

          {/* 服务商名称 + 收藏按钮 */}
          <div className="min-w-0 flex-1">
            {showProvider && (
              <div className="flex items-center gap-1.5">
                <span className="font-semibold text-primary truncate text-sm leading-tight">
                  <ExternalLink href={item.providerUrl} compact requireConfirm>{item.providerName}</ExternalLink>
                </span>
                <FavoriteButton
                  isFavorite={isFavorite}
                  onToggle={onToggleFavorite}
                  size={12}
                  inline
                />
              </div>
            )}
            <div className="flex items-center gap-2 mt-0.5 text-xs text-secondary">
              {/* 赞助者（放在服务类型前） */}
              {showSponsor && item.sponsor && (
                <span className="text-[10px] text-muted truncate max-w-[80px]">
                  <ExternalLink href={item.sponsorUrl} compact>{item.sponsor}</ExternalLink>
                </span>
              )}
              <span
                className={`px-1.5 py-0.5 rounded text-[10px] font-mono border flex-shrink-0 ${
                  item.serviceType === 'cc'
                    ? 'border-service-cc text-service-cc bg-service-cc'
                    : item.serviceType === 'gm'
                    ? 'border-service-gm text-service-gm bg-service-gm'
                    : 'border-service-cx text-service-cx bg-service-cx'
                }`}
              >
                {item.serviceName.toUpperCase()}
              </span>
              {item.channel && (
                <ChannelCell
                  channel={item.channelName || item.channel}
                  probeUrl={item.probeUrl}
                  templateName={item.templateName}
                  className="text-muted truncate"
                />
              )}
              {/* 收录时间 */}
              {item.listedDays != null && (
                <span className="text-[10px] text-muted font-mono flex-shrink-0">
                  {item.listedDays}d
                </span>
              )}
            </div>
          </div>
        </div>

        {/* 状态、可用率、时间和延迟 */}
        <div className="flex flex-col items-end gap-1 flex-shrink-0">
          <div className="flex items-center gap-1.5 px-2 py-1 rounded-full bg-elevated border border-default">
            <StatusDot status={item.currentStatus} size="sm" />
            <span className={`text-xs font-bold ${STATUS[item.currentStatus].text}`}>
              {STATUS[item.currentStatus].label}
            </span>
          </div>
          <span
            className="text-sm font-mono font-bold"
            style={{ color: availabilityToColor(item.uptime) }}
          >
            {item.uptime >= 0 ? `${item.uptime}%` : '--'}
          </span>
          {/* 时间和延迟（总是显示） */}
          <div className="flex items-center gap-2 text-[10px] text-muted font-mono">
            {item.lastCheckTimestamp && (
              <span>
                {new Date(item.lastCheckTimestamp * 1000).toLocaleString(i18n.language, {
                  month: '2-digit',
                  day: '2-digit',
                  hour: '2-digit',
                  minute: '2-digit',
                })}
              </span>
            )}
            {item.lastCheckLatency !== undefined && (
              <span style={{ color: item.currentStatus === 'UNAVAILABLE' ? 'hsl(var(--text-muted))' : latencyToColor(item.lastCheckLatency, item.slowLatencyMs ?? slowLatencyMs) }}>
                {item.lastCheckLatency}ms
              </span>
            )}
          </div>
        </div>
      </div>

      {/* 热力图 */}
      <div className="flex items-center gap-[2px] h-5 w-full overflow-hidden rounded-sm">
        {/* 多模型标记 */}
        {item.isMultiModel && item.layers && (
          <MultiModelIndicator layers={item.layers} className="flex-shrink-0 mr-1" />
        )}
        {aggregatedHistory.map((point, idx) => (
          <HeatmapBlock
            key={idx}
            point={point}
            width={`${100 / aggregatedHistory.length}%`}
            height="h-full"
            onHover={onBlockHover}
            onLeave={onBlockLeave}
            isMobile
          />
        ))}
      </div>
    </div>
  );
}

// 移动端排序菜单
function MobileSortMenu({
  sortConfig,
  isInitialSort,
  onSort,
}: {
  sortConfig: SortConfig;
  isInitialSort?: boolean;
  onSort: (key: string) => void;
}) {
  const { t } = useTranslation();

  const sortOptions = [
    { key: 'badgeScore', label: t('table.sorting.badge') },
    { key: 'providerName', label: t('table.sorting.provider') },
    { key: 'uptime', label: t('table.sorting.uptime') },
    { key: 'currentStatus', label: t('table.sorting.status') },
    { key: 'latency', label: t('table.sorting.latency') },
    { key: 'serviceType', label: t('table.sorting.service') },
    { key: 'priceRatio', label: t('table.sorting.priceRatio') },
    { key: 'listedDays', label: t('table.sorting.listedDays') },
  ];

  return (
    <div className="flex items-center gap-2 mb-2 overflow-x-auto pb-2">
      <span className="text-xs text-muted flex-shrink-0">{t('controls.sortBy')}</span>
      {sortOptions.map((option) => {
        // 初始状态下不高亮任何排序按钮
        const isActive = !isInitialSort && sortConfig.key === option.key;
        return (
          <button
            key={option.key}
            onClick={() => onSort(option.key)}
            className={`flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-xs font-medium transition-colors flex-shrink-0 focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none ${
              isActive
                ? 'bg-accent/20 text-accent border border-accent/30'
                : 'bg-elevated text-secondary border border-default hover:text-primary'
            }`}
          >
            {option.label}
            {isActive && (
              sortConfig.direction === 'asc' ? (
                <ArrowUp size={12} />
              ) : (
                <ArrowDown size={12} />
              )
            )}
          </button>
        );
      })}
    </div>
  );
}

function StatusTableComponent({
  data,
  sortConfig,
  isInitialSort = false,
  timeRange,
  slowLatencyMs,
  enableBadges = true,
  showCategoryTag = true,
  showProvider = true,
  showSponsor = true,
  isFavorite,
  onToggleFavorite,
  onSort,
  onBlockHover,
  onBlockLeave,
  onFilterProvider,
}: StatusTableProps) {
  const { t, i18n } = useTranslation();
  const [isMobile, setIsMobile] = useState(false);
  const STATUS = getStatusConfig(t);

  // 检测是否为平板/移动端（< 960px，兼容 Safari ≤13）
  useEffect(() => {
    const cleanup = createMediaQueryEffect('tablet', setIsMobile);
    return cleanup;
  }, []);

  // 排序图标：初始状态下不显示高亮
  const SortIcon = ({ columnKey }: { columnKey: string }) => {
    // 初始状态下所有排序图标都不高亮
    if (isInitialSort || sortConfig.key !== columnKey) {
      return <ArrowUpDown size={14} className="opacity-30 ml-1" />;
    }
    return sortConfig.direction === 'asc' ? (
      <ArrowUp size={14} className="text-accent ml-1" />
    ) : (
      <ArrowDown size={14} className="text-accent ml-1" />
    );
  };

  const currentTimeRange = getTimeRanges(t).find((r) => r.id === timeRange);

  // 移动端：虚拟滚动卡片列表视图
  if (isMobile) {
    // 计算虚拟列表高度（最大 MOBILE_MAX_HEIGHT，最小为所有项目高度）
    const mobileListHeight = Math.min(
      data.length * MOBILE_ROW_HEIGHT,
      MOBILE_MAX_HEIGHT
    );

    // 虚拟列表行渲染函数（itemSize=208，卡片高度200，底部留8px间距）
    const renderMobileRow = ({ index, style }: ListChildComponentProps) => {
      const item = data[index];
      return (
        <div style={style}>
          <div style={{ marginBottom: 8 }}>
            <MobileListItem
              item={item}
              slowLatencyMs={slowLatencyMs}
              enableBadges={enableBadges}
              showCategoryTag={showCategoryTag}
              showProvider={showProvider}
              showSponsor={showSponsor}
              isFavorite={isFavorite(item.id)}
              onToggleFavorite={() => onToggleFavorite(item.id)}
              onBlockHover={onBlockHover}
              onBlockLeave={onBlockLeave}
            />
          </div>
        </div>
      );
    };

    return (
      <div>
        <MobileSortMenu sortConfig={sortConfig} isInitialSort={isInitialSort} onSort={onSort} />
        <List
          height={mobileListHeight}
          itemCount={data.length}
          itemSize={MOBILE_ROW_HEIGHT}
          width="100%"
          overscanCount={3}
          itemKey={(index) => data[index].id}
        >
          {renderMobileRow}
        </List>
      </div>
    );
  }

  // 检查是否有任何徽标需要显示
  const hasBadges = hasAnyBadgeInList(data, { enableBadges, showCategoryTag, showSponsor, showRisk: true });

  // 桌面端：表格视图
  return (
    <div className="overflow-x-auto rounded-2xl border border-default/50 shadow-xl bg-surface/40 backdrop-blur-sm">
      <table className="w-full text-left border-collapse bg-transparent">
        <thead>
          <tr className="border-b border-default/50 text-secondary text-xs uppercase tracking-wider">
            {/* 徽标列 - 仅在有徽标时显示，可排序 */}
            {hasBadges && (
              <th
                className="px-2 py-3 font-medium w-12 cursor-pointer hover:text-accent transition-colors focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none"
                onClick={() => onSort('badgeScore')}
                onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), onSort('badgeScore'))}
                tabIndex={0}
                role="button"
              >
                <div className="flex items-center">
                  {t('table.headers.badge')} <SortIcon columnKey="badgeScore" />
                </div>
              </th>
            )}
            {/* 服务商列（合并赞助者） */}
            {showProvider && (
              <th
                className="px-3 py-3 font-medium cursor-pointer hover:text-accent transition-colors focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none"
                onClick={() => onSort('providerName')}
                onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), onSort('providerName'))}
                tabIndex={0}
                role="button"
              >
                <div className="flex items-center">
                  {t('table.headers.provider')} <SortIcon columnKey="providerName" />
                </div>
              </th>
            )}
            <th
              className="px-2 py-3 font-medium cursor-pointer hover:text-accent transition-colors whitespace-nowrap focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none"
              onClick={() => onSort('serviceType')}
              onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), onSort('serviceType'))}
              tabIndex={0}
              role="button"
            >
              <div className="flex items-center">
                {t('table.headers.service')} <SortIcon columnKey="serviceType" />
              </div>
            </th>
            <th
              className="px-2 py-3 font-medium cursor-pointer hover:text-accent transition-colors focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none"
              onClick={() => onSort('channel')}
              onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), onSort('channel'))}
              tabIndex={0}
              role="button"
            >
              <div className="flex items-center">
                {t('table.headers.channel')} <SortIcon columnKey="channel" />
              </div>
            </th>
            <th
              className="px-2 py-3 font-medium cursor-pointer hover:text-accent transition-colors focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none"
              onClick={() => onSort('priceRatio')}
              onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), onSort('priceRatio'))}
              tabIndex={0}
              role="button"
            >
              <div className="flex items-center">
                <div className="flex flex-col leading-tight">
                  <span>{t('table.headers.priceRatio')}</span>
                  <span className="text-[10px] opacity-50 font-normal">{t('table.headers.priceRatioUnit')}</span>
                </div>
                <SortIcon columnKey="priceRatio" />
              </div>
            </th>
            <th
              className="px-2 py-3 font-medium cursor-pointer hover:text-accent transition-colors whitespace-nowrap focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none"
              onClick={() => onSort('listedDays')}
              onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), onSort('listedDays'))}
              tabIndex={0}
              role="button"
            >
              <div className="flex items-center">
                {t('table.headers.listedDays')} <SortIcon columnKey="listedDays" />
              </div>
            </th>
            <th
              className="px-2 py-3 font-medium cursor-pointer hover:text-accent transition-colors whitespace-nowrap focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none"
              onClick={() => onSort('currentStatus')}
              onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), onSort('currentStatus'))}
              tabIndex={0}
              role="button"
            >
              <div className="flex items-center">
                {t('table.headers.status')} <SortIcon columnKey="currentStatus" />
              </div>
            </th>
            <th
              className="px-2 py-3 font-medium cursor-pointer hover:text-accent transition-colors whitespace-nowrap focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none"
              onClick={() => onSort('uptime')}
              onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), onSort('uptime'))}
              tabIndex={0}
              role="button"
            >
              <div className="flex items-center">
                {t('table.headers.uptime')} <SortIcon columnKey="uptime" />
              </div>
            </th>
            <th
              className="px-2 py-3 font-medium cursor-pointer hover:text-accent transition-colors whitespace-nowrap focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none"
              onClick={() => onSort('latency')}
              onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), onSort('latency'))}
              tabIndex={0}
              role="button"
            >
              <div className="flex items-center">
                {t('table.headers.lastCheck')} <SortIcon columnKey="latency" />
              </div>
            </th>
            <th className="pl-2 pr-4 py-3 font-medium w-[360px] min-w-[320px]">
              <div className="flex items-center gap-2">
                {t('table.headers.trend')}
                <span className="text-[10px] normal-case opacity-50 border border-default px-1 rounded">
                  {currentTimeRange?.label}
                </span>
              </div>
            </th>
          </tr>
        </thead>
        <tbody className="divide-y divide-default/50 text-sm">
          {data.map((item, rowIndex) => {
            const ServiceIcon = getCachedServiceIcon(item.serviceType);
            const hasItemBadges = hasAnyBadge(item, { enableBadges, showCategoryTag, showSponsor, showRisk: true });
            const pinnedBg = item.pinned ? sponsorLevelToPinnedBgClass(item.sponsorLevel) : '';
            return (
            <tr
              key={item.id}
              className={`group hover:bg-elevated/40 transition-[background-color,color] ${pinnedBg} ${sponsorLevelToBorderClass(item.sponsorLevel)}`}
            >
              {/* 徽标列 - 使用 BadgeCell 统一渲染 */}
              {hasBadges && (
                <td className="px-2 py-1">
                  {hasItemBadges ? (
                    <BadgeCell
                      item={item}
                      showCategoryTag={showCategoryTag}
                      showSponsor={showSponsor}
                      showRisk={true}
                      tooltipPlacement={rowIndex === 0 ? 'bottom' : 'top'}
                    />
                  ) : null}
                </td>
              )}
              {/* 服务商列（两行紧贴，整体垂直居中） */}
              {showProvider && (
                <td className="px-2 py-1.5">
                  <div className="flex items-center h-8 group/provider">
                    <div className="flex flex-col gap-0 flex-1 min-w-0">
                      <div className="flex items-center gap-1.5">
                        <span className="font-medium text-primary text-sm leading-tight truncate">
                          <ExternalLink href={item.providerUrl} inline requireConfirm>{item.providerName}</ExternalLink>
                        </span>
                        {/* 收藏按钮：始终显示，未收藏时弱化 */}
                        <div className="flex-shrink-0">
                          <FavoriteButton
                            isFavorite={isFavorite(item.id)}
                            onToggle={() => onToggleFavorite(item.id)}
                            size={12}
                            inline
                          />
                        </div>
                        {/* 过滤按钮：悬浮时显示 */}
                        {onFilterProvider && (
                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation();
                              onFilterProvider(item.providerId);
                            }}
                            className="flex-shrink-0 p-0.5 rounded opacity-0 group-hover/provider:opacity-60 hover:!opacity-100 hover:text-accent transition-opacity cursor-pointer"
                            title={t('table.filterByProvider')}
                          >
                            <Filter size={10} />
                          </button>
                        )}
                      </div>
                      {/* 官方 API Key (api_key_official) 时隐藏赞助者 */}
                      {showSponsor && item.sponsor && !item.badges?.some(b => b.id === 'api_key_official') && (
                        <span className="text-[9px] text-muted leading-none">
                          <ExternalLink href={item.sponsorUrl} inline>{item.sponsor}</ExternalLink>
                        </span>
                      )}
                    </div>
                  </div>
                </td>
              )}
              <td className="px-2 py-1">
                <span
                  className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-mono border ${
                    item.serviceType === 'cc'
                      ? 'border-service-cc text-service-cc bg-service-cc'
                      : item.serviceType === 'gm'
                      ? 'border-service-gm text-service-gm bg-service-gm'
                      : 'border-service-cx text-service-cx bg-service-cx'
                  }`}
                >
                  {ServiceIcon ? (
                    <ServiceIcon className="w-3.5 h-3.5 mr-1 text-primary" />
                  ) : (
                    <>
                      {item.serviceType === 'cc' && <Zap size={10} className="mr-1 text-primary" />}
                      {item.serviceType === 'cx' && <Shield size={10} className="mr-1 text-primary" />}
                    </>
                  )}
                  {item.serviceName.toUpperCase()}
                </span>
              </td>
              <td className="px-2 py-1 text-secondary text-xs">
                <ChannelCell
                  channel={item.channelName || item.channel}
                  probeUrl={item.probeUrl}
                  templateName={item.templateName}
                />
              </td>
              <td className="px-2 py-1 font-mono text-xs whitespace-nowrap">
                {(() => {
                  const priceData = formatPriceRatioStructured(item.priceMin, item.priceMax);
                  if (!priceData) return <span className="text-muted">-</span>;
                  return (
                    <div className="flex flex-col leading-tight">
                      <span className="text-secondary">{priceData.base}</span>
                      {priceData.sub && (
                        <span className="text-[10px] text-muted">{priceData.sub}</span>
                      )}
                    </div>
                  );
                })()}
              </td>
              <td className="px-2 py-1 font-mono text-xs text-secondary whitespace-nowrap">
                {item.listedDays != null ? `${item.listedDays}d` : '-'}
              </td>
              <td className="px-2 py-1">
                <div className="flex items-center gap-1.5 whitespace-nowrap">
                  <StatusDot status={item.currentStatus} size="sm" />
                  <span className={STATUS[item.currentStatus].text}>
                    {STATUS[item.currentStatus].label}
                  </span>
                </div>
              </td>
              <td className="px-2 py-1 font-mono font-bold whitespace-nowrap">
                <span style={{ color: availabilityToColor(item.uptime) }}>
                  {item.uptime >= 0 ? `${item.uptime}%` : '--'}
                </span>
              </td>
              <td className="px-2 py-1">
                {item.lastCheckTimestamp ? (
                  <div className="text-xs text-secondary font-mono flex flex-col gap-0.5">
                    <span>{new Date(item.lastCheckTimestamp * 1000).toLocaleString(i18n.language, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })}</span>
                    {item.lastCheckLatency !== undefined && (
                      <span
                        className="text-[10px] font-mono"
                        style={{ color: item.currentStatus === 'UNAVAILABLE' ? 'hsl(var(--text-muted))' : latencyToColor(item.lastCheckLatency, item.slowLatencyMs ?? slowLatencyMs) }}
                      >
                        {item.lastCheckLatency}ms
                      </span>
                    )}
                  </div>
                ) : (
                  <span className="text-muted text-xs">-</span>
                )}
              </td>
              <td className="pl-2 pr-4 py-1.5 align-middle">
                <div className="flex items-center gap-[2px] h-5 w-full overflow-hidden rounded-sm">
                  {/* 多模型标记 */}
                  {item.isMultiModel && item.layers && (
                    <MultiModelIndicator layers={item.layers} className="flex-shrink-0 mr-1" />
                  )}
                  {/* 热力图：多层 vs 单层 */}
                  {item.isMultiModel && item.layers ? (
                    // Phase B: 多层垂直堆叠热力图
                    item.history.map((_, idx) => (
                      <LayeredHeatmapBlock
                        key={idx}
                        layers={item.layers!}
                        timeIndex={idx}
                        width={`${100 / item.history.length}%`}
                        height="h-full"
                        onHover={onBlockHover}
                        onLeave={onBlockLeave}
                        isMobile={false}
                        slowLatencyMs={item.slowLatencyMs ?? slowLatencyMs}
                      />
                    ))
                  ) : (
                    // Phase A: 单层传统热力图
                    item.history.map((point, idx) => (
                      <HeatmapBlock
                        key={idx}
                        point={point}
                        width={`${100 / item.history.length}%`}
                        height="h-full"
                        onHover={onBlockHover}
                        onLeave={onBlockLeave}
                        isMobile={false}
                      />
                    ))
                  )}
                </div>
              </td>
            </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

export const StatusTable = memo(StatusTableComponent);
