package config

import (
	"fmt"
	"strings"
	"time"

	"monitor/internal/logger"
)

// applyParentInheritance 实现子通道从父通道继承配置
func (c *AppConfig) applyParentInheritance() error {
	// 构建父通道索引
	rootByPath := make(map[string]*ServiceConfig)
	for i := range c.Monitors {
		if strings.TrimSpace(c.Monitors[i].Parent) != "" {
			continue
		}
		path := fmt.Sprintf("%s/%s/%s", c.Monitors[i].Provider, c.Monitors[i].Service, c.Monitors[i].Channel)
		if existing, exists := rootByPath[path]; exists {
			// 标记为多定义（nil 表示冲突）
			if existing != nil {
				rootByPath[path] = nil
			}
			continue
		}
		rootByPath[path] = &c.Monitors[i]
	}

	// 应用继承
	for i := range c.Monitors {
		child := &c.Monitors[i]
		parentPath := strings.TrimSpace(child.Parent)
		if parentPath == "" {
			continue
		}

		// 注意：provider/service/channel 的继承已在 Validate() 步骤 0 完成
		// 这里直接查找父通道
		parent := rootByPath[parentPath]
		if parent == nil {
			return fmt.Errorf("monitor[%d]: 找不到父通道: %s", i, parentPath)
		}

		// 应用各类配置继承
		inheritCoreBehavior(child, parent)
		inheritedTimings := inheritTimings(child, parent)
		inheritedRetry := inheritRetry(child, parent)
		inheritMeta(child, parent)
		inheritState(child, parent)
		inheritBadgesAndDisplay(child, parent)

		// 修复继承后的 Duration 字段
		if err := fixInheritedDurations(i, child, inheritedTimings, inheritedRetry); err != nil {
			return err
		}

		// 注意：以下字段不继承（有特殊约束）：
		// - Model: 父子关系的唯一区分字段，若继承则变成重复项
		// - Provider/Service/Channel: 由父子路径验证强制一致
	}

	return nil
}

// inheritCoreBehavior 继承核心监测行为配置
// 包括：APIKey、URL、Method、Body、BodyTemplateName、SuccessContains、EnvVarName、Proxy、Headers
func inheritCoreBehavior(child, parent *ServiceConfig) {
	if child.APIKey == "" {
		child.APIKey = parent.APIKey
	}
	if child.URL == "" {
		child.URL = parent.URL
	}
	if child.Method == "" {
		child.Method = parent.Method
	}
	if child.Body == "" {
		child.Body = parent.Body
		// Body 继承时一并继承 BodyTemplateName（用于 API 返回 template_name）
		if child.BodyTemplateName == "" {
			child.BodyTemplateName = parent.BodyTemplateName
		}
	}
	if child.SuccessContains == "" {
		child.SuccessContains = parent.SuccessContains
	}
	// 自定义环境变量名（用于 API Key 查找）
	if child.EnvVarName == "" {
		child.EnvVarName = parent.EnvVarName
	}

	// Proxy 继承（子通道可继承父通道的代理配置）
	// 使用 TrimSpace 判空，与 Normalize 逻辑保持一致
	if strings.TrimSpace(child.Proxy) == "" {
		child.Proxy = parent.Proxy
	}

	// Headers 继承（合并策略：父为基础，子覆盖）
	if len(parent.Headers) > 0 {
		merged := make(map[string]string, len(parent.Headers)+len(child.Headers))
		for k, v := range parent.Headers {
			merged[k] = v
		}
		for k, v := range child.Headers {
			merged[k] = v // 子覆盖父
		}
		child.Headers = merged
	}
}

// inheritedTimingsFlags 记录哪些时间配置字段是从 parent 继承的
type inheritedTimingsFlags struct {
	SlowLatency bool
	Timeout     bool
	Interval    bool
}

// inheritTimings 继承时间/阈值配置
// 返回标记哪些字段是继承的（用于后续重新计算 Duration）
func inheritTimings(child, parent *ServiceConfig) inheritedTimingsFlags {
	flags := inheritedTimingsFlags{}

	// SlowLatency: 字符串形式，空值表示未配置
	if strings.TrimSpace(child.SlowLatency) == "" && strings.TrimSpace(parent.SlowLatency) != "" {
		child.SlowLatency = parent.SlowLatency
		flags.SlowLatency = true
	}
	// Timeout: 字符串形式，空值表示未配置
	if strings.TrimSpace(child.Timeout) == "" && strings.TrimSpace(parent.Timeout) != "" {
		child.Timeout = parent.Timeout
		flags.Timeout = true
	}
	// Interval: 字符串形式，空值表示未配置
	if strings.TrimSpace(child.Interval) == "" && strings.TrimSpace(parent.Interval) != "" {
		child.Interval = parent.Interval
		flags.Interval = true
	}

	return flags
}

