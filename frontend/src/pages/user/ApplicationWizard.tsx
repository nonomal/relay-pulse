import { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { ProtectedRoute } from '../../components/auth';
import type {
  Service,
  MonitorTemplate,
  Application,
  TestSession,
  WizardStep,
  VendorType,
} from '../../types/application';
import {
  fetchServices,
  fetchDefaultTemplate,
  createApplication,
  updateApplication,
  startTest,
  fetchTestSession,
  submitApplication,
} from '../../api/application';

// 步骤指示器
function StepIndicator({ currentStep, steps }: { currentStep: WizardStep; steps: WizardStep[] }) {
  const { t } = useTranslation();
  const stepLabels: Record<WizardStep, string> = {
    service: t('application.steps.service', 'Select Service'),
    info: t('application.steps.info', 'Basic Info'),
    apikey: t('application.steps.apikey', 'API Key'),
    test: t('application.steps.test', 'Test'),
    result: t('application.steps.result', 'Result'),
  };

  const currentIndex = steps.indexOf(currentStep);

  return (
    <div className="flex items-center justify-center mb-8">
      {steps.map((step, index) => (
        <div key={step} className="flex items-center">
          <div
            className={`
              flex items-center justify-center w-8 h-8 rounded-full text-sm font-medium
              ${index < currentIndex ? 'bg-success text-white' : ''}
              ${index === currentIndex ? 'bg-accent text-white' : ''}
              ${index > currentIndex ? 'bg-muted/30 text-muted' : ''}
            `}
          >
            {index < currentIndex ? (
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
              </svg>
            ) : (
              index + 1
            )}
          </div>
          <span
            className={`ml-2 text-sm hidden sm:inline ${
              index === currentIndex ? 'text-primary font-medium' : 'text-muted'
            }`}
          >
            {stepLabels[step]}
          </span>
          {index < steps.length - 1 && (
            <div
              className={`w-8 sm:w-16 h-0.5 mx-2 ${
                index < currentIndex ? 'bg-success' : 'bg-muted/30'
              }`}
            />
          )}
        </div>
      ))}
    </div>
  );
}

// 服务选择步骤
function ServiceStep({
  services,
  selectedId,
  onSelect,
  isLoading,
}: {
  services: Service[];
  selectedId: string | null;
  onSelect: (service: Service) => void;
  isLoading: boolean;
}) {
  const { t } = useTranslation();

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="animate-spin rounded-full h-8 w-8 border-2 border-accent border-t-transparent" />
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <h2 className="text-lg font-semibold text-primary">
        {t('application.selectService', 'Select a service to monitor')}
      </h2>
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        {services.map((service) => (
          <button
            key={service.id}
            onClick={() => onSelect(service)}
            className={`
              p-4 rounded-lg border-2 text-left transition-all
              ${selectedId === service.id
                ? 'border-accent bg-accent/10'
                : 'border-muted/30 hover:border-accent/50 bg-surface'
              }
            `}
          >
            <div className="flex items-center gap-3">
              {service.icon_svg ? (
                <div
                  className="w-10 h-10 flex-shrink-0"
                  dangerouslySetInnerHTML={{ __html: service.icon_svg }}
                />
              ) : (
                <div className="w-10 h-10 rounded-full bg-muted/30 flex items-center justify-center">
                  <span className="text-lg font-bold text-muted">
                    {service.name.charAt(0).toUpperCase()}
                  </span>
                </div>
              )}
              <div>
                <h3 className="font-medium text-primary">{service.name}</h3>
                <p className="text-sm text-muted">{service.id.toUpperCase()}</p>
              </div>
            </div>
          </button>
        ))}
      </div>
    </div>
  );
}

