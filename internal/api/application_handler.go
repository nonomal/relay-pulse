package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"monitor/internal/logger"
	"monitor/internal/storage"
)

// =====================================================
// 用户申请 API
// =====================================================

// ApplicationHandler 申请管理处理器
type ApplicationHandler struct {
	storage storage.Storage
}

// NewApplicationHandler 创建申请管理处理器
func NewApplicationHandler(s storage.Storage) *ApplicationHandler {
	return &ApplicationHandler{storage: s}
}

// RegisterUserRoutes 注册用户申请路由
func (h *ApplicationHandler) RegisterUserRoutes(r *gin.RouterGroup, authMiddleware *AuthMiddleware) {
	apps := r.Group("/applications")
	apps.Use(authMiddleware.RequireAuth())
	{
		apps.GET("", h.ListMyApplications)
		apps.POST("", h.CreateApplication)
		apps.GET("/:id", h.GetApplication)
		apps.PATCH("/:id", h.UpdateApplication)
		apps.DELETE("/:id", h.DeleteApplication)
		apps.POST("/:id/submit", h.SubmitApplication)
	}
}

// RegisterAdminRoutes 注册管理员申请路由
func (h *ApplicationHandler) RegisterAdminRoutes(r *gin.RouterGroup, authMiddleware *AuthMiddleware) {
	apps := r.Group("/applications")
	apps.Use(authMiddleware.RequireAdmin())
	{
		apps.GET("", h.ListAllApplications)
		apps.GET("/:id", h.GetApplicationAdmin)
		apps.POST("/:id/approve", h.ApproveApplication)
		apps.POST("/:id/reject", h.RejectApplication)
	}
}

// =====================================================
// 用户端 API
// =====================================================

// ListMyApplications 列出我的申请
// GET /api/user/applications
func (h *ApplicationHandler) ListMyApplications(c *gin.Context) {
	appStorage, ok := h.storage.(storage.ApplicationStorage)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "存储层不支持申请管理",
			"code":  "STORAGE_NOT_SUPPORTED",
		})
		return
	}

	user := GetUserFromContext(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "未登录",
			"code":  "UNAUTHORIZED",
		})
		return
	}

	opts := &storage.ListApplicationsOptions{
		ApplicantUserID: &user.ID,
	}
	if status := c.Query("status"); status != "" {
		s := storage.ApplicationStatus(status)
		opts.Status = &s
	}
	if limit := c.Query("limit"); limit != "" {
		if v, err := strconv.Atoi(limit); err == nil {
			opts.Limit = v
		}
	}
	if offset := c.Query("offset"); offset != "" {
		if v, err := strconv.Atoi(offset); err == nil {
			opts.Offset = v
		}
	}

	apps, total, err := appStorage.ListApplications(c.Request.Context(), opts)
	if err != nil {
		logger.Error("api", "查询申请列表失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "查询申请列表失败",
			"code":  "LIST_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  apps,
		"total": total,
	})
}

// CreateApplicationRequest 创建申请请求
type CreateApplicationRequest struct {
	ServiceID    string `json:"service_id" binding:"required"`
	ProviderName string `json:"provider_name" binding:"required"`
	ChannelName  string `json:"channel_name"`
	VendorType   string `json:"vendor_type" binding:"required"`
	WebsiteURL   string `json:"website_url"`
	RequestURL   string `json:"request_url" binding:"required"`
}

