package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"monitor/internal/config"
	"monitor/internal/logger"
	"monitor/internal/storage"
)

// AdminHandler 管理 API 处理器
// 使用 AdminStorage 接口，支持 SQLite 和 PostgreSQL
type AdminHandler struct {
	storage storage.AdminStorage
}

// NewAdminHandler 创建管理 API 处理器
func NewAdminHandler(store storage.AdminStorage) *AdminHandler {
	return &AdminHandler{
		storage: store,
	}
}

// ===== 监测项管理 API =====

// ListMonitors 列出监测项配置
// GET /api/admin/monitors
func (h *AdminHandler) ListMonitors(c *gin.Context) {
	// 解析查询参数
	filter := &storage.MonitorConfigFilter{
		Provider: c.Query("provider"),
		Service:  c.Query("service"),
		Channel:  c.Query("channel"),
		Model:    c.Query("model"),
		Search:   c.Query("search"),
	}

	// 解析 enabled 参数
	if enabledStr := c.Query("enabled"); enabledStr != "" {
		enabled := enabledStr == "true" || enabledStr == "1"
		filter.Enabled = &enabled
	}

	// 解析 include_deleted 参数
	filter.IncludeDeleted = c.Query("include_deleted") == "true"

	// 解析分页参数
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit <= 500 {
			filter.Limit = limit
		}
	}

	// 查询
	configs, total, err := h.storage.ListMonitorConfigs(filter)
	if err != nil {
		logger.FromContext(c.Request.Context(), "api").Error("列出监测项配置失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "查询失败",
		})
		return
	}

	// 转换响应（添加 API Key 状态）
	items := make([]gin.H, 0, len(configs))
	for _, cfg := range configs {
		item := h.monitorConfigToResponse(cfg)
		// 检查是否有 API Key
		secret, err := h.storage.GetMonitorSecret(cfg.ID)
		if err != nil {
			logger.FromContext(c.Request.Context(), "api").Warn("查询 API Key 状态失败",
				"error", err, "monitor_id", cfg.ID)
			item["has_api_key"] = nil // 未知状态
		} else if secret != nil {
			item["has_api_key"] = true
			// 尝试解密并脱敏
			if apiKey, err := config.DecryptAPIKey(secret.APIKeyCiphertext, secret.APIKeyNonce, cfg.ID, secret.KeyVersion); err == nil {
				item["api_key_masked"] = maskAPIKey(apiKey)
			} else {
				logger.FromContext(c.Request.Context(), "api").Warn("解密 API Key 失败",
					"error", err, "monitor_id", cfg.ID)
			}
		} else {
			item["has_api_key"] = false
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  items,
		"total": total,
		"meta": gin.H{
			"offset": filter.Offset,
			"limit":  filter.Limit,
		},
	})
}

// CreateMonitor 创建监测项配置
// POST /api/admin/monitors
func (h *AdminHandler) CreateMonitor(c *gin.Context) {
	var req CreateMonitorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("请求参数错误: %v", err),
		})
		return
	}

	// 验证必填字段
	if strings.TrimSpace(req.Provider) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider 不能为空"})
		return
	}
	if strings.TrimSpace(req.Service) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "service 不能为空"})
		return
	}

	// 序列化 ConfigPayload
	configBlob, err := json.Marshal(req.Config)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("配置序列化失败: %v", err),
		})
		return
	}

	// 创建监测项配置
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	monitorConfig := &storage.MonitorConfig{
		Provider:   req.Provider,
		Service:    req.Service,
		Channel:    req.Channel,
		Model:      req.Model,
		Name:       req.Name,
		Enabled:    enabled,
		ParentKey:  req.ParentKey,
		ConfigBlob: string(configBlob),
	}

	if err := h.storage.CreateMonitorConfig(monitorConfig); err != nil {
		if strings.Contains(err.Error(), "已存在") {
			c.JSON(http.StatusConflict, gin.H{
				"error": err.Error(),
			})
			return
		}
		logger.FromContext(c.Request.Context(), "api").Error("创建监测项配置失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "创建失败",
		})
		return
	}

	// 设置 API Key（如果提供）
	secretChanged := false
	var apiKeyWarning string
	if req.APIKey != "" {
		keyVersion, err := config.GetDefaultKeyVersion()
		if err != nil {
			logger.FromContext(c.Request.Context(), "api").Error("获取加密密钥版本失败", "error", err, "monitor_id", monitorConfig.ID)
			apiKeyWarning = "API Key 保存失败：加密密钥未配置"
		} else {
			result, err := config.EncryptAPIKey(req.APIKey, monitorConfig.ID, keyVersion)
			if err != nil {
				logger.FromContext(c.Request.Context(), "api").Error("加密 API Key 失败", "error", err, "monitor_id", monitorConfig.ID)
				apiKeyWarning = "API Key 加密失败"
			} else {
				if err := h.storage.SetMonitorSecret(monitorConfig.ID, result.Ciphertext, result.Nonce, result.KeyVersion, result.EncVersion); err != nil {
					logger.FromContext(c.Request.Context(), "api").Error("保存 API Key 失败", "error", err, "monitor_id", monitorConfig.ID)
					apiKeyWarning = "API Key 保存失败"
				} else {
					secretChanged = true
				}
			}
		}
	}

	// 记录审计日志
	actor := GetAdminActor(c)
	audit := &storage.MonitorConfigAudit{
		MonitorID:     monitorConfig.ID,
		Provider:      monitorConfig.Provider,
		Service:       monitorConfig.Service,
		Channel:       monitorConfig.Channel,
		Model:         monitorConfig.Model,
		Action:        storage.AuditActionCreate,
		AfterBlob:     string(configBlob),
		AfterVersion:  &monitorConfig.Version,
		SecretChanged: secretChanged,
		Actor:         actor.Token,
		ActorIP:       actor.IP,
		UserAgent:     actor.UserAgent,
		RequestID:     actor.RequestID,
	}
	if err := h.storage.CreateMonitorConfigAudit(audit); err != nil {
		logger.FromContext(c.Request.Context(), "api").Warn("记录审计日志失败", "error", err)
	}

	resp := gin.H{
		"data": h.monitorConfigToResponse(monitorConfig),
	}
	if apiKeyWarning != "" {
		resp["warning"] = apiKeyWarning
		resp["api_key_saved"] = false
	}
	c.JSON(http.StatusCreated, resp)
}

// GetMonitor 获取监测项配置详情
// GET /api/admin/monitors/:id
func (h *AdminHandler) GetMonitor(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	cfg, err := h.storage.GetMonitorConfig(id)
	if err != nil {
		logger.FromContext(c.Request.Context(), "api").Error("获取监测项配置失败", "error", err, "id", id)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	if cfg == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "监测项不存在"})
		return
	}

	item := h.monitorConfigToResponse(cfg)

	// 检查是否有 API Key
	secret, err := h.storage.GetMonitorSecret(cfg.ID)
	if err != nil {
		logger.FromContext(c.Request.Context(), "api").Warn("查询 API Key 状态失败",
			"error", err, "monitor_id", cfg.ID)
		item["has_api_key"] = nil
	} else if secret != nil {
		item["has_api_key"] = true
		if apiKey, err := config.DecryptAPIKey(secret.APIKeyCiphertext, secret.APIKeyNonce, cfg.ID, secret.KeyVersion); err == nil {
			item["api_key_masked"] = maskAPIKey(apiKey)
		} else {
			logger.FromContext(c.Request.Context(), "api").Warn("解密 API Key 失败",
				"error", err, "monitor_id", cfg.ID)
		}
	} else {
		item["has_api_key"] = false
	}

	c.JSON(http.StatusOK, gin.H{
		"data": item,
	})
}

