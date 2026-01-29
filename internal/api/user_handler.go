package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"monitor/internal/logger"
	"monitor/internal/storage"
)

// UserHandler 用户管理处理器（管理员端）
type UserHandler struct {
	storage storage.Storage
}

// NewUserHandler 创建用户管理处理器
func NewUserHandler(s storage.Storage) *UserHandler {
	return &UserHandler{storage: s}
}

// RegisterRoutes 注册管理员用户管理路由
func (h *UserHandler) RegisterRoutes(r *gin.RouterGroup, authMiddleware *AuthMiddleware) {
	users := r.Group("/users")
	users.Use(authMiddleware.RequireAdmin())
	{
		users.GET("", h.ListUsers)
		users.GET("/:id", h.GetUser)
		users.PATCH("/:id", h.UpdateUser)
	}
}

// ListUsers 获取用户列表
// GET /api/v1/admin/users
func (h *UserHandler) ListUsers(c *gin.Context) {
	userStorage, ok := h.storage.(storage.UserStorage)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "存储层不支持用户管理",
			"code":  "STORAGE_NOT_SUPPORTED",
		})
		return
	}

	opts := &storage.ListUsersOptions{}

	// 解析查询参数
	if role := strings.TrimSpace(c.Query("role")); role != "" {
		r := storage.UserRole(role)
		if r == storage.UserRoleAdmin || r == storage.UserRoleUser {
			opts.Role = &r
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "无效的角色参数",
				"code":  "INVALID_ROLE",
			})
			return
		}
	}

	if status := strings.TrimSpace(c.Query("status")); status != "" {
		s := storage.UserStatus(status)
		if s == storage.UserStatusActive || s == storage.UserStatusDisabled {
			opts.Status = &s
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "无效的状态参数",
				"code":  "INVALID_STATUS",
			})
			return
		}
	}

	if search := strings.TrimSpace(c.Query("search")); search != "" {
		opts.Search = search
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

	users, total, err := userStorage.ListUsers(c.Request.Context(), opts)
	if err != nil {
		logger.FromContext(c.Request.Context(), "api").Error("查询用户列表失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "查询用户列表失败",
			"code":  "LIST_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  users,
		"total": total,
		"meta": gin.H{
			"offset": opts.Offset,
			"limit":  opts.Limit,
		},
	})
}

// GetUser 获取单个用户详情
// GET /api/v1/admin/users/:id
func (h *UserHandler) GetUser(c *gin.Context) {
	userStorage, ok := h.storage.(storage.UserStorage)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "存储层不支持用户管理",
			"code":  "STORAGE_NOT_SUPPORTED",
		})
		return
	}

	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "缺少用户 ID",
			"code":  "MISSING_ID",
		})
		return
	}

	user, err := userStorage.GetUserByID(c.Request.Context(), id)
	if err != nil {
		logger.FromContext(c.Request.Context(), "api").Error("查询用户失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "查询用户失败",
			"code":  "GET_FAILED",
		})
		return
	}

	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "用户不存在",
			"code":  "NOT_FOUND",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": user,
	})
}

// UpdateUserRequest 更新用户请求
type UpdateUserRequest struct {
	Role   *storage.UserRole   `json:"role"`
	Status *storage.UserStatus `json:"status"`
}

// UpdateUser 更新用户角色或状态
// PATCH /api/v1/admin/users/:id
func (h *UserHandler) UpdateUser(c *gin.Context) {
	userStorage, ok := h.storage.(storage.UserStorage)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "存储层不支持用户管理",
			"code":  "STORAGE_NOT_SUPPORTED",
		})
		return
	}

	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "缺少用户 ID",
			"code":  "MISSING_ID",
		})
		return
	}

	// 获取当前操作者
	currentUser := GetUserFromContext(c)
	if currentUser == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "未登录",
			"code":  "NOT_LOGGED_IN",
		})
		return
	}

	// 防止自己修改自己
	if id == currentUser.ID {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "不能修改自己的角色或状态",
			"code":  "CANNOT_MODIFY_SELF",
		})
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数无效",
			"code":  "INVALID_REQUEST",
		})
		return
	}

	// 至少需要修改一项
	if req.Role == nil && req.Status == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请提供要修改的字段",
			"code":  "NO_FIELDS_TO_UPDATE",
		})
		return
	}

	// 验证角色
	if req.Role != nil {
		if *req.Role != storage.UserRoleAdmin && *req.Role != storage.UserRoleUser {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "无效的角色值",
				"code":  "INVALID_ROLE",
			})
			return
		}
	}

	// 验证状态
	if req.Status != nil {
		if *req.Status != storage.UserStatusActive && *req.Status != storage.UserStatusDisabled {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "无效的状态值",
				"code":  "INVALID_STATUS",
			})
			return
		}
	}

	// 获取用户
	user, err := userStorage.GetUserByID(c.Request.Context(), id)
	if err != nil {
		logger.FromContext(c.Request.Context(), "api").Error("查询用户失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "查询用户失败",
			"code":  "GET_FAILED",
		})
		return
	}

	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "用户不存在",
			"code":  "NOT_FOUND",
		})
		return
	}

	// 保存更新前的状态
	oldUser := *user

	// 更新字段
	if req.Role != nil {
		user.Role = *req.Role
	}
	if req.Status != nil {
		user.Status = *req.Status
	}
	user.UpdatedAt = time.Now().Unix()

	if err := userStorage.UpdateUser(c.Request.Context(), user); err != nil {
		logger.FromContext(c.Request.Context(), "api").Error("更新用户失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "更新用户失败",
			"code":  "UPDATE_FAILED",
		})
		return
	}

	// 如果禁用用户，同时撤销其所有会话
	if req.Status != nil && *req.Status == storage.UserStatusDisabled {
		if _, err := userStorage.DeleteUserSessions(c.Request.Context(), id); err != nil {
			logger.FromContext(c.Request.Context(), "api").Warn("撤销用户会话失败", "error", err, "user_id", id)
		}
	}

	// 记录审计日志
	h.createAuditLog(c, storage.AuditActionUpdate, storage.AuditResourceUser, id, &oldUser, user)

	logger.FromContext(c.Request.Context(), "api").Info("更新用户成功",
		"user_id", id,
		"operator", currentUser.Username,
		"role", user.Role,
		"status", user.Status)

	c.JSON(http.StatusOK, gin.H{
		"data": user,
	})
}

// createAuditLog 创建审计日志
func (h *UserHandler) createAuditLog(c *gin.Context, action storage.AuditAction, resourceType storage.AuditResourceType, resourceID string, before, after interface{}) {
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
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		IPAddress:    getClientIP(c),
		UserAgent:    c.GetHeader("User-Agent"),
		CreatedAt:    time.Now().Unix(),
	}

	// 构建变更内容（before/after 结构）
	if before != nil || after != nil {
		changes := make(map[string]interface{})
		if before != nil {
			changes["before"] = before
		}
		if after != nil {
			changes["after"] = after
		}
		if data, err := json.Marshal(changes); err == nil {
			log.Changes = data
		}
	}

	if err := auditStorage.CreateAuditLog(c.Request.Context(), log); err != nil {
		logger.FromContext(c.Request.Context(), "api").Warn("创建审计日志失败", "error", err)
	}
}
