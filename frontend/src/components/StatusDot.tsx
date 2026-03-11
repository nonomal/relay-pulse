import { useTranslation } from 'react-i18next';
import { getStatusConfig } from '../constants';
import type { StatusKey } from '../types';

interface StatusDotProps {
  status: StatusKey;
  size?: 'sm' | 'md' | 'lg';
  showLabel?: boolean;
}

export function StatusDot({ status, size = 'md', showLabel = false }: StatusDotProps) {
  const { t } = useTranslation();
  const sizeClasses = {
    sm: 'w-2 h-2',
    md: 'w-3 h-3',
    lg: 'w-4 h-4',
  };

  const statusInfo = getStatusConfig(t)[status];

  return (
    <div
      className={`${sizeClasses[size]} rounded-full ${statusInfo.color} ${statusInfo.glow} transition-all duration-500`}
      role="img"
      aria-label={statusInfo.label}
      title={showLabel ? undefined : statusInfo.label}
    >
      {/* 为屏幕阅读器提供状态描述 */}
      <span className="sr-only">{statusInfo.label}</span>
    </div>
  );
}
