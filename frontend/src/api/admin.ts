// 管理后台 API 客户端 (v1.0)
import type {
  AdminUser,
  UserListParams,
  UpdateUserRequest,
  AdminApplication,
  ApplicationListParams,
  ReviewApplicationRequest,
  AdminAuditLog,
  AdminAuditListParams,
  AdminListResponse,
  AdminItemResponse,
} from '../types/admin';

const API_BASE = '/api/v1/admin';

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

// 构建查询字符串
function buildQueryString(params: Record<string, unknown>): string {
  const searchParams = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== '') {
      searchParams.append(key, String(value));
    }
  }
  const qs = searchParams.toString();
  return qs ? `?${qs}` : '';
}

// =====================================================
// 用户管理 API
// =====================================================

// 获取用户列表
export async function fetchUsers(params: UserListParams = {}): Promise<AdminListResponse<AdminUser>> {
  const response = await fetch(`${API_BASE}/users${buildQueryString(params)}`, {
    headers: authHeaders(),
  });
  return handleResponse<AdminListResponse<AdminUser>>(response);
}

// 获取用户详情
export async function fetchUser(id: string): Promise<AdminItemResponse<AdminUser>> {
  const response = await fetch(`${API_BASE}/users/${id}`, {
    headers: authHeaders(),
  });
  return handleResponse<AdminItemResponse<AdminUser>>(response);
}

// 更新用户
export async function updateUser(id: string, data: UpdateUserRequest): Promise<AdminItemResponse<AdminUser>> {
  const response = await fetch(`${API_BASE}/users/${id}`, {
    method: 'PATCH',
    headers: authHeaders(),
    body: JSON.stringify(data),
  });
  return handleResponse<AdminItemResponse<AdminUser>>(response);
}

// =====================================================
// 申请管理 API
// =====================================================

// 获取申请列表
export async function fetchApplications(params: ApplicationListParams = {}): Promise<AdminListResponse<AdminApplication>> {
  const response = await fetch(`${API_BASE}/applications${buildQueryString(params)}`, {
    headers: authHeaders(),
  });
  return handleResponse<AdminListResponse<AdminApplication>>(response);
}

// 获取申请详情
export async function fetchApplicationAdmin(id: number): Promise<AdminItemResponse<AdminApplication>> {
  const response = await fetch(`${API_BASE}/applications/${id}`, {
    headers: authHeaders(),
  });
  return handleResponse<AdminItemResponse<AdminApplication>>(response);
}

// 审核通过
export async function approveApplication(id: number): Promise<AdminItemResponse<AdminApplication>> {
  const response = await fetch(`${API_BASE}/applications/${id}/approve`, {
    method: 'POST',
    headers: authHeaders(),
  });
  return handleResponse<AdminItemResponse<AdminApplication>>(response);
}

// 审核拒绝
export async function rejectApplication(id: number, data: ReviewApplicationRequest): Promise<AdminItemResponse<AdminApplication>> {
  const response = await fetch(`${API_BASE}/applications/${id}/reject`, {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify(data),
  });
  return handleResponse<AdminItemResponse<AdminApplication>>(response);
}

// =====================================================
// 审计日志 API
// =====================================================

// 获取审计日志列表
export async function fetchAuditLogs(params: AdminAuditListParams = {}): Promise<AdminListResponse<AdminAuditLog>> {
  const response = await fetch(`${API_BASE}/audits${buildQueryString(params)}`, {
    headers: authHeaders(),
  });
  return handleResponse<AdminListResponse<AdminAuditLog>>(response);
}

// =====================================================
// 工具函数
// =====================================================

// 获取申请状态显示文本
export function getApplicationStatusLabel(status: AdminApplication['status']): string {
  const statusMap: Record<AdminApplication['status'], string> = {
    pending_test: '待测试',
    test_passed: '测试通过',
    test_failed: '测试失败',
    pending_review: '待审核',
    approved: '已通过',
    rejected: '已拒绝',
  };
  return statusMap[status] || status;
}

// 获取申请状态颜色
export function getApplicationStatusColor(status: AdminApplication['status']): string {
  const colorMap: Record<AdminApplication['status'], string> = {
    pending_test: 'text-muted bg-muted/20',
    test_passed: 'text-success bg-success/20',
    test_failed: 'text-danger bg-danger/20',
    pending_review: 'text-warning bg-warning/20',
    approved: 'text-success bg-success/20',
    rejected: 'text-danger bg-danger/20',
  };
  return colorMap[status] || 'text-muted bg-muted/20';
}

// 获取用户角色显示文本
export function getUserRoleLabel(role: AdminUser['role']): string {
  return role === 'admin' ? '管理员' : '用户';
}

// 获取用户状态显示文本
export function getUserStatusLabel(status: AdminUser['status']): string {
  return status === 'active' ? '正常' : '已禁用';
}
