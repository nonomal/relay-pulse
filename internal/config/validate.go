package config

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"monitor/internal/logger"
)

// validateContext 承载 Validate() 过程中的中间数据
// 采用结构体而非多返回值，便于扩展且更具可读性
type validateContext struct {
	// 四元组唯一性集合 (provider/service/channel/model)
	quadrupleKeySet map[string]struct{}
	// 父通道索引 (三元组 path -> *ServiceConfig, nil 表示多定义冲突)
	rootByPath map[string]*ServiceConfig
	// 父子关系图 (childKey -> parentKey，单父映射)
	parentOf map[string]string
}

// newValidateContext 创建并初始化 validateContext
// 集中初始化避免各步骤遗漏 make() 导致 nil map panic
func newValidateContext() *validateContext {
	return &validateContext{
		quadrupleKeySet: make(map[string]struct{}),
		rootByPath:      make(map[string]*ServiceConfig),
		parentOf:        make(map[string]string),
	}
}

// Validate 验证配置合法性
// 注意：此方法有副作用，会预处理子通道的 provider/service/channel 继承
func (c *AppConfig) Validate() error {
	if len(c.Monitors) == 0 {
		return fmt.Errorf("至少需要配置一个监测项")
	}

	// 0. 预处理：子项的 provider/service/channel 从 parent 路径继承
	if err := c.preprocessParentInheritance(); err != nil {
		return err
	}

	// 1. 四元组唯一性检查
	ctx := newValidateContext()
	if err := c.validateMonitorUniqueness(ctx); err != nil {
		return err
	}

	// 2-4. 父子约束校验：收集引用、构建索引、验证存在性
	if err := c.buildAndValidateParentGraph(ctx); err != nil {
		return err
	}

	// 5. 循环引用检测
	if err := c.validateNoCycles(ctx); err != nil {
		return err
	}

	// 5.5 多父层警告
	c.warnMultipleParentLayers()

	// 6. 监测项字段校验
	if err := c.validateMonitorFields(); err != nil {
		return err
	}

	// 7. Provider 配置校验
	if err := c.validateProviderConfigs(); err != nil {
		return err
	}

	// 8. Badge 配置校验
	if err := c.validateBadgeConfigs(); err != nil {
		return err
	}

	return nil
}

// preprocessParentInheritance 预处理子通道的 provider/service/channel 继承
// 必须在唯一性检查前完成，否则子项的 key 不完整
// 注意：此方法会修改 c.Monitors，需保证幂等
func (c *AppConfig) preprocessParentInheritance() error {
	for i := range c.Monitors {
		m := &c.Monitors[i]
		parentPath := strings.TrimSpace(m.Parent)
		if parentPath == "" {
			continue
		}

		parts := strings.Split(parentPath, "/")
		if len(parts) != 3 {
			return fmt.Errorf("monitor[%d]: parent 格式错误: %s (应为 provider/service/channel)", i, parentPath)
		}
		parentProvider, parentService, parentChannel := parts[0], parts[1], parts[2]

		// 子的 provider/service/channel：为空则从 parent 继承；非空则必须与 parent 一致
		if m.Provider == "" {
			m.Provider = parentProvider
		} else if m.Provider != parentProvider {
			return fmt.Errorf("monitor[%d]: 子通道 provider '%s' 与 parent '%s' 不一致，不支持覆盖", i, m.Provider, parentProvider)
		}
		if m.Service == "" {
			m.Service = parentService
		} else if m.Service != parentService {
			return fmt.Errorf("monitor[%d]: 子通道 service '%s' 与 parent '%s' 不一致，不支持覆盖", i, m.Service, parentService)
		}
		if m.Channel == "" {
			m.Channel = parentChannel
		} else if m.Channel != parentChannel {
			return fmt.Errorf("monitor[%d]: 子通道 channel '%s' 与 parent '%s' 不一致，不支持覆盖", i, m.Channel, parentChannel)
		}
	}
	return nil
}

// validateMonitorUniqueness 检查四元组唯一性 (provider/service/channel/model)
func (c *AppConfig) validateMonitorUniqueness(ctx *validateContext) error {
	for _, m := range c.Monitors {
		key := fmt.Sprintf("%s/%s/%s/%s", m.Provider, m.Service, m.Channel, m.Model)
		if _, exists := ctx.quadrupleKeySet[key]; exists {
			return fmt.Errorf("重复的监测项: %s", key)
		}
		ctx.quadrupleKeySet[key] = struct{}{}
	}
	return nil
}

