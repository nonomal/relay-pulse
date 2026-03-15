/** 变更请求状态 */
export type ChangeRequestStatus = 'pending' | 'approved' | 'rejected' | 'applied';

/** 认证候选通道 */
export interface AuthCandidate {
  provider: string;
  service: string;
  channel: string;
  monitor_key: string;
  apply_mode: 'auto' | 'manual';
  provider_name: string;
  provider_url: string;
  channel_name: string;
  category: string;
  sponsor_level: string;
  base_url: string;
  key_last4: string;
}

/** 认证响应 */
export interface AuthResponse {
  candidates: AuthCandidate[];
}

/** 提交变更请求参数 */
export interface SubmitChangeRequest {
  api_key: string;
  target_key: string;
  proposed_changes: Record<string, string>;
  new_api_key?: string;
  test_proof?: string;
  test_job_id?: string;
  test_type?: string;
  test_api_url?: string;
  test_latency?: number;
  test_http_code?: number;
  locale?: string;
}

/** 提交变更响应 */
export interface SubmitChangeResponse {
  public_id: string;
}

/** 变更请求状态查询响应 */
export interface ChangeStatusResponse {
  public_id: string;
  status: ChangeRequestStatus;
  target_key: string;
  apply_mode: string;
  created_at: number;
  updated_at: number;
}

/** 管理端变更请求 */
export interface AdminChangeRequest {
  id: number;
  public_id: string;
  status: ChangeRequestStatus;
  target_provider: string;
  target_service: string;
  target_channel: string;
  target_key: string;
  apply_mode: string;
  auth_fingerprint: string;
  auth_last4: string;
  current_snapshot: string;
  proposed_changes: string;
  new_key_encrypted?: string;
  new_key_fingerprint?: string;
  new_key_last4?: string;
  requires_test: boolean;
  test_job_id?: string;
  test_passed_at?: number;
  test_latency_ms?: number;
  test_http_code?: number;
  admin_note?: string;
  reviewed_at?: number;
  applied_at?: number;
  submitter_ip_hash?: string;
  locale?: string;
  created_at: number;
  updated_at: number;
}

/** 允许用户变更的字段集合 */
export const EDITABLE_FIELDS = [
  'provider_name',
  'provider_url',
  'channel_name',
  'category',
  'sponsor_level',
  'base_url',
  'api_key',
] as const;

export type EditableField = (typeof EDITABLE_FIELDS)[number];

/** 需要测试的字段 */
export const FIELDS_REQUIRING_TEST: ReadonlySet<string> = new Set([
  'base_url',
  'api_key',
]);