// CreateApplication 创建申请
// POST /api/user/applications
func (h *ApplicationHandler) CreateApplication(c *gin.Context) {
	appStorage, ok := h.storage.(storage.ApplicationStorage)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "存储层不支持申请管理",
			"code":  "STORAGE_NOT_SUPPORTED",
		})
		return
	}

	user := GetUserFromContext(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "未登录",
			"code":  "UNAUTHORIZED",
		})
		return
	}

	var req CreateApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数无效: " + err.Error(),
			"code":  "INVALID_REQUEST",
		})
		return
	}

	// 获取服务的默认模板
	serviceStorage, ok := h.storage.(storage.ServiceStorage)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "存储层不支持服务管理",
			"code":  "STORAGE_NOT_SUPPORTED",
		})
		return
	}

	service, err := serviceStorage.GetServiceByID(c.Request.Context(), req.ServiceID)
	if err != nil {
		logger.Error("api", "获取服务失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取服务失败",
			"code":  "GET_SERVICE_FAILED",
		})
		return
	}
	if service == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "服务不存在",
			"code":  "SERVICE_NOT_FOUND",
		})
		return
	}

	templateID := 0
	var templateSnapshot json.RawMessage
	if service.DefaultTemplateID != nil {
		templateID = *service.DefaultTemplateID
		// 获取模板快照
		templateStorage, ok := h.storage.(storage.TemplateStorage)
		if ok {
			template, err := templateStorage.GetTemplateWithModels(c.Request.Context(), templateID)
			if err == nil && template != nil {
				templateSnapshot, _ = json.Marshal(template)
			}
		}
	}

	now := time.Now().Unix()
	app := &storage.MonitorApplication{
		ApplicantUserID:  user.ID,
		ServiceID:        req.ServiceID,
		TemplateID:       templateID,
		TemplateSnapshot: templateSnapshot,
		ProviderName:     req.ProviderName,
		ChannelName:      req.ChannelName,
		VendorType:       storage.VendorType(req.VendorType),
		WebsiteURL:       req.WebsiteURL,
		RequestURL:       req.RequestURL,
		Status:           storage.ApplicationStatusPendingTest,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := appStorage.CreateApplication(c.Request.Context(), app); err != nil {
		logger.Error("api", "创建申请失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "创建申请失败",
			"code":  "CREATE_FAILED",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"data": app,
	})
}

// GetApplication 获取申请详情
// GET /api/user/applications/:id
func (h *ApplicationHandler) GetApplication(c *gin.Context) {
	appStorage, ok := h.storage.(storage.ApplicationStorage)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "存储层不支持申请管理",
			"code":  "STORAGE_NOT_SUPPORTED",
		})
		return
	}

	user := GetUserFromContext(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "未登录",
			"code":  "UNAUTHORIZED",
		})
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的申请 ID",
			"code":  "INVALID_ID",
		})
		return
	}

	app, err := appStorage.GetApplicationWithDetails(c.Request.Context(), id)
	if err != nil {
		logger.Error("api", "获取申请失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取申请失败",
			"code":  "GET_FAILED",
		})
		return
	}
	if app == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "申请不存在",
			"code":  "NOT_FOUND",
		})
		return
	}

	// 检查权限：只能查看自己的申请
	if app.ApplicantUserID != user.ID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "无权访问此申请",
			"code":  "FORBIDDEN",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": app,
	})
}

// UpdateApplicationRequest 更新申请请求
type UpdateApplicationRequest struct {
	ProviderName *string `json:"provider_name"`
	ChannelName  *string `json:"channel_name"`
	VendorType   *string `json:"vendor_type"`
	WebsiteURL   *string `json:"website_url"`
	RequestURL   *string `json:"request_url"`
}

