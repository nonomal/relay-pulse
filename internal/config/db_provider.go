package config

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

// AdminConfigStorage 配置管理存储接口
// 定义在 config 包中以避免与 storage 包的循环依赖
// 由 storage.PostgresStorage 实现
type AdminConfigStorage interface {
	// MonitorConfig 相关
	ListMonitorConfigs(filter *MonitorConfigFilter) ([]*MonitorConfigRecord, int, error)
	GetMonitorSecret(monitorID int64) (*MonitorSecretRecord, error)

	// Provider 策略相关
	ListProviderPolicies() ([]*ProviderPolicyRecord, error)

	// 徽标相关
	ListBadgeDefinitions() ([]*BadgeDefinitionRecord, error)
	ListBadgeBindings(filter *BadgeBindingFilter) ([]*BadgeBindingRecord, error)

	// Board 相关
	ListBoardConfigs() ([]*BoardConfigRecord, error)

	// 全局设置相关
	GetGlobalSetting(key string) (*GlobalSettingRecord, error)

	// 配置版本相关
	GetConfigVersions() (*ConfigVersionsRecord, error)
}

// ===== 记录类型定义（镜像 storage 包中的类型） =====

// MonitorConfigFilter 监测项配置查询过滤器
type MonitorConfigFilter struct {
	Provider       string `json:"provider,omitempty"`
	Service        string `json:"service,omitempty"`
	Channel        string `json:"channel,omitempty"`
	Model          string `json:"model,omitempty"`
	Enabled        *bool  `json:"enabled,omitempty"`
	IncludeDeleted bool   `json:"include_deleted,omitempty"`
	Search         string `json:"search,omitempty"`
	Offset         int    `json:"offset,omitempty"`
	Limit          int    `json:"limit,omitempty"`
}

// MonitorConfigRecord 监测项配置记录
type MonitorConfigRecord struct {
	ID         int64
	Provider   string
	Service    string
	Channel    string
	Model      string
	Name       string
	Enabled    bool
	ParentKey  string
	ConfigBlob string
	Version    int64
	DeletedAt  *int64
}

// MonitorSecretRecord API Key 加密存储记录
type MonitorSecretRecord struct {
	MonitorID        int64
	APIKeyCiphertext []byte
	APIKeyNonce      []byte
	KeyVersion       int
	EncVersion       int
}

// ProviderPolicyRecord Provider 策略记录
type ProviderPolicyRecord struct {
	ID         int64
	PolicyType string // "disabled", "hidden", "risk"
	Provider   string
	Reason     string
	Risks      string // JSON 数组
}

// BadgeDefinitionRecord 徽标定义记录
type BadgeDefinitionRecord struct {
	ID          string
	Kind        string // "sponsor", "risk", "feature", "info"
	Weight      int
	LabelI18n   string
	TooltipI18n string
	Icon        string
	Color       string
	Category    string // sponsor_level/metric/negative/vendor_type
	SVGSource   string // SVG 图标源码
}

// BadgeBindingFilter 徽标绑定查询过滤器
type BadgeBindingFilter struct {
	BadgeID  string `json:"badge_id,omitempty"`
	Scope    string `json:"scope,omitempty"` // "global", "provider", "service", "channel"
	Provider string `json:"provider,omitempty"`
	Service  string `json:"service,omitempty"`
	Channel  string `json:"channel,omitempty"`
}

// BadgeBindingRecord 徽标绑定记录
type BadgeBindingRecord struct {
	ID              int64
	BadgeID         string
	Scope           string
	Provider        string
	Service         string
	Channel         string
	TooltipOverride string // JSON 格式的多语言文本
}

// BoardConfigRecord Board 配置记录
type BoardConfigRecord struct {
	Board       string
	DisplayName string
	Description string
	SortOrder   int
}

// GlobalSettingRecord 全局设置记录
type GlobalSettingRecord struct {
	Key     string
	Value   string
	Version int64
}

