package storage

import (
	"encoding/json"
)

// =====================================================
// v1.0 用户体系类型
// =====================================================

// UserRole 用户角色
type UserRole string

const (
	UserRoleAdmin UserRole = "admin"
	UserRoleUser  UserRole = "user"
)

// UserStatus 用户状态
type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusDisabled UserStatus = "disabled"
)

// User 用户
type User struct {
	ID        string     `json:"id"`         // UUID
	GitHubID  int64      `json:"github_id"`  // GitHub 用户 ID
	Username  string     `json:"username"`   // GitHub 用户名
	AvatarURL string     `json:"avatar_url"` // 头像 URL
	Email     string     `json:"email"`      // 邮箱
	Role      UserRole   `json:"role"`       // 角色：admin/user
	Status    UserStatus `json:"status"`     // 状态：active/disabled
	CreatedAt int64      `json:"created_at"` // 创建时间（Unix 时间戳）
	UpdatedAt int64      `json:"updated_at"` // 更新时间
}

// UserSession 用户会话
type UserSession struct {
	ID         string `json:"id"`                     // UUID
	UserID     string `json:"user_id"`                // 用户 ID
	TokenHash  string `json:"token_hash"`             // Token 哈希
	ExpiresAt  int64  `json:"expires_at"`             // 过期时间
	CreatedAt  int64  `json:"created_at"`             // 创建时间
	LastSeenAt *int64 `json:"last_seen_at,omitempty"` // 最后活跃时间
	RevokedAt  *int64 `json:"revoked_at,omitempty"`   // 撤销时间
	IP         string `json:"ip,omitempty"`           // 登录 IP
	UserAgent  string `json:"user_agent,omitempty"`   // User-Agent
}

// =====================================================
// v1.0 服务类型
// =====================================================

// Service 服务类型（如 Claude Code, Codex, Gemini）
type Service struct {
	ID                string `json:"id"`                  // 服务标识：cc, cx, gm
	Name              string `json:"name"`                // 显示名称
	IconSVG           string `json:"icon_svg,omitempty"`  // SVG 图标源码
	DefaultTemplateID *int   `json:"default_template_id"` // 默认模板 ID
	Status            string `json:"status"`              // active/disabled
	SortOrder         int    `json:"sort_order"`          // 排序顺序
	CreatedAt         int64  `json:"created_at"`
	UpdatedAt         int64  `json:"updated_at"`
}

// =====================================================
// v1.0 监测模板类型
// =====================================================

// MonitorTemplate 监测模板
type MonitorTemplate struct {
	ID                 int                    `json:"id"`
	ServiceID          string                 `json:"service_id"`           // 关联服务
	Name               string                 `json:"name"`                 // 模板名称
	Slug               string                 `json:"slug"`                 // 模板标识
	Description        string                 `json:"description"`          // 描述
	IsDefault          bool                   `json:"is_default"`           // 是否默认模板
	RequestMethod      string                 `json:"request_method"`       // 请求方法
	BaseRequestHeaders json.RawMessage        `json:"base_request_headers"` // 基础请求头（JSONB）
	BaseRequestBody    json.RawMessage        `json:"base_request_body"`    // 基础请求体（JSONB）
	BaseResponseChecks json.RawMessage        `json:"base_response_checks"` // 基础响应检查（JSONB）
	TimeoutMs          int                    `json:"timeout_ms"`           // 超时时间（毫秒）
	SlowLatencyMs      int                    `json:"slow_latency_ms"`      // 慢请求阈值（毫秒）
	RetryPolicy        json.RawMessage        `json:"retry_policy"`         // 重试策略（JSONB）
	CreatedBy          *string                `json:"created_by"`           // 创建者用户 ID
	CreatedAt          int64                  `json:"created_at"`
	UpdatedAt          int64                  `json:"updated_at"`
	Models             []MonitorTemplateModel `json:"models,omitempty"` // 关联的模型配置（查询时填充）
}

// MonitorTemplateModel 模板模型配置
type MonitorTemplateModel struct {
	ID                      int             `json:"id"`
	TemplateID              int             `json:"template_id"`
	ModelKey                string          `json:"model_key"`                 // 模型标识
	DisplayName             string          `json:"display_name"`              // 显示名称
	RequestBodyOverrides    json.RawMessage `json:"request_body_overrides"`    // 请求体覆盖
	ResponseChecksOverrides json.RawMessage `json:"response_checks_overrides"` // 响应检查覆盖
	Enabled                 bool            `json:"enabled"`                   // 是否启用
	SortOrder               int             `json:"sort_order"`                // 排序顺序
	CreatedAt               int64           `json:"created_at"`
	UpdatedAt               int64           `json:"updated_at"`
}

// =====================================================
// v1.0 监测项类型
// =====================================================