// buildAndValidateParentGraph 构建并校验父子关系图
// 包括：收集父通道引用、构建索引、验证父存在性
func (c *AppConfig) buildAndValidateParentGraph(ctx *validateContext) error {
	// 收集父通道引用
	parentRefs := make(map[string]struct{})
	for i, m := range c.Monitors {
		parentPath := strings.TrimSpace(m.Parent)
		if parentPath == "" {
			continue
		}

		// 子通道必须有 model
		if strings.TrimSpace(m.Model) == "" {
			return fmt.Errorf("monitor[%d]: 子通道 %s/%s/%s 有 parent 但缺少 model", i, m.Provider, m.Service, m.Channel)
		}

		parentRefs[parentPath] = struct{}{}
	}

	// 被引用为父的监测项必须有 model
	for i, m := range c.Monitors {
		path := fmt.Sprintf("%s/%s/%s", m.Provider, m.Service, m.Channel)
		if _, isReferencedAsParent := parentRefs[path]; isReferencedAsParent {
			if strings.TrimSpace(m.Model) == "" {
				return fmt.Errorf("monitor[%d]: 监测项 %s 被引用为父但缺少 model", i, path)
			}
		}
	}

	// 构建父通道索引（parent 为空的 monitor 定义）
	// 注意：ctx.rootByPath 已在 newValidateContext() 中初始化
	for i := range c.Monitors {
		if strings.TrimSpace(c.Monitors[i].Parent) != "" {
			continue
		}
		path := fmt.Sprintf("%s/%s/%s", c.Monitors[i].Provider, c.Monitors[i].Service, c.Monitors[i].Channel)
		if existing, exists := ctx.rootByPath[path]; exists {
			// 标记为多定义（nil 表示冲突）
			if existing != nil {
				ctx.rootByPath[path] = nil
			}
			continue
		}
		ctx.rootByPath[path] = &c.Monitors[i]
	}

	// 父存在性校验，并构建 parent 关系图（用于循环检测）
	// 注意：ctx.parentOf 已在 newValidateContext() 中初始化
	for i, m := range c.Monitors {
		parentPath := strings.TrimSpace(m.Parent)
		if parentPath == "" {
			continue
		}

		// 验证父存在且唯一
		parent := ctx.rootByPath[parentPath]
		if parent == nil {
			if _, pathExists := ctx.rootByPath[parentPath]; pathExists {
				return fmt.Errorf("monitor[%d]: 父通道 %s 存在多个定义", i, parentPath)
			}
			return fmt.Errorf("monitor[%d]: 找不到父通道: %s", i, parentPath)
		}

		// 构建父子关系图
		childKey := fmt.Sprintf("%s/%s/%s/%s", m.Provider, m.Service, m.Channel, m.Model)
		parentKey := fmt.Sprintf("%s/%s/%s/%s", parent.Provider, parent.Service, parent.Channel, parent.Model)
		ctx.parentOf[childKey] = parentKey
	}

	return nil
}

// validateNoCycles 检测循环引用（DFS 颜色标记：0=白, 1=灰, 2=黑）
func (c *AppConfig) validateNoCycles(ctx *validateContext) error {
	color := make(map[string]int)
	var dfsCheckCycle func(key string) error
	dfsCheckCycle = func(key string) error {
		switch color[key] {
		case 1:
			return fmt.Errorf("检测到循环引用: %s", key)
		case 2:
			return nil
		}

		color[key] = 1 // 标记为灰色（访问中）

		if parentKey, hasParent := ctx.parentOf[key]; hasParent {
			if err := dfsCheckCycle(parentKey); err != nil {
				return err
			}
		}

		color[key] = 2 // 标记为黑色（已完成）
		return nil
	}

	for key := range ctx.quadrupleKeySet {
		if color[key] == 0 {
			if err := dfsCheckCycle(key); err != nil {
				return err
			}
		}
	}

	return nil
}

// warnMultipleParentLayers 警告同一 PSC 下存在多个父层
// 只有第一个会被视为父层，其他会从 API 输出中丢失
func (c *AppConfig) warnMultipleParentLayers() {
	pscNoParentCount := make(map[string]int)
	for _, m := range c.Monitors {
		if strings.TrimSpace(m.Parent) == "" && strings.TrimSpace(m.Model) != "" {
			psc := fmt.Sprintf("%s/%s/%s", m.Provider, m.Service, m.Channel)
			pscNoParentCount[psc]++
		}
	}

	// 收集需要警告的 PSC 并排序，保证输出稳定性
	var warnings []string
	for psc, count := range pscNoParentCount {
		if count > 1 {
			warnings = append(warnings, psc)
		}
	}
	sort.Strings(warnings)

	for _, psc := range warnings {
		logger.Warn("config", "同一 PSC 下存在多个父层 (Parent='', Model!='')，只有第一个会作为父层，其他会丢失",
			"psc", psc, "count", pscNoParentCount[psc])
	}
}

