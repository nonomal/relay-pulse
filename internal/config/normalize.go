package config

import (
	"fmt"
	"strings"
	"time"

	"monitor/internal/logger"
)

// Normalize 规范化配置（填充默认值等）
func (c *AppConfig) Normalize() error {
	// 1. 全局时间配置（interval, slow_latency, timeout 及 by_service）
	if err := c.normalizeGlobalTimings(); err != nil {
		return err
	}

	// 2. 全局参数默认值
	if err := c.normalizeGlobalDefaults(); err != nil {
		return err
	}

	// 3. 功能模块配置（sponsor_pin, selftest, events, github, announcements）
	if err := c.normalizeFeatureConfigs(); err != nil {
		return err
	}

	// 4. 存储配置
	if err := c.normalizeStorageConfig(); err != nil {
		return err
	}

	// 5. 构建 Provider/Badge 映射索引
	ctx := newNormalizeContext()
	if err := c.buildNormalizeIndexes(ctx); err != nil {
		return err
	}

	// 6. 规范化每个监测项（不含徽标解析，徽标需在继承后处理）
	if err := c.normalizeMonitorsPreInheritance(ctx); err != nil {
		return err
	}

	// 7. 父子继承（必须在 per-monitor 规范化之后，因为继承依赖已规范化的路径/键）
	if err := c.applyParentInheritance(); err != nil {
		return err
	}

	// 8. 继承后处理：徽标解析、board/cold_reason 清理
	// 必须在继承之后，确保子通道继承的 Badges 能正确解析为 ResolvedBadges
	if err := c.normalizeMonitorsPostInheritance(ctx); err != nil {
		return err
	}

	return nil
}

// normalizeContext 承载 Normalize() 过程中的中间数据
// 主要用于传递全局构建的映射给 per-monitor 处理
type normalizeContext struct {
	// Provider 可见性映射
	disabledProviderMap map[string]string // provider -> reason
	hiddenProviderMap   map[string]string // provider -> reason
	riskProviderMap     map[string][]RiskBadge

	// Badge 体系索引
	badgeDefMap      map[string]BadgeDef   // id -> def（含内置默认）
	badgeProviderMap map[string][]BadgeRef // provider -> badges
}

// newNormalizeContext 创建并初始化 normalizeContext
func newNormalizeContext() *normalizeContext {
	return &normalizeContext{
		disabledProviderMap: make(map[string]string),
		hiddenProviderMap:   make(map[string]string),
		riskProviderMap:     make(map[string][]RiskBadge),
		badgeDefMap:         make(map[string]BadgeDef),
		badgeProviderMap:    make(map[string][]BadgeRef),
	}
}

