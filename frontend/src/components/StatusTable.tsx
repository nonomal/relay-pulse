import { useState, useEffect, useMemo } from 'react';
import { ArrowUpDown, ArrowUp, ArrowDown, Zap, Shield, ChevronDown, ChevronUp } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { StatusDot } from './StatusDot';
import { HeatmapBlock } from './HeatmapBlock';
import { ExternalLink } from './ExternalLink';
import { getStatusConfig, getTimeRanges } from '../constants';
import { availabilityToColor, latencyToColor } from '../utils/color';
import { aggregateHeatmap } from '../utils/heatmapAggregator';
import { createMediaQueryEffect } from '../utils/mediaQuery';
import { getServiceIconComponent } from './ServiceIcon';
import type { ProcessedMonitorData, SortConfig } from '../types';

type HistoryPoint = ProcessedMonitorData['history'][number];

interface StatusTableProps {
  data: ProcessedMonitorData[];
  sortConfig: SortConfig;
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
  const [expanded, setExpanded] = useState(false);
  const STATUS = getStatusConfig(t);
  const ServiceIcon = getServiceIconComponent(item.serviceType);

  // 聚合热力图数据
  const aggregatedHistory = useMemo(
    () => aggregateHeatmap(item.history, 30),
    [item.history]
  );