// 基本信息步骤
function InfoStep({
  formData,
  onChange,
  onBack,
  onNext,
  isSubmitting,
}: {
  formData: {
    provider_name: string;
    channel_name: string;
    vendor_type: VendorType;
    website_url: string;
    request_url: string;
  };
  onChange: (field: string, value: string) => void;
  onBack: () => void;
  onNext: () => void;
  isSubmitting: boolean;
}) {
  const { t } = useTranslation();

  const isValid =
    formData.provider_name.trim() !== '' &&
    formData.request_url.trim() !== '' &&
    formData.request_url.startsWith('https://');

  return (
    <div className="space-y-6 max-w-xl mx-auto">
      <h2 className="text-lg font-semibold text-primary">
        {t('application.basicInfo', 'Basic Information')}
      </h2>

      <div className="space-y-4">
        {/* 服务商名称 */}
        <div>
          <label className="block text-sm font-medium text-secondary mb-1">
            {t('application.providerName', 'Provider Name')} *
          </label>
          <input
            type="text"
            value={formData.provider_name}
            onChange={(e) => onChange('provider_name', e.target.value)}
            placeholder={t('application.providerNamePlaceholder', 'e.g., MyRelay')}
            className="w-full px-3 py-2 bg-surface border border-muted/30 rounded-lg text-primary placeholder-muted focus:outline-none focus:border-accent"
          />
        </div>

        {/* 通道名称 */}
        <div>
          <label className="block text-sm font-medium text-secondary mb-1">
            {t('application.channelName', 'Channel Name')}
          </label>
          <input
            type="text"
            value={formData.channel_name}
            onChange={(e) => onChange('channel_name', e.target.value)}
            placeholder={t('application.channelNamePlaceholder', 'e.g., VIP, Standard (optional)')}
            className="w-full px-3 py-2 bg-surface border border-muted/30 rounded-lg text-primary placeholder-muted focus:outline-none focus:border-accent"
          />
        </div>

        {/* 供应商类型 */}
        <div>
          <label className="block text-sm font-medium text-secondary mb-1">
            {t('application.vendorType', 'Vendor Type')} *
          </label>
          <div className="flex gap-4">
            <label className="flex items-center gap-2 cursor-pointer">
              <input
                type="radio"
                name="vendor_type"
                value="merchant"
                checked={formData.vendor_type === 'merchant'}
                onChange={(e) => onChange('vendor_type', e.target.value)}
                className="text-accent focus:ring-accent"
              />
              <span className="text-primary">{t('application.merchant', 'Merchant')}</span>
            </label>
            <label className="flex items-center gap-2 cursor-pointer">
              <input
                type="radio"
                name="vendor_type"
                value="individual"
                checked={formData.vendor_type === 'individual'}
                onChange={(e) => onChange('vendor_type', e.target.value)}
                className="text-accent focus:ring-accent"
              />
              <span className="text-primary">{t('application.individual', 'Individual')}</span>
            </label>
          </div>
        </div>

        {/* 官网地址 */}
        <div>
          <label className="block text-sm font-medium text-secondary mb-1">
            {t('application.websiteUrl', 'Website URL')}
          </label>
          <input
            type="url"
            value={formData.website_url}
            onChange={(e) => onChange('website_url', e.target.value)}
            placeholder="https://example.com"
            className="w-full px-3 py-2 bg-surface border border-muted/30 rounded-lg text-primary placeholder-muted focus:outline-none focus:border-accent"
          />
        </div>

        {/* API 端点 */}
        <div>
          <label className="block text-sm font-medium text-secondary mb-1">
            {t('application.requestUrl', 'API Endpoint URL')} *
          </label>
          <input
            type="url"
            value={formData.request_url}
            onChange={(e) => onChange('request_url', e.target.value)}
            placeholder="https://api.example.com/v1/chat/completions"
            className="w-full px-3 py-2 bg-surface border border-muted/30 rounded-lg text-primary placeholder-muted focus:outline-none focus:border-accent"
          />
          <p className="mt-1 text-xs text-muted">
            {t('application.requestUrlHint', 'HTTPS only. This is the API endpoint for monitoring.')}
          </p>
        </div>
      </div>

      {/* 按钮 */}
      <div className="flex justify-between pt-4">
        <button
          onClick={onBack}
          className="px-4 py-2 text-secondary hover:text-primary transition-colors"
        >
          {t('common.back', 'Back')}
        </button>
        <button
          onClick={onNext}
          disabled={!isValid || isSubmitting}
          className="px-6 py-2 bg-accent hover:bg-accent-strong text-white rounded-lg disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          {t('common.next', 'Next')}
        </button>
      </div>
    </div>
  );
}