// ConfigVersionsRecord 配置版本记录
type ConfigVersionsRecord struct {
	Monitors int64
	Policies int64
	Badges   int64
	Boards   int64
	Settings int64
}

// ===== DBConfigProvider 实现 =====

// DBConfigProvider 从数据库加载配置的提供者
// 实现从数据库表到 AppConfig 结构的转换
type DBConfigProvider struct {
	storage AdminConfigStorage
}

// NewDBConfigProvider 创建数据库配置提供者
func NewDBConfigProvider(s AdminConfigStorage) *DBConfigProvider {
	return &DBConfigProvider{storage: s}
}

// MonitorConfigPayload ConfigBlob 中存储的 JSON 结构
// 包含 ServiceConfig 中需要持久化的探测配置字段
type MonitorConfigPayload struct {
	// 探测配置
	URL             string            `json:"url"`
	Method          string            `json:"method"`
	Headers         map[string]string `json:"headers,omitempty"`
	Body            string            `json:"body,omitempty"`
	SuccessContains string            `json:"success_contains,omitempty"`

	// 时间配置（可选，覆盖全局/服务级设置）
	Interval    string `json:"interval,omitempty"`
	SlowLatency string `json:"slow_latency,omitempty"`
	Timeout     string `json:"timeout,omitempty"`

	// 重试配置
	Retry          *int     `json:"retry,omitempty"`
	RetryBaseDelay string   `json:"retry_base_delay,omitempty"`
	RetryMaxDelay  string   `json:"retry_max_delay,omitempty"`
	RetryJitter    *float64 `json:"retry_jitter,omitempty"`

	// 代理配置
	Proxy string `json:"proxy,omitempty"`

	// 元数据
	ProviderName string `json:"provider_name,omitempty"`
	ProviderSlug string `json:"provider_slug,omitempty"`
	ProviderURL  string `json:"provider_url,omitempty"`
	ServiceName  string `json:"service_name,omitempty"`
	ChannelName  string `json:"channel_name,omitempty"`

	// 分类和赞助
	Category     string       `json:"category,omitempty"`
	Sponsor      string       `json:"sponsor,omitempty"`
	SponsorURL   string       `json:"sponsor_url,omitempty"`
	SponsorLevel SponsorLevel `json:"sponsor_level,omitempty"`

	// 价格区间
	PriceMin *float64 `json:"price_min,omitempty"`
	PriceMax *float64 `json:"price_max,omitempty"`

	// 徽标引用
	Badges []BadgeRef `json:"badges,omitempty"`

	// 收录日期
	ListedSince string `json:"listed_since,omitempty"`

	// 板块配置
	Board      string `json:"board,omitempty"`
	ColdReason string `json:"cold_reason,omitempty"`

	// 禁用/隐藏配置
	Disabled       bool   `json:"disabled,omitempty"`
	DisabledReason string `json:"disabled_reason,omitempty"`
	Hidden         bool   `json:"hidden,omitempty"`
	HiddenReason   string `json:"hidden_reason,omitempty"`

	// 自定义环境变量名（用于环境变量覆盖）
	EnvVarName string `json:"env_var_name,omitempty"`
}

