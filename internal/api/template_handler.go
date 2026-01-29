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
// 模板管理 API（管理员）
// =====================================================

// TemplateHandler 模板管理处理器
type TemplateHandler struct {
	storage storage.Storage
}

// NewTemplateHandler 创建模板管理处理器
func NewTemplateHandler(s storage.Storage) *TemplateHandler {
	return &TemplateHandler{storage: s}
}

// RegisterRoutes 注册模板管理路由
func (h *TemplateHandler) RegisterRoutes(r *gin.RouterGroup, authMiddleware *AuthMiddleware) {
	templates := r.Group("/templates")
	templates.Use(authMiddleware.RequireAdmin())
	{
		templates.GET("", h.ListTemplates)
		templates.POST("", h.CreateTemplate)
		templates.GET("/:id", h.GetTemplate)
		templates.PATCH("/:id", h.UpdateTemplate)
		templates.DELETE("/:id", h.DeleteTemplate)

		// 模板模型管理
		templates.GET("/:id/models", h.ListTemplateModels)
		templates.POST("/:id/models", h.CreateTemplateModel)
		templates.PATCH("/:id/models/:modelId", h.UpdateTemplateModel)
		templates.DELETE("/:id/models/:modelId", h.DeleteTemplateModel)
	}
}

// ListTemplates 列出模板
// GET /api/admin/templates
func (h *TemplateHandler) ListTemplates(c *gin.Context) {
	templateStorage, ok := h.storage.(storage.TemplateStorage)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "存储层不支持模板管理",
			"code":  "STORAGE_NOT_SUPPORTED",
		})
		return
	}

	// 解析查询参数
	opts := &storage.ListTemplatesOptions{
		ServiceID:  c.Query("service_id"),
		WithModels: c.Query("with_models") == "true",
	}
	if isDefault := c.Query("is_default"); isDefault != "" {
		val := isDefault == "true"
		opts.IsDefault = &val
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

	templates, total, err := templateStorage.ListTemplates(c.Request.Context(), opts)
	if err != nil {
		logger.Error("api", "查询模板列表失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "查询模板列表失败",
			"code":  "LIST_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  templates,
		"total": total,
	})
}

// CreateTemplateRequest 创建模板请求
type CreateTemplateRequest struct {
	ServiceID          string          `json:"service_id" binding:"required"`
	Name               string          `json:"name" binding:"required"`
	Slug               string          `json:"slug" binding:"required"`
	Description        string          `json:"description"`
	IsDefault          bool            `json:"is_default"`
	RequestMethod      string          `json:"request_method"`
	BaseRequestHeaders json.RawMessage `json:"base_request_headers"`
	BaseRequestBody    json.RawMessage `json:"base_request_body"`
	BaseResponseChecks json.RawMessage `json:"base_response_checks"`
	TimeoutMs          int             `json:"timeout_ms"`
	SlowLatencyMs      int             `json:"slow_latency_ms"`
	RetryPolicy        json.RawMessage `json:"retry_policy"`
}

// CreateTemplate 创建模板
// POST /api/admin/templates
func (h *TemplateHandler) CreateTemplate(c *gin.Context) {
	templateStorage, ok := h.storage.(storage.TemplateStorage)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "存储层不支持模板管理",
			"code":  "STORAGE_NOT_SUPPORTED",
		})
		return
	}

	var req CreateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数无效: " + err.Error(),
			"code":  "INVALID_REQUEST",
		})
		return
	}

	// 检查 slug 是否已存在
	existing, err := templateStorage.GetTemplateBySlug(c.Request.Context(), req.ServiceID, req.Slug)
	if err != nil {
		logger.Error("api", "检查模板是否存在失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "检查模板失败",
			"code":  "CHECK_FAILED",
		})
		return
	}
	if existing != nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": "模板 slug 已存在",
			"code":  "SLUG_EXISTS",
		})
		return
	}

	now := time.Now().Unix()
	user := GetUserFromContext(c)
	var createdBy *string
	if user != nil {
		createdBy = &user.ID
	}

	// 设置默认值
	requestMethod := req.RequestMethod
	if requestMethod == "" {
		requestMethod = "POST"
	}
	timeoutMs := req.TimeoutMs
	if timeoutMs == 0 {
		timeoutMs = 10000
	}
	slowLatencyMs := req.SlowLatencyMs
	if slowLatencyMs == 0 {
		slowLatencyMs = 5000
	}

	template := &storage.MonitorTemplate{
		ServiceID:          req.ServiceID,
		Name:               req.Name,
		Slug:               req.Slug,
		Description:        req.Description,
		IsDefault:          req.IsDefault,
		RequestMethod:      requestMethod,
		BaseRequestHeaders: req.BaseRequestHeaders,
		BaseRequestBody:    req.BaseRequestBody,
		BaseResponseChecks: req.BaseResponseChecks,
		TimeoutMs:          timeoutMs,
		SlowLatencyMs:      slowLatencyMs,
		RetryPolicy:        req.RetryPolicy,
		CreatedBy:          createdBy,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if err := templateStorage.CreateTemplate(c.Request.Context(), template); err != nil {
		logger.Error("api", "创建模板失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "创建模板失败",
			"code":  "CREATE_FAILED",
		})
		return
	}

	// 记录审计日志
	h.logAudit(c, "create", "template", strconv.Itoa(template.ID), nil, template)

	c.JSON(http.StatusCreated, gin.H{
		"data": template,
	})
}

