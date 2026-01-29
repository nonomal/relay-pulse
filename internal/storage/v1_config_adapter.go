package storage

import (
	"context"

	"monitor/internal/config"
)

// PostgresV1ConfigAdapter 实现 config.V1Storage 接口
// 将 PostgresStorage 的 v1 监测项查询转换为 config 包期望的格式
type PostgresV1ConfigAdapter struct {
	storage *PostgresStorage
}

// NewPostgresV1ConfigAdapter 创建 v1 配置适配器
func NewPostgresV1ConfigAdapter(s *PostgresStorage) *PostgresV1ConfigAdapter {
	return &PostgresV1ConfigAdapter{storage: s}
}

// ListEnabledMonitorsWithTemplates 查询所有启用的监测项（含模板数据）
// 实现 config.V1Storage 接口
func (a *PostgresV1ConfigAdapter) ListEnabledMonitorsWithTemplates(ctx context.Context) ([]*config.V1MonitorRecord, error) {
	// 使用 storage 层的查询方法
	monitors, err := a.storage.ListEnabledMonitorsWithTemplatesInternal(ctx)
	if err != nil {
		return nil, err
	}

	// 转换为 config 包的类型
	result := make([]*config.V1MonitorRecord, len(monitors))
	for i, m := range monitors {
		record := &config.V1MonitorRecord{
			ID:                  m.ID,
			Provider:            m.Provider,
			ProviderName:        m.ProviderName,
			Service:             m.Service,
			ServiceName:         m.ServiceName,
			Channel:             m.Channel,
			ChannelName:         m.ChannelName,
			Model:               m.Model,
			TemplateID:          m.TemplateID,
			URL:                 m.URL,
			Method:              m.Method,
			Headers:             m.Headers,
			Body:                m.Body,
			SuccessContains:     m.SuccessContains,
			IntervalOverride:    m.IntervalOverride,
			TimeoutOverride:     m.TimeoutOverride,
			SlowLatencyOverride: m.SlowLatencyOverride,
			Enabled:             m.Enabled,
			BoardID:             m.BoardID,
			Metadata:            m.Metadata,
			OwnerUserID:         m.OwnerUserID,
			VendorType:          m.VendorType,
			WebsiteURL:          m.WebsiteURL,
			CreatedAt:           m.CreatedAt,
			UpdatedAt:           m.UpdatedAt,
		}

		// 转换模板数据
		if m.TemplateServiceID != nil {
			record.Template = &config.V1TemplateRecord{
				ServiceID:          *m.TemplateServiceID,
				RequestMethod:      stringPtrValue(m.TemplateRequestMethod),
				BaseRequestHeaders: m.TemplateBaseRequestHeaders,
				BaseRequestBody:    m.TemplateBaseRequestBody,
				BaseResponseChecks: m.TemplateBaseResponseChecks,
				TimeoutMs:          intPtrValue(m.TemplateTimeoutMs),
				SlowLatencyMs:      intPtrValue(m.TemplateSlowLatencyMs),
			}
			if m.TemplateName != nil {
				record.Template.Name = *m.TemplateName
			}
			if m.TemplateSlug != nil {
				record.Template.Slug = *m.TemplateSlug
			}
		}

		result[i] = record
	}

	return result, nil
}

// GetMonitorSecretByV1ID 按 v1 监测项 ID 获取 API Key 密文
// 实现 config.V1Storage 接口
func (a *PostgresV1ConfigAdapter) GetMonitorSecretByV1ID(ctx context.Context, monitorID int) (*config.MonitorSecretRecord, error) {
	secret, err := a.storage.GetMonitorSecretByV1ID(ctx, monitorID)
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

// GetGlobalSetting 获取全局设置
// 实现 config.V1Storage 接口
func (a *PostgresV1ConfigAdapter) GetGlobalSetting(key string) (*config.GlobalSettingRecord, error) {
	setting, err := a.storage.GetGlobalSetting(key)
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

// stringPtrValue 安全地获取字符串指针的值
func stringPtrValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// intPtrValue 安全地获取整数指针的值
func intPtrValue(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}
