package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"monitor/internal/config"
	"monitor/internal/logger"
	"monitor/internal/selftest"
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
		apps.POST("/:id/test", h.TestApplication)
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

// SetAPIKeyRequest 设置 API Key 请求（用于测试前设置）
type SetAPIKeyRequest struct {
	APIKey string `json:"api_key" binding:"required"`
}

// TestApplication 测试申请配置
// POST /api/user/applications/:id/test
func (h *ApplicationHandler) TestApplication(c *gin.Context) {
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
			"error": "无权测试此申请",
			"code":  "FORBIDDEN",
		})
		return
	}

	// 检查状态：只有待测试或测试失败状态可以测试
	if app.Status != storage.ApplicationStatusPendingTest && app.Status != storage.ApplicationStatusTestFailed {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "当前状态不允许测试",
			"code":  "STATUS_NOT_ALLOWED",
		})
		return
	}

	// 解析请求中的 API Key（可选，用于更新 API Key）
	var req SetAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err == nil && req.APIKey != "" {
		// 加密并保存新的 API Key
		encResult, err := config.EncryptAPIKey(req.APIKey, int64(app.ID), 0)
		if err != nil {
			logger.Error("api", "加密 API Key 失败", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "加密 API Key 失败",
				"code":  "ENCRYPT_FAILED",
			})
			return
		}
		app.APIKeyEncrypted = encResult.Ciphertext
		app.APIKeyNonce = encResult.Nonce
		app.APIKeyVersion = encResult.KeyVersion
		app.UpdatedAt = time.Now().Unix()
		if err := appStorage.UpdateApplication(c.Request.Context(), app); err != nil {
			logger.Error("api", "保存 API Key 失败", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "保存 API Key 失败",
				"code":  "SAVE_FAILED",
			})
			return
		}
	}

	// 检查是否有有效的 API Key（需要密文、nonce 和版本号）
	if len(app.APIKeyEncrypted) == 0 || len(app.APIKeyNonce) == 0 || app.APIKeyVersion <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请先设置 API Key",
			"code":  "API_KEY_REQUIRED",
		})
		return
	}

	// SSRF/URL 安全校验（HTTPS/域名/私网）
	guard := selftest.NewSSRFGuard()
	if err := guard.ValidateURL(app.RequestURL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求 URL 不安全: " + err.Error(),
			"code":  "INVALID_REQUEST_URL",
		})
		return
	}

	// 解密 API Key
	apiKey, err := config.DecryptAPIKey(app.APIKeyEncrypted, app.APIKeyNonce, int64(app.ID), app.APIKeyVersion)
	if err != nil {
		logger.Error("api", "解密 API Key 失败", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "API Key 已失效，请重新设置",
			"code":  "DECRYPT_FAILED",
		})
		return
	}

	// 解析模板快照
	var templateSnapshot storage.MonitorTemplate
	if len(app.TemplateSnapshot) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "模板快照缺失，请重新保存申请",
			"code":  "TEMPLATE_SNAPSHOT_MISSING",
		})
		return
	}
	if err := json.Unmarshal(app.TemplateSnapshot, &templateSnapshot); err != nil {
		logger.Error("api", "解析模板快照失败", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "模板快照已损坏，请重新保存申请",
			"code":  "PARSE_TEMPLATE_FAILED",
		})
		return
	}

	// 检查是否有启用的模型
	enabledModels := 0
	for _, m := range templateSnapshot.Models {
		if m.Enabled {
			enabledModels++
		}
	}
	if enabledModels == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "模板没有配置可用的测试模型",
			"code":  "NO_MODELS",
		})
		return
	}

	// 创建测试会话
	now := time.Now().Unix()
	session := &storage.ApplicationTestSession{
		ApplicationID:    app.ID,
		TemplateSnapshot: app.TemplateSnapshot,
		Status:           storage.TestSessionStatusRunning,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := appStorage.CreateTestSession(c.Request.Context(), session); err != nil {
		logger.Error("api", "创建测试会话失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "创建测试会话失败",
			"code":  "CREATE_SESSION_FAILED",
		})
		return
	}

	// 执行测试
	results, summary := h.executeTests(c.Request.Context(), app, &templateSnapshot, apiKey, session.ID, appStorage)

	// 更新测试会话
	summaryJSON, _ := json.Marshal(summary)
	session.Status = storage.TestSessionStatusDone
	session.Summary = summaryJSON
	session.UpdatedAt = time.Now().Unix()
	if err := appStorage.UpdateTestSession(c.Request.Context(), session); err != nil {
		logger.Warn("api", "更新测试会话失败", "error", err)
	}

	// 更新申请状态
	app.LastTestSessionID = &session.ID
	app.UpdatedAt = time.Now().Unix()
	if summary.Failed == 0 && summary.Total > 0 {
		app.Status = storage.ApplicationStatusTestPassed
	} else {
		app.Status = storage.ApplicationStatusTestFailed
	}
	if err := appStorage.UpdateApplication(c.Request.Context(), app); err != nil {
		logger.Warn("api", "更新申请状态失败", "error", err)
	}

	// 返回测试结果
	session.Results = results
	c.JSON(http.StatusOK, gin.H{
		"data":    session,
		"status":  app.Status,
		"message": getTestResultMessage(summary),
	})
}

