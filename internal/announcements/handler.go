package announcements

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler 公告 API 处理器
type Handler struct {
	service *Service
}

// NewHandler 创建公告 API 处理器
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// GetAnnouncements 处理 GET /api/announcements 请求
func (h *Handler) GetAnnouncements(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	snapshot, err := h.service.GetAnnouncements(ctx)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{
				"message": "公告服务暂不可用",
				"detail":  err.Error(),
			},
		})
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