// normalizeGlobalTimings 规范化全局时间配置
// 包括：interval, slow_latency, timeout 及其 by_service 版本
func (c *AppConfig) normalizeGlobalTimings() error {
	// 巡检间隔
	if c.Interval == "" {
		c.IntervalDuration = time.Minute
	} else {
		d, err := time.ParseDuration(c.Interval)
		if err != nil {
			return fmt.Errorf("解析 interval 失败: %w", err)
		}
		if d <= 0 {
			return fmt.Errorf("interval 必须大于 0")
		}
		c.IntervalDuration = d
	}

	// 慢请求阈值
	if c.SlowLatency == "" {
		c.SlowLatencyDuration = 5 * time.Second
	} else {
		d, err := time.ParseDuration(c.SlowLatency)
		if err != nil {
			return fmt.Errorf("解析 slow_latency 失败: %w", err)
		}
		if d <= 0 {
			return fmt.Errorf("slow_latency 必须大于 0")
		}
		c.SlowLatencyDuration = d
	}

	// 按服务类型覆盖的慢请求阈值
	if len(c.SlowLatencyByService) > 0 {
		c.SlowLatencyByServiceDuration = make(map[string]time.Duration, len(c.SlowLatencyByService))
		for service, raw := range c.SlowLatencyByService {
			normalizedService := strings.ToLower(strings.TrimSpace(service))
			if normalizedService == "" {
				return fmt.Errorf("slow_latency_by_service: service 名称不能为空")
			}
			if _, exists := c.SlowLatencyByServiceDuration[normalizedService]; exists {
				return fmt.Errorf("slow_latency_by_service: service '%s' 重复配置（大小写不敏感）", normalizedService)
			}

			trimmed := strings.TrimSpace(raw)
			if trimmed == "" {
				return fmt.Errorf("slow_latency_by_service[%s]: 值不能为空", service)
			}
			d, err := time.ParseDuration(trimmed)
			if err != nil {
				return fmt.Errorf("解析 slow_latency_by_service[%s] 失败: %w", service, err)
			}
			if d <= 0 {
				return fmt.Errorf("slow_latency_by_service[%s] 必须大于 0", service)
			}
			c.SlowLatencyByServiceDuration[normalizedService] = d
		}
	} else {
		// 热更新场景：清除旧的覆盖配置
		c.SlowLatencyByServiceDuration = nil
	}

	// 请求超时时间（默认 10 秒）
	if c.Timeout == "" {
		c.TimeoutDuration = 10 * time.Second
	} else {
		d, err := time.ParseDuration(c.Timeout)
		if err != nil {
			return fmt.Errorf("解析 timeout 失败: %w", err)
		}
		if d <= 0 {
			return fmt.Errorf("timeout 必须大于 0")
		}
		c.TimeoutDuration = d
	}

	// 按服务类型覆盖的超时时间
	if len(c.TimeoutByService) > 0 {
		c.TimeoutByServiceDuration = make(map[string]time.Duration, len(c.TimeoutByService))
		for service, raw := range c.TimeoutByService {
			normalizedService := strings.ToLower(strings.TrimSpace(service))
			if normalizedService == "" {
				return fmt.Errorf("timeout_by_service: service 名称不能为空")
			}
			if _, exists := c.TimeoutByServiceDuration[normalizedService]; exists {
				return fmt.Errorf("timeout_by_service: service '%s' 重复配置（大小写不敏感）", normalizedService)
			}

			trimmed := strings.TrimSpace(raw)
			if trimmed == "" {
				return fmt.Errorf("timeout_by_service[%s]: 值不能为空", service)
			}
			d, err := time.ParseDuration(trimmed)
			if err != nil {
				return fmt.Errorf("解析 timeout_by_service[%s] 失败: %w", service, err)
			}
			if d <= 0 {
				return fmt.Errorf("timeout_by_service[%s] 必须大于 0", service)
			}
			c.TimeoutByServiceDuration[normalizedService] = d
		}
	} else {
		// 热更新场景：清除旧的覆盖配置
		c.TimeoutByServiceDuration = nil
	}

	// ===== 重试配置 =====
	// 重试次数（默认 0，不重试）
	if c.Retry == nil {
		c.RetryCount = 0
	} else {
		c.RetryCount = *c.Retry
	}
	if c.RetryCount < 0 {
		return fmt.Errorf("retry 必须 >= 0")
	}

	// 按服务类型覆盖的重试次数
	if len(c.RetryByService) > 0 {
		c.RetryByServiceCount = make(map[string]int, len(c.RetryByService))
		for service, v := range c.RetryByService {
			key := strings.ToLower(strings.TrimSpace(service))
			if key == "" {
				return fmt.Errorf("retry_by_service: service 名称不能为空")
			}
			if _, exists := c.RetryByServiceCount[key]; exists {
				return fmt.Errorf("retry_by_service: service '%s' 重复配置（大小写不敏感）", key)
			}
			if v < 0 {
				return fmt.Errorf("retry_by_service[%s] 必须 >= 0", service)
			}
			c.RetryByServiceCount[key] = v
		}
	} else {
		c.RetryByServiceCount = nil
	}

	// 退避基准间隔（默认 200ms）
	if strings.TrimSpace(c.RetryBaseDelay) == "" {
		c.RetryBaseDelayDuration = 200 * time.Millisecond
	} else {
		d, err := time.ParseDuration(strings.TrimSpace(c.RetryBaseDelay))
		if err != nil {
			return fmt.Errorf("解析 retry_base_delay 失败: %w", err)
		}
		if d <= 0 {
			return fmt.Errorf("retry_base_delay 必须 > 0")
		}
		c.RetryBaseDelayDuration = d
	}

	// 按服务类型覆盖的退避基准间隔
	if len(c.RetryBaseDelayByService) > 0 {
		c.RetryBaseDelayByServiceDuration = make(map[string]time.Duration, len(c.RetryBaseDelayByService))
		for service, raw := range c.RetryBaseDelayByService {
			key := strings.ToLower(strings.TrimSpace(service))
			if key == "" {
				return fmt.Errorf("retry_base_delay_by_service: service 名称不能为空")
			}
			if _, exists := c.RetryBaseDelayByServiceDuration[key]; exists {
				return fmt.Errorf("retry_base_delay_by_service: service '%s' 重复配置（大小写不敏感）", key)
			}
			trimmed := strings.TrimSpace(raw)
			if trimmed == "" {
				return fmt.Errorf("retry_base_delay_by_service[%s]: 值不能为空", service)
			}
			d, err := time.ParseDuration(trimmed)
			if err != nil {
				return fmt.Errorf("解析 retry_base_delay_by_service[%s] 失败: %w", service, err)
			}
			if d <= 0 {
				return fmt.Errorf("retry_base_delay_by_service[%s] 必须 > 0", service)
			}
			c.RetryBaseDelayByServiceDuration[key] = d
		}
	} else {
		c.RetryBaseDelayByServiceDuration = nil
	}

	// 退避最大间隔（默认 2s）
	if strings.TrimSpace(c.RetryMaxDelay) == "" {
		c.RetryMaxDelayDuration = 2 * time.Second
	} else {
		d, err := time.ParseDuration(strings.TrimSpace(c.RetryMaxDelay))
		if err != nil {
			return fmt.Errorf("解析 retry_max_delay 失败: %w", err)
		}
		if d <= 0 {
			return fmt.Errorf("retry_max_delay 必须 > 0")
		}
		c.RetryMaxDelayDuration = d
	}

	// 按服务类型覆盖的退避最大间隔
	if len(c.RetryMaxDelayByService) > 0 {
		c.RetryMaxDelayByServiceDuration = make(map[string]time.Duration, len(c.RetryMaxDelayByService))
		for service, raw := range c.RetryMaxDelayByService {
			key := strings.ToLower(strings.TrimSpace(service))
			if key == "" {
				return fmt.Errorf("retry_max_delay_by_service: service 名称不能为空")
			}
			if _, exists := c.RetryMaxDelayByServiceDuration[key]; exists {
				return fmt.Errorf("retry_max_delay_by_service: service '%s' 重复配置（大小写不敏感）", key)
			}
			trimmed := strings.TrimSpace(raw)
			if trimmed == "" {
				return fmt.Errorf("retry_max_delay_by_service[%s]: 值不能为空", service)
			}
			d, err := time.ParseDuration(trimmed)
			if err != nil {
				return fmt.Errorf("解析 retry_max_delay_by_service[%s] 失败: %w", service, err)
			}
			if d <= 0 {
				return fmt.Errorf("retry_max_delay_by_service[%s] 必须 > 0", service)
			}
			c.RetryMaxDelayByServiceDuration[key] = d
		}
	} else {
		c.RetryMaxDelayByServiceDuration = nil
	}

	// 全局 max >= base 校验
	if c.RetryMaxDelayDuration < c.RetryBaseDelayDuration {
		return fmt.Errorf("retry_max_delay 必须 >= retry_base_delay")
	}

	// 抖动比例（默认 0.2）
	if c.RetryJitter == nil {
		c.RetryJitterValue = 0.2
	} else {
		c.RetryJitterValue = *c.RetryJitter
	}
	if c.RetryJitterValue < 0 || c.RetryJitterValue > 1 {
		return fmt.Errorf("retry_jitter 必须在 0 到 1 之间，当前值: %.2f", c.RetryJitterValue)
	}

	// 按服务类型覆盖的抖动比例
	if len(c.RetryJitterByService) > 0 {
		c.RetryJitterByServiceValue = make(map[string]float64, len(c.RetryJitterByService))
		for service, v := range c.RetryJitterByService {
			key := strings.ToLower(strings.TrimSpace(service))
			if key == "" {
				return fmt.Errorf("retry_jitter_by_service: service 名称不能为空")
			}
			if _, exists := c.RetryJitterByServiceValue[key]; exists {
				return fmt.Errorf("retry_jitter_by_service: service '%s' 重复配置（大小写不敏感）", key)
			}
			if v < 0 || v > 1 {
				return fmt.Errorf("retry_jitter_by_service[%s] 必须在 0 到 1 之间", service)
			}
			c.RetryJitterByServiceValue[key] = v
		}
	} else {
		c.RetryJitterByServiceValue = nil
	}

	return nil
}

