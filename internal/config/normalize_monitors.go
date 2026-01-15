package config

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"monitor/internal/logger"
)

// normalizeMonitorsPreInheritance 继承前的监测项规范化
// 包括：派生字段重置、时间配置下发、重试配置下发、元数据规范化、Provider 状态注入
// 注意：不包括 board 默认值填充和徽标解析，这些需要在继承后处理
func (c *AppConfig) normalizeMonitorsPreInheritance(ctx *normalizeContext) error {
	for i := range c.Monitors {
		// 注意：以下 yaml:"-" 字段在热更新/复用 slice 元素的场景下，旧值可能残留。
		// 每次 Normalize 都从零值开始重新计算，确保派生逻辑稳定。
		c.Monitors[i].SlowLatencyDuration = 0
		c.Monitors[i].TimeoutDuration = 0
		c.Monitors[i].IntervalDuration = 0
		c.Monitors[i].RetryCount = 0
		c.Monitors[i].RetryBaseDelayDuration = 0
		c.Monitors[i].RetryMaxDelayDuration = 0
		c.Monitors[i].RetryJitterValue = 0
		c.Monitors[i].Risks = nil          // 由 ctx.riskProviderMap 重新注入
		c.Monitors[i].ResolvedBadges = nil // 由徽标解析逻辑重新计算（在 post-inheritance 阶段）

		// 解析 monitor 级 slow_latency（如有配置）
		if trimmed := strings.TrimSpace(c.Monitors[i].SlowLatency); trimmed != "" {
			d, err := time.ParseDuration(trimmed)
			if err != nil {
				return fmt.Errorf("monitor[%d] (provider=%s, service=%s, channel=%s): 解析 slow_latency 失败: %w",
					i, c.Monitors[i].Provider, c.Monitors[i].Service, c.Monitors[i].Channel, err)
			}
			if d <= 0 {
				return fmt.Errorf("monitor[%d] (provider=%s, service=%s, channel=%s): slow_latency 必须大于 0",
					i, c.Monitors[i].Provider, c.Monitors[i].Service, c.Monitors[i].Channel)
			}
			c.Monitors[i].SlowLatencyDuration = d
		}

		// slow_latency 下发：monitor > by_service > global
		if c.Monitors[i].SlowLatencyDuration == 0 {
			serviceKey := strings.ToLower(strings.TrimSpace(c.Monitors[i].Service))
			if d, ok := c.SlowLatencyByServiceDuration[serviceKey]; ok {
				c.Monitors[i].SlowLatencyDuration = d
			} else {
				c.Monitors[i].SlowLatencyDuration = c.SlowLatencyDuration
			}
		}

		// 解析 monitor 级 timeout（如有配置）
		if trimmed := strings.TrimSpace(c.Monitors[i].Timeout); trimmed != "" {
			d, err := time.ParseDuration(trimmed)
			if err != nil {
				return fmt.Errorf("monitor[%d] (provider=%s, service=%s, channel=%s): 解析 timeout 失败: %w",
					i, c.Monitors[i].Provider, c.Monitors[i].Service, c.Monitors[i].Channel, err)
			}
			if d <= 0 {
				return fmt.Errorf("monitor[%d] (provider=%s, service=%s, channel=%s): timeout 必须大于 0",
					i, c.Monitors[i].Provider, c.Monitors[i].Service, c.Monitors[i].Channel)
			}
			c.Monitors[i].TimeoutDuration = d
		}

		// timeout 下发：monitor > by_service > global
		if c.Monitors[i].TimeoutDuration == 0 {
			serviceKey := strings.ToLower(strings.TrimSpace(c.Monitors[i].Service))
			if d, ok := c.TimeoutByServiceDuration[serviceKey]; ok {
				c.Monitors[i].TimeoutDuration = d
			} else {
				c.Monitors[i].TimeoutDuration = c.TimeoutDuration
			}
		}

		// 警告：slow_latency >= timeout 时黄灯基本不会触发
		if c.Monitors[i].SlowLatencyDuration >= c.Monitors[i].TimeoutDuration {
			logger.Warn("config", "slow_latency >= timeout，慢响应黄灯可能不会触发",
				"monitor_index", i,
				"provider", c.Monitors[i].Provider,
				"service", c.Monitors[i].Service,
				"channel", c.Monitors[i].Channel,
				"slow_latency", c.Monitors[i].SlowLatencyDuration,
				"timeout", c.Monitors[i].TimeoutDuration)
		}

		// 解析单监测项的 interval，空值回退到全局
		if trimmed := strings.TrimSpace(c.Monitors[i].Interval); trimmed != "" {
			d, err := time.ParseDuration(trimmed)
			if err != nil {
				return fmt.Errorf("monitor[%d]: 解析 interval 失败: %w", i, err)
			}
			if d <= 0 {
				return fmt.Errorf("monitor[%d]: interval 必须大于 0", i)
			}
			c.Monitors[i].IntervalDuration = d
		} else {
			c.Monitors[i].IntervalDuration = c.IntervalDuration
		}

		// ===== 重试配置下发 =====
		serviceKey := strings.ToLower(strings.TrimSpace(c.Monitors[i].Service))

		// retry 下发：monitor > by_service > global
		if c.Monitors[i].Retry != nil {
			if *c.Monitors[i].Retry < 0 {
				return fmt.Errorf("monitor[%d] (provider=%s, service=%s, channel=%s): retry 必须 >= 0",
					i, c.Monitors[i].Provider, c.Monitors[i].Service, c.Monitors[i].Channel)
			}
			c.Monitors[i].RetryCount = *c.Monitors[i].Retry
		} else if v, ok := c.RetryByServiceCount[serviceKey]; ok {
			c.Monitors[i].RetryCount = v
		} else {
			c.Monitors[i].RetryCount = c.RetryCount
		}

		// retry_base_delay 下发：monitor > by_service > global
		if trimmed := strings.TrimSpace(c.Monitors[i].RetryBaseDelay); trimmed != "" {
			d, err := time.ParseDuration(trimmed)
			if err != nil {
				return fmt.Errorf("monitor[%d] (provider=%s, service=%s, channel=%s): 解析 retry_base_delay 失败: %w",
					i, c.Monitors[i].Provider, c.Monitors[i].Service, c.Monitors[i].Channel, err)
			}
			if d <= 0 {
				return fmt.Errorf("monitor[%d] (provider=%s, service=%s, channel=%s): retry_base_delay 必须 > 0",
					i, c.Monitors[i].Provider, c.Monitors[i].Service, c.Monitors[i].Channel)
			}
			c.Monitors[i].RetryBaseDelayDuration = d
		} else if d, ok := c.RetryBaseDelayByServiceDuration[serviceKey]; ok {
			c.Monitors[i].RetryBaseDelayDuration = d
		} else {
			c.Monitors[i].RetryBaseDelayDuration = c.RetryBaseDelayDuration
		}

		// retry_max_delay 下发：monitor > by_service > global
		if trimmed := strings.TrimSpace(c.Monitors[i].RetryMaxDelay); trimmed != "" {
			d, err := time.ParseDuration(trimmed)
			if err != nil {
				return fmt.Errorf("monitor[%d] (provider=%s, service=%s, channel=%s): 解析 retry_max_delay 失败: %w",
					i, c.Monitors[i].Provider, c.Monitors[i].Service, c.Monitors[i].Channel, err)
			}
			if d <= 0 {
				return fmt.Errorf("monitor[%d] (provider=%s, service=%s, channel=%s): retry_max_delay 必须 > 0",
					i, c.Monitors[i].Provider, c.Monitors[i].Service, c.Monitors[i].Channel)
			}
			c.Monitors[i].RetryMaxDelayDuration = d
		} else if d, ok := c.RetryMaxDelayByServiceDuration[serviceKey]; ok {
			c.Monitors[i].RetryMaxDelayDuration = d
		} else {
			c.Monitors[i].RetryMaxDelayDuration = c.RetryMaxDelayDuration
		}

		// retry_jitter 下发：monitor > by_service > global
		if c.Monitors[i].RetryJitter != nil {
			v := *c.Monitors[i].RetryJitter
			if v < 0 || v > 1 {
				return fmt.Errorf("monitor[%d] (provider=%s, service=%s, channel=%s): retry_jitter 必须在 0 到 1 之间",
					i, c.Monitors[i].Provider, c.Monitors[i].Service, c.Monitors[i].Channel)
			}
			c.Monitors[i].RetryJitterValue = v
		} else if v, ok := c.RetryJitterByServiceValue[serviceKey]; ok {
			c.Monitors[i].RetryJitterValue = v
		} else {
			c.Monitors[i].RetryJitterValue = c.RetryJitterValue
		}

		// 最终校验：max >= base
		if c.Monitors[i].RetryMaxDelayDuration < c.Monitors[i].RetryBaseDelayDuration {
			return fmt.Errorf("monitor[%d] (provider=%s, service=%s, channel=%s): retry_max_delay 必须 >= retry_base_delay",
				i, c.Monitors[i].Provider, c.Monitors[i].Service, c.Monitors[i].Channel)
		}

		// Board 和 ColdReason 仅做 trim，不填充默认值（留给 post-inheritance 处理）
		// 这样子通道可以正确继承父通道的 board 配置
		c.Monitors[i].Board = strings.ToLower(strings.TrimSpace(c.Monitors[i].Board))
		c.Monitors[i].ColdReason = strings.TrimSpace(c.Monitors[i].ColdReason)

		// 标准化 category 为小写（与 Validate 的 isValidCategory 保持一致）
		c.Monitors[i].Category = strings.ToLower(strings.TrimSpace(c.Monitors[i].Category))

		// 规范化 URLs：去除首尾空格和末尾的 /
		c.Monitors[i].ProviderURL = strings.TrimRight(strings.TrimSpace(c.Monitors[i].ProviderURL), "/")
		c.Monitors[i].SponsorURL = strings.TrimRight(strings.TrimSpace(c.Monitors[i].SponsorURL), "/")

		// provider_slug 仅做 trim，不填充默认值（留给 post-inheritance 处理）
		// 这样子通道可以正确继承父通道的 provider_slug 配置
		c.Monitors[i].ProviderSlug = strings.TrimSpace(c.Monitors[i].ProviderSlug)

		// 显示名称：仅做 trim 处理，不做回退
		// 空值表示"未配置"，由前端使用默认格式化逻辑
		c.Monitors[i].ProviderName = strings.TrimSpace(c.Monitors[i].ProviderName)
		c.Monitors[i].ServiceName = strings.TrimSpace(c.Monitors[i].ServiceName)
		c.Monitors[i].ChannelName = strings.TrimSpace(c.Monitors[i].ChannelName)

		// 计算最终禁用状态：providerDisabled || monitorDisabled
		// 原因优先级：monitor.DisabledReason > provider.Reason
		// 注意：查找时使用小写 provider，与 disabledProviderMap 构建逻辑一致
		normalizedProvider := strings.ToLower(strings.TrimSpace(c.Monitors[i].Provider))
		providerDisabledReason, providerDisabled := ctx.disabledProviderMap[normalizedProvider]
		if providerDisabled || c.Monitors[i].Disabled {
			c.Monitors[i].Disabled = true
			// 如果 monitor 自身没有设置原因，使用 provider 级别的原因
			monitorReason := strings.TrimSpace(c.Monitors[i].DisabledReason)
			if monitorReason == "" && providerDisabled {
				c.Monitors[i].DisabledReason = providerDisabledReason
			} else {
				c.Monitors[i].DisabledReason = monitorReason
			}
			// 停用即视为隐藏，防止展示，同时使用停用原因作为隐藏原因
			c.Monitors[i].Hidden = true
			if strings.TrimSpace(c.Monitors[i].HiddenReason) == "" {
				c.Monitors[i].HiddenReason = c.Monitors[i].DisabledReason
			}
		}

		// 计算最终隐藏状态：providerHidden || monitorHidden（仅对未禁用的项）
		// 原因优先级：monitor.HiddenReason > provider.Reason
		// 已禁用的监测项无需再覆盖隐藏原因
		providerReason, providerHidden := ctx.hiddenProviderMap[normalizedProvider]
		if !c.Monitors[i].Disabled && (providerHidden || c.Monitors[i].Hidden) {
			c.Monitors[i].Hidden = true
			// 如果 monitor 自身没有设置原因，使用 provider 级别的原因
			monitorReason := strings.TrimSpace(c.Monitors[i].HiddenReason)
			if monitorReason == "" && providerHidden {
				c.Monitors[i].HiddenReason = providerReason
			} else {
				c.Monitors[i].HiddenReason = monitorReason
			}
		}

		// 从 risk_providers 注入风险徽标到对应的 monitors
		if risks, exists := ctx.riskProviderMap[normalizedProvider]; exists {
			c.Monitors[i].Risks = risks
		}

		// Proxy 规范化（TrimSpace + scheme 小写化 + 去掉尾部 /）
		// 无条件先做 TrimSpace，确保空白字符串被清空
		c.Monitors[i].Proxy = strings.TrimSpace(c.Monitors[i].Proxy)
		if c.Monitors[i].Proxy != "" {
			parsed, err := url.Parse(c.Monitors[i].Proxy)
			if err == nil && parsed.Scheme != "" {
				// scheme 小写化
				parsed.Scheme = strings.ToLower(parsed.Scheme)
				// 去掉尾部 /（path 只保留空或非 / 的情况）
				if parsed.Path == "/" {
					parsed.Path = ""
				}
				c.Monitors[i].Proxy = parsed.String()
			}
			// 解析失败时保留 TrimSpace 后的值（防御性保留，正常调用链下不应触发）
		}
	}

	return nil
}