// API Key 步骤
function ApiKeyStep({
  apiKey,
  onChange,
  onBack,
  onNext,
  isSubmitting,
}: {
  apiKey: string;
  onChange: (value: string) => void;
  onBack: () => void;
  onNext: () => void;
  isSubmitting: boolean;
}) {
  const { t } = useTranslation();

  return (
    <div className="space-y-6 max-w-xl mx-auto">
      <h2 className="text-lg font-semibold text-primary">
        {t('application.enterApiKey', 'Enter API Key')}
      </h2>

      <div className="p-4 bg-warning/10 border border-warning/30 rounded-lg">
        <p className="text-sm text-warning">
          {t('application.apiKeyWarning', 'Your API key will be encrypted and stored securely. It will only be used for monitoring purposes.')}
        </p>
      </div>

      <div>
        <label className="block text-sm font-medium text-secondary mb-1">
          {t('application.apiKey', 'API Key')} *
        </label>
        <input
          type="password"
          value={apiKey}
          onChange={(e) => onChange(e.target.value)}
          placeholder="sk-..."
          className="w-full px-3 py-2 bg-surface border border-muted/30 rounded-lg text-primary placeholder-muted focus:outline-none focus:border-accent font-mono"
        />
      </div>

      {/* 按钮 */}
      <div className="flex justify-between pt-4">
        <button
          onClick={onBack}
          className="px-4 py-2 text-secondary hover:text-primary transition-colors"
        >
          {t('common.back', 'Back')}
        </button>
        <button
          onClick={onNext}
          disabled={!apiKey.trim() || isSubmitting}
          className="px-6 py-2 bg-accent hover:bg-accent-strong text-white rounded-lg disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting ? t('common.submitting', 'Submitting...') : t('application.startTest', 'Start Test')}
        </button>
      </div>
    </div>
  );
}

// 测试步骤
function TestStep({
  testSession,
  template,
  onRetry,
  onNext,
  isLoading,
}: {
  testSession: TestSession | null;
  template: MonitorTemplate | null;
  onRetry: () => void;
  onNext: () => void;
  isLoading: boolean;
}) {
  const { t } = useTranslation();

  const allPassed = testSession?.summary?.passed === testSession?.summary?.total;

  return (
    <div className="space-y-6 max-w-2xl mx-auto">
      <h2 className="text-lg font-semibold text-primary">
        {t('application.testResults', 'Test Results')}
      </h2>

      {isLoading || testSession?.status === 'running' ? (
        <div className="flex flex-col items-center justify-center py-12 gap-4">
          <div className="animate-spin rounded-full h-12 w-12 border-2 border-accent border-t-transparent" />
          <p className="text-secondary">{t('application.testRunning', 'Running tests...')}</p>
        </div>
      ) : testSession?.status === 'done' ? (
        <>
          {/* 摘要 */}
          <div className={`p-4 rounded-lg ${allPassed ? 'bg-success/10 border border-success/30' : 'bg-danger/10 border border-danger/30'}`}>
            <div className="flex items-center gap-3">
              {allPassed ? (
                <svg className="w-6 h-6 text-success" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              ) : (
                <svg className="w-6 h-6 text-danger" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              )}
              <div>
                <p className={`font-medium ${allPassed ? 'text-success' : 'text-danger'}`}>
                  {allPassed
                    ? t('application.allTestsPassed', 'All tests passed!')
                    : t('application.someTestsFailed', 'Some tests failed')}
                </p>
                <p className="text-sm text-secondary">
                  {t('application.testSummary', '{{passed}}/{{total}} passed', {
                    passed: testSession.summary?.passed || 0,
                    total: testSession.summary?.total || 0,
                  })}
                  {testSession.summary?.avg_latency_ms && (
                    <span className="ml-2">
                      {t('application.avgLatency', 'Avg latency: {{ms}}ms', {
                        ms: testSession.summary.avg_latency_ms,
                      })}
                    </span>
                  )}
                </p>
              </div>
            </div>
          </div>

          {/* 详细结果 */}
          <div className="space-y-2">
            {testSession.results?.map((result) => {
              const model = template?.models?.find((m) => m.id === result.template_model_id);
              return (
                <div
                  key={result.id}
                  className={`p-3 rounded-lg border ${
                    result.status === 'pass'
                      ? 'bg-success/5 border-success/20'
                      : 'bg-danger/5 border-danger/20'
                  }`}
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      {result.status === 'pass' ? (
                        <span className="text-success">✓</span>
                      ) : (
                        <span className="text-danger">✗</span>
                      )}
                      <span className="font-medium text-primary">
                        {model?.display_name || result.model_key}
                      </span>
                    </div>
                    <div className="text-sm text-secondary">
                      {result.latency_ms && <span>{result.latency_ms}ms</span>}
                      {result.http_code && <span className="ml-2">HTTP {result.http_code}</span>}
                    </div>
                  </div>
                  {result.error_message && (
                    <p className="mt-1 text-sm text-danger">{result.error_message}</p>
                  )}
                </div>
              );
            })}
          </div>

          {/* 按钮 */}
          <div className="flex justify-between pt-4">
            <button
              onClick={onRetry}
              className="px-4 py-2 text-secondary hover:text-primary transition-colors"
            >
              {t('application.retryTest', 'Retry Test')}
            </button>
            {allPassed && (
              <button
                onClick={onNext}
                className="px-6 py-2 bg-accent hover:bg-accent-strong text-white rounded-lg transition-colors"
              >
                {t('application.submitForReview', 'Submit for Review')}
              </button>
            )}
          </div>
        </>
      ) : (
        <div className="text-center py-12 text-muted">
          {t('application.noTestResults', 'No test results yet')}
        </div>
      )}
    </div>
  );
}

