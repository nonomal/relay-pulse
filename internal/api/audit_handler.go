package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"monitor/internal/logger"
	"monitor/internal/storage"
)

// AuditHandler v1 审计日志处理器
type AuditHandler struct {
	storage storage.Storage
}

// NewAuditHandler 创建审计日志处理器
func NewAuditHandler(s storage.Storage) *AuditHandler {
	return &AuditHandler{storage: s}
}

// RegisterRoutes 注册审计日志路由
func (h *AuditHandler) RegisterRoutes(r *gin.RouterGroup, authMiddleware *AuthMiddleware) {
	audits := r.Group("/audits")
	audits.Use(authMiddleware.RequireAdmin())
	{
		audits.GET("", h.ListAuditLogs)
	}
}

// ListAuditLogs 获取审计日志列表
// GET /api/v1/admin/audits
func (h *AuditHandler) ListAuditLogs(c *gin.Context) {
	auditStorage, ok := h.storage.(storage.AuditLogStorage)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "存储层不支持审计日志",
			"code":  "STORAGE_NOT_SUPPORTED",
		})
		return
	}

	opts := &storage.ListAuditLogsOptions{}

	// 解析查询参数
	if userID := strings.TrimSpace(c.Query("user_id")); userID != "" {
		opts.UserID = &userID
	}

	if action := strings.TrimSpace(c.Query("action")); action != "" {
		a := storage.AuditAction(action)
		// 验证操作类型
		validActions := map[storage.AuditAction]bool{
			storage.AuditActionCreate:       true,
			storage.AuditActionUpdate:       true,
			storage.AuditActionDelete:       true,
			storage.AuditActionRestore:      true,
			storage.AuditActionRotateSecret: true,
			storage.AuditActionApprove:      true,
			storage.AuditActionReject:       true,
		}
		if !validActions[a] {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "无效的操作类型",
				"code":  "INVALID_ACTION",
			})
			return
		}
		opts.Action = &a
	}

	if resourceType := strings.TrimSpace(c.Query("resource_type")); resourceType != "" {
		rt := storage.AuditResourceType(resourceType)
		// 验证资源类型
		validTypes := map[storage.AuditResourceType]bool{
			storage.AuditResourceUser:        true,
			storage.AuditResourceService:     true,
			storage.AuditResourceTemplate:    true,
			storage.AuditResourceMonitor:     true,
			storage.AuditResourceApplication: true,
			storage.AuditResourceBadge:       true,
		}
		if !validTypes[rt] {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "无效的资源类型",
				"code":  "INVALID_RESOURCE_TYPE",
			})
			return
		}
		opts.ResourceType = &rt
	}

	if resourceID := strings.TrimSpace(c.Query("resource_id")); resourceID != "" {
		opts.ResourceID = resourceID
	}

	// 解析时间范围
	if startTimeStr := c.Query("start_time"); startTimeStr != "" {
		if startTime, err := strconv.ParseInt(startTimeStr, 10, 64); err == nil && startTime > 0 {
			opts.StartTime = &startTime
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "无效的开始时间",
				"code":  "INVALID_START_TIME",
			})
			return
		}
	}

	if endTimeStr := c.Query("end_time"); endTimeStr != "" {
		if endTime, err := strconv.ParseInt(endTimeStr, 10, 64); err == nil && endTime > 0 {
			opts.EndTime = &endTime
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "无效的结束时间",
				"code":  "INVALID_END_TIME",
			})
			return
		}
	}

	// 解析分页参数
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			opts.Offset = offset
		}
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit <= 100 {
			opts.Limit = limit
		}
	}
	if opts.Limit <= 0 {
		opts.Limit = 20
	}

	logs, total, err := auditStorage.ListAuditLogs(c.Request.Context(), opts)
	if err != nil {
		logger.FromContext(c.Request.Context(), "api").Error("查询审计日志失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "查询审计日志失败",
			"code":  "LIST_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  logs,
		"total": total,
		"meta": gin.H{
			"offset": opts.Offset,
			"limit":  opts.Limit,
		},
	})
}