// normalizeMonitorsPostInheritance 继承后的监测项规范化
// 包括：board 默认值填充、provider_slug 默认值填充、cold_reason 清理、徽标解析
// 必须在 applyParentInheritance() 之后调用，确保继承的字段能正确处理
func (c *AppConfig) normalizeMonitorsPostInheritance(ctx *normalizeContext) error {
	for i := range c.Monitors {
		// Board 默认值填充：继承后仍为空则设为 "hot"
		if c.Monitors[i].Board == "" {
			c.Monitors[i].Board = "hot"
		}

		// cold_reason 仅在 board=cold 时有意义，其他情况清空并警告
		// 必须在继承后检查，因为继承可能带入 cold_reason
		if c.Monitors[i].ColdReason != "" && c.Monitors[i].Board != "cold" {
			logger.Warn("config", "cold_reason 仅在 board=cold 时有效，已忽略",
				"monitor_index", i,
				"provider", c.Monitors[i].Provider,
				"service", c.Monitors[i].Service)
			c.Monitors[i].ColdReason = ""
		}

		// provider_slug 默认值填充和验证：继承后仍为空则自动生成
		slug := c.Monitors[i].ProviderSlug
		if slug == "" {
			// 未配置时，自动生成: provider 转小写
			slug = strings.ToLower(strings.TrimSpace(c.Monitors[i].Provider))
		}

		// 无论自动生成还是手动配置/继承，都进行格式验证
		// 确保配置期即可发现 slug 格式问题，避免运行时 404
		if err := validateProviderSlug(slug); err != nil {
			return fmt.Errorf("monitor[%d]: provider_slug '%s' 无效 (来源: %s): %w",
				i, slug,
				map[bool]string{true: "自动生成", false: "手动配置或继承"}[c.Monitors[i].ProviderSlug == ""],
				err)
		}
		c.Monitors[i].ProviderSlug = slug

		// 从 badge_providers + monitors[].badges 解析徽标
		// 必须在继承后处理，确保子通道继承的 Badges 能正确解析为 ResolvedBadges
		// 仅在启用徽标系统时处理
		if c.EnableBadges {
			normalizedProvider := strings.ToLower(strings.TrimSpace(c.Monitors[i].Provider))
			var refs []BadgeRef
			if injected, ok := ctx.badgeProviderMap[normalizedProvider]; ok && len(injected) > 0 {
				refs = append(refs, injected...)
			}
			if len(c.Monitors[i].Badges) > 0 {
				refs = append(refs, c.Monitors[i].Badges...)
			}

			// 如果没有配置任何徽标，注入默认徽标
			if len(refs) == 0 {
				refs = []BadgeRef{{ID: "api_key_official"}}
			}

			// 去重并解析为 ResolvedBadge
			order := make([]string, 0, len(refs))
			resolvedMap := make(map[string]ResolvedBadge, len(refs))
			for _, ref := range refs {
				id := strings.TrimSpace(ref.ID)
				if id == "" {
					continue
				}
				def, exists := ctx.badgeDefMap[id]
				if !exists {
					continue // 验证阶段已检查，此处跳过
				}

				// monitor 级 tooltip 覆盖
				tooltipOverride := strings.TrimSpace(ref.Tooltip)

				if _, seen := resolvedMap[id]; !seen {
					order = append(order, id)
				}
				resolvedMap[id] = ResolvedBadge{
					ID:              id,
					Kind:            def.Kind,
					Variant:         def.Variant,
					Weight:          def.Weight,
					URL:             def.URL,
					TooltipOverride: tooltipOverride,
				}
			}

			// 按 kind 组顺序 → weight desc → id asc 排序
			result := make([]ResolvedBadge, 0, len(order))
			for _, id := range order {
				result = append(result, resolvedMap[id])
			}
			sort.SliceStable(result, func(a, b int) bool {
				kindOrder := map[BadgeKind]int{BadgeKindSource: 0, BadgeKindFeature: 1, BadgeKindInfo: 2}
				if kindOrder[result[a].Kind] != kindOrder[result[b].Kind] {
					return kindOrder[result[a].Kind] < kindOrder[result[b].Kind]
				}
				if result[a].Weight != result[b].Weight {
					return result[a].Weight > result[b].Weight // desc
				}
				return result[a].ID < result[b].ID // asc
			})
			c.Monitors[i].ResolvedBadges = result
		}
	}

	return nil
}
