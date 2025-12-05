package config

import (
	"testing"
)

// minimalMonitor 创建最小有效的监控配置
func minimalMonitor(provider, service string) ServiceConfig {
	return ServiceConfig{
		Provider: provider,
		Service:  service,
		URL:      "https://example.com",
		Method:   "POST",
		Category: "public",
		Sponsor:  "test",
	}
}

// TestDisabledProvidersValidation 测试 disabled_providers 验证逻辑
func TestDisabledProvidersValidation(t *testing.T) {
	tests := []struct {
		name      string
		providers []DisabledProviderConfig
		wantError bool
	}{
		{
			name:      "空列表",
			providers: nil,
			wantError: false,
		},
		{
			name: "正常配置",
			providers: []DisabledProviderConfig{
				{Provider: "provider-a", Reason: "已跑路"},
				{Provider: "provider-b", Reason: "已关站"},
			},
			wantError: false,
		},
		{
			name: "provider 为空",
			providers: []DisabledProviderConfig{
				{Provider: "", Reason: "测试"},
			},
			wantError: true,
		},
		{
			name: "重复 provider",
			providers: []DisabledProviderConfig{
				{Provider: "same-provider", Reason: "原因1"},
				{Provider: "same-provider", Reason: "原因2"},
			},
			wantError: true,
		},
		{
			name: "重复 provider（大小写不同）",
			providers: []DisabledProviderConfig{
				{Provider: "Provider-A", Reason: "原因1"},
				{Provider: "provider-a", Reason: "原因2"},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &AppConfig{
				DisabledProviders: tt.providers,
				Monitors:          []ServiceConfig{minimalMonitor("test", "test")},
			}

			err := cfg.Validate()
			if tt.wantError && err == nil {
				t.Errorf("期望报错但没有错误")
			}
			if !tt.wantError && err != nil {
				t.Errorf("不期望错误但得到: %v", err)
			}
		})
	}
}

