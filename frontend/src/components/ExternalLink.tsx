import { ExternalLink as ExternalLinkIcon, AlertTriangle } from 'lucide-react';
import { trackEvent } from '../utils/analytics';

interface ExternalLinkProps {
  href: string | null | undefined;
  children: React.ReactNode;
  className?: string;
  trackLabel?: string;
  compact?: boolean; // 是否使用紧凑模式（32px 高度，用于表格行）
}

/**
 * 通用外链组件
 * - 自动添加安全属性 rel="noopener noreferrer"
 * - 显示外链图标
 * - HTTP 链接显示警告图标
 */
export function ExternalLink({ href, children, className = '', trackLabel, compact = false }: ExternalLinkProps) {
  // compact 模式仍保留 32px 最小点击高度（WCAG 建议）
  const sizeClass = compact ? 'min-h-[32px] py-0.5 -my-0.5' : 'min-h-[44px] py-1 -my-1';
  const baseClass = `inline-flex items-center gap-1 ${sizeClass} ${className}`;

  // 如果没有 URL，显示纯文本但保持相同行高，避免表格行高不一致
  if (!href) {
    return <span className={baseClass}>{children}</span>;
  }

  const isHttp = href.startsWith('http://');

  const handleClick = () => {
    // 自动生成 label：优先使用 trackLabel，否则使用 children（如果是字符串）
    const label = trackLabel || (typeof children === 'string' ? children : href);
    trackEvent('click_external_link', {
      label,
      url: href,
    });
  };

  // 生成无障碍标签
  const ariaLabel = typeof children === 'string'
    ? `访问 ${children}（在新窗口打开）`
    : '外部链接（在新窗口打开）';

  return (
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
        <span title="非加密 HTTP 链接" className="inline-flex" aria-label="警告：非加密链接">
          <AlertTriangle
            size={12}
            className="text-yellow-500 flex-shrink-0"
            aria-hidden="true"
          />
        </span>
      )}
    </a>
  );
}