// UpdateApplication 更新申请
// PATCH /api/user/applications/:id
func (h *ApplicationHandler) UpdateApplication(c *gin.Context) {
	appStorage, ok := h.storage.(storage.ApplicationStorage)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "存储层不支持申请管理",
			"code":  "STORAGE_NOT_SUPPORTED",
		})
		return
	}

	user := GetUserFromContext(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "未登录",
			"code":  "UNAUTHORIZED",
		})
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的申请 ID",
			"code":  "INVALID_ID",
		})
		return
	}

	app, err := appStorage.GetApplicationByID(c.Request.Context(), id)
	if err != nil {
		logger.Error("api", "获取申请失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取申请失败",
			"code":  "GET_FAILED",
		})
		return
	}
	if app == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "申请不存在",
			"code":  "NOT_FOUND",
		})
		return
	}

	// 检查权限
	if app.ApplicantUserID != user.ID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "无权修改此申请",
			"code":  "FORBIDDEN",
		})
		return
	}

	// 检查状态：只有待测试或测试失败状态可以修改
	if app.Status != storage.ApplicationStatusPendingTest && app.Status != storage.ApplicationStatusTestFailed {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "当前状态不允许修改",
			"code":  "STATUS_NOT_ALLOWED",
		})
		return
	}

	var req UpdateApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数无效: " + err.Error(),
			"code":  "INVALID_REQUEST",
		})
		return
	}

	// 更新字段
	if req.ProviderName != nil {
		app.ProviderName = *req.ProviderName
	}
	if req.ChannelName != nil {
		app.ChannelName = *req.ChannelName
	}
	if req.VendorType != nil {
		app.VendorType = storage.VendorType(*req.VendorType)
	}
	if req.WebsiteURL != nil {
		app.WebsiteURL = *req.WebsiteURL
	}
	if req.RequestURL != nil {
		app.RequestURL = *req.RequestURL
	}
	app.UpdatedAt = time.Now().Unix()

	if err := appStorage.UpdateApplication(c.Request.Context(), app); err != nil {
		logger.Error("api", "更新申请失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "更新申请失败",
			"code":  "UPDATE_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": app,
	})
}

// DeleteApplication 删除申请
// DELETE /api/user/applications/:id
func (h *ApplicationHandler) DeleteApplication(c *gin.Context) {
	appStorage, ok := h.storage.(storage.ApplicationStorage)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "存储层不支持申请管理",
			"code":  "STORAGE_NOT_SUPPORTED",
		})
		return
	}

	user := GetUserFromContext(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "未登录",
			"code":  "UNAUTHORIZED",
		})
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的申请 ID",
			"code":  "INVALID_ID",
		})
		return
	}

	app, err := appStorage.GetApplicationByID(c.Request.Context(), id)
	if err != nil {
		logger.Error("api", "获取申请失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取申请失败",
			"code":  "GET_FAILED",
		})
		return
	}
	if app == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "申请不存在",
			"code":  "NOT_FOUND",
		})
		return
	}

	// 检查权限
	if app.ApplicantUserID != user.ID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "无权删除此申请",
			"code":  "FORBIDDEN",
		})
		return
	}

	// 检查状态：已通过的申请不能删除
	if app.Status == storage.ApplicationStatusApproved {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "已通过的申请不能删除",
			"code":  "STATUS_NOT_ALLOWED",
		})
		return
	}

	if err := appStorage.DeleteApplication(c.Request.Context(), id); err != nil {
		logger.Error("api", "删除申请失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "删除申请失败",
			"code":  "DELETE_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "申请已删除",
	})
}

// SubmitApplication 提交申请审核
// POST /api/user/applications/:id/submit
func (h *ApplicationHandler) SubmitApplication(c *gin.Context) {
	appStorage, ok := h.storage.(storage.ApplicationStorage)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "存储层不支持申请管理",
			"code":  "STORAGE_NOT_SUPPORTED",
		})
		return
	}

	user := GetUserFromContext(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "未登录",
			"code":  "UNAUTHORIZED",
		})
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的申请 ID",
			"code":  "INVALID_ID",
		})
		return
	}

	app, err := appStorage.GetApplicationByID(c.Request.Context(), id)
	if err != nil {
		logger.Error("api", "获取申请失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取申请失败",
			"code":  "GET_FAILED",
		})
		return
	}
	if app == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "申请不存在",
			"code":  "NOT_FOUND",
		})
		return
	}

	// 检查权限
	if app.ApplicantUserID != user.ID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "无权提交此申请",
			"code":  "FORBIDDEN",
		})
		return
	}

	// 检查状态：只有测试通过状态可以提交
	if app.Status != storage.ApplicationStatusTestPassed {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请先完成测试",
			"code":  "TEST_REQUIRED",
		})
		return
	}

	// 更新状态为待审核
	app.Status = storage.ApplicationStatusPendingReview
	app.UpdatedAt = time.Now().Unix()

	if err := appStorage.UpdateApplication(c.Request.Context(), app); err != nil {
		logger.Error("api", "提交申请失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "提交申请失败",
			"code":  "SUBMIT_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":    app,
		"message": "申请已提交审核",
	})
}

