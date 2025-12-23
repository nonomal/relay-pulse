import { useState, useCallback } from 'react';
import { Loader2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { TelegramIcon } from './icons/TelegramIcon';

interface SubscribeButtonProps {
  favorites: Set<string>;
  /** 图标模式（仅显示图标） */
  iconOnly?: boolean;
  /** 在按钮组内时使用（去除独立边框） */
  inGroup?: boolean;
  className?: string;
}

// Notifier service URL - can be configured via environment variable
const NOTIFIER_API_URL = import.meta.env.VITE_NOTIFIER_API_URL || '';

interface BindTokenResponse {
  token: string;
  expires_in: number;
  deep_link: string;
}

export function SubscribeButton({ favorites, iconOnly = false, inGroup = false, className = '' }: SubscribeButtonProps) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubscribe = useCallback(async () => {
    if (favorites.size === 0) {
      setError(t('controls.subscribe.noFavorites'));
      setTimeout(() => setError(null), 3000);
      return;
    }

    if (!NOTIFIER_API_URL) {
      setError(t('controls.subscribe.error'));
      setTimeout(() => setError(null), 3000);
      return;
    }

    setLoading(true);
    setError(null);

    try {
      const response = await fetch(`${NOTIFIER_API_URL}/api/bind-token`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          favorites: Array.from(favorites),
        }),
      });

      if (!response.ok) {
        throw new Error('API request failed');
      }

      const data: BindTokenResponse = await response.json();

      // Open Telegram deeplink
      window.open(data.deep_link, '_blank');
    } catch (err) {
      console.error('Subscribe error:', err);
      setError(t('controls.subscribe.error'));
      setTimeout(() => setError(null), 3000);
    } finally {
      setLoading(false);
    }
  }, [favorites, t]);

  // Don't render if notifier URL is not configured
  if (!NOTIFIER_API_URL) {
    return null;
  }

  const isDisabled = loading || favorites.size === 0;

  // 图标模式
  if (iconOnly) {
    return (
      <div className="relative">
        <button
          type="button"
          onClick={handleSubscribe}
          disabled={isDisabled}
          className={`
            ${inGroup ? 'px-3 h-full' : 'p-2 rounded-lg h-10'} transition-all duration-200
            focus-visible:ring-2 ${inGroup ? 'focus-visible:ring-inset' : ''} focus-visible:ring-accent/50 focus-visible:outline-none
            ${isDisabled
              ? 'text-muted cursor-not-allowed'
              : 'text-secondary hover:text-accent hover:bg-muted/50'
            }
            ${className}
          `}
          title={favorites.size === 0
            ? t('controls.subscribe.noFavorites')
            : t('controls.subscribe.tooltip')
          }
          aria-label={t('controls.subscribe.button')}
        >
          {loading ? (
            <Loader2 size={16} className="animate-spin" />
          ) : (
            <TelegramIcon size={16} />
          )}
        </button>

        {/* Error tooltip */}
        {error && (
          <div className="absolute top-full left-1/2 -translate-x-1/2 mt-2 px-3 py-1.5 bg-danger/10 text-danger text-xs rounded-lg whitespace-nowrap shadow-lg border border-danger/20 z-50">
            {error}
          </div>
        )}
      </div>
    );
  }

  // 完整模式（移动端筛选抽屉中使用）
  return (
    <div className="relative">
      <button
        type="button"
        onClick={handleSubscribe}
        disabled={isDisabled}
        className={`
          flex items-center gap-1.5 px-3 py-2 rounded-lg transition-all duration-200
          focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none
          ${isDisabled
            ? 'bg-elevated/50 text-muted cursor-not-allowed'
            : 'bg-accent/10 text-accent hover:bg-accent/20 border border-accent/30'
          }
          ${className}
        `}
        title={favorites.size === 0
          ? t('controls.subscribe.noFavorites')
          : t('controls.subscribe.tooltip')
        }
      >
        {loading ? (
          <Loader2 size={14} className="animate-spin" />
        ) : (
          <TelegramIcon size={14} />
        )}
        <span className="text-sm font-medium">
          {loading ? t('controls.subscribe.loading') : t('controls.subscribe.button')}
        </span>
      </button>

      {/* Error tooltip */}
      {error && (
        <div className="absolute top-full left-1/2 -translate-x-1/2 mt-2 px-3 py-1.5 bg-danger/10 text-danger text-xs rounded-lg whitespace-nowrap shadow-lg border border-danger/20 z-50">
          {error}
        </div>
      )}
    </div>
  );
}
