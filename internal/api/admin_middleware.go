package api

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"monitor/internal/logger"
)

const (
	// AdminTokenHeader 管理 API Token 请求头
	AdminTokenHeader = "X-Config-Token"

	// AdminTokenEnvVar 管理 API Token 环境变量
	AdminTokenEnvVar = "CONFIG_API_TOKEN"

	// AdminActorContextKey 操作者信息上下文键
	AdminActorContextKey = "admin_actor"
)

// AdminActor 管理 API 操作者信息
// 用于审计日志记录
type AdminActor struct {
	Token     string // Token 名称（脱敏后）
	IP        string // 来源 IP
	UserAgent string // User-Agent
	RequestID string // 请求追踪 ID
}

// AdminAuthMiddleware 管理 API Token 认证中间件
// 验证 X-Config-Token 头并注入操作者信息到上下文
func AdminAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取配置的 Token
		configToken := os.Getenv(AdminTokenEnvVar)
		if configToken == "" {
			logger.Warn("api", "管理 API 未配置 Token，拒绝所有请求",
				"env_var", AdminTokenEnvVar)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error": "管理 API 未启用",
			})
			return
		}

		// 验证请求 Token
		requestToken := c.GetHeader(AdminTokenHeader)
		if requestToken == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "缺少认证 Token",
			})
			return
		}

		// 使用常量时间比较防止时序攻击
		if subtle.ConstantTimeCompare([]byte(requestToken), []byte(configToken)) != 1 {
			logger.Warn("api", "管理 API Token 认证失败",
				"ip", getClientIP(c),
				"user_agent", c.GetHeader("User-Agent"))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Token 无效",
			})
			return
		}

		// 构建操作者信息
		actor := AdminActor{
			Token:     maskToken(requestToken),
			IP:        getClientIP(c),
			UserAgent: c.GetHeader("User-Agent"),
			RequestID: c.GetString("request_id"),
		}

		// 注入到上下文
		c.Set(AdminActorContextKey, actor)

		c.Next()
	}
}

// GetAdminActor 从上下文获取操作者信息
func GetAdminActor(c *gin.Context) AdminActor {
	if v, exists := c.Get(AdminActorContextKey); exists {
		if actor, ok := v.(AdminActor); ok {
			return actor
		}
	}
	return AdminActor{}
}

// getClientIP 获取客户端真实 IP
// 支持 X-Forwarded-For 和 X-Real-IP 头（Cloudflare/Nginx 代理场景）
func getClientIP(c *gin.Context) string {
	// 优先使用 X-Forwarded-For（取第一个 IP）
	xff := c.GetHeader("X-Forwarded-For")
	if xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if ip != "" {
				return ip
			}
		}
	}

	// 其次使用 X-Real-IP
	xri := c.GetHeader("X-Real-IP")
	if xri != "" {
		return strings.TrimSpace(xri)
	}

	// 最后使用 Gin 提供的 ClientIP
	return c.ClientIP()
}

// maskToken 脱敏 Token（保留前 4 位和后 4 位）
func maskToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "***" + token[len(token)-4:]
}