// =====================================================
// 管理员端 API
// =====================================================

// ListAllApplications 列出所有申请（管理员）
// GET /api/admin/applications
func (h *ApplicationHandler) ListAllApplications(c *gin.Context) {
	appStorage, ok := h.storage.(storage.ApplicationStorage)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "存储层不支持申请管理",
			"code":  "STORAGE_NOT_SUPPORTED",
		})
		return
	}

	opts := &storage.ListApplicationsOptions{}
	if status := c.Query("status"); status != "" {
		s := storage.ApplicationStatus(status)
		opts.Status = &s
	}
	if serviceID := c.Query("service_id"); serviceID != "" {
		opts.ServiceID = serviceID
	}
	if vendorType := c.Query("vendor_type"); vendorType != "" {
		v := storage.VendorType(vendorType)
		opts.VendorType = &v
	}
	if search := c.Query("search"); search != "" {
		opts.Search = search
	}
	if limit := c.Query("limit"); limit != "" {
		if v, err := strconv.Atoi(limit); err == nil {
			opts.Limit = v
		}
	}
	if offset := c.Query("offset"); offset != "" {
		if v, err := strconv.Atoi(offset); err == nil {
			opts.Offset = v
		}
	}

	apps, total, err := appStorage.ListApplications(c.Request.Context(), opts)
	if err != nil {
		logger.Error("api", "查询申请列表失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "查询申请列表失败",
			"code":  "LIST_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  apps,
		"total": total,
	})
}

// GetApplicationAdmin 获取申请详情（管理员）
// GET /api/admin/applications/:id
func (h *ApplicationHandler) GetApplicationAdmin(c *gin.Context) {
	appStorage, ok := h.storage.(storage.ApplicationStorage)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "存储层不支持申请管理",
			"code":  "STORAGE_NOT_SUPPORTED",
		})
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的申请 ID",
			"code":  "INVALID_ID",
		})
		return
	}

	app, err := appStorage.GetApplicationWithDetails(c.Request.Context(), id)
	if err != nil {
		logger.Error("api", "获取申请失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取申请失败",
			"code":  "GET_FAILED",
		})
		return
	}
	if app == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "申请不存在",
			"code":  "NOT_FOUND",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": app,
	})
}

// ApproveApplication 通过申请
// POST /api/admin/applications/:id/approve
func (h *ApplicationHandler) ApproveApplication(c *gin.Context) {
	appStorage, ok := h.storage.(storage.ApplicationStorage)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "存储层不支持申请管理",
			"code":  "STORAGE_NOT_SUPPORTED",
		})
		return
	}

	user := GetUserFromContext(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "未登录",
			"code":  "UNAUTHORIZED",
		})
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的申请 ID",
			"code":  "INVALID_ID",
		})
		return
	}

	app, err := appStorage.GetApplicationByID(c.Request.Context(), id)
	if err != nil {
		logger.Error("api", "获取申请失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取申请失败",
			"code":  "GET_FAILED",
		})
		return
	}
	if app == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "申请不存在",
			"code":  "NOT_FOUND",
		})
		return
	}

	// 检查状态
	if app.Status != storage.ApplicationStatusPendingReview {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "当前状态不允许审核",
			"code":  "STATUS_NOT_ALLOWED",
		})
		return
	}

	now := time.Now().Unix()
	app.Status = storage.ApplicationStatusApproved
	app.ReviewerUserID = &user.ID
	app.ReviewedAt = &now
	app.UpdatedAt = now

	if err := appStorage.UpdateApplication(c.Request.Context(), app); err != nil {
		logger.Error("api", "审核申请失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "审核申请失败",
			"code":  "APPROVE_FAILED",
		})
		return
	}

	// TODO: 创建监测项

	// 记录审计日志
	h.logAudit(c, "approve", "application", strconv.Itoa(id), nil, app)

	c.JSON(http.StatusOK, gin.H{
		"data":    app,
		"message": "申请已通过",
	})
}

