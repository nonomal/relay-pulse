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
	Database   DatabaseConfig   `yaml:"database"`
	API        APIConfig        `yaml:"api"`
	Limits     LimitsConfig     `yaml:"limits"`
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
	RateLimitPerSecond      int           `yaml:"rate_limit_per_second"`
	MaxRetries              int           `yaml:"max_retries"`
	BindTokenTTL            time.Duration `yaml:"bind_token_ttl"`
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
		c.Database.DSN = "file:notifier.db?_journal_mode=WAL"
	}
	if c.API.Addr == "" {
		c.API.Addr = ":8081"
	}
	if c.Limits.MaxSubscriptionsPerUser == 0 {
		c.Limits.MaxSubscriptionsPerUser = 20
	}
	if c.Limits.RateLimitPerSecond == 0 {
		c.Limits.RateLimitPerSecond = 25
	}
	if c.Limits.MaxRetries == 0 {
		c.Limits.MaxRetries = 3
	}
	if c.Limits.BindTokenTTL == 0 {
		c.Limits.BindTokenTTL = 5 * time.Minute
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