// GlobalSettingsPayload 全局设置 JSON 结构
// 存储在 global_settings 表的 value 字段中
type GlobalSettingsPayload struct {
	// 探测时间配置
	Interval             string            `json:"interval"`
	SlowLatency          string            `json:"slow_latency"`
	SlowLatencyByService map[string]string `json:"slow_latency_by_service,omitempty"`
	Timeout              string            `json:"timeout"`
	TimeoutByService     map[string]string `json:"timeout_by_service,omitempty"`

	// 重试配置
	Retry                   *int               `json:"retry,omitempty"`
	RetryByService          map[string]int     `json:"retry_by_service,omitempty"`
	RetryBaseDelay          string             `json:"retry_base_delay,omitempty"`
	RetryBaseDelayByService map[string]string  `json:"retry_base_delay_by_service,omitempty"`
	RetryMaxDelay           string             `json:"retry_max_delay,omitempty"`
	RetryMaxDelayByService  map[string]string  `json:"retry_max_delay_by_service,omitempty"`
	RetryJitter             *float64           `json:"retry_jitter,omitempty"`
	RetryJitterByService    map[string]float64 `json:"retry_jitter_by_service,omitempty"`

	// 运行时配置
	DegradedWeight        float64        `json:"degraded_weight"`
	MaxConcurrency        int            `json:"max_concurrency,omitempty"`
	StaggerProbes         *bool          `json:"stagger_probes,omitempty"`
	EnableConcurrentQuery bool           `json:"enable_concurrent_query,omitempty"`
	ConcurrentQueryLimit  int            `json:"concurrent_query_limit,omitempty"`
	EnableBatchQuery      bool           `json:"enable_batch_query,omitempty"`
	EnableDBTimelineAgg   bool           `json:"enable_db_timeline_agg,omitempty"`
	BatchQueryMaxKeys     int            `json:"batch_query_max_keys,omitempty"`
	CacheTTL              CacheTTLConfig `json:"cache_ttl,omitempty"`

	// 功能开关
	ExposeChannelDetails *bool            `json:"expose_channel_details,omitempty"`
	EnableBadges         bool             `json:"enable_badges,omitempty"`
	SponsorPin           SponsorPinConfig `json:"sponsor_pin,omitempty"`

	// Boards 配置
	Boards BoardsConfig `json:"boards,omitempty"`

	// 徽标定义
	BadgeDefs map[string]BadgeDef `json:"badge_definitions,omitempty"`
}

// LoadMonitors 从数据库加载监测项配置并转换为 ServiceConfig 切片
func (p *DBConfigProvider) LoadMonitors(ctx context.Context) ([]ServiceConfig, error) {
	// 查询所有启用且未删除的监测项
	filter := &MonitorConfigFilter{
		Enabled:        boolPtr(true),
		IncludeDeleted: false,
		Limit:          -1, // 无限制，加载所有监测项
	}

	monitorConfigs, _, err := p.storage.ListMonitorConfigs(filter)
	if err != nil {
		return nil, fmt.Errorf("从数据库加载监测项失败: %w", err)
	}

	result := make([]ServiceConfig, 0, len(monitorConfigs))
	for _, mc := range monitorConfigs {
		sc, err := p.convertMonitorConfig(ctx, mc)
		if err != nil {
			// 记录错误但继续处理其他配置
			// 避免单个配置错误导致整体加载失败
			continue
		}
		result = append(result, *sc)
	}

	return result, nil
}