// executeTests 执行多模型测试
func (h *ApplicationHandler) executeTests(
	ctx context.Context,
	app *storage.MonitorApplication,
	template *storage.MonitorTemplate,
	apiKey string,
	sessionID int,
	appStorage storage.ApplicationStorage,
) ([]storage.ApplicationTestResult, *storage.TestSummary) {
	var results []storage.ApplicationTestResult
	summary := &storage.TestSummary{
		Total: len(template.Models),
	}
	totalLatency := 0

	// 创建测试探测器
	guard := selftest.NewSSRFGuard()
	prober := selftest.NewSelfTestProber(guard, 0)

	for _, model := range template.Models {
		if !model.Enabled {
			summary.Total-- // 跳过禁用的模型
			continue
		}

		// 构建测试配置
		testConfig := h.buildTestConfig(app, template, &model, apiKey)

		// 执行探测
		probeResult := prober.Probe(ctx, testConfig)

		// 构建测试结果
		result := storage.ApplicationTestResult{
			SessionID:       sessionID,
			TemplateModelID: model.ID,
			ModelKey:        model.ModelKey,
			LatencyMs:       &probeResult.Latency,
			HTTPCode:        &probeResult.HTTPCode,
			CheckedAt:       time.Now().Unix(),
		}

		// 构建请求快照（脱敏）
		reqSnapshot := map[string]interface{}{
			"url":     sanitizeURL(testConfig.URL),
			"method":  testConfig.Method,
			"headers": sanitizeHeaders(testConfig.Headers),
		}
		result.RequestSnapshot, _ = json.Marshal(reqSnapshot)

		// 构建响应快照（脱敏）
		respSnapshot := map[string]interface{}{
			"status_code": probeResult.HTTPCode,
			"snippet":     truncateString(probeResult.ResponseSnippet, 512),
		}
		result.ResponseSnapshot, _ = json.Marshal(respSnapshot)

		// 判断测试结果
		if probeResult.Status == 1 || probeResult.Status == 2 {
			result.Status = storage.TestResultStatusPass
			summary.Passed++
			totalLatency += probeResult.Latency
		} else {
			result.Status = storage.TestResultStatusFail
			summary.Failed++
			if probeResult.Err != nil {
				result.ErrorMessage = probeResult.Err.Error()
			} else {
				result.ErrorMessage = probeResult.SubStatus
			}
		}

		// 保存测试结果
		if err := appStorage.CreateTestResult(ctx, &result); err != nil {
			logger.Warn("api", "保存测试结果失败", "error", err, "model", model.ModelKey)
		}

		results = append(results, result)
	}

	// 计算平均延迟
	if summary.Passed > 0 {
		summary.AvgLatencyMs = totalLatency / summary.Passed
	}

	return results, summary
}

