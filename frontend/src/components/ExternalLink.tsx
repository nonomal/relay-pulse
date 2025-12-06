import { useState, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { ExternalLink as ExternalLinkIcon, AlertTriangle } from 'lucide-react';
import { trackEvent } from '../utils/analytics';
import { ExternalLinkModal } from './ExternalLinkModal';

// sessionStorage key 用于记住"不再提示"选项
const DONT_SHOW_AGAIN_KEY = 'externalLink_dontShowAgain';

// 检查是否已选择"不再提示"
const shouldSkipConfirm = () => {
  try {
    return sessionStorage.getItem(DONT_SHOW_AGAIN_KEY) === 'true';
  } catch {
    return false;
  }
};

// 保存"不再提示"选项
const saveDontShowAgain = () => {
  try {
    sessionStorage.setItem(DONT_SHOW_AGAIN_KEY, 'true');
  } catch {
    // sessionStorage 不可用时忽略
  }
};

interface ExternalLinkProps {
  href: string | null | undefined;
  children: React.ReactNode;
  className?: string;
  trackLabel?: string;
  compact?: boolean; // 是否使用紧凑模式（32px 高度，用于表格行）
  requireConfirm?: boolean; // 是否需要二次确认弹窗
}

/**
 * 通用外链组件
 * - 自动添加安全属性 rel="noopener noreferrer"
 * - 显示外链图标
 * - HTTP 链接显示警告图标
 * - 可选二次确认弹窗（用于服务商链接）
 */
export function ExternalLink({
  href,
  children,
  className = '',
  trackLabel,
  compact = false,
  requireConfirm = false,
}: ExternalLinkProps) {
  const { t } = useTranslation();
  const [showModal, setShowModal] = useState(false);

  // 获取显示名称（用于弹窗和埋点）
  const displayName = typeof children === 'string' ? children : trackLabel || href || '';

  // 记录点击事件
  const trackClick = useCallback(() => {
    if (!href) return;
    const label = trackLabel || (typeof children === 'string' ? children : href);
    trackEvent('click_external_link', {
      label,
      url: href,
    });
  }, [trackLabel, children, href]);

  // 执行跳转
  const openLink = useCallback(() => {
    if (!href) return;
    trackClick();
    window.open(href, '_blank', 'noopener,noreferrer');
  }, [href, trackClick]);

  // 点击处理
  const handleClick = useCallback(
    (e: React.MouseEvent) => {
      // 需要确认且未勾选"不再提示"
      if (requireConfirm && !shouldSkipConfirm()) {
        e.preventDefault();
        setShowModal(true);
        return;
      }

      // 直接跳转
      trackClick();
    },
    [requireConfirm, trackClick]
  );

  // 确认跳转
  const handleConfirm = useCallback(() => {
    setShowModal(false);
    openLink();
  }, [openLink]);

  // 取消
  const handleCancel = useCallback(() => {
    setShowModal(false);
  }, []);

  // 不再提示
  const handleDontShowAgain = useCallback(() => {
    saveDontShowAgain();
  }, []);

  // compact 模式仍保留 32px 最小点击高度（WCAG 建议）
  const sizeClass = compact ? 'min-h-[32px] py-0.5 -my-0.5' : 'min-h-[44px] py-1 -my-1';
  const baseClass = `inline-flex items-center gap-1 ${sizeClass} ${className}`;

  // 如果没有 URL，显示纯文本但保持相同行高，避免表格行高不一致
  if (!href) {
    return <span className={baseClass}>{children}</span>;
  }

  const isHttp = href.startsWith('http://');

  // 生成无障碍标签
  const ariaLabel =
    typeof children === 'string'
      ? t('externalLink.ariaLabel', { name: children })
      : t('externalLink.ariaLabelGeneric');

  return (
    <>
      <a
        href={href}
        target="_blank"
        rel="noopener noreferrer"
        className={`hover:underline active:underline ${baseClass}`}
        onClick={handleClick}
        aria-label={ariaLabel}
      >
        {children}
        <ExternalLinkIcon size={12} className="flex-shrink-0" aria-hidden="true" />
        {isHttp && (
          <span
            title={t('externalLink.httpWarning')}
            className="inline-flex"
            aria-label={t('externalLink.httpWarning')}
          >
            <AlertTriangle size={12} className="text-yellow-500 flex-shrink-0" aria-hidden="true" />
          </span>
        )}
      </a>

      {requireConfirm && (
        <ExternalLinkModal
          isOpen={showModal}
          targetUrl={href}
          targetName={displayName}
          onConfirm={handleConfirm}
          onCancel={handleCancel}
          onDontShowAgain={handleDontShowAgain}
        />
      )}
    </>
  );
}
