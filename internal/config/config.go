package config

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SponsorLevel 赞助商等级
type SponsorLevel string

const (
	SponsorLevelNone       SponsorLevel = ""           // 无赞助徽章
	SponsorLevelBasic      SponsorLevel = "basic"      // 基础赞助（三角形）
	SponsorLevelAdvanced   SponsorLevel = "advanced"   // 进阶赞助（六边形）
	SponsorLevelEnterprise SponsorLevel = "enterprise" // 企业赞助（八边形钻石）
)

// IsValid 检查赞助商等级是否有效
func (s SponsorLevel) IsValid() bool {
	switch s {
	case SponsorLevelNone, SponsorLevelBasic, SponsorLevelAdvanced, SponsorLevelEnterprise:
		return true
	default:
		return false
	}
}

// ServiceConfig 单个服务监控配置
type ServiceConfig struct {
	Provider     string            `yaml:"provider" json:"provider"`
	ProviderSlug string            `yaml:"provider_slug" json:"provider_slug"` // URL slug（可选，未配置时使用 provider 小写）
	ProviderURL  string            `yaml:"provider_url" json:"provider_url"`   // 服务商官网链接（可选）
	Service      string            `yaml:"service" json:"service"`
	Category     string            `yaml:"category" json:"category"`             // 分类：commercial（商业站）或 public（公益站）
	Sponsor      string            `yaml:"sponsor" json:"sponsor"`               // 赞助者：提供 API Key 的个人或组织
	SponsorURL   string            `yaml:"sponsor_url" json:"sponsor_url"`       // 赞助者链接（可选）
	SponsorLevel SponsorLevel      `yaml:"sponsor_level" json:"sponsor_level"`   // 赞助商等级：basic/advanced/enterprise（可选）
	Channel      string            `yaml:"channel" json:"channel"`               // 业务通道标识（如 "vip-channel"、"standard-channel"），用于分类和过滤
	URL          string            `yaml:"url" json:"url"`
	Method       string            `yaml:"method" json:"method"`
	Headers      map[string]string `yaml:"headers" json:"headers"`
	Body         string            `yaml:"body" json:"body"`

	// SuccessContains 可选：响应体需包含的关键字，用于判定请求语义是否成功
	SuccessContains string `yaml:"success_contains" json:"success_contains"`

	// 临时下架配置：隐藏但继续探测，用于商家整改期间
	// Hidden 为 true 时，API 不返回该监控项，但调度器继续探测并存储结果
	Hidden       bool   `yaml:"hidden" json:"hidden"`
	HiddenReason string `yaml:"hidden_reason" json:"hidden_reason"` // 下架原因（可选）

	// 解析后的"慢请求"阈值（来自全局配置），用于黄灯判定
	SlowLatencyDuration time.Duration `yaml:"-" json:"-"`

	APIKey string `yaml:"api_key" json:"-"` // 不返回给前端
}

// HiddenProviderConfig 批量隐藏指定 provider 的配置
// 用于临时下架某个服务商的所有监控项
type HiddenProviderConfig struct {
	Provider string `yaml:"provider" json:"provider"` // provider 名称，需与 monitors 中的 provider 完全匹配
	Reason   string `yaml:"reason" json:"reason"`     // 下架原因（可选）
}

// StorageConfig 存储配置
type StorageConfig struct {
	Type string `yaml:"type" json:"type"` // "sqlite" 或 "postgres"

	// SQLite 配置
	SQLite SQLiteConfig `yaml:"sqlite" json:"sqlite"`

	// PostgreSQL 配置
	Postgres PostgresConfig `yaml:"postgres" json:"postgres"`
}

// SQLiteConfig SQLite 配置
type SQLiteConfig struct {
	Path string `yaml:"path" json:"path"` // 数据库文件路径
}