// Monitor 监测项（替代 MonitorConfig）
type Monitor struct {
	ID                  int             `json:"id"`
	Provider            string          `json:"provider"`                        // 服务商标识
	ProviderName        string          `json:"provider_name,omitempty"`         // 服务商显示名称
	Service             string          `json:"service"`                         // 服务标识
	ServiceName         string          `json:"service_name,omitempty"`          // 服务显示名称
	Channel             string          `json:"channel,omitempty"`               // 通道标识
	ChannelName         string          `json:"channel_name,omitempty"`          // 通道显示名称
	Model               string          `json:"model,omitempty"`                 // 模型标识（可多个，逗号分隔）
	TemplateID          *int            `json:"template_id"`                     // 关联模板
	URL                 string          `json:"url,omitempty"`                   // 请求 URL（必填）
	Method              *string         `json:"method,omitempty"`                // 请求方法，NULL = 继承模板
	Headers             json.RawMessage `json:"headers,omitempty"`               // 请求头（与模板合并）
	Body                json.RawMessage `json:"body,omitempty"`                  // 请求体（与模板合并）
	SuccessContains     *string         `json:"success_contains,omitempty"`      // 响应检查，NULL = 继承模板
	IntervalOverride    *string         `json:"interval_override,omitempty"`     // 巡检间隔覆盖
	TimeoutOverride     *string         `json:"timeout_override,omitempty"`      // 超时覆盖
	SlowLatencyOverride *string         `json:"slow_latency_override,omitempty"` // 慢响应阈值覆盖
	Enabled             bool            `json:"enabled"`                         // 是否启用
	BoardID             *int            `json:"board_id"`                        // 板块 ID
	Metadata            json.RawMessage `json:"metadata,omitempty"`              // 元数据（category, sponsor, price 等）
	OwnerUserID         *string         `json:"owner_user_id"`                   // 所有者用户 ID
	VendorType          string          `json:"vendor_type,omitempty"`           // merchant/individual
	WebsiteURL          string          `json:"website_url,omitempty"`           // 官网地址
	ApplicationID       *int            `json:"application_id"`                  // 关联申请
	CreatedAt           int64           `json:"created_at"`
	UpdatedAt           int64           `json:"updated_at"`
}

// MonitorMetadata 监测项元数据（存储在 metadata JSONB 字段）
type MonitorMetadata struct {
	Category     string   `json:"category,omitempty"`      // 官转/Tier 分类
	Sponsor      string   `json:"sponsor,omitempty"`       // 赞助商名称
	SponsorURL   string   `json:"sponsor_url,omitempty"`   // 赞助商链接
	SponsorLevel string   `json:"sponsor_level,omitempty"` // 赞助级别
	ProviderURL  string   `json:"provider_url,omitempty"`  // 官网链接
	ProviderSlug string   `json:"provider_slug,omitempty"` // 服务商别名
	PriceMin     *float64 `json:"price_min,omitempty"`     // 最低价
	PriceMax     *float64 `json:"price_max,omitempty"`     // 最高价
	ListedSince  string   `json:"listed_since,omitempty"`  // 收录日期
	Hidden       bool     `json:"hidden,omitempty"`        // 是否隐藏
	HiddenReason string   `json:"hidden_reason,omitempty"` // 隐藏原因
	ColdReason   string   `json:"cold_reason,omitempty"`   // 冷板原因
	Badges       []string `json:"badges,omitempty"`        // 徽标 ID 列表
}

// MonitorIDMapping 监测项 ID 映射（迁移用）
type MonitorIDMapping struct {
	OldID          int    `json:"old_id"`          // 旧 monitor_configs.id（可为 0 表示无旧 ID）
	NewID          int    `json:"new_id"`          // 新 monitors.id
	LegacyProvider string `json:"legacy_provider"` // 旧 provider
	LegacyService  string `json:"legacy_service"`  // 旧 service
	LegacyChannel  string `json:"legacy_channel"`  // 旧 channel
	MigratedAt     int64  `json:"migrated_at"`     // 迁移时间
}

// =====================================================
// v1.0 申请系统类型
// =====================================================

// ApplicationStatus 申请状态
type ApplicationStatus string

const (
	ApplicationStatusPendingTest   ApplicationStatus = "pending_test"   // 待测试
	ApplicationStatusTestFailed    ApplicationStatus = "test_failed"    // 测试失败
	ApplicationStatusTestPassed    ApplicationStatus = "test_passed"    // 测试通过
	ApplicationStatusPendingReview ApplicationStatus = "pending_review" // 待审核
	ApplicationStatusApproved      ApplicationStatus = "approved"       // 已通过
	ApplicationStatusRejected      ApplicationStatus = "rejected"       // 已拒绝
)

// VendorType 服务商类型
type VendorType string

const (
	VendorTypeMerchant   VendorType = "merchant"   // 商家
	VendorTypeIndividual VendorType = "individual" // 个人
)

