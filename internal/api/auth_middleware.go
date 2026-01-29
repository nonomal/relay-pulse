package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"monitor/internal/logger"
	"monitor/internal/storage"
)

// =====================================================
// v1.0 认证中间件
// =====================================================

// Context keys（使用私有类型避免冲突）
type contextKey string

const (
	ctxKeyUser    contextKey = "auth_user"
	ctxKeySession contextKey = "auth_session"
)

// Session 配置
const (
	SessionTokenLength = 32                    // Token 长度（字节）
	SessionCookieName  = "relay_pulse_session" // Cookie 名称
	SessionDefaultTTL  = 7 * 24 * time.Hour    // 默认会话有效期（7天）
	SessionSlidingTTL  = 24 * time.Hour        // 滑动续期阈值（剩余不足24小时时续期）
	SessionExtendedTTL = 7 * 24 * time.Hour    // 续期后的有效期
)

// AuthMiddleware 认证中间件配置
type AuthMiddleware struct {
	storage storage.Storage
}

// NewAuthMiddleware 创建认证中间件
func NewAuthMiddleware(s storage.Storage) *AuthMiddleware {
	return &AuthMiddleware{storage: s}
}

// RequireAuth 需要登录的中间件
func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, session, err := m.authenticate(c)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "未登录或会话已过期",
				"code":  "UNAUTHORIZED",
			})
			return
		}

		// 检查用户状态
		if user.Status != storage.UserStatusActive {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "账号已被禁用",
				"code":  "USER_DISABLED",
			})
			return
		}

		// 将用户和会话信息存入 context
		setUserToContext(c, user)
		setSessionToContext(c, session)

		// 滑动续期
		m.slidingExpire(c, session)

		c.Next()
	}
}

// RequireAdmin 需要管理员角色的中间件
func (m *AuthMiddleware) RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, session, err := m.authenticate(c)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "未登录或会话已过期",
				"code":  "UNAUTHORIZED",
			})
			return
		}

		// 检查用户状态
		if user.Status != storage.UserStatusActive {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "账号已被禁用",
				"code":  "USER_DISABLED",
			})
			return
		}

		// 检查管理员角色
		if user.Role != storage.UserRoleAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "需要管理员权限",
				"code":  "ADMIN_REQUIRED",
			})
			return
		}

		// 将用户和会话信息存入 context
		setUserToContext(c, user)
		setSessionToContext(c, session)

		// 滑动续期
		m.slidingExpire(c, session)

		c.Next()
	}
}

// OptionalAuth 可选登录的中间件
// 有 token 则验证并注入用户信息，无 token 则跳过
func (m *AuthMiddleware) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			// 无 token，直接放行
			c.Next()
			return
		}

		// 有 token，尝试验证
		user, session, err := m.authenticateWithToken(c, token)
		if err != nil {
			// token 无效，返回 401（防止攻击者用垃圾 token 绕过）
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "会话无效或已过期",
				"code":  "INVALID_SESSION",
			})
			return
		}

		// 检查用户状态（可选认证时，禁用用户也返回 401）
		if user.Status != storage.UserStatusActive {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "账号已被禁用",
				"code":  "USER_DISABLED",
			})
			return
		}

		// 将用户和会话信息存入 context
		setUserToContext(c, user)
		setSessionToContext(c, session)

		// 滑动续期
		m.slidingExpire(c, session)

		c.Next()
	}
}

// authenticate 验证请求中的 token
func (m *AuthMiddleware) authenticate(c *gin.Context) (*storage.User, *storage.UserSession, error) {
	token := extractToken(c)
	if token == "" {
		return nil, nil, ErrNoToken
	}
	return m.authenticateWithToken(c, token)
}

