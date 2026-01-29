package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"monitor/internal/logger"
	"monitor/internal/storage"
)

// =====================================================
// GitHub OAuth 配置
// =====================================================

// GitHub OAuth 环境变量
const (
	EnvGitHubClientID     = "GITHUB_CLIENT_ID"
	EnvGitHubClientSecret = "GITHUB_CLIENT_SECRET"
	EnvGitHubCallbackURL  = "GITHUB_CALLBACK_URL"
)

// GitHub OAuth URLs
const (
	GitHubAuthorizeURL = "https://github.com/login/oauth/authorize"
	GitHubTokenURL     = "https://github.com/login/oauth/access_token"
	GitHubUserURL      = "https://api.github.com/user"
)

// OAuth state 配置
const (
	OAuthStateLength = 16               // State 长度（字节）
	OAuthStateTTL    = 10 * time.Minute // State 有效期
	OAuthStateCookie = "oauth_state"    // State cookie 名称
)

// =====================================================
// GitHub OAuth 处理器
// =====================================================

// AuthHandler 认证处理器
type AuthHandler struct {
	storage      storage.Storage
	clientID     string
	clientSecret string
	callbackURL  string
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(s storage.Storage) *AuthHandler {
	return &AuthHandler{
		storage:      s,
		clientID:     os.Getenv(EnvGitHubClientID),
		clientSecret: os.Getenv(EnvGitHubClientSecret),
		callbackURL:  os.Getenv(EnvGitHubCallbackURL),
	}
}

// IsConfigured 检查 OAuth 是否已配置
func (h *AuthHandler) IsConfigured() bool {
	return h.clientID != "" && h.clientSecret != ""
}

// RegisterRoutes 注册认证路由
func (h *AuthHandler) RegisterRoutes(r *gin.RouterGroup) {
	auth := r.Group("/auth")
	{
		auth.GET("/github/login", h.GitHubLogin)
		auth.GET("/github/callback", h.GitHubCallback)
		auth.GET("/me", h.GetCurrentUser)
		auth.POST("/logout", h.Logout)
	}
}

// GitHubLogin 发起 GitHub OAuth 登录
// GET /api/auth/github/login
func (h *AuthHandler) GitHubLogin(c *gin.Context) {
	if !h.IsConfigured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "GitHub OAuth 未配置",
			"code":  "OAUTH_NOT_CONFIGURED",
		})
		return
	}

	// 生成 state 防止 CSRF
	state, err := generateOAuthState()
	if err != nil {
		logger.Error("api", "生成 OAuth state 失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "生成认证状态失败",
			"code":  "STATE_GENERATION_FAILED",
		})
		return
	}

	// 将 state 存入 cookie（设置 SameSite=Lax 防止 CSRF）
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		OAuthStateCookie,
		state,
		int(OAuthStateTTL.Seconds()),
		"/",
		"",
		true, // secure
		true, // httpOnly
	)

	// 构建 GitHub 授权 URL
	params := url.Values{
		"client_id":    {h.clientID},
		"redirect_uri": {h.callbackURL},
		"scope":        {"read:user user:email"},
		"state":        {state},
	}
	authURL := GitHubAuthorizeURL + "?" + params.Encode()

	// 获取 redirect 参数（登录后跳转地址）
	redirect := c.Query("redirect")
	if redirect != "" {
		// 将 redirect 编码到 state 中（简化处理，实际可用 session 存储）
		// 这里直接重定向，前端可以自行处理
	}

	c.Redirect(http.StatusTemporaryRedirect, authURL)
}

