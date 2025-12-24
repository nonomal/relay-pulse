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
  /** 是否显示右上角 toggle 圆点，默认 true。false 时状态通过颜色表示，点击=切换+刷新 */
  showToggle?: boolean;
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
  showToggle = true,
}: RefreshButtonProps) {
  const { t } = useTranslation();

  const isSmall = size === 'sm';
  const buttonSize = isSmall ? 'p-1.5' : 'p-2.5';
  const iconSize = isSmall ? 14 : 18;
  const minSize = isSmall ? '' : 'w-10 h-10';

  // 点击处理：showToggle=false 时，切换状态 + 刷新（冷却期内不切换，只显示冷却提示）
  const handleClick = () => {
    if (!showToggle && onToggleAutoRefresh && !refreshCooldown) {
      onToggleAutoRefresh();
    }
    onRefresh();
  };

  // 按钮样式：showToggle=false 且有 onToggleAutoRefresh 时根据 autoRefresh 状态决定颜色
  const buttonStyles = !showToggle && onToggleAutoRefresh
    ? autoRefresh
      ? 'bg-success/10 text-success border-success/50 hover:bg-success/20'
      : 'bg-elevated/50 text-muted border-muted hover:bg-muted/30'
    : 'bg-accent/10 text-accent border-accent/20 hover:bg-accent/20';

  // 提示文案：showToggle=false 且有 onToggleAutoRefresh 时说明点击会切换状态
  const buttonTitle = !showToggle && onToggleAutoRefresh
    ? (autoRefresh ? t('controls.autoRefresh.enabledHint') : t('controls.autoRefresh.disabledHint'))
    : t('common.refresh');

  return (
    <div className="relative inline-flex items-center">
      {/* 刷新按钮 */}
      <button
        type="button"
        onClick={handleClick}
        className={`${buttonSize} rounded-lg ${buttonStyles} transition-colors border group ${minSize} flex items-center justify-center cursor-pointer focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none`}
        title={buttonTitle}
        aria-label={buttonTitle}
      >
        <RefreshCw
          size={iconSize}
          className={loading ? 'animate-spin' : ''}
        />
      </button>

      {/* 自动刷新状态圆点（仅在 showToggle=true 且 onToggleAutoRefresh 存在时显示） */}
      {showToggle && onToggleAutoRefresh && (
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
          {/* 状态圆点：开启=实心绿，关闭=底色灰 */}
          <span
            aria-hidden="true"
            className={`w-2.5 h-2.5 rounded-full transition-all duration-200 ${
              autoRefresh
                ? 'bg-success border border-success/60 shadow-[0_0_6px_hsl(var(--success)_/_0.25)]'
                : 'bg-elevated border border-default/50'
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