// authenticateWithToken 使用指定 token 进行验证
func (m *AuthMiddleware) authenticateWithToken(c *gin.Context, token string) (*storage.User, *storage.UserSession, error) {
	// 计算 token hash
	tokenHash := HashToken(token)

	// 查询会话
	ctx := c.Request.Context()

	// 类型断言获取 UserStorage
	userStorage, ok := m.storage.(storage.UserStorage)
	if !ok {
		logger.Error("api", "存储层不支持 UserStorage 接口")
		return nil, nil, ErrStorageNotSupported
	}

	session, err := userStorage.GetSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		logger.Error("api", "查询会话失败", "error", err)
		return nil, nil, err
	}
	if session == nil {
		return nil, nil, ErrSessionNotFound
	}

	// 检查会话是否过期
	now := time.Now().Unix()
	if session.ExpiresAt < now {
		return nil, nil, ErrSessionExpired
	}

	// 检查会话是否被撤销
	if session.RevokedAt != nil && *session.RevokedAt > 0 {
		return nil, nil, ErrSessionRevoked
	}

	// 查询用户
	user, err := userStorage.GetUserByID(ctx, session.UserID)
	if err != nil {
		logger.Error("api", "查询用户失败", "error", err)
		return nil, nil, err
	}
	if user == nil {
		return nil, nil, ErrUserNotFound
	}

	return user, session, nil
}

// slidingExpire 滑动续期
func (m *AuthMiddleware) slidingExpire(c *gin.Context, session *storage.UserSession) {
	now := time.Now().Unix()
	remaining := session.ExpiresAt - now

	// 如果剩余时间不足阈值，则续期
	if remaining < int64(SessionSlidingTTL.Seconds()) {
		userStorage, ok := m.storage.(storage.UserStorage)
		if !ok {
			return
		}

		ctx := c.Request.Context()
		if err := userStorage.TouchSession(ctx, session.ID); err != nil {
			logger.Warn("api", "会话续期失败", "session_id", session.ID, "error", err)
		}
	}
}

// extractToken 从请求中提取 token
// 优先级：Authorization: Bearer > Cookie
func extractToken(c *gin.Context) string {
	// 1. 尝试从 Authorization header 获取
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return strings.TrimSpace(parts[1])
		}
	}

	// 2. 尝试从 Cookie 获取
	cookie, err := c.Cookie(SessionCookieName)
	if err == nil && cookie != "" {
		return cookie
	}

	return ""
}

// =====================================================
// Context 辅助函数
// =====================================================

// setUserToContext 将用户信息存入 context
func setUserToContext(c *gin.Context, user *storage.User) {
	c.Set(string(ctxKeyUser), user)
}

// setSessionToContext 将会话信息存入 context
func setSessionToContext(c *gin.Context, session *storage.UserSession) {
	c.Set(string(ctxKeySession), session)
}

// GetUserFromContext 从 context 获取用户信息
func GetUserFromContext(c *gin.Context) *storage.User {
	if val, exists := c.Get(string(ctxKeyUser)); exists {
		if user, ok := val.(*storage.User); ok {
			return user
		}
	}
	return nil
}

// GetSessionFromContext 从 context 获取会话信息
func GetSessionFromContext(c *gin.Context) *storage.UserSession {
	if val, exists := c.Get(string(ctxKeySession)); exists {
		if session, ok := val.(*storage.UserSession); ok {
			return session
		}
	}
	return nil
}

// GetUserFromStdContext 从标准 context 获取用户信息
func GetUserFromStdContext(ctx context.Context) *storage.User {
	if val := ctx.Value(ctxKeyUser); val != nil {
		if user, ok := val.(*storage.User); ok {
			return user
		}
	}
	return nil
}

// =====================================================
// Token 工具函数
// =====================================================

// GenerateSessionToken 生成会话 token
func GenerateSessionToken() (string, error) {
	bytes := make([]byte, SessionTokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// HashToken 计算 token 的 hash
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// =====================================================
// 错误定义
// =====================================================

// 认证相关错误
var (
	ErrNoToken             = &AuthError{Code: "NO_TOKEN", Message: "缺少认证 token"}
	ErrSessionNotFound     = &AuthError{Code: "SESSION_NOT_FOUND", Message: "会话不存在"}
	ErrSessionExpired      = &AuthError{Code: "SESSION_EXPIRED", Message: "会话已过期"}
	ErrSessionRevoked      = &AuthError{Code: "SESSION_REVOKED", Message: "会话已被撤销"}
	ErrUserNotFound        = &AuthError{Code: "USER_NOT_FOUND", Message: "用户不存在"}
	ErrStorageNotSupported = &AuthError{Code: "STORAGE_NOT_SUPPORTED", Message: "存储层不支持用户认证"}
)

// AuthError 认证错误
type AuthError struct {
	Code    string
	Message string
}

func (e *AuthError) Error() string {
	return e.Message
}
