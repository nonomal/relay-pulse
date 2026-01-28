package config

import (
	"context"
	"encoding/json"
	"testing"
)

// mockAdminConfigStorage 用于测试的 mock 存储
type mockAdminConfigStorage struct {
	monitors []*MonitorConfigRecord
	secrets  map[int64]*MonitorSecretRecord
	policies []*ProviderPolicyRecord
	badges   []*BadgeDefinitionRecord
	bindings []*BadgeBindingRecord
	boards   []*BoardConfigRecord
	settings map[string]*GlobalSettingRecord
	versions *ConfigVersionsRecord
}

func newMockStorage() *mockAdminConfigStorage {
	return &mockAdminConfigStorage{
		secrets:  make(map[int64]*MonitorSecretRecord),
		settings: make(map[string]*GlobalSettingRecord),
		versions: &ConfigVersionsRecord{
			Monitors: 1,
			Policies: 1,
			Badges:   1,
			Boards:   1,
			Settings: 1,
		},
	}
}

func (m *mockAdminConfigStorage) ListMonitorConfigs(filter *MonitorConfigFilter) ([]*MonitorConfigRecord, int, error) {
	result := make([]*MonitorConfigRecord, 0)
	for _, mc := range m.monitors {
		// 简单过滤
		if filter != nil && filter.Enabled != nil && *filter.Enabled != mc.Enabled {
			continue
		}
		if filter != nil && !filter.IncludeDeleted && mc.DeletedAt != nil {
			continue
		}
		result = append(result, mc)
	}
	return result, len(result), nil
}

func (m *mockAdminConfigStorage) GetMonitorSecret(monitorID int64) (*MonitorSecretRecord, error) {
	return m.secrets[monitorID], nil
}

func (m *mockAdminConfigStorage) ListProviderPolicies() ([]*ProviderPolicyRecord, error) {
	return m.policies, nil
}

func (m *mockAdminConfigStorage) ListBadgeDefinitions() ([]*BadgeDefinitionRecord, error) {
	return m.badges, nil
}

func (m *mockAdminConfigStorage) ListBadgeBindings(filter *BadgeBindingFilter) ([]*BadgeBindingRecord, error) {
	result := make([]*BadgeBindingRecord, 0)
	for _, b := range m.bindings {
		if filter != nil && filter.Scope != "" && filter.Scope != b.Scope {
			continue
		}
		result = append(result, b)
	}
	return result, nil
}

func (m *mockAdminConfigStorage) ListBoardConfigs() ([]*BoardConfigRecord, error) {
	return m.boards, nil
}

func (m *mockAdminConfigStorage) GetGlobalSetting(key string) (*GlobalSettingRecord, error) {
	return m.settings[key], nil
}

func (m *mockAdminConfigStorage) GetConfigVersions() (*ConfigVersionsRecord, error) {
	return m.versions, nil
}

func TestDBConfigProviderLoadMonitors(t *testing.T) {
	storage := newMockStorage()

	// 添加测试数据
	payload := MonitorConfigPayload{
		URL:      "https://api.example.com/v1/chat",
		Method:   "POST",
		Headers:  map[string]string{"Content-Type": "application/json"},
		Body:     `{"model": "test"}`,
		Category: "commercial",
	}
	payloadJSON, _ := json.Marshal(payload)

	storage.monitors = []*MonitorConfigRecord{
		{
			ID:         1,
			Provider:   "test-provider",
			Service:    "cc",
			Channel:    "default",
			Model:      "",
			Name:       "Test Monitor",
			Enabled:    true,
			ConfigBlob: string(payloadJSON),
			Version:    1,
		},
		{
			ID:       2,
			Provider: "test-provider",
			Service:  "cc",
			Channel:  "vip",
			Model:    "",
			Enabled:  false, // 禁用的
			Version:  1,
		},
	}

	provider := NewDBConfigProvider(storage)
	monitors, err := provider.LoadMonitors(context.Background())
	if err != nil {
		t.Fatalf("LoadMonitors 失败: %v", err)
	}

	// 应只加载启用的监测项
	if len(monitors) != 1 {
		t.Errorf("期望 1 个监测项，实际 %d 个", len(monitors))
	}

	if monitors[0].Provider != "test-provider" {
		t.Errorf("Provider 不匹配: %s", monitors[0].Provider)
	}
	if monitors[0].URL != "https://api.example.com/v1/chat" {
		t.Errorf("URL 不匹配: %s", monitors[0].URL)
	}
	if monitors[0].Category != "commercial" {
		t.Errorf("Category 不匹配: %s", monitors[0].Category)
	}
}

