import { useMemo, memo } from 'react';
import { Activity, Clock, Zap, Shield } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { StatusDot } from './StatusDot';
import { HeatmapBlock } from './HeatmapBlock';
import { ExternalLink } from './ExternalLink';
import { FavoriteButton } from './FavoriteButton';
import { getStatusConfig, getTimeRanges } from '../constants';
import { availabilityToColor, latencyToColor, sponsorLevelToCardBorderColor, sponsorLevelToPinnedBgClass } from '../utils/color';
import { formatPriceRatioStructured } from '../utils/format';
import { aggregateHeatmap } from '../utils/heatmapAggregator';
import { getServiceIconComponent } from './ServiceIcon';
import { BadgeCell } from './badges';
import { hasAnyBadge } from '../utils/badgeUtils';
import type { ProcessedMonitorData } from '../types';

type HistoryPoint = ProcessedMonitorData['history'][number];

// ServiceIcon 模块级缓存，与 StatusTable 保持一致
const serviceIconCache = new Map<string, ReturnType<typeof getServiceIconComponent>>();
const getCachedServiceIcon = (serviceType: string) => {
  if (!serviceIconCache.has(serviceType)) {
    serviceIconCache.set(serviceType, getServiceIconComponent(serviceType));
  }
  return serviceIconCache.get(serviceType);
};

interface StatusCardProps {
  item: ProcessedMonitorData;
  timeRange: string;
  slowLatencyMs: number;
  enableBadges?: boolean;      // 徽标系统总开关，默认 true
  showCategoryTag?: boolean; // 是否显示分类标签（推荐/公益），默认 true
  showProvider?: boolean;    // 是否显示服务商名称，默认 true
  showSponsor?: boolean;     // 是否显示赞助者信息，默认 true
  isFavorite?: (id: string) => boolean;  // 检查是否收藏
  onToggleFavorite?: (id: string) => void;  // 切换收藏状态
  onBlockHover: (e: React.MouseEvent<HTMLDivElement>, point: HistoryPoint) => void;
  onBlockLeave: () => void;
}

