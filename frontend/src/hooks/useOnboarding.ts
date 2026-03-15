import { useState, useCallback, useEffect, useRef } from 'react';
import { apiGet, apiPost, ApiError } from '../utils/apiClient';
import type {
  OnboardingMeta,
  OnboardingFormData,
  OnboardingTestResult,
  SubmitOnboardingRequest,
  SubmitOnboardingResponse,
} from '../types/onboarding';

const DRAFT_KEY = 'relay-pulse-onboarding-draft';
const POLL_INTERVAL = 2000;

/** 加载保存的草稿 */
function loadDraft(): Partial<OnboardingFormData> {
  try {
    const raw = localStorage.getItem(DRAFT_KEY);
    if (raw) return JSON.parse(raw);
  } catch { /* ignore */ }
  return {};
}

/** 保存草稿 */
function saveDraft(data: Partial<OnboardingFormData>) {
  try {
    // 不保存 apiKey
    const { apiKey: _, ...safe } = data as OnboardingFormData;
    void _;
    localStorage.setItem(DRAFT_KEY, JSON.stringify(safe));
  } catch { /* ignore */ }
}

/** 清除草稿 */
function clearDraft() {
  try { localStorage.removeItem(DRAFT_KEY); } catch { /* ignore */ }
}

const defaultForm: OnboardingFormData = {
  providerName: '',
  websiteUrl: '',
  category: 'commercial',
  serviceType: 'cc',
  sponsorLevel: 'public',
  channelType: 'O',
  channelSource: 'API',
  contactInfo: '',
  agreementAccepted: false,
  baseUrl: '',
  apiKey: '',
  testType: '',
  testVariant: '',
};

export function useOnboarding() {
  const [step, setStep] = useState(1);
  const [meta, setMeta] = useState<OnboardingMeta | null>(null);
  const [formData, setFormData] = useState<OnboardingFormData>(() => ({
    ...defaultForm,
    ...loadDraft(),
  }));

  // Test state
  const [testJobId, setTestJobId] = useState<string | null>(null);
  const [testResult, setTestResult] = useState<OnboardingTestResult | null>(null);
  const [testProof, setTestProof] = useState<string | null>(null);
  const [isTesting, setIsTesting] = useState(false);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Submit state
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitResult, setSubmitResult] = useState<SubmitOnboardingResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [metaError, setMetaError] = useState<string | null>(null);

  // Load meta on mount
  useEffect(() => {
    apiGet<OnboardingMeta>('/api/onboarding/meta')
      .then(setMeta)
      .catch((e) => {
        const msg = e instanceof ApiError ? e.message : '加载表单配置失败';
        setMetaError(msg);
      });
  }, []);

  // Save draft on form change
  useEffect(() => { saveDraft(formData); }, [formData]);

  // Cleanup polling on unmount
  useEffect(() => {
    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, []);

  const updateField = useCallback(<K extends keyof OnboardingFormData>(key: K, value: OnboardingFormData[K]) => {
    setFormData(prev => ({ ...prev, [key]: value }));
    setError(null);
  }, []);

  const goToStep = useCallback((s: number) => {
    setStep(s);
    setError(null);
  }, []);

  /** 构建 selftest 的 API URL */
  const buildTestApiUrl = useCallback(() => {
    // The test API URL is derived from base_url + template path
    // For simplicity, use the base_url directly - the selftest system handles templates
    return formData.baseUrl;
  }, [formData.baseUrl]);

  /** 运行连通性测试 */
  const runTest = useCallback(async () => {
    setIsTesting(true);
    setError(null);
    setTestResult(null);
    setTestProof(null);

    try {
      const testApiUrl = buildTestApiUrl();
      const resp = await apiPost<{ id: string }>('/api/selftest', {
        test_type: formData.testType,
        payload_variant: formData.testVariant || undefined,
        api_url: testApiUrl,
        api_key: formData.apiKey,
      });

      setTestJobId(resp.id);

      // Start polling
      if (pollRef.current) clearInterval(pollRef.current);
      pollRef.current = setInterval(async () => {
        try {
          const result = await apiGet<OnboardingTestResult>(`/api/selftest/${resp.id}`);
          setTestResult(result);

          const terminal = ['success', 'failed', 'timeout', 'canceled'].includes(result.status);
          if (terminal) {
            if (pollRef.current) {
              clearInterval(pollRef.current);
              pollRef.current = null;
            }
            setIsTesting(false);

            if (result.test_proof) {
              setTestProof(result.test_proof);
            }
          }
        } catch {
          if (pollRef.current) {
            clearInterval(pollRef.current);
            pollRef.current = null;
          }
          setIsTesting(false);
        }
      }, POLL_INTERVAL);

    } catch (e) {
      setIsTesting(false);
      setError(e instanceof ApiError ? e.message : '测试请求失败');
    }
  }, [formData, buildTestApiUrl]);

  /** 提交申请 */
  const submit = useCallback(async () => {
    if (!testProof || !testJobId || !testResult) {
      setError('请先通过连通性测试');
      return;
    }

    setIsSubmitting(true);
    setError(null);

    // 自动补全 URL 协议前缀
    const ensureUrl = (v: string) => {
      const s = v.trim();
      if (s && !/^https?:\/\//i.test(s)) return 'https://' + s;
      return s;
    };

    try {
      const req: SubmitOnboardingRequest = {
        provider_name: formData.providerName,
        website_url: ensureUrl(formData.websiteUrl),
        category: formData.category,
        service_type: formData.serviceType,
        template_name: formData.testVariant || formData.testType, // variant is the actual template name
        sponsor_level: formData.sponsorLevel,
        channel_type: formData.channelType,
        channel_source: formData.channelSource,
        base_url: ensureUrl(formData.baseUrl),
        api_key: formData.apiKey,
        test_proof: testProof,
        test_job_id: testJobId,
        test_type: formData.testType,
        test_api_url: buildTestApiUrl(),
        test_latency: testResult.latency ?? 0,
        test_http_code: testResult.http_code ?? 0,
        contact_info: formData.contactInfo,
        locale: navigator.language || 'zh-CN',
      };

      const resp = await apiPost<SubmitOnboardingResponse>('/api/onboarding/submit', req);
      setSubmitResult(resp);
      clearDraft();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : '提交失败');
    } finally {
      setIsSubmitting(false);
    }
  }, [formData, testProof, testJobId, testResult, buildTestApiUrl]);

  const reset = useCallback(() => {
    setStep(1);
    setFormData(defaultForm);
    setTestJobId(null);
    setTestResult(null);
    setTestProof(null);
    setSubmitResult(null);
    setError(null);
    clearDraft();
  }, []);

  return {
    step,
    meta,
    metaError,
    formData,
    testJobId,
    testResult,
    testProof,
    isTesting,
    isSubmitting,
    submitResult,
    error,
    updateField,
    goToStep,
    runTest,
    submit,
    reset,
  };
}
