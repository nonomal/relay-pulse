package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// ApplyEnvOverrides 应用环境变量覆盖
// API Key 格式：MONITOR_<PROVIDER>_<SERVICE>_<CHANNEL>_API_KEY（优先）或 MONITOR_<PROVIDER>_<SERVICE>_API_KEY（向后兼容）
// 存储配置格式：MONITOR_STORAGE_TYPE, MONITOR_POSTGRES_HOST 等
func (c *AppConfig) ApplyEnvOverrides() {
	// PublicBaseURL 环境变量覆盖
	if envBaseURL := os.Getenv("MONITOR_PUBLIC_BASE_URL"); envBaseURL != "" {
		c.PublicBaseURL = envBaseURL
	}

	// 存储配置环境变量覆盖
	if envType := os.Getenv("MONITOR_STORAGE_TYPE"); envType != "" {
		c.Storage.Type = envType
	}

	// PostgreSQL 配置环境变量覆盖
	if envHost := os.Getenv("MONITOR_POSTGRES_HOST"); envHost != "" {
		c.Storage.Postgres.Host = envHost
	}
	if envPort := os.Getenv("MONITOR_POSTGRES_PORT"); envPort != "" {
		if port, err := fmt.Sscanf(envPort, "%d", &c.Storage.Postgres.Port); err == nil && port == 1 {
			// Port parsed successfully
		}
	}
	if envUser := os.Getenv("MONITOR_POSTGRES_USER"); envUser != "" {
		c.Storage.Postgres.User = envUser
	}
	if envPass := os.Getenv("MONITOR_POSTGRES_PASSWORD"); envPass != "" {
		c.Storage.Postgres.Password = envPass
	}
	if envDB := os.Getenv("MONITOR_POSTGRES_DATABASE"); envDB != "" {
		c.Storage.Postgres.Database = envDB
	}
	if envSSL := os.Getenv("MONITOR_POSTGRES_SSLMODE"); envSSL != "" {
		c.Storage.Postgres.SSLMode = envSSL
	}

	// SQLite 配置环境变量覆盖
	if envPath := os.Getenv("MONITOR_SQLITE_PATH"); envPath != "" {
		c.Storage.SQLite.Path = envPath
	}

	// Events API Token 环境变量覆盖
	if envToken := os.Getenv("EVENTS_API_TOKEN"); envToken != "" {
		c.Events.APIToken = envToken
	}

	// API Key 覆盖
	for i := range c.Monitors {
		m := &c.Monitors[i]

		// 优先使用自定义环境变量名（解决channel名称冲突）
		if m.EnvVarName != "" {
			if envVal := os.Getenv(m.EnvVarName); envVal != "" {
				m.APIKey = envVal
				continue
			}
		}

		// 优先查找包含 channel 的环境变量（新格式）
		envKeyWithChannel := fmt.Sprintf("MONITOR_%s_%s_%s_API_KEY",
			strings.ToUpper(strings.ReplaceAll(m.Provider, "-", "_")),
			strings.ToUpper(strings.ReplaceAll(m.Service, "-", "_")),
			strings.ToUpper(strings.ReplaceAll(m.Channel, "-", "_")))

		if envVal := os.Getenv(envKeyWithChannel); envVal != "" {
			m.APIKey = envVal
			continue
		}

		// 向后兼容：查找不带 channel 的环境变量（旧格式）
		envKey := fmt.Sprintf("MONITOR_%s_%s_API_KEY",
			strings.ToUpper(strings.ReplaceAll(m.Provider, "-", "_")),
			strings.ToUpper(strings.ReplaceAll(m.Service, "-", "_")))

		if envVal := os.Getenv(envKey); envVal != "" {
			m.APIKey = envVal
		}
	}
}

// ResolveBodyIncludes 允许 body 字段引用 data/ 目录下的 JSON 文件
func (c *AppConfig) ResolveBodyIncludes(configDir string) error {
	for i := range c.Monitors {
		if err := c.Monitors[i].resolveBodyInclude(configDir); err != nil {
			return err
		}
	}
	return nil
}

