package config

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"monitor/internal/logger"
)

// AppConfig 应用配置
type AppConfig struct {
	// 巡检间隔（支持 Go duration 格式，例如 "30s"、"1m", "5m"）
	Interval string `yaml:"interval" json:"interval"`

	// 解析后的巡检间隔（内部使用，不序列化）
	IntervalDuration time.Duration `yaml:"-" json:"-"`

	// 慢请求阈值（超过则从绿降为黄），支持 Go duration 格式，例如 "5s"、"3s"
	SlowLatency string `yaml:"slow_latency" json:"slow_latency"`

	// 解析后的慢请求阈值（内部使用，不序列化）
	SlowLatencyDuration time.Duration `yaml:"-" json:"-"`

	// 按服务类型覆盖的慢请求阈值（可选，支持 Go duration 格式）
	// 例如 cc: "15s", gm: "3s"
	SlowLatencyByService map[string]string `yaml:"slow_latency_by_service" json:"slow_latency_by_service"`

	// 解析后的按服务慢请求阈值（内部使用，不序列化）
	SlowLatencyByServiceDuration map[string]time.Duration `yaml:"-" json:"-"`

	// 请求超时时间（支持 Go duration 格式，例如 "10s"、"30s"，默认 "10s"）
	Timeout string `yaml:"timeout" json:"timeout"`

	// 解析后的超时时间（内部使用，不序列化）
	TimeoutDuration time.Duration `yaml:"-" json:"-"`

	// 按服务类型覆盖的超时时间（可选，支持 Go duration 格式）
	// 例如 cc: "30s", gm: "10s"
	TimeoutByService map[string]string `yaml:"timeout_by_service" json:"timeout_by_service"`

	// 解析后的按服务超时时间（内部使用，不序列化）
	TimeoutByServiceDuration map[string]time.Duration `yaml:"-" json:"-"`

	// 可用率中黄色状态的权重（0-1，默认 0.7）
	// 绿色=1.0, 黄色=degraded_weight, 红色=0.0
	DegradedWeight float64 `yaml:"degraded_weight" json:"degraded_weight"`

	// 并发探测的最大 goroutine 数（默认 10）
	// - 不配置或 0: 使用默认值 10
	// - -1: 无限制，自动扩容到监测项数量
	// - >0: 硬上限，超过时监测项会排队等待执行
	MaxConcurrency int `yaml:"max_concurrency" json:"max_concurrency"`

	// 是否在单个周期内对探测进行错峰（默认 true）
	// 开启后会将监测项均匀分散在整个巡检周期内，避免流量突发
	StaggerProbes *bool `yaml:"stagger_probes,omitempty" json:"stagger_probes,omitempty"`

	// 是否启用并发查询（API 层优化，默认 false）
	// 开启后 /api/status 接口会使用 goroutine 并发查询多个监测项，显著降低响应时间
	// 注意：需要确保数据库连接池足够大（建议 max_open_conns >= 50）
	EnableConcurrentQuery bool `yaml:"enable_concurrent_query" json:"enable_concurrent_query"`

	// 并发查询时的最大并发度（默认 10，仅当 enable_concurrent_query=true 时生效）
	// 限制同时执行的数据库查询数量，防止连接池耗尽
	ConcurrentQueryLimit int `yaml:"concurrent_query_limit" json:"concurrent_query_limit"`

	// 是否启用批量查询（API 层优化，默认 false）
	// 开启后 /api/status 在 7d/30d 场景会优先使用批量查询，将 N 个监测项的 GetLatest+GetHistory 从 2N 次往返降为 2 次
	EnableBatchQuery bool `yaml:"enable_batch_query" json:"enable_batch_query"`

	// 是否启用 DB 侧时间轴聚合（默认 false）
	// 仅对 PostgreSQL 生效：将 7d/30d 的 timeline bucket 聚合下推到数据库，减少数据传输与应用层计算
	// 需要同时启用 enable_batch_query=true 才能生效
	EnableDBTimelineAgg bool `yaml:"enable_db_timeline_agg" json:"enable_db_timeline_agg"`

	// 批量查询最大 key 数（默认 300）
	// 注意：SQLite 场景下会自动回退到 249（因为参数上限 999，每 key 需要 4 个参数）
	BatchQueryMaxKeys int `yaml:"batch_query_max_keys" json:"batch_query_max_keys"`

	// API 响应缓存 TTL 配置（按 period 区分）
	// 默认值：90m/24h = 10s，7d/30d = 60s
	CacheTTL CacheTTLConfig `yaml:"cache_ttl" json:"cache_ttl"`

	// 存储配置
	Storage StorageConfig `yaml:"storage" json:"storage"`

	// 公开访问的基础 URL（用于 SEO、sitemap 等）
	// 默认: https://relaypulse.top
	// 可通过环境变量 MONITOR_PUBLIC_BASE_URL 覆盖
	PublicBaseURL string `yaml:"public_base_url" json:"public_base_url"`

	// 批量禁用的服务商列表（彻底停用，不探测、不存储、不展示）
	// 列表中的 provider 会自动继承 disabled=true 状态到对应的 monitors
	DisabledProviders []DisabledProviderConfig `yaml:"disabled_providers" json:"disabled_providers"`

	// 批量隐藏的服务商列表
	// 列表中的 provider 会自动继承 hidden=true 状态到对应的 monitors
	// 用于临时下架整个服务商（如商家不配合整改）
	HiddenProviders []HiddenProviderConfig `yaml:"hidden_providers" json:"hidden_providers"`

	// 风险服务商列表
	// 列表中的 provider 会自动继承 risks 到对应的所有 monitors
	// 用于标记存在风险的服务商（如跑路风险）
	RiskProviders []RiskProviderConfig `yaml:"risk_providers" json:"risk_providers"`

	// 热板/冷板功能配置（默认禁用，保持向后兼容）
	// 启用后可通过 monitor.board 字段控制监测项归属
	Boards BoardsConfig `yaml:"boards" json:"boards"`

	// 是否启用徽标系统（默认 false）
	// 开启后会显示 API Key 来源、监测频率等徽标
	// 未配置任何徽标时，默认显示"官方 API Key"徽标
	EnableBadges bool `yaml:"enable_badges" json:"enable_badges"`

	// 全局徽标定义（map 格式，key 为徽标 ID）
	// Label 和 Tooltip 由前端 i18n 提供，后端只存储 id/kind/variant/weight/url
	BadgeDefs map[string]BadgeDef `yaml:"badge_definitions" json:"badge_definitions"`

	// provider 级徽标注入配置
	// 列表中的 provider 会自动继承 badges 到对应的所有 monitors
	BadgeProviders []BadgeProviderConfig `yaml:"badge_providers" json:"badge_providers"`

	// 赞助商置顶配置
	// 用于在页面初始加载时置顶符合条件的赞助商监测项
	SponsorPin SponsorPinConfig `yaml:"sponsor_pin" json:"sponsor_pin"`

	// 自助测试功能配置
	SelfTest SelfTestConfig `yaml:"selftest" json:"selftest"`

	// 状态订阅通知（事件）配置
	Events EventsConfig `yaml:"events" json:"events"`

	// 公告通知配置（GitHub Discussions / Announcements 分类）
	Announcements AnnouncementsConfig `yaml:"announcements" json:"announcements"`

	// GitHub 通用配置（token/proxy/timeout）
	GitHub GitHubConfig `yaml:"github" json:"github"`

	Monitors []ServiceConfig `yaml:"monitors"`
}

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
		case "", "hot", "cold":
			// 有效值
		default:
			return fmt.Errorf("monitor[%d]: board '%s' 无效，必须是 hot/cold（或留空）", i, m.Board)
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

	// 6. 规范化每个监测项
	if err := c.normalizeMonitors(ctx); err != nil {
		return err
	}

	// 7. 父子继承（必须在 per-monitor 规范化之后，因为继承依赖已规范化的路径/键）
	if err := c.applyParentInheritance(); err != nil {
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

// normalizeMonitors 规范化每个监测项
// 包括：时间配置下发、字段规范化、Provider 状态注入、徽标解析
func (c *AppConfig) normalizeMonitors(ctx *normalizeContext) error {
	// 将慢请求阈值和超时时间下发到每个监测项（优先级：monitor > by_service > global），并标准化 category、URLs、provider_slug
	for i := range c.Monitors {
		// 注意：以下 yaml:"-" 字段在热更新/复用 slice 元素的场景下，旧值可能残留。
		// 每次 Normalize 都从零值开始重新计算，确保派生逻辑稳定。
		c.Monitors[i].SlowLatencyDuration = 0
		c.Monitors[i].TimeoutDuration = 0
		c.Monitors[i].IntervalDuration = 0
		c.Monitors[i].Risks = nil          // 由 ctx.riskProviderMap 重新注入
		c.Monitors[i].ResolvedBadges = nil // 由徽标解析逻辑重新计算

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

		// 规范化 board：空值视为 hot
		c.Monitors[i].Board = strings.ToLower(strings.TrimSpace(c.Monitors[i].Board))
		if c.Monitors[i].Board == "" {
			c.Monitors[i].Board = "hot"
		}
		c.Monitors[i].ColdReason = strings.TrimSpace(c.Monitors[i].ColdReason)

		// cold_reason 仅在 board=cold 时有意义，其他情况清空并警告
		if c.Monitors[i].ColdReason != "" && c.Monitors[i].Board != "cold" {
			logger.Warn("config", "cold_reason 仅在 board=cold 时有效，已忽略",
				"monitor_index", i,
				"provider", c.Monitors[i].Provider,
				"service", c.Monitors[i].Service)
			c.Monitors[i].ColdReason = ""
		}

		// 标准化 category 为小写（与 Validate 的 isValidCategory 保持一致）
		c.Monitors[i].Category = strings.ToLower(strings.TrimSpace(c.Monitors[i].Category))

		// 规范化 URLs：去除首尾空格和末尾的 /
		c.Monitors[i].ProviderURL = strings.TrimRight(strings.TrimSpace(c.Monitors[i].ProviderURL), "/")
		c.Monitors[i].SponsorURL = strings.TrimRight(strings.TrimSpace(c.Monitors[i].SponsorURL), "/")

		// provider_slug 验证和自动生成
		slug := strings.TrimSpace(c.Monitors[i].ProviderSlug)
		if slug == "" {
			// 未配置时，自动生成: provider 转小写
			slug = strings.ToLower(strings.TrimSpace(c.Monitors[i].Provider))
		}

		// 无论自动生成还是手动配置，都进行格式验证
		// 确保配置期即可发现 slug 格式问题，避免运行时 404
		if err := validateProviderSlug(slug); err != nil {
			return fmt.Errorf("monitor[%d]: provider_slug '%s' 无效 (来源: %s): %w",
				i, slug,
				map[bool]string{true: "自动生成", false: "手动配置"}[c.Monitors[i].ProviderSlug == ""],
				err)
		}

		c.Monitors[i].ProviderSlug = slug

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

		// 从 badge_providers + monitors[].badges 注入徽标
		// 合并策略：provider 级在前，monitor 级在后（同 id 时 monitor 级覆盖）
		// 仅在启用徽标系统时处理
		if c.EnableBadges {
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

		// ===== 完整继承逻辑 =====
		// 设计原则：子通道只需配置 parent 即可继承父通道所有配置，
		// 仅需在子通道中覆盖有差异的字段

		// --- 监测行为配置（核心继承）---
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
		}
		if child.SuccessContains == "" {
			child.SuccessContains = parent.SuccessContains
		}
		// 自定义环境变量名（用于 API Key 查找）
		if child.EnvVarName == "" {
			child.EnvVarName = parent.EnvVarName
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

		// --- 时间/阈值配置 ---
		// 标记哪些字段是从 parent 继承的（用于后续重新计算 Duration）
		// 注意：使用 TrimSpace 判空，与 Validate() 保持一致
		inheritedSlowLatency := false
		inheritedTimeout := false
		inheritedInterval := false

		// SlowLatency: 字符串形式，空值表示未配置
		if strings.TrimSpace(child.SlowLatency) == "" && strings.TrimSpace(parent.SlowLatency) != "" {
			child.SlowLatency = parent.SlowLatency
			inheritedSlowLatency = true
		}
		// Timeout: 字符串形式，空值表示未配置
		if strings.TrimSpace(child.Timeout) == "" && strings.TrimSpace(parent.Timeout) != "" {
			child.Timeout = parent.Timeout
			inheritedTimeout = true
		}
		// Interval: 字符串形式，空值表示未配置
		if strings.TrimSpace(child.Interval) == "" && strings.TrimSpace(parent.Interval) != "" {
			child.Interval = parent.Interval
			inheritedInterval = true
		}

		// --- 元数据配置 ---
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

		// --- 状态配置（级联 OR 逻辑）---
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

		// --- 徽标配置 ---
		// Badges: 子为空时继承父的徽标（替换策略，非合并）
		if len(child.Badges) == 0 && len(parent.Badges) > 0 {
			child.Badges = make([]BadgeRef, len(parent.Badges))
			copy(child.Badges, parent.Badges)
		}

		// --- 显示名称继承（子为空时继承） ---
		if child.ChannelName == "" {
			child.ChannelName = parent.ChannelName
		}

		// --- 定价信息继承（子为 nil 时继承） ---
		if child.PriceMin == nil && parent.PriceMin != nil {
			v := *parent.PriceMin
			child.PriceMin = &v
		}
		if child.PriceMax == nil && parent.PriceMax != nil {
			v := *parent.PriceMax
			child.PriceMax = &v
		}

		// --- 收录日期继承（子为空时继承） ---
		if child.ListedSince == "" {
			child.ListedSince = parent.ListedSince
		}

		// ===== Duration 字段修复 =====
		// Validate() 中 Duration 解析发生在 applyParentInheritance() 之前，
		// 当子通道通过 parent 继承了字符串字段（Interval/SlowLatency/Timeout）时，
		// 需要在此重新计算对应的 Duration 字段。
		if inheritedInterval {
			trimmed := strings.TrimSpace(child.Interval)
			d, err := time.ParseDuration(trimmed)
			if err != nil {
				return fmt.Errorf("monitor[%d]: 解析继承的 interval 失败: %w", i, err)
			}
			if d <= 0 {
				return fmt.Errorf("monitor[%d]: 继承的 interval 必须大于 0", i)
			}
			child.IntervalDuration = d
		}

		if inheritedSlowLatency {
			trimmed := strings.TrimSpace(child.SlowLatency)
			d, err := time.ParseDuration(trimmed)
			if err != nil {
				return fmt.Errorf("monitor[%d] (provider=%s, service=%s, channel=%s): 解析继承的 slow_latency 失败: %w",
					i, child.Provider, child.Service, child.Channel, err)
			}
			if d <= 0 {
				return fmt.Errorf("monitor[%d] (provider=%s, service=%s, channel=%s): 继承的 slow_latency 必须大于 0",
					i, child.Provider, child.Service, child.Channel)
			}
			child.SlowLatencyDuration = d
		}

		if inheritedTimeout {
			trimmed := strings.TrimSpace(child.Timeout)
			d, err := time.ParseDuration(trimmed)
			if err != nil {
				return fmt.Errorf("monitor[%d] (provider=%s, service=%s, channel=%s): 解析继承的 timeout 失败: %w",
					i, child.Provider, child.Service, child.Channel, err)
			}
			if d <= 0 {
				return fmt.Errorf("monitor[%d] (provider=%s, service=%s, channel=%s): 继承的 timeout 必须大于 0",
					i, child.Provider, child.Service, child.Channel)
			}
			child.TimeoutDuration = d
		}

		// 继承后重新检查：slow_latency >= timeout 时黄灯基本不会触发
		if (inheritedSlowLatency || inheritedTimeout) &&
			child.SlowLatencyDuration >= child.TimeoutDuration {
			logger.Warn("config", "slow_latency >= timeout，慢响应黄灯可能不会触发（继承自 parent）",
				"monitor_index", i,
				"provider", child.Provider,
				"service", child.Service,
				"channel", child.Channel,
				"model", child.Model,
				"slow_latency", child.SlowLatencyDuration,
				"timeout", child.TimeoutDuration)
		}

		// 注意：以下字段不继承（有特殊约束）：
		// - Model: 父子关系的唯一区分字段，若继承则变成重复项
		// - Provider/Service/Channel: 由父子路径验证强制一致
	}

	return nil
}

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

	clone := &AppConfig{
		Interval:                     c.Interval,
		IntervalDuration:             c.IntervalDuration,
		SlowLatency:                  c.SlowLatency,
		SlowLatencyDuration:          c.SlowLatencyDuration,
		SlowLatencyByService:         make(map[string]string, len(c.SlowLatencyByService)),
		SlowLatencyByServiceDuration: make(map[string]time.Duration, len(c.SlowLatencyByServiceDuration)),
		DegradedWeight:               c.DegradedWeight,
		MaxConcurrency:               c.MaxConcurrency,
		StaggerProbes:                staggerPtr,
		EnableConcurrentQuery:        c.EnableConcurrentQuery,
		ConcurrentQueryLimit:         c.ConcurrentQueryLimit,
		EnableBatchQuery:             c.EnableBatchQuery,
		EnableDBTimelineAgg:          c.EnableDBTimelineAgg,
		BatchQueryMaxKeys:            c.BatchQueryMaxKeys,
		CacheTTL:                     c.CacheTTL, // CacheTTL 是值类型，直接复制
		Storage:                      c.Storage,
		PublicBaseURL:                c.PublicBaseURL,
		DisabledProviders:            make([]DisabledProviderConfig, len(c.DisabledProviders)),
		HiddenProviders:              make([]HiddenProviderConfig, len(c.HiddenProviders)),
		RiskProviders:                make([]RiskProviderConfig, len(c.RiskProviders)),
		Boards:                       c.Boards, // Boards 是值类型，直接复制
		EnableBadges:                 c.EnableBadges,
		BadgeDefs:                    make(map[string]BadgeDef, len(c.BadgeDefs)),
		BadgeProviders:               make([]BadgeProviderConfig, len(c.BadgeProviders)),
		SponsorPin: SponsorPinConfig{
			Enabled:      sponsorPinEnabledPtr,
			MaxPinned:    c.SponsorPin.MaxPinned,
			ServiceCount: c.SponsorPin.ServiceCount,
			MinUptime:    c.SponsorPin.MinUptime,
			MinLevel:     c.SponsorPin.MinLevel,
		},
		SelfTest: c.SelfTest, // SelfTest 是值类型，直接复制
		Events:   c.Events,   // Events 是值类型，直接复制
		Monitors: make([]ServiceConfig, len(c.Monitors)),
	}
	copy(clone.DisabledProviders, c.DisabledProviders)
	copy(clone.HiddenProviders, c.HiddenProviders)
	copy(clone.RiskProviders, c.RiskProviders)
	for k, v := range c.SlowLatencyByService {
		clone.SlowLatencyByService[k] = v
	}
	for k, v := range c.SlowLatencyByServiceDuration {
		clone.SlowLatencyByServiceDuration[k] = v
	}
	for id, bd := range c.BadgeDefs {
		clone.BadgeDefs[id] = bd
	}
	copy(clone.BadgeProviders, c.BadgeProviders)
	copy(clone.Monitors, c.Monitors)

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

	// 深拷贝 monitors 中的 slice/map 字段
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
	}

	return clone
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
