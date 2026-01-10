package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 通知服务配置
type Config struct {
	RelayPulse RelayPulseConfig `yaml:"relay_pulse"`
	Telegram   TelegramConfig   `yaml:"telegram"`
	QQ         QQConfig         `yaml:"qq"`
	Database   DatabaseConfig   `yaml:"database"`
	API        APIConfig        `yaml:"api"`
	Limits     LimitsConfig     `yaml:"limits"`
	Screenshot ScreenshotConfig `yaml:"screenshot"`
}

// RelayPulseConfig relay-pulse 事件 API 配置
type RelayPulseConfig struct {
	EventsURL    string        `yaml:"events_url"`
	APIToken     string        `yaml:"api_token"`
	PollInterval time.Duration `yaml:"poll_interval"`
}

// TelegramConfig Telegram Bot 配置
type TelegramConfig struct {
	BotToken    string `yaml:"bot_token"`
	BotUsername string `yaml:"bot_username"`
}

// QQConfig QQ Bot 配置（OneBot v11 / NapCatQQ）
type QQConfig struct {
	Enabled        bool    `yaml:"enabled"`         // 是否启用 QQ 通知
	OneBotHTTPURL  string  `yaml:"onebot_http_url"` // OneBot HTTP API 地址
	AccessToken    string  `yaml:"access_token"`    // OneBot API Token（可选）
	CallbackPath   string  `yaml:"callback_path"`   // 接收上报的路径，默认 /qq/callback
	CallbackSecret string  `yaml:"callback_secret"` // Webhook 签名密钥（可选）
	AdminWhitelist []int64 `yaml:"admin_whitelist"` // 管理员白名单 QQ 号（可越权执行管理命令）
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Driver string `yaml:"driver"` // sqlite 或 postgres
	DSN    string `yaml:"dsn"`
}

// APIConfig HTTP API 配置
type APIConfig struct {
	Addr string `yaml:"addr"` // 监听地址，如 :8081
}

// LimitsConfig 限制配置
type LimitsConfig struct {
	MaxSubscriptionsPerUser int           `yaml:"max_subscriptions_per_user"`
	MaxRetries              int           `yaml:"max_retries"`
	BindTokenTTL            time.Duration `yaml:"bind_token_ttl"`

	// 平台独立限流配置
	TelegramRateLimitPerSecond int `yaml:"telegram_rate_limit_per_second"` // Telegram 发送限流（每秒消息数）
	QQRateLimitPerSecond       int `yaml:"qq_rate_limit_per_second"`       // QQ 发送限流（每秒消息数，建议 1-2）

	// QQ 发送抖动：在通过限流后额外 sleep 一段随机时间，用于错峰降低风控
	QQJitterMin time.Duration `yaml:"qq_jitter_min"`
	QQJitterMax time.Duration `yaml:"qq_jitter_max"`

	// RateLimitPerSecond 兼容旧配置（deprecated）：等价于 TelegramRateLimitPerSecond
	RateLimitPerSecond int `yaml:"rate_limit_per_second"`
}

// ScreenshotConfig 截图功能配置
type ScreenshotConfig struct {
	Enabled       bool          `yaml:"enabled"`        // 是否启用截图功能
	BaseURL       string        `yaml:"base_url"`       // 截图目标 URL，默认 https://relaypulse.top
	Timeout       time.Duration `yaml:"timeout"`        // 截图超时时间，默认 30s
	MaxConcurrent int           `yaml:"max_concurrent"` // 最大并发数，默认 3
}

