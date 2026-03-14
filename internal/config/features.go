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

// SponsorPinConfig 赞助通道置顶配置
// 用于在页面初始加载时置顶符合条件的赞助通道
type SponsorPinConfig struct {
	// 是否启用置顶功能（默认 true）
	Enabled *bool `yaml:"enabled" json:"enabled"`

	// 最多置顶数量（默认 3，0 表示禁用）
	MaxPinned int `yaml:"max_pinned" json:"max_pinned"`

	// 最低可用率要求（默认 95.0，百分比 0-100）
	MinUptime float64 `yaml:"min_uptime" json:"min_uptime"`

	// 最低赞助级别（默认 "beacon"，可选 public/signal/pulse/beacon/backbone/core）
	MinLevel SponsorLevel `yaml:"min_level" json:"min_level"`
}

// IsEnabled 返回是否启用置顶功能
func (c *SponsorPinConfig) IsEnabled() bool {
	if c.Enabled == nil {
		return true // 默认启用
	}
	return *c.Enabled
}

// BoardAutoMoveConfig 基于 7 天可用率自动在 hot/secondary/cold 间移板的配置。
// 自动冷板为粘性状态，不会自动恢复，需人工设置 auto_cold_exempt 解除。
type BoardAutoMoveConfig struct {
	// 是否启用自动移板（默认 false）
	Enabled bool `yaml:"enabled" json:"enabled"`

	// 冷板阈值：可用率低于此值 → cold（默认 10.0，百分比 0-100）
	// 自动冷板是 sticky 的，不会自动恢复，需通过 auto_cold_exempt 手动解除
	ThresholdCold float64 `yaml:"threshold_cold" json:"threshold_cold"`

	// 降级阈值：hot 板可用率低于此值 → secondary（默认 50.0，百分比 0-100）
	ThresholdDown float64 `yaml:"threshold_down" json:"threshold_down"`

	// 升级阈值：secondary 板可用率达到此值 → hot（默认 55.0，高于 down 以防抖）
	ThresholdUp float64 `yaml:"threshold_up" json:"threshold_up"`

	// 评估间隔（默认 "30m"）
	CheckInterval string `yaml:"check_interval" json:"check_interval"`

	// 最少探测次数，不足则不判断（新服务商保护，默认 10）
	MinProbes int `yaml:"min_probes" json:"min_probes"`

	// 解析后的运行时字段
	CheckIntervalDuration time.Duration `yaml:"-" json:"-"`
}

// BoardsConfig 热板/冷板功能配置
// 用于将监测项分为热板（正常监测）和冷板（停止监测，仅展示历史）
type BoardsConfig struct {
	// 是否启用热板/冷板功能（默认 false，保持向后兼容）
	Enabled bool `yaml:"enabled" json:"enabled"`

	// 自动移板配置（基于 7 天可用率在 hot/secondary 间切换）
	AutoMove BoardAutoMoveConfig `yaml:"auto_move" json:"auto_move"`
}
