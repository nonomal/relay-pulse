import { memo, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import type { MonitorLayer, ProcessedMonitorData } from '../types';
import { STATUS_MAP } from '../types';
import { availabilityToStyle } from '../utils/color';

type HeatmapPoint = ProcessedMonitorData['history'][number];

interface LayeredHeatmapBlockProps {
  /** 所有层的数据 */
  layers: MonitorLayer[];
  /** 时间点索引 */
  timeIndex: number;
  /** 宽度（CSS 字符串） */
  width: string;
  /** 总高度（CSS 类名，如 'h-5'），将平均分配给所有层 */
  height?: string;
  /** 悬停回调 */
  onHover: (e: React.MouseEvent<HTMLDivElement>, point: HeatmapPoint) => void;
  /** 离开回调 */
  onLeave: () => void;
  /** 是否为移动端 */
  isMobile?: boolean;
  /** 慢延迟阈值（毫秒） */
  slowLatencyMs: number;
}

/**
 * 分层热力图块组件（Phase B）
 *
 * 垂直堆叠显示多个 layer 的同一时间点数据
 * - 每层占一个子块，父层在上，子层在下
 * - 总高度固定，N 层时每层高度 = 总高度 / N
 * - 悬停某层时，Tooltip 显示该层的数据
 */
export const LayeredHeatmapBlock = memo(function LayeredHeatmapBlock({
  layers,
  timeIndex,
  width,
  height = 'h-5',
  onHover,
  onLeave,
  isMobile = false,
  slowLatencyMs,
}: LayeredHeatmapBlockProps) {
  const { t } = useTranslation();

  // 按 layer_order 排序（父层在上）
  const sortedLayers = useMemo(
    () => [...layers].sort((a, b) => a.layer_order - b.layer_order),
    [layers]
  );

  // 提取高度数值（如 'h-5' → 5 → 1.25rem → 20px）
  // Tailwind h-5 = 1.25rem = 20px，h-8 = 2rem = 32px
  const heightMap: Record<string, number> = {
    'h-full': 20, // 默认 20px
    'h-5': 20,
    'h-8': 32,
  };
  const totalHeightPx = heightMap[height] || 20;
  const layerHeightPx = totalHeightPx / sortedLayers.length;

  // 将时间点数据转换为 HeatmapPoint 格式
  const convertToHeatmapPoint = (layer: MonitorLayer, index: number): HeatmapPoint | null => {
    const timePoint = layer.timeline[index];
    if (!timePoint) return null;

    return {
      index,
      status: STATUS_MAP[timePoint.status] || 'UNAVAILABLE',
      timestamp: timePoint.time,
      timestampNum: timePoint.timestamp,
      latency: timePoint.latency,
      availability: timePoint.availability,
      statusCounts: timePoint.status_counts || {
        available: 0,
        degraded: 0,
        unavailable: 0,
        missing: 0,
        slow_latency: 0,
        rate_limit: 0,
        server_error: 0,
        client_error: 0,
        auth_error: 0,
        invalid_request: 0,
        network_error: 0,
        content_mismatch: 0,
      },
      slowLatencyMs,
      model: layer.model,
      layerOrder: layer.layer_order,
    };
  };

  // 处理点击事件（移动端）
  const handleLayerClick = (e: React.MouseEvent<HTMLDivElement>, layer: MonitorLayer) => {
    e.stopPropagation();
    const point = convertToHeatmapPoint(layer, timeIndex);
    if (point) {
      onHover(e, point);
    }
  };

  return (
    <div
      className="flex flex-col rounded-sm overflow-hidden"
      style={{ width }}
    >
      {sortedLayers.map((layer, layerIdx) => {
        const point = convertToHeatmapPoint(layer, timeIndex);
        if (!point) return null;

        const availabilityStyle = availabilityToStyle(point.availability);

        return (
          <div
            key={`${layer.model}-${layer.layer_order}`}
            role="button"
            tabIndex={0}
            className="transition-all duration-200 hover:brightness-110 active:brightness-105 cursor-pointer opacity-80 hover:opacity-100 active:opacity-100 relative hover:z-10"
            style={{
              height: `${layerHeightPx}px`,
              ...availabilityStyle,
              // 层间细微分隔（1px 深色边框，所有非第一层）
              ...(layerIdx > 0
                ? { borderTop: '1px solid rgba(0, 0, 0, 0.1)' }
                : {}),
            }}
            // 鼠标事件（仅桌面端）
            onMouseEnter={isMobile ? undefined : (e) => {
              const p = convertToHeatmapPoint(layer, timeIndex);
              if (p) onHover(e, p);
            }}
            onMouseLeave={isMobile ? undefined : onLeave}
            // 点击事件（移动端）
            onClick={(e) => handleLayerClick(e, layer)}
            // 键盘可访问性（仅桌面端）
            onFocus={isMobile ? undefined : (e) => {
              const p = convertToHeatmapPoint(layer, timeIndex);
              if (p) onHover(e as unknown as React.MouseEvent<HTMLDivElement>, p);
            }}
            onBlur={isMobile ? undefined : onLeave}
            // 无障碍标签
            aria-label={
              point.availability >= 0
                ? `${layer.model} (${t('multiModel.layer', { order: layer.layer_order })}): ${t('accessibility.uptimeBlock', { uptime: point.availability.toFixed(1) })}`
                : `${layer.model} (${t('multiModel.layer', { order: layer.layer_order })}): ${t('accessibility.noDataBlock')}`
            }
          />
        );
      })}
    </div>
  );
});
