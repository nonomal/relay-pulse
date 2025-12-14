import { useTranslation } from 'react-i18next';
import type { JobStatus } from '../types/selftest';

interface TestStatusCardProps {
  status: JobStatus | null;
  queuePosition: number | null;
  isPolling: boolean;
}

const statusConfig: Record<
  JobStatus,
  {
    label: string;
    color: string;
    bgColor: string;
    icon: string;
  }
> = {
  queued: {
    label: 'selftest.status.queued',
    color: 'text-accent',
    bgColor: 'bg-accent/10',
    icon: '⏳',
  },
  running: {
    label: 'selftest.status.running',
    color: 'text-accent',
    bgColor: 'bg-accent/10',
    icon: '▶',
  },
  success: {
    label: 'selftest.status.success',
    color: 'text-success',
    bgColor: 'bg-success/10',
    icon: '✓',
  },
  failed: {
    label: 'selftest.status.failed',
    color: 'text-danger',
    bgColor: 'bg-danger/10',
    icon: '✗',
  },
  timeout: {
    label: 'selftest.status.timeout',
    color: 'text-warning',
    bgColor: 'bg-warning/10',
    icon: '⏱',
  },
  canceled: {
    label: 'selftest.status.canceled',
    color: 'text-muted',
    bgColor: 'bg-muted/10',
    icon: '⊘',
  },
};

export const TestStatusCard: React.FC<TestStatusCardProps> = ({ status, queuePosition, isPolling }) => {
  const { t } = useTranslation();

  if (!status) {
    return null;
  }

  const config = statusConfig[status];

  return (
    <div className={`p-6 rounded-lg ${config.bgColor} border border-${config.color.replace('text-', '')}/20`}>
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <span className={`text-2xl ${config.color}`}>{config.icon}</span>
          <div>
            <h3 className={`text-lg font-semibold ${config.color}`}>{t(config.label)}</h3>
            {status === 'queued' && queuePosition !== null && queuePosition > 0 && (
              <p className="text-sm text-secondary mt-1">
                {t('selftest.status.queuePosition', { position: queuePosition })}
              </p>
            )}
            {isPolling && (status === 'queued' || status === 'running') && (
              <p className="text-sm text-secondary mt-1">{t('selftest.status.updating')}</p>
            )}
          </div>
        </div>

        {/* 动画指示器 */}
        {isPolling && (status === 'queued' || status === 'running') && (
          <div className="flex gap-1">
            <span className={`w-2 h-2 rounded-full ${config.bgColor} animate-pulse`} style={{ animationDelay: '0ms' }} />
            <span className={`w-2 h-2 rounded-full ${config.bgColor} animate-pulse`} style={{ animationDelay: '200ms' }} />
            <span className={`w-2 h-2 rounded-full ${config.bgColor} animate-pulse`} style={{ animationDelay: '400ms' }} />
          </div>
        )}
      </div>
    </div>
  );
};