// GitHubCallback 处理 GitHub OAuth 回调
// GET /api/auth/github/callback
func (h *AuthHandler) GitHubCallback(c *gin.Context) {
	if !h.IsConfigured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "GitHub OAuth 未配置",
			"code":  "OAUTH_NOT_CONFIGURED",
		})
		return
	}

	// 验证 state
	state := c.Query("state")
	cookieState, err := c.Cookie(OAuthStateCookie)
	if err != nil || state == "" || state != cookieState {
		logger.Warn("api", "OAuth state 验证失败",
			"state", state,
			"cookie_state", cookieState,
			"ip", getClientIP(c))
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "认证状态无效",
			"code":  "INVALID_STATE",
		})
		return
	}

	// 清除 state cookie
	c.SetCookie(OAuthStateCookie, "", -1, "/", "", true, true)

	// 检查错误
	if errCode := c.Query("error"); errCode != "" {
		errDesc := c.Query("error_description")
		logger.Warn("api", "GitHub OAuth 错误",
			"error", errCode,
			"description", errDesc)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": errDesc,
			"code":  "OAUTH_ERROR",
		})
		return
	}

	// 获取授权码
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "缺少授权码",
			"code":  "MISSING_CODE",
		})
		return
	}

	// 交换 access token
	accessToken, err := h.exchangeCodeForToken(c.Request.Context(), code)
	if err != nil {
		logger.Error("api", "交换 access token 失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取访问令牌失败",
			"code":  "TOKEN_EXCHANGE_FAILED",
		})
		return
	}

	// 获取 GitHub 用户信息
	ghUser, err := h.getGitHubUser(c.Request.Context(), accessToken)
	if err != nil {
		logger.Error("api", "获取 GitHub 用户信息失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取用户信息失败",
			"code":  "USER_INFO_FAILED",
		})
		return
	}

	// 创建或更新用户
	user, err := h.findOrCreateUser(c.Request.Context(), ghUser)
	if err != nil {
		logger.Error("api", "创建/更新用户失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "用户处理失败",
			"code":  "USER_PROCESSING_FAILED",
		})
		return
	}

	// 检查用户状态
	if user.Status != storage.UserStatusActive {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "账号已被禁用",
			"code":  "USER_DISABLED",
		})
		return
	}

	// 创建会话
	sessionToken, err := h.createSession(c, user)
	if err != nil {
		logger.Error("api", "创建会话失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "创建会话失败",
			"code":  "SESSION_CREATION_FAILED",
		})
		return
	}

	// 设置会话 cookie（设置 SameSite=Lax 防止 CSRF）
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		SessionCookieName,
		sessionToken,
		int(SessionDefaultTTL.Seconds()),
		"/",
		"",
		true, // secure
		true, // httpOnly
	)

	// 返回用户信息和 token（供前端存储）
	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":         user.ID,
			"username":   user.Username,
			"avatar_url": user.AvatarURL,
			"email":      user.Email,
			"role":       user.Role,
		},
		"token": sessionToken,
	})
}

// GetCurrentUser 获取当前登录用户
// GET /api/auth/me
func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	// 尝试从 context 获取用户（需要先经过 OptionalAuth 中间件）
	user := GetUserFromContext(c)
	if user == nil {
		// 手动验证 token
		token := extractToken(c)
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "未登录",
				"code":  "NOT_LOGGED_IN",
			})
			return
		}

		// 验证 token
		authMiddleware := NewAuthMiddleware(h.storage)
		var err error
		user, _, err = authMiddleware.authenticateWithToken(c, token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "会话无效或已过期",
				"code":  "INVALID_SESSION",
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":         user.ID,
			"username":   user.Username,
			"avatar_url": user.AvatarURL,
			"email":      user.Email,
			"role":       user.Role,
			"status":     user.Status,
			"created_at": user.CreatedAt,
		},
	})
}

// Logout 登出
// POST /api/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	token := extractToken(c)
	if token != "" {
		// 撤销会话
		tokenHash := HashToken(token)
		userStorage, ok := h.storage.(storage.UserStorage)
		if ok {
			session, err := userStorage.GetSessionByTokenHash(c.Request.Context(), tokenHash)
			if err == nil && session != nil {
				_ = userStorage.RevokeSession(c.Request.Context(), session.ID)
			}
		}
	}

	// 清除 cookie（设置 SameSite 保持一致性）
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(SessionCookieName, "", -1, "/", "", true, true)

	c.JSON(http.StatusOK, gin.H{
		"message": "已登出",
	})
}