// inheritedRetryFlags 记录哪些重试配置字段是从 parent 继承的
type inheritedRetryFlags struct {
	Retry          bool
	RetryBaseDelay bool
	RetryMaxDelay  bool
	RetryJitter    bool
}

// inheritRetry 继承重试配置
// 返回标记哪些字段是继承的（用于后续重新计算 Duration）
func inheritRetry(child, parent *ServiceConfig) inheritedRetryFlags {
	flags := inheritedRetryFlags{}

	// Retry: nil 表示未配置，从 parent 继承
	if child.Retry == nil && parent.Retry != nil {
		v := *parent.Retry
		child.Retry = &v
		child.RetryCount = v
		flags.Retry = true
	}
	if strings.TrimSpace(child.RetryBaseDelay) == "" && strings.TrimSpace(parent.RetryBaseDelay) != "" {
		child.RetryBaseDelay = parent.RetryBaseDelay
		flags.RetryBaseDelay = true
	}
	if strings.TrimSpace(child.RetryMaxDelay) == "" && strings.TrimSpace(parent.RetryMaxDelay) != "" {
		child.RetryMaxDelay = parent.RetryMaxDelay
		flags.RetryMaxDelay = true
	}
	if child.RetryJitter == nil && parent.RetryJitter != nil {
		v := *parent.RetryJitter
		child.RetryJitter = &v
		child.RetryJitterValue = v
		flags.RetryJitter = true
	}

	return flags
}

// inheritMeta 继承元数据配置
// 包括：Category、Sponsor、Provider 相关元数据、Board 配置
func inheritMeta(child, parent *ServiceConfig) {
	// Category: 必填字段，但子通道可能想继承
	if child.Category == "" {
		child.Category = parent.Category
	}
	// Sponsor: 继承（通常同一 provider 的赞助者相同）
	if child.Sponsor == "" {
		child.Sponsor = parent.Sponsor
	}
	if child.SponsorURL == "" {
		child.SponsorURL = parent.SponsorURL
	}
	if child.SponsorLevel == "" {
		child.SponsorLevel = parent.SponsorLevel
	}
	// Provider 相关元数据
	if child.ProviderURL == "" {
		child.ProviderURL = parent.ProviderURL
	}
	if child.ProviderSlug == "" {
		child.ProviderSlug = parent.ProviderSlug
	}
	if child.ProviderName == "" {
		child.ProviderName = parent.ProviderName
	}
	if child.ServiceName == "" {
		child.ServiceName = parent.ServiceName
	}

	// --- 板块配置 ---
	// Board: 空值会在 Normalize 中默认为 "hot"，这里只继承显式配置
	if child.Board == "" && parent.Board != "" {
		child.Board = parent.Board
	}
	if child.ColdReason == "" && parent.ColdReason != "" {
		child.ColdReason = parent.ColdReason
	}
}

// inheritState 继承状态配置（级联 OR 逻辑）
// 包括：Disabled、Hidden
func inheritState(child, parent *ServiceConfig) {
	// Disabled: 父禁用则子也禁用
	if parent.Disabled {
		child.Disabled = true
		if child.DisabledReason == "" {
			child.DisabledReason = parent.DisabledReason
		}
	}
	// Hidden: 父隐藏则子也隐藏
	if parent.Hidden {
		child.Hidden = true
		if child.HiddenReason == "" {
			child.HiddenReason = parent.HiddenReason
		}
	}
}

// inheritBadgesAndDisplay 继承徽标和显示相关配置
// 包括：Badges、ChannelName、定价信息、收录日期
func inheritBadgesAndDisplay(child, parent *ServiceConfig) {
	// Badges: 子为空时继承父的徽标（替换策略，非合并）
	if len(child.Badges) == 0 && len(parent.Badges) > 0 {
		child.Badges = make([]BadgeRef, len(parent.Badges))
		copy(child.Badges, parent.Badges)
	}

	// 显示名称继承（子为空时继承）
	if child.ChannelName == "" {
		child.ChannelName = parent.ChannelName
	}

	// 定价信息继承（子为 nil 时继承）
	if child.PriceMin == nil && parent.PriceMin != nil {
		v := *parent.PriceMin
		child.PriceMin = &v
	}
	if child.PriceMax == nil && parent.PriceMax != nil {
		v := *parent.PriceMax
		child.PriceMax = &v
	}

	// 收录日期继承（子为空时继承）
	if child.ListedSince == "" {
		child.ListedSince = parent.ListedSince
	}
}