// normalizeGlobalDefaults 规范化全局参数默认值
func (c *AppConfig) normalizeGlobalDefaults() error {
	// 黄色状态权重（默认 0.7，允许 0.01-1.0）
	// 注意：0 被视为未配置，将使用默认值 0.7
	// 如果需要极低权重，请使用 0.01 或更小的正数
	if c.DegradedWeight == 0 {
		c.DegradedWeight = 0.7 // 未配置时使用默认值
	}
	if c.DegradedWeight < 0 || c.DegradedWeight > 1 {
		return fmt.Errorf("degraded_weight 必须在 0 到 1 之间（0 表示使用默认值 0.7），当前值: %.2f", c.DegradedWeight)
	}

	// 公开访问的基础 URL（默认 https://relaypulse.top）
	if c.PublicBaseURL == "" {
		c.PublicBaseURL = "https://relaypulse.top"
	}

	// 规范化 baseURL：去除尾随斜杠、验证协议
	c.PublicBaseURL = strings.TrimRight(c.PublicBaseURL, "/")
	if err := validateBaseURL(c.PublicBaseURL); err != nil {
		return fmt.Errorf("public_base_url 无效: %w", err)
	}

	// 最大并发数（默认 10）
	// - 未配置或 0：使用默认值 10
	// - -1：无限制（自动扩容到监测项数量）
	// - >0：作为硬上限，超过时排队执行
	if c.MaxConcurrency == 0 {
		c.MaxConcurrency = 10
	}
	if c.MaxConcurrency < -1 {
		return fmt.Errorf("max_concurrency 无效值 %d，有效值：-1(无限制)、0(默认10)、>0(硬上限)", c.MaxConcurrency)
	}

	// 探测错峰（默认开启）
	if c.StaggerProbes == nil {
		defaultValue := true
		c.StaggerProbes = &defaultValue
	}

	// 并发查询限制（默认 10）
	if c.ConcurrentQueryLimit == 0 {
		c.ConcurrentQueryLimit = 10
	}
	if c.ConcurrentQueryLimit < 1 {
		return fmt.Errorf("concurrent_query_limit 必须 >= 1，当前值: %d", c.ConcurrentQueryLimit)
	}

	// 批量查询最大 key 数（默认 300）
	if c.BatchQueryMaxKeys == 0 {
		c.BatchQueryMaxKeys = 300
	}
	if c.BatchQueryMaxKeys < 1 {
		return fmt.Errorf("batch_query_max_keys 必须 >= 1，当前值: %d", c.BatchQueryMaxKeys)
	}

	// 缓存 TTL 配置
	if err := c.CacheTTL.Normalize(); err != nil {
		return err
	}

	// 通道技术细节暴露配置（默认 true，保持向后兼容）
	if c.ExposeChannelDetails == nil {
		defaultValue := true
		c.ExposeChannelDetails = &defaultValue
	}

	return nil
}

