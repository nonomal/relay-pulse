package config

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// V1Storage v1.0 监测配置存储接口
// 定义在 config 包中以避免与 storage 包的循环依赖
type V1Storage interface {
	// 监测项相关
	ListEnabledMonitorsWithTemplates(ctx context.Context) ([]*V1MonitorRecord, error)

	// API Key 相关（复用现有 monitor_secrets 表）
	GetMonitorSecretByV1ID(ctx context.Context, monitorID int) (*MonitorSecretRecord, error)

	// 全局设置相关（复用现有接口）
	GetGlobalSetting(key string) (*GlobalSettingRecord, error)
}

// V1MonitorRecord 从 monitors 表加载的监测项记录（含模板数据）
type V1MonitorRecord struct {
	// 监测项基础信息
	ID           int
	Provider     string
	ProviderName string
	Service      string
	ServiceName  string
	Channel      string
	ChannelName  string
	Model        string

	// 模板关联
	TemplateID *int

	// 监测项配置（可覆盖模板）
	URL             string
	Method          *string // NULL = 继承模板
	Headers         json.RawMessage
	Body            json.RawMessage
	SuccessContains *string // NULL = 继承模板

	// 时间配置覆盖
	IntervalOverride    *string
	TimeoutOverride     *string
	SlowLatencyOverride *string

	// 状态
	Enabled bool
	BoardID *int

	// 元数据
	Metadata    json.RawMessage
	OwnerUserID *string
	VendorType  string
	WebsiteURL  string

	// 时间戳
	CreatedAt int64
	UpdatedAt int64

	// 关联的模板数据（查询时 JOIN 填充）
	Template *V1TemplateRecord
}

// V1TemplateRecord 模板记录
type V1TemplateRecord struct {
	ID                 int
	ServiceID          string
	Name               string
	Slug               string
	RequestMethod      string
	BaseRequestHeaders json.RawMessage
	BaseRequestBody    json.RawMessage
	BaseResponseChecks json.RawMessage
	TimeoutMs          int
	SlowLatencyMs      int
}

// V1MonitorMetadata 监测项元数据（存储在 metadata JSONB 字段）
type V1MonitorMetadata struct {
	Category     string   `json:"category,omitempty"`
	Sponsor      string   `json:"sponsor,omitempty"`
	SponsorURL   string   `json:"sponsor_url,omitempty"`
	SponsorLevel string   `json:"sponsor_level,omitempty"`
	ProviderURL  string   `json:"provider_url,omitempty"`
	ProviderSlug string   `json:"provider_slug,omitempty"`
	PriceMin     *float64 `json:"price_min,omitempty"`
	PriceMax     *float64 `json:"price_max,omitempty"`
	ListedSince  string   `json:"listed_since,omitempty"`
	Hidden       bool     `json:"hidden,omitempty"`
	HiddenReason string   `json:"hidden_reason,omitempty"`
	ColdReason   string   `json:"cold_reason,omitempty"`
	Badges       []string `json:"badges,omitempty"`
}

// V1ConfigProvider 从 v1.0 monitors 表加载配置的提供者
type V1ConfigProvider struct {
	storage V1Storage
}

// NewV1ConfigProvider 创建 v1.0 配置提供者
func NewV1ConfigProvider(s V1Storage) *V1ConfigProvider {
	return &V1ConfigProvider{storage: s}
}

// LoadMonitors 从 v1 monitors 表加载监测项配置并转换为 ServiceConfig 切片
func (p *V1ConfigProvider) LoadMonitors(ctx context.Context) ([]ServiceConfig, error) {
	// 查询所有启用的监测项（含模板数据）
	monitors, err := p.storage.ListEnabledMonitorsWithTemplates(ctx)
	if err != nil {
		return nil, fmt.Errorf("从 v1 monitors 表加载监测项失败: %w", err)
	}

	result := make([]ServiceConfig, 0, len(monitors))
	for _, m := range monitors {
		sc, err := p.convertMonitor(ctx, m)
		if err != nil {
			// 记录错误但继续处理其他配置
			continue
		}
		result = append(result, *sc)
	}

	return result, nil
}

