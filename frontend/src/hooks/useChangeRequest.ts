import { useState, useCallback, useRef, useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { apiPost, apiGet, ApiError } from '../utils/apiClient';
import type {
  AuthCandidate,
  AuthResponse,
  SubmitChangeRequest,
  SubmitChangeResponse,
} from '../types/change';

const POLL_INTERVAL = 2000;

export type ChangeStep = 'auth' | 'edit' | 'test' | 'review' | 'done';

interface SelfTestResult {
  status: string;
  probe_status?: number;
  latency?: number;
  http_code?: number;
  test_proof?: string;
  error_message?: string;
}

export function useChangeRequest() {
  const { t, i18n } = useTranslation();

  // 步骤控制
  const [step, setStepRaw] = useState<ChangeStep>('auth');

  // Auth 步骤
  const [apiKey, setApiKey] = useState('');
  const [candidates, setCandidates] = useState<AuthCandidate[]>([]);
  const [selectedCandidate, setSelectedCandidateRaw] = useState<AuthCandidate | null>(null);
  const [selectedVariant, setSelectedVariant] = useState('');
  const [isAuthenticating, setIsAuthenticating] = useState(false);

  // Edit 步骤
  const [changes, setChanges] = useState<Record<string, string>>({});
  const [newApiKey, setNewApiKey] = useState('');

  // Test 步骤
  const [isTesting, setIsTesting] = useState(false);
  const [testJobId, setTestJobId] = useState('');
  const [testResult, setTestResult] = useState<SelfTestResult | null>(null);
  const [testProof, setTestProof] = useState('');
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Submit
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [publicId, setPublicId] = useState('');

  // 通用
  const [error, setError] = useState<string | null>(null);

  // 停止轮询的共享工具
  const stopPolling = useCallback(() => {
    if (pollRef.current) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
  }, []);

  // 卸载时清理轮询
  useEffect(() => {
    return stopPolling;
  }, [stopPolling]);

  // 重置测试状态（共享工具）
  const resetTestState = useCallback(() => {
    stopPolling();
    setIsTesting(false);
    setTestJobId('');
    setTestResult(null);
    setTestProof('');
  }, [stopPolling]);

  // 解析候选通道的默认变体（优先 default_test_variant，兜底首个变体）
  const resolveDefaultVariant = useCallback((c: AuthCandidate | null): string => {
    if (!c) return '';
    if (c.default_test_variant) return c.default_test_variant;
    const variants = c.test_variants ?? [];
    if (variants.length > 0) {
      const sorted = variants.slice().sort((a, b) => a.order - b.order);
      return sorted[0].id;
    }
    return '';
  }, []);

  // 切换候选通道时同步重置变体和测试状态
  const setSelectedCandidate = useCallback((c: AuthCandidate | null) => {
    setSelectedCandidateRaw(c);
    setSelectedVariant(resolveDefaultVariant(c));
    resetTestState();
  }, [resolveDefaultVariant, resetTestState]);

  // 变体切换时重置测试状态，防止提交与实际测试不匹配的 variant 审计值
  const handleSetSelectedVariant = useCallback((v: string) => {
    setSelectedVariant(v);
    resetTestState();
  }, [resetTestState]);

  // 切步骤时自动停止轮询（离开 test 步骤）
  const setStep = useCallback((next: ChangeStep) => {
    setStepRaw(prev => {
      if (prev === 'test' && next !== 'test') {
        stopPolling();
        setIsTesting(false);
      }
      return next;
    });
  }, [stopPolling]);

  // 认证
  const authenticate = useCallback(async () => {
    if (!apiKey || apiKey.length < 10) {
      setError(t('changeRequest.auth.invalidKey'));
      return;
    }
    setIsAuthenticating(true);
    setError(null);
    // 清理上一次的编辑状态，防止换 Key 后沿用旧数据
    setSelectedCandidateRaw(null);
    setSelectedVariant('');
    setChanges({});
    setNewApiKey('');
    setTestResult(null);
    setTestProof('');
    try {
      const resp = await apiPost<AuthResponse>('/api/change/auth', { api_key: apiKey });
      setCandidates(resp.candidates);
      if (resp.candidates.length === 1) {
        setSelectedCandidate(resp.candidates[0]);
      }
      setStep('edit');
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t('changeRequest.auth.authFailed'));
    } finally {
      setIsAuthenticating(false);
    }
  }, [apiKey, t, setStep, setSelectedCandidate]);

  // 更新变更字段
  const updateChange = useCallback((field: string, value: string) => {
    setChanges(prev => {
      const next = { ...prev };
      if (value === '') {
        delete next[field];
      } else {
        next[field] = value;
      }
      return next;
    });
  }, []);

  // 判断是否需要测试
  const requiresTest = Object.keys(changes).some(
    f => f === 'base_url' || f === 'api_key'
  ) || newApiKey !== '';

  // 进入测试/提交步骤
  const proceedFromEdit = useCallback(() => {
    if (Object.keys(changes).length === 0 && newApiKey === '') {
      setError(t('changeRequest.edit.noChanges'));
      return;
    }
    setError(null);
    if (requiresTest) {
      setStep('test');
    } else {
      setStep('review');
    }
  }, [changes, newApiKey, requiresTest, t, setStep]);

  // 运行测试
  const runTest = useCallback(async () => {
    if (!selectedCandidate) return;
    setIsTesting(true);
    setError(null);
    setTestResult(null);
    setTestProof('');

    try {
      // 确定测试参数：使用变更后的值
      const testBaseUrl = changes['base_url'] || selectedCandidate.base_url;
      const testKey = newApiKey || apiKey;

      // 调用 selftest API
      const resp = await apiPost<{ id: string; status: string }>('/api/selftest', {
        test_type: selectedCandidate.test_type || selectedCandidate.service,
        payload_variant: selectedVariant || undefined,
        api_url: testBaseUrl,
        api_key: testKey,
      });
      setTestJobId(resp.id);

      // 开始轮询
      pollRef.current = setInterval(async () => {
        try {
          const result = await apiGet<SelfTestResult>(`/api/selftest/${resp.id}`);
          setTestResult(result);

          if (result.status === 'success' || result.status === 'failed' || result.status === 'timeout' || result.status === 'canceled') {
            stopPolling();
            setIsTesting(false);

            if (result.probe_status === 1 && result.test_proof) {
              setTestProof(result.test_proof);
            }
          }
        } catch {
          // 轮询失败不中断
        }
      }, POLL_INTERVAL);
    } catch (e) {
      setIsTesting(false);
      setError(e instanceof ApiError ? e.message : t('changeRequest.test.requestFailed'));
    }
  }, [selectedCandidate, selectedVariant, changes, newApiKey, apiKey, t, stopPolling]);

  // 提交变更
  const submit = useCallback(async () => {
    if (!selectedCandidate) return;
    setIsSubmitting(true);
    setError(null);

    try {
      const testBaseUrl = changes['base_url'] || selectedCandidate.base_url;

      const req: SubmitChangeRequest = {
        api_key: apiKey,
        target_key: selectedCandidate.monitor_key,
        proposed_changes: changes,
        locale: i18n.language,
      };

      if (newApiKey) {
        req.new_api_key = newApiKey;
      }

      if (requiresTest && testProof) {
        req.test_proof = testProof;
        req.test_job_id = testJobId;
        req.test_type = selectedCandidate.test_type || selectedCandidate.service;
        req.test_variant = selectedVariant || undefined;
        req.test_api_url = testBaseUrl;
        req.test_latency = testResult?.latency;
        req.test_http_code = testResult?.http_code;
      }

      const resp = await apiPost<SubmitChangeResponse>('/api/change/submit', req);
      setPublicId(resp.public_id);
      setStep('done');
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t('changeRequest.review.submitFailed'));
    } finally {
      setIsSubmitting(false);
    }
  }, [selectedCandidate, selectedVariant, changes, newApiKey, apiKey, requiresTest, testProof, testJobId, testResult, i18n.language, t, setStep]);

  // 重置
  const reset = useCallback(() => {
    stopPolling();
    setStepRaw('auth');
    setApiKey('');
    setCandidates([]);
    setSelectedCandidateRaw(null);
    setSelectedVariant('');
    setChanges({});
    setNewApiKey('');
    setIsTesting(false);
    setTestJobId('');
    setTestResult(null);
    setTestProof('');
    setIsSubmitting(false);
    setPublicId('');
    setError(null);
  }, [stopPolling]);

  return {
    // 步骤
    step,
    setStep,

    // Auth
    apiKey,
    setApiKey,
    candidates,
    selectedCandidate,
    setSelectedCandidate,
    selectedVariant,
    setSelectedVariant: handleSetSelectedVariant,
    isAuthenticating,
    authenticate,

    // Edit
    changes,
    updateChange,
    newApiKey,
    setNewApiKey,
    requiresTest,
    proceedFromEdit,

    // Test
    isTesting,
    testResult,
    testProof,
    runTest,

    // Submit
    isSubmitting,
    publicId,
    submit,

    // Common
    error,
    setError,
    reset,
  };
}
