// v1.0 申请相关类型定义

// 申请状态
export type ApplicationStatus =
  | 'pending_test'    // 待测试
  | 'test_passed'     // 测试通过
  | 'test_failed'     // 测试失败
  | 'pending_review'  // 待审核
  | 'approved'        // 已通过
  | 'rejected';       // 已拒绝

// 供应商类型
export type VendorType = 'merchant' | 'individual';

// 服务信息
export interface Service {
  id: string;
  name: string;
  icon_svg?: string;
  default_template_id?: number;
  status: string;
  sort_order: number;
}

// 模板模型
export interface TemplateModel {
  id: number;
  template_id: number;
  model_key: string;
  display_name: string;
  enabled: boolean;
  sort_order: number;
}

// 监测模板
export interface MonitorTemplate {
  id: number;
  service_id: string;
  name: string;
  slug: string;
  description?: string;
  is_default: boolean;
  request_method: string;
  timeout_ms: number;
  slow_latency_ms: number;
  models?: TemplateModel[];
}

// 测试结果
export interface TestResult {
  id: number;
  session_id: number;
  template_model_id: number;
  model_key: string;
  status: 'pass' | 'fail';
  latency_ms?: number;
  http_code?: number;
  error_message?: string;
  checked_at: number;
}

// 测试会话
export interface TestSession {
  id: number;
  application_id: number;
  status: 'pending' | 'running' | 'done';
  summary?: {
    total: number;
    passed: number;
    failed: number;
    avg_latency_ms?: number;
  };
  results?: TestResult[];
  created_at: number;
}

// 申请信息
export interface Application {
  id: number;
  applicant_user_id: string;
  service_id: string;
  template_id: number;
  provider_name: string;
  channel_name?: string;
  vendor_type: VendorType;
  website_url?: string;
  request_url: string;
  status: ApplicationStatus;
  reject_reason?: string;
  reviewer_user_id?: string;
  reviewed_at?: number;
  last_test_session_id?: number;
  last_test_session?: TestSession;
  created_at: number;
  updated_at: number;
}

// 创建申请请求
export interface CreateApplicationRequest {
  service_id: string;
  provider_name: string;
  channel_name?: string;
  vendor_type: VendorType;
  website_url?: string;
  request_url: string;
  api_key: string;
}

// 更新申请请求
export interface UpdateApplicationRequest {
  provider_name?: string;
  channel_name?: string;
  vendor_type?: VendorType;
  website_url?: string;
  request_url?: string;
  api_key?: string;
}

// 申请列表响应
export interface ApplicationListResponse {
  data: Application[];
  total: number;
}

// 申请详情响应
export interface ApplicationResponse {
  data: Application;
}

// 服务列表响应
export interface ServiceListResponse {
  data: Service[];
  total: number;
}

// 测试会话响应
export interface TestSessionResponse {
  data: TestSession;
}

// API 错误响应
export interface ApiErrorResponse {
  error: string;
  code: string;
}

// 申请向导步骤
export type WizardStep = 'service' | 'info' | 'apikey' | 'test' | 'result';

// 申请向导状态
export interface WizardState {
  step: WizardStep;
  serviceId: string | null;
  service: Service | null;
  template: MonitorTemplate | null;
  formData: {
    provider_name: string;
    channel_name: string;
    vendor_type: VendorType;
    website_url: string;
    request_url: string;
    api_key: string;
  };
  application: Application | null;
  testSession: TestSession | null;
  isSubmitting: boolean;
  error: string | null;
}
