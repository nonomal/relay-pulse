import { useMemo } from 'react';
import { Activity, Clock, Zap, Shield } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { StatusDot } from './StatusDot';
import { HeatmapBlock } from './HeatmapBlock';
import { ExternalLink } from './ExternalLink';
import { getStatusConfig, getTimeRanges } from '../constants';
import { availabilityToColor, latencyToColor, sponsorLevelToCardBorderColor } from '../utils/color';
import { aggregateHeatmap } from '../utils/heatmapAggregator';
import { getServiceIconComponent } from './ServiceIcon';
import { BadgeCell } from './badges';
import { hasAnyBadge } from '../utils/badgeUtils';
import type { ProcessedMonitorData } from '../types';

type HistoryPoint = ProcessedMonitorData['history'][number];

interface StatusCardProps {
  item: ProcessedMonitorData;
  timeRange: string;
  slowLatencyMs: number;
  showCategoryTag?: boolean; // 是否显示分类标签（推荐/公益），默认 true
  showProvider?: boolean;    // 是否显示服务商名称，默认 true
  showSponsor?: boolean;     // 是否显示赞助者信息，默认 true
  onBlockHover: (e: React.MouseEvent<HTMLDivElement>, point: HistoryPoint) => void;
  onBlockLeave: () => void;
}

export function StatusCard({
  item,
  timeRange,
  slowLatencyMs,
  showCategoryTag = true,
  showProvider = true,
  showSponsor = true,
  onBlockHover,
  onBlockLeave
}: StatusCardProps) {
  const { t, i18n } = useTranslation();

  // 聚合热力图数据（移动端）
  const aggregatedHistory = useMemo(
    () => aggregateHeatmap(item.history, 50),
    [item.history]
  );

  const STATUS = getStatusConfig(t);
  const currentTimeRange = getTimeRanges(t).find((r) => r.id === timeRange);
  const ServiceIcon = getServiceIconComponent(item.serviceType);

  // 检查是否有徽标需要显示
  const hasItemBadges = hasAnyBadge(item, { showCategoryTag, showSponsor, showRisk: true });

  // 卡片左边框颜色（内联样式）
  const borderColor = sponsorLevelToCardBorderColor(item.sponsorLevel);

  return (
    <div
      className={`group relative bg-slate-900/60 border border-slate-800 hover:border-cyan-500/30 ${item.sponsorLevel ? 'rounded-l-sm border-l-2' : 'rounded-l-2xl'} rounded-r-2xl p-4 sm:p-6 transition-all duration-300 hover:shadow-[0_0_30px_rgba(6,182,212,0.1)] backdrop-blur-sm overflow-hidden`}
      style={borderColor ? { borderLeftColor: borderColor } : undefined}
    >
      {/* 徽标行 - 仅在有徽标时显示 */}
      {hasItemBadges && (
        <div className="mb-4">
          <BadgeCell
            item={item}
            showCategoryTag={showCategoryTag}
            showSponsor={showSponsor}
            showRisk={true}
          />
        </div>
      )}

      {/* 头部信息 - 使用 Grid 布局响应式 */}
      <div className="grid grid-cols-1 sm:grid-cols-[1fr_auto] gap-4 mb-6">
        {/* 左侧：图标 + 服务信息 */}
        <div className="flex gap-3 sm:gap-4 items-start sm:items-center">
          <div className="w-10 h-10 sm:w-12 sm:h-12 flex-shrink-0 rounded-xl bg-slate-800 flex items-center justify-center border border-slate-700 group-hover:border-slate-600 transition-colors text-slate-200">
            {ServiceIcon ? (
              <ServiceIcon className="w-5 h-5 sm:w-6 sm:h-6" />
            ) : item.serviceType === 'cc' ? (
              <Zap className="text-purple-400" size={20} />
            ) : (
              <Shield className="text-blue-400" size={20} />
            )}
          </div>
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 flex-wrap">
              {showProvider && (
                <h3 className="text-base sm:text-lg font-bold text-slate-100">
                  <ExternalLink href={item.providerUrl}>{item.providerName}</ExternalLink>
                </h3>
              )}
              <span
                className={`px-2 py-0.5 rounded text-[10px] font-mono border flex-shrink-0 ${
                  item.serviceType === 'cc'
                    ? 'border-purple-500/30 text-purple-300 bg-purple-500/10'
                    : 'border-blue-500/30 text-blue-300 bg-blue-500/10'
                }`}
              >
                {item.serviceType.toUpperCase()}
              </span>
            </div>
            <div className="flex items-center gap-2 mt-1 text-xs font-mono">
              <Activity size={12} className="text-slate-400" />
              <span style={{ color: availabilityToColor(item.uptime) }}>
                {t('card.uptime')} {item.uptime >= 0 ? `${item.uptime}%` : '--'}
              </span>
            </div>
          </div>
        </div>

        {/* 右侧：状态 + 时间 */}
        <div className="flex sm:flex-col items-start sm:items-end gap-2 sm:gap-1.5">
          {/* 状态徽章 */}
          <div className="flex items-center gap-2 px-3 py-1.5 sm:py-1 rounded-full bg-slate-800 border border-slate-700">
            <StatusDot status={item.currentStatus} />
            <span className={`text-xs font-bold ${STATUS[item.currentStatus].text}`}>
              {STATUS[item.currentStatus].label}
            </span>
          </div>

          {/* 最后检测时间 */}
          {item.lastCheckTimestamp && (
            <div className="text-[10px] text-slate-500 font-mono flex flex-col items-start sm:items-end gap-0.5">
              <span className="whitespace-nowrap">
                {new Date(item.lastCheckTimestamp * 1000).toLocaleString(i18n.language, {
                  month: '2-digit',
                  day: '2-digit',
                  hour: '2-digit',
                  minute: '2-digit',
                })}
              </span>
              {item.lastCheckLatency !== undefined && (
                <span style={{ color: latencyToColor(item.lastCheckLatency, slowLatencyMs) }}>
                  {item.lastCheckLatency}ms
                </span>
              )}
            </div>
          )}
        </div>
      </div>

      {/* 热力图 */}
      <div>
        <div className="flex justify-between text-xs text-slate-500 mb-2">
          <span className="flex items-center gap-1">
            <Clock size={12} /> {currentTimeRange?.label || timeRange}
          </span>
          <span>{t('common.now')}</span>
        </div>
        <div className="flex gap-[3px] h-10 w-full">
          {aggregatedHistory.map((point, idx) => (
            <HeatmapBlock
              key={idx}
              point={point}
              width={`${100 / aggregatedHistory.length}%`}
              onHover={onBlockHover}
              onLeave={onBlockLeave}
            />
          ))}
        </div>

        {/* 移动端提示：点击查看详情 */}
        {aggregatedHistory.length < item.history.length && (
          <div className="mt-2 text-[10px] text-slate-600 text-center sm:hidden">
            {t('table.heatmapHint', { from: item.history.length, to: aggregatedHistory.length })}
          </div>
        )}
      </div>
    </div>
  );
}
