import { useTranslation } from 'react-i18next';
import type { SponsorLevel } from '../types';

// 赞助商等级对应的图标
const SPONSOR_ICONS: Record<SponsorLevel, string> = {
  individual: '♥️',
  generous: '💕',
  silver: '🤍',
  top: '💜',
};

interface SponsorBadgeProps {
  level: SponsorLevel;
  className?: string;
}

/**
 * 赞助商徽章组件
 * 显示纯图标，hover 时显示翻译后的完整名称
 */
export function SponsorBadge({ level, className = '' }: SponsorBadgeProps) {
  const { t } = useTranslation();

  return (
    <span
      title={t(`badges.sponsor.${level}`)}
      className={`cursor-default select-none ${className}`}
      role="img"
      aria-label={t(`badges.sponsor.${level}`)}
    >
      {SPONSOR_ICONS[level]}
    </span>
  );
}
