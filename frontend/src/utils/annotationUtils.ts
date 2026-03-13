import type { SponsorLevel, ProcessedMonitorData } from '../types';

/**
 * 赞助等级权重（用于置顶排序比较）
 */
export const SPONSOR_WEIGHTS: Record<SponsorLevel, number> = {
  core: 100,
  backbone: 80,
  beacon: 60,
  pulse: 40,
  signal: 20,
  public: 10,
};

/**
 * 检查监控项是否有任何注解（用于条件渲染）
 */
export function hasAnyAnnotation(
  item: ProcessedMonitorData,
  options: { enableAnnotations?: boolean } = {}
): boolean {
  const { enableAnnotations = true } = options;
  if (!enableAnnotations) return false;
  return (item.annotations?.length ?? 0) > 0;
}

/**
 * 检查数据列表中是否有任何项包含注解（用于显示注解列）
 */
export function hasAnyAnnotationInList(
  data: ProcessedMonitorData[],
  options: { enableAnnotations?: boolean } = {}
): boolean {
  return data.some(item => hasAnyAnnotation(item, options));
}
