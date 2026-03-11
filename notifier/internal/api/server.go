package api

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"notifier/internal/config"
	"notifier/internal/storage"
)

// QQCallbackHandler QQ 回调处理器接口
type QQCallbackHandler interface {
	HandleCallback(w http.ResponseWriter, r *http.Request)
}

const (
	// bindTokenMaxBodyBytes bind-token 请求体上限（64KB，收藏列表不需要更大）
	bindTokenMaxBodyBytes int64 = 64 << 10
	// bindTokenRateBurst 每 IP 令牌桶容量
	bindTokenRateBurst = 10
	// bindTokenRateWindow 令牌桶回填窗口
	bindTokenRateWindow = time.Minute
)

// Server HTTP API 服务器
type Server struct {
	cfg              *config.Config
	storage          storage.Storage
	server           *http.Server
	mux              *http.ServeMux
	bindTokenLimiter *ipRateLimiter
}

// NewServer 创建 API 服务器
func NewServer(cfg *config.Config, store storage.Storage) *Server {
	s := &Server{
		cfg:              cfg,
		storage:          store,
		mux:              http.NewServeMux(),
		bindTokenLimiter: newIPRateLimiter(bindTokenRateBurst, bindTokenRateWindow),
	}

	// 健康检查
	s.mux.HandleFunc("GET /health", s.handleHealth)

	// 绑定 token API — 链式中间件：rate limit → auth → handler
	// 限流在鉴权前，防止 Bearer token 猜测绕过节流
	bindTokenHandler := http.Handler(http.HandlerFunc(s.handleCreateBindToken))
	bindTokenHandler = s.bindTokenAuthMiddleware(bindTokenHandler)
	bindTokenHandler = s.bindTokenRateLimitMiddleware(bindTokenHandler)
	s.mux.Handle("POST /api/bind-token", bindTokenHandler)
	s.mux.HandleFunc("GET /api/bind-token/{token}", s.handleGetBindToken)

	s.server = &http.Server{
		Addr:         cfg.API.Addr,
		Handler:      corsMiddleware(cfg.API.CORSAllowedOrigins, loggingMiddleware(s.mux)),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// RegisterQQCallback 注册 QQ Bot 回调路由
func (s *Server) RegisterQQCallback(path string, handler QQCallbackHandler) {
	s.mux.HandleFunc("POST "+path, handler.HandleCallback)
	slog.Info("注册 QQ 回调路由", "path", path)
}

// Start 启动服务器
func (s *Server) Start() error {
	slog.Info("HTTP API 服务器启动", "addr", s.cfg.API.Addr)
	return s.server.ListenAndServe()
}

// Shutdown 优雅关闭服务器
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// handleHealth 健康检查
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// CreateBindTokenRequest 创建绑定 token 请求
type CreateBindTokenRequest struct {
	Favorites []string `json:"favorites"`
}

// CreateBindTokenResponse 创建绑定 token 响应
type CreateBindTokenResponse struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expires_in"` // 秒
	DeepLink  string `json:"deep_link"`
}

// handleCreateBindToken 创建绑定 token
func (s *Server) handleCreateBindToken(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, bindTokenMaxBodyBytes)

	var req CreateBindTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "请求体过大")
			return
		}
		writeError(w, http.StatusBadRequest, "无效的请求体")
		return
	}

	if len(req.Favorites) == 0 {
		writeError(w, http.StatusBadRequest, "收藏列表不能为空")
		return
	}

	// 生成随机 token
	token, err := generateToken(16)
	if err != nil {
		slog.Error("生成 token 失败", "error", err)
		writeError(w, http.StatusInternalServerError, "内部错误")
		return
	}

	// 序列化收藏列表
	favoritesJSON, err := json.Marshal(req.Favorites)
	if err != nil {
		slog.Error("序列化收藏列表失败", "error", err)
		writeError(w, http.StatusInternalServerError, "内部错误")
		return
	}

	now := time.Now()
	expiresAt := now.Add(s.cfg.Limits.BindTokenTTL)

	bindToken := &storage.BindToken{
		Token:     token,
		Favorites: string(favoritesJSON),
		ExpiresAt: expiresAt.Unix(),
		CreatedAt: now.Unix(),
	}

	if err := s.storage.CreateBindToken(r.Context(), bindToken); err != nil {
		slog.Error("保存绑定 token 失败", "error", err)
		writeError(w, http.StatusInternalServerError, "内部错误")
		return
	}

	// 生成 Telegram deeplink
	deepLink := "https://t.me/" + s.cfg.Telegram.BotUsername + "?start=" + token

	resp := CreateBindTokenResponse{
		Token:     token,
		ExpiresIn: int(s.cfg.Limits.BindTokenTTL.Seconds()),
		DeepLink:  deepLink,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// GetBindTokenResponse 获取绑定 token 响应
type GetBindTokenResponse struct {
	Favorites []string `json:"favorites"`
	Used      bool     `json:"used"`
}

// handleGetBindToken 获取并消费绑定 token
func (s *Server) handleGetBindToken(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "缺少 token 参数")
		return
	}

	// 消费 token（标记已使用）
	bindToken, err := s.storage.ConsumeBindToken(r.Context(), token)
	if err != nil {
		slog.Warn("消费绑定 token 失败", "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if bindToken == nil {
		writeError(w, http.StatusNotFound, "token 不存在")
		return
	}

	// 解析收藏列表
	var favorites []string
	if err := json.Unmarshal([]byte(bindToken.Favorites), &favorites); err != nil {
		slog.Error("解析收藏列表失败", "error", err)
		writeError(w, http.StatusInternalServerError, "内部错误")
		return
	}

	resp := GetBindTokenResponse{
		Favorites: favorites,
		Used:      true,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// 辅助函数

func writeError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func generateToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// ── IP 速率限制（令牌桶） ──────────────────────────────

type ipRateLimiter struct {
	mu          sync.Mutex
	buckets     map[string]*rateBucket
	capacity    float64
	refillRate  float64 // tokens/second
	ttl         time.Duration
	lastCleanup time.Time
}

type rateBucket struct {
	tokens     float64
	lastRefill time.Time
	lastSeen   time.Time
}

func newIPRateLimiter(capacity int, window time.Duration) *ipRateLimiter {
	if capacity <= 0 {
		capacity = 1
	}
	if window <= 0 {
		window = time.Minute
	}
	return &ipRateLimiter{
		buckets:    make(map[string]*rateBucket),
		capacity:   float64(capacity),
		refillRate: float64(capacity) / window.Seconds(),
		ttl:        2 * window,
	}
}

// Allow 检查 IP 是否允许请求。返回是否允许及建议重试间隔。
func (l *ipRateLimiter) Allow(ip string, now time.Time) (bool, time.Duration) {
	if ip == "" {
		ip = "unknown"
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// 定期清理过期桶
	if l.lastCleanup.IsZero() || now.Sub(l.lastCleanup) >= l.ttl {
		cutoff := now.Add(-l.ttl)
		for key, b := range l.buckets {
			if b.lastSeen.Before(cutoff) {
				delete(l.buckets, key)
			}
		}
		l.lastCleanup = now
	}

	b, ok := l.buckets[ip]
	if !ok {
		b = &rateBucket{tokens: l.capacity, lastRefill: now, lastSeen: now}
		l.buckets[ip] = b
	}

	// 回填令牌
	if elapsed := now.Sub(b.lastRefill).Seconds(); elapsed > 0 {
		b.tokens = math.Min(l.capacity, b.tokens+elapsed*l.refillRate)
		b.lastRefill = now
	}
	b.lastSeen = now

	if b.tokens < 1 {
		retryAfter := time.Duration(math.Ceil((1-b.tokens)/l.refillRate)) * time.Second
		if retryAfter < time.Second {
			retryAfter = time.Second
		}
		return false, retryAfter
	}

	b.tokens--
	return true, 0
}

// ── 中间件 ──────────────────────────────────────────────

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Debug("HTTP 请求",
			"method", r.Method,
			"path", r.URL.Path,
			"duration", time.Since(start),
		)
	})
}