// PostgresConfig PostgreSQL 配置
type PostgresConfig struct {
	Host            string `yaml:"host" json:"host"`
	Port            int    `yaml:"port" json:"port"`
	User            string `yaml:"user" json:"user"`
	Password        string `yaml:"password" json:"-"` // 不输出到 JSON
	Database        string `yaml:"database" json:"database"`
	SSLMode         string `yaml:"sslmode" json:"sslmode"`
	MaxOpenConns    int    `yaml:"max_open_conns" json:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns" json:"max_idle_conns"`
	ConnMaxLifetime string `yaml:"conn_max_lifetime" json:"conn_max_lifetime"`
}

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

	// 可用率中黄色状态的权重（0-1，默认 0.7）
	// 绿色=1.0, 黄色=degraded_weight, 红色=0.0
	DegradedWeight float64 `yaml:"degraded_weight" json:"degraded_weight"`

	// 并发探测的最大 goroutine 数（默认 10）
	// - 不配置或 0: 使用默认值 10
	// - -1: 无限制，自动扩容到监控项数量
	// - >0: 硬上限，超过时监控项会排队等待执行
	MaxConcurrency int `yaml:"max_concurrency" json:"max_concurrency"`

	// 是否在单个周期内对探测进行错峰（默认 true）
	// 开启后会将监控项均匀分散在整个巡检周期内，避免流量突发
	StaggerProbes *bool `yaml:"stagger_probes,omitempty" json:"stagger_probes,omitempty"`

	// 是否启用并发查询（API 层优化，默认 false）
	// 开启后 /api/status 接口会使用 goroutine 并发查询多个监控项，显著降低响应时间
	// 注意：需要确保数据库连接池足够大（建议 max_open_conns >= 50）
	EnableConcurrentQuery bool `yaml:"enable_concurrent_query" json:"enable_concurrent_query"`

	// 并发查询时的最大并发度（默认 10，仅当 enable_concurrent_query=true 时生效）
	// 限制同时执行的数据库查询数量，防止连接池耗尽
	ConcurrentQueryLimit int `yaml:"concurrent_query_limit" json:"concurrent_query_limit"`

	// 存储配置
	Storage StorageConfig `yaml:"storage" json:"storage"`

	// 公开访问的基础 URL（用于 SEO、sitemap 等）
	// 默认: https://relaypulse.top
	// 可通过环境变量 MONITOR_PUBLIC_BASE_URL 覆盖
	PublicBaseURL string `yaml:"public_base_url" json:"public_base_url"`

	// 批量隐藏的服务商列表
	// 列表中的 provider 会自动继承 hidden=true 状态到对应的 monitors
	// 用于临时下架整个服务商（如商家不配合整改）
	HiddenProviders []HiddenProviderConfig `yaml:"hidden_providers" json:"hidden_providers"`

	Monitors []ServiceConfig `yaml:"monitors"`
}

// Validate 验证配置合法性
func (c *AppConfig) Validate() error {
	if len(c.Monitors) == 0 {
		return fmt.Errorf("至少需要配置一个监控项")
	}

	// 检查重复和必填字段
	seen := make(map[string]bool)
	for i, m := range c.Monitors {
		// 必填字段检查
		if m.Provider == "" {
			return fmt.Errorf("monitor[%d]: provider 不能为空", i)
		}
		if m.Service == "" {
			return fmt.Errorf("monitor[%d]: service 不能为空", i)
		}
		if m.URL == "" {
			return fmt.Errorf("monitor[%d]: URL 不能为空", i)
		}
		if m.Method == "" {
			return fmt.Errorf("monitor[%d]: method 不能为空", i)
		}
		if m.Category == "" {
			return fmt.Errorf("monitor[%d]: category 不能为空（必须是 commercial 或 public）", i)
		}
		if strings.TrimSpace(m.Sponsor) == "" {
			return fmt.Errorf("monitor[%d]: sponsor 不能为空", i)
		}

		// Method 枚举检查
		validMethods := map[string]bool{"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true}
		if !validMethods[strings.ToUpper(m.Method)] {
			return fmt.Errorf("monitor[%d]: method '%s' 无效，必须是 GET/POST/PUT/DELETE/PATCH 之一", i, m.Method)
		}

		// Category 枚举检查
		if !isValidCategory(m.Category) {
			return fmt.Errorf("monitor[%d]: category '%s' 无效，必须是 commercial 或 public", i, m.Category)
		}

		// SponsorLevel 枚举检查（可选字段，空值有效）
		if !m.SponsorLevel.IsValid() {
			return fmt.Errorf("monitor[%d]: sponsor_level '%s' 无效，必须是 basic/advanced/enterprise 之一（或留空）", i, m.SponsorLevel)
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

		// 唯一性检查（provider + service + channel 组合唯一）
		key := m.Provider + "/" + m.Service + "/" + m.Channel
		if seen[key] {
			return fmt.Errorf("重复的监控项: provider=%s, service=%s, channel=%s", m.Provider, m.Service, m.Channel)
		}
		seen[key] = true
	}

	return nil
}

// Normalize 规范化配置（填充默认值等）
func (c *AppConfig) Normalize() error {
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
	// - -1：无限制（自动扩容到监控项数量）
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

	// 存储配置默认值
	if c.Storage.Type == "" {
		c.Storage.Type = "sqlite" // 默认使用 SQLite
	}
	if c.Storage.Type == "sqlite" && c.Storage.SQLite.Path == "" {
		c.Storage.SQLite.Path = "monitor.db" // 默认路径
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
				log.Printf("[Config] 警告: max_open_conns(%d) < concurrent_query_limit(%d)，可能导致连接池等待",
					c.Storage.Postgres.MaxOpenConns, c.ConcurrentQueryLimit)
			}
		}
	}

	// SQLite 场景下的并发查询警告
	if c.Storage.Type == "sqlite" && c.EnableConcurrentQuery {
		log.Println("[Config] 警告: SQLite 使用单连接（max_open_conns=1），并发查询无性能收益，建议关闭 enable_concurrent_query")
	}

	// 构建隐藏的服务商映射（provider -> reason）
	// 注意：provider 统一转小写，与 API 查询逻辑保持一致
	hiddenProviderMap := make(map[string]string)
	for i, hp := range c.HiddenProviders {
		provider := strings.ToLower(strings.TrimSpace(hp.Provider))
		if provider == "" {
			return fmt.Errorf("hidden_providers[%d]: provider 不能为空", i)
		}
		if _, exists := hiddenProviderMap[provider]; exists {
			return fmt.Errorf("hidden_providers[%d]: provider '%s' 重复配置", i, hp.Provider)
		}
		hiddenProviderMap[provider] = strings.TrimSpace(hp.Reason)
	}

	// 将全局慢请求阈值下发到每个监控项，并标准化 category、URLs、provider_slug
	slugSet := make(map[string]int) // slug -> monitor index (用于检测重复)
	for i := range c.Monitors {
		if c.Monitors[i].SlowLatencyDuration == 0 {
			c.Monitors[i].SlowLatencyDuration = c.SlowLatencyDuration
		}
		// 标准化 category 为小写
		c.Monitors[i].Category = strings.ToLower(c.Monitors[i].Category)

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

		// 检测 slug 重复 (同一 slug 可用于不同 service，仅记录不报错)
		if prevIdx, exists := slugSet[slug]; exists {
			log.Printf("[Config] 注意: provider_slug '%s' 被多个监控项使用 (monitor[%d] 和 monitor[%d])", slug, prevIdx, i)
		} else {
			slugSet[slug] = i
		}

		// 计算最终隐藏状态：providerHidden || monitorHidden
		// 原因优先级：monitor.HiddenReason > provider.Reason
		// 注意：查找时使用小写 provider，与 hiddenProviderMap 构建逻辑一致
		normalizedProvider := strings.ToLower(strings.TrimSpace(c.Monitors[i].Provider))
		providerReason, providerHidden := hiddenProviderMap[normalizedProvider]
		if providerHidden || c.Monitors[i].Hidden {
			c.Monitors[i].Hidden = true
			// 如果 monitor 自身没有设置原因，使用 provider 级别的原因
			monitorReason := strings.TrimSpace(c.Monitors[i].HiddenReason)
			if monitorReason == "" && providerHidden {
				c.Monitors[i].HiddenReason = providerReason
			} else {
				c.Monitors[i].HiddenReason = monitorReason
			}
		}
	}

	return nil
}