// normalizeFeatureConfigs 规范化功能模块配置
// 包括：sponsor_pin, selftest, events, github, announcements
func (c *AppConfig) normalizeFeatureConfigs() error {
	// 赞助商置顶配置默认值
	if c.SponsorPin.MaxPinned == 0 {
		c.SponsorPin.MaxPinned = 3
	}
	if c.SponsorPin.ServiceCount == 0 {
		c.SponsorPin.ServiceCount = 3
	}
	if c.SponsorPin.MinUptime == 0 {
		c.SponsorPin.MinUptime = 95.0
	}
	if c.SponsorPin.MinLevel == "" {
		c.SponsorPin.MinLevel = SponsorLevelBasic
	}
	// 验证赞助商置顶配置
	if c.SponsorPin.MaxPinned < 0 {
		logger.Warn("config", "sponsor_pin.max_pinned 无效，已回退默认值", "value", c.SponsorPin.MaxPinned, "default", 3)
		c.SponsorPin.MaxPinned = 3
	}
	if c.SponsorPin.ServiceCount < 1 {
		logger.Warn("config", "sponsor_pin.service_count 无效，已回退默认值", "value", c.SponsorPin.ServiceCount, "default", 3)
		c.SponsorPin.ServiceCount = 3
	}
	if c.SponsorPin.MinUptime < 0 || c.SponsorPin.MinUptime > 100 {
		logger.Warn("config", "sponsor_pin.min_uptime 超出范围，已回退默认值", "value", c.SponsorPin.MinUptime, "default", 95.0)
		c.SponsorPin.MinUptime = 95.0
	}
	if !c.SponsorPin.MinLevel.IsValid() || c.SponsorPin.MinLevel == SponsorLevelNone {
		logger.Warn("config", "sponsor_pin.min_level 无效，已回退默认值", "value", c.SponsorPin.MinLevel, "default", SponsorLevelBasic)
		c.SponsorPin.MinLevel = SponsorLevelBasic
	}

	// 自助测试配置默认值与解析（确保运行期与 /api/selftest/config 一致）
	// 注意：默认值与 cmd/server/main.go 保持一致
	if c.SelfTest.MaxConcurrent <= 0 {
		c.SelfTest.MaxConcurrent = 10
	}
	if c.SelfTest.MaxQueueSize <= 0 {
		c.SelfTest.MaxQueueSize = 50
	}
	if c.SelfTest.RateLimitPerMinute <= 0 {
		c.SelfTest.RateLimitPerMinute = 10
	}

	if strings.TrimSpace(c.SelfTest.JobTimeout) == "" {
		c.SelfTest.JobTimeout = "30s"
	}
	{
		d, err := time.ParseDuration(strings.TrimSpace(c.SelfTest.JobTimeout))
		if err != nil || d <= 0 {
			// 保守回退到默认值，避免因为历史配置导致无法启动
			logger.Warn("config", "selftest.job_timeout 无效，已回退默认值", "value", c.SelfTest.JobTimeout, "default", "30s")
			d = 30 * time.Second
			c.SelfTest.JobTimeout = "30s"
		}
		c.SelfTest.JobTimeoutDuration = d
	}

	if strings.TrimSpace(c.SelfTest.ResultTTL) == "" {
		c.SelfTest.ResultTTL = "2m"
	}
	{
		d, err := time.ParseDuration(strings.TrimSpace(c.SelfTest.ResultTTL))
		if err != nil || d <= 0 {
			logger.Warn("config", "selftest.result_ttl 无效，已回退默认值", "value", c.SelfTest.ResultTTL, "default", "2m")
			d = 2 * time.Minute
			c.SelfTest.ResultTTL = "2m"
		}
		c.SelfTest.ResultTTLDuration = d
	}

	// Events 配置默认值
	if c.Events.Mode == "" {
		c.Events.Mode = "model" // 默认按模型独立触发事件
	}
	if c.Events.Mode != "model" && c.Events.Mode != "channel" {
		return fmt.Errorf("events.mode 必须是 'model' 或 'channel'，当前值: %s", c.Events.Mode)
	}
	if c.Events.DownThreshold == 0 {
		c.Events.DownThreshold = 2 // 默认连续 2 次不可用触发 DOWN
	}
	if c.Events.UpThreshold == 0 {
		c.Events.UpThreshold = 1 // 默认 1 次可用触发 UP
	}
	if c.Events.ChannelDownThreshold == 0 {
		c.Events.ChannelDownThreshold = 1 // 默认 1 个模型 DOWN 触发通道 DOWN
	}
	if c.Events.DownThreshold < 1 {
		return fmt.Errorf("events.down_threshold 必须 >= 1，当前值: %d", c.Events.DownThreshold)
	}
	if c.Events.UpThreshold < 1 {
		return fmt.Errorf("events.up_threshold 必须 >= 1，当前值: %d", c.Events.UpThreshold)
	}
	if c.Events.ChannelDownThreshold < 1 {
		return fmt.Errorf("events.channel_down_threshold 必须 >= 1，当前值: %d", c.Events.ChannelDownThreshold)
	}
	if c.Events.ChannelCountMode == "" {
		c.Events.ChannelCountMode = "recompute" // 默认使用重算模式，更稳定
	}
	if c.Events.ChannelCountMode != "incremental" && c.Events.ChannelCountMode != "recompute" {
		return fmt.Errorf("events.channel_count_mode 必须是 'incremental' 或 'recompute'，当前值: %s", c.Events.ChannelCountMode)
	}

	// GitHub 配置默认值与环境变量覆盖
	if err := c.GitHub.Normalize(); err != nil {
		return err
	}

	// 公告配置默认值与解析
	if err := c.Announcements.Normalize(); err != nil {
		return err
	}

	// 公告启用但未配置 token：仅警告（可匿名访问，但容易被限流）
	if c.Announcements.IsEnabled() && strings.TrimSpace(c.GitHub.Token) == "" {
		logger.Warn("config", "announcements 已启用但未配置 GITHUB_TOKEN，将使用匿名请求（可能触发限流）")
	}

	return nil
}