// Clone 深拷贝配置（用于热更新回滚）
func (c *AppConfig) Clone() *AppConfig {
	// 深拷贝指针字段
	var staggerPtr *bool
	if c.StaggerProbes != nil {
		value := *c.StaggerProbes
		staggerPtr = &value
	}

	// 深拷贝 SponsorPin.Enabled 指针
	var sponsorPinEnabledPtr *bool
	if c.SponsorPin.Enabled != nil {
		value := *c.SponsorPin.Enabled
		sponsorPinEnabledPtr = &value
	}

	// 深拷贝 ExposeChannelDetails 指针
	var exposeChannelDetailsPtr *bool
	if c.ExposeChannelDetails != nil {
		value := *c.ExposeChannelDetails
		exposeChannelDetailsPtr = &value
	}

	clone := &AppConfig{
		Interval:                     c.Interval,
		IntervalDuration:             c.IntervalDuration,
		SlowLatency:                  c.SlowLatency,
		SlowLatencyDuration:          c.SlowLatencyDuration,
		SlowLatencyByService:         make(map[string]string, len(c.SlowLatencyByService)),
		SlowLatencyByServiceDuration: make(map[string]time.Duration, len(c.SlowLatencyByServiceDuration)),
		Timeout:                      c.Timeout,
		TimeoutDuration:              c.TimeoutDuration,
		TimeoutByService:             make(map[string]string, len(c.TimeoutByService)),
		TimeoutByServiceDuration:     make(map[string]time.Duration, len(c.TimeoutByServiceDuration)),
		// 重试配置
		Retry:                           cloneIntPtr(c.Retry),
		RetryCount:                      c.RetryCount,
		RetryByService:                  make(map[string]int, len(c.RetryByService)),
		RetryByServiceCount:             make(map[string]int, len(c.RetryByServiceCount)),
		RetryBaseDelay:                  c.RetryBaseDelay,
		RetryBaseDelayDuration:          c.RetryBaseDelayDuration,
		RetryBaseDelayByService:         make(map[string]string, len(c.RetryBaseDelayByService)),
		RetryBaseDelayByServiceDuration: make(map[string]time.Duration, len(c.RetryBaseDelayByServiceDuration)),
		RetryMaxDelay:                   c.RetryMaxDelay,
		RetryMaxDelayDuration:           c.RetryMaxDelayDuration,
		RetryMaxDelayByService:          make(map[string]string, len(c.RetryMaxDelayByService)),
		RetryMaxDelayByServiceDuration:  make(map[string]time.Duration, len(c.RetryMaxDelayByServiceDuration)),
		RetryJitter:                     cloneFloat64Ptr(c.RetryJitter),
		RetryJitterValue:                c.RetryJitterValue,
		RetryJitterByService:            make(map[string]float64, len(c.RetryJitterByService)),
		RetryJitterByServiceValue:       make(map[string]float64, len(c.RetryJitterByServiceValue)),
		DegradedWeight:                  c.DegradedWeight,
		MaxConcurrency:                  c.MaxConcurrency,
		StaggerProbes:                   staggerPtr,
		EnableConcurrentQuery:           c.EnableConcurrentQuery,
		ConcurrentQueryLimit:            c.ConcurrentQueryLimit,
		EnableBatchQuery:                c.EnableBatchQuery,
		EnableDBTimelineAgg:             c.EnableDBTimelineAgg,
		BatchQueryMaxKeys:               c.BatchQueryMaxKeys,
		CacheTTL:                        c.CacheTTL, // CacheTTL 是值类型，直接复制
		Storage:                         c.Storage,
		PublicBaseURL:                   c.PublicBaseURL,
		DisabledProviders:               make([]DisabledProviderConfig, len(c.DisabledProviders)),
		HiddenProviders:                 make([]HiddenProviderConfig, len(c.HiddenProviders)),
		RiskProviders:                   make([]RiskProviderConfig, len(c.RiskProviders)),
		Boards:                          c.Boards, // Boards 是值类型，直接复制
		ExposeChannelDetails:            exposeChannelDetailsPtr,
		ChannelDetailsProviders:         make([]ChannelDetailsProviderConfig, len(c.ChannelDetailsProviders)),
		EnableBadges:                    c.EnableBadges,
		BadgeDefs:                       make(map[string]BadgeDef, len(c.BadgeDefs)),
		BadgeProviders:                  make([]BadgeProviderConfig, len(c.BadgeProviders)),
		SponsorPin: SponsorPinConfig{
			Enabled:      sponsorPinEnabledPtr,
			MaxPinned:    c.SponsorPin.MaxPinned,
			ServiceCount: c.SponsorPin.ServiceCount,
			MinUptime:    c.SponsorPin.MinUptime,
			MinLevel:     c.SponsorPin.MinLevel,
		},
		SelfTest:      c.SelfTest,      // SelfTest 是值类型，直接复制
		Events:        c.Events,        // Events 是值类型，直接复制
		Announcements: c.Announcements, // Announcements 是值类型，直接复制
		GitHub:        c.GitHub,        // GitHub 是值类型，直接复制
		Monitors:      make([]ServiceConfig, len(c.Monitors)),
	}

	// 复制 slice
	copy(clone.DisabledProviders, c.DisabledProviders)
	copy(clone.HiddenProviders, c.HiddenProviders)
	copy(clone.RiskProviders, c.RiskProviders)
	copy(clone.ChannelDetailsProviders, c.ChannelDetailsProviders)
	copy(clone.BadgeProviders, c.BadgeProviders)
	copy(clone.Monitors, c.Monitors)

	// 复制 map
	for k, v := range c.SlowLatencyByService {
		clone.SlowLatencyByService[k] = v
	}
	for k, v := range c.SlowLatencyByServiceDuration {
		clone.SlowLatencyByServiceDuration[k] = v
	}
	for k, v := range c.TimeoutByService {
		clone.TimeoutByService[k] = v
	}
	for k, v := range c.TimeoutByServiceDuration {
		clone.TimeoutByServiceDuration[k] = v
	}
	for k, v := range c.RetryByService {
		clone.RetryByService[k] = v
	}
	for k, v := range c.RetryByServiceCount {
		clone.RetryByServiceCount[k] = v
	}
	for k, v := range c.RetryBaseDelayByService {
		clone.RetryBaseDelayByService[k] = v
	}
	for k, v := range c.RetryBaseDelayByServiceDuration {
		clone.RetryBaseDelayByServiceDuration[k] = v
	}
	for k, v := range c.RetryMaxDelayByService {
		clone.RetryMaxDelayByService[k] = v
	}
	for k, v := range c.RetryMaxDelayByServiceDuration {
		clone.RetryMaxDelayByServiceDuration[k] = v
	}
	for k, v := range c.RetryJitterByService {
		clone.RetryJitterByService[k] = v
	}
	for k, v := range c.RetryJitterByServiceValue {
		clone.RetryJitterByServiceValue[k] = v
	}
	for id, bd := range c.BadgeDefs {
		clone.BadgeDefs[id] = bd
	}

	// 深拷贝 risk_providers 中的 risks slice
	for i := range clone.RiskProviders {
		if len(c.RiskProviders[i].Risks) > 0 {
			clone.RiskProviders[i].Risks = make([]RiskBadge, len(c.RiskProviders[i].Risks))
			copy(clone.RiskProviders[i].Risks, c.RiskProviders[i].Risks)
		}
	}

	// 深拷贝 badge_providers 中的 badges slice
	for i := range clone.BadgeProviders {
		if len(c.BadgeProviders[i].Badges) > 0 {
			clone.BadgeProviders[i].Badges = make([]BadgeRef, len(c.BadgeProviders[i].Badges))
			copy(clone.BadgeProviders[i].Badges, c.BadgeProviders[i].Badges)
		}
	}

	// 深拷贝 monitors 中的 slice/map/指针 字段
	for i := range clone.Monitors {
		// headers map
		if c.Monitors[i].Headers != nil {
			clone.Monitors[i].Headers = make(map[string]string, len(c.Monitors[i].Headers))
			for k, v := range c.Monitors[i].Headers {
				clone.Monitors[i].Headers[k] = v
			}
		}
		// risks slice
		if len(c.Monitors[i].Risks) > 0 {
			clone.Monitors[i].Risks = make([]RiskBadge, len(c.Monitors[i].Risks))
			copy(clone.Monitors[i].Risks, c.Monitors[i].Risks)
		}
		// badges refs slice
		if len(c.Monitors[i].Badges) > 0 {
			clone.Monitors[i].Badges = make([]BadgeRef, len(c.Monitors[i].Badges))
			copy(clone.Monitors[i].Badges, c.Monitors[i].Badges)
		}
		// resolved badges slice
		if len(c.Monitors[i].ResolvedBadges) > 0 {
			clone.Monitors[i].ResolvedBadges = make([]ResolvedBadge, len(c.Monitors[i].ResolvedBadges))
			copy(clone.Monitors[i].ResolvedBadges, c.Monitors[i].ResolvedBadges)
		}
		// 指针字段深拷贝
		clone.Monitors[i].PriceMin = cloneFloat64Ptr(c.Monitors[i].PriceMin)
		clone.Monitors[i].PriceMax = cloneFloat64Ptr(c.Monitors[i].PriceMax)
		clone.Monitors[i].Retry = cloneIntPtr(c.Monitors[i].Retry)
		clone.Monitors[i].RetryJitter = cloneFloat64Ptr(c.Monitors[i].RetryJitter)
	}

	return clone
}

// cloneIntPtr 深拷贝 *int 指针
func cloneIntPtr(p *int) *int {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

// cloneFloat64Ptr 深拷贝 *float64 指针
func cloneFloat64Ptr(p *float64) *float64 {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

// ShouldStaggerProbes 返回当前配置是否启用错峰探测
func (c *AppConfig) ShouldStaggerProbes() bool {
	if c == nil {
		return false
	}
	if c.StaggerProbes == nil {
		return true // 默认开启
	}
	return *c.StaggerProbes
}

// ShouldExposeChannelDetails 判断指定 provider 是否应该暴露通道技术细节
// 优先查找 provider 级覆盖配置，否则回退到全局配置
func (c *AppConfig) ShouldExposeChannelDetails(provider string) bool {
	if c == nil {
		return true // 默认暴露
	}

	// 先查 provider 级覆盖
	normalizedProvider := strings.ToLower(strings.TrimSpace(provider))
	for _, p := range c.ChannelDetailsProviders {
		if strings.ToLower(strings.TrimSpace(p.Provider)) == normalizedProvider {
			return p.Expose
		}
	}

	// 回退全局配置
	if c.ExposeChannelDetails == nil {
		return true // 默认暴露
	}
	return *c.ExposeChannelDetails
}
