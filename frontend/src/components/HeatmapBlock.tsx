import { memo, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import type { ProcessedMonitorData } from '../types';
import { availabilityToStyle } from '../utils/color';

// 直接使用 ProcessedMonitorData 中的 history 类型，确保字段完整性
type HeatmapPoint = ProcessedMonitorData['history'][number];

interface HeatmapBlockProps {
  point: HeatmapPoint;
  width: string;
  height?: string;
  onHover: (e: React.MouseEvent<HTMLDivElement>, point: HeatmapPoint) => void;
  onLeave: () => void;
  /** 是否为移动端，由父组件统一检测后传递，避免每个块独立监听 */
  isMobile?: boolean;
}

export const HeatmapBlock = memo(function HeatmapBlock({
  point,
  width,
  height = 'h-8',
  onHover,
  onLeave,
  isMobile = false,
}: HeatmapBlockProps) {
  const { t } = useTranslation();

  // 缓存样式计算，避免每次渲染都重新计算
  const availabilityStyle = useMemo(
    () => availabilityToStyle(point.availability),
    [point.availability]
  );

  // 处理点击事件（移动端使用点击而非触摸，避免滑动时误触发）
  const handleClick = (e: React.MouseEvent<HTMLDivElement>) => {
    // 阻止事件冒泡，避免触发父元素的点击
    e.stopPropagation();
    onHover(e, point);
  };

  return (
    <div
      role="button"
      tabIndex={0}
      className={`${height} rounded-sm transition-all duration-200 hover:scale-110 active:scale-105 hover:z-10 cursor-pointer opacity-80 hover:opacity-100 active:opacity-100`}
      style={{ width, ...availabilityStyle }}
      // 鼠标事件（仅桌面端，移动端禁用避免闪烁）
      onMouseEnter={isMobile ? undefined : (e) => onHover(e, point)}
      onMouseLeave={isMobile ? undefined : onLeave}
      // 点击事件（移动端需明确点击才触发，滑动不触发）
      onClick={handleClick}
      // 键盘可访问性（仅桌面端）
      onFocus={isMobile ? undefined : (e) => onHover(e as unknown as React.MouseEvent<HTMLDivElement>, point)}
      onBlur={isMobile ? undefined : onLeave}
      // 无障碍标签
      aria-label={
        point.availability >= 0
          ? t('accessibility.uptimeBlock', { uptime: point.availability.toFixed(1) })
          : t('accessibility.noDataBlock')
      }
    />
  );
});
