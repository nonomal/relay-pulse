package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"monitor/internal/logger"
	"monitor/internal/storage"
)

// =====================================================
// 服务管理 API（管理员）
// =====================================================

// ServiceHandler 服务管理处理器
type ServiceHandler struct {
	storage storage.Storage
}

// NewServiceHandler 创建服务管理处理器
func NewServiceHandler(s storage.Storage) *ServiceHandler {
	return &ServiceHandler{storage: s}
}

// RegisterRoutes 注册服务管理路由
func (h *ServiceHandler) RegisterRoutes(r *gin.RouterGroup, authMiddleware *AuthMiddleware) {
	services := r.Group("/services")
	services.Use(authMiddleware.RequireAdmin())
	{
		services.GET("", h.ListServices)
		services.POST("", h.CreateService)
		services.GET("/:id", h.GetService)
		services.PATCH("/:id", h.UpdateService)
		services.DELETE("/:id", h.DeleteService)
	}
}

// ListServices 列出所有服务
// GET /api/admin/services
func (h *ServiceHandler) ListServices(c *gin.Context) {
	serviceStorage, ok := h.storage.(storage.ServiceStorage)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "存储层不支持服务管理",
			"code":  "STORAGE_NOT_SUPPORTED",
		})
		return
	}

	// 解析查询参数
	opts := &storage.ListServicesOptions{
		Status: c.Query("status"),
	}

	services, err := serviceStorage.ListServices(c.Request.Context(), opts)
	if err != nil {
		logger.Error("api", "查询服务列表失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "查询服务列表失败",
			"code":  "LIST_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  services,
		"total": len(services),
	})
}

// CreateServiceRequest 创建服务请求
type CreateServiceRequest struct {
	ID                string  `json:"id" binding:"required"`
	Name              string  `json:"name" binding:"required"`
	IconSVG           *string `json:"icon_svg"`
	DefaultTemplateID *int    `json:"default_template_id"`
	Status            string  `json:"status"`
	SortOrder         int     `json:"sort_order"`
}

// CreateService 创建服务
// POST /api/admin/services
func (h *ServiceHandler) CreateService(c *gin.Context) {
	serviceStorage, ok := h.storage.(storage.ServiceStorage)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "存储层不支持服务管理",
			"code":  "STORAGE_NOT_SUPPORTED",
		})
		return
	}

	var req CreateServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数无效: " + err.Error(),
			"code":  "INVALID_REQUEST",
		})
		return
	}

	// 检查 ID 是否已存在
	existing, err := serviceStorage.GetServiceByID(c.Request.Context(), req.ID)
	if err != nil {
		logger.Error("api", "检查服务是否存在失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "检查服务失败",
			"code":  "CHECK_FAILED",
		})
		return
	}
	if existing != nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": "服务 ID 已存在",
			"code":  "ID_EXISTS",
		})
		return
	}

	now := time.Now().Unix()
	status := req.Status
	if status == "" {
		status = "active"
	}

	service := &storage.Service{
		ID:                req.ID,
		Name:              req.Name,
		IconSVG:           req.IconSVG,
		DefaultTemplateID: req.DefaultTemplateID,
		Status:            status,
		SortOrder:         req.SortOrder,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := serviceStorage.CreateService(c.Request.Context(), service); err != nil {
		logger.Error("api", "创建服务失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "创建服务失败",
			"code":  "CREATE_FAILED",
		})
		return
	}

	// 记录审计日志
	h.logAudit(c, "create", "service", service.ID, nil, service)

	c.JSON(http.StatusCreated, gin.H{
		"data": service,
	})
}

// GetService 获取服务详情
// GET /api/admin/services/:id
func (h *ServiceHandler) GetService(c *gin.Context) {
	serviceStorage, ok := h.storage.(storage.ServiceStorage)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "存储层不支持服务管理",
			"code":  "STORAGE_NOT_SUPPORTED",
		})
		return
	}

	id := c.Param("id")
	service, err := serviceStorage.GetServiceByID(c.Request.Context(), id)
	if err != nil {
		logger.Error("api", "获取服务失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取服务失败",
			"code":  "GET_FAILED",
		})
		return
	}
	if service == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "服务不存在",
			"code":  "NOT_FOUND",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": service,
	})
}

// UpdateServiceRequest 更新服务请求
type UpdateServiceRequest struct {
	Name              *string `json:"name"`
	IconSVG           *string `json:"icon_svg"`
	DefaultTemplateID *int    `json:"default_template_id"`
	Status            *string `json:"status"`
	SortOrder         *int    `json:"sort_order"`
}

// UpdateService 更新服务
// PATCH /api/admin/services/:id
func (h *ServiceHandler) UpdateService(c *gin.Context) {
	serviceStorage, ok := h.storage.(storage.ServiceStorage)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "存储层不支持服务管理",
			"code":  "STORAGE_NOT_SUPPORTED",
		})
		return
	}

	id := c.Param("id")
	service, err := serviceStorage.GetServiceByID(c.Request.Context(), id)
	if err != nil {
		logger.Error("api", "获取服务失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取服务失败",
			"code":  "GET_FAILED",
		})
		return
	}
	if service == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "服务不存在",
			"code":  "NOT_FOUND",
		})
		return
	}

	var req UpdateServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数无效: " + err.Error(),
			"code":  "INVALID_REQUEST",
		})
		return
	}

	// 保存旧值用于审计
	oldService := *service

	// 更新字段
	if req.Name != nil {
		service.Name = *req.Name
	}
	if req.IconSVG != nil {
		service.IconSVG = req.IconSVG
	}
	if req.DefaultTemplateID != nil {
		service.DefaultTemplateID = req.DefaultTemplateID
	}
	if req.Status != nil {
		service.Status = *req.Status
	}
	if req.SortOrder != nil {
		service.SortOrder = *req.SortOrder
	}
	service.UpdatedAt = time.Now().Unix()

	if err := serviceStorage.UpdateService(c.Request.Context(), service); err != nil {
		logger.Error("api", "更新服务失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "更新服务失败",
			"code":  "UPDATE_FAILED",
		})
		return
	}

	// 记录审计日志
	h.logAudit(c, "update", "service", service.ID, &oldService, service)

	c.JSON(http.StatusOK, gin.H{
		"data": service,
	})
}

// DeleteService 删除服务
// DELETE /api/admin/services/:id
func (h *ServiceHandler) DeleteService(c *gin.Context) {
	serviceStorage, ok := h.storage.(storage.ServiceStorage)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "存储层不支持服务管理",
			"code":  "STORAGE_NOT_SUPPORTED",
		})
		return
	}

	id := c.Param("id")
	service, err := serviceStorage.GetServiceByID(c.Request.Context(), id)
	if err != nil {
		logger.Error("api", "获取服务失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取服务失败",
			"code":  "GET_FAILED",
		})
		return
	}
	if service == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "服务不存在",
			"code":  "NOT_FOUND",
		})
		return
	}

	if err := serviceStorage.DeleteService(c.Request.Context(), id); err != nil {
		logger.Error("api", "删除服务失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "删除服务失败",
			"code":  "DELETE_FAILED",
		})
		return
	}

	// 记录审计日志
	h.logAudit(c, "delete", "service", id, service, nil)

	c.JSON(http.StatusOK, gin.H{
		"message": "服务已删除",
	})
}

// logAudit 记录审计日志
func (h *ServiceHandler) logAudit(c *gin.Context, action, resourceType, resourceID string, before, after interface{}) {
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

	// 构建变更内容
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
