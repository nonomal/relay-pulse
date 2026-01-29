package storage

import "encoding/json"

// ===== 配置管理相关数据类型 =====
// 对应实施方案中定义的 10 张配置管理表

// AuditAction 审计操作类型
type AuditAction string

const (
	AuditActionCreate       AuditAction = "create"
	AuditActionUpdate       AuditAction = "update"
	AuditActionDelete       AuditAction = "delete"
	AuditActionRestore      AuditAction = "restore"
	AuditActionRotateSecret AuditAction = "rotate_secret"
	AuditActionApprove      AuditAction = "approve" // v1.0: 审核通过
	AuditActionReject       AuditAction = "reject"  // v1.0: 审核拒绝
)

// PolicyType Provider 策略类型
type PolicyType string

const (
	PolicyTypeDisabled PolicyType = "disabled"
	PolicyTypeHidden   PolicyType = "hidden"
	PolicyTypeRisk     PolicyType = "risk"
)

// BadgeKind 徽标类型
type BadgeKind string

const (
	BadgeKindSource  BadgeKind = "source" // 标识来源（如 用户Key/官方Key）
	BadgeKindSponsor BadgeKind = "sponsor"
	BadgeKindRisk    BadgeKind = "risk"
	BadgeKindFeature BadgeKind = "feature"
	BadgeKindInfo    BadgeKind = "info"
)

// BadgeScope 徽标绑定作用域
type BadgeScope string

const (
	BadgeScopeGlobal   BadgeScope = "global"
	BadgeScopeProvider BadgeScope = "provider"
	BadgeScopeService  BadgeScope = "service"
	BadgeScopeChannel  BadgeScope = "channel"
)

// ConfigScope 配置元数据作用域
type ConfigScope string

const (
	ConfigScopeMonitors ConfigScope = "monitors"
	ConfigScopePolicies ConfigScope = "policies"
	ConfigScopeBadges   ConfigScope = "badges"
	ConfigScopeBoards   ConfigScope = "boards"
	ConfigScopeSettings ConfigScope = "settings"
)

// MonitorConfig 监测项配置（对应 monitor_configs 表）
//
// 设计说明：
// - 四元组 (provider, service, channel, model) 是业务唯一键，创建后不可修改
// - config_blob 存储 JSON 格式的原始配置，不含派生字段
// - version 用于乐观锁，每次更新递增
// - parent_key 支持父子继承关系
// - enabled 控制是否启用，软删除用 deleted_at
type MonitorConfig struct {
	ID            int64  `db:"id" json:"id"`
	Provider      string `db:"provider" json:"provider"`
	Service       string `db:"service" json:"service"`
	Channel       string `db:"channel" json:"channel"`
	Model         string `db:"model" json:"model"`
	Name          string `db:"name" json:"name,omitempty"` // 可选显示名称
	Enabled       bool   `db:"enabled" json:"enabled"`
	ParentKey     string `db:"parent_key" json:"parent_key,omitempty"` // 父配置引用 "provider/service/channel"
	ConfigBlob    string `db:"config_blob" json:"config_blob"`         // JSON 格式的原始配置
	SchemaVersion int    `db:"schema_version" json:"schema_version"`   // 配置 schema 版本
	Version       int64  `db:"version" json:"version"`                 // 乐观锁版本
	CreatedAt     int64  `db:"created_at" json:"created_at"`           // Unix 秒
	UpdatedAt     int64  `db:"updated_at" json:"updated_at"`           // Unix 秒
	DeletedAt     *int64 `db:"deleted_at" json:"deleted_at,omitempty"` // 软删除时间戳，NULL 表示未删除
	HasAPIKey     bool   `db:"-" json:"has_api_key,omitempty"`         // API 返回时标识是否设置了密钥（不存储）
	APIKeyMasked  string `db:"-" json:"api_key_masked,omitempty"`      // API 返回时的脱敏密钥（不存储）
	ParsedConfig  any    `db:"-" json:"parsed_config,omitempty"`       // 解析后的配置对象（不存储）
}

// MonitorKey 返回监测项的唯一键字符串
func (m *MonitorConfig) MonitorKey() string {
	if m.Model != "" {
		return m.Provider + "/" + m.Service + "/" + m.Channel + "/" + m.Model
	}
	return m.Provider + "/" + m.Service + "/" + m.Channel
}

// IsDeleted 返回是否已软删除
func (m *MonitorConfig) IsDeleted() bool {
	return m.DeletedAt != nil && *m.DeletedAt > 0
}

