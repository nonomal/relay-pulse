package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"monitor/internal/logger"
)

// Loader 配置加载器
type Loader struct {
	currentConfig *AppConfig
}

// NewLoader 创建配置加载器
func NewLoader() *Loader {
	return &Loader{}
}

// Load 加载并验证配置文件
func (l *Loader) Load(filename string) (*AppConfig, error) {
	// 读取文件
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析 YAML
	var cfg AppConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	absPath, err := filepath.Abs(filename)
	if err != nil {
		return nil, fmt.Errorf("解析配置文件路径失败: %w", err)
	}
	configDir := filepath.Dir(absPath)

	// 验证配置
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	// 应用环境变量覆盖
	cfg.applyEnvOverrides()

	// 解析模板引用
	if err := cfg.resolveTemplates(configDir); err != nil {
		return nil, err
	}

	// 规范化配置（填充默认值等）
	if err := cfg.normalize(); err != nil {
		return nil, fmt.Errorf("配置规范化失败: %w", err)
	}

	// Phase 1: 最终态校验仅告警，不阻断启动或热更新
	// 捕获 env 覆盖、模板注入、继承和 Normalize 之后可能遗留的不一致配置
	for _, warn := range cfg.validateFinal() {
		logger.Warn("config", "最终配置校验告警", "detail", warn.Error())
	}

	l.currentConfig = &cfg
	return &cfg, nil
}

// loadOrRollback 加载配置，失败时保持旧配置
func (l *Loader) loadOrRollback(filename string) (*AppConfig, error) {
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

// getCurrent 获取当前配置（仅包内使用）
func (l *Loader) getCurrent() *AppConfig {
	return l.currentConfig
}
