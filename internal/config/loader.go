package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ConfigSource 配置来源常量
const (
	ConfigSourceYAML     = "yaml"     // YAML 配置文件（默认）
	ConfigSourceDatabase = "database" // 数据库配置
)

// Loader 配置加载器
type Loader struct {
	currentConfig *AppConfig
	dbProvider    *DBConfigProvider // 数据库配置提供者（可选）
}

// NewLoader 创建配置加载器
func NewLoader() *Loader {
	return &Loader{}
}

// SetDBProvider 设置数据库配置提供者
// 用于支持 config_source=database 模式
func (l *Loader) SetDBProvider(provider *DBConfigProvider) {
	l.dbProvider = provider
}

// Load 加载并验证配置文件
// 根据 config_source 字段决定加载方式：
// - yaml（默认）: 完全从 YAML 文件加载
// - database: 从 YAML 加载启动引导配置，从数据库加载功能配置
func (l *Loader) Load(filename string) (*AppConfig, error) {
	// 读取 YAML 文件
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析 YAML（获取基础配置）
	var baseCfg AppConfig
	if err := yaml.Unmarshal(data, &baseCfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	absPath, err := filepath.Abs(filename)
	if err != nil {
		return nil, fmt.Errorf("解析配置文件路径失败: %w", err)
	}
	configDir := filepath.Dir(absPath)

	// 根据 config_source 决定加载方式
	configSource := strings.ToLower(strings.TrimSpace(baseCfg.ConfigSource))
	if configSource == "" {
		configSource = ConfigSourceYAML
	}

	var cfg *AppConfig

	switch configSource {
	case ConfigSourceYAML:
		// YAML 模式：使用完整的 YAML 配置
		cfg = &baseCfg
	case ConfigSourceDatabase:
		// 数据库模式：从数据库加载功能配置
		cfg, err = l.loadFromDatabase(context.Background(), &baseCfg)
		if err != nil {
			return nil, fmt.Errorf("从数据库加载配置失败: %w", err)
		}
	default:
		return nil, fmt.Errorf("不支持的 config_source: %s（可选: yaml, database）", configSource)
	}

	// 验证配置
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	// 应用环境变量覆盖
	cfg.ApplyEnvOverrides()

	// 解析 body include（仅 YAML 模式需要）
	if configSource == ConfigSourceYAML {
		if err := cfg.ResolveBodyIncludes(configDir); err != nil {
			return nil, err
		}
	}

	// 规范化配置（填充默认值等）
	if err := cfg.Normalize(); err != nil {
		return nil, fmt.Errorf("配置规范化失败: %w", err)
	}

	// 处理占位符
	for i := range cfg.Monitors {
		cfg.Monitors[i].ProcessPlaceholders()
	}

	l.currentConfig = cfg
	return cfg, nil
}

// loadFromDatabase 从数据库加载功能配置
// baseCfg 包含启动引导配置（storage、config_source 等）
func (l *Loader) loadFromDatabase(ctx context.Context, baseCfg *AppConfig) (*AppConfig, error) {
	if l.dbProvider == nil {
		return nil, fmt.Errorf("config_source=database 但未设置数据库配置提供者")
	}

	// 从数据库加载完整功能配置
	dbCfg, err := l.dbProvider.LoadFullConfig(ctx)
	if err != nil {
		return nil, err
	}

	// 合并启动引导配置（YAML 中的运行时/元配置优先）
	mergeBootstrapConfig(dbCfg, baseCfg)

	return dbCfg, nil
}

// mergeBootstrapConfig 将 YAML 中的启动引导配置合并到数据库配置
// 启动引导配置（运行时/元配置）始终从 YAML 读取，不会被数据库覆盖
func mergeBootstrapConfig(target, bootstrap *AppConfig) {
	// Storage 配置（必须从 YAML 读取）
	target.Storage = bootstrap.Storage

	// ConfigSource 配置
	target.ConfigSource = bootstrap.ConfigSource

	// PublicBaseURL
	if bootstrap.PublicBaseURL != "" {
		target.PublicBaseURL = bootstrap.PublicBaseURL
	}

	// SelfTest 配置（包含签名密钥，必须从 YAML/环境变量读取）
	target.SelfTest = bootstrap.SelfTest

	// Events 配置（包含 API Token，必须从 YAML/环境变量读取）
	target.Events = bootstrap.Events

	// Announcements 配置
	target.Announcements = bootstrap.Announcements

	// GitHub 配置（包含 Token，必须从 YAML/环境变量读取）
	target.GitHub = bootstrap.GitHub

	// ChannelDetailsProviders（provider 级覆盖配置）
	if len(bootstrap.ChannelDetailsProviders) > 0 {
		target.ChannelDetailsProviders = bootstrap.ChannelDetailsProviders
	}
}

// LoadOrRollback 加载配置，失败时保持旧配置
func (l *Loader) LoadOrRollback(filename string) (*AppConfig, error) {
	newConfig, err := l.Load(filename)
	if err != nil {
		// 返回错误但保持旧配置
		if l.currentConfig != nil {
			return l.currentConfig, fmt.Errorf("配置加载失败，保持旧配置: %w", err)
		}
		return nil, err
	}
	return newConfig, nil
}

// GetCurrent 获取当前配置
func (l *Loader) GetCurrent() *AppConfig {
	return l.currentConfig
}

// LoadFromDatabaseOnly 直接从数据库加载配置（不依赖 YAML 文件）
// 用于热更新场景，仅重新加载数据库中的功能配置
func (l *Loader) LoadFromDatabaseOnly(ctx context.Context) (*AppConfig, error) {
	if l.dbProvider == nil {
		return nil, fmt.Errorf("未设置数据库配置提供者")
	}
	if l.currentConfig == nil {
		return nil, fmt.Errorf("当前配置为空，无法进行热更新")
	}

	// 从数据库加载功能配置
	dbCfg, err := l.dbProvider.LoadFullConfig(ctx)
	if err != nil {
		return nil, err
	}

	// 保留当前的启动引导配置
	mergeBootstrapConfig(dbCfg, l.currentConfig)

	// 验证配置
	if err := dbCfg.Validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	// 规范化配置
	if err := dbCfg.Normalize(); err != nil {
		return nil, fmt.Errorf("配置规范化失败: %w", err)
	}

	// 处理占位符
	for i := range dbCfg.Monitors {
		dbCfg.Monitors[i].ProcessPlaceholders()
	}

	l.currentConfig = dbCfg
	return dbCfg, nil
}
