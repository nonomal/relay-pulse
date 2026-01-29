// 申请相关 API 客户端
import type {
  Application,
  ApplicationListResponse,
  ApplicationResponse,
  CreateApplicationRequest,
  UpdateApplicationRequest,
  ServiceListResponse,
  TestSessionResponse,
  MonitorTemplate,
} from '../types/application';

const API_BASE = '/api';

// 获取认证 token
function getToken(): string | null {
  return localStorage.getItem('relay-pulse-token');
}

// 构建认证头
function authHeaders(): HeadersInit {
  const token = getToken();
  const headers: HeadersInit = {
    'Content-Type': 'application/json',
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }
  return headers;
}

// 处理响应
async function handleResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: 'Unknown error', code: 'UNKNOWN' }));
    throw new Error(error.error || `HTTP ${response.status}`);
  }
  return response.json();
}

// =====================================================
// 公开 API（无需认证）
// =====================================================

// 获取服务列表
export async function fetchServices(): Promise<ServiceListResponse> {
  const response = await fetch(`${API_BASE}/v1/public/services`);
  return handleResponse<ServiceListResponse>(response);
}

// =====================================================
// 用户 API（需要认证）
// =====================================================

// 获取我的申请列表
export async function fetchMyApplications(): Promise<ApplicationListResponse> {
  const response = await fetch(`${API_BASE}/v1/user/applications`, {
    headers: authHeaders(),
  });
  return handleResponse<ApplicationListResponse>(response);
}

// 获取申请详情
export async function fetchApplication(id: number): Promise<ApplicationResponse> {
  const response = await fetch(`${API_BASE}/v1/user/applications/${id}`, {
    headers: authHeaders(),
  });
  return handleResponse<ApplicationResponse>(response);
}

// 创建申请
export async function createApplication(data: CreateApplicationRequest): Promise<ApplicationResponse> {
  const response = await fetch(`${API_BASE}/v1/user/applications`, {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify(data),
  });
  return handleResponse<ApplicationResponse>(response);
}

// 更新申请
export async function updateApplication(id: number, data: UpdateApplicationRequest): Promise<ApplicationResponse> {
  const response = await fetch(`${API_BASE}/v1/user/applications/${id}`, {
    method: 'PATCH',
    headers: authHeaders(),
    body: JSON.stringify(data),
  });
  return handleResponse<ApplicationResponse>(response);
}

// 删除申请
export async function deleteApplication(id: number): Promise<void> {
  const response = await fetch(`${API_BASE}/v1/user/applications/${id}`, {
    method: 'DELETE',
    headers: authHeaders(),
  });
  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: 'Unknown error' }));
    throw new Error(error.error || `HTTP ${response.status}`);
  }
}

// 开始测试
export async function startTest(applicationId: number): Promise<TestSessionResponse> {
  const response = await fetch(`${API_BASE}/v1/user/applications/${applicationId}/test`, {
    method: 'POST',
    headers: authHeaders(),
  });
  return handleResponse<TestSessionResponse>(response);
}

// 获取测试会话状态
export async function fetchTestSession(applicationId: number, sessionId: number): Promise<TestSessionResponse> {
  const response = await fetch(`${API_BASE}/v1/user/applications/${applicationId}/test/${sessionId}`, {
    headers: authHeaders(),
  });
  return handleResponse<TestSessionResponse>(response);
}

// 提交审核
export async function submitApplication(id: number): Promise<ApplicationResponse> {
  const response = await fetch(`${API_BASE}/v1/user/applications/${id}/submit`, {
    method: 'POST',
    headers: authHeaders(),
  });
  return handleResponse<ApplicationResponse>(response);
}

// 获取服务的默认模板
export async function fetchDefaultTemplate(serviceId: string): Promise<{ data: MonitorTemplate }> {
  const response = await fetch(`${API_BASE}/v1/user/templates?service_id=${serviceId}&is_default=true&with_models=true`, {
    headers: authHeaders(),
  });
  const result = await handleResponse<{ data: MonitorTemplate[]; total: number }>(response);
  if (result.data.length === 0) {
    throw new Error('No default template found for this service');
  }
  return { data: result.data[0] };
}

// =====================================================
// 工具函数
// =====================================================

// 获取申请状态显示文本
export function getApplicationStatusText(status: Application['status']): string {
  const statusMap: Record<Application['status'], string> = {
    pending_test: '待测试',
    test_passed: '测试通过',
    test_failed: '测试失败',
    pending_review: '待审核',
    approved: '已通过',
    rejected: '已拒绝',
  };
  return statusMap[status] || status;
}

// 获取申请状态颜色类
export function getApplicationStatusColor(status: Application['status']): string {
  const colorMap: Record<Application['status'], string> = {
    pending_test: 'text-muted',
    test_passed: 'text-success',
    test_failed: 'text-danger',
    pending_review: 'text-warning',
    approved: 'text-success',
    rejected: 'text-danger',
  };
  return colorMap[status] || 'text-muted';
}

// 检查申请是否可以修改
export function canEditApplication(application: Application): boolean {
  return ['pending_test', 'test_failed'].includes(application.status);
}

// 检查申请是否可以测试
export function canTestApplication(application: Application): boolean {
  return ['pending_test', 'test_failed'].includes(application.status);
}

// 检查申请是否可以提交审核
export function canSubmitApplication(application: Application): boolean {
  return application.status === 'test_passed';
}
