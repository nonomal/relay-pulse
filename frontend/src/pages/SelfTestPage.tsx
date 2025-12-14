import { Helmet } from 'react-helmet-async';
import { useTranslation } from 'react-i18next';
import { useSelfTest } from '../hooks/useSelfTest';
import { SelfTestForm } from '../components/SelfTestForm';
import { TestStatusCard } from '../components/TestStatusCard';
import { TestResultPanel } from '../components/TestResultPanel';

export const SelfTestPage: React.FC = () => {
  const { t } = useTranslation();
  const { jobId, status, queuePosition, result, error, isSubmitting, isPolling, submitTest, reset } = useSelfTest();

  const hasStarted = jobId !== null;
  const isTerminal = status === 'success' || status === 'failed' || status === 'timeout' || status === 'canceled';

  return (
    <>
      {/* SEO 优化 */}
      <Helmet>
        <title>{t('selftest.meta.title')} | RelayPulse</title>
        <meta name="description" content={t('selftest.meta.description')} />
        <meta name="robots" content="noindex,nofollow" />
        <link rel="canonical" href={`${window.location.origin}/selftest`} />
      </Helmet>

      <main className="min-h-screen bg-page py-8 px-4">
        <div className="max-w-2xl mx-auto space-y-8">
          {/* 页面标题 */}
          <header className="text-center space-y-3">
            <h1 className="text-3xl font-bold text-primary">{t('selftest.title')}</h1>
            <p className="text-secondary">{t('selftest.description')}</p>
          </header>

          {/* 错误提示 */}
          {error && (
            <div className="p-4 bg-danger/10 border border-danger/20 rounded-lg">
              <p className="text-danger font-medium">{error}</p>
            </div>
          )}

          {/* 测试表单 */}
          {!hasStarted && (
            <div className="bg-surface border border-muted rounded-lg p-6">
              <SelfTestForm onSubmit={submitTest} isSubmitting={isSubmitting} />
            </div>
          )}

          {/* 测试状态 */}
          {hasStarted && (
            <>
              <TestStatusCard status={status} queuePosition={queuePosition} isPolling={isPolling} />

              {/* 测试结果 */}
              {isTerminal && result && <TestResultPanel result={result} />}

              {/* 操作按钮 */}
              {isTerminal && (
                <div className="flex justify-center">
                  <button
                    onClick={reset}
                    className="px-6 py-3 bg-accent text-white rounded-lg font-medium hover:bg-accent-strong transition-colors"
                  >
                    {t('selftest.actions.newTest')}
                  </button>
                </div>
              )}
            </>
          )}

          {/* 使用提示 */}
          <div className="bg-muted/10 border border-muted/20 rounded-lg p-4">
            <h3 className="text-sm font-semibold text-primary mb-2">{t('selftest.hints.title')}</h3>
            <ul className="text-sm text-secondary space-y-1">
              <li>• {t('selftest.hints.httpsOnly')}</li>
              <li>• {t('selftest.hints.domainOnly')}</li>
              <li>• {t('selftest.hints.rateLimit')}</li>
              <li>• {t('selftest.hints.privacy')}</li>
            </ul>
          </div>
        </div>
      </main>
    </>
  );
};