// UpdateMonitor 更新监测项配置
// PUT /api/admin/monitors/:id
func (h *AdminHandler) UpdateMonitor(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	var req UpdateMonitorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("请求参数错误: %v", err),
		})
		return
	}

	// 获取现有配置
	existing, err := h.storage.GetMonitorConfig(id)
	if err != nil {
		logger.FromContext(c.Request.Context(), "api").Error("获取监测项配置失败", "error", err, "id", id)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "监测项不存在"})
		return
	}
	if existing.IsDeleted() {
		c.JSON(http.StatusGone, gin.H{"error": "监测项已被删除"})
		return
	}

	// 乐观锁检查
	if req.Version > 0 && req.Version != existing.Version {
		c.JSON(http.StatusConflict, gin.H{
			"error":           "版本冲突，请刷新后重试",
			"current_version": existing.Version,
		})
		return
	}

	// 保存变更前的配置
	beforeBlob := existing.ConfigBlob
	beforeVersion := existing.Version

	// 更新字段
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.ParentKey != nil {
		existing.ParentKey = *req.ParentKey
	}
	if req.Config != nil {
		configBlob, err := json.Marshal(req.Config)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("配置序列化失败: %v", err),
			})
			return
		}
		existing.ConfigBlob = string(configBlob)
	}

	// 执行更新
	if err := h.storage.UpdateMonitorConfig(existing); err != nil {
		if strings.Contains(err.Error(), "版本冲突") {
			c.JSON(http.StatusConflict, gin.H{
				"error": "版本冲突，请刷新后重试",
			})
			return
		}
		logger.FromContext(c.Request.Context(), "api").Error("更新监测项配置失败", "error", err, "id", id)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}

	// 更新 API Key（如果提供）
	secretChanged := false
	var apiKeyWarning string
	if req.APIKey != nil {
		if *req.APIKey == "" {
			// 删除 API Key
			if err := h.storage.DeleteMonitorSecret(id); err != nil && !strings.Contains(err.Error(), "不存在") {
				logger.FromContext(c.Request.Context(), "api").Warn("删除 API Key 失败", "error", err, "monitor_id", id)
				apiKeyWarning = "API Key 删除失败"
			} else {
				secretChanged = true
			}
		} else {
			// 更新 API Key
			keyVersion, err := config.GetDefaultKeyVersion()
			if err != nil {
				logger.FromContext(c.Request.Context(), "api").Error("获取加密密钥版本失败", "error", err, "monitor_id", id)
				apiKeyWarning = "API Key 保存失败：加密密钥未配置"
			} else {
				result, err := config.EncryptAPIKey(*req.APIKey, id, keyVersion)
				if err != nil {
					logger.FromContext(c.Request.Context(), "api").Error("加密 API Key 失败", "error", err, "monitor_id", id)
					apiKeyWarning = "API Key 加密失败"
				} else {
					if err := h.storage.SetMonitorSecret(id, result.Ciphertext, result.Nonce, result.KeyVersion, result.EncVersion); err != nil {
						logger.FromContext(c.Request.Context(), "api").Error("保存 API Key 失败", "error", err, "monitor_id", id)
						apiKeyWarning = "API Key 保存失败"
					} else {
						secretChanged = true
					}
				}
			}
		}
	}

	// 记录审计日志
	actor := GetAdminActor(c)
	audit := &storage.MonitorConfigAudit{
		MonitorID:     id,
		Provider:      existing.Provider,
		Service:       existing.Service,
		Channel:       existing.Channel,
		Model:         existing.Model,
		Action:        storage.AuditActionUpdate,
		BeforeBlob:    beforeBlob,
		AfterBlob:     existing.ConfigBlob,
		BeforeVersion: &beforeVersion,
		AfterVersion:  &existing.Version,
		SecretChanged: secretChanged,
		Actor:         actor.Token,
		ActorIP:       actor.IP,
		UserAgent:     actor.UserAgent,
		RequestID:     actor.RequestID,
	}
	if err := h.storage.CreateMonitorConfigAudit(audit); err != nil {
		logger.FromContext(c.Request.Context(), "api").Warn("记录审计日志失败", "error", err)
	}

	resp := gin.H{
		"data": h.monitorConfigToResponse(existing),
	}
	if apiKeyWarning != "" {
		resp["warning"] = apiKeyWarning
		resp["api_key_saved"] = false
	}
	c.JSON(http.StatusOK, resp)
}

