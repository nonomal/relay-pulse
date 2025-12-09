import { useState, useEffect, useMemo, memo } from 'react';
import { FixedSizeList as List, type ListChildComponentProps } from 'react-window';
import { ArrowUpDown, ArrowUp, ArrowDown, Zap, Shield } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { StatusDot } from './StatusDot';
import { HeatmapBlock } from './HeatmapBlock';
import { ExternalLink } from './ExternalLink';
import { BadgeCell } from './badges';
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

interface StatusTableProps {
  data: ProcessedMonitorData[];
  sortConfig: SortConfig;
  isInitialSort?: boolean;   // 是否为初始排序状态（控制高亮显示）
  timeRange: string;
  slowLatencyMs: number;
  showCategoryTag?: boolean; // 是否显示分类标签（推荐/公益），默认 true
  showProvider?: boolean;    // 是否显示服务商名称，默认 true
  showSponsor?: boolean;     // 是否显示赞助者信息，默认 true
  onSort: (key: string) => void;
  onBlockHover: (e: React.MouseEvent<HTMLDivElement>, point: HistoryPoint) => void;
  onBlockLeave: () => void;
}

// 移动端卡片列表项组件
function MobileListItem({
  item,
  slowLatencyMs,
  showCategoryTag = true,
  showProvider = true,
  showSponsor = true,
  onBlockHover,
  onBlockLeave,
}: {
  item: ProcessedMonitorData;
  slowLatencyMs: number;
  showCategoryTag?: boolean;
  showProvider?: boolean;
  showSponsor?: boolean;
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
  const hasItemBadges = hasAnyBadge(item, { showCategoryTag, showSponsor, showRisk: true });

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

          {/* 服务商名称 */}
          <div className="min-w-0 flex-1">
            {showProvider && (
              <span className="font-semibold text-primary truncate text-sm leading-tight block">
                <ExternalLink href={item.providerUrl} compact requireConfirm>{item.providerName}</ExternalLink>
              </span>
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
                    : 'border-service-cx text-service-cx bg-service-cx'
                }`}
              >
                {item.serviceType.toUpperCase()}
              </span>
              {item.channel && (
                <span className="text-muted truncate">{item.channel}</span>
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
              <span style={{ color: item.currentStatus === 'UNAVAILABLE' ? 'hsl(var(--text-muted))' : latencyToColor(item.lastCheckLatency, slowLatencyMs) }}>
                {item.lastCheckLatency}ms
              </span>
            )}
          </div>
        </div>
      </div>

      {/* 热力图 */}
      <div className="flex items-center gap-[2px] h-5 w-full overflow-hidden rounded-sm">
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
  showCategoryTag = true,
  showProvider = true,
  showSponsor = true,
  onSort,
  onBlockHover,
  onBlockLeave,
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
    const renderMobileRow = ({ index, style }: ListChildComponentProps) => (
      <div style={style}>
        <div style={{ marginBottom: 8 }}>
          <MobileListItem
            item={data[index]}
            slowLatencyMs={slowLatencyMs}
            showCategoryTag={showCategoryTag}
            showProvider={showProvider}
            showSponsor={showSponsor}
            onBlockHover={onBlockHover}
            onBlockLeave={onBlockLeave}
          />
        </div>
      </div>
    );

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
  const hasBadges = hasAnyBadgeInList(data, { showCategoryTag, showSponsor, showRisk: true });

  // 桌面端：表格视图
  return (
    <div className="overflow-x-auto overflow-y-hidden rounded-2xl border border-default/50 shadow-xl">
      <table className="w-full text-left border-collapse bg-surface/40 backdrop-blur-sm table-fixed">
        {/* 使用 colgroup 定义列宽，确保热力图列获得足够空间 */}
        <colgroup>
          {hasBadges && <col className="w-12" />}
          {showProvider && <col className="w-[120px]" />}
          <col className="w-16" />  {/* 服务类型 */}
          <col className="w-20" />  {/* 渠道 */}
          <col className="w-16" />  {/* 倍率 */}
          <col className="w-14" />  {/* 收录天数 */}
          <col className="w-20" />  {/* 当前状态 */}
          <col className="w-16" />  {/* 可用率 */}
          <col className="w-24" />  {/* 最后检测 */}
          <col />  {/* 热力图列 - 自动填充剩余空间 */}
        </colgroup>
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
            <th className="pl-2 pr-4 py-3 font-medium min-w-[280px]">
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
          {data.map((item) => {
            const ServiceIcon = getCachedServiceIcon(item.serviceType);
            const hasItemBadges = hasAnyBadge(item, { showCategoryTag, showSponsor, showRisk: true });
            const pinnedBg = item.pinned ? sponsorLevelToPinnedBgClass(item.sponsorLevel) : '';
            return (
            <tr
              key={item.id}
              className={`group hover:bg-elevated/40 transition-[background-color,color] ${pinnedBg} ${sponsorLevelToBorderClass(item.sponsorLevel)}`}
            >
              {/* 徽标列 - 使用 BadgeCell 统一渲染 */}
              {hasBadges && (
                <td className="px-2 py-1.5">
                  {hasItemBadges ? (
                    <BadgeCell
                      item={item}
                      showCategoryTag={showCategoryTag}
                      showSponsor={showSponsor}
                      showRisk={true}
                    />
                  ) : null}
                </td>
              )}
              {/* 服务商列（合并赞助者，紧凑两行布局） */}
              {showProvider && (
                <td className="px-2 py-1.5">
                  <div className="flex flex-col gap-0">
                    <span className="font-medium text-primary text-sm leading-tight">
                      <ExternalLink href={item.providerUrl} compact requireConfirm>{item.providerName}</ExternalLink>
                    </span>
                    {showSponsor && item.sponsor && (
                      <span className="text-[10px] text-muted leading-tight -mt-1">
                        <ExternalLink href={item.sponsorUrl} compact>{item.sponsor}</ExternalLink>
                      </span>
                    )}
                  </div>
                </td>
              )}
              <td className="px-2 py-1.5">
                <span
                  className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-mono border ${
                    item.serviceType === 'cc'
                      ? 'border-service-cc text-service-cc bg-service-cc'
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
                  {item.serviceType.toUpperCase()}
                </span>
              </td>
              <td className="px-2 py-2 text-secondary text-xs">
                {item.channel || '-'}
              </td>
              <td className="px-2 py-2 font-mono text-xs whitespace-nowrap">
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
              <td className="px-2 py-2 font-mono text-xs text-secondary whitespace-nowrap">
                {item.listedDays != null ? `${item.listedDays}d` : '-'}
              </td>
              <td className="px-2 py-1.5">
                <div className="flex items-center gap-1.5 whitespace-nowrap">
                  <StatusDot status={item.currentStatus} size="sm" />
                  <span className={STATUS[item.currentStatus].text}>
                    {STATUS[item.currentStatus].label}
                  </span>
                </div>
              </td>
              <td className="px-2 py-2 font-mono font-bold whitespace-nowrap">
                <span style={{ color: availabilityToColor(item.uptime) }}>
                  {item.uptime >= 0 ? `${item.uptime}%` : '--'}
                </span>
              </td>
              <td className="px-2 py-1.5">
                {item.lastCheckTimestamp ? (
                  <div className="text-xs text-secondary font-mono flex flex-col gap-0.5">
                    <span>{new Date(item.lastCheckTimestamp * 1000).toLocaleString(i18n.language, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })}</span>
                    {item.lastCheckLatency !== undefined && (
                      <span
                        className="text-[10px] font-mono"
                        style={{ color: latencyToColor(item.lastCheckLatency, slowLatencyMs) }}
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
                  {item.history.map((point, idx) => (
                    <HeatmapBlock
                      key={idx}
                      point={point}
                      width={`${100 / item.history.length}%`}
                      height="h-full"
                      onHover={onBlockHover}
                      onLeave={onBlockLeave}
                      isMobile={false}
                    />
                  ))}
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
