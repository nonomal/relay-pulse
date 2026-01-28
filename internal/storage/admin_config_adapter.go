package storage

import (
	"monitor/internal/config"
)

// AdminConfigAdapter 适配 config.AdminConfigStorage 接口
// 将 storage.SQLiteStorage 的方法转换为 config 包定义的接口
type AdminConfigAdapter struct {
	*SQLiteStorage
}

// NewAdminConfigAdapter 创建配置管理适配器
func NewAdminConfigAdapter(s *SQLiteStorage) *AdminConfigAdapter {
	return &AdminConfigAdapter{SQLiteStorage: s}
}

// ListMonitorConfigs 实现 config.AdminConfigStorage 接口
func (a *AdminConfigAdapter) ListMonitorConfigs(filter *config.MonitorConfigFilter) ([]*config.MonitorConfigRecord, int, error) {
	// 转换 filter
	var storageFilter *MonitorConfigFilter
	if filter != nil {
		storageFilter = &MonitorConfigFilter{
			Provider:       filter.Provider,
			Service:        filter.Service,
			Channel:        filter.Channel,
			Model:          filter.Model,
			Enabled:        filter.Enabled,
			IncludeDeleted: filter.IncludeDeleted,
			Search:         filter.Search,
			Offset:         filter.Offset,
			Limit:          filter.Limit,
		}
	}

	configs, total, err := a.SQLiteStorage.ListMonitorConfigs(storageFilter)
	if err != nil {
		return nil, 0, err
	}

	// 转换结果
	result := make([]*config.MonitorConfigRecord, len(configs))
	for i, c := range configs {
		result[i] = &config.MonitorConfigRecord{
			ID:         c.ID,
			Provider:   c.Provider,
			Service:    c.Service,
			Channel:    c.Channel,
			Model:      c.Model,
			Name:       c.Name,
			Enabled:    c.Enabled,
			ParentKey:  c.ParentKey,
			ConfigBlob: c.ConfigBlob,
			Version:    c.Version,
			DeletedAt:  c.DeletedAt,
		}
	}

	return result, total, nil
}

// GetMonitorSecret 实现 config.AdminConfigStorage 接口
func (a *AdminConfigAdapter) GetMonitorSecret(monitorID int64) (*config.MonitorSecretRecord, error) {
	secret, err := a.SQLiteStorage.GetMonitorSecret(monitorID)
	if err != nil {
		return nil, err
	}
	if secret == nil {
		return nil, nil
	}

	return &config.MonitorSecretRecord{
		MonitorID:        secret.MonitorID,
		APIKeyCiphertext: secret.APIKeyCiphertext,
		APIKeyNonce:      secret.APIKeyNonce,
		KeyVersion:       secret.KeyVersion,
		EncVersion:       secret.EncVersion,
	}, nil
}

// ListProviderPolicies 实现 config.AdminConfigStorage 接口
func (a *AdminConfigAdapter) ListProviderPolicies() ([]*config.ProviderPolicyRecord, error) {
	policies, err := a.SQLiteStorage.ListProviderPolicies()
	if err != nil {
		return nil, err
	}

	result := make([]*config.ProviderPolicyRecord, len(policies))
	for i, p := range policies {
		result[i] = &config.ProviderPolicyRecord{
			ID:         p.ID,
			PolicyType: string(p.PolicyType),
			Provider:   p.Provider,
			Reason:     p.Reason,
			Risks:      p.Risks,
		}
	}

	return result, nil
}

// ListBadgeDefinitions 实现 config.AdminConfigStorage 接口
func (a *AdminConfigAdapter) ListBadgeDefinitions() ([]*config.BadgeDefinitionRecord, error) {
	badges, err := a.SQLiteStorage.ListBadgeDefinitions()
	if err != nil {
		return nil, err
	}

	result := make([]*config.BadgeDefinitionRecord, len(badges))
	for i, b := range badges {
		result[i] = &config.BadgeDefinitionRecord{
			ID:          b.ID,
			Kind:        string(b.Kind),
			Weight:      b.Weight,
			LabelI18n:   b.LabelI18n,
			TooltipI18n: b.TooltipI18n,
			Icon:        b.Icon,
			Color:       b.Color,
		}
	}

	return result, nil
}

// ListBadgeBindings 实现 config.AdminConfigStorage 接口
func (a *AdminConfigAdapter) ListBadgeBindings(filter *config.BadgeBindingFilter) ([]*config.BadgeBindingRecord, error) {
	// 转换 filter
	var storageFilter *BadgeBindingFilter
	if filter != nil {
		storageFilter = &BadgeBindingFilter{
			BadgeID:  filter.BadgeID,
			Scope:    BadgeScope(filter.Scope),
			Provider: filter.Provider,
			Service:  filter.Service,
			Channel:  filter.Channel,
		}
	}

	bindings, err := a.SQLiteStorage.ListBadgeBindings(storageFilter)
	if err != nil {
		return nil, err
	}

	result := make([]*config.BadgeBindingRecord, len(bindings))
	for i, b := range bindings {
		result[i] = &config.BadgeBindingRecord{
			ID:              b.ID,
			BadgeID:         b.BadgeID,
			Scope:           string(b.Scope),
			Provider:        b.Provider,
			Service:         b.Service,
			Channel:         b.Channel,
			TooltipOverride: b.TooltipOverride,
		}
	}

	return result, nil
}

// ListBoardConfigs 实现 config.AdminConfigStorage 接口
func (a *AdminConfigAdapter) ListBoardConfigs() ([]*config.BoardConfigRecord, error) {
	configs, err := a.SQLiteStorage.ListBoardConfigs()
	if err != nil {
		return nil, err
	}

	result := make([]*config.BoardConfigRecord, len(configs))
	for i, c := range configs {
		result[i] = &config.BoardConfigRecord{
			Board:       c.Board,
			DisplayName: c.DisplayName,
			Description: c.Description,
			SortOrder:   c.SortOrder,
		}
	}

	return result, nil
}

// GetGlobalSetting 实现 config.AdminConfigStorage 接口
func (a *AdminConfigAdapter) GetGlobalSetting(key string) (*config.GlobalSettingRecord, error) {
	setting, err := a.SQLiteStorage.GetGlobalSetting(key)
	if err != nil {
		return nil, err
	}
	if setting == nil {
		return nil, nil
	}

	return &config.GlobalSettingRecord{
		Key:     setting.Key,
		Value:   setting.Value,
		Version: setting.Version,
	}, nil
}

// GetConfigVersions 实现 config.AdminConfigStorage 接口
func (a *AdminConfigAdapter) GetConfigVersions() (*config.ConfigVersionsRecord, error) {
	versions, err := a.SQLiteStorage.GetConfigVersions()
	if err != nil {
		return nil, err
	}

	return &config.ConfigVersionsRecord{
		Monitors: versions.Monitors,
		Policies: versions.Policies,
		Badges:   versions.Badges,
		Boards:   versions.Boards,
		Settings: versions.Settings,
	}, nil
}