// convertMonitorConfig 将 MonitorConfigRecord 转换为 ServiceConfig
func (p *DBConfigProvider) convertMonitorConfig(_ context.Context, mc *MonitorConfigRecord) (*ServiceConfig, error) {
	// 解析 ConfigBlob JSON
	var payload MonitorConfigPayload
	if mc.ConfigBlob != "" {
		if err := json.Unmarshal([]byte(mc.ConfigBlob), &payload); err != nil {
			return nil, fmt.Errorf("解析 ConfigBlob 失败 (monitor_id=%d): %w", mc.ID, err)
		}
	}

	// 构建 ServiceConfig
	sc := &ServiceConfig{
		Provider:        mc.Provider,
		Service:         mc.Service,
		Channel:         mc.Channel,
		Model:           mc.Model,
		Parent:          mc.ParentKey,
		URL:             payload.URL,
		Method:          payload.Method,
		Headers:         payload.Headers,
		Body:            payload.Body,
		SuccessContains: payload.SuccessContains,
		Interval:        payload.Interval,
		SlowLatency:     payload.SlowLatency,
		Timeout:         payload.Timeout,
		Retry:           payload.Retry,
		RetryBaseDelay:  payload.RetryBaseDelay,
		RetryMaxDelay:   payload.RetryMaxDelay,
		RetryJitter:     payload.RetryJitter,
		Proxy:           payload.Proxy,
		ProviderName:    payload.ProviderName,
		ProviderSlug:    payload.ProviderSlug,
		ProviderURL:     payload.ProviderURL,
		ServiceName:     payload.ServiceName,
		ChannelName:     payload.ChannelName,
		Category:        payload.Category,
		Sponsor:         payload.Sponsor,
		SponsorURL:      payload.SponsorURL,
		SponsorLevel:    payload.SponsorLevel,
		PriceMin:        payload.PriceMin,
		PriceMax:        payload.PriceMax,
		Badges:          payload.Badges,
		ListedSince:     payload.ListedSince,
		Board:           payload.Board,
		ColdReason:      payload.ColdReason,
		Disabled:        payload.Disabled,
		DisabledReason:  payload.DisabledReason,
		Hidden:          payload.Hidden,
		HiddenReason:    payload.HiddenReason,
		EnvVarName:      payload.EnvVarName,
	}

	// 如果有显示名称配置，优先使用
	if mc.Name != "" {
		sc.ChannelName = mc.Name
	}

	// 加载并解密 API Key
	apiKey, err := p.loadAPIKey(mc.ID)
	if err != nil {
		// API Key 解密失败不应阻止加载
		// 该监测项将无法正常探测，但不影响其他项
		sc.APIKey = ""
	} else {
		sc.APIKey = apiKey
	}

	return sc, nil
}

// loadAPIKey 从数据库加载并解密 API Key
func (p *DBConfigProvider) loadAPIKey(monitorID int64) (string, error) {
	secret, err := p.storage.GetMonitorSecret(monitorID)
	if err != nil {
		return "", fmt.Errorf("获取密钥失败 (monitor_id=%d): %w", monitorID, err)
	}
	if secret == nil {
		// 没有设置 API Key
		return "", nil
	}

	// 解密 API Key
	apiKey, err := DecryptAPIKey(
		secret.APIKeyCiphertext,
		secret.APIKeyNonce,
		monitorID,
		secret.KeyVersion,
	)
	if err != nil {
		return "", fmt.Errorf("解密 API Key 失败 (monitor_id=%d): %w", monitorID, err)
	}

	return apiKey, nil
}

// LoadGlobalSettings 从数据库加载全局设置
func (p *DBConfigProvider) LoadGlobalSettings(ctx context.Context) (*GlobalSettingsPayload, error) {
	// 初始化默认值
	payload := &GlobalSettingsPayload{
		Interval:       "1m",
		SlowLatency:    "5s",
		Timeout:        "10s",
		DegradedWeight: 0.7,
	}

	// 尝试读取旧格式（单个 "global" JSON 对象）
	setting, err := p.storage.GetGlobalSetting("global")
	if err != nil {
		return nil, fmt.Errorf("加载全局设置失败: %w", err)
	}
	if setting != nil {
		if err := json.Unmarshal([]byte(setting.Value), payload); err != nil {
			return nil, fmt.Errorf("解析全局设置失败: %w", err)
		}
		return payload, nil
	}

	// 新格式：分别读取每个设置
	if s, _ := p.storage.GetGlobalSetting("interval"); s != nil && s.Value != "" {
		payload.Interval = s.Value
	}
	if s, _ := p.storage.GetGlobalSetting("slow_latency"); s != nil && s.Value != "" {
		payload.SlowLatency = s.Value
	}
	if s, _ := p.storage.GetGlobalSetting("timeout"); s != nil && s.Value != "" {
		payload.Timeout = s.Value
	}
	if s, _ := p.storage.GetGlobalSetting("degraded_weight"); s != nil && s.Value != "" {
		if v, err := strconv.ParseFloat(s.Value, 64); err == nil {
			payload.DegradedWeight = v
		}
	}
	if s, _ := p.storage.GetGlobalSetting("enable_badges"); s != nil && s.Value == "true" {
		payload.EnableBadges = true
	}

	// SponsorPin 配置
	if s, _ := p.storage.GetGlobalSetting("sponsor_pin_enabled"); s != nil && s.Value == "true" {
		enabled := true
		payload.SponsorPin.Enabled = &enabled
	}
	if s, _ := p.storage.GetGlobalSetting("sponsor_pin_max_pinned"); s != nil && s.Value != "" {
		if v, err := strconv.Atoi(s.Value); err == nil {
			payload.SponsorPin.MaxPinned = v
		}
	}
	if s, _ := p.storage.GetGlobalSetting("sponsor_pin_min_uptime"); s != nil && s.Value != "" {
		if v, err := strconv.ParseFloat(s.Value, 64); err == nil {
			payload.SponsorPin.MinUptime = v
		}
	}
	if s, _ := p.storage.GetGlobalSetting("sponsor_pin_min_level"); s != nil && s.Value != "" {
		payload.SponsorPin.MinLevel = SponsorLevel(s.Value)
	}

	return payload, nil
}

