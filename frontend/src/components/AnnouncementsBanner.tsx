import { useState, useEffect, useRef } from 'react';
import { X, Megaphone, ExternalLink, ChevronRight } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import type { AnnouncementsResponse } from '../hooks/useAnnouncements';

interface AnnouncementsBannerProps {
  className?: string;
  data: AnnouncementsResponse | null;
  loading: boolean;
  shouldShowBanner: boolean;
  onDismiss: () => void;
}

/**
 * 公告横幅组件
 *
 * 显示最新公告，支持：
 * - 关闭后有新公告时重新出现
 * - 点击查看跳转到公告详情
 * - 响应式布局
 */
export function AnnouncementsBanner({
  className = '',
  data,
  loading,
  shouldShowBanner,
  onDismiss,
}: AnnouncementsBannerProps) {
  const { t } = useTranslation();

  // 入场动画状态
  const [isVisible, setIsVisible] = useState(false);
  const dismissTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (shouldShowBanner) {
      // 延迟启动入场动画
      const timer = setTimeout(() => setIsVisible(true), 100);
      return () => clearTimeout(timer);
    } else {
      // 使用 requestAnimationFrame 避免同步 setState 警告
      const raf = requestAnimationFrame(() => setIsVisible(false));
      return () => cancelAnimationFrame(raf);
    }
  }, [shouldShowBanner]);

  // 清理 dismiss timer
  useEffect(() => {
    return () => {
      if (dismissTimerRef.current) {
        clearTimeout(dismissTimerRef.current);
      }
    };
  }, []);

  // 不显示条件：加载中、无数据、功能禁用、无未读
  if (loading || !data?.enabled || !shouldShowBanner || !data.latest) {
    return null;
  }

  const latest = data.latest;
  const discussionsUrl = data.source.discussionsUrl;
  const itemCount = data.items.length;

  // 格式化日期
  const formatDate = (dateStr: string) => {
    try {
      const date = new Date(dateStr);
      return date.toLocaleDateString(undefined, {
        month: 'short',
        day: 'numeric',
      });
    } catch {
      return '';
    }
  };

  const handleClose = () => {
    setIsVisible(false);
    // 等待退场动画完成后执行 dismiss
    dismissTimerRef.current = setTimeout(onDismiss, 200);
  };

  // handleViewClick 不再调用 onDismiss()
  // 点击链接只负责跳转，不关闭 Banner
  // 只有点击 X 按钮才会关闭

  return (
    <div
      className={`
        relative overflow-hidden
        bg-accent/10 border border-accent/20 rounded-lg
        transition-all duration-300 ease-out
        ${isVisible ? 'opacity-100 translate-y-0' : 'opacity-0 -translate-y-2'}
        ${className}
      `}
    >
      <div className="flex items-center gap-3 px-4 py-3">
        {/* 图标 */}
        <div className="flex-shrink-0">
          <Megaphone className="w-5 h-5 text-accent" />
        </div>

        {/* 内容 */}
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            {/* 标签 */}
            <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-accent/20 text-accent">
              {t('announcements.newAnnouncement')}
            </span>

            {/* 标题 */}
            <a
              href={latest.url}
              target="_blank"
              rel="noopener noreferrer"
              className="text-sm font-medium text-primary hover:text-accent transition-colors truncate max-w-[300px] sm:max-w-[400px] md:max-w-[500px]"
              title={latest.title}
            >
              {latest.title}
            </a>

            {/* 日期 */}
            <span className="text-xs text-muted hidden sm:inline">
              {formatDate(latest.createdAt)}
            </span>
          </div>

          {/* 更多公告提示 */}
          {itemCount > 1 && (
            <div className="mt-1 text-xs text-muted">
              {t('announcements.moreAnnouncements', { count: itemCount - 1 })}
            </div>
          )}
        </div>

        {/* 操作按钮 */}
        <div className="flex items-center gap-2 flex-shrink-0">
          {/* 查看全部 */}
          <a
            href={discussionsUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="hidden sm:inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium text-accent hover:bg-accent/10 rounded-md transition-colors"
          >
            {t('announcements.viewAll')}
            <ChevronRight className="w-3 h-3" />
          </a>

          {/* 移动端简化按钮 - 增大热区至 44px 便于触控 */}
          <a
            href={discussionsUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="sm:hidden min-w-[44px] min-h-[44px] flex items-center justify-center text-accent hover:bg-accent/10 rounded-md transition-colors"
            title={t('announcements.viewAll')}
            aria-label={t('announcements.viewAll')}
          >
            <ExternalLink className="w-4 h-4" />
          </a>

          {/* 关闭按钮 - 移动端增大热区，桌面端保持紧凑 */}
          <button
            onClick={handleClose}
            className="min-w-[44px] min-h-[44px] sm:min-w-0 sm:min-h-0 sm:p-1.5 flex items-center justify-center text-muted hover:text-primary hover:bg-elevated rounded-md transition-colors"
            title={t('announcements.close')}
            aria-label={t('announcements.close')}
          >
            <X className="w-4 h-4" />
          </button>
        </div>
      </div>
    </div>
  );
}
