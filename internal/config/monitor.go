package config

import "time"

// ServiceConfig 单个服务监测配置
type ServiceConfig struct {
	Provider       string            `yaml:"provider" json:"provider"`
	ProviderName   string            `yaml:"provider_name" json:"provider_name,omitempty"` // Provider 显示名称（可选，未配置时回退到 provider）
	ProviderSlug   string            `yaml:"provider_slug" json:"provider_slug"`           // URL slug（可选，未配置时使用 provider 小写）
	ProviderURL    string            `yaml:"provider_url" json:"provider_url"`             // 服务商官网链接（可选）
	Service        string            `yaml:"service" json:"service"`
	ServiceName    string            `yaml:"service_name" json:"service_name,omitempty"` // Service 显示名称（可选，未配置时回退到 service）
	Category       string            `yaml:"category" json:"category"`                   // 分类：commercial（商业站）或 public（公益站）
	Sponsor        string            `yaml:"sponsor" json:"sponsor"`                     // 赞助者：提供 API Key 的个人或组织
	SponsorURL     string            `yaml:"sponsor_url" json:"sponsor_url"`             // 赞助者链接（可选）
	SponsorLevel   SponsorLevel      `yaml:"sponsor_level" json:"sponsor_level"`         // 赞助商等级：basic/advanced/enterprise（可选）
	PriceMin       *float64          `yaml:"price_min" json:"price_min"`                 // 参考倍率下限（可选，如 0.05）
	PriceMax       *float64          `yaml:"price_max" json:"price_max"`                 // 参考倍率（可选，如 0.2）
	Risks          []RiskBadge       `yaml:"-" json:"risks,omitempty"`                   // 风险徽标（由 risk_providers 自动注入，不在此配置）
	Badges         []BadgeRef        `yaml:"badges" json:"-"`                            // 徽标引用（可选，支持 tooltip 覆盖）
	ResolvedBadges []ResolvedBadge   `yaml:"-" json:"badges,omitempty"`                  // 解析后的徽标（由 badges + badge_providers 注入）
	Channel        string            `yaml:"channel" json:"channel"`                     // 业务通道标识（如 "vip-channel"），用于分类和过滤
	Model          string            `yaml:"model" json:"model,omitempty"`               // 模型名称（父子结构必填）
	Parent         string            `yaml:"parent" json:"parent,omitempty"`             // 父通道引用，格式 provider/service/channel
	ChannelName    string            `yaml:"channel_name" json:"channel_name,omitempty"` // Channel 显示名称（可选，未配置时回退到 channel）
	ListedSince    string            `yaml:"listed_since" json:"listed_since"`           // 收录日期（可选，格式 "2006-01-02"），用于计算收录天数
	URL            string            `yaml:"url" json:"url"`
	Method         string            `yaml:"method" json:"method"`
	Headers        map[string]string `yaml:"headers" json:"headers"`
	Body           string            `yaml:"body" json:"body"`

	// SuccessContains 可选：响应体需包含的关键字，用于判定请求语义是否成功
	SuccessContains string `yaml:"success_contains" json:"success_contains"`

	// EnvVarName 可选：自定义环境变量名（用于解决channel名称冲突）
	// 如果指定，则使用此名称覆盖 APIKey，否则使用自动生成的 MONITOR_{PROVIDER}_{SERVICE}_{CHANNEL}_API_KEY
	EnvVarName string `yaml:"env_var_name" json:"-"`

	// 自定义巡检间隔（可选，留空则使用全局 interval）
	// 支持 Go duration 格式，例如 "30s"、"1m"、"5m"
	// 付费高频监测可使用更短间隔
	Interval string `yaml:"interval" json:"interval"`

	// 彻底停用配置：不探测、不存储、不展示
	// Disabled 为 true 时，调度器不会创建任务，API 不返回，探测结果不写库
	Disabled       bool   `yaml:"disabled" json:"disabled"`
	DisabledReason string `yaml:"disabled_reason" json:"disabled_reason"` // 停用原因（可选）

	// 临时下架配置：隐藏但继续探测，用于商家整改期间
	// Hidden 为 true 时，API 不返回该监测项，但调度器继续探测并存储结果
	Hidden       bool   `yaml:"hidden" json:"hidden"`
	HiddenReason string `yaml:"hidden_reason" json:"hidden_reason"` // 下架原因（可选）

	// 热板/冷板配置：冷板项停止探测，仅展示历史数据（需 boards.enabled=true）
	// Board 可选值：空/"hot"（默认热板）、"cold"（冷板）
	Board      string `yaml:"board" json:"board"`
	ColdReason string `yaml:"cold_reason" json:"cold_reason,omitempty"` // 冷板原因（可选）

	// 通道级慢请求阈值（可选，覆盖 slow_latency_by_service 和全局 slow_latency）
	// 支持 Go duration 格式，例如 "5s"、"15s"
	SlowLatency string `yaml:"slow_latency" json:"slow_latency"`

	// 解析后的"慢请求"阈值，用于黄灯判定
	// 优先级：monitor.slow_latency > slow_latency_by_service > 全局 slow_latency
	SlowLatencyDuration time.Duration `yaml:"-" json:"-"`

	// 通道级超时时间（可选，覆盖 timeout_by_service 和全局 timeout）
	// 支持 Go duration 格式，例如 "10s"、"30s"
	Timeout string `yaml:"timeout" json:"timeout"`

	// 解析后的超时时间
	// 优先级：monitor.timeout > timeout_by_service > 全局 timeout
	TimeoutDuration time.Duration `yaml:"-" json:"-"`

	// 解析后的巡检间隔（可选，为空时使用全局 interval）
	IntervalDuration time.Duration `yaml:"-" json:"-"`

	// BodyTemplateName 请求体模板文件名（如 cc_base.json）
	// 在配置加载时从 body: "!include data/xxx.json" 提取，供 API 返回
	BodyTemplateName string `yaml:"-" json:"-"`

	APIKey string `yaml:"api_key" json:"-"` // 不返回给前端
}

// DisabledProviderConfig 批量禁用指定 provider 的配置
// 用于彻底停用某个服务商的所有监测项（不探测、不存储、不展示）
type DisabledProviderConfig struct {
	Provider string `yaml:"provider" json:"provider"` // provider 名称，需与 monitors 中的 provider 完全匹配
	Reason   string `yaml:"reason" json:"reason"`     // 停用原因（可选）
}

// HiddenProviderConfig 批量隐藏指定 provider 的配置
// 用于临时下架某个服务商的所有监测项
type HiddenProviderConfig struct {
	Provider string `yaml:"provider" json:"provider"` // provider 名称，需与 monitors 中的 provider 完全匹配
	Reason   string `yaml:"reason" json:"reason"`     // 下架原因（可选）
}

// RiskProviderConfig 服务商风险配置
// 用于标记存在风险的服务商，风险会自动继承到该服务商的所有监测项
type RiskProviderConfig struct {
	Provider string      `yaml:"provider" json:"provider"` // provider 名称，需与 monitors 中的 provider 完全匹配
	Risks    []RiskBadge `yaml:"risks" json:"risks"`       // 风险徽标数组
}