// RejectApplicationRequest 拒绝申请请求
type RejectApplicationRequest struct {
	Reason string `json:"reason" binding:"required"`
}

// RejectApplication 拒绝申请
// POST /api/admin/applications/:id/reject
func (h *ApplicationHandler) RejectApplication(c *gin.Context) {
	appStorage, ok := h.storage.(storage.ApplicationStorage)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "存储层不支持申请管理",
			"code":  "STORAGE_NOT_SUPPORTED",
		})
		return
	}

	user := GetUserFromContext(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "未登录",
			"code":  "UNAUTHORIZED",
		})
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的申请 ID",
			"code":  "INVALID_ID",
		})
		return
	}

	var req RejectApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数无效: " + err.Error(),
			"code":  "INVALID_REQUEST",
		})
		return
	}

	app, err := appStorage.GetApplicationByID(c.Request.Context(), id)
	if err != nil {
		logger.Error("api", "获取申请失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取申请失败",
			"code":  "GET_FAILED",
		})
		return
	}
	if app == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "申请不存在",
			"code":  "NOT_FOUND",
		})
		return
	}

	// 检查状态
	if app.Status != storage.ApplicationStatusPendingReview {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "当前状态不允许审核",
			"code":  "STATUS_NOT_ALLOWED",
		})
		return
	}

	now := time.Now().Unix()
	app.Status = storage.ApplicationStatusRejected
	app.RejectReason = req.Reason
	app.ReviewerUserID = &user.ID
	app.ReviewedAt = &now
	app.UpdatedAt = now

	if err := appStorage.UpdateApplication(c.Request.Context(), app); err != nil {
		logger.Error("api", "拒绝申请失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "拒绝申请失败",
			"code":  "REJECT_FAILED",
		})
		return
	}

	// 记录审计日志
	h.logAudit(c, "reject", "application", strconv.Itoa(id), nil, app)

	c.JSON(http.StatusOK, gin.H{
		"data":    app,
		"message": "申请已拒绝",
	})
}

// logAudit 记录审计日志
func (h *ApplicationHandler) logAudit(c *gin.Context, action, resourceType, resourceID string, before, after interface{}) {
	auditStorage, ok := h.storage.(storage.AuditLogStorage)
	if !ok {
		return
	}

	user := GetUserFromContext(c)
	var userID *string
	if user != nil {
		userID = &user.ID
	}

	log := &storage.AdminAuditLog{
		UserID:       userID,
		Action:       storage.AuditAction(action),
		ResourceType: storage.AuditResourceType(resourceType),
		ResourceID:   resourceID,
		IPAddress:    getClientIP(c),
		UserAgent:    c.GetHeader("User-Agent"),
		CreatedAt:    time.Now().Unix(),
	}

	if before != nil || after != nil {
		changes := make(map[string]interface{})
		if before != nil {
			changes["before"] = before
		}
		if after != nil {
			changes["after"] = after
		}
		changesJSON, err := json.Marshal(changes)
		if err == nil {
			log.Changes = changesJSON
		}
	}

	if err := auditStorage.CreateAuditLog(c.Request.Context(), log); err != nil {
		logger.Warn("api", "记录审计日志失败", "error", err)
	}
}
