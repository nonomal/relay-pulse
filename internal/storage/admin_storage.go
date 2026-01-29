package storage

// AdminStorage 定义配置管理后台所需的存储操作接口
// 由 PostgresStorage 实现
type AdminStorage interface {
	// ===== MonitorConfig CRUD =====

	// ListMonitorConfigs 列表查询监测项配置
	ListMonitorConfigs(filter *MonitorConfigFilter) ([]*MonitorConfig, int, error)

	// CreateMonitorConfig 创建监测项配置
	CreateMonitorConfig(config *MonitorConfig) error

	// GetMonitorConfig 按 ID 获取监测项配置
	GetMonitorConfig(id int64) (*MonitorConfig, error)

	// UpdateMonitorConfig 更新监测项配置（含乐观锁）
	UpdateMonitorConfig(config *MonitorConfig) error

	// DeleteMonitorConfig 软删除监测项配置
	DeleteMonitorConfig(id int64) error

	// RestoreMonitorConfig 恢复软删除的监测项配置
	RestoreMonitorConfig(id int64) error

	// ImportMonitorConfigs 批量导入监测项配置（事务操作）
	ImportMonitorConfigs(configs []*MonitorConfig) (*ImportResult, error)

	// ===== MonitorSecret CRUD =====

	// SetMonitorSecret 设置或更新监测项密钥
	SetMonitorSecret(monitorID int64, ciphertext, nonce []byte, keyVersion, encVersion int) error

	// GetMonitorSecret 获取监测项密钥
	GetMonitorSecret(monitorID int64) (*MonitorSecret, error)

	// DeleteMonitorSecret 删除监测项密钥
	DeleteMonitorSecret(monitorID int64) error

	// ===== MonitorConfigAudit CRUD =====

	// CreateMonitorConfigAudit 创建审计记录
	CreateMonitorConfigAudit(audit *MonitorConfigAudit) error

	// ListMonitorConfigAudits 查询审计记录列表
	ListMonitorConfigAudits(filter *AuditFilter) ([]*MonitorConfigAudit, int, error)

	// ===== ConfigMeta CRUD =====

	// GetConfigVersions 获取所有 scope 的配置版本
	GetConfigVersions() (*ConfigVersions, error)

	// ===== ProviderPolicy CRUD =====

	// ListProviderPolicies 列出所有 Provider 策略
	ListProviderPolicies() ([]*ProviderPolicy, error)

	// CreateProviderPolicy 创建 Provider 策略
	CreateProviderPolicy(policy *ProviderPolicy) error

	// DeleteProviderPolicy 删除 Provider 策略
	DeleteProviderPolicy(id int64) error

	// ===== BadgeDefinition CRUD =====

	// ListBadgeDefinitions 列出所有徽标定义
	ListBadgeDefinitions() ([]*BadgeDefinition, error)

	// CreateBadgeDefinition 创建徽标定义
	CreateBadgeDefinition(badge *BadgeDefinition) error

	// DeleteBadgeDefinition 删除徽标定义
	DeleteBadgeDefinition(id string) error

	// ===== BadgeBinding CRUD =====

	// ListBadgeBindings 列出徽标绑定
	ListBadgeBindings(filter *BadgeBindingFilter) ([]*BadgeBinding, error)

	// CreateBadgeBinding 创建徽标绑定
	CreateBadgeBinding(binding *BadgeBinding) error

	// DeleteBadgeBinding 删除徽标绑定
	DeleteBadgeBinding(id int64) error

	// ===== BoardConfig CRUD =====

	// ListBoardConfigs 列出所有 Board 配置
	ListBoardConfigs() ([]*BoardConfig, error)

	// ===== GlobalSetting CRUD =====

	// GetGlobalSetting 获取全局设置
	GetGlobalSetting(key string) (*GlobalSetting, error)

	// SetGlobalSetting 设置全局设置（upsert）
	SetGlobalSetting(key, value string) error
}