// 结果步骤
function ResultStep({ application }: { application: Application | null }) {
  const { t } = useTranslation();

  if (!application) return null;

  const isPending = application.status === 'pending_review';

  return (
    <div className="space-y-6 max-w-xl mx-auto text-center">
      <div className={`w-16 h-16 mx-auto rounded-full flex items-center justify-center ${
        isPending ? 'bg-warning/20' : 'bg-success/20'
      }`}>
        {isPending ? (
          <svg className="w-8 h-8 text-warning" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        ) : (
          <svg className="w-8 h-8 text-success" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        )}
      </div>

      <div>
        <h2 className="text-xl font-semibold text-primary">
          {isPending
            ? t('application.submittedTitle', 'Application Submitted!')
            : t('application.approvedTitle', 'Application Approved!')}
        </h2>
        <p className="mt-2 text-secondary">
          {isPending
            ? t('application.submittedDesc', 'Your application is now pending review. We will notify you once it is processed.')
            : t('application.approvedDesc', 'Your monitoring endpoint has been added to the system.')}
        </p>
      </div>

      <div className="pt-4">
        <a
          href="/user/applications"
          className="inline-block px-6 py-2 bg-accent hover:bg-accent-strong text-white rounded-lg transition-colors"
        >
          {t('application.viewMyApplications', 'View My Applications')}
        </a>
      </div>
    </div>
  );
}

