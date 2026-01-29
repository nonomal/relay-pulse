package storage

import (
	"context"
)

// =====================================================
// v1.0 存储接口定义
// =====================================================

// UserStorage 用户存储接口
type UserStorage interface {
	// 用户 CRUD
	CreateUser(ctx context.Context, user *User) error
	GetUserByID(ctx context.Context, id string) (*User, error)
	GetUserByGitHubID(ctx context.Context, githubID int64) (*User, error)
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	UpdateUser(ctx context.Context, user *User) error
	ListUsers(ctx context.Context, opts *ListUsersOptions) ([]*User, int, error)

	// 会话管理
	CreateSession(ctx context.Context, session *UserSession) error
	GetSessionByTokenHash(ctx context.Context, tokenHash string) (*UserSession, error)
	TouchSession(ctx context.Context, id string) error  // 更新最后活跃时间
	RevokeSession(ctx context.Context, id string) error // 撤销会话
	DeleteSession(ctx context.Context, id string) error
	DeleteExpiredSessions(ctx context.Context) (int64, error)
	DeleteUserSessions(ctx context.Context, userID string) (int64, error)
}

// ListUsersOptions 用户列表查询选项
type ListUsersOptions struct {
	Role   *UserRole   // 按角色筛选
	Status *UserStatus // 按状态筛选
	Search string      // 搜索用户名/邮箱
	Offset int
	Limit  int
}

// ServiceStorage 服务存储接口
type ServiceStorage interface {
	CreateService(ctx context.Context, service *Service) error
	GetServiceByID(ctx context.Context, id string) (*Service, error)
	UpdateService(ctx context.Context, service *Service) error
	DeleteService(ctx context.Context, id string) error
	ListServices(ctx context.Context, opts *ListServicesOptions) ([]*Service, error)
}

// ListServicesOptions 服务列表查询选项
type ListServicesOptions struct {
	Status    string // 按状态筛选
	WithStats bool   // 是否包含统计信息
}

// TemplateStorage 模板存储接口
type TemplateStorage interface {
	// 模板 CRUD
	CreateTemplate(ctx context.Context, template *MonitorTemplate) error
	GetTemplateByID(ctx context.Context, id int) (*MonitorTemplate, error)
	GetTemplateBySlug(ctx context.Context, serviceID, slug string) (*MonitorTemplate, error)
	UpdateTemplate(ctx context.Context, template *MonitorTemplate) error
	DeleteTemplate(ctx context.Context, id int) error
	ListTemplates(ctx context.Context, opts *ListTemplatesOptions) ([]*MonitorTemplate, int, error)

	// 模板模型 CRUD
	CreateTemplateModel(ctx context.Context, model *MonitorTemplateModel) error
	GetTemplateModelByID(ctx context.Context, id int) (*MonitorTemplateModel, error)
	UpdateTemplateModel(ctx context.Context, model *MonitorTemplateModel) error
	DeleteTemplateModel(ctx context.Context, id int) error
	ListTemplateModels(ctx context.Context, templateID int) ([]*MonitorTemplateModel, error)

	// 获取模板及其所有模型（用于申请测试）
	GetTemplateWithModels(ctx context.Context, id int) (*MonitorTemplate, error)
}

// ListTemplatesOptions 模板列表查询选项
type ListTemplatesOptions struct {
	ServiceID  string // 按服务筛选
	IsDefault  *bool  // 按默认状态筛选
	WithModels bool   // 是否包含模型列表
	Offset     int
	Limit      int
}

