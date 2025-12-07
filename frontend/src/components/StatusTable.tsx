import { useState, useEffect, useMemo } from 'react';
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
import { formatPriceRatio } from '../utils/format';
import { getServiceIconComponent } from './ServiceIcon';
import type { ProcessedMonitorData, SortConfig } from '../types';

type HistoryPoint = ProcessedMonitorData['history'][number];

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
  const ServiceIcon = getServiceIconComponent(item.serviceType);

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
  const baseBgClass = pinnedBgClass || 'bg-slate-900/60';

  return (
    <div
      className={`${baseBgClass} border border-slate-800 rounded-r-xl ${hasLeftBorder ? 'rounded-l-sm border-l-2' : 'rounded-l-xl'} p-3 space-y-2`}
      style={borderColor ? { borderLeftColor: borderColor } : undefined}
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
          <div className="w-8 h-8 flex-shrink-0 rounded-lg bg-slate-800 flex items-center justify-center border border-slate-700 text-slate-200">
            {ServiceIcon ? (
              <ServiceIcon className="w-4 h-4" />
            ) : item.serviceType === 'cc' ? (
              <Zap className="text-purple-400" size={14} />
            ) : (
              <Shield className="text-blue-400" size={14} />
            )}
          </div>

          {/* 服务商名称 */}
          <div className="min-w-0 flex-1">
            {showProvider && (
              <span className="font-semibold text-slate-100 truncate text-sm leading-none block">
                <ExternalLink href={item.providerUrl} compact requireConfirm>{item.providerName}</ExternalLink>
              </span>
            )}
            <div className="flex items-center gap-2 mt-1 text-xs text-slate-400">
              {/* 赞助者（放在服务类型前） */}
              {showSponsor && item.sponsor && (
                <span className="text-[10px] text-slate-500 truncate max-w-[80px]">
                  <ExternalLink href={item.sponsorUrl} compact>{item.sponsor}</ExternalLink>
                </span>
              )}
              <span
                className={`px-1.5 py-0.5 rounded text-[10px] font-mono border flex-shrink-0 ${
                  item.serviceType === 'cc'
                    ? 'border-purple-500/30 text-purple-300 bg-purple-500/10'
                    : 'border-blue-500/30 text-blue-300 bg-blue-500/10'
                }`}
              >
                {item.serviceType.toUpperCase()}
              </span>
              {item.channel && (
                <span className="text-slate-500 truncate">{item.channel}</span>
              )}
            </div>
          </div>
        </div>

        {/* 状态、可用率、时间和延迟 */}
        <div className="flex flex-col items-end gap-1 flex-shrink-0">
          <div className="flex items-center gap-1.5 px-2 py-1 rounded-full bg-slate-800 border border-slate-700">
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
          <div className="flex items-center gap-2 text-[10px] text-slate-500 font-mono">
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
              <span style={{ color: latencyToColor(item.lastCheckLatency, slowLatencyMs) }}>
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
    { key: 'serviceType', label: t('table.sorting.service') },
    { key: 'priceRatio', label: t('table.sorting.priceRatio') },
    { key: 'listedDays', label: t('table.sorting.listedDays') },
  ];

  return (
    <div className="flex items-center gap-2 mb-2 overflow-x-auto pb-2">
      <span className="text-xs text-slate-500 flex-shrink-0">{t('controls.sortBy')}</span>
      {sortOptions.map((option) => {
        // 初始状态下不高亮任何排序按钮
        const isActive = !isInitialSort && sortConfig.key === option.key;
        return (
          <button
            key={option.key}
            onClick={() => onSort(option.key)}
            className={`flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-xs font-medium transition-colors flex-shrink-0 ${
              isActive
                ? 'bg-cyan-500/20 text-cyan-300 border border-cyan-500/30'
                : 'bg-slate-800 text-slate-400 border border-slate-700 hover:text-slate-200'
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

export function StatusTable({
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
      <ArrowUp size={14} className="text-cyan-400 ml-1" />
    ) : (
      <ArrowDown size={14} className="text-cyan-400 ml-1" />
    );
  };

  const currentTimeRange = getTimeRanges(t).find((r) => r.id === timeRange);

  // 移动端：卡片列表视图
  if (isMobile) {
    return (
      <div>
        <MobileSortMenu sortConfig={sortConfig} isInitialSort={isInitialSort} onSort={onSort} />
        <div className="space-y-2">
          {data.map((item) => (
            <MobileListItem
              key={item.id}
              item={item}
              slowLatencyMs={slowLatencyMs}
              showCategoryTag={showCategoryTag}
              showProvider={showProvider}
              showSponsor={showSponsor}
              onBlockHover={onBlockHover}
              onBlockLeave={onBlockLeave}
            />
          ))}
        </div>
      </div>
    );
  }

  // 检查是否有任何徽标需要显示
  const hasBadges = hasAnyBadgeInList(data, { showCategoryTag, showSponsor, showRisk: true });

  // 桌面端：表格视图
  return (
    <div className="overflow-x-auto rounded-2xl border border-slate-800/50 shadow-xl">
      <table className="w-full text-left border-collapse bg-slate-900/40 backdrop-blur-sm">
        <thead>
          <tr className="border-b border-slate-700/50 text-slate-400 text-xs uppercase tracking-wider">
            {/* 徽标列 - 仅在有徽标时显示，可排序 */}
            {hasBadges && (
              <th
                className="px-2 py-3 font-medium w-12 cursor-pointer hover:text-cyan-400 transition-colors"
                onClick={() => onSort('badgeScore')}
              >
                <div className="flex items-center">
                  {t('table.headers.badge')} <SortIcon columnKey="badgeScore" />
                </div>
              </th>
            )}
            {/* 服务商列（合并赞助者） */}
            {showProvider && (
              <th
                className="px-3 py-3 font-medium cursor-pointer hover:text-cyan-400 transition-colors"
                onClick={() => onSort('providerName')}
              >
                <div className="flex items-center">
                  {t('table.headers.provider')} <SortIcon columnKey="providerName" />
                </div>
              </th>
            )}
            <th
              className="px-2 py-3 font-medium cursor-pointer hover:text-cyan-400 transition-colors whitespace-nowrap"
              onClick={() => onSort('serviceType')}
            >
              <div className="flex items-center">
                {t('table.headers.service')} <SortIcon columnKey="serviceType" />
              </div>
            </th>
            <th
              className="px-2 py-3 font-medium cursor-pointer hover:text-cyan-400 transition-colors"
              onClick={() => onSort('channel')}
            >
              <div className="flex items-center">
                {t('table.headers.channel')} <SortIcon columnKey="channel" />
              </div>
            </th>
            <th
              className="px-2 py-3 font-medium cursor-pointer hover:text-cyan-400 transition-colors whitespace-nowrap"
              onClick={() => onSort('priceRatio')}
            >
              <div className="flex items-center">
                {t('table.headers.priceRatio')} <SortIcon columnKey="priceRatio" />
              </div>
            </th>
            <th
              className="px-2 py-3 font-medium cursor-pointer hover:text-cyan-400 transition-colors whitespace-nowrap"
              onClick={() => onSort('listedDays')}
            >
              <div className="flex items-center">
                {t('table.headers.listedDays')} <SortIcon columnKey="listedDays" />
              </div>
            </th>
            <th
              className="px-2 py-3 font-medium cursor-pointer hover:text-cyan-400 transition-colors whitespace-nowrap"
              onClick={() => onSort('currentStatus')}
            >
              <div className="flex items-center">
                {t('table.headers.status')} <SortIcon columnKey="currentStatus" />
              </div>
            </th>
            <th
              className="px-2 py-3 font-medium cursor-pointer hover:text-cyan-400 transition-colors whitespace-nowrap"
              onClick={() => onSort('uptime')}
            >
              <div className="flex items-center">
                {t('table.headers.uptime')} <SortIcon columnKey="uptime" />
              </div>
            </th>
            <th className="px-2 py-3 font-medium whitespace-nowrap">{t('table.headers.lastCheck')}</th>
            <th className="px-2 py-3 font-medium flex-1 min-w-[180px]">
              <div className="flex items-center gap-2">
                {t('table.headers.trend')}
                <span className="text-[10px] normal-case opacity-50 border border-slate-700 px-1 rounded">
                  {currentTimeRange?.label}
                </span>
              </div>
            </th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-800/50 text-sm">
          {data.map((item) => {
            const ServiceIcon = getServiceIconComponent(item.serviceType);
            const hasItemBadges = hasAnyBadge(item, { showCategoryTag, showSponsor, showRisk: true });
            const pinnedBg = item.pinned ? sponsorLevelToPinnedBgClass(item.sponsorLevel) : '';
            return (
            <tr
              key={item.id}
              className={`group hover:bg-slate-800/40 transition-[background-color,color] ${pinnedBg} ${sponsorLevelToBorderClass(item.sponsorLevel)}`}
            >
              {/* 徽标列 - 使用 BadgeCell 统一渲染 */}
              {hasBadges && (
                <td className="px-2 py-2">
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
                <td className="px-3 py-2">
                  <div className="flex flex-col">
                    <span className="font-medium text-slate-200 text-sm leading-none">
                      <ExternalLink href={item.providerUrl} compact requireConfirm>{item.providerName}</ExternalLink>
                    </span>
                    {showSponsor && item.sponsor && (
                      <span className="text-[10px] text-slate-500 leading-none">
                        <ExternalLink href={item.sponsorUrl} compact>{item.sponsor}</ExternalLink>
                      </span>
                    )}
                  </div>
                </td>
              )}
              <td className="px-2 py-2">
                <span
                  className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-mono border ${
                    item.serviceType === 'cc'
                      ? 'border-purple-500/30 text-purple-300 bg-purple-500/10'
                      : 'border-blue-500/30 text-blue-300 bg-blue-500/10'
                  }`}
                >
                  {ServiceIcon ? (
                    <ServiceIcon className="w-3.5 h-3.5 mr-1" />
                  ) : (
                    <>
                      {item.serviceType === 'cc' && <Zap size={10} className="mr-1" />}
                      {item.serviceType === 'cx' && <Shield size={10} className="mr-1" />}
                    </>
                  )}
                  {item.serviceType.toUpperCase()}
                </span>
              </td>
              <td className="px-2 py-2 text-slate-400 text-xs">
                {item.channel || '-'}
              </td>
              <td className="px-2 py-2 font-mono text-xs text-slate-300 whitespace-nowrap">
                {formatPriceRatio(item.priceRatio, item.priceVariance)}
              </td>
              <td className="px-2 py-2 font-mono text-xs text-slate-400 whitespace-nowrap">
                {item.listedDays != null ? `${item.listedDays}d` : '-'}
              </td>
              <td className="px-2 py-2">
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
              <td className="px-2 py-2">
                {item.lastCheckTimestamp ? (
                  <div className="text-xs text-slate-400 font-mono flex flex-col gap-0.5">
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
                  <span className="text-slate-600 text-xs">-</span>
                )}
              </td>
              <td className="px-2 py-2 align-middle">
                <div className="flex items-center gap-[2px] h-6 w-full max-w-xs overflow-hidden rounded-sm">
                  {item.history.map((point, idx) => (
                    <HeatmapBlock
                      key={idx}
                      point={point}
                      width={`${100 / item.history.length}%`}
                      height="h-full"
                      onHover={onBlockHover}
                      onLeave={onBlockLeave}
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
