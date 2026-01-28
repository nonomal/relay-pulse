// ===== 管理后台 API 类型定义 =====

/** 分页列表响应 */
export interface AdminListResponse<T> {
  data: T[];
  total: number;
  meta: {
    offset: number;
    limit: number;
  };
}

/** 单项响应 */
export interface AdminItemResponse<T> {
  data: T;
  warning?: string;
  api_key_saved?: boolean;
}

// ===== 监测项 =====

/** 监测项配置（来自后端） */
export interface MonitorConfig {
  id: number;
  provider: string;
  service: string;
  channel: string;
  model: string;
  name: string;
  enabled: boolean;
  parent_key: string;
  config: Record<string, unknown>;
  schema_version: number;
  version: number;
  created_at: number;
  updated_at: number;
  deleted_at?: number;
  has_api_key?: boolean | null;
  api_key_masked?: string;
}

/** 配置 payload 中的常用字段 */
export interface ConfigPayload {
  url: string;
  method: string;
  category: 'commercial' | 'public';
  headers?: Record<string, string>;
  body?: string;
  success_contains?: string;
  interval?: string;
  slow_latency?: string;
  timeout?: string;
  retry?: number;
  proxy?: string;
  sponsor?: string;
  sponsor_url?: string;
  sponsor_level?: string;
  board?: string;
  cold_reason?: string;
  [key: string]: unknown;
}

/** 创建监测项请求 */
export interface CreateMonitorRequest {
  provider: string;
  service: string;
  channel?: string;
  model?: string;
  name?: string;
  enabled?: boolean;
  parent_key?: string;
  config: ConfigPayload;
  api_key?: string;
}

/** 更新监测项请求 */
export interface UpdateMonitorRequest {
  name?: string;
  enabled?: boolean;
  parent_key?: string;
  config: ConfigPayload;
  version: number;
  api_key?: string;
}

/** 批量操作请求 */
export interface BatchMonitorsRequest {
  action: 'enable' | 'disable' | 'delete';
  ids: number[];
}

/** 监测项列表查询参数 */
export interface MonitorListParams {
  provider?: string;
  service?: string;
  channel?: string;
  model?: string;
  search?: string;
  enabled?: boolean;
  include_deleted?: boolean;
  offset?: number;
  limit?: number;
}

// ===== 审计日志 =====

export interface MonitorConfigAudit {
  id: number;
  monitor_id: number;
  provider: string;
  service: string;
  channel: string;
  model: string;
  action: 'create' | 'update' | 'delete' | 'restore' | 'rotate_secret';
  before_blob?: string;
  after_blob?: string;
  before_version?: number;
  after_version?: number;
  secret_changed: boolean;
  actor?: string;
  actor_ip?: string;
  user_agent?: string;
  request_id?: string;
  batch_id?: string;
  reason?: string;
  created_at: number;
}

// ===== Provider 策略 =====

export interface ProviderPolicy {
  id: number;
  policy_type: 'disabled' | 'hidden' | 'risk';
  provider: string;
  reason?: string;
  risks?: unknown[];
  created_at: number;
  updated_at: number;
}

export interface CreateProviderPolicyRequest {
  policy_type: 'disabled' | 'hidden' | 'risk';
  provider: string;
  reason?: string;
  risks?: unknown[];
}

// ===== Badge =====

export interface BadgeDefinition {
  id: string;
  kind: 'sponsor' | 'risk' | 'feature' | 'info';
  weight: number;
  label_i18n: Record<string, string>;
  tooltip_i18n?: Record<string, string>;
  icon?: string;
  color?: string;
  created_at: number;
  updated_at: number;
}

export interface CreateBadgeDefinitionRequest {
  id: string;
  kind: 'sponsor' | 'risk' | 'feature' | 'info';
  weight?: number;
  label_i18n: Record<string, string>;
  tooltip_i18n?: Record<string, string>;
  icon?: string;
  color?: string;
}

export type BadgeScope = 'global' | 'provider' | 'service' | 'channel';

export interface BadgeBinding {
  id: number;
  badge_id: string;
  scope: BadgeScope;
  provider?: string;
  service?: string;
  channel?: string;
  tooltip_override?: Record<string, string>;
  created_at: number;
  updated_at: number;
}

export interface CreateBadgeBindingRequest {
  badge_id: string;
  scope: BadgeScope;
  provider?: string;
  service?: string;
  channel?: string;
  tooltip_override?: Record<string, string>;
}

// ===== Board =====

export interface BoardConfig {
  board: string;
  display_name: string;
  description?: string;
  sort_order: number;
  created_at: number;
  updated_at: number;
}

// ===== 全局设置 =====

export interface GlobalSetting {
  key: string;
  value: unknown;
  schema_version: number;
  version: number;
  created_at: number;
  updated_at: number;
}

// ===== 配置版本 =====

export interface ConfigVersions {
  monitors: number;
  policies: number;
  badges: number;
  boards: number;
  settings: number;
}

// ===== 审计日志查询参数 =====

export interface AuditListParams {
  provider?: string;
  service?: string;
  action?: 'create' | 'update' | 'delete' | 'restore' | 'rotate_secret';
  since?: number;
  until?: number;
  offset?: number;
  limit?: number;
}

// ===== 导入结果 =====

export interface ImportResult {
  created: number;
  skipped: number;
  errors?: string[];
}