function StatusCardComponent({
  item,
  timeRange,
  slowLatencyMs,
  enableBadges = true,
  showCategoryTag = true,
  showProvider = true,
  showSponsor = true,
  isFavorite,
  onToggleFavorite,
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
  const ServiceIcon = getCachedServiceIcon(item.serviceType);

  // 检查是否有徽标需要显示
  const hasItemBadges = hasAnyBadge(item, { enableBadges, showCategoryTag, showSponsor, showRisk: true });

  // 卡片左边框颜色（仅基于赞助级别，置顶改用背景色）
  const borderColor = sponsorLevelToCardBorderColor(item.sponsorLevel);

  // 是否显示左边框（仅基于赞助级别）
  const hasLeftBorder = !!item.sponsorLevel;

  // 置顶项使用对应徽标颜色的极淡背景色
  const pinnedBgClass = item.pinned ? sponsorLevelToPinnedBgClass(item.sponsorLevel) : '';
  const baseBgClass = pinnedBgClass || 'bg-surface/60';

  return (
    <div
      className={`group relative ${baseBgClass} border border-default hover:border-accent/30 ${hasLeftBorder ? 'rounded-l-sm border-l-2' : 'rounded-l-2xl'} rounded-r-2xl p-4 sm:p-6 transition-all duration-300 hover:shadow-accent-lg backdrop-blur-sm overflow-hidden`}
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
          <div className="w-10 h-10 sm:w-12 sm:h-12 flex-shrink-0 rounded-xl bg-elevated flex items-center justify-center border border-default group-hover:border-strong transition-colors text-primary">
            {ServiceIcon ? (
              <ServiceIcon className="w-5 h-5 sm:w-6 sm:h-6" />
            ) : item.serviceType === 'cc' ? (
              <Zap className="text-service-cc" size={20} />
            ) : (
              <Shield className="text-service-cx" size={20} />
            )}
          </div>
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 flex-wrap">
              {showProvider && (
                <h3 className="text-base sm:text-lg font-bold text-primary">
                  <ExternalLink href={item.providerUrl} requireConfirm>{item.providerName}</ExternalLink>
                </h3>
              )}
              {/* 收藏按钮 */}
              {isFavorite && onToggleFavorite && (
                <FavoriteButton
                  isFavorite={isFavorite(item.id)}
                  onToggle={() => onToggleFavorite(item.id)}
                  size={14}
                  inline
                />
              )}
              <span
                className={`px-2 py-0.5 rounded text-[10px] font-mono border flex-shrink-0 ${
                  item.serviceType === 'cc'
                    ? 'border-service-cc text-service-cc bg-service-cc'
                    : 'border-service-cx text-service-cx bg-service-cx'
                }`}
              >
                {item.serviceType.toUpperCase()}
              </span>
            </div>
            <div className="flex items-center gap-3 mt-1 text-xs font-mono flex-wrap">
              <span className="flex items-center gap-1">
                <Activity size={12} className="text-secondary" />
                <span style={{ color: availabilityToColor(item.uptime) }}>
                  {t('card.uptime')} {item.uptime >= 0 ? `${item.uptime}%` : '--'}
                </span>
              </span>
              {(item.priceMin != null || item.priceMax != null) && (() => {
                const priceData = formatPriceRatioStructured(item.priceMin, item.priceMax);
                if (!priceData) return null;
                return (
                  <span className="text-secondary">
                    {t('table.headers.priceRatio')}: <span className="text-secondary">{priceData.base}</span>
                    {priceData.sub && <span className="text-muted text-[10px] ml-0.5">({priceData.sub})</span>}
                  </span>
                );
              })()}
              {item.listedDays != null && (
                <span className="text-secondary">
                  {t('table.headers.listedDays')}: <span className="text-secondary">{item.listedDays}d</span>
                </span>
              )}
            </div>
          </div>
        </div>

        {/* 右侧：状态 + 时间 */}
        <div className="flex sm:flex-col items-start sm:items-end gap-2 sm:gap-1.5">
          {/* 状态徽章 */}
          <div className="flex items-center gap-2 px-3 py-1.5 sm:py-1 rounded-full bg-elevated border border-default">
            <StatusDot status={item.currentStatus} />
            <span className={`text-xs font-bold ${STATUS[item.currentStatus].text}`}>
              {STATUS[item.currentStatus].label}
            </span>
          </div>

          {/* 最后监测时间 */}
          {item.lastCheckTimestamp && (
            <div className="text-[10px] text-muted font-mono flex flex-col items-start sm:items-end gap-0.5">
              <span className="whitespace-nowrap">
                {new Date(item.lastCheckTimestamp * 1000).toLocaleString(i18n.language, {
                  month: '2-digit',
                  day: '2-digit',
                  hour: '2-digit',
                  minute: '2-digit',
                })}
              </span>
              {item.lastCheckLatency !== undefined && (
                <span style={{ color: item.currentStatus === 'UNAVAILABLE' ? 'hsl(var(--text-muted))' : latencyToColor(item.lastCheckLatency, slowLatencyMs) }}>
                  {item.lastCheckLatency}ms
                </span>
              )}
            </div>
          )}
        </div>
      </div>

      {/* 热力图 */}
      <div>
        <div className="flex justify-between text-xs text-muted mb-2">
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
              isMobile={false}
            />
          ))}
        </div>

        {/* 移动端提示：点击查看详情 */}
        {aggregatedHistory.length < item.history.length && (
          <div className="mt-2 text-[10px] text-muted text-center sm:hidden">
            {t('table.heatmapHint', { from: item.history.length, to: aggregatedHistory.length })}
          </div>
        )}
      </div>
    </div>
  );
}

export const StatusCard = memo(StatusCardComponent);