// MonitorSecret API Key 加密存储（对应 monitor_secrets 表）
//
// 设计说明：
// - 使用 AES-256-GCM 加密，api_key_ciphertext 存储密文
// - api_key_nonce 存储随机 nonce（每次加密生成新的）
// - key_version 支持密钥轮换（多版本密钥共存）
// - monitor_id 作为 AAD (Additional Authenticated Data)，防止密钥被换绑
type MonitorSecret struct {
	MonitorID        int64  `db:"monitor_id" json:"monitor_id"`   // 主键，关联 monitor_configs.id
	APIKeyCiphertext []byte `db:"api_key_ciphertext" json:"-"`    // AES-256-GCM 密文
	APIKeyNonce      []byte `db:"api_key_nonce" json:"-"`         // GCM nonce
	KeyVersion       int    `db:"key_version" json:"key_version"` // 加密密钥版本（用于轮换）
	EncVersion       int    `db:"enc_version" json:"enc_version"` // 加密算法版本
	CreatedAt        int64  `db:"created_at" json:"created_at"`
	UpdatedAt        int64  `db:"updated_at" json:"updated_at"`
}

// MonitorConfigAudit 配置变更审计记录（对应 monitor_config_audits 表）
//
// 设计说明：
// - 冗余四元组以便跨监测项生命周期查询
// - before_blob/after_blob 存储变更前后的完整配置快照
// - secret_changed 仅标记密钥是否变更，不记录密钥内容
// - batch_id 用于批量操作归组
type MonitorConfigAudit struct {
	ID            int64       `db:"id" json:"id"`
	MonitorID     int64       `db:"monitor_id" json:"monitor_id"`
	Provider      string      `db:"provider" json:"provider"`
	Service       string      `db:"service" json:"service"`
	Channel       string      `db:"channel" json:"channel"`
	Model         string      `db:"model" json:"model"`
	Action        AuditAction `db:"action" json:"action"`                     // create/update/delete/restore/rotate_secret
	BeforeBlob    string      `db:"before_blob" json:"before_blob,omitempty"` // 变更前快照
	AfterBlob     string      `db:"after_blob" json:"after_blob,omitempty"`   // 变更后快照
	BeforeVersion *int64      `db:"before_version" json:"before_version,omitempty"`
	AfterVersion  *int64      `db:"after_version" json:"after_version,omitempty"`
	SecretChanged bool        `db:"secret_changed" json:"secret_changed"`   // 是否涉及密钥变更
	Actor         string      `db:"actor" json:"actor,omitempty"`           // 操作者标识（token 名称）
	ActorIP       string      `db:"actor_ip" json:"actor_ip,omitempty"`     // 来源 IP
	UserAgent     string      `db:"user_agent" json:"user_agent,omitempty"` // User-Agent
	RequestID     string      `db:"request_id" json:"request_id,omitempty"` // 请求追踪 ID
	BatchID       string      `db:"batch_id" json:"batch_id,omitempty"`     // 批量操作归组 ID
	Reason        string      `db:"reason" json:"reason,omitempty"`         // 变更原因
	CreatedAt     int64       `db:"created_at" json:"created_at"`
}

// ProviderPolicy Provider 策略配置（对应 provider_policies 表）
//
// 设计说明：
// - policy_type 区分不同策略类型（disabled/hidden/risk）
// - (policy_type, provider) 唯一约束
// - risks 为 JSON 数组，仅 risk 类型使用
type ProviderPolicy struct {
	ID         int64      `db:"id" json:"id"`
	PolicyType PolicyType `db:"policy_type" json:"policy_type"` // disabled/hidden/risk
	Provider   string     `db:"provider" json:"provider"`
	Reason     string     `db:"reason" json:"reason,omitempty"`
	Risks      string     `db:"risks" json:"risks,omitempty"` // JSON 数组，用于 risk 类型
	CreatedAt  int64      `db:"created_at" json:"created_at"`
	UpdatedAt  int64      `db:"updated_at" json:"updated_at"`
}

// ParseRisks 解析 Risks JSON 字段为结构体数组
func (p *ProviderPolicy) ParseRisks() ([]RiskBadgeData, error) {
	if p.Risks == "" {
		return nil, nil
	}
	var risks []RiskBadgeData
	if err := json.Unmarshal([]byte(p.Risks), &risks); err != nil {
		return nil, err
	}
	return risks, nil
}