// LoadProviderPolicies 从数据库加载 Provider 策略
func (p *DBConfigProvider) LoadProviderPolicies(ctx context.Context) (
	disabled []DisabledProviderConfig,
	hidden []HiddenProviderConfig,
	risks []RiskProviderConfig,
	err error,
) {
	policies, err := p.storage.ListProviderPolicies()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("加载 Provider 策略失败: %w", err)
	}

	for _, policy := range policies {
		switch policy.PolicyType {
		case "disabled":
			disabled = append(disabled, DisabledProviderConfig{
				Provider: policy.Provider,
				Reason:   policy.Reason,
			})
		case "hidden":
			hidden = append(hidden, HiddenProviderConfig{
				Provider: policy.Provider,
				Reason:   policy.Reason,
			})
		case "risk":
			var riskData []struct {
				Label         string `json:"label"`
				DiscussionURL string `json:"discussion_url"`
			}
			if policy.Risks != "" {
				json.Unmarshal([]byte(policy.Risks), &riskData)
			}
			var badges []RiskBadge
			for _, r := range riskData {
				badges = append(badges, RiskBadge{
					Label:         r.Label,
					DiscussionURL: r.DiscussionURL,
				})
			}
			risks = append(risks, RiskProviderConfig{
				Provider: policy.Provider,
				Risks:    badges,
			})
		}
	}

	return disabled, hidden, risks, nil
}

// LoadBadges 从数据库加载徽标定义和绑定
func (p *DBConfigProvider) LoadBadges(ctx context.Context) (
	defs map[string]BadgeDef,
	providers []BadgeProviderConfig,
	err error,
) {
	// 加载徽标定义
	badgeDefs, err := p.storage.ListBadgeDefinitions()
	if err != nil {
		return nil, nil, fmt.Errorf("加载徽标定义失败: %w", err)
	}

	defs = make(map[string]BadgeDef)
	for _, bd := range badgeDefs {
		defs[bd.ID] = BadgeDef{
			ID:      bd.ID,
			Kind:    BadgeKind(bd.Kind),
			Weight:  bd.Weight,
			Variant: BadgeVariantDefault, // 默认样式
		}
	}

	// 加载徽标绑定（provider 级别）
	bindings, err := p.storage.ListBadgeBindings(&BadgeBindingFilter{
		Scope: "provider",
	})
	if err != nil {
		return nil, nil, fmt.Errorf("加载徽标绑定失败: %w", err)
	}

	// 按 provider 分组
	providerBadges := make(map[string][]BadgeRef)
	for _, binding := range bindings {
		ref := BadgeRef{ID: binding.BadgeID}
		// 解析 tooltip override（简单提取，不再使用 map）
		if binding.TooltipOverride != "" {
			ref.Tooltip = binding.TooltipOverride
		}
		providerBadges[binding.Provider] = append(providerBadges[binding.Provider], ref)
	}

	for provider, badges := range providerBadges {
		providers = append(providers, BadgeProviderConfig{
			Provider: provider,
			Badges:   badges,
		})
	}

	return defs, providers, nil
}

