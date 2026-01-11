package config

import "time"

// SelfTestConfig 自助测试功能配置
type SelfTestConfig struct {
	Enabled            bool   `yaml:"enabled" json:"enabled"`                             // 是否启用自助测试功能（默认禁用）
	MaxConcurrent      int    `yaml:"max_concurrent" json:"max_concurrent"`               // 最大并发测试数（默认 10）
	MaxQueueSize       int    `yaml:"max_queue_size" json:"max_queue_size"`               // 最大队列长度（默认 50）
	JobTimeout         string `yaml:"job_timeout" json:"job_timeout"`                     // 单任务超时时间（默认 "30s"）
	ResultTTL          string `yaml:"result_ttl" json:"result_ttl"`                       // 结果保留时间（默认 "2m"）
	RateLimitPerMinute int    `yaml:"rate_limit_per_minute" json:"rate_limit_per_minute"` // IP 限流（次/分钟，默认 10）
	SignatureSecret    string `yaml:"signature_secret" json:"-"`                          // 签名密钥（不返回给前端）

	// 解析后的时间间隔（内部使用，不序列化）
	JobTimeoutDuration time.Duration `yaml:"-" json:"-"`
	ResultTTLDuration  time.Duration `yaml:"-" json:"-"`
}

// EventsConfig 状态订阅通知（事件）配置
type EventsConfig struct {
	// 是否启用事件功能（默认禁用）
	Enabled bool `yaml:"enabled" json:"enabled"`

	// 事件模式："model"（默认，按模型独立触发）或 "channel"（按通道整体判定）
	// - model: 每个模型独立维护状态机，独立触发 DOWN/UP 事件
	// - channel: 按通道整体判定，任意 N 个模型 DOWN 触发通道 DOWN，所有模型恢复触发通道 UP
	Mode string `yaml:"mode" json:"mode"`

	// 连续 N 次不可用触发 DOWN 事件（默认 2，mode=model 时使用）
	DownThreshold int `yaml:"down_threshold" json:"down_threshold"`

	// 连续 N 次可用触发 UP 事件（默认 1，mode=model 时使用）
	UpThreshold int `yaml:"up_threshold" json:"up_threshold"`

	// 通道级 DOWN 阈值：N 个模型 DOWN 触发通道 DOWN（默认 1，mode=channel 时使用）
	ChannelDownThreshold int `yaml:"channel_down_threshold" json:"channel_down_threshold"`

	// 通道级计数模式（mode=channel 时使用）：
	// - "recompute"（默认）：每次基于活跃模型集合重新计算 down_count/known_count，解决迁移/模型删除等边界问题
	// - "incremental"：增量维护计数，性能最优，适合大规模稳定运行的系统
	ChannelCountMode string `yaml:"channel_count_mode" json:"channel_count_mode"`

	// API 访问令牌（可选，空值表示无鉴权）
	// 配置后需要在请求头中携带 Authorization: Bearer <token>
	APIToken string `yaml:"api_token" json:"-"`
}

// SponsorPinConfig 赞助商置顶配置
// 用于在页面初始加载时置顶符合条件的赞助商监测项
type SponsorPinConfig struct {
	// 是否启用置顶功能（默认 true）
	Enabled *bool `yaml:"enabled" json:"enabled"`

	// 最多置顶数量（默认 3，0 表示禁用）
	MaxPinned int `yaml:"max_pinned" json:"max_pinned"`

	// 服务数量（固定配置值，用于计算赞助商置顶配额；默认 3）
	// enterprise: service_count 个, advanced: max(1, service_count-1) 个, basic: 1 个
	ServiceCount int `yaml:"service_count" json:"service_count"`

	// 最低可用率要求（默认 95.0，百分比 0-100）
	MinUptime float64 `yaml:"min_uptime" json:"min_uptime"`

	// 最低赞助级别（默认 "basic"，可选 basic/advanced/enterprise）
	MinLevel SponsorLevel `yaml:"min_level" json:"min_level"`
}

// IsEnabled 返回是否启用置顶功能
func (c *SponsorPinConfig) IsEnabled() bool {
	if c.Enabled == nil {
		return true // 默认启用
	}
	return *c.Enabled
}

// BoardsConfig 热板/冷板功能配置
// 用于将监测项分为热板（正常监测）和冷板（停止监测，仅展示历史）
type BoardsConfig struct {
	// 是否启用热板/冷板功能（默认 false，保持向后兼容）
	Enabled bool `yaml:"enabled" json:"enabled"`
}