// bindTokenAuthMiddleware 可选 Bearer token 鉴权（仅当 AuthToken 非空时启用）
func (s *Server) bindTokenAuthMiddleware(next http.Handler) http.Handler {
	expectedToken := s.cfg.API.AuthToken
	if expectedToken == "" {
		return next // 未配置 → 跳过鉴权（向后兼容）
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := strings.TrimSpace(r.Header.Get("Authorization"))
		parts := strings.Fields(auth)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") ||
			subtle.ConstantTimeCompare([]byte(parts[1]), []byte(expectedToken)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="relay-pulse-notifier"`)
			slog.Warn("bind-token 鉴权失败", "client_ip", clientIP(r, s.cfg.API.TrustProxy))
			writeError(w, http.StatusUnauthorized, "未授权")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// bindTokenRateLimitMiddleware IP 维度令牌桶限流
func (s *Server) bindTokenRateLimitMiddleware(next http.Handler) http.Handler {
	if s.bindTokenLimiter == nil {
		return next
	}

	trustProxy := s.cfg.API.TrustProxy

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r, trustProxy)
		allowed, retryAfter := s.bindTokenLimiter.Allow(ip, time.Now())
		if !allowed {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(retryAfter.Seconds())))
			slog.Warn("bind-token 请求限流", "client_ip", ip, "retry_after", retryAfter)
			writeError(w, http.StatusTooManyRequests, "请求过于频繁，请稍后再试")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// corsMiddleware 基于配置的 CORS 中间件
func corsMiddleware(allowedOrigins []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		allowedOrigin, matched := resolveCORSOrigin(origin, allowedOrigins)

		if allowedOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			if allowedOrigin != "*" {
				w.Header().Add("Vary", "Origin")
			}
		}

		if matched {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// resolveCORSOrigin 匹配请求 Origin 与允许列表，返回应设置的 ACAO 值和是否匹配。
func resolveCORSOrigin(origin string, allowedOrigins []string) (string, bool) {
	// 空列表或含 "*" → 通配符模式
	if len(allowedOrigins) == 0 {
		return "*", true
	}
	for _, o := range allowedOrigins {
		if o == "*" {
			return "*", true
		}
	}
	// 无 Origin 头（非浏览器请求）→ 不设 ACAO 但允许通过
	if origin == "" {
		return "", true
	}
	// 精确匹配
	for _, o := range allowedOrigins {
		if origin == o {
			return origin, true
		}
	}
	return "", false
}

// clientIP 提取客户端 IP。trustProxy 为 true 时信任 X-Forwarded-For / X-Real-IP，
// 否则仅使用 RemoteAddr（防止攻击者伪造代理头绕过限流）。
func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
			if parts := strings.SplitN(xff, ",", 2); len(parts) > 0 {
				if ip := strings.TrimSpace(parts[0]); ip != "" {
					return ip
				}
			}
		}
		if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
			return realIP
		}
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
