import { useTranslation } from 'react-i18next';
import type { MonitorLayer } from '../types';

interface MultiModelIndicatorProps {
  layers: MonitorLayer[];
  className?: string;
}

/**
 * 多模型标记组件
 *
 * 在多模型监测项的热力图前显示 "!" 标记
 * hover 时显示所有模型名称列表
 */
export function MultiModelIndicator({ layers, className = '' }: MultiModelIndicatorProps) {
  const { t } = useTranslation();

  if (layers.length === 0) return null;

  // 按 layer_order 排序（父层在前）
  const sortedLayers = [...layers].sort((a, b) => a.layer_order - b.layer_order);
  const modelNames = sortedLayers.map((layer) => layer.model).join(', ');

  return (
    <span
      className={`inline-flex items-center justify-center w-4 h-4 text-xs font-bold text-warning bg-warning/10 rounded-sm cursor-help ${className}`}
      title={t('multiModel.indicator.title', { models: modelNames })}
      aria-label={t('multiModel.indicator.ariaLabel', { count: layers.length, models: modelNames })}
    >
      !
    </span>
  );
}