// ApplyEnvOverrides 应用环境变量覆盖
// API Key 格式：MONITOR_<PROVIDER>_<SERVICE>_API_KEY
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

	// API Key 覆盖
	for i := range c.Monitors {
		m := &c.Monitors[i]
		envKey := fmt.Sprintf("MONITOR_%s_%s_API_KEY",
			strings.ToUpper(strings.ReplaceAll(m.Provider, "-", "_")),
			strings.ToUpper(strings.ReplaceAll(m.Service, "-", "_")))

		if envVal := os.Getenv(envKey); envVal != "" {
			m.APIKey = envVal
		}
	}
}

// ProcessPlaceholders 处理 {{API_KEY}} 占位符替换（headers 和 body）
func (m *ServiceConfig) ProcessPlaceholders() {
	// Headers 中替换
	for k, v := range m.Headers {
		m.Headers[k] = strings.ReplaceAll(v, "{{API_KEY}}", m.APIKey)
	}

	// Body 中替换
	m.Body = strings.ReplaceAll(m.Body, "{{API_KEY}}", m.APIKey)
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

func (m *ServiceConfig) resolveBodyInclude(configDir string) error {
	const includePrefix = "!include "
	trimmed := strings.TrimSpace(m.Body)
	if trimmed == "" || !strings.HasPrefix(trimmed, includePrefix) {
		return nil
	}

	relativePath := strings.TrimSpace(trimmed[len(includePrefix):])
	if relativePath == "" {
		return fmt.Errorf("monitor provider=%s service=%s: body include 路径不能为空", m.Provider, m.Service)
	}

	if filepath.IsAbs(relativePath) {
		return fmt.Errorf("monitor provider=%s service=%s: body include 必须使用相对路径", m.Provider, m.Service)
	}

	cleanPath := filepath.Clean(relativePath)
	targetPath := filepath.Join(configDir, cleanPath)

	dataDir := filepath.Clean(filepath.Join(configDir, "data"))
	targetPath = filepath.Clean(targetPath)

	// 确保引用的文件位于 data/ 目录内
	if targetPath != dataDir && !strings.HasPrefix(targetPath, dataDir+string(os.PathSeparator)) {
		return fmt.Errorf("monitor provider=%s service=%s: body include 路径必须位于 data/ 目录", m.Provider, m.Service)
	}

	content, err := os.ReadFile(targetPath)
	if err != nil {
		return fmt.Errorf("monitor provider=%s service=%s: 读取 body include 文件失败: %w", m.Provider, m.Service, err)
	}

	m.Body = string(content)
	return nil
}

// isValidCategory 检查 category 是否为有效值
func isValidCategory(category string) bool {
	normalized := strings.ToLower(strings.TrimSpace(category))
	return normalized == "commercial" || normalized == "public"
}

// Clone 深拷贝配置（用于热更新回滚）
func (c *AppConfig) Clone() *AppConfig {
	// 深拷贝指针字段
	var staggerPtr *bool
	if c.StaggerProbes != nil {
		value := *c.StaggerProbes
		staggerPtr = &value
	}

	clone := &AppConfig{
		Interval:              c.Interval,
		IntervalDuration:      c.IntervalDuration,
		SlowLatency:           c.SlowLatency,
		SlowLatencyDuration:   c.SlowLatencyDuration,
		DegradedWeight:        c.DegradedWeight,
		MaxConcurrency:        c.MaxConcurrency,
		StaggerProbes:         staggerPtr,
		EnableConcurrentQuery: c.EnableConcurrentQuery,
		ConcurrentQueryLimit:  c.ConcurrentQueryLimit,
		Storage:               c.Storage,
		PublicBaseURL:         c.PublicBaseURL,
		HiddenProviders:       make([]HiddenProviderConfig, len(c.HiddenProviders)),
		Monitors:              make([]ServiceConfig, len(c.Monitors)),
	}
	copy(clone.HiddenProviders, c.HiddenProviders)
	copy(clone.Monitors, c.Monitors)
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

// validateURL 验证 URL 格式和协议安全性
func validateURL(rawURL, fieldName string) error {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil
	}

	parsed, err := url.ParseRequestURI(trimmed)
	if err != nil {
		return fmt.Errorf("%s 格式无效: %w", fieldName, err)
	}

	// 只允许 http 和 https 协议
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("%s 只支持 http:// 或 https:// 协议，收到: %s", fieldName, parsed.Scheme)
	}

	// 非 HTTPS 警告
	if scheme == "http" {
		log.Printf("[Config] 警告: %s 使用了非加密的 http:// 协议: %s", fieldName, trimmed)
	}

	return nil
}