// buildTestConfig 构建测试配置
func (h *ApplicationHandler) buildTestConfig(
	app *storage.MonitorApplication,
	template *storage.MonitorTemplate,
	model *storage.MonitorTemplateModel,
	apiKey string,
) *config.ServiceConfig {
	// 基础配置
	cfg := &config.ServiceConfig{
		Provider: app.ProviderName,
		Service:  app.ServiceID,
		URL:      app.RequestURL,
		Method:   template.RequestMethod,
		APIKey:   apiKey,
	}

	// 合并请求头：模板基础头 + API Key 替换
	headers := make(map[string]string)
	if len(template.BaseRequestHeaders) > 0 {
		_ = json.Unmarshal(template.BaseRequestHeaders, &headers)
	}
	// 替换 API Key 占位符
	for k, v := range headers {
		if v == "{{API_KEY}}" {
			headers[k] = apiKey
		} else if v == "Bearer {{API_KEY}}" {
			headers[k] = "Bearer " + apiKey
		}
	}
	cfg.Headers = headers

	// 合并请求体：模板基础体 + 模型覆盖
	var body map[string]interface{}
	if len(template.BaseRequestBody) > 0 {
		_ = json.Unmarshal(template.BaseRequestBody, &body)
	}
	if body == nil {
		body = make(map[string]interface{})
	}
	// 应用模型覆盖
	if len(model.RequestBodyOverrides) > 0 {
		var overrides map[string]interface{}
		if err := json.Unmarshal(model.RequestBodyOverrides, &overrides); err == nil {
			for k, v := range overrides {
				body[k] = v
			}
		}
	}
	bodyJSON, _ := json.Marshal(body)
	cfg.Body = string(bodyJSON)

	// 响应检查：模型覆盖 > 模板基础
	var checks struct {
		SuccessContains string `json:"success_contains"`
	}
	if len(model.ResponseChecksOverrides) > 0 {
		_ = json.Unmarshal(model.ResponseChecksOverrides, &checks)
	}
	if checks.SuccessContains == "" && len(template.BaseResponseChecks) > 0 {
		_ = json.Unmarshal(template.BaseResponseChecks, &checks)
	}
	cfg.SuccessContains = checks.SuccessContains

	// 超时和慢响应阈值
	cfg.SlowLatencyDuration = time.Duration(template.SlowLatencyMs) * time.Millisecond

	return cfg
}

// sanitizeHeaders 脱敏请求头
func sanitizeHeaders(headers map[string]string) map[string]string {
	result := make(map[string]string)
	sensitiveKeys := []string{"authorization", "x-api-key", "api-key", "x-goog-api-key", "cookie"}
	for k, v := range headers {
		lowerKey := strings.ToLower(k)
		isSensitive := false
		for _, sk := range sensitiveKeys {
			if strings.Contains(lowerKey, sk) {
				isSensitive = true
				break
			}
		}
		if isSensitive {
			result[k] = "***REDACTED***"
		} else {
			result[k] = v
		}
	}
	return result
}

// sanitizeURL 脱敏 URL 中的查询参数（避免泄露 api_key/token 等）
func sanitizeURL(raw string) string {
	if idx := strings.Index(raw, "?"); idx >= 0 {
		return raw[:idx] + "?***REDACTED***"
	}
	return raw
}

// truncateString 截断字符串
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...(truncated)"
}

