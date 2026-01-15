import { useRef, type FC } from 'react';
import { useTranslation } from 'react-i18next';
import type { GenericBadge as GenericBadgeType, BadgeVariant } from '../../types';
import { useBadgeTooltip } from '../../hooks/useBadgeTooltip';
import { BadgeTooltip } from './BadgeTooltip';

interface GenericBadgeProps {
  badge: GenericBadgeType;
  className?: string;
  tooltipPlacement?: 'top' | 'bottom';
}

/**
 * 用户 API Key 图标 - 人形轮廓
 */
function UserKeyIcon({ variant }: { variant: BadgeVariant }) {
  const colorClass = getVariantColorClass(variant);
  return (
    <svg className="w-4 h-4" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      {/* 头部 */}
      <circle cx="12" cy="7" r="4" className={colorClass} />
      {/* 身体 */}
      <path
        d="M12,13 C8,13 5,15.5 5,19 L5,20 L19,20 L19,19 C19,15.5 16,13 12,13 Z"
        className={colorClass}
      />
    </svg>
  );
}

/**
 * 官方 API Key 图标 - 徽章带勾
 */
function OfficialKeyIcon({ variant }: { variant: BadgeVariant }) {
  const colorClass = getVariantColorClass(variant);
  return (
    <svg className="w-4 h-4" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      {/* 盾牌外形 */}
      <path
        d="M12,2 L4,5 L4,11 C4,16.5 7.4,21.7 12,23 C16.6,21.7 20,16.5 20,11 L20,5 L12,2 Z"
        className={colorClass}
      />
      {/* 白色勾号 */}
      <path
        d="M9,12 L11,14 L15,10"
        fill="none"
        stroke="white"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

/**
 * 官方基准通道图标 - 靶心/准星
 * 表示 RelayPulse 官方基准监测通道，用于基准对比
 */
function OfficialBaselineIcon({ variant }: { variant: BadgeVariant }) {
  const colorClass = getVariantColorClass(variant);
  return (
    <svg className="w-4 h-4" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      {/* 外圈 */}
      <circle cx="12" cy="12" r="10" className={colorClass} fillOpacity="0.3" />
      {/* 中圈 */}
      <circle cx="12" cy="12" r="6" className={colorClass} fillOpacity="0.5" />
      {/* 中心点 */}
      <circle cx="12" cy="12" r="2" className={colorClass} />
      {/* 十字准星线 - 使用 rect 替代 line 以便应用 fill 颜色 */}
      <rect x="11.25" y="2" width="1.5" height="4" className={colorClass} />
      <rect x="11.25" y="18" width="1.5" height="4" className={colorClass} />
      <rect x="2" y="11.25" width="4" height="1.5" className={colorClass} />
      <rect x="18" y="11.25" width="4" height="1.5" className={colorClass} />
    </svg>
  );
}

/**
 * 通用信息图标 - 圆形带 i
 */
function InfoIcon({ variant }: { variant: BadgeVariant }) {
  const colorClass = getVariantColorClass(variant);
  return (
    <svg className="w-4 h-4" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <circle cx="12" cy="12" r="10" className={colorClass} />
      <circle cx="12" cy="8" r="1" fill="white" />
      <rect x="11" y="11" width="2" height="6" rx="1" fill="white" />
    </svg>
  );
}

/**
 * 功能特性图标 - 闪电
 */
function FeatureIcon({ variant }: { variant: BadgeVariant }) {
  const colorClass = getVariantColorClass(variant);
  return (
    <svg className="w-4 h-4" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path
        d="M13,2 L4,14 L11,14 L11,22 L20,10 L13,10 L13,2 Z"
        className={colorClass}
      />
    </svg>
  );
}

/**
 * 根据 variant 返回 Tailwind 填充颜色类
 */
function getVariantColorClass(variant: BadgeVariant): string {
  switch (variant) {
    case 'success':
      return 'fill-success';
    case 'warning':
      return 'fill-warning';
    case 'danger':
      return 'fill-danger';
    case 'info':
      return 'fill-info';
    case 'default':
    default:
      return 'fill-muted';
  }
}

/**
 * 根据 badge.id 返回图标的额外样式类
 * 用于调整特定徽标的显示效果
 */
function getBadgeIconClass(badgeId: string): string {
  switch (badgeId) {
    case 'api_key_official':
      return 'opacity-60'; // 降低醒目程度
    default:
      return '';
  }
}

/**
 * 根据 badge.id 返回对应的图标组件
 * 支持的图标：api_key_user, api_key_official, official_baseline
 * 未知 id 回退到基于 kind 的通用图标
 */
function getBadgeIcon(badge: GenericBadgeType): FC<{ variant: BadgeVariant }> {
  switch (badge.id) {
    case 'api_key_user':
      return UserKeyIcon;
    case 'api_key_official':
      return OfficialKeyIcon;
    case 'official_baseline':
      return OfficialBaselineIcon;
    default:
      // 基于 kind 回退
      switch (badge.kind) {
        case 'source':
          return UserKeyIcon;
        case 'feature':
          return FeatureIcon;
        case 'info':
        default:
          return InfoIcon;
      }
  }
}

/**
 * 通用徽标组件
 * 纯图标样式，类似 SponsorBadge
 * 支持 tooltip 显示（700ms 延迟）
 */
export function GenericBadge({ badge, className = '', tooltipPlacement = 'top' }: GenericBadgeProps) {
  const { t } = useTranslation();
  const triggerRef = useRef<HTMLSpanElement>(null);
  const { isOpen, position, handleMouseEnter, handleMouseLeave } = useBadgeTooltip(
    triggerRef,
    tooltipPlacement
  );

  const BadgeIcon = getBadgeIcon(badge);
  const iconClass = getBadgeIconClass(badge.id);

  // tooltip 文本：优先使用 override，否则使用 i18n
  const tooltipText = badge.tooltip_override || t(`badges.generic.${badge.id}.tooltip`, { defaultValue: '' });
  const labelText = t(`badges.generic.${badge.id}.label`, { defaultValue: badge.id });

  const hasLink = Boolean(badge.url);

  const content = (
    <span
      ref={triggerRef}
      onMouseEnter={handleMouseEnter}
      onMouseLeave={handleMouseLeave}
      className={`inline-flex items-center select-none ${hasLink ? 'cursor-pointer' : 'cursor-default'} ${className}`}
      role="img"
      aria-label={tooltipText ? `${labelText}: ${tooltipText}` : labelText}
    >
      <span className={iconClass}>
        <BadgeIcon variant={badge.variant} />
      </span>
    </span>
  );

  const tooltipContent = tooltipText ? (
    <BadgeTooltip isOpen={isOpen} position={position}>
      <span className="font-medium">{labelText}</span>
      <span className="text-secondary ml-1">- {tooltipText}</span>
    </BadgeTooltip>
  ) : null;

  // 如果有链接，包裹为可点击链接
  if (hasLink) {
    return (
      <>
        <a
          href={badge.url}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex hover:opacity-80 transition-opacity"
          onClick={(e) => e.stopPropagation()}
        >
          {content}
        </a>
        {tooltipContent}
      </>
    );
  }

  return (
    <>
      {content}
      {tooltipContent}
    </>
  );
}