// LoadBoards 从数据库加载 Board 配置
func (p *DBConfigProvider) LoadBoards(ctx context.Context) (*BoardsConfig, error) {
	// 检查是否启用 boards
	setting, err := p.storage.GetGlobalSetting("boards_enabled")
	if err != nil {
		return nil, fmt.Errorf("检查 boards 设置失败: %w", err)
	}

	enabled := false
	if setting != nil && setting.Value == "true" {
		enabled = true
	}

	return &BoardsConfig{Enabled: enabled}, nil
}

// LoadFullConfig 从数据库加载完整配置
// 返回的 AppConfig 包含所有功能配置（监测项 + 全局设置 + 策略 + 徽标 + Board）
func (p *DBConfigProvider) LoadFullConfig(ctx context.Context) (*AppConfig, error) {
	// 1. 加载全局设置
	globalSettings, err := p.LoadGlobalSettings(ctx)
	if err != nil {
		return nil, err
	}

	// 2. 加载监测项
	monitors, err := p.LoadMonitors(ctx)
	if err != nil {
		return nil, err
	}

	// 3. 加载 Provider 策略
	disabled, hidden, risks, err := p.LoadProviderPolicies(ctx)
	if err != nil {
		return nil, err
	}

	// 4. 加载徽标
	badgeDefs, badgeProviders, err := p.LoadBadges(ctx)
	if err != nil {
		return nil, err
	}

	// 5. 加载 Board 配置
	boards, err := p.LoadBoards(ctx)
	if err != nil {
		return nil, err
	}

	// 构建 AppConfig
	cfg := &AppConfig{
		// 时间配置
		Interval:             globalSettings.Interval,
		SlowLatency:          globalSettings.SlowLatency,
		SlowLatencyByService: globalSettings.SlowLatencyByService,
		Timeout:              globalSettings.Timeout,
		TimeoutByService:     globalSettings.TimeoutByService,

		// 重试配置
		Retry:                   globalSettings.Retry,
		RetryByService:          globalSettings.RetryByService,
		RetryBaseDelay:          globalSettings.RetryBaseDelay,
		RetryBaseDelayByService: globalSettings.RetryBaseDelayByService,
		RetryMaxDelay:           globalSettings.RetryMaxDelay,
		RetryMaxDelayByService:  globalSettings.RetryMaxDelayByService,
		RetryJitter:             globalSettings.RetryJitter,
		RetryJitterByService:    globalSettings.RetryJitterByService,

		// 运行时配置
		DegradedWeight:        globalSettings.DegradedWeight,
		MaxConcurrency:        globalSettings.MaxConcurrency,
		StaggerProbes:         globalSettings.StaggerProbes,
		EnableConcurrentQuery: globalSettings.EnableConcurrentQuery,
		ConcurrentQueryLimit:  globalSettings.ConcurrentQueryLimit,
		EnableBatchQuery:      globalSettings.EnableBatchQuery,
		EnableDBTimelineAgg:   globalSettings.EnableDBTimelineAgg,
		BatchQueryMaxKeys:     globalSettings.BatchQueryMaxKeys,
		CacheTTL:              globalSettings.CacheTTL,

		// 功能开关
		ExposeChannelDetails: globalSettings.ExposeChannelDetails,
		EnableBadges:         globalSettings.EnableBadges,
		SponsorPin:           globalSettings.SponsorPin,
		Boards:               *boards,

		// 策略
		DisabledProviders: disabled,
		HiddenProviders:   hidden,
		RiskProviders:     risks,

		// 徽标
		BadgeDefs:      badgeDefs,
		BadgeProviders: badgeProviders,

		// 监测项
		Monitors: monitors,
	}

	return cfg, nil
}

// boolPtr 返回 bool 指针
func boolPtr(b bool) *bool {
	return &b
}