// Load 从文件加载配置，并应用环境变量覆盖
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 应用环境变量覆盖
	cfg.applyEnvOverrides()

	// 设置默认值
	cfg.setDefaults()

	// 验证配置
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// applyEnvOverrides 从环境变量覆盖配置
func (c *Config) applyEnvOverrides() {
	if v := os.Getenv("RELAY_PULSE_EVENTS_URL"); v != "" {
		c.RelayPulse.EventsURL = v
	}
	if v := os.Getenv("RELAY_PULSE_API_TOKEN"); v != "" {
		c.RelayPulse.APIToken = v
	}
	if v := os.Getenv("TELEGRAM_BOT_TOKEN"); v != "" {
		c.Telegram.BotToken = v
	}
	if v := os.Getenv("DATABASE_DSN"); v != "" {
		c.Database.DSN = v
	}
	if v := os.Getenv("API_ADDR"); v != "" {
		c.API.Addr = v
	}
	// QQ 相关环境变量
	if v := os.Getenv("QQ_ONEBOT_HTTP_URL"); v != "" {
		c.QQ.OneBotHTTPURL = v
		c.QQ.Enabled = true
	}
	if v := os.Getenv("QQ_ACCESS_TOKEN"); v != "" {
		c.QQ.AccessToken = v
	}
	if v := os.Getenv("QQ_CALLBACK_SECRET"); v != "" {
		c.QQ.CallbackSecret = v
	}
}

// setDefaults 设置默认值
func (c *Config) setDefaults() {
	if c.RelayPulse.PollInterval == 0 {
		c.RelayPulse.PollInterval = 5 * time.Second
	}
	if c.Database.Driver == "" {
		c.Database.Driver = "sqlite"
	}
	if c.Database.DSN == "" {
		c.Database.DSN = "file:notifier.db?_journal_mode=WAL&_timeout=5000&_busy_timeout=5000"
	}
	if c.API.Addr == "" {
		c.API.Addr = ":8081"
	}
	if c.Limits.MaxSubscriptionsPerUser == 0 {
		c.Limits.MaxSubscriptionsPerUser = 20
	}
	// 平台独立限流默认值
	// 兼容旧字段：rate_limit_per_second 视为 Telegram 限流
	if c.Limits.TelegramRateLimitPerSecond == 0 {
		if c.Limits.RateLimitPerSecond > 0 {
			c.Limits.TelegramRateLimitPerSecond = c.Limits.RateLimitPerSecond
		} else {
			c.Limits.TelegramRateLimitPerSecond = 25
		}
	}
	if c.Limits.QQRateLimitPerSecond == 0 {
		c.Limits.QQRateLimitPerSecond = 2 // QQ 保守限流
	}
	// QQ 抖动默认值：0-300ms
	if c.Limits.QQJitterMax == 0 {
		c.Limits.QQJitterMax = 300 * time.Millisecond
	}
	if c.Limits.MaxRetries == 0 {
		c.Limits.MaxRetries = 3
	}
	if c.Limits.BindTokenTTL == 0 {
		c.Limits.BindTokenTTL = 5 * time.Minute
	}
	// QQ 默认值
	if c.QQ.CallbackPath == "" {
		c.QQ.CallbackPath = "/qq/callback"
	}
	// Screenshot 默认值
	if c.Screenshot.BaseURL == "" {
		c.Screenshot.BaseURL = "https://relaypulse.top"
	}
	if c.Screenshot.Timeout == 0 {
		c.Screenshot.Timeout = 30 * time.Second
	}
	if c.Screenshot.MaxConcurrent == 0 {
		c.Screenshot.MaxConcurrent = 3
	}
}

// validate 验证配置
func (c *Config) validate() error {
	if c.RelayPulse.EventsURL == "" {
		return fmt.Errorf("relay_pulse.events_url 是必需的")
	}
	if c.RelayPulse.APIToken == "" {
		return fmt.Errorf("relay_pulse.api_token 是必需的（环境变量 RELAY_PULSE_API_TOKEN）")
	}
	// Telegram Bot Token 在开发环境可选（仅 API 服务启动）
	// 如果未设置，Bot 和 Poller 功能将不可用
	return nil
}

// HasTelegramToken 检查是否配置了 Telegram Bot Token
func (c *Config) HasTelegramToken() bool {
	return c.Telegram.BotToken != ""
}

// HasQQ 检查是否启用了 QQ 通知
func (c *Config) HasQQ() bool {
	return c.QQ.Enabled && c.QQ.OneBotHTTPURL != ""
}

// HasScreenshot 检查是否启用了截图功能
func (c *Config) HasScreenshot() bool {
	return c.Screenshot.Enabled
}
