import type { SponsorLevel, ProcessedMonitorData } from '../types';

/**
 * 徽标权重常量（前端常量，修改需发版）
 *
 * 正向徽标：分数越高越优先
 * 风险徽标：负向分数，降低优先级
 */

// 赞助商等级权重（正向）
export const SPONSOR_WEIGHTS: Record<SponsorLevel, number> = {
  enterprise: 100,  // 全球伙伴
  advanced: 50,     // 核心服务商
  basic: 20,        // 节点支持
};

// 风险徽标权重（负向）
export const RISK_WEIGHT = -50;

/**
 * 计算单个监控项的徽标综合分数
 *
 * 分数 = 赞助商等级权重 + 风险徽标权重总和
 * 公益站不参与排序计算
 *
 * @param item 监控数据项
 * @returns 综合分数（用于排序）
 */
export function calculateBadgeScore(item: ProcessedMonitorData): number {
  let score = 0;

  // 正向徽标：赞助商等级
  if (item.sponsorLevel) {
    score += SPONSOR_WEIGHTS[item.sponsorLevel] || 0;
  }

  // 风险徽标（负向）：每个风险徽标扣分
  if (item.risks?.length) {
    score += item.risks.length * RISK_WEIGHT;
  }

  return score;
}

/**
 * 检查监控项是否有任何徽标（用于条件渲染）
 *
 * @param item 监控数据项
 * @param options 选项
 * @returns 是否有徽标
 */
export function hasAnyBadge(
  item: ProcessedMonitorData,
  options: {
    showCategoryTag?: boolean;  // 是否显示站点类型标签（商业/公益）
    showSponsor?: boolean;      // 是否显示赞助商徽标
    showRisk?: boolean;         // 是否显示风险徽标
  } = {}
): boolean {
  const {
    showCategoryTag = true,
    showSponsor = true,
    showRisk = true,
  } = options;

  // showCategoryTag 时始终有徽标（所有项都有商业/公益类型）
  return Boolean(
    showCategoryTag ||
    (showSponsor && item.sponsorLevel) ||
    (showRisk && item.risks?.length)
  );
}

/**
 * 检查数据列表中是否有任何项包含徽标（用于显示徽标列）
 *
 * @param data 监控数据列表
 * @param options 选项
 * @returns 是否有任何徽标
 */
export function hasAnyBadgeInList(
  data: ProcessedMonitorData[],
  options: {
    showCategoryTag?: boolean;  // 是否显示站点类型标签（商业/公益）
    showSponsor?: boolean;      // 是否显示赞助商徽标
    showRisk?: boolean;         // 是否显示风险徽标
  } = {}
): boolean {
  return data.some(item => hasAnyBadge(item, options));
}