// RiskBadgeData 风险徽标数据
type RiskBadgeData struct {
	Kind    string `json:"kind"`
	Variant string `json:"variant,omitempty"`
}

// BadgeDefinition 徽标定义（对应 badge_definitions 表）
//
// 设计说明：
// - id 是徽标的唯一标识符（如 "official", "high_frequency"）
// - label_i18n/tooltip_i18n 是 JSON 格式的多语言文本
// - weight 用于排序，值越大优先级越高
// - category 用于分类：sponsor_level/metric/negative/vendor_type
// - svg_source 存储 SVG 图标源码
type BadgeDefinition struct {
	ID          string    `db:"id" json:"id"`                               // 徽标 ID（主键）
	Kind        BadgeKind `db:"kind" json:"kind"`                           // sponsor/risk/feature/info
	Weight      int       `db:"weight" json:"weight"`                       // 排序权重
	LabelI18n   string    `db:"label_i18n" json:"label_i18n"`               // JSON: {"zh-CN":"...", "en-US":"..."}
	TooltipI18n string    `db:"tooltip_i18n" json:"tooltip_i18n,omitempty"` // JSON: {"zh-CN":"...", "en-US":"..."}
	Icon        string    `db:"icon" json:"icon,omitempty"`                 // 图标标识（旧字段，保留兼容）
	Color       string    `db:"color" json:"color,omitempty"`               // 颜色代码
	Category    string    `db:"category" json:"category,omitempty"`         // 分类：sponsor_level/metric/negative/vendor_type
	SVGSource   string    `db:"svg_source" json:"svg_source,omitempty"`     // SVG 图标源码
	CreatedAt   int64     `db:"created_at" json:"created_at"`
	UpdatedAt   int64     `db:"updated_at" json:"updated_at"`
}

// BadgeBinding 徽标绑定（对应 badge_bindings 表）
//
// 设计说明：
// - 支持四级作用域：global/provider/service/channel
// - provider/service/channel 字段根据 scope 选择性填充
// - tooltip_override 用于覆盖徽标的默认提示文本
type BadgeBinding struct {
	ID              int64      `db:"id" json:"id"`
	BadgeID         string     `db:"badge_id" json:"badge_id"`                           // 关联 badge_definitions.id
	Scope           BadgeScope `db:"scope" json:"scope"`                                 // global/provider/service/channel
	Provider        string     `db:"provider" json:"provider,omitempty"`                 // scope>=provider 时必填
	Service         string     `db:"service" json:"service,omitempty"`                   // scope>=service 时必填
	Channel         string     `db:"channel" json:"channel,omitempty"`                   // scope=channel 时必填
	TooltipOverride string     `db:"tooltip_override" json:"tooltip_override,omitempty"` // JSON: {"zh-CN":"...", "en-US":"..."}
	CreatedAt       int64      `db:"created_at" json:"created_at"`
	UpdatedAt       int64      `db:"updated_at" json:"updated_at"`
}

// BoardConfig Board 配置（对应 board_configs 表）
//
// 设计说明：
// - board 是主键（如 "hot", "secondary", "cold"）
// - sort_order 用于 Board 列表排序
type BoardConfig struct {
	Board       string `db:"board" json:"board"` // 主键：hot/secondary/cold
	DisplayName string `db:"display_name" json:"display_name"`
	Description string `db:"description" json:"description,omitempty"`
	SortOrder   int    `db:"sort_order" json:"sort_order"`
	CreatedAt   int64  `db:"created_at" json:"created_at"`
	UpdatedAt   int64  `db:"updated_at" json:"updated_at"`
}

// BoardItem Board 成员关联（对应 board_items 表）
//
// 设计说明：
// - (board, monitor_id) 唯一约束
// - sort_order 用于 Board 内排序
type BoardItem struct {
	ID        int64  `db:"id" json:"id"`
	Board     string `db:"board" json:"board"`           // 关联 board_configs.board
	MonitorID int64  `db:"monitor_id" json:"monitor_id"` // 关联 monitor_configs.id
	SortOrder int    `db:"sort_order" json:"sort_order"`
	CreatedAt int64  `db:"created_at" json:"created_at"`
	UpdatedAt int64  `db:"updated_at" json:"updated_at"`
}