// MonitorApplication 监测项申请
type MonitorApplication struct {
	ID                int               `json:"id"`
	ApplicantUserID   string            `json:"applicant_user_id"`    // 申请人用户 ID
	ServiceID         string            `json:"service_id"`           // 服务类型
	TemplateID        int               `json:"template_id"`          // 模板 ID
	TemplateSnapshot  json.RawMessage   `json:"template_snapshot"`    // 模板快照
	ProviderName      string            `json:"provider_name"`        // 服务商名称
	ChannelName       string            `json:"channel_name"`         // 通道名称
	VendorType        VendorType        `json:"vendor_type"`          // 服务商类型
	WebsiteURL        string            `json:"website_url"`          // 官网地址
	RequestURL        string            `json:"request_url"`          // API 端点 URL
	APIKeyEncrypted   []byte            `json:"-"`                    // 加密的 API Key（不序列化）
	APIKeyNonce       []byte            `json:"-"`                    // 加密随机数（不序列化）
	APIKeyVersion     int               `json:"-"`                    // API Key 加密版本（不序列化）
	Status            ApplicationStatus `json:"status"`               // 状态
	RejectReason      string            `json:"reject_reason"`        // 拒绝原因
	ReviewerUserID    *string           `json:"reviewer_user_id"`     // 审核人用户 ID
	ReviewedAt        *int64            `json:"reviewed_at"`          // 审核时间
	LastTestSessionID *int              `json:"last_test_session_id"` // 最后测试会话 ID
	CreatedAt         int64             `json:"created_at"`
	UpdatedAt         int64             `json:"updated_at"`

	// 关联数据（查询时填充）
	Applicant   *User                   `json:"applicant,omitempty"`
	Reviewer    *User                   `json:"reviewer,omitempty"`
	TestSession *ApplicationTestSession `json:"test_session,omitempty"`
}

// TestSessionStatus 测试会话状态
type TestSessionStatus string

const (
	TestSessionStatusPending TestSessionStatus = "pending" // 待执行
	TestSessionStatusRunning TestSessionStatus = "running" // 执行中
	TestSessionStatusDone    TestSessionStatus = "done"    // 已完成
)

// ApplicationTestSession 申请测试会话
type ApplicationTestSession struct {
	ID               int               `json:"id"`
	ApplicationID    int               `json:"application_id"`
	TemplateSnapshot json.RawMessage   `json:"template_snapshot"` // 模板快照
	Status           TestSessionStatus `json:"status"`            // 状态
	Summary          json.RawMessage   `json:"summary"`           // 聚合统计
	CreatedAt        int64             `json:"created_at"`
	UpdatedAt        int64             `json:"updated_at"`

	// 关联数据（查询时填充）
	Results []ApplicationTestResult `json:"results,omitempty"`
}

// TestResultStatus 测试结果状态
type TestResultStatus string

const (
	TestResultStatusPass TestResultStatus = "pass"
	TestResultStatusFail TestResultStatus = "fail"
)

// ApplicationTestResult 申请测试结果
type ApplicationTestResult struct {
	ID               int              `json:"id"`
	SessionID        int              `json:"session_id"`
	TemplateModelID  int              `json:"template_model_id"` // 快照中的模型 ID
	ModelKey         string           `json:"model_key"`         // 模型标识
	Status           TestResultStatus `json:"status"`            // pass/fail
	LatencyMs        *int             `json:"latency_ms"`        // 延迟（毫秒）
	HTTPCode         *int             `json:"http_code"`         // HTTP 状态码
	ErrorMessage     string           `json:"error_message"`     // 错误信息
	RequestSnapshot  json.RawMessage  `json:"request_snapshot"`  // 请求快照（脱敏）
	ResponseSnapshot json.RawMessage  `json:"response_snapshot"` // 响应快照（脱敏）
	CheckedAt        int64            `json:"checked_at"`        // 检查时间
}

// TestSummary 测试汇总
type TestSummary struct {
	Total        int `json:"total"`          // 总数
	Passed       int `json:"passed"`         // 通过数
	Failed       int `json:"failed"`         // 失败数
	AvgLatencyMs int `json:"avg_latency_ms"` // 平均延迟
}

// =====================================================
// v1.0 审计日志类型
// =====================================================

// AuditResourceType 审计资源类型
type AuditResourceType string

const (
	AuditResourceUser        AuditResourceType = "user"
	AuditResourceService     AuditResourceType = "service"
	AuditResourceTemplate    AuditResourceType = "template"
	AuditResourceMonitor     AuditResourceType = "monitor"
	AuditResourceApplication AuditResourceType = "application"
	AuditResourceBadge       AuditResourceType = "badge"
)

// AdminAuditLog 管理员审计日志
type AdminAuditLog struct {
	ID           int               `json:"id"`
	UserID       *string           `json:"user_id"`       // 操作者用户 ID
	Action       AuditAction       `json:"action"`        // 操作类型
	ResourceType AuditResourceType `json:"resource_type"` // 资源类型
	ResourceID   string            `json:"resource_id"`   // 资源 ID
	Changes      json.RawMessage   `json:"changes"`       // 变更内容
	IPAddress    string            `json:"ip_address"`    // IP 地址
	UserAgent    string            `json:"user_agent"`    // User-Agent
	CreatedAt    int64             `json:"created_at"`

	// 关联数据（查询时填充）
	User *User `json:"user,omitempty"`
}

// AuditChanges 审计变更内容
type AuditChanges struct {
	Before json.RawMessage `json:"before,omitempty"` // 变更前
	After  json.RawMessage `json:"after,omitempty"`  // 变更后
}