// convertMonitor 将 V1MonitorRecord 转换为 ServiceConfig
func (p *V1ConfigProvider) convertMonitor(ctx context.Context, m *V1MonitorRecord) (*ServiceConfig, error) {
	// 解析元数据
	var metadata V1MonitorMetadata
	if len(m.Metadata) > 0 {
		_ = json.Unmarshal(m.Metadata, &metadata)
	}

	// 构建基础 ServiceConfig
	sc := &ServiceConfig{
		Provider:     m.Provider,
		ProviderName: m.ProviderName,
		ProviderSlug: metadata.ProviderSlug,
		ProviderURL:  metadata.ProviderURL,
		Service:      m.Service,
		ServiceName:  m.ServiceName,
		Channel:      m.Channel,
		ChannelName:  m.ChannelName,
		Model:        m.Model,
		URL:          m.URL,

		// 元数据字段
		Category:     metadata.Category,
		Sponsor:      metadata.Sponsor,
		SponsorURL:   metadata.SponsorURL,
		SponsorLevel: SponsorLevel(metadata.SponsorLevel),
		PriceMin:     metadata.PriceMin,
		PriceMax:     metadata.PriceMax,
		ListedSince:  metadata.ListedSince,
		Hidden:       metadata.Hidden,
		HiddenReason: metadata.HiddenReason,
		ColdReason:   metadata.ColdReason,
	}

	// 处理徽标引用
	for _, badgeID := range metadata.Badges {
		sc.Badges = append(sc.Badges, BadgeRef{ID: badgeID})
	}

	// 处理 Board
	if m.BoardID != nil {
		// BoardID 转换为 board 字符串
		// 0 = hot, 1 = cold (简化处理)
		if *m.BoardID == 1 {
			sc.Board = "cold"
		}
	}

	// 合并模板配置
	if m.Template != nil {
		p.mergeTemplateConfig(sc, m, m.Template)
	} else {
		// 无模板时，直接使用监测项自身配置
		p.applyMonitorOnlyConfig(sc, m)
	}

	// 加载并解密 API Key
	apiKey, err := p.loadAPIKey(ctx, m.ID)
	if err != nil {
		// API Key 解密失败不应阻止加载
		sc.APIKey = ""
	} else {
		sc.APIKey = apiKey
	}

	return sc, nil
}

// mergeTemplateConfig 合并模板配置到 ServiceConfig
// 规则：monitor 覆盖 > template 基础配置
func (p *V1ConfigProvider) mergeTemplateConfig(sc *ServiceConfig, m *V1MonitorRecord, t *V1TemplateRecord) {
	// Method: 监测项覆盖 > 模板
	if m.Method != nil {
		sc.Method = *m.Method
	} else {
		sc.Method = t.RequestMethod
	}

	// Headers: 模板基础 + 监测项合并
	headers := make(map[string]string)
	if len(t.BaseRequestHeaders) > 0 {
		_ = json.Unmarshal(t.BaseRequestHeaders, &headers)
	}
	if len(m.Headers) > 0 {
		var monitorHeaders map[string]string
		if err := json.Unmarshal(m.Headers, &monitorHeaders); err == nil {
			for k, v := range monitorHeaders {
				headers[k] = v
			}
		}
	}
	sc.Headers = headers

	// Body: 监测项覆盖 > 模板
	if len(m.Body) > 0 {
		sc.Body = string(m.Body)
	} else if len(t.BaseRequestBody) > 0 {
		sc.Body = string(t.BaseRequestBody)
	}

	// SuccessContains: 监测项覆盖 > 模板（从 response_checks 提取）
	if m.SuccessContains != nil {
		sc.SuccessContains = *m.SuccessContains
	} else if len(t.BaseResponseChecks) > 0 {
		var checks struct {
			SuccessContains string `json:"success_contains"`
		}
		if err := json.Unmarshal(t.BaseResponseChecks, &checks); err == nil {
			sc.SuccessContains = checks.SuccessContains
		}
	}

	// Timeout: 监测项覆盖 > 模板
	if m.TimeoutOverride != nil {
		sc.Timeout = *m.TimeoutOverride
	} else if t.TimeoutMs > 0 {
		sc.Timeout = fmt.Sprintf("%dms", t.TimeoutMs)
	}

	// SlowLatency: 监测项覆盖 > 模板
	if m.SlowLatencyOverride != nil {
		sc.SlowLatency = *m.SlowLatencyOverride
	} else if t.SlowLatencyMs > 0 {
		sc.SlowLatency = fmt.Sprintf("%dms", t.SlowLatencyMs)
	}

	// Interval: 监测项覆盖
	if m.IntervalOverride != nil {
		sc.Interval = *m.IntervalOverride
	}
}

