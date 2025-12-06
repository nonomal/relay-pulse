import { useEffect, useRef, useCallback } from 'react';
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

  // 打开时聚焦确认按钮
  useEffect(() => {
    if (isOpen && confirmButtonRef.current) {
      confirmButtonRef.current.focus();
    }
  }, [isOpen]);

  // 点击遮罩关闭
  const handleBackdropClick = useCallback(
    (e: React.MouseEvent) => {
      if (e.target === e.currentTarget) {
        onCancel();
      }
    },
    [onCancel]
  );

  // 确认并勾选"不再提示"
  const handleConfirmWithDontShow = useCallback(() => {
    onDontShowAgain?.();
    onConfirm();
  }, [onConfirm, onDontShowAgain]);

  if (!isOpen) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm"
      onClick={handleBackdropClick}
      role="dialog"
      aria-modal="true"
      aria-labelledby="external-link-modal-title"
    >
      <div
        ref={modalRef}
        className="relative mx-4 w-full max-w-md rounded-lg bg-white p-6 shadow-xl dark:bg-gray-800"
      >
        {/* 关闭按钮 */}
        <button
          onClick={onCancel}
          className="absolute right-4 top-4 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
          aria-label={t('common.close')}
        >
          <X size={20} />
        </button>

        {/* 标题 */}
        <div className="mb-4 flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-full bg-yellow-100 dark:bg-yellow-900/30">
            <AlertTriangle className="h-5 w-5 text-yellow-600 dark:text-yellow-500" />
          </div>
          <h2
            id="external-link-modal-title"
            className="text-lg font-semibold text-gray-900 dark:text-gray-100"
          >
            {t('externalLink.title')}
          </h2>
        </div>

        {/* 目标信息 */}
        <div className="mb-4 rounded-md bg-gray-100 px-3 py-2 dark:bg-gray-700">
          <p className="text-sm text-gray-600 dark:text-gray-300">
            {t('externalLink.target')}: <strong>{targetName}</strong>
          </p>
          <p className="mt-1 truncate text-xs text-gray-500 dark:text-gray-400">{targetDomain}</p>
        </div>

        {/* 风险提示 */}
        <div className="mb-6 space-y-2 text-sm text-gray-600 dark:text-gray-300">
          <p>{t('externalLink.thirdPartyNotice')}</p>
          <ul className="ml-4 list-disc space-y-1 text-gray-500 dark:text-gray-400">
            <li>{t('externalLink.riskTip1')}</li>
            <li>{t('externalLink.riskTip2')}</li>
          </ul>
        </div>

        {/* 按钮组 */}
        <div className="flex flex-col gap-3 sm:flex-row sm:justify-end">
          <button
            onClick={onCancel}
            className="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
          >
            {t('externalLink.cancel')}
          </button>
          <button
            ref={confirmButtonRef}
            onClick={onConfirm}
            className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
          >
            {t('externalLink.confirm')}
          </button>
        </div>

        {/* 不再提示选项 */}
        {onDontShowAgain && (
          <div className="mt-4 border-t border-gray-200 pt-3 dark:border-gray-700">
            <button
              onClick={handleConfirmWithDontShow}
              className="text-xs text-gray-500 hover:text-gray-700 hover:underline dark:text-gray-400 dark:hover:text-gray-300"
            >
              {t('externalLink.dontShowAgain')}
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
