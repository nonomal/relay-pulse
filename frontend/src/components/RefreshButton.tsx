import { RefreshCw } from 'lucide-react';
import { useTranslation } from 'react-i18next';

interface RefreshButtonProps {
  loading: boolean;
  autoRefresh?: boolean;
  refreshCooldown: boolean;
  onRefresh: () => void;
  onToggleAutoRefresh?: () => void;
  /** 按钮尺寸：'sm' 用于移动端，'md' 用于桌面端 */
  size?: 'sm' | 'md';
}

/**
 * 合并的刷新按钮组件
 * - 点击刷新图标：执行手动刷新
 * - 右上角微型 toggle：切换自动刷新开关（可选）
 */
export function RefreshButton({
  loading,
  autoRefresh = true,
  refreshCooldown,
  onRefresh,
  onToggleAutoRefresh,
  size = 'md',
}: RefreshButtonProps) {
  const { t } = useTranslation();

  const isSmall = size === 'sm';
  const buttonSize = isSmall ? 'p-1.5' : 'p-2.5';
  const iconSize = isSmall ? 14 : 18;
  const minSize = isSmall ? '' : 'min-w-[44px] min-h-[44px]';

  return (
    <div className="relative inline-flex items-center">
      {/* 刷新按钮 */}
      <button
        type="button"
        onClick={onRefresh}
        className={`${buttonSize} rounded-lg bg-accent/10 text-accent hover:bg-accent/20 transition-colors border border-accent/20 group ${minSize} flex items-center justify-center cursor-pointer focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none`}
        title={t('common.refresh')}
        aria-label={t('common.refresh')}
      >
        <RefreshCw
          size={iconSize}
          className={loading ? 'animate-spin' : ''}
        />
      </button>

      {/* 自动刷新状态圆点（仅在 onToggleAutoRefresh 存在时显示） */}
      {onToggleAutoRefresh && (
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            onToggleAutoRefresh();
          }}
          className="absolute -top-1 -right-1 z-10 w-5 h-5 grid place-items-center cursor-pointer touch-manipulation select-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50 rounded-full"
          title={autoRefresh ? t('controls.autoRefresh.enabledHint') : t('controls.autoRefresh.disabledHint')}
          aria-label={t('controls.autoRefresh.toggle')}
          aria-pressed={autoRefresh}
        >
          {/* 状态圆点：开启=实心绿，关闭=空心灰 */}
          <span
            aria-hidden="true"
            className={`w-2.5 h-2.5 rounded-full transition-all duration-200 ${
              autoRefresh
                ? 'bg-success border border-success shadow-[0_0_4px_rgba(34,197,94,0.5)]'
                : 'bg-transparent border border-muted'
            }`}
          />
        </button>
      )}

      {/* 冷却提示 */}
      {refreshCooldown && (
        <div className={`absolute top-full left-1/2 -translate-x-1/2 ${isSmall ? 'mt-1' : 'mt-2'} px-2 py-1 bg-elevated text-secondary text-[10px] rounded whitespace-nowrap shadow-lg border border-default z-50`}>
          {t('common.refreshCooldown')}
        </div>
      )}
    </div>
  );
}
