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

// 默认状态计数（用于缺失数据点）
const defaultStatusCounts: HeatmapPoint['statusCounts'] = {
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
};

/**
 * 分层热力图块组件（Phase B）
 *
 * 垂直堆叠显示多个 layer 的同一时间点数据
 * - 每层占一个子块，父层在上，子层在下
 * - 总高度固定，N 层时每层高度 = 总高度 / N
 * - 悬停某层时，Tooltip 显示该层的数据
 * - 当某层缺少该时间点数据时，显示灰色（无数据）
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

  // 参考时间点：当某层缺失该 index 时，用其他层的时间信息填充
  // 注意：必须在条件返回前调用所有 Hooks
  const referenceTimePoint = useMemo(() => {
    for (const l of sortedLayers) {
      const p = l.timeline[timeIndex];
      if (p) return p;
    }
    return undefined;
  }, [sortedLayers, timeIndex]);

  // 边界情况：空数组时不渲染
  if (sortedLayers.length === 0) {
    return null;
  }

  // 提取高度数值（如 'h-5' → 5 → 1.25rem → 20px）
  // Tailwind h-5 = 1.25rem = 20px，h-8 = 2rem = 32px
  const heightMap: Record<string, number> = {
    'h-full': 20, // 默认 20px
    'h-5': 20,
    'h-8': 32,
  };
  const totalHeightPx = heightMap[height] || 20;
  const gapPx = 2; // 层间间隙 2px
  const totalGapPx = (sortedLayers.length - 1) * gapPx;
  const layerHeightPx = (totalHeightPx - totalGapPx) / sortedLayers.length;

  // 将时间点数据转换为 HeatmapPoint 格式
  // 当该层缺少数据时，返回占位点（availability=-1 -> 灰色）而不是 null
  const convertToHeatmapPoint = (layer: MonitorLayer, index: number): HeatmapPoint => {
    const ownTimePoint = layer.timeline[index];
    const timePointForTimestamp = ownTimePoint ?? referenceTimePoint;

    // 极端兜底：所有层都没有该 index（理论上不该发生）
    if (!timePointForTimestamp) {
      return {
        index,
        status: 'MISSING',
        timestamp: '',
        timestampNum: 0,
        latency: 0,
        availability: -1,
        statusCounts: { ...defaultStatusCounts, missing: 1 },
        slowLatencyMs,
        model: layer.model,
        layerOrder: layer.layer_order,
      };
    }

    // 该层缺失该时间点：返回占位点（灰色），保持层数一致
    if (!ownTimePoint) {
      return {
        index,
        status: 'MISSING',
        timestamp: timePointForTimestamp.time,
        timestampNum: timePointForTimestamp.timestamp,
        latency: 0,
        availability: -1,
        statusCounts: { ...defaultStatusCounts, missing: 1 },
        slowLatencyMs,
        model: layer.model,
        layerOrder: layer.layer_order,
      };
    }

    // 正常情况：该层有数据
    // 使用展开运算符确保 statusCounts 所有字段都存在（防止后端返回不完整对象）
    return {
      index,
      status: STATUS_MAP[ownTimePoint.status] || 'UNAVAILABLE',
      timestamp: ownTimePoint.time,
      timestampNum: ownTimePoint.timestamp,
      latency: ownTimePoint.latency,
      availability: ownTimePoint.availability,
      statusCounts: { ...defaultStatusCounts, ...ownTimePoint.status_counts },
      slowLatencyMs,
      model: layer.model,
      layerOrder: layer.layer_order,
    };
  };

  // 处理点击事件（移动端）
  const handleLayerClick = (e: React.MouseEvent<HTMLDivElement>, layer: MonitorLayer) => {
    e.stopPropagation();
    const point = convertToHeatmapPoint(layer, timeIndex);
    // 只有有效数据才触发 hover（灰色块不弹 tooltip）
    if (point.availability >= 0) {
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
              // 层间间隙（非第一层加 marginTop，兼容 Safari ≤13）
              ...(layerIdx > 0 ? { marginTop: `${gapPx}px` } : {}),
            }}
            // 鼠标事件（仅桌面端，灰色块不弹 tooltip）
            onMouseEnter={isMobile ? undefined : (e) => {
              const p = convertToHeatmapPoint(layer, timeIndex);
              if (p.availability >= 0) onHover(e, p);
            }}
            onMouseLeave={isMobile ? undefined : onLeave}
            // 点击事件（移动端）
            onClick={(e) => handleLayerClick(e, layer)}
            // 键盘可访问性（仅桌面端）
            onFocus={isMobile ? undefined : (e) => {
              const p = convertToHeatmapPoint(layer, timeIndex);
              if (p.availability >= 0) onHover(e as unknown as React.MouseEvent<HTMLDivElement>, p);
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