// GetTemplate 获取模板详情
// GET /api/admin/templates/:id
func (h *TemplateHandler) GetTemplate(c *gin.Context) {
	templateStorage, ok := h.storage.(storage.TemplateStorage)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "存储层不支持模板管理",
			"code":  "STORAGE_NOT_SUPPORTED",
		})
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的模板 ID",
			"code":  "INVALID_ID",
		})
		return
	}

	// 获取模板及其模型
	withModels := c.Query("with_models") == "true"
	var template *storage.MonitorTemplate
	if withModels {
		template, err = templateStorage.GetTemplateWithModels(c.Request.Context(), id)
	} else {
		template, err = templateStorage.GetTemplateByID(c.Request.Context(), id)
	}

	if err != nil {
		logger.Error("api", "获取模板失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取模板失败",
			"code":  "GET_FAILED",
		})
		return
	}
	if template == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "模板不存在",
			"code":  "NOT_FOUND",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": template,
	})
}

// UpdateTemplateRequest 更新模板请求
type UpdateTemplateRequest struct {
	Name               *string          `json:"name"`
	Slug               *string          `json:"slug"`
	Description        *string          `json:"description"`
	IsDefault          *bool            `json:"is_default"`
	RequestMethod      *string          `json:"request_method"`
	BaseRequestHeaders *json.RawMessage `json:"base_request_headers"`
	BaseRequestBody    *json.RawMessage `json:"base_request_body"`
	BaseResponseChecks *json.RawMessage `json:"base_response_checks"`
	TimeoutMs          *int             `json:"timeout_ms"`
	SlowLatencyMs      *int             `json:"slow_latency_ms"`
	RetryPolicy        *json.RawMessage `json:"retry_policy"`
}

// UpdateTemplate 更新模板
// PATCH /api/admin/templates/:id
func (h *TemplateHandler) UpdateTemplate(c *gin.Context) {
	templateStorage, ok := h.storage.(storage.TemplateStorage)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "存储层不支持模板管理",
			"code":  "STORAGE_NOT_SUPPORTED",
		})
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的模板 ID",
			"code":  "INVALID_ID",
		})
		return
	}

	template, err := templateStorage.GetTemplateByID(c.Request.Context(), id)
	if err != nil {
		logger.Error("api", "获取模板失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取模板失败",
			"code":  "GET_FAILED",
		})
		return
	}
	if template == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "模板不存在",
			"code":  "NOT_FOUND",
		})
		return
	}

	var req UpdateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数无效: " + err.Error(),
			"code":  "INVALID_REQUEST",
		})
		return
	}

	// 保存旧值用于审计
	oldTemplate := *template

	// 更新字段
	if req.Name != nil {
		template.Name = *req.Name
	}
	if req.Slug != nil {
		// 检查新 slug 是否冲突
		if *req.Slug != template.Slug {
			existing, err := templateStorage.GetTemplateBySlug(c.Request.Context(), template.ServiceID, *req.Slug)
			if err != nil {
				logger.Error("api", "检查模板 slug 失败", "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "检查模板失败",
					"code":  "CHECK_FAILED",
				})
				return
			}
			if existing != nil {
				c.JSON(http.StatusConflict, gin.H{
					"error": "模板 slug 已存在",
					"code":  "SLUG_EXISTS",
				})
				return
			}
		}
		template.Slug = *req.Slug
	}
	if req.Description != nil {
		template.Description = *req.Description
	}
	if req.IsDefault != nil {
		template.IsDefault = *req.IsDefault
	}
	if req.RequestMethod != nil {
		template.RequestMethod = *req.RequestMethod
	}
	if req.BaseRequestHeaders != nil {
		template.BaseRequestHeaders = *req.BaseRequestHeaders
	}
	if req.BaseRequestBody != nil {
		template.BaseRequestBody = *req.BaseRequestBody
	}
	if req.BaseResponseChecks != nil {
		template.BaseResponseChecks = *req.BaseResponseChecks
	}
	if req.TimeoutMs != nil {
		template.TimeoutMs = *req.TimeoutMs
	}
	if req.SlowLatencyMs != nil {
		template.SlowLatencyMs = *req.SlowLatencyMs
	}
	if req.RetryPolicy != nil {
		template.RetryPolicy = *req.RetryPolicy
	}
	template.UpdatedAt = time.Now().Unix()

	if err := templateStorage.UpdateTemplate(c.Request.Context(), template); err != nil {
		logger.Error("api", "更新模板失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "更新模板失败",
			"code":  "UPDATE_FAILED",
		})
		return
	}

	// 记录审计日志
	h.logAudit(c, "update", "template", strconv.Itoa(template.ID), &oldTemplate, template)

	c.JSON(http.StatusOK, gin.H{
		"data": template,
	})
}

