package announcements

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"monitor/internal/api"
	"monitor/internal/logger"
)

// Handler 公告 API 处理器（支持热更新时动态替换 Service 实例）
type Handler struct {
	mu      sync.RWMutex
	service *Service
}

// NewHandler 创建公告 API 处理器
// service 可以为 nil（功能未启用时），后续通过 SetService 动态注入
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// SetService 热替换公告服务实例（并发安全）
func (h *Handler) SetService(service *Service) {
	h.mu.Lock()
	h.service = service
	h.mu.Unlock()
}

// currentService 获取当前服务实例（并发安全）
func (h *Handler) currentService() *Service {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.service
}

// GetAnnouncements 处理 GET /api/announcements 请求
func (h *Handler) GetAnnouncements(c *gin.Context) {
	service := h.currentService()
	if service == nil {
		// 返回 disabled 状态快照（200），让前端优雅停止轮询而非进入错误态
		snap := &Snapshot{
			Enabled:   false,
			Items:     []Announcement{},
			APIMaxAge: 3600,
		}
		snap.Fetch.TTLSeconds = 3600
		c.Header("Cache-Control", "private, max-age=3600")
		c.JSON(http.StatusOK, snap)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	snapshot, err := service.GetAnnouncements(ctx)
	if err != nil {
		logger.FromContext(c.Request.Context(), "announcements").Error("GetAnnouncements 失败", "error", err)
		api.WriteAPIError(c, http.StatusServiceUnavailable, api.ErrCodeServiceUnavailable, "公告服务暂不可用")
		return
	}

	// 设置缓存头
	maxAge := snapshot.APIMaxAge
	if maxAge <= 0 {
		maxAge = 60
	}
	c.Header("Cache-Control", fmt.Sprintf("private, max-age=%d", maxAge))

	// 如果数据是陈旧的，添加警告头
	if snapshot.Fetch.Stale {
		c.Header("X-Data-Stale", "true")
	}

	c.JSON(http.StatusOK, snapshot)
}
