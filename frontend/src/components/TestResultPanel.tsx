import { useTranslation } from 'react-i18next';
import type { TestResult } from '../types/selftest';

interface TestResultPanelProps {
  result: TestResult | null;
}

const probeStatusConfig: Record<number, { label: string; color: string; icon: string }> = {
  1: { label: 'selftest.result.available', color: 'text-success', icon: '🟢' },
  2: { label: 'selftest.result.degraded', color: 'text-warning', icon: '🟡' },
  0: { label: 'selftest.result.unavailable', color: 'text-danger', icon: '🔴' },
};

export const TestResultPanel: React.FC<TestResultPanelProps> = ({ result }) => {
  const { t } = useTranslation();

  if (!result) {
    return null;
  }

  const statusConfig = probeStatusConfig[result.probeStatus] || probeStatusConfig[0];

  return (
    <div className="bg-surface border border-muted rounded-lg p-6 space-y-4">
      <h3 className="text-lg font-semibold text-primary">{t('selftest.result.title')}</h3>

      <div className="space-y-3">
        {/* 探测状态 */}
        <div className="flex items-center justify-between">
          <span className="text-secondary">{t('selftest.result.probeStatus')}</span>
          <span className={`flex items-center gap-2 font-medium ${statusConfig.color}`}>
            <span>{statusConfig.icon}</span>
            {t(statusConfig.label)}
          </span>
        </div>

        {/* 细分状态 */}
        {result.subStatus && (
          <div className="flex items-center justify-between">
            <span className="text-secondary">{t('selftest.result.subStatus')}</span>
            <span className="text-primary font-mono text-sm">{result.subStatus}</span>
          </div>
        )}

        {/* HTTP 状态码 */}
        {result.httpCode !== undefined && result.httpCode > 0 && (
          <div className="flex items-center justify-between">
            <span className="text-secondary">{t('selftest.result.httpCode')}</span>
            <span className="text-primary font-mono text-sm">{result.httpCode}</span>
          </div>
        )}

        {/* 延迟 */}
        {result.latency !== undefined && result.latency > 0 && (
          <div className="flex items-center justify-between">
            <span className="text-secondary">{t('selftest.result.latency')}</span>
            <span className="text-primary font-mono text-sm">{result.latency} ms</span>
          </div>
        )}

        {/* 错误信息 */}
        {result.errorMessage && (
          <div className="mt-4 p-3 bg-danger/10 border border-danger/20 rounded">
            <p className="text-sm font-medium text-danger mb-1">{t('selftest.result.error')}</p>
            <p className="text-sm text-secondary font-mono break-all">{result.errorMessage}</p>
          </div>
        )}

        {/* 服务端响应片段（失败时显示，便于排查） */}
        {result.responseSnippet && result.probeStatus === 0 && (
          <div className="mt-4 p-3 bg-muted/30 border border-muted rounded">
            <p className="text-sm font-medium text-secondary mb-2">{t('selftest.result.responseSnippet')}</p>
            <pre className="text-xs text-muted font-mono whitespace-pre-wrap break-all max-h-40 overflow-auto">{result.responseSnippet}</pre>
          </div>
        )}
      </div>
    </div>
  );
};