// fixInheritedDurations 修复继承后的 Duration 字段
// Validate() 中 Duration 解析发生在 applyParentInheritance() 之前，
// 当子通道通过 parent 继承了字符串字段时，需要重新计算对应的 Duration 字段
func fixInheritedDurations(
	monitorIdx int,
	child *ServiceConfig,
	timings inheritedTimingsFlags,
	retry inheritedRetryFlags,
) error {
	// 修复时间配置 Duration
	if timings.Interval {
		trimmed := strings.TrimSpace(child.Interval)
		d, err := time.ParseDuration(trimmed)
		if err != nil {
			return fmt.Errorf("monitor[%d]: 解析继承的 interval 失败: %w", monitorIdx, err)
		}
		if d <= 0 {
			return fmt.Errorf("monitor[%d]: 继承的 interval 必须大于 0", monitorIdx)
		}
		child.IntervalDuration = d
	}

	if timings.SlowLatency {
		trimmed := strings.TrimSpace(child.SlowLatency)
		d, err := time.ParseDuration(trimmed)
		if err != nil {
			return fmt.Errorf("monitor[%d] (provider=%s, service=%s, channel=%s): 解析继承的 slow_latency 失败: %w",
				monitorIdx, child.Provider, child.Service, child.Channel, err)
		}
		if d <= 0 {
			return fmt.Errorf("monitor[%d] (provider=%s, service=%s, channel=%s): 继承的 slow_latency 必须大于 0",
				monitorIdx, child.Provider, child.Service, child.Channel)
		}
		child.SlowLatencyDuration = d
	}

	if timings.Timeout {
		trimmed := strings.TrimSpace(child.Timeout)
		d, err := time.ParseDuration(trimmed)
		if err != nil {
			return fmt.Errorf("monitor[%d] (provider=%s, service=%s, channel=%s): 解析继承的 timeout 失败: %w",
				monitorIdx, child.Provider, child.Service, child.Channel, err)
		}
		if d <= 0 {
			return fmt.Errorf("monitor[%d] (provider=%s, service=%s, channel=%s): 继承的 timeout 必须大于 0",
				monitorIdx, child.Provider, child.Service, child.Channel)
		}
		child.TimeoutDuration = d
	}

	// 继承后重新检查：slow_latency >= timeout 时黄灯基本不会触发
	if (timings.SlowLatency || timings.Timeout) &&
		child.SlowLatencyDuration >= child.TimeoutDuration {
		logger.Warn("config", "slow_latency >= timeout，慢响应黄灯可能不会触发（继承自 parent）",
			"monitor_index", monitorIdx,
			"provider", child.Provider,
			"service", child.Service,
			"channel", child.Channel,
			"model", child.Model,
			"slow_latency", child.SlowLatencyDuration,
			"timeout", child.TimeoutDuration)
	}

	// 修复重试配置 Duration
	// RetryCount 和 RetryJitterValue 已在继承时直接赋值，无需额外解析
	_ = retry.Retry
	_ = retry.RetryJitter

	if retry.RetryBaseDelay {
		trimmed := strings.TrimSpace(child.RetryBaseDelay)
		d, err := time.ParseDuration(trimmed)
		if err != nil {
			return fmt.Errorf("monitor[%d] (provider=%s, service=%s, channel=%s): 解析继承的 retry_base_delay 失败: %w",
				monitorIdx, child.Provider, child.Service, child.Channel, err)
		}
		if d <= 0 {
			return fmt.Errorf("monitor[%d] (provider=%s, service=%s, channel=%s): 继承的 retry_base_delay 必须 > 0",
				monitorIdx, child.Provider, child.Service, child.Channel)
		}
		child.RetryBaseDelayDuration = d
	}

	if retry.RetryMaxDelay {
		trimmed := strings.TrimSpace(child.RetryMaxDelay)
		d, err := time.ParseDuration(trimmed)
		if err != nil {
			return fmt.Errorf("monitor[%d] (provider=%s, service=%s, channel=%s): 解析继承的 retry_max_delay 失败: %w",
				monitorIdx, child.Provider, child.Service, child.Channel, err)
		}
		if d <= 0 {
			return fmt.Errorf("monitor[%d] (provider=%s, service=%s, channel=%s): 继承的 retry_max_delay 必须 > 0",
				monitorIdx, child.Provider, child.Service, child.Channel)
		}
		child.RetryMaxDelayDuration = d
	}

	// 继承后重新检查：max >= base
	if (retry.RetryBaseDelay || retry.RetryMaxDelay) &&
		child.RetryMaxDelayDuration < child.RetryBaseDelayDuration {
		return fmt.Errorf("monitor[%d] (provider=%s, service=%s, channel=%s): 继承后 retry_max_delay 必须 >= retry_base_delay",
			monitorIdx, child.Provider, child.Service, child.Channel)
	}

	return nil
}
