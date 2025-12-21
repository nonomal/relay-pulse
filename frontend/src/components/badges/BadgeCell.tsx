import type { ProcessedMonitorData } from '../../types';
import { SponsorBadge } from './SponsorBadge';
import { CategoryBadge } from './CategoryBadge';
import { RiskBadge } from './RiskBadge';
import { GenericBadge } from './GenericBadge';
import { FrequencyIndicator } from './FrequencyIndicator';

interface BadgeCellProps {
  item: ProcessedMonitorData;
  showCategoryTag?: boolean;  // 是否显示站点类型标签（仅公益站显示）
  showSponsor?: boolean;      // 是否显示赞助商徽标
  showRisk?: boolean;         // 是否显示风险徽标
  showGenericBadges?: boolean; // 是否显示通用徽标
  showFrequency?: boolean;    // 是否显示监测频率指示器
  className?: string;
  tooltipPlacement?: 'top' | 'bottom';  // tooltip 显示方向
}

/**
 * 徽标单元格组件 - 统一渲染所有徽标
 *
 * 渲染顺序：
 * 1. 站点类型标签（仅公益站显示蓝色「益」标签）
 * 2. 赞助商徽标（正向）
 * 3. 通用徽标（source/info/feature）
 * 4. 监测频率指示器
 * 5. 分隔符 | （仅在正负徽标都存在时显示）
 * 6. 风险徽标（负向，黄色警告）
 */
export function BadgeCell({
  item,
  showCategoryTag = true,
  showSponsor = true,
  showRisk = true,
  showGenericBadges = true,
  showFrequency = true,
  className = '',
  tooltipPlacement = 'top',
}: BadgeCellProps) {
  const hasSponsor = Boolean(item.sponsorLevel);
  const hasRisks = Boolean(item.risks?.length);
  const hasGenericBadges = Boolean(item.badges?.length);
  const isPublic = item.category === 'public';

  // 检查是否有正向徽标（公益站类型标签 + 赞助商 + 通用徽标 + 频率）
  // 注意：商业站不显示站点类型标签，所以只有公益站才算有站点类型徽标
  const hasPositiveBadges =
    (showCategoryTag && isPublic) ||
    (showSponsor && hasSponsor) ||
    (showGenericBadges && hasGenericBadges) ||
    (showFrequency && (item.intervalMs ?? 0) > 0);
  // 检查是否有负向徽标（风险）
  const hasNegativeBadges = showRisk && hasRisks;

  // 检查是否有任何徽标需要显示
  const hasAnyBadge = hasPositiveBadges || hasNegativeBadges;

  if (!hasAnyBadge) {
    return null;
  }

  return (
    <div className={`flex items-center gap-0.5 ${className}`}>
      {/* 正向徽标组 */}
      {/* 站点类型标签 - 仅公益站显示 */}
      {showCategoryTag && <CategoryBadge category={item.category} tooltipPlacement={tooltipPlacement} />}

      {/* 赞助商徽标 */}
      {showSponsor && hasSponsor && item.sponsorLevel && (
        <SponsorBadge level={item.sponsorLevel} tooltipPlacement={tooltipPlacement} />
      )}

      {/* 通用徽标 */}
      {showGenericBadges && hasGenericBadges && item.badges?.map((badge) => (
        <GenericBadge key={badge.id} badge={badge} tooltipPlacement={tooltipPlacement} />
      ))}

      {/* 监测频率指示器 */}
      {showFrequency && (item.intervalMs ?? 0) > 0 && (
        <FrequencyIndicator intervalMs={item.intervalMs ?? 0} tooltipPlacement={tooltipPlacement} />
      )}

      {/* 分隔符 - 仅在正负徽标都存在时显示 */}
      {hasPositiveBadges && hasNegativeBadges && (
        <span className="text-muted text-xs select-none mx-0.5">|</span>
      )}

      {/* 负向徽标组 */}
      {/* 风险徽标 - 可能有多个 */}
      {showRisk && hasRisks && item.risks?.map((risk, index) => (
        <RiskBadge key={index} risk={risk} tooltipPlacement={tooltipPlacement} />
      ))}
    </div>
  );
}