// MonitorStorageV1 监测项存储接口（v1.0）
type MonitorStorageV1 interface {
	// 监测项 CRUD
	CreateMonitor(ctx context.Context, monitor *Monitor) error
	GetMonitorByID(ctx context.Context, id int) (*Monitor, error)
	GetMonitorByKey(ctx context.Context, provider, service, channel string) (*Monitor, error)
	UpdateMonitor(ctx context.Context, monitor *Monitor) error
	DeleteMonitor(ctx context.Context, id int) error
	ListMonitors(ctx context.Context, opts *ListMonitorsOptions) ([]*Monitor, int, error)

	// 批量操作
	BatchUpdateMonitors(ctx context.Context, ids []int, updates map[string]interface{}) (int64, error)
	BatchDeleteMonitors(ctx context.Context, ids []int) (int64, error)

	// ID 映射（迁移用）
	CreateIDMapping(ctx context.Context, mapping *MonitorIDMapping) error
	GetNewIDByOldID(ctx context.Context, oldID int) (int, error)
	GetOldIDByNewID(ctx context.Context, newID int) (int, error)
	GetNewIDByLegacyKey(ctx context.Context, provider, service, channel string) (int, error)
}

// ListMonitorsOptions 监测项列表查询选项
type ListMonitorsOptions struct {
	Provider    string  // 按服务商筛选
	Service     string  // 按服务筛选
	Channel     string  // 按通道筛选
	BoardID     *int    // 按板块筛选
	OwnerUserID *string // 按所有者筛选
	Enabled     *bool   // 按启用状态筛选
	Search      string  // 搜索服务商名称
	Offset      int
	Limit       int
}

// ApplicationStorage 申请存储接口
type ApplicationStorage interface {
	// 申请 CRUD
	CreateApplication(ctx context.Context, app *MonitorApplication) error
	GetApplicationByID(ctx context.Context, id int) (*MonitorApplication, error)
	UpdateApplication(ctx context.Context, app *MonitorApplication) error
	DeleteApplication(ctx context.Context, id int) error
	ListApplications(ctx context.Context, opts *ListApplicationsOptions) ([]*MonitorApplication, int, error)

	// 测试会话
	CreateTestSession(ctx context.Context, session *ApplicationTestSession) error
	GetTestSessionByID(ctx context.Context, id int) (*ApplicationTestSession, error)
	UpdateTestSession(ctx context.Context, session *ApplicationTestSession) error
	ListTestSessions(ctx context.Context, applicationID int) ([]*ApplicationTestSession, error)

	// 测试结果
	CreateTestResult(ctx context.Context, result *ApplicationTestResult) error
	ListTestResults(ctx context.Context, sessionID int) ([]*ApplicationTestResult, error)

	// 获取申请详情（包含关联数据）
	GetApplicationWithDetails(ctx context.Context, id int) (*MonitorApplication, error)
}

// ListApplicationsOptions 申请列表查询选项
type ListApplicationsOptions struct {
	ApplicantUserID *string            // 按申请人筛选
	ServiceID       string             // 按服务筛选
	Status          *ApplicationStatus // 按状态筛选
	VendorType      *VendorType        // 按服务商类型筛选
	Search          string             // 搜索服务商名称
	Offset          int
	Limit           int
}

// AuditLogStorage 审计日志存储接口
type AuditLogStorage interface {
	CreateAuditLog(ctx context.Context, log *AdminAuditLog) error
	ListAuditLogs(ctx context.Context, opts *ListAuditLogsOptions) ([]*AdminAuditLog, int, error)
}

// ListAuditLogsOptions 审计日志列表查询选项
type ListAuditLogsOptions struct {
	UserID       *string            // 按操作者筛选
	Action       *AuditAction       // 按操作类型筛选
	ResourceType *AuditResourceType // 按资源类型筛选
	ResourceID   string             // 按资源 ID 筛选
	StartTime    *int64             // 开始时间
	EndTime      *int64             // 结束时间
	Offset       int
	Limit        int
}

// =====================================================
// v1.0 聚合存储接口
// =====================================================

// AdminStorageV1 v1.0 管理存储聚合接口
type AdminStorageV1 interface {
	UserStorage
	ServiceStorage
	TemplateStorage
	MonitorStorageV1
	ApplicationStorage
	AuditLogStorage

	// 初始化 v1.0 表结构
	InitV1Tables(ctx context.Context) error

	// 数据迁移
	MigrateFromV0(ctx context.Context) error
}