  return (
    <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-4 space-y-3">
      {/* 主要信息行 */}
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-center gap-3 min-w-0 flex-1">
          {/* 服务图标 */}
          <div className="w-10 h-10 flex-shrink-0 rounded-lg bg-slate-800 flex items-center justify-center border border-slate-700 text-slate-200">
            {ServiceIcon ? (
              <ServiceIcon className="w-5 h-5" />
            ) : item.serviceType === 'cc' ? (
              <Zap className="text-purple-400" size={18} />
            ) : (
              <Shield className="text-blue-400" size={18} />
            )}
          </div>

          {/* 服务商名称 */}
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2 flex-wrap">
              {showProvider && (
                <span className="font-semibold text-slate-100 truncate">
                  <ExternalLink href={item.providerUrl}>{item.providerName}</ExternalLink>
                </span>
              )}
              {/* Category 标签 - 可通过 showCategoryTag 控制显示 */}
              {showCategoryTag && (
                <span
                  className={`flex-shrink-0 px-1.5 py-0.5 rounded text-[10px] font-semibold uppercase ${
                    item.category === 'commercial'
                      ? 'text-emerald-300 bg-emerald-500/10 border border-emerald-500/30'
                      : 'text-cyan-300 bg-cyan-500/10 border border-cyan-500/30'
                  }`}
                >
                  {item.category === 'commercial' ? t('table.categoryShort.promoted') : t('table.categoryShort.charity')}
                </span>
              )}
            </div>
            <div className="flex items-center gap-2 mt-1 text-xs text-slate-400">
              <span
                className={`px-1.5 py-0.5 rounded text-[10px] font-mono border ${
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

        {/* 状态和可用率 */}
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
        </div>
      </div>

      {/* 热力图 */}
      <div className="flex items-center gap-[2px] h-6 w-full overflow-hidden rounded-sm">
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

      {/* 展开/收起按钮 */}
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center justify-center gap-1 py-1.5 text-xs text-slate-500 hover:text-slate-300 transition-colors"
      >
        {expanded ? (
          <>
            <ChevronUp size={14} />
            {t('table.collapseDetails')}
          </>
        ) : (
          <>
            <ChevronDown size={14} />
            {t('table.expandDetails')}
          </>
        )}
      </button>

      {/* 展开的详细信息 */}
      {expanded && (
        <div className="pt-3 border-t border-slate-800 space-y-2 text-xs">
          {showSponsor && (
            <div className="flex justify-between">
              <span className="text-slate-500">{t('common.sponsor')}</span>
              <span className="text-slate-300">
                <ExternalLink href={item.sponsorUrl}>{item.sponsor}</ExternalLink>
              </span>
            </div>
          )}
          <div className="flex justify-between">
            <span className="text-slate-500">{t('common.channel')}</span>
            <span className="text-slate-300">{item.channel || '-'}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-slate-500">{t('common.lastCheck')}</span>
            <span className="text-slate-300 font-mono">
              {item.lastCheckTimestamp
                ? new Date(item.lastCheckTimestamp * 1000).toLocaleString(i18n.language, {
                    month: '2-digit',
                    day: '2-digit',
                    hour: '2-digit',
                    minute: '2-digit',
                  })
                : '-'}
            </span>
          </div>
          {item.lastCheckLatency !== undefined && (
            <div className="flex justify-between">
              <span className="text-slate-500">{t('common.latency')}</span>
              <span
                className="font-mono"
                style={{ color: latencyToColor(item.lastCheckLatency, slowLatencyMs) }}
              >
                {item.lastCheckLatency}ms
              </span>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// 移动端排序菜单
function MobileSortMenu({
  sortConfig,
  onSort,
}: {
  sortConfig: SortConfig;
  onSort: (key: string) => void;
}) {
  const { t } = useTranslation();

  const sortOptions = [
    { key: 'providerName', label: t('table.sorting.provider') },
    { key: 'uptime', label: t('table.sorting.uptime') },
    { key: 'currentStatus', label: t('table.sorting.status') },
    { key: 'serviceType', label: t('table.sorting.service') },
  ];

  return (
    <div className="flex items-center gap-2 mb-4 overflow-x-auto pb-2">
      <span className="text-xs text-slate-500 flex-shrink-0">{t('controls.sortBy')}</span>
      {sortOptions.map((option) => (
        <button
          key={option.key}
          onClick={() => onSort(option.key)}
          className={`flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-xs font-medium transition-colors flex-shrink-0 ${
            sortConfig.key === option.key
              ? 'bg-cyan-500/20 text-cyan-300 border border-cyan-500/30'
              : 'bg-slate-800 text-slate-400 border border-slate-700 hover:text-slate-200'
          }`}
        >
          {option.label}
          {sortConfig.key === option.key && (
            sortConfig.direction === 'asc' ? (
              <ArrowUp size={12} />
            ) : (
              <ArrowDown size={12} />
            )
          )}
        </button>
      ))}
    </div>
  );
}

export function StatusTable({
  data,
  sortConfig,
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

  const SortIcon = ({ columnKey }: { columnKey: string }) => {
    if (sortConfig.key !== columnKey)
      return <ArrowUpDown size={14} className="opacity-30 ml-1" />;
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
        <MobileSortMenu sortConfig={sortConfig} onSort={onSort} />
        <div className="space-y-3">
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

  // 桌面端：表格视图
  return (
    <div className="overflow-x-auto rounded-2xl border border-slate-800/50 shadow-xl">
      <table className="w-full text-left border-collapse bg-slate-900/40 backdrop-blur-sm">
        <thead>
          <tr className="border-b border-slate-700/50 text-slate-400 text-xs uppercase tracking-wider">
            {showProvider && (
              <th
                className="p-4 font-medium cursor-pointer hover:text-cyan-400 transition-colors"
                onClick={() => onSort('providerName')}
              >
                <div className="flex items-center">
                  {t('table.headers.provider')} <SortIcon columnKey="providerName" />
                </div>
              </th>
            )}
            {showSponsor && (
              <th
                className="p-4 font-medium cursor-pointer hover:text-cyan-400 transition-colors"
                onClick={() => onSort('sponsor')}
              >
                <div className="flex items-center">
                  {t('table.headers.sponsor')} <SortIcon columnKey="sponsor" />
                </div>
              </th>
            )}
            <th
              className="p-4 font-medium cursor-pointer hover:text-cyan-400 transition-colors"
              onClick={() => onSort('serviceType')}
            >
              <div className="flex items-center">
                {t('table.headers.service')} <SortIcon columnKey="serviceType" />
              </div>
            </th>
            <th
              className="p-4 font-medium cursor-pointer hover:text-cyan-400 transition-colors"
              onClick={() => onSort('channel')}
            >
              <div className="flex items-center">
                {t('table.headers.channel')} <SortIcon columnKey="channel" />
              </div>
            </th>
            <th
              className="p-4 font-medium cursor-pointer hover:text-cyan-400 transition-colors"
              onClick={() => onSort('currentStatus')}
            >
              <div className="flex items-center">
                {t('table.headers.status')} <SortIcon columnKey="currentStatus" />
              </div>
            </th>
            <th
              className="p-4 font-medium cursor-pointer hover:text-cyan-400 transition-colors"
              onClick={() => onSort('uptime')}
            >
              <div className="flex items-center">
                {t('table.headers.uptime')} <SortIcon columnKey="uptime" />
              </div>
            </th>
            <th className="p-4 font-medium">{t('table.headers.lastCheck')}</th>
            <th className="p-4 font-medium w-1/3 min-w-[200px]">
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
            return (
            <tr
              key={item.id}
              className="group hover:bg-slate-800/40 transition-[background-color,color]"
            >
              {showProvider && (
                <td className="p-4 font-medium text-slate-200">
                  <div className="flex items-center gap-2">
                    <ExternalLink href={item.providerUrl}>{item.providerName}</ExternalLink>
                    {/* Category 标签 - 可通过 showCategoryTag 控制显示 */}
                    {showCategoryTag && (
                      <span
                        className={`inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-semibold uppercase tracking-wide ${
                          item.category === 'commercial'
                            ? 'text-emerald-300 bg-emerald-500/10 border border-emerald-500/30'
                            : 'text-cyan-300 bg-cyan-500/10 border border-cyan-500/30'
                        }`}
                        title={item.category === 'commercial' ? t('table.categoryLabels.promoted') : t('table.categoryLabels.charity')}
                      >
                        {item.category === 'commercial' ? t('table.categoryShort.promoted') : t('table.categoryShort.charity')}
                      </span>
                    )}
                  </div>
                </td>
              )}
              {showSponsor && (
                <td className="p-4 text-slate-300 text-sm">
                  <ExternalLink href={item.sponsorUrl}>{item.sponsor}</ExternalLink>
                </td>
              )}
              <td className="p-4">
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
              <td className="p-4 text-slate-400 text-xs">
                {item.channel || '-'}
              </td>
              <td className="p-4">
                <div className="flex items-center gap-2">
                  <StatusDot status={item.currentStatus} size="sm" />
                  <span className={STATUS[item.currentStatus].text}>
                    {STATUS[item.currentStatus].label}
                  </span>
                </div>
              </td>
              <td className="p-4 font-mono font-bold">
                <span style={{ color: availabilityToColor(item.uptime) }}>
                  {item.uptime >= 0 ? `${item.uptime}%` : '--'}
                </span>
              </td>
              <td className="p-4">
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
              <td className="p-4 align-middle">
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
