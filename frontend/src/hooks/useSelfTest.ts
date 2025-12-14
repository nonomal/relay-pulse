import { useState, useCallback, useRef, useEffect } from 'react';
import type {
  SelfTestFormData,
  CreateTestRequest,
  CreateTestResponse,
  TestJobDetail,
  JobStatus,
  TestResult,
} from '../types/selftest';

interface UseSelfTestReturn {
  jobId: string | null;
  status: JobStatus | null;
  queuePosition: number | null;
  result: TestResult | null;
  error: string | null;
  isSubmitting: boolean;
  isPolling: boolean;
  submitTest: (data: SelfTestFormData) => Promise<void>;
  reset: () => void;
}

const POLL_INTERVAL = 1500; // 1.5 秒轮询一次
const MAX_POLL_ATTEMPTS = 120; // 最多轮询 120 次（3 分钟）

export const useSelfTest = (): UseSelfTestReturn => {
  const [jobId, setJobId] = useState<string | null>(null);
  const [status, setStatus] = useState<JobStatus | null>(null);
  const [queuePosition, setQueuePosition] = useState<number | null>(null);
  const [result, setResult] = useState<TestResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isPolling, setIsPolling] = useState(false);

  const pollTimerRef = useRef<number | null>(null);
  const pollCountRef = useRef(0);

  // 清理轮询定时器
  const clearPollTimer = useCallback(() => {
    if (pollTimerRef.current !== null) {
      clearTimeout(pollTimerRef.current);
      pollTimerRef.current = null;
    }
  }, []);

  // 轮询任务状态
  const pollStatus = useCallback(
    async (id: string) => {
      try {
        const response = await fetch(`/api/selftest/${id}`);

        if (!response.ok) {
          if (response.status === 404) {
            setError('任务不存在或已过期');
            setIsPolling(false);
            return;
          }
          throw new Error(`Failed to fetch status: ${response.statusText}`);
        }

        const data: TestJobDetail = await response.json();

        // 更新状态
        setStatus(data.status);
        setQueuePosition(data.queue_position ?? null);

        // 如果有结果，更新结果
        if (data.probe_status !== undefined) {
          setResult({
            probeStatus: data.probe_status,
            subStatus: data.sub_status,
            httpCode: data.http_code,
            latency: data.latency,
            errorMessage: data.error_message,
            responseSnippet: data.response_snippet,
          });
        }

        // 检查是否是终态
        const terminalStatuses: JobStatus[] = ['success', 'failed', 'timeout', 'canceled'];
        if (terminalStatuses.includes(data.status)) {
          setIsPolling(false);
          clearPollTimer();
          return;
        }

        // 继续轮询
        pollCountRef.current++;
        if (pollCountRef.current < MAX_POLL_ATTEMPTS) {
          pollTimerRef.current = window.setTimeout(() => {
            pollStatus(id);
          }, POLL_INTERVAL);
        } else {
          setError('轮询超时');
          setIsPolling(false);
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : '未知错误');
        setIsPolling(false);
        clearPollTimer();
      }
    },
    [clearPollTimer]
  );

  // 提交测试
  const submitTest = useCallback(
    async (data: SelfTestFormData) => {
      // 重置状态
      setError(null);
      setResult(null);
      setStatus(null);
      setQueuePosition(null);
      setIsSubmitting(true);
      pollCountRef.current = 0;

      try {
        const request: CreateTestRequest = {
          test_type: data.testType,
          api_url: data.apiUrl,
          api_key: data.apiKey,
        };

        const response = await fetch('/api/selftest', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify(request),
        });

        if (!response.ok) {
          const errorText = await response.text();
          let errorMessage = errorText;

          // 尝试解析 JSON 错误
          try {
            const errorJson = JSON.parse(errorText);
            errorMessage = errorJson.error || errorText;
          } catch {
            // 如果不是 JSON，使用原始文本
          }

          throw new Error(errorMessage);
        }

        const responseData: CreateTestResponse = await response.json();

        setJobId(responseData.id);
        setStatus(responseData.status);
        setQueuePosition(responseData.queue_position ?? null);

        // 开始轮询
        setIsPolling(true);
        pollStatus(responseData.id);
      } catch (err) {
        setError(err instanceof Error ? err.message : '提交失败');
      } finally {
        setIsSubmitting(false);
      }
    },
    [pollStatus]
  );

  // 重置状态
  const reset = useCallback(() => {
    clearPollTimer();
    setJobId(null);
    setStatus(null);
    setQueuePosition(null);
    setResult(null);
    setError(null);
    setIsSubmitting(false);
    setIsPolling(false);
    pollCountRef.current = 0;
  }, [clearPollTimer]);

  // 组件卸载时清理
  useEffect(() => {
    return () => {
      clearPollTimer();
    };
  }, [clearPollTimer]);

  return {
    jobId,
    status,
    queuePosition,
    result,
    error,
    isSubmitting,
    isPolling,
    submitTest,
    reset,
  };
};
