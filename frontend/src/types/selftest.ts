// 自助测试相关类型定义

// 任务状态
export type JobStatus = 'queued' | 'running' | 'success' | 'failed' | 'timeout' | 'canceled';

// Payload 变体
export interface PayloadVariant {
  id: string;
  filename: string;
  order: number;
}

// 测试类型
export interface TestType {
  id: string;
  name: string;
  description: string;
  default_variant: string;
  variants: PayloadVariant[];
}

// 表单数据
export interface SelfTestFormData {
  testType: string;
  payloadVariant: string;
  apiUrl: string;
  apiKey: string;
}

// 创建测试请求
export interface CreateTestRequest {
  test_type: string;
  payload_variant?: string;
  api_url: string;
  api_key: string;
}

// 创建测试响应
export interface CreateTestResponse {
  id: string;
  status: JobStatus;
  payload_variant: string;
  queue_position?: number;
  created_at: number;
}

// 测试任务详情
export interface TestJobDetail {
  id: string;
  status: JobStatus;
  queue_position?: number;
  test_type: string;
  payload_variant: string;

  // 结果字段（完成后有值）
  probe_status?: number; // 1/0/2 (green/red/yellow)
  sub_status?: string;
  http_code?: number;
  latency?: number; // ms
  error_message?: string;
  response_snippet?: string; // 服务端响应片段（错误时便于排查）

  created_at: number;
  started_at?: number;
  finished_at?: number;
}

// 自助测试配置
export interface SelfTestConfig {
  max_concurrent: number;
  max_queue_size: number;
  job_timeout_seconds: number;
  rate_limit_per_minute: number;
}

// 测试结果（用于 UI 展示）
export interface TestResult {
  probeStatus: number;
  subStatus?: string;
  httpCode?: number;
  latency?: number;
  errorMessage?: string;
  responseSnippet?: string; // 服务端响应片段
}