// 主向导组件
function ApplicationWizardContent() {
  const { t } = useTranslation();
  const [step, setStep] = useState<WizardStep>('service');
  const [services, setServices] = useState<Service[]>([]);
  const [selectedService, setSelectedService] = useState<Service | null>(null);
  const [template, setTemplate] = useState<MonitorTemplate | null>(null);
  const [application, setApplication] = useState<Application | null>(null);
  const [testSession, setTestSession] = useState<TestSession | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [formData, setFormData] = useState({
    provider_name: '',
    channel_name: '',
    vendor_type: 'merchant' as VendorType,
    website_url: '',
    request_url: '',
    api_key: '',
  });

  const steps: WizardStep[] = ['service', 'info', 'apikey', 'test', 'result'];

  // 加载服务列表
  useEffect(() => {
    fetchServices()
      .then((res) => setServices(res.data))
      .catch((err) => setError(err.message))
      .finally(() => setIsLoading(false));
  }, []);

  // 选择服务
  const handleSelectService = useCallback(async (service: Service) => {
    setSelectedService(service);
    setIsLoading(true);
    setError(null);

    try {
      const res = await fetchDefaultTemplate(service.id);
      setTemplate(res.data);
      setStep('info');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load template');
    } finally {
      setIsLoading(false);
    }
  }, []);

  // 更新表单字段
  const handleFormChange = useCallback((field: string, value: string) => {
    setFormData((prev) => ({ ...prev, [field]: value }));
  }, []);

  // 创建或更新申请并开始测试
  const handleStartTest = useCallback(async () => {
    if (!selectedService || !template) return;

    setIsSubmitting(true);
    setError(null);

    try {
      let app = application;

      if (!app) {
        // 创建新申请
        const res = await createApplication({
          service_id: selectedService.id,
          provider_name: formData.provider_name,
          channel_name: formData.channel_name || undefined,
          vendor_type: formData.vendor_type,
          website_url: formData.website_url || undefined,
          request_url: formData.request_url,
          api_key: formData.api_key,
        });
        app = res.data;
        setApplication(app);
      } else {
        // 更新现有申请
        const res = await updateApplication(app.id, {
          provider_name: formData.provider_name,
          channel_name: formData.channel_name || undefined,
          vendor_type: formData.vendor_type,
          website_url: formData.website_url || undefined,
          request_url: formData.request_url,
          api_key: formData.api_key,
        });
        app = res.data;
        setApplication(app);
      }

      // 开始测试
      setStep('test');
      const testRes = await startTest(app.id);
      setTestSession(testRes.data);

      // 轮询测试状态
      if (testRes.data.status !== 'done') {
        const pollInterval = setInterval(async () => {
          try {
            const statusRes = await fetchTestSession(app!.id, testRes.data.id);
            setTestSession(statusRes.data);
            if (statusRes.data.status === 'done') {
              clearInterval(pollInterval);
            }
          } catch {
            clearInterval(pollInterval);
          }
        }, 2000);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to start test');
      setStep('apikey');
    } finally {
      setIsSubmitting(false);
    }
  }, [selectedService, template, application, formData]);

  // 重试测试
  const handleRetryTest = useCallback(() => {
    setStep('apikey');
    setTestSession(null);
  }, []);

  // 提交审核
  const handleSubmitForReview = useCallback(async () => {
    if (!application) return;

    setIsSubmitting(true);
    setError(null);

    try {
      const res = await submitApplication(application.id);
      setApplication(res.data);
      setStep('result');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to submit');
    } finally {
      setIsSubmitting(false);
    }
  }, [application]);

  return (
    <div className="min-h-screen bg-page">
      <div className="max-w-4xl mx-auto px-4 py-8">
        {/* 标题 */}
        <div className="text-center mb-8">
          <h1 className="text-2xl font-bold text-primary">
            {t('application.title', 'Submit Monitoring Application')}
          </h1>
          <p className="mt-2 text-secondary">
            {t('application.subtitle', 'Add your API endpoint to RelayPulse monitoring')}
          </p>
        </div>

        {/* 步骤指示器 */}
        <StepIndicator currentStep={step} steps={steps} />

        {/* 错误提示 */}
        {error && (
          <div className="mb-6 p-4 bg-danger/10 border border-danger/30 rounded-lg text-danger">
            {error}
          </div>
        )}

        {/* 步骤内容 */}
        <div className="bg-surface rounded-xl p-6 shadow-lg">
          {step === 'service' && (
            <ServiceStep
              services={services}
              selectedId={selectedService?.id || null}
              onSelect={handleSelectService}
              isLoading={isLoading}
            />
          )}

          {step === 'info' && (
            <InfoStep
              formData={formData}
              onChange={handleFormChange}
              onBack={() => setStep('service')}
              onNext={() => setStep('apikey')}
              isSubmitting={isSubmitting}
            />
          )}

          {step === 'apikey' && (
            <ApiKeyStep
              apiKey={formData.api_key}
              onChange={(value) => handleFormChange('api_key', value)}
              onBack={() => setStep('info')}
              onNext={handleStartTest}
              isSubmitting={isSubmitting}
            />
          )}

          {step === 'test' && (
            <TestStep
              testSession={testSession}
              template={template}
              onRetry={handleRetryTest}
              onNext={handleSubmitForReview}
              isLoading={isSubmitting}
            />
          )}

          {step === 'result' && <ResultStep application={application} />}
        </div>
      </div>
    </div>
  );
}

// 导出带保护的页面组件
export default function ApplicationWizard() {
  return (
    <ProtectedRoute>
      <ApplicationWizardContent />
    </ProtectedRoute>
  );
}