// normalizeStorageConfig 规范化存储配置
// 包括：SQLite/PostgreSQL 配置默认值、连接池参数、retention/archive 配置
func (c *AppConfig) normalizeStorageConfig() error {
	// 存储配置默认值
	if c.Storage.Type == "" {
		c.Storage.Type = "sqlite" // 默认使用 SQLite
	}
	if c.Storage.Type == "sqlite" && c.Storage.SQLite.Path == "" {
		c.Storage.SQLite.Path = "monitor.db" // 默认路径
	}
	// SQLite 参数上限保护：默认上限通常为 999，每个 key 需要 4 个参数 (provider, service, channel, model)
	if c.Storage.Type == "sqlite" && c.EnableBatchQuery {
		const sqliteMaxParams = 999
		const keyParams = 4
		maxKeys := sqliteMaxParams / keyParams
		if c.BatchQueryMaxKeys > maxKeys {
			logger.Warn("config", "batch_query_max_keys 超出 SQLite 参数上限，已回退",
				"value", c.BatchQueryMaxKeys, "sqlite_max_params", sqliteMaxParams, "fallback", maxKeys)
			c.BatchQueryMaxKeys = maxKeys
		}
	}

	// DB 侧 timeline 聚合相关验证
	if c.EnableDBTimelineAgg {
		if c.Storage.Type != "postgres" {
			logger.Warn("config", "enable_db_timeline_agg 仅支持 PostgreSQL，将自动回退到应用层聚合", "storage_type", c.Storage.Type)
		}
		if !c.EnableBatchQuery {
			logger.Info("config", "enable_db_timeline_agg 依赖 enable_batch_query=true 才会生效")
		}
	}

	if c.Storage.Type == "postgres" {
		if c.Storage.Postgres.Port == 0 {
			c.Storage.Postgres.Port = 5432
		}
		if c.Storage.Postgres.SSLMode == "" {
			c.Storage.Postgres.SSLMode = "disable"
		}
		// 连接池配置（考虑并发查询场景）
		// - 串行查询：25 个连接足够（默认保守配置）
		// - 并发查询：建议 50+ 连接（支持多个并发请求）
		if c.Storage.Postgres.MaxOpenConns == 0 {
			// 根据是否启用并发查询设置默认值
			if c.EnableConcurrentQuery {
				c.Storage.Postgres.MaxOpenConns = 50 // 并发查询模式
			} else {
				c.Storage.Postgres.MaxOpenConns = 25 // 串行查询模式
			}
		}
		if c.Storage.Postgres.MaxIdleConns == 0 {
			// 空闲连接数建议为最大连接数的 20-30%
			if c.EnableConcurrentQuery {
				c.Storage.Postgres.MaxIdleConns = 10
			} else {
				c.Storage.Postgres.MaxIdleConns = 5
			}
		}
		if c.Storage.Postgres.ConnMaxLifetime == "" {
			c.Storage.Postgres.ConnMaxLifetime = "1h"
		}

		// 并发查询配置校验（仅警告，不强制修改）
		if c.EnableConcurrentQuery {
			if c.Storage.Postgres.MaxOpenConns > 0 && c.Storage.Postgres.MaxOpenConns < c.ConcurrentQueryLimit {
				logger.Warn("config", "max_open_conns 小于 concurrent_query_limit，可能导致连接池等待",
					"max_open_conns", c.Storage.Postgres.MaxOpenConns, "concurrent_query_limit", c.ConcurrentQueryLimit)
			}
		}
	}

	// SQLite 场景下的并发查询警告
	if c.Storage.Type == "sqlite" && c.EnableConcurrentQuery {
		logger.Warn("config", "SQLite 使用单连接，并发查询无性能收益，建议关闭 enable_concurrent_query")
	}

	// 历史数据保留与清理配置
	if err := c.Storage.Retention.Normalize(); err != nil {
		return err
	}

	// 历史数据归档配置（仅在启用时校验）
	if c.Storage.Archive.IsEnabled() {
		if err := c.Storage.Archive.Normalize(); err != nil {
			return err
		}
		// 校验归档天数应小于保留天数，避免数据在归档前被清理
		// 这是一个严重的配置错误，直接返回错误防止启动
		if c.Storage.Retention.IsEnabled() && c.Storage.Archive.ArchiveDays >= c.Storage.Retention.Days {
			return fmt.Errorf("配置冲突: archive.archive_days(%d) >= retention.days(%d)，数据将在归档前被清理。"+
				"建议: retention.days >= archive_days + backfill_days，如 retention.days=%d",
				c.Storage.Archive.ArchiveDays, c.Storage.Retention.Days,
				c.Storage.Archive.ArchiveDays+c.Storage.Archive.BackfillDays)
		}
		// 校验 backfill 窗口：如果 retention.days < archive_days + backfill_days，停机补齐可能产生空归档
		if c.Storage.Retention.IsEnabled() && c.Storage.Archive.BackfillDays > 1 {
			oldestNeeded := c.Storage.Archive.ArchiveDays + c.Storage.Archive.BackfillDays - 1
			if oldestNeeded >= c.Storage.Retention.Days {
				return fmt.Errorf("配置冲突: archive.archive_days(%d) + backfill_days(%d) - 1 = %d >= retention.days(%d)，"+
					"停机补齐时数据可能已被清理。建议: retention.days >= %d",
					c.Storage.Archive.ArchiveDays, c.Storage.Archive.BackfillDays,
					oldestNeeded, c.Storage.Retention.Days, oldestNeeded+1)
			}
		}
	}

	return nil
}