// TestDisabledNormalize 测试 disabled 规范化逻辑
func TestDisabledNormalize(t *testing.T) {
	t.Run("Provider 级别禁用继承到 Monitor", func(t *testing.T) {
		cfg := &AppConfig{
			Interval:    "1m",
			SlowLatency: "5s",
			DisabledProviders: []DisabledProviderConfig{
				{Provider: "disabled-provider", Reason: "商家已跑路"},
			},
			Monitors: []ServiceConfig{
				minimalMonitor("disabled-provider", "cc"),
				minimalMonitor("active-provider", "cc"),
			},
		}

		if err := cfg.Normalize(); err != nil {
			t.Fatalf("Normalize 失败: %v", err)
		}

		// 检查禁用的监控项
		if !cfg.Monitors[0].Disabled {
			t.Errorf("monitor[0].Disabled = false, want true")
		}
		if cfg.Monitors[0].DisabledReason != "商家已跑路" {
			t.Errorf("monitor[0].DisabledReason = %q, want %q", cfg.Monitors[0].DisabledReason, "商家已跑路")
		}
		// 禁用应自动设置 Hidden
		if !cfg.Monitors[0].Hidden {
			t.Errorf("monitor[0].Hidden = false, want true (disabled 应自动设置 hidden)")
		}

		// 检查活跃的监控项
		if cfg.Monitors[1].Disabled {
			t.Errorf("monitor[1].Disabled = true, want false")
		}
		if cfg.Monitors[1].Hidden {
			t.Errorf("monitor[1].Hidden = true, want false")
		}
	})

	t.Run("Monitor 级别禁用原因优先", func(t *testing.T) {
		cfg := &AppConfig{
			Interval:    "1m",
			SlowLatency: "5s",
			DisabledProviders: []DisabledProviderConfig{
				{Provider: "provider-a", Reason: "Provider 级别原因"},
			},
			Monitors: []ServiceConfig{
				{
					Provider:       "provider-a",
					Service:        "cc",
					URL:            "https://example.com",
					Method:         "POST",
					Category:       "public",
					Sponsor:        "test",
					Disabled:       true,
					DisabledReason: "Monitor 级别原因",
				},
			},
		}

		if err := cfg.Normalize(); err != nil {
			t.Fatalf("Normalize 失败: %v", err)
		}

		// Monitor 级别原因应优先
		if cfg.Monitors[0].DisabledReason != "Monitor 级别原因" {
			t.Errorf("DisabledReason = %q, want %q", cfg.Monitors[0].DisabledReason, "Monitor 级别原因")
		}
	})

	t.Run("单独 Monitor 禁用（无 Provider 配置）", func(t *testing.T) {
		cfg := &AppConfig{
			Interval:    "1m",
			SlowLatency: "5s",
			Monitors: []ServiceConfig{
				{
					Provider:       "provider-a",
					Service:        "cc",
					URL:            "https://example.com",
					Method:         "POST",
					Category:       "public",
					Sponsor:        "test",
					Disabled:       true,
					DisabledReason: "该通道已废弃",
				},
			},
		}

		if err := cfg.Normalize(); err != nil {
			t.Fatalf("Normalize 失败: %v", err)
		}

		if !cfg.Monitors[0].Disabled {
			t.Errorf("Disabled = false, want true")
		}
		if !cfg.Monitors[0].Hidden {
			t.Errorf("Hidden = false, want true")
		}
		if cfg.Monitors[0].HiddenReason != "该通道已废弃" {
			t.Errorf("HiddenReason = %q, want %q", cfg.Monitors[0].HiddenReason, "该通道已废弃")
		}
	})

	t.Run("Disabled 和 Hidden 混合场景", func(t *testing.T) {
		cfg := &AppConfig{
			Interval:    "1m",
			SlowLatency: "5s",
			DisabledProviders: []DisabledProviderConfig{
				{Provider: "disabled-provider", Reason: "已跑路"},
			},
			HiddenProviders: []HiddenProviderConfig{
				{Provider: "hidden-provider", Reason: "整改中"},
			},
			Monitors: []ServiceConfig{
				minimalMonitor("disabled-provider", "cc"), // 应禁用
				minimalMonitor("hidden-provider", "cc"),   // 应隐藏但继续监控
				minimalMonitor("active-provider", "cc"),   // 应正常
			},
		}

		if err := cfg.Normalize(); err != nil {
			t.Fatalf("Normalize 失败: %v", err)
		}

		// 检查禁用的
		if !cfg.Monitors[0].Disabled || !cfg.Monitors[0].Hidden {
			t.Errorf("monitor[0]: Disabled=%v, Hidden=%v, want both true",
				cfg.Monitors[0].Disabled, cfg.Monitors[0].Hidden)
		}

		// 检查隐藏的（非禁用）
		if cfg.Monitors[1].Disabled {
			t.Errorf("monitor[1].Disabled = true, want false")
		}
		if !cfg.Monitors[1].Hidden {
			t.Errorf("monitor[1].Hidden = false, want true")
		}

		// 检查活跃的
		if cfg.Monitors[2].Disabled || cfg.Monitors[2].Hidden {
			t.Errorf("monitor[2]: Disabled=%v, Hidden=%v, want both false",
				cfg.Monitors[2].Disabled, cfg.Monitors[2].Hidden)
		}
	})
}

// TestDisabledClone 测试 Clone 正确复制 DisabledProviders
func TestDisabledClone(t *testing.T) {
	original := &AppConfig{
		Interval:    "1m",
		SlowLatency: "5s",
		DisabledProviders: []DisabledProviderConfig{
			{Provider: "provider-a", Reason: "原因A"},
			{Provider: "provider-b", Reason: "原因B"},
		},
		Monitors: []ServiceConfig{minimalMonitor("test", "test")},
	}

	if err := original.Normalize(); err != nil {
		t.Fatalf("Normalize 失败: %v", err)
	}

	cloned := original.Clone()

	// 检查 DisabledProviders 被正确复制
	if len(cloned.DisabledProviders) != len(original.DisabledProviders) {
		t.Errorf("Clone DisabledProviders 长度不匹配: got %d, want %d",
			len(cloned.DisabledProviders), len(original.DisabledProviders))
	}

	// 修改原始配置，确保不影响克隆
	original.DisabledProviders[0].Reason = "修改后的原因"
	if cloned.DisabledProviders[0].Reason == "修改后的原因" {
		t.Errorf("Clone 应该是深拷贝，但修改原始配置影响了克隆")
	}
}
