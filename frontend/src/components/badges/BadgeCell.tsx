import type { ProcessedMonitorData } from '../../types';
import { SponsorBadge } from './SponsorBadge';
import { PublicBadge } from './PublicBadge';
import { RiskBadge } from './RiskBadge';

interface BadgeCellProps {
  item: ProcessedMonitorData;
  showCategoryTag?: boolean;  // 是否显示公益站标签
  showSponsor?: boolean;      // 是否显示赞助商徽标
  showRisk?: boolean;         // 是否显示风险徽标
  className?: string;
}

/**
 * 徽标单元格组件 - 统一渲染所有徽标
 *
 * 渲染顺序：
 * 1. 公益站标签（蓝色）
 * 2. 赞助商徽标（正向）
 * 3. 风险徽标（负向，红色警告）
 */
export function BadgeCell({
  item,
  showCategoryTag = true,
  showSponsor = true,
  showRisk = true,
  className = '',
}: BadgeCellProps) {
  const isPublic = item.category === 'public';
  const hasSponsor = Boolean(item.sponsorLevel);
  const hasRisks = Boolean(item.risks?.length);

  // 检查是否有任何徽标需要显示
  const hasAnyBadge =
    (showCategoryTag && isPublic) ||
    (showSponsor && hasSponsor) ||
    (showRisk && hasRisks);

  if (!hasAnyBadge) {
    return null;
  }

  return (
    <div className={`flex items-center gap-1.5 ${className}`}>
      {/* 公益站标签 */}
      {showCategoryTag && isPublic && <PublicBadge />}

      {/* 赞助商徽标 */}
      {showSponsor && hasSponsor && item.sponsorLevel && (
        <SponsorBadge level={item.sponsorLevel} />
      )}

      {/* 风险徽标 - 可能有多个 */}
      {showRisk && hasRisks && item.risks?.map((risk, index) => (
        <RiskBadge key={index} risk={risk} />
      ))}
    </div>
  );
}