func TestDBConfigProviderLoadGlobalSettings(t *testing.T) {
	storage := newMockStorage()

	// 设置全局配置
	settings := GlobalSettingsPayload{
		Interval:       "30s",
		SlowLatency:    "3s",
		Timeout:        "15s",
		DegradedWeight: 0.8,
	}
	settingsJSON, _ := json.Marshal(settings)
	storage.settings["global"] = &GlobalSettingRecord{
		Key:     "global",
		Value:   string(settingsJSON),
		Version: 1,
	}

	provider := NewDBConfigProvider(storage)
	result, err := provider.LoadGlobalSettings(context.Background())
	if err != nil {
		t.Fatalf("LoadGlobalSettings 失败: %v", err)
	}

	if result.Interval != "30s" {
		t.Errorf("Interval 不匹配: %s", result.Interval)
	}
	if result.SlowLatency != "3s" {
		t.Errorf("SlowLatency 不匹配: %s", result.SlowLatency)
	}
	if result.Timeout != "15s" {
		t.Errorf("Timeout 不匹配: %s", result.Timeout)
	}
	if result.DegradedWeight != 0.8 {
		t.Errorf("DegradedWeight 不匹配: %f", result.DegradedWeight)
	}
}

func TestDBConfigProviderLoadGlobalSettingsDefault(t *testing.T) {
	storage := newMockStorage()
	// 不设置任何全局配置，应返回默认值

	provider := NewDBConfigProvider(storage)
	result, err := provider.LoadGlobalSettings(context.Background())
	if err != nil {
		t.Fatalf("LoadGlobalSettings 失败: %v", err)
	}

	if result.Interval != "1m" {
		t.Errorf("默认 Interval 应为 1m，实际 %s", result.Interval)
	}
	if result.SlowLatency != "5s" {
		t.Errorf("默认 SlowLatency 应为 5s，实际 %s", result.SlowLatency)
	}
	if result.DegradedWeight != 0.7 {
		t.Errorf("默认 DegradedWeight 应为 0.7，实际 %f", result.DegradedWeight)
	}
}

func TestDBConfigProviderLoadProviderPolicies(t *testing.T) {
	storage := newMockStorage()

	storage.policies = []*ProviderPolicyRecord{
		{
			ID:         1,
			PolicyType: "disabled",
			Provider:   "bad-provider",
			Reason:     "已跑路",
		},
		{
			ID:         2,
			PolicyType: "hidden",
			Provider:   "temp-hidden",
			Reason:     "整改中",
		},
		{
			ID:         3,
			PolicyType: "risk",
			Provider:   "risky-provider",
			Risks:      `[{"label":"跑路风险","discussion_url":"https://example.com/d/1"}]`,
		},
	}

	provider := NewDBConfigProvider(storage)
	disabled, hidden, risks, err := provider.LoadProviderPolicies(context.Background())
	if err != nil {
		t.Fatalf("LoadProviderPolicies 失败: %v", err)
	}

	if len(disabled) != 1 {
		t.Errorf("期望 1 个禁用策略，实际 %d 个", len(disabled))
	}
	if disabled[0].Provider != "bad-provider" {
		t.Errorf("禁用 Provider 不匹配: %s", disabled[0].Provider)
	}

	if len(hidden) != 1 {
		t.Errorf("期望 1 个隐藏策略，实际 %d 个", len(hidden))
	}
	if hidden[0].Provider != "temp-hidden" {
		t.Errorf("隐藏 Provider 不匹配: %s", hidden[0].Provider)
	}

	if len(risks) != 1 {
		t.Errorf("期望 1 个风险策略，实际 %d 个", len(risks))
	}
	if risks[0].Provider != "risky-provider" {
		t.Errorf("风险 Provider 不匹配: %s", risks[0].Provider)
	}
	if len(risks[0].Risks) != 1 {
		t.Errorf("期望 1 个风险徽标，实际 %d 个", len(risks[0].Risks))
	}
}

func TestDBConfigProviderLoadFullConfig(t *testing.T) {
	storage := newMockStorage()

	// 设置监测项
	payload := MonitorConfigPayload{
		URL:    "https://api.example.com/v1/chat",
		Method: "POST",
	}
	payloadJSON, _ := json.Marshal(payload)
	storage.monitors = []*MonitorConfigRecord{
		{
			ID:         1,
			Provider:   "test",
			Service:    "cc",
			Channel:    "default",
			Enabled:    true,
			ConfigBlob: string(payloadJSON),
			Version:    1,
		},
	}

	// 设置全局配置
	settings := GlobalSettingsPayload{
		Interval:       "2m",
		SlowLatency:    "10s",
		Timeout:        "30s",
		DegradedWeight: 0.6,
	}
	settingsJSON, _ := json.Marshal(settings)
	storage.settings["global"] = &GlobalSettingRecord{
		Key:     "global",
		Value:   string(settingsJSON),
		Version: 1,
	}

	provider := NewDBConfigProvider(storage)
	cfg, err := provider.LoadFullConfig(context.Background())
	if err != nil {
		t.Fatalf("LoadFullConfig 失败: %v", err)
	}

	if cfg.Interval != "2m" {
		t.Errorf("Interval 不匹配: %s", cfg.Interval)
	}
	if len(cfg.Monitors) != 1 {
		t.Errorf("期望 1 个监测项，实际 %d 个", len(cfg.Monitors))
	}
	if cfg.Monitors[0].Provider != "test" {
		t.Errorf("Monitor Provider 不匹配: %s", cfg.Monitors[0].Provider)
	}
}
