package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"notifier/internal/config"
	"notifier/internal/storage"
)

// QQCallbackHandler QQ 回调处理器接口
type QQCallbackHandler interface {
	HandleCallback(w http.ResponseWriter, r *http.Request)
}

// Server HTTP API 服务器
type Server struct {
	cfg     *config.Config
	storage storage.Storage
	server  *http.Server
	mux     *http.ServeMux
}

// NewServer 创建 API 服务器
func NewServer(cfg *config.Config, store storage.Storage) *Server {
	s := &Server{
		cfg:     cfg,
		storage: store,
		mux:     http.NewServeMux(),
	}

	// 健康检查
	s.mux.HandleFunc("GET /health", s.handleHealth)

	// 绑定 token API
	s.mux.HandleFunc("POST /api/bind-token", s.handleCreateBindToken)
	s.mux.HandleFunc("GET /api/bind-token/{token}", s.handleGetBindToken)

	s.server = &http.Server{
		Addr:         cfg.API.Addr,
		Handler:      corsMiddleware(loggingMiddleware(s.mux)),
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
	var req CreateBindTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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

// 中间件

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

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