// applyMonitorOnlyConfig 应用仅监测项自身的配置（无模板时）
func (p *V1ConfigProvider) applyMonitorOnlyConfig(sc *ServiceConfig, m *V1MonitorRecord) {
	// Method
	if m.Method != nil {
		sc.Method = *m.Method
	} else {
		sc.Method = "POST" // 默认 POST
	}

	// Headers
	if len(m.Headers) > 0 {
		var headers map[string]string
		if err := json.Unmarshal(m.Headers, &headers); err == nil {
			sc.Headers = headers
		}
	}

	// Body
	if len(m.Body) > 0 {
		sc.Body = string(m.Body)
	}

	// SuccessContains
	if m.SuccessContains != nil {
		sc.SuccessContains = *m.SuccessContains
	}

	// Timeout
	if m.TimeoutOverride != nil {
		sc.Timeout = *m.TimeoutOverride
	}

	// SlowLatency
	if m.SlowLatencyOverride != nil {
		sc.SlowLatency = *m.SlowLatencyOverride
	}

	// Interval
	if m.IntervalOverride != nil {
		sc.Interval = *m.IntervalOverride
	}
}

// loadAPIKey 从 monitor_secrets 加载并解密 API Key
func (p *V1ConfigProvider) loadAPIKey(ctx context.Context, monitorID int) (string, error) {
	secret, err := p.storage.GetMonitorSecretByV1ID(ctx, monitorID)
	if err != nil {
		return "", fmt.Errorf("获取密钥失败 (v1_monitor_id=%d): %w", monitorID, err)
	}
	if secret == nil {
		// 没有设置 API Key
		return "", nil
	}

	// 解密 API Key（使用 v1 monitor ID）
	apiKey, err := DecryptAPIKey(
		secret.APIKeyCiphertext,
		secret.APIKeyNonce,
		int64(monitorID),
		secret.KeyVersion,
	)
	if err != nil {
		return "", fmt.Errorf("解密 API Key 失败 (v1_monitor_id=%d): %w", monitorID, err)
	}

	return apiKey, nil
}

// LoadGlobalSettings 从数据库加载全局设置（复用 DBConfigProvider 的逻辑）
func (p *V1ConfigProvider) LoadGlobalSettings(ctx context.Context) (*GlobalSettingsPayload, error) {
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

	return payload, nil
}

// LoadFullConfig 从 v1 表加载完整配置
func (p *V1ConfigProvider) LoadFullConfig(ctx context.Context) (*AppConfig, error) {
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

	// 3. 构建 AppConfig（v1 不再使用 Provider 策略，这些逻辑在 metadata 中处理）
	cfg := &AppConfig{
		// 时间配置
		Interval:             globalSettings.Interval,
		SlowLatency:          globalSettings.SlowLatency,
		SlowLatencyByService: globalSettings.SlowLatencyByService,
		Timeout:              globalSettings.Timeout,
		TimeoutByService:     globalSettings.TimeoutByService,

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
		Boards:               globalSettings.Boards,

		// 徽标（v1 中徽标定义存储在 badge_definitions 表）
		BadgeDefs: globalSettings.BadgeDefs,

		// 监测项
		Monitors: monitors,
	}

	// 解析时间配置
	if d, err := time.ParseDuration(cfg.Interval); err == nil {
		cfg.IntervalDuration = d
	} else {
		cfg.IntervalDuration = time.Minute
	}

	if d, err := time.ParseDuration(cfg.SlowLatency); err == nil {
		cfg.SlowLatencyDuration = d
	} else {
		cfg.SlowLatencyDuration = 5 * time.Second
	}

	if d, err := time.ParseDuration(cfg.Timeout); err == nil {
		cfg.TimeoutDuration = d
	} else {
		cfg.TimeoutDuration = 10 * time.Second
	}

	// 解析 by_service 配置
	cfg.SlowLatencyByServiceDuration = make(map[string]time.Duration)
	for k, v := range cfg.SlowLatencyByService {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.SlowLatencyByServiceDuration[k] = d
		}
	}

	cfg.TimeoutByServiceDuration = make(map[string]time.Duration)
	for k, v := range cfg.TimeoutByService {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.TimeoutByServiceDuration[k] = d
		}
	}

	return cfg, nil
}
