package change

import "context"

// RequestStatus 变更请求状态
type RequestStatus string

const (
	StatusPending  RequestStatus = "pending"
	StatusApproved RequestStatus = "approved"
	StatusRejected RequestStatus = "rejected"
	StatusApplied  RequestStatus = "applied"
)

// ChangeRequest 变更请求
type ChangeRequest struct {
	ID       int64         `json:"id"`
	PublicID string        `json:"public_id"`
	Status   RequestStatus `json:"status"`

	// 目标通道
	TargetProvider string `json:"target_provider"`
	TargetService  string `json:"target_service"`
	TargetChannel  string `json:"target_channel"`
	TargetKey      string `json:"target_key"` // provider--service--channel
	ApplyMode      string `json:"apply_mode"` // "auto" | "manual"

	// 认证
	AuthFingerprint string `json:"auth_fingerprint"`
	AuthLast4       string `json:"auth_last4"`

	// 变更内容
	CurrentSnapshot string `json:"current_snapshot"` // JSON: 提交时的当前值快照
	ProposedChanges string `json:"proposed_changes"` // JSON: {field: newValue} patch

	// 新 API Key（如有变更）
	NewKeyEncrypted   string `json:"new_key_encrypted,omitempty"`
	NewKeyFingerprint string `json:"new_key_fingerprint,omitempty"`
	NewKeyLast4       string `json:"new_key_last4,omitempty"`

	// 测试（base_url/api_key 变更时必须）
	RequiresTest bool   `json:"requires_test"`
	TestJobID    string `json:"test_job_id,omitempty"`
	TestPassedAt int64  `json:"test_passed_at,omitempty"`
	TestLatency  int    `json:"test_latency_ms,omitempty"`
	TestHTTPCode int    `json:"test_http_code,omitempty"`

	// 管理
	AdminNote  string `json:"admin_note,omitempty"`
	ReviewedAt *int64 `json:"reviewed_at,omitempty"`
	AppliedAt  *int64 `json:"applied_at,omitempty"`

	// 提交者元数据
	SubmitterIPHash string `json:"submitter_ip_hash,omitempty"`
	Locale          string `json:"locale,omitempty"`

	// 时间戳
	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

// AuthCandidate 认证后返回的通道候选
type AuthCandidate struct {
	Provider   string `json:"provider"`
	Service    string `json:"service"`
	Channel    string `json:"channel"`
	MonitorKey string `json:"monitor_key"` // provider--service--channel
	ApplyMode  string `json:"apply_mode"`  // "auto" | "manual"

	// 当前可编辑值
	ProviderName string `json:"provider_name"`
	ProviderURL  string `json:"provider_url"`
	ChannelName  string `json:"channel_name"`
	Category     string `json:"category"`
	SponsorLevel string `json:"sponsor_level"`
	BaseURL      string `json:"base_url"`
	KeyLast4     string `json:"key_last4"`
}

// Store 变更请求持久化接口
type Store interface {
	Save(ctx context.Context, r *ChangeRequest) error
	GetByPublicID(ctx context.Context, publicID string) (*ChangeRequest, error)
	List(ctx context.Context, status string, limit, offset int) ([]*ChangeRequest, int, error)
	Update(ctx context.Context, r *ChangeRequest) error
	CountByIPToday(ctx context.Context, ipHash string) (int, error)
	DeleteByPublicID(ctx context.Context, publicID string) error
}