// GlobalSetting 全局键值配置（对应 global_settings 表）
//
// 设计说明：
// - key 是主键，存储配置项名称
// - value 存储 JSON 格式的配置值
// - version 用于乐观锁
type GlobalSetting struct {
	Key           string `db:"key" json:"key"` // 主键
	Value         string `db:"value" json:"value"`
	SchemaVersion int    `db:"schema_version" json:"schema_version"`
	Version       int64  `db:"version" json:"version"`
	CreatedAt     int64  `db:"created_at" json:"created_at"`
	UpdatedAt     int64  `db:"updated_at" json:"updated_at"`
}

// ConfigMeta 配置版本元数据（对应 config_meta 表）
//
// 设计说明：
// - scope 是主键（monitors/policies/badges/boards/settings）
// - version 每次写入时递增，用于热更新检测
// - 客户端通过轮询此表检测配置变更
type ConfigMeta struct {
	Scope     ConfigScope `db:"scope" json:"scope"` // 主键
	Version   int64       `db:"version" json:"version"`
	UpdatedAt int64       `db:"updated_at" json:"updated_at"`
}

// ===== 批量操作相关类型 =====

// BatchOperation 批量操作请求
type BatchOperation struct {
	Action    AuditAction       `json:"action"`               // create/update/delete/restore
	MonitorID int64             `json:"monitor_id,omitempty"` // update/delete/restore 时必填
	Config    *MonitorConfigDTO `json:"config,omitempty"`     // create/update 时必填
}

// MonitorConfigDTO 监测项配置数据传输对象（API 层使用）
type MonitorConfigDTO struct {
	Provider      string `json:"provider"`
	Service       string `json:"service"`
	Channel       string `json:"channel"`
	Model         string `json:"model,omitempty"`
	Name          string `json:"name,omitempty"`
	Enabled       *bool  `json:"enabled,omitempty"`
	ParentKey     string `json:"parent_key,omitempty"`
	APIKey        string `json:"api_key,omitempty"` // 仅写入时使用，不会返回
	Version       int64  `json:"version,omitempty"` // 更新时用于乐观锁校验
	ConfigPayload any    `json:"config"`            // 原始配置对象（URL, Method, Headers, Body 等）
}

// BatchOperationResult 批量操作结果
type BatchOperationResult struct {
	Index     int    `json:"index"`
	Success   bool   `json:"success"`
	MonitorID int64  `json:"monitor_id,omitempty"`
	Error     string `json:"error,omitempty"`
}

// ImportResult 导入配置结果
type ImportResult struct {
	Created int      `json:"created"`
	Skipped int      `json:"skipped"`
	Errors  []string `json:"errors,omitempty"`
}

// ===== 查询过滤器 =====

// MonitorConfigFilter 监测项配置查询过滤器
type MonitorConfigFilter struct {
	Provider       string `json:"provider,omitempty"`
	Service        string `json:"service,omitempty"`
	Channel        string `json:"channel,omitempty"`
	Model          string `json:"model,omitempty"`
	Enabled        *bool  `json:"enabled,omitempty"`
	IncludeDeleted bool   `json:"include_deleted,omitempty"`
	Search         string `json:"search,omitempty"` // 模糊搜索 name/provider/service/channel
	Offset         int    `json:"offset,omitempty"`
	Limit          int    `json:"limit,omitempty"`
}

// AuditFilter 审计记录查询过滤器
type AuditFilter struct {
	MonitorID int64       `json:"monitor_id,omitempty"`
	Provider  string      `json:"provider,omitempty"`
	Service   string      `json:"service,omitempty"`
	Action    AuditAction `json:"action,omitempty"`
	Actor     string      `json:"actor,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
	BatchID   string      `json:"batch_id,omitempty"`
	Since     int64       `json:"since,omitempty"` // Unix 秒
	Until     int64       `json:"until,omitempty"` // Unix 秒
	Offset    int         `json:"offset,omitempty"`
	Limit     int         `json:"limit,omitempty"`
}

// BadgeBindingFilter 徽标绑定查询过滤器
type BadgeBindingFilter struct {
	BadgeID  string     `json:"badge_id,omitempty"`
	Scope    BadgeScope `json:"scope,omitempty"` // global/provider/service/channel
	Provider string     `json:"provider,omitempty"`
	Service  string     `json:"service,omitempty"`
	Channel  string     `json:"channel,omitempty"`
}

// ConfigVersions 所有 scope 的配置版本
type ConfigVersions struct {
	Monitors int64 `json:"monitors"`
	Policies int64 `json:"policies"`
	Badges   int64 `json:"badges"`
	Boards   int64 `json:"boards"`
	Settings int64 `json:"settings"`
}