// validateMonitorFields 校验监测项的必填字段和字段合法性
func (c *AppConfig) validateMonitorFields() error {
	for i, m := range c.Monitors {
		hasParent := strings.TrimSpace(m.Parent) != ""

		// 基础必填字段（provider/service/channel 已在预处理步骤处理）
		if m.Provider == "" {
			return fmt.Errorf("monitor[%d]: provider 不能为空", i)
		}
		if m.Service == "" {
			return fmt.Errorf("monitor[%d]: service 不能为空", i)
		}

		// Category: 非子通道必填（子通道可以继承）
		if !hasParent && m.Category == "" {
			return fmt.Errorf("monitor[%d]: category 不能为空（必须是 commercial 或 public）", i)
		}

		// URL 和 Method 对于非子通道是必填的（子通道可以继承）
		if !hasParent && m.URL == "" {
			return fmt.Errorf("monitor[%d]: URL 不能为空", i)
		}
		if !hasParent && m.Method == "" {
			return fmt.Errorf("monitor[%d]: method 不能为空", i)
		}

		// Method 枚举检查（子通道允许留空继承）
		if m.Method != "" {
			validMethods := map[string]bool{"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true}
			if !validMethods[strings.ToUpper(m.Method)] {
				return fmt.Errorf("monitor[%d]: method '%s' 无效，必须是 GET/POST/PUT/DELETE/PATCH 之一", i, m.Method)
			}
		}

		// Category 枚举检查（子通道允许留空继承）
		if m.Category != "" && !isValidCategory(m.Category) {
			return fmt.Errorf("monitor[%d]: category '%s' 无效，必须是 commercial 或 public", i, m.Category)
		}

		// SponsorLevel 枚举检查（可选字段，空值有效）
		if !m.SponsorLevel.IsValid() {
			return fmt.Errorf("monitor[%d]: sponsor_level '%s' 无效，必须是 basic/advanced/enterprise 之一（或留空）", i, m.SponsorLevel)
		}

		// Board 枚举检查（可选字段，空值视为 hot）
		normalizedBoard := strings.ToLower(strings.TrimSpace(m.Board))
		switch normalizedBoard {
		case "", "hot", "secondary", "cold":
			// 有效值
		default:
			return fmt.Errorf("monitor[%d]: board '%s' 无效，必须是 hot/secondary/cold（或留空）", i, m.Board)
		}
		// 注意：cold_reason 的有效性检查在 Normalize() 中进行（非致命，仅警告并清空）

		// PriceMin/PriceMax 验证（可选字段）
		if m.PriceMin != nil && *m.PriceMin < 0 {
			return fmt.Errorf("monitor[%d]: price_min 不能为负数", i)
		}
		if m.PriceMax != nil && *m.PriceMax < 0 {
			return fmt.Errorf("monitor[%d]: price_max 不能为负数", i)
		}
		// 若同时配置了 min 和 max，min 必须 <= max
		if m.PriceMin != nil && m.PriceMax != nil && *m.PriceMin > *m.PriceMax {
			return fmt.Errorf("monitor[%d]: price_min 不能大于 price_max", i)
		}

		// ListedSince 验证（可选字段，格式必须为 "2006-01-02"）
		if m.ListedSince != "" {
			if _, err := time.Parse("2006-01-02", m.ListedSince); err != nil {
				return fmt.Errorf("monitor[%d]: listed_since 格式错误，应为 YYYY-MM-DD", i)
			}
		}

		// ProviderURL 验证（可选字段）
		if m.ProviderURL != "" {
			if err := validateURL(m.ProviderURL, "provider_url"); err != nil {
				return fmt.Errorf("monitor[%d]: %w", i, err)
			}
		}

		// SponsorURL 验证（可选字段）
		if m.SponsorURL != "" {
			if err := validateURL(m.SponsorURL, "sponsor_url"); err != nil {
				return fmt.Errorf("monitor[%d]: %w", i, err)
			}
		}

		// Proxy 验证（可选字段）
		if trimmedProxy := strings.TrimSpace(m.Proxy); trimmedProxy != "" {
			if err := validateProxyURL(trimmedProxy); err != nil {
				return fmt.Errorf("monitor[%d]: %w", i, err)
			}
		}
	}
	return nil
}

// validateProviderConfigs 校验 Provider 相关配置
func (c *AppConfig) validateProviderConfigs() error {
	// 验证 disabled_providers
	disabledProviderSet := make(map[string]struct{})
	for i, dp := range c.DisabledProviders {
		provider := strings.ToLower(strings.TrimSpace(dp.Provider))
		if provider == "" {
			return fmt.Errorf("disabled_providers[%d]: provider 不能为空", i)
		}
		if _, exists := disabledProviderSet[provider]; exists {
			return fmt.Errorf("disabled_providers[%d]: provider '%s' 重复配置", i, dp.Provider)
		}
		disabledProviderSet[provider] = struct{}{}
	}

	// 验证 risk_providers
	for i, rp := range c.RiskProviders {
		if strings.TrimSpace(rp.Provider) == "" {
			return fmt.Errorf("risk_providers[%d]: provider 不能为空", i)
		}
		if len(rp.Risks) == 0 {
			return fmt.Errorf("risk_providers[%d]: risks 不能为空", i)
		}
		for j, risk := range rp.Risks {
			if strings.TrimSpace(risk.Label) == "" {
				return fmt.Errorf("risk_providers[%d].risks[%d]: label 不能为空", i, j)
			}
			if risk.DiscussionURL != "" {
				if err := validateURL(risk.DiscussionURL, "discussion_url"); err != nil {
					return fmt.Errorf("risk_providers[%d].risks[%d]: %w", i, j, err)
				}
			}
		}
	}

	return nil
}

// validateBadgeConfigs 校验 Badge 相关配置
func (c *AppConfig) validateBadgeConfigs() error {
	// 验证 badges（全局徽标定义）
	for id, bd := range c.BadgeDefs {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("badge_definitions: id 不能为空")
		}

		// 允许空值通过校验（Normalize() 会填充默认值）
		if bd.Kind != "" && !bd.Kind.IsValid() {
			return fmt.Errorf("badge_definitions[%s]: kind '%s' 无效，必须是 source/info/feature", id, bd.Kind)
		}
		if bd.Variant != "" && !bd.Variant.IsValid() {
			return fmt.Errorf("badge_definitions[%s]: variant '%s' 无效，必须是 default/success/warning/danger/info", id, bd.Variant)
		}
		if bd.Weight < 0 || bd.Weight > 100 {
			return fmt.Errorf("badge_definitions[%s]: weight 必须在 0-100 范围内", id)
		}
		if bd.URL != "" {
			if err := validateURL(bd.URL, "url"); err != nil {
				return fmt.Errorf("badge_definitions[%s]: %w", id, err)
			}
		}
	}

	// 验证 badge_providers
	badgeProviderSet := make(map[string]struct{})
	for i, bp := range c.BadgeProviders {
		provider := strings.ToLower(strings.TrimSpace(bp.Provider))
		if provider == "" {
			return fmt.Errorf("badge_providers[%d]: provider 不能为空", i)
		}
		if _, exists := badgeProviderSet[provider]; exists {
			return fmt.Errorf("badge_providers[%d]: provider '%s' 重复配置", i, bp.Provider)
		}
		badgeProviderSet[provider] = struct{}{}
		for j, ref := range bp.Badges {
			refID := strings.TrimSpace(ref.ID)
			if refID == "" {
				return fmt.Errorf("badge_providers[%d].badges[%d]: id 不能为空", i, j)
			}
			// 检查用户配置和内置默认徽标
			_, inUserDefs := c.BadgeDefs[refID]
			_, inDefaultDefs := defaultBadgeDefs[refID]
			if !inUserDefs && !inDefaultDefs {
				return fmt.Errorf("badge_providers[%d].badges[%d]: 未找到徽标定义 '%s'", i, j, refID)
			}
		}
	}

	// 验证 monitors[].badges
	for i, m := range c.Monitors {
		for j, ref := range m.Badges {
			refID := strings.TrimSpace(ref.ID)
			if refID == "" {
				return fmt.Errorf("monitors[%d].badges[%d]: id 不能为空", i, j)
			}
			// 检查用户配置和内置默认徽标
			_, inUserDefs := c.BadgeDefs[refID]
			_, inDefaultDefs := defaultBadgeDefs[refID]
			if !inUserDefs && !inDefaultDefs {
				return fmt.Errorf("monitors[%d].badges[%d]: 未找到徽标定义 '%s'", i, j, refID)
			}
		}
	}

	return nil
}