// validateProviderSlug 验证 provider_slug 格式
// 规则：仅允许小写字母(a-z)、数字(0-9)、连字符(-)，且不允许连续连字符，长度不超过 100 字符
func validateProviderSlug(slug string) error {
	if slug == "" {
		return fmt.Errorf("slug 不能为空")
	}

	// 检查长度上限（与 isValidProviderSlug 保持一致）
	if len(slug) > 100 {
		return fmt.Errorf("长度超过限制（当前 %d，最大 100）", len(slug))
	}

	// 检查字符合法性
	prevIsHyphen := false
	for i, c := range slug {
		isLower := c >= 'a' && c <= 'z'
		isDigit := c >= '0' && c <= '9'
		isHyphen := c == '-'

		if !isLower && !isDigit && !isHyphen {
			return fmt.Errorf("包含非法字符 '%c' (位置 %d)，仅允许小写字母、数字、连字符", c, i)
		}

		// 检查连续连字符
		if isHyphen && prevIsHyphen {
			return fmt.Errorf("不允许连续连字符（位置 %d）", i)
		}

		prevIsHyphen = isHyphen
	}

	// 不能以连字符开头或结尾
	if slug[0] == '-' || slug[len(slug)-1] == '-' {
		return fmt.Errorf("不能以连字符开头或结尾")
	}

	return nil
}

// validateBaseURL 验证 baseURL 格式和协议
func validateBaseURL(baseURL string) error {
	if baseURL == "" {
		return fmt.Errorf("baseURL 不能为空")
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("URL 格式无效: %w", err)
	}

	// 只允许 http 和 https 协议
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("只支持 http:// 或 https:// 协议，收到: %s", parsed.Scheme)
	}

	// Host 不能为空
	if parsed.Host == "" {
		return fmt.Errorf("URL 缺少主机名")
	}

	// 非 HTTPS 警告
	if scheme == "http" {
		log.Printf("[Config] 警告: public_base_url 使用了非加密的 http:// 协议: %s", baseURL)
	}

	return nil
}