// buildNormalizeIndexes 构建 Provider/Badge 映射索引
// 这些索引用于后续 normalizeMonitors() 中的状态注入
func (c *AppConfig) buildNormalizeIndexes(ctx *normalizeContext) error {
	// 构建禁用的服务商映射（provider -> reason）
	// 注意：provider 统一转小写，与 API 查询逻辑保持一致
	for i, dp := range c.DisabledProviders {
		provider := strings.ToLower(strings.TrimSpace(dp.Provider))
		if provider == "" {
			return fmt.Errorf("disabled_providers[%d]: provider 不能为空", i)
		}
		if _, exists := ctx.disabledProviderMap[provider]; exists {
			return fmt.Errorf("disabled_providers[%d]: provider '%s' 重复配置", i, dp.Provider)
		}
		ctx.disabledProviderMap[provider] = strings.TrimSpace(dp.Reason)
	}

	// 构建隐藏的服务商映射（provider -> reason）
	// 注意：provider 统一转小写，与 API 查询逻辑保持一致
	for i, hp := range c.HiddenProviders {
		provider := strings.ToLower(strings.TrimSpace(hp.Provider))
		if provider == "" {
			return fmt.Errorf("hidden_providers[%d]: provider 不能为空", i)
		}
		if _, exists := ctx.hiddenProviderMap[provider]; exists {
			return fmt.Errorf("hidden_providers[%d]: provider '%s' 重复配置", i, hp.Provider)
		}
		ctx.hiddenProviderMap[provider] = strings.TrimSpace(hp.Reason)
	}

	// 构建 risk_providers 快速查找 map
	for i, rp := range c.RiskProviders {
		provider := strings.ToLower(strings.TrimSpace(rp.Provider))
		if provider == "" {
			return fmt.Errorf("risk_providers[%d]: provider 不能为空", i)
		}
		if _, exists := ctx.riskProviderMap[provider]; exists {
			return fmt.Errorf("risk_providers[%d]: provider '%s' 重复配置", i, rp.Provider)
		}
		ctx.riskProviderMap[provider] = rp.Risks
	}

	// 构建 badges 定义 map（id -> def），并填充默认值
	// 先加载内置默认徽标，再加载用户配置（用户配置可覆盖内置）

	// 1. 加载内置默认徽标
	for id, bd := range defaultBadgeDefs {
		ctx.badgeDefMap[id] = bd
	}

	// 2. 加载用户配置的徽标（可覆盖内置）
	for id, bd := range c.BadgeDefs {
		// 填充默认值
		if bd.Kind == "" {
			bd.Kind = BadgeKindInfo
		}
		if bd.Variant == "" {
			bd.Variant = BadgeVariantDefault
		}
		bd.ID = id // 确保 ID 字段与 map key 一致
		ctx.badgeDefMap[id] = bd
	}

	// 构建 badge_providers 快速查找 map（provider -> []BadgeRef）
	for _, bp := range c.BadgeProviders {
		provider := strings.ToLower(strings.TrimSpace(bp.Provider))
		ctx.badgeProviderMap[provider] = bp.Badges
	}

	return nil
}