// =====================================================
// 内部方法
// =====================================================

// exchangeCodeForToken 用授权码交换 access token
func (h *AuthHandler) exchangeCodeForToken(ctx context.Context, code string) (string, error) {
	data := url.Values{
		"client_id":     {h.clientID},
		"client_secret": {h.clientSecret},
		"code":          {code},
		"redirect_uri":  {h.callbackURL},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", GitHubTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if result.Error != "" {
		return "", fmt.Errorf("GitHub OAuth error: %s - %s", result.Error, result.ErrorDesc)
	}

	return result.AccessToken, nil
}

// GitHubUser GitHub 用户信息
type GitHubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
	Email     string `json:"email"`
	Name      string `json:"name"`
}

// getGitHubUser 获取 GitHub 用户信息
func (h *AuthHandler) getGitHubUser(ctx context.Context, accessToken string) (*GitHubUser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", GitHubUserURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error: %d - %s", resp.StatusCode, string(body))
	}

	var user GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	return &user, nil
}

// findOrCreateUser 查找或创建用户
func (h *AuthHandler) findOrCreateUser(ctx context.Context, ghUser *GitHubUser) (*storage.User, error) {
	userStorage, ok := h.storage.(storage.UserStorage)
	if !ok {
		return nil, fmt.Errorf("存储层不支持 UserStorage 接口")
	}

	// 先按 GitHub ID 查找
	user, err := userStorage.GetUserByGitHubID(ctx, ghUser.ID)
	if err != nil {
		return nil, err
	}

	now := time.Now().Unix()

	if user != nil {
		// 更新用户信息
		user.Username = ghUser.Login
		user.AvatarURL = ghUser.AvatarURL
		if ghUser.Email != "" {
			user.Email = ghUser.Email
		}
		user.UpdatedAt = now

		if err := userStorage.UpdateUser(ctx, user); err != nil {
			return nil, err
		}
		return user, nil
	}

	// 创建新用户
	userID, err := generateUserID()
	if err != nil {
		return nil, err
	}

	user = &storage.User{
		ID:        userID,
		GitHubID:  ghUser.ID,
		Username:  ghUser.Login,
		AvatarURL: ghUser.AvatarURL,
		Email:     ghUser.Email,
		Role:      storage.UserRoleUser, // 默认普通用户
		Status:    storage.UserStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := userStorage.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	logger.Info("api", "新用户注册",
		"user_id", user.ID,
		"github_id", user.GitHubID,
		"username", user.Username)

	return user, nil
}

// createSession 创建会话
func (h *AuthHandler) createSession(c *gin.Context, user *storage.User) (string, error) {
	userStorage, ok := h.storage.(storage.UserStorage)
	if !ok {
		return "", fmt.Errorf("存储层不支持 UserStorage 接口")
	}

	// 生成 session token
	token, err := GenerateSessionToken()
	if err != nil {
		return "", err
	}

	now := time.Now().Unix()
	sessionID, err := generateSessionID()
	if err != nil {
		return "", err
	}

	session := &storage.UserSession{
		ID:         sessionID,
		UserID:     user.ID,
		TokenHash:  HashToken(token),
		ExpiresAt:  now + int64(SessionDefaultTTL.Seconds()),
		CreatedAt:  now,
		LastSeenAt: &now,
		IP:         getClientIP(c),
		UserAgent:  c.GetHeader("User-Agent"),
	}

	if err := userStorage.CreateSession(c.Request.Context(), session); err != nil {
		return "", err
	}

	return token, nil
}

// =====================================================
// 工具函数
// =====================================================

// generateOAuthState 生成 OAuth state
func generateOAuthState() (string, error) {
	bytes := make([]byte, OAuthStateLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// generateUserID 生成用户 ID（UUID）
func generateUserID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	// 设置 UUID 版本 4
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16]), nil
}

// generateSessionID 生成会话 ID（UUID）
func generateSessionID() (string, error) {
	return generateUserID() // 使用相同的 UUID 生成逻辑
}