// UpdateMonitorStatus 更新监测项启用状态
// PATCH /api/admin/monitors/:id/status
func (h *AdminHandler) UpdateMonitorStatus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	var req struct {
		Enabled *bool `json:"enabled" binding:"required"`
		Version int64 `json:"version,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("请求参数错误: %v", err),
		})
		return
	}

	// 验证 enabled 字段必填
	if req.Enabled == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "enabled 字段不能为空",
		})
		return
	}

	// 获取现有配置
	existing, err := h.storage.GetMonitorConfig(id)
	if err != nil {
		logger.FromContext(c.Request.Context(), "api").Error("获取监测项配置失败", "error", err, "id", id)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "监测项不存在"})
		return
	}
	if existing.IsDeleted() {
		c.JSON(http.StatusGone, gin.H{"error": "监测项已被删除"})
		return
	}

	// 乐观锁检查
	if req.Version > 0 && req.Version != existing.Version {
		c.JSON(http.StatusConflict, gin.H{
			"error":           "版本冲突，请刷新后重试",
			"current_version": existing.Version,
		})
		return
	}

	beforeVersion := existing.Version
	existing.Enabled = *req.Enabled

	if err := h.storage.UpdateMonitorConfig(existing); err != nil {
		if strings.Contains(err.Error(), "版本冲突") {
			c.JSON(http.StatusConflict, gin.H{"error": "版本冲突，请刷新后重试"})
			return
		}
		logger.FromContext(c.Request.Context(), "api").Error("更新监测项状态失败", "error", err, "id", id)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}

	// 记录审计日志
	actor := GetAdminActor(c)
	audit := &storage.MonitorConfigAudit{
		MonitorID:     id,
		Provider:      existing.Provider,
		Service:       existing.Service,
		Channel:       existing.Channel,
		Model:         existing.Model,
		Action:        storage.AuditActionUpdate,
		BeforeVersion: &beforeVersion,
		AfterVersion:  &existing.Version,
		Actor:         actor.Token,
		ActorIP:       actor.IP,
		UserAgent:     actor.UserAgent,
		RequestID:     actor.RequestID,
		Reason:        fmt.Sprintf("状态变更: enabled=%v", *req.Enabled),
	}
	if err := h.storage.CreateMonitorConfigAudit(audit); err != nil {
		logger.FromContext(c.Request.Context(), "api").Warn("记录审计日志失败", "error", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"data": h.monitorConfigToResponse(existing),
	})
}

// DeleteMonitor 软删除监测项配置
// DELETE /api/admin/monitors/:id
func (h *AdminHandler) DeleteMonitor(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	// 获取配置信息用于审计
	existing, err := h.storage.GetMonitorConfig(id)
	if err != nil {
		logger.FromContext(c.Request.Context(), "api").Error("获取监测项配置失败", "error", err, "id", id)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "监测项不存在"})
		return
	}
	if existing.IsDeleted() {
		c.JSON(http.StatusGone, gin.H{"error": "监测项已被删除"})
		return
	}

	beforeVersion := existing.Version

	if err := h.storage.DeleteMonitorConfig(id); err != nil {
		logger.FromContext(c.Request.Context(), "api").Error("删除监测项配置失败", "error", err, "id", id)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}

	// 记录审计日志
	actor := GetAdminActor(c)
	afterVersion := beforeVersion + 1
	audit := &storage.MonitorConfigAudit{
		MonitorID:     id,
		Provider:      existing.Provider,
		Service:       existing.Service,
		Channel:       existing.Channel,
		Model:         existing.Model,
		Action:        storage.AuditActionDelete,
		BeforeBlob:    existing.ConfigBlob,
		BeforeVersion: &beforeVersion,
		AfterVersion:  &afterVersion,
		Actor:         actor.Token,
		ActorIP:       actor.IP,
		UserAgent:     actor.UserAgent,
		RequestID:     actor.RequestID,
	}
	if err := h.storage.CreateMonitorConfigAudit(audit); err != nil {
		logger.FromContext(c.Request.Context(), "api").Warn("记录审计日志失败", "error", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "删除成功",
	})
}

// RestoreMonitor 恢复软删除的监测项配置
// POST /api/admin/monitors/:id/restore
func (h *AdminHandler) RestoreMonitor(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	// 获取配置信息
	existing, err := h.storage.GetMonitorConfig(id)
	if err != nil {
		logger.FromContext(c.Request.Context(), "api").Error("获取监测项配置失败", "error", err, "id", id)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "监测项不存在"})
		return
	}
	if !existing.IsDeleted() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "监测项未被删除"})
		return
	}

	beforeVersion := existing.Version

	if err := h.storage.RestoreMonitorConfig(id); err != nil {
		logger.FromContext(c.Request.Context(), "api").Error("恢复监测项配置失败", "error", err, "id", id)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "恢复失败"})
		return
	}

	// 记录审计日志
	actor := GetAdminActor(c)
	afterVersion := beforeVersion + 1
	audit := &storage.MonitorConfigAudit{
		MonitorID:     id,
		Provider:      existing.Provider,
		Service:       existing.Service,
		Channel:       existing.Channel,
		Model:         existing.Model,
		Action:        storage.AuditActionRestore,
		AfterBlob:     existing.ConfigBlob,
		BeforeVersion: &beforeVersion,
		AfterVersion:  &afterVersion,
		Actor:         actor.Token,
		ActorIP:       actor.IP,
		UserAgent:     actor.UserAgent,
		RequestID:     actor.RequestID,
	}
	if err := h.storage.CreateMonitorConfigAudit(audit); err != nil {
		logger.FromContext(c.Request.Context(), "api").Warn("记录审计日志失败", "error", err)
	}

	// 重新获取恢复后的配置
	restored, _ := h.storage.GetMonitorConfig(id)
	if restored != nil {
		c.JSON(http.StatusOK, gin.H{
			"data": h.monitorConfigToResponse(restored),
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"message": "恢复成功",
		})
	}
}

// BatchMonitors 批量操作监测项
// POST /api/admin/monitors/batch
func (h *AdminHandler) BatchMonitors(c *gin.Context) {
	var req BatchMonitorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("请求参数错误: %v", err),
		})
		return
	}

	if len(req.Operations) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "操作列表不能为空"})
		return
	}

	if len(req.Operations) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "单次批量操作最多 100 条"})
		return
	}

	// 生成批量操作 ID
	batchID := uuid.New().String()[:8]
	actor := GetAdminActor(c)

	results := make([]BatchOperationResult, len(req.Operations))

	for i, op := range req.Operations {
		result := BatchOperationResult{Index: i}

		switch op.Action {
		case "enable":
			if op.MonitorID <= 0 {
				result.Error = "monitor_id 不能为空"
			} else {
				result.MonitorID = op.MonitorID
				if err := h.batchUpdateStatus(c, op.MonitorID, true, batchID, actor); err != nil {
					result.Error = err.Error()
				} else {
					result.Success = true
				}
			}

		case "disable":
			if op.MonitorID <= 0 {
				result.Error = "monitor_id 不能为空"
			} else {
				result.MonitorID = op.MonitorID
				if err := h.batchUpdateStatus(c, op.MonitorID, false, batchID, actor); err != nil {
					result.Error = err.Error()
				} else {
					result.Success = true
				}
			}

		case "delete":
			if op.MonitorID <= 0 {
				result.Error = "monitor_id 不能为空"
			} else {
				result.MonitorID = op.MonitorID
				if err := h.batchDelete(c, op.MonitorID, batchID, actor); err != nil {
					result.Error = err.Error()
				} else {
					result.Success = true
				}
			}

		case "restore":
			if op.MonitorID <= 0 {
				result.Error = "monitor_id 不能为空"
			} else {
				result.MonitorID = op.MonitorID
				if err := h.batchRestore(c, op.MonitorID, batchID, actor); err != nil {
					result.Error = err.Error()
				} else {
					result.Success = true
				}
			}

		default:
			result.Error = fmt.Sprintf("不支持的操作: %s", op.Action)
		}

		results[i] = result
	}

	// 统计结果
	successCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"batch_id": batchID,
		"total":    len(req.Operations),
		"success":  successCount,
		"failed":   len(req.Operations) - successCount,
		"results":  results,
	})
}

// GetMonitorHistory 获取监测项审计历史
// GET /api/admin/monitors/:id/history
func (h *AdminHandler) GetMonitorHistory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	// 解析分页参数
	filter := &storage.AuditFilter{
		MonitorID: id,
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit <= 500 {
			filter.Limit = limit
		}
	}

	audits, total, err := h.storage.ListMonitorConfigAudits(filter)
	if err != nil {
		logger.FromContext(c.Request.Context(), "api").Error("查询审计历史失败", "error", err, "monitor_id", id)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  audits,
		"total": total,
		"meta": gin.H{
			"offset": filter.Offset,
			"limit":  filter.Limit,
		},
	})
}

// ===== 全局审计日志 API =====

// ListAudits 获取全局审计日志
// GET /api/admin/audits
func (h *AdminHandler) ListAudits(c *gin.Context) {
	filter := &storage.AuditFilter{}

	// 解析过滤参数
	if v := strings.TrimSpace(c.Query("provider")); v != "" {
		filter.Provider = v
	}
	if v := strings.TrimSpace(c.Query("service")); v != "" {
		filter.Service = v
	}
	if actionRaw := strings.TrimSpace(c.Query("action")); actionRaw != "" {
		action := storage.AuditAction(strings.ToLower(actionRaw))
		switch action {
		case storage.AuditActionCreate,
			storage.AuditActionUpdate,
			storage.AuditActionDelete,
			storage.AuditActionRestore,
			storage.AuditActionRotateSecret:
			filter.Action = action
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "action 参数无效"})
			return
		}
	}
	if sinceStr := strings.TrimSpace(c.Query("since")); sinceStr != "" {
		since, err := strconv.ParseInt(sinceStr, 10, 64)
		if err != nil || since < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "since 参数无效"})
			return
		}
		filter.Since = since
	}
	if untilStr := strings.TrimSpace(c.Query("until")); untilStr != "" {
		until, err := strconv.ParseInt(untilStr, 10, 64)
		if err != nil || until < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "until 参数无效"})
			return
		}
		filter.Until = until
	}

	// 解析分页参数
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit <= 500 {
			filter.Limit = limit
		}
	}

	audits, total, err := h.storage.ListMonitorConfigAudits(filter)
	if err != nil {
		logger.FromContext(c.Request.Context(), "api").Error("查询审计日志失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  audits,
		"total": total,
		"meta": gin.H{
			"offset": filter.Offset,
			"limit":  filter.Limit,
		},
	})
}

// ===== 配置版本 API =====

// GetConfigVersion 获取配置版本
// GET /api/admin/config/version
func (h *AdminHandler) GetConfigVersion(c *gin.Context) {
	versions, err := h.storage.GetConfigVersions()
	if err != nil {
		logger.FromContext(c.Request.Context(), "api").Error("获取配置版本失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": versions,
	})
}

// ===== Provider 策略管理 API =====

// ListProviderPolicies 列出 Provider 策略
// GET /api/admin/policies
func (h *AdminHandler) ListProviderPolicies(c *gin.Context) {
	policies, err := h.storage.ListProviderPolicies()
	if err != nil {
		logger.FromContext(c.Request.Context(), "api").Error("列出 Provider 策略失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": policies})
}

// CreateProviderPolicy 创建 Provider 策略
// POST /api/admin/policies
func (h *AdminHandler) CreateProviderPolicy(c *gin.Context) {
	var req CreateProviderPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("请求参数错误: %v", err)})
		return
	}

	policyTypeRaw := strings.TrimSpace(req.PolicyType)
	if policyTypeRaw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "policy_type 不能为空"})
		return
	}
	policyType := storage.PolicyType(strings.ToLower(policyTypeRaw))
	switch policyType {
	case storage.PolicyTypeDisabled, storage.PolicyTypeHidden, storage.PolicyTypeRisk:
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("无效的 policy_type: %s (支持: disabled/hidden/risk)", policyTypeRaw),
		})
		return
	}

	provider := strings.TrimSpace(req.Provider)
	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider 不能为空"})
		return
	}

	risks := strings.TrimSpace(string(req.Risks))
	if risks != "" && !json.Valid([]byte(risks)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "risks 格式错误"})
		return
	}

	policy := &storage.ProviderPolicy{
		PolicyType: policyType,
		Provider:   provider,
		Reason:     strings.TrimSpace(req.Reason),
		Risks:      risks,
	}

	if err := h.storage.CreateProviderPolicy(policy); err != nil {
		if strings.Contains(err.Error(), "已存在") {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		logger.FromContext(c.Request.Context(), "api").Error("创建 Provider 策略失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": policy})
}

// DeleteProviderPolicy 删除 Provider 策略
// DELETE /api/admin/policies/:id
func (h *AdminHandler) DeleteProviderPolicy(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	if err := h.storage.DeleteProviderPolicy(id); err != nil {
		if strings.Contains(err.Error(), "不存在") {
			c.JSON(http.StatusNotFound, gin.H{"error": "策略不存在"})
			return
		}
		logger.FromContext(c.Request.Context(), "api").Error("删除 Provider 策略失败", "error", err, "id", id)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// ===== Badge 管理 API =====

// ListBadgeDefinitions 列出徽标定义
// GET /api/admin/badges/definitions
func (h *AdminHandler) ListBadgeDefinitions(c *gin.Context) {
	badges, err := h.storage.ListBadgeDefinitions()
	if err != nil {
		logger.FromContext(c.Request.Context(), "api").Error("列出徽标定义失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": badges})
}

// CreateBadgeDefinition 创建徽标定义
// POST /api/admin/badges/definitions
func (h *AdminHandler) CreateBadgeDefinition(c *gin.Context) {
	var req CreateBadgeDefinitionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("请求参数错误: %v", err)})
		return
	}

	badgeID := strings.TrimSpace(req.ID)
	if badgeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 不能为空"})
		return
	}

	kindRaw := strings.TrimSpace(req.Kind)
	if kindRaw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kind 不能为空"})
		return
	}
	kind := storage.BadgeKind(strings.ToLower(kindRaw))
	switch kind {
	case storage.BadgeKindSource, storage.BadgeKindSponsor, storage.BadgeKindRisk, storage.BadgeKindFeature, storage.BadgeKindInfo:
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("无效的 kind: %s (支持: source/sponsor/risk/feature/info)", kindRaw),
		})
		return
	}

	labelI18n := strings.TrimSpace(string(req.LabelI18n))
	if labelI18n == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "label_i18n 不能为空"})
		return
	}
	if !json.Valid([]byte(labelI18n)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "label_i18n 格式错误"})
		return
	}

	tooltipI18n := strings.TrimSpace(string(req.TooltipI18n))
	if tooltipI18n != "" && !json.Valid([]byte(tooltipI18n)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tooltip_i18n 格式错误"})
		return
	}

	badge := &storage.BadgeDefinition{
		ID:          badgeID,
		Kind:        kind,
		Weight:      req.Weight,
		LabelI18n:   labelI18n,
		TooltipI18n: tooltipI18n,
		Icon:        strings.TrimSpace(req.Icon),
		Color:       strings.TrimSpace(req.Color),
	}

	if err := h.storage.CreateBadgeDefinition(badge); err != nil {
		if strings.Contains(err.Error(), "已存在") {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		logger.FromContext(c.Request.Context(), "api").Error("创建徽标定义失败", "error", err, "badge_id", badgeID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": badge})
}

// DeleteBadgeDefinition 删除徽标定义
// DELETE /api/admin/badges/definitions/:id
func (h *AdminHandler) DeleteBadgeDefinition(c *gin.Context) {
	badgeID := strings.TrimSpace(c.Param("id"))
	if badgeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	if err := h.storage.DeleteBadgeDefinition(badgeID); err != nil {
		if strings.Contains(err.Error(), "不存在") {
			c.JSON(http.StatusNotFound, gin.H{"error": "徽标不存在"})
			return
		}
		logger.FromContext(c.Request.Context(), "api").Error("删除徽标定义失败", "error", err, "badge_id", badgeID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// ListBadgeBindings 列出徽标绑定
// GET /api/admin/badges/bindings
func (h *AdminHandler) ListBadgeBindings(c *gin.Context) {
	filter := &storage.BadgeBindingFilter{
		BadgeID:  c.Query("badge_id"),
		Provider: c.Query("provider"),
		Service:  c.Query("service"),
		Channel:  c.Query("channel"),
	}

	if scopeRaw := strings.TrimSpace(c.Query("scope")); scopeRaw != "" {
		scope := storage.BadgeScope(strings.ToLower(scopeRaw))
		switch scope {
		case storage.BadgeScopeGlobal, storage.BadgeScopeProvider, storage.BadgeScopeService, storage.BadgeScopeChannel:
			filter.Scope = scope
		default:
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("无效的 scope: %s (支持: global/provider/service/channel)", scopeRaw),
			})
			return
		}
	}

	bindings, err := h.storage.ListBadgeBindings(filter)
	if err != nil {
		logger.FromContext(c.Request.Context(), "api").Error("列出徽标绑定失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": bindings})
}

// CreateBadgeBinding 创建徽标绑定
// POST /api/admin/badges/bindings
func (h *AdminHandler) CreateBadgeBinding(c *gin.Context) {
	var req CreateBadgeBindingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("请求参数错误: %v", err)})
		return
	}

	badgeID := strings.TrimSpace(req.BadgeID)
	if badgeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "badge_id 不能为空"})
		return
	}

	scopeRaw := strings.TrimSpace(req.Scope)
	if scopeRaw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "scope 不能为空"})
		return
	}

	scope := storage.BadgeScope(strings.ToLower(scopeRaw))
	provider := strings.TrimSpace(req.Provider)
	service := strings.TrimSpace(req.Service)
	channel := strings.TrimSpace(req.Channel)

	// 校验 scope 与 provider/service/channel 的一致性
	switch scope {
	case storage.BadgeScopeGlobal:
		if provider != "" || service != "" || channel != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "scope=global 时 provider/service/channel 必须为空"})
			return
		}
	case storage.BadgeScopeProvider:
		if provider == "" || service != "" || channel != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "scope=provider 需要 provider，且 service/channel 必须为空"})
			return
		}
	case storage.BadgeScopeService:
		if provider == "" || service == "" || channel != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "scope=service 需要 provider/service，且 channel 必须为空"})
			return
		}
	case storage.BadgeScopeChannel:
		if provider == "" || service == "" || channel == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "scope=channel 需要 provider/service/channel"})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("无效的 scope: %s (支持: global/provider/service/channel)", scopeRaw),
		})
		return
	}

	tooltipOverride := strings.TrimSpace(string(req.TooltipOverride))
	if tooltipOverride != "" && !json.Valid([]byte(tooltipOverride)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tooltip_override 格式错误"})
		return
	}

	binding := &storage.BadgeBinding{
		BadgeID:         badgeID,
		Scope:           scope,
		Provider:        provider,
		Service:         service,
		Channel:         channel,
		TooltipOverride: tooltipOverride,
	}

	if err := h.storage.CreateBadgeBinding(binding); err != nil {
		if strings.Contains(err.Error(), "已存在") {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		logger.FromContext(c.Request.Context(), "api").Error("创建徽标绑定失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": binding})
}

// DeleteBadgeBinding 删除徽标绑定
// DELETE /api/admin/badges/bindings/:id
func (h *AdminHandler) DeleteBadgeBinding(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	if err := h.storage.DeleteBadgeBinding(id); err != nil {
		if strings.Contains(err.Error(), "不存在") {
			c.JSON(http.StatusNotFound, gin.H{"error": "绑定不存在"})
			return
		}
		logger.FromContext(c.Request.Context(), "api").Error("删除徽标绑定失败", "error", err, "id", id)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// ===== Board 管理 API =====

// ListBoardConfigs 列出 Board 配置
// GET /api/admin/boards
func (h *AdminHandler) ListBoardConfigs(c *gin.Context) {
	configs, err := h.storage.ListBoardConfigs()
	if err != nil {
		logger.FromContext(c.Request.Context(), "api").Error("列出 Board 配置失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": configs})
}

// ===== 全局设置 API =====

// GetGlobalSetting 获取全局设置
// GET /api/admin/settings/:key
func (h *AdminHandler) GetGlobalSetting(c *gin.Context) {
	key := strings.TrimSpace(c.Param("key"))
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key 不能为空"})
		return
	}

	setting, err := h.storage.GetGlobalSetting(key)
	if err != nil {
		logger.FromContext(c.Request.Context(), "api").Error("获取全局设置失败", "error", err, "key", key)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	if setting == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "设置不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": setting})
}

// SetGlobalSetting 设置全局设置
// PUT /api/admin/settings/:key
func (h *AdminHandler) SetGlobalSetting(c *gin.Context) {
	key := strings.TrimSpace(c.Param("key"))
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key 不能为空"})
		return
	}

	var req SetGlobalSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("请求参数错误: %v", err)})
		return
	}

	value := strings.TrimSpace(string(req.Value))
	if value == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "value 不能为空"})
		return
	}
	if !json.Valid([]byte(value)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "value 格式错误（需要有效的 JSON）"})
		return
	}

	if err := h.storage.SetGlobalSetting(key, value); err != nil {
		logger.FromContext(c.Request.Context(), "api").Error("设置全局设置失败", "error", err, "key", key)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "设置失败"})
		return
	}

	// 返回更新后的设置
	setting, err := h.storage.GetGlobalSetting(key)
	if err != nil {
		logger.FromContext(c.Request.Context(), "api").Warn("读取更新后的设置失败", "error", err, "key", key)
		c.JSON(http.StatusOK, gin.H{"message": "设置成功"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": setting})
}

// ===== 辅助方法 =====

// monitorConfigToResponse 将 MonitorConfig 转换为 API 响应格式
func (h *AdminHandler) monitorConfigToResponse(cfg *storage.MonitorConfig) gin.H {
	result := gin.H{
		"id":             cfg.ID,
		"provider":       cfg.Provider,
		"service":        cfg.Service,
		"channel":        cfg.Channel,
		"model":          cfg.Model,
		"name":           cfg.Name,
		"enabled":        cfg.Enabled,
		"parent_key":     cfg.ParentKey,
		"schema_version": cfg.SchemaVersion,
		"version":        cfg.Version,
		"created_at":     cfg.CreatedAt,
		"updated_at":     cfg.UpdatedAt,
	}

	// 解析 ConfigBlob
	if cfg.ConfigBlob != "" {
		var configData map[string]any
		if err := json.Unmarshal([]byte(cfg.ConfigBlob), &configData); err == nil {
			result["config"] = configData
		}
	}

	if cfg.DeletedAt != nil {
		result["deleted_at"] = *cfg.DeletedAt
	}

	return result
}

// batchUpdateStatus 批量更新状态的辅助方法
func (h *AdminHandler) batchUpdateStatus(c *gin.Context, id int64, enabled bool, batchID string, actor AdminActor) error {
	existing, err := h.storage.GetMonitorConfig(id)
	if err != nil {
		return fmt.Errorf("查询失败: %w", err)
	}
	if existing == nil {
		return fmt.Errorf("监测项不存在")
	}
	if existing.IsDeleted() {
		return fmt.Errorf("监测项已被删除")
	}

	beforeVersion := existing.Version
	existing.Enabled = enabled

	if err := h.storage.UpdateMonitorConfig(existing); err != nil {
		return err
	}

	// 记录审计日志
	audit := &storage.MonitorConfigAudit{
		MonitorID:     id,
		Provider:      existing.Provider,
		Service:       existing.Service,
		Channel:       existing.Channel,
		Model:         existing.Model,
		Action:        storage.AuditActionUpdate,
		BeforeVersion: &beforeVersion,
		AfterVersion:  &existing.Version,
		Actor:         actor.Token,
		ActorIP:       actor.IP,
		UserAgent:     actor.UserAgent,
		RequestID:     actor.RequestID,
		BatchID:       batchID,
		Reason:        fmt.Sprintf("批量操作: enabled=%v", enabled),
	}
	if err := h.storage.CreateMonitorConfigAudit(audit); err != nil {
		logger.FromContext(c.Request.Context(), "api").Warn("批量操作审计日志写入失败", "error", err, "monitor_id", id, "batch_id", batchID)
	}

	return nil
}

// batchDelete 批量删除的辅助方法
func (h *AdminHandler) batchDelete(c *gin.Context, id int64, batchID string, actor AdminActor) error {
	existing, err := h.storage.GetMonitorConfig(id)
	if err != nil {
		return fmt.Errorf("查询失败: %w", err)
	}
	if existing == nil {
		return fmt.Errorf("监测项不存在")
	}
	if existing.IsDeleted() {
		return fmt.Errorf("监测项已被删除")
	}

	beforeVersion := existing.Version

	if err := h.storage.DeleteMonitorConfig(id); err != nil {
		return err
	}

	// 记录审计日志
	afterVersion := beforeVersion + 1
	audit := &storage.MonitorConfigAudit{
		MonitorID:     id,
		Provider:      existing.Provider,
		Service:       existing.Service,
		Channel:       existing.Channel,
		Model:         existing.Model,
		Action:        storage.AuditActionDelete,
		BeforeBlob:    existing.ConfigBlob,
		BeforeVersion: &beforeVersion,
		AfterVersion:  &afterVersion,
		Actor:         actor.Token,
		ActorIP:       actor.IP,
		UserAgent:     actor.UserAgent,
		RequestID:     actor.RequestID,
		BatchID:       batchID,
	}
	if err := h.storage.CreateMonitorConfigAudit(audit); err != nil {
		logger.FromContext(c.Request.Context(), "api").Warn("批量操作审计日志写入失败", "error", err, "monitor_id", id, "batch_id", batchID)
	}

	return nil
}

// batchRestore 批量恢复的辅助方法
func (h *AdminHandler) batchRestore(c *gin.Context, id int64, batchID string, actor AdminActor) error {
	existing, err := h.storage.GetMonitorConfig(id)
	if err != nil {
		return fmt.Errorf("查询失败: %w", err)
	}
	if existing == nil {
		return fmt.Errorf("监测项不存在")
	}
	if !existing.IsDeleted() {
		return fmt.Errorf("监测项未被删除")
	}

	beforeVersion := existing.Version

	if err := h.storage.RestoreMonitorConfig(id); err != nil {
		return err
	}

	// 记录审计日志
	afterVersion := beforeVersion + 1
	audit := &storage.MonitorConfigAudit{
		MonitorID:     id,
		Provider:      existing.Provider,
		Service:       existing.Service,
		Channel:       existing.Channel,
		Model:         existing.Model,
		Action:        storage.AuditActionRestore,
		AfterBlob:     existing.ConfigBlob,
		BeforeVersion: &beforeVersion,
		AfterVersion:  &afterVersion,
		Actor:         actor.Token,
		ActorIP:       actor.IP,
		UserAgent:     actor.UserAgent,
		RequestID:     actor.RequestID,
		BatchID:       batchID,
	}
	if err := h.storage.CreateMonitorConfigAudit(audit); err != nil {
		logger.FromContext(c.Request.Context(), "api").Warn("批量操作审计日志写入失败", "error", err, "monitor_id", id, "batch_id", batchID)
	}

	return nil
}

// maskAPIKey 脱敏 API Key
// 保留前 4 位和后 4 位，中间用 *** 替代
func maskAPIKey(apiKey string) string {
	if len(apiKey) <= 8 {
		return "***"
	}
	return apiKey[:4] + "***" + apiKey[len(apiKey)-4:]
}

// ===== 导入导出 API =====

// ImportConfig 导入 YAML 配置（徽标定义 + 监测项）
// POST /api/admin/import
func (h *AdminHandler) ImportConfig(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请上传 YAML 配置文件"})
		return
	}

	stream, err := file.Open()
	if err != nil {
		logger.FromContext(c.Request.Context(), "api").Error("读取上传文件失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败"})
		return
	}
	defer stream.Close()

	data, err := io.ReadAll(stream)
	if err != nil {
		logger.FromContext(c.Request.Context(), "api").Error("读取���传内容失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败"})
		return
	}

	var cfg config.AppConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("YAML 解析失败: %v", err)})
		return
	}

	// 预处理父子继承（子通道从 parent 继承 provider/service/channel）
	if err := cfg.PreprocessForImport(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("配置预处理失败: %v", err)})
		return
	}

	// 导入徽标定义（在导入监测项之前）
	var badgesCreated, badgesSkipped int
	var badgeErrors []string
	if len(cfg.BadgeDefs) > 0 {
		for id, badgeDef := range cfg.BadgeDefs {
			storageBadge := convertBadgeDefToStorage(id, badgeDef)
			if err := h.storage.CreateBadgeDefinition(storageBadge); err != nil {
				if strings.Contains(err.Error(), "已存在") {
					badgesSkipped++
				} else {
					badgeErrors = append(badgeErrors, fmt.Sprintf("badge[%s]: %v", id, err))
				}
			} else {
				badgesCreated++
			}
		}
		logger.FromContext(c.Request.Context(), "api").Info("导入徽标定义完成",
			"created", badgesCreated, "skipped", badgesSkipped, "errors", len(badgeErrors))
	}

	if len(cfg.Monitors) == 0 {
		// 如果只有徽标定义，也返回成功
		if len(cfg.BadgeDefs) > 0 {
			resp := gin.H{
				"badges_created": badgesCreated,
				"badges_skipped": badgesSkipped,
				"created":        0,
				"skipped":        0,
			}
			if len(badgeErrors) > 0 {
				resp["badge_errors"] = badgeErrors
			}
			c.JSON(http.StatusOK, resp)
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "配置文件未包含 monitors"})
		return
	}

	monitorConfigs := make([]*storage.MonitorConfig, 0, len(cfg.Monitors))
	var buildErrors []string
	for i, monitor := range cfg.Monitors {
		mc, err := buildMonitorConfigFromServiceConfig(monitor)
		if err != nil {
			buildErrors = append(buildErrors, fmt.Sprintf("monitors[%d]: %v", i, err))
			continue
		}
		monitorConfigs = append(monitorConfigs, mc)
	}
	if len(buildErrors) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "配置解析失败",
			"created": 0,
			"skipped": 0,
			"errors":  buildErrors,
		})
		return
	}

	result, err := h.storage.ImportMonitorConfigs(monitorConfigs)
	if err != nil {
		logger.FromContext(c.Request.Context(), "api").Error("导入配置失败", "error", err)
		status := http.StatusInternalServerError
		if result != nil && len(result.Errors) > 0 {
			status = http.StatusBadRequest
		}
		resp := gin.H{"error": "导入失败"}
		if result != nil {
			resp["created"] = result.Created
			resp["skipped"] = result.Skipped
			if len(result.Errors) > 0 {
				resp["errors"] = result.Errors
			}
		}
		c.JSON(status, resp)
		return
	}

	// 合并徽标导入结果
	resp := gin.H{
		"created": result.Created,
		"skipped": result.Skipped,
	}
	if len(result.Errors) > 0 {
		resp["errors"] = result.Errors
	}
	if len(cfg.BadgeDefs) > 0 {
		resp["badges_created"] = badgesCreated
		resp["badges_skipped"] = badgesSkipped
		if len(badgeErrors) > 0 {
			resp["badge_errors"] = badgeErrors
		}
	}
	c.JSON(http.StatusOK, resp)
}

// convertBadgeDefToStorage 将 config.BadgeDef 转换为 storage.BadgeDefinition
func convertBadgeDefToStorage(id string, def config.BadgeDef) *storage.BadgeDefinition {
	// 将 config.BadgeKind 映射到 storage.BadgeKind
	var kind storage.BadgeKind
	switch def.Kind {
	case config.BadgeKindSource:
		kind = storage.BadgeKindSource
	case config.BadgeKindInfo:
		kind = storage.BadgeKindInfo
	case config.BadgeKindFeature:
		kind = storage.BadgeKindFeature
	default:
		kind = storage.BadgeKindInfo
	}

	// 将 Variant 映射为 Color（前端样式提示）
	var color string
	switch def.Variant {
	case config.BadgeVariantSuccess:
		color = "success"
	case config.BadgeVariantWarning:
		color = "warning"
	case config.BadgeVariantDanger:
		color = "danger"
	case config.BadgeVariantInfo:
		color = "info"
	default:
		color = "default"
	}

	// 生成简单的 i18n JSON（使用 badge ID 作为占位符，前端会用 i18n key 替换）
	labelI18n := fmt.Sprintf(`{"zh-CN":"%s","en-US":"%s"}`, id, id)

	return &storage.BadgeDefinition{
		ID:        id,
		Kind:      kind,
		Weight:    def.Weight,
		LabelI18n: labelI18n,
		Icon:      def.URL, // URL 暂存到 Icon 字段
		Color:     color,
	}
}

// ExportConfig 导出 YAML 配置
// GET /api/admin/export
func (h *AdminHandler) ExportConfig(c *gin.Context) {
	monitors, err := h.loadMonitorsForExport()
	if err != nil {
		logger.FromContext(c.Request.Context(), "api").Error("导出监测项失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "导出失败"})
		return
	}

	cfg := config.AppConfig{
		Monitors: monitors,
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		logger.FromContext(c.Request.Context(), "api").Error("序列化导出配置失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "导出失败"})
		return
	}

	c.Header("Content-Disposition", `attachment; filename="relay-pulse-config.yaml"`)
	c.Data(http.StatusOK, "application/x-yaml; charset=utf-8", data)
}

// loadMonitorsForExport 加载用于导出的监测项（API Key 脱敏）
func (h *AdminHandler) loadMonitorsForExport() ([]config.ServiceConfig, error) {
	filter := &storage.MonitorConfigFilter{
		IncludeDeleted: false,
		Limit:          -1, // 无限制，导出所有
	}
	configs, _, err := h.storage.ListMonitorConfigs(filter)
	if err != nil {
		return nil, fmt.Errorf("查询监测项失败: %w", err)
	}

	monitors := make([]config.ServiceConfig, 0, len(configs))
	for _, cfg := range configs {
		monitor, err := buildServiceConfigFromMonitorConfig(cfg)
		if err != nil {
			return nil, err
		}
		monitor.APIKey = "***" // 脱敏
		monitors = append(monitors, *monitor)
	}

	return monitors, nil
}

// buildMonitorConfigFromServiceConfig 从 ServiceConfig 构建 MonitorConfig
func buildMonitorConfigFromServiceConfig(monitor config.ServiceConfig) (*storage.MonitorConfig, error) {
	provider := strings.TrimSpace(monitor.Provider)
	service := strings.TrimSpace(monitor.Service)
	if provider == "" || service == "" {
		return nil, fmt.Errorf("provider/service 不能为空")
	}

	payload := map[string]any{
		"url":              monitor.URL,
		"method":           monitor.Method,
		"headers":          monitor.Headers,
		"body":             monitor.Body,
		"success_contains": monitor.SuccessContains,
		"interval":         monitor.Interval,
		"slow_latency":     monitor.SlowLatency,
		"timeout":          monitor.Timeout,
		"proxy":            monitor.Proxy,
		"provider_name":    monitor.ProviderName,
		"provider_slug":    monitor.ProviderSlug,
		"provider_url":     monitor.ProviderURL,
		"service_name":     monitor.ServiceName,
		"channel_name":     monitor.ChannelName,
		"category":         monitor.Category,
		"sponsor":          monitor.Sponsor,
		"sponsor_url":      monitor.SponsorURL,
		"sponsor_level":    monitor.SponsorLevel,
		"price_min":        monitor.PriceMin,
		"price_max":        monitor.PriceMax,
		"listed_since":     monitor.ListedSince,
		"board":            monitor.Board,
		"cold_reason":      monitor.ColdReason,
		"disabled":         monitor.Disabled,
		"disabled_reason":  monitor.DisabledReason,
		"hidden":           monitor.Hidden,
		"hidden_reason":    monitor.HiddenReason,
	}

	// 添加重试相关配置
	if monitor.Retry != nil {
		payload["retry"] = *monitor.Retry
	}
	if monitor.RetryBaseDelay != "" {
		payload["retry_base_delay"] = monitor.RetryBaseDelay
	}
	if monitor.RetryMaxDelay != "" {
		payload["retry_max_delay"] = monitor.RetryMaxDelay
	}
	if monitor.RetryJitter != nil {
		payload["retry_jitter"] = *monitor.RetryJitter
	}

	// 添加徽标
	if len(monitor.Badges) > 0 {
		payload["badges"] = monitor.Badges
	}

	// 记录原始 env_var_name 便于追溯
	if monitor.EnvVarName != "" {
		payload["_imported_env_var"] = monitor.EnvVarName
	}

	configBlob, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("配置序列化失败: %w", err)
	}

	return &storage.MonitorConfig{
		Provider:   provider,
		Service:    service,
		Channel:    strings.TrimSpace(monitor.Channel),
		Model:      strings.TrimSpace(monitor.Model),
		Name:       strings.TrimSpace(monitor.ChannelName),
		Enabled:    !monitor.Disabled,
		ParentKey:  strings.TrimSpace(monitor.Parent),
		ConfigBlob: string(configBlob),
	}, nil
}

// buildServiceConfigFromMonitorConfig 从 MonitorConfig 构建 ServiceConfig
func buildServiceConfigFromMonitorConfig(cfg *storage.MonitorConfig) (*config.ServiceConfig, error) {
	if cfg == nil {
		return nil, fmt.Errorf("监测项配置为空")
	}

	var payload map[string]any
	if cfg.ConfigBlob != "" {
		if err := json.Unmarshal([]byte(cfg.ConfigBlob), &payload); err != nil {
			return nil, fmt.Errorf("解析 ConfigBlob 失败 (monitor_id=%d): %w", cfg.ID, err)
		}
	}

	monitor := &config.ServiceConfig{
		Provider: cfg.Provider,
		Service:  cfg.Service,
		Channel:  cfg.Channel,
		Model:    cfg.Model,
		Parent:   cfg.ParentKey,
	}

	// 从 payload 提取字段
	if payload != nil {
		if v, ok := payload["url"].(string); ok {
			monitor.URL = v
		}
		if v, ok := payload["method"].(string); ok {
			monitor.Method = v
		}
		if v, ok := payload["headers"].(map[string]any); ok {
			monitor.Headers = make(map[string]string)
			for k, val := range v {
				if s, ok := val.(string); ok {
					monitor.Headers[k] = s
				}
			}
		}
		if v, ok := payload["body"].(string); ok {
			monitor.Body = v
		}
		if v, ok := payload["success_contains"].(string); ok {
			monitor.SuccessContains = v
		}
		if v, ok := payload["interval"].(string); ok {
			monitor.Interval = v
		}
		if v, ok := payload["slow_latency"].(string); ok {
			monitor.SlowLatency = v
		}
		if v, ok := payload["timeout"].(string); ok {
			monitor.Timeout = v
		}
		if v, ok := payload["proxy"].(string); ok {
			monitor.Proxy = v
		}
		if v, ok := payload["provider_name"].(string); ok {
			monitor.ProviderName = v
		}
		if v, ok := payload["provider_slug"].(string); ok {
			monitor.ProviderSlug = v
		}
		if v, ok := payload["provider_url"].(string); ok {
			monitor.ProviderURL = v
		}
		if v, ok := payload["service_name"].(string); ok {
			monitor.ServiceName = v
		}
		if v, ok := payload["channel_name"].(string); ok {
			monitor.ChannelName = v
		}
		if v, ok := payload["category"].(string); ok {
			monitor.Category = v
		}
		if v, ok := payload["sponsor"].(string); ok {
			monitor.Sponsor = v
		}
		if v, ok := payload["sponsor_url"].(string); ok {
			monitor.SponsorURL = v
		}
		if v, ok := payload["sponsor_level"].(string); ok {
			monitor.SponsorLevel = config.SponsorLevel(v)
		}
		if v, ok := payload["price_min"].(float64); ok {
			monitor.PriceMin = &v
		}
		if v, ok := payload["price_max"].(float64); ok {
			monitor.PriceMax = &v
		}
		if v, ok := payload["listed_since"].(string); ok {
			monitor.ListedSince = v
		}
		if v, ok := payload["board"].(string); ok {
			monitor.Board = v
		}
		if v, ok := payload["cold_reason"].(string); ok {
			monitor.ColdReason = v
		}
		if v, ok := payload["disabled"].(bool); ok {
			monitor.Disabled = v
		}
		if v, ok := payload["disabled_reason"].(string); ok {
			monitor.DisabledReason = v
		}
		if v, ok := payload["hidden"].(bool); ok {
			monitor.Hidden = v
		}
		if v, ok := payload["hidden_reason"].(string); ok {
			monitor.HiddenReason = v
		}
		if v, ok := payload["retry"].(float64); ok {
			retry := int(v)
			monitor.Retry = &retry
		}
		if v, ok := payload["retry_base_delay"].(string); ok {
			monitor.RetryBaseDelay = v
		}
		if v, ok := payload["retry_max_delay"].(string); ok {
			monitor.RetryMaxDelay = v
		}
		if v, ok := payload["retry_jitter"].(float64); ok {
			monitor.RetryJitter = &v
		}
	}

	// 使用 cfg.Name 覆盖 channel_name
	if cfg.Name != "" {
		monitor.ChannelName = cfg.Name
	}
	// 根据 enabled 设置 disabled
	if !cfg.Enabled {
		monitor.Disabled = true
	}

	return monitor, nil
}

// ===== 请求/响应类型 =====

// CreateMonitorRequest 创建监测项请求
type CreateMonitorRequest struct {
	Provider  string `json:"provider" binding:"required"`
	Service   string `json:"service" binding:"required"`
	Channel   string `json:"channel"`
	Model     string `json:"model"`
	Name      string `json:"name"`
	Enabled   *bool  `json:"enabled"`
	ParentKey string `json:"parent_key"`
	APIKey    string `json:"api_key"`
	Config    any    `json:"config"` // 原始配置对象
}

// UpdateMonitorRequest 更新监测项请求
type UpdateMonitorRequest struct {
	Name      *string `json:"name"`
	Enabled   *bool   `json:"enabled"`
	ParentKey *string `json:"parent_key"`
	APIKey    *string `json:"api_key"` // 空字符串表示删除
	Config    any     `json:"config"`
	Version   int64   `json:"version"` // 乐观锁
}

// BatchMonitorRequest 批量操作请求
type BatchMonitorRequest struct {
	Operations []BatchOperation `json:"operations" binding:"required"`
}

// BatchOperation 单个批量操作
type BatchOperation struct {
	Action    string `json:"action" binding:"required"` // enable/disable/delete/restore
	MonitorID int64  `json:"monitor_id"`
}

// BatchOperationResult 批量操作结果
type BatchOperationResult struct {
	Index     int    `json:"index"`
	Success   bool   `json:"success"`
	MonitorID int64  `json:"monitor_id,omitempty"`
	Error     string `json:"error,omitempty"`
}

// CreateProviderPolicyRequest 创建 Provider 策略请求
type CreateProviderPolicyRequest struct {
	PolicyType string          `json:"policy_type" binding:"required"`
	Provider   string          `json:"provider" binding:"required"`
	Reason     string          `json:"reason"`
	Risks      json.RawMessage `json:"risks"`
}

// CreateBadgeDefinitionRequest 创建徽标定义请求
type CreateBadgeDefinitionRequest struct {
	ID          string          `json:"id" binding:"required"`
	Kind        string          `json:"kind" binding:"required"`
	Weight      int             `json:"weight"`
	LabelI18n   json.RawMessage `json:"label_i18n" binding:"required"`
	TooltipI18n json.RawMessage `json:"tooltip_i18n"`
	Icon        string          `json:"icon"`
	Color       string          `json:"color"`
}

// CreateBadgeBindingRequest 创建徽标绑定请求
type CreateBadgeBindingRequest struct {
	BadgeID         string          `json:"badge_id" binding:"required"`
	Scope           string          `json:"scope" binding:"required"`
	Provider        string          `json:"provider"`
	Service         string          `json:"service"`
	Channel         string          `json:"channel"`
	TooltipOverride json.RawMessage `json:"tooltip_override"`
}

// SetGlobalSettingRequest 设置全局设置请求
type SetGlobalSettingRequest struct {
	Value json.RawMessage `json:"value" binding:"required"`
}