// DeleteTemplate 删除模板
// DELETE /api/admin/templates/:id
func (h *TemplateHandler) DeleteTemplate(c *gin.Context) {
	templateStorage, ok := h.storage.(storage.TemplateStorage)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "存储层不支持模板管理",
			"code":  "STORAGE_NOT_SUPPORTED",
		})
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的模板 ID",
			"code":  "INVALID_ID",
		})
		return
	}

	template, err := templateStorage.GetTemplateByID(c.Request.Context(), id)
	if err != nil {
		logger.Error("api", "获取模板失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取模板失败",
			"code":  "GET_FAILED",
		})
		return
	}
	if template == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "模板不存在",
			"code":  "NOT_FOUND",
		})
		return
	}

	if err := templateStorage.DeleteTemplate(c.Request.Context(), id); err != nil {
		logger.Error("api", "删除模板失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "删除模板失败",
			"code":  "DELETE_FAILED",
		})
		return
	}

	// 记录审计日志
	h.logAudit(c, "delete", "template", strconv.Itoa(id), template, nil)

	c.JSON(http.StatusOK, gin.H{
		"message": "模板已删除",
	})
}

// =====================================================
// 模板模型管理
// =====================================================

// ListTemplateModels 列出模板的模型
// GET /api/admin/templates/:id/models
func (h *TemplateHandler) ListTemplateModels(c *gin.Context) {
	templateStorage, ok := h.storage.(storage.TemplateStorage)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "存储层不支持模板管理",
			"code":  "STORAGE_NOT_SUPPORTED",
		})
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的模板 ID",
			"code":  "INVALID_ID",
		})
		return
	}

	models, err := templateStorage.ListTemplateModels(c.Request.Context(), id)
	if err != nil {
		logger.Error("api", "查询模板模型列表失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "查询模板模型列表失败",
			"code":  "LIST_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  models,
		"total": len(models),
	})
}

// CreateTemplateModelRequest 创建模板模型请求
type CreateTemplateModelRequest struct {
	ModelKey                string          `json:"model_key" binding:"required"`
	DisplayName             string          `json:"display_name" binding:"required"`
	RequestBodyOverrides    json.RawMessage `json:"request_body_overrides"`
	ResponseChecksOverrides json.RawMessage `json:"response_checks_overrides"`
	Enabled                 bool            `json:"enabled"`
	SortOrder               int             `json:"sort_order"`
}

// CreateTemplateModel 创建模板模型
// POST /api/admin/templates/:id/models
func (h *TemplateHandler) CreateTemplateModel(c *gin.Context) {
	templateStorage, ok := h.storage.(storage.TemplateStorage)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "存储层不支持模板管理",
			"code":  "STORAGE_NOT_SUPPORTED",
		})
		return
	}

	templateID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的模板 ID",
			"code":  "INVALID_ID",
		})
		return
	}

	// 检查模板是否存在
	template, err := templateStorage.GetTemplateByID(c.Request.Context(), templateID)
	if err != nil {
		logger.Error("api", "获取模板失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取模板失败",
			"code":  "GET_FAILED",
		})
		return
	}
	if template == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "模板不存在",
			"code":  "TEMPLATE_NOT_FOUND",
		})
		return
	}

	var req CreateTemplateModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数无效: " + err.Error(),
			"code":  "INVALID_REQUEST",
		})
		return
	}

	now := time.Now().Unix()
	model := &storage.MonitorTemplateModel{
		TemplateID:              templateID,
		ModelKey:                req.ModelKey,
		DisplayName:             req.DisplayName,
		RequestBodyOverrides:    req.RequestBodyOverrides,
		ResponseChecksOverrides: req.ResponseChecksOverrides,
		Enabled:                 req.Enabled,
		SortOrder:               req.SortOrder,
		CreatedAt:               now,
		UpdatedAt:               now,
	}

	if err := templateStorage.CreateTemplateModel(c.Request.Context(), model); err != nil {
		logger.Error("api", "创建模板模型失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "创建模板模型失败",
			"code":  "CREATE_FAILED",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"data": model,
	})
}

