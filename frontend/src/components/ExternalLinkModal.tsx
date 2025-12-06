import { useEffect, useRef, useCallback } from 'react';
import { createPortal } from 'react-dom';
import { useTranslation } from 'react-i18next';
import { AlertTriangle, X } from 'lucide-react';

interface ExternalLinkModalProps {
  isOpen: boolean;
  targetUrl: string;
  targetName: string;
  onConfirm: () => void;
  onCancel: () => void;
  onDontShowAgain?: () => void;
}

/**
 * 外部链接跳转确认弹窗
 * 用于提醒用户即将离开本站前往第三方网站
 */
export function ExternalLinkModal({
  isOpen,
  targetUrl,
  targetName,
  onConfirm,
  onCancel,
  onDontShowAgain,
}: ExternalLinkModalProps) {
  const { t } = useTranslation();
  const modalRef = useRef<HTMLDivElement>(null);
  const confirmButtonRef = useRef<HTMLButtonElement>(null);

  // 提取目标域名用于显示
  const targetDomain = (() => {
    try {
      return new URL(targetUrl).hostname;
    } catch {
      return targetUrl;
    }
  })();

  // ESC 键关闭
  useEffect(() => {
    if (!isOpen) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onCancel();
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onCancel]);

  // 打开时聚焦确认按钮，并禁止背景滚动
  useEffect(() => {
    if (isOpen) {
      confirmButtonRef.current?.focus();
      document.body.style.overflow = 'hidden';
    }
    return () => {
      document.body.style.overflow = '';
    };
  }, [isOpen]);

  // 确认并勾选"不再提示"
  const handleConfirmWithDontShow = useCallback(() => {
    onDontShowAgain?.();
    onConfirm();
  }, [onConfirm, onDontShowAgain]);

  if (!isOpen) return null;

  // 使用 Portal 渲染到 body，避免被父元素的 overflow 或定位影响
  return createPortal(
    <div
      className="fixed inset-0 z-[9999] flex items-center justify-center bg-black/70 backdrop-blur-sm p-4"
      role="dialog"
      aria-modal="true"
      aria-labelledby="external-link-modal-title"
    >
      <div
        ref={modalRef}
        className="relative w-full max-w-lg rounded-xl border border-slate-600 bg-slate-800 p-6 shadow-2xl"
      >
        {/* 关闭按钮 */}
        <button
          onClick={onCancel}
          className="absolute right-4 top-4 text-slate-400 hover:text-slate-200 transition-colors"
          aria-label={t('common.close')}
        >
          <X size={20} />
        </button>

        {/* 标题 */}
        <div className="mb-4 flex items-center gap-3 pr-8">
          <div className="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-full bg-yellow-500/20">
            <AlertTriangle className="h-5 w-5 text-yellow-500" />
          </div>
          <h2
            id="external-link-modal-title"
            className="text-lg font-semibold text-slate-100"
          >
            {t('externalLink.title')}
          </h2>
        </div>

        {/* 目标信息 */}
        <div className="mb-4 rounded-lg bg-slate-700/50 px-4 py-3 border border-slate-600">
          <p className="text-sm text-slate-200">
            {t('externalLink.target')}: <strong className="text-white">{targetName}</strong>
          </p>
          <p className="mt-1 truncate text-xs text-slate-400">{targetDomain}</p>
        </div>

        {/* 风险提示 */}
        <div className="mb-6 space-y-3 text-sm text-slate-300">
          <p>{t('externalLink.thirdPartyNotice')}</p>
          <ul className="ml-4 list-disc space-y-2 text-slate-400">
            <li>{t('externalLink.riskTip1')}</li>
            <li>{t('externalLink.riskTip2')}</li>
          </ul>
        </div>

        {/* 按钮组 */}
        <div className="flex flex-col gap-3 sm:flex-row sm:justify-end">
          <button
            onClick={onCancel}
            className="rounded-lg border border-slate-600 px-4 py-2.5 text-sm font-medium text-slate-300 hover:bg-slate-700 hover:text-slate-100 transition-colors"
          >
            {t('externalLink.cancel')}
          </button>
          <button
            ref={confirmButtonRef}
            onClick={onConfirm}
            className="rounded-lg bg-blue-600 px-4 py-2.5 text-sm font-medium text-white hover:bg-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 focus:ring-offset-slate-800 transition-colors"
          >
            {t('externalLink.confirm')}
          </button>
        </div>

        {/* 不再提示选项 */}
        {onDontShowAgain && (
          <div className="mt-4 border-t border-slate-700 pt-3">
            <button
              onClick={handleConfirmWithDontShow}
              className="text-xs text-slate-500 hover:text-slate-300 hover:underline transition-colors"
            >
              {t('externalLink.dontShowAgain')}
            </button>
          </div>
        )}
      </div>
    </div>,
    document.body
  );
}