// getTestResultMessage 获取测试结果消息
func getTestResultMessage(summary *storage.TestSummary) string {
	if summary.Failed == 0 && summary.Total > 0 {
		return "所有测试通过，可以提交审核"
	}
	return "部分测试失败，请检查配置后重试"
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

	// 创建监测项
	monitor, err := h.createMonitorFromApplication(c.Request.Context(), app)
	if err != nil {
		logger.Error("api", "创建监测项失败", "error", err, "application_id", app.ID)
		// 审批已成功，但监测项创建失败，返回警告
		c.JSON(http.StatusOK, gin.H{
			"data":    app,
			"message": "申请已通过，但监测项创建失败: " + err.Error(),
			"warning": true,
		})
		return
	}

	// 记录审计日志
	h.logAudit(c, "approve", "application", strconv.Itoa(id), nil, app)
	h.logAudit(c, "create", "monitor", strconv.Itoa(monitor.ID), nil, monitor)

	c.JSON(http.StatusOK, gin.H{
		"data":       app,
		"monitor_id": monitor.ID,
		"message":    "申请已通过，监测项已创建",
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

// createMonitorFromApplication 从申请创建监测项
func (h *ApplicationHandler) createMonitorFromApplication(ctx context.Context, app *storage.MonitorApplication) (*storage.Monitor, error) {
	// 获取监测项存储接口
	monitorStorage, ok := h.storage.(storage.MonitorStorageV1)
	if !ok {
		return nil, fmt.Errorf("存储层不支持监测项管理")
	}

	// 生成 provider 标识（使用服务商名称的小写形式，替换特殊字符）
	provider := generateProviderID(app.ProviderName)

	// 生成 channel 标识（如果有通道名称）
	channel := ""
	if app.ChannelName != "" {
		channel = generateProviderID(app.ChannelName)
	}

	// 检查是否已存在同样的监测项
	existing, err := monitorStorage.GetMonitorByKey(ctx, provider, app.ServiceID, channel)
	if err != nil {
		return nil, fmt.Errorf("检查监测项唯一性失败: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("监测项 %s/%s/%s 已存在", provider, app.ServiceID, channel)
	}

	now := time.Now().Unix()

	// 创建监测项
	monitor := &storage.Monitor{
		Provider:      provider,
		ProviderName:  app.ProviderName,
		Service:       app.ServiceID,
		ServiceName:   "", // 由模板或服务表获取
		Channel:       channel,
		ChannelName:   app.ChannelName,
		TemplateID:    &app.TemplateID,
		URL:           app.RequestURL,
		Enabled:       true,
		OwnerUserID:   &app.ApplicantUserID,
		VendorType:    string(app.VendorType),
		WebsiteURL:    app.WebsiteURL,
		ApplicationID: &app.ID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := monitorStorage.CreateMonitor(ctx, monitor); err != nil {
		return nil, fmt.Errorf("创建监测项失败: %w", err)
	}

	// 保存 API Key 到 monitor_secrets（如果有的话）
	// 注意：API Key 使用 app.ID 作为 AAD 加密，需要重新加密为 monitor.ID
	if len(app.APIKeyEncrypted) > 0 && len(app.APIKeyNonce) > 0 && app.APIKeyVersion > 0 {
		secretStorage, ok := h.storage.(storage.AdminStorage)
		if !ok {
			logger.Warn("api", "存储层不支持 AdminStorage，无法保存监测项密钥", "monitor_id", monitor.ID)
		} else {
			// 先用 app.ID 解密
			apiKey, err := config.DecryptAPIKey(app.APIKeyEncrypted, app.APIKeyNonce, int64(app.ID), app.APIKeyVersion)
			if err != nil {
				logger.Error("api", "解密申请 API Key 失败", "error", err, "application_id", app.ID)
				// 这是严重错误，但不影响监测项创建
			} else {
				// 用 monitor.ID 重新加密
				encResult, err := config.EncryptAPIKey(apiKey, int64(monitor.ID), 0)
				if err != nil {
					logger.Error("api", "重新加密 API Key 失败", "error", err, "monitor_id", monitor.ID)
				} else {
					if err := secretStorage.SetMonitorSecret(
						int64(monitor.ID),
						encResult.Ciphertext,
						encResult.Nonce,
						encResult.KeyVersion,
						encResult.EncVersion,
					); err != nil {
						logger.Error("api", "保存监测项密钥失败", "error", err, "monitor_id", monitor.ID)
					}
				}
			}
		}
	}

	return monitor, nil
}

// generateProviderID 从名称生成 provider/channel 标识
func generateProviderID(name string) string {
	// 转小写，替换空格和特殊字符
	id := strings.ToLower(name)
	id = strings.ReplaceAll(id, " ", "-")
	// 移除其他特殊字符，只保留字母、数字、连字符
	var result strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	return result.String()
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