// UpdateTemplateModelRequest 更新模板模型请求
type UpdateTemplateModelRequest struct {
	ModelKey                *string          `json:"model_key"`
	DisplayName             *string          `json:"display_name"`
	RequestBodyOverrides    *json.RawMessage `json:"request_body_overrides"`
	ResponseChecksOverrides *json.RawMessage `json:"response_checks_overrides"`
	Enabled                 *bool            `json:"enabled"`
	SortOrder               *int             `json:"sort_order"`
}

// UpdateTemplateModel 更新模板模型
// PATCH /api/admin/templates/:id/models/:modelId
func (h *TemplateHandler) UpdateTemplateModel(c *gin.Context) {
	templateStorage, ok := h.storage.(storage.TemplateStorage)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "存储层不支持模板管理",
			"code":  "STORAGE_NOT_SUPPORTED",
		})
		return
	}

	modelID, err := strconv.Atoi(c.Param("modelId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的模型 ID",
			"code":  "INVALID_ID",
		})
		return
	}

	model, err := templateStorage.GetTemplateModelByID(c.Request.Context(), modelID)
	if err != nil {
		logger.Error("api", "获取模板模型失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取模板模型失败",
			"code":  "GET_FAILED",
		})
		return
	}
	if model == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "模板模型不存在",
			"code":  "NOT_FOUND",
		})
		return
	}

	var req UpdateTemplateModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数无效: " + err.Error(),
			"code":  "INVALID_REQUEST",
		})
		return
	}

	// 更新字段
	if req.ModelKey != nil {
		model.ModelKey = *req.ModelKey
	}
	if req.DisplayName != nil {
		model.DisplayName = *req.DisplayName
	}
	if req.RequestBodyOverrides != nil {
		model.RequestBodyOverrides = *req.RequestBodyOverrides
	}
	if req.ResponseChecksOverrides != nil {
		model.ResponseChecksOverrides = *req.ResponseChecksOverrides
	}
	if req.Enabled != nil {
		model.Enabled = *req.Enabled
	}
	if req.SortOrder != nil {
		model.SortOrder = *req.SortOrder
	}
	model.UpdatedAt = time.Now().Unix()

	if err := templateStorage.UpdateTemplateModel(c.Request.Context(), model); err != nil {
		logger.Error("api", "更新模板模型失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "更新模板模型失败",
			"code":  "UPDATE_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": model,
	})
}

// DeleteTemplateModel 删除模板模型
// DELETE /api/admin/templates/:id/models/:modelId
func (h *TemplateHandler) DeleteTemplateModel(c *gin.Context) {
	templateStorage, ok := h.storage.(storage.TemplateStorage)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "存储层不支持模板管理",
			"code":  "STORAGE_NOT_SUPPORTED",
		})
		return
	}

	modelID, err := strconv.Atoi(c.Param("modelId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的模型 ID",
			"code":  "INVALID_ID",
		})
		return
	}

	model, err := templateStorage.GetTemplateModelByID(c.Request.Context(), modelID)
	if err != nil {
		logger.Error("api", "获取模板模型失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取模板模型失败",
			"code":  "GET_FAILED",
		})
		return
	}
	if model == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "模板模型不存在",
			"code":  "NOT_FOUND",
		})
		return
	}

	if err := templateStorage.DeleteTemplateModel(c.Request.Context(), modelID); err != nil {
		logger.Error("api", "删除模板模型失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "删除模板模型失败",
			"code":  "DELETE_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "模板模型已删除",
	})
}

// logAudit 记录审计日志
func (h *TemplateHandler) logAudit(c *gin.Context, action, resourceType, resourceID string, before, after interface{}) {
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
