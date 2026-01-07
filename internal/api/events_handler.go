package api

import (
	"crypto/subtle"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"monitor/internal/storage"
)

// EventsResponse 事件列表响应
type EventsResponse struct {
	Events []EventItem `json:"events"`
	Meta   EventsMeta  `json:"meta"`
}

// EventItem 单个事件
type EventItem struct {
	ID              int64          `json:"id"`
	Provider        string         `json:"provider"`
	Service         string         `json:"service"`
	Channel         string         `json:"channel,omitempty"`
	Model           string         `json:"model,omitempty"`
	Type            string         `json:"type"`
	FromStatus      int            `json:"from_status"`
	ToStatus        int            `json:"to_status"`
	TriggerRecordID int64          `json:"trigger_record_id"`
	ObservedAt      int64          `json:"observed_at"`
	CreatedAt       int64          `json:"created_at"`
	Meta            map[string]any `json:"meta,omitempty"`
}

// EventsMeta 事件列表元数据
type EventsMeta struct {
	NextSinceID int64 `json:"next_since_id"`
	HasMore     bool  `json:"has_more"`
	Count       int   `json:"count"`
}

// LatestEventResponse 最新事件ID响应
type LatestEventResponse struct {
	LatestID  int64 `json:"latest_id"`
	Timestamp int64 `json:"timestamp,omitempty"`
}

// GetEvents 获取事件列表
// GET /api/events?since_id=0&limit=20&provider=xxx&service=xxx&channel=xxx&types=DOWN,UP
// limit 默认 20，最大 100
func (h *Handler) GetEvents(c *gin.Context) {
	// 检查 API Token（如果配置了）
	if !h.checkEventsAPIToken(c) {
		return
	}

	// 解析查询参数
	sinceID, _ := strconv.ParseInt(c.DefaultQuery("since_id", "0"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	// 限制范围：默认 20，最大 100
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// 构建过滤器
	var filters *storage.EventFilters
	provider := c.Query("provider")
	service := c.Query("service")
	channel := c.Query("channel")
	typesStr := c.Query("types")

	if provider != "" || service != "" || channel != "" || typesStr != "" {
		filters = &storage.EventFilters{
			Provider: provider,
			Service:  service,
			Channel:  channel,
		}

		if typesStr != "" {
			types := strings.Split(typesStr, ",")
			for _, t := range types {
				t = strings.TrimSpace(t)
				if t == "DOWN" || t == "UP" {
					filters.Types = append(filters.Types, storage.EventType(t))
				}
			}
		}
	}

	// 查询事件
	events, err := h.storage.GetStatusEvents(sinceID, limit+1, filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "查询事件失败",
		})
		return
	}

	// 判断是否还有更多
	hasMore := len(events) > limit
	if hasMore {
		events = events[:limit]
	}

	// 计算下一个 since_id
	var nextSinceID int64
	if len(events) > 0 {
		nextSinceID = events[len(events)-1].ID
	} else {
		nextSinceID = sinceID
	}

	// 构建响应
	items := make([]EventItem, 0, len(events))
	for _, e := range events {
		items = append(items, EventItem{
			ID:              e.ID,
			Provider:        e.Provider,
			Service:         e.Service,
			Channel:         e.Channel,
			Model:           e.Model,
			Type:            string(e.EventType),
			FromStatus:      e.FromStatus,
			ToStatus:        e.ToStatus,
			TriggerRecordID: e.TriggerRecordID,
			ObservedAt:      e.ObservedAt,
			CreatedAt:       e.CreatedAt,
			Meta:            e.Meta,
		})
	}

	c.JSON(http.StatusOK, EventsResponse{
		Events: items,
		Meta: EventsMeta{
			NextSinceID: nextSinceID,
			HasMore:     hasMore,
			Count:       len(items),
		},
	})
}

// GetLatestEventID 获取最新事件ID
// GET /api/events/latest
func (h *Handler) GetLatestEventID(c *gin.Context) {
	// 检查 API Token（如果配置了）
	if !h.checkEventsAPIToken(c) {
		return
	}

	latestID, err := h.storage.GetLatestEventID()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "查询最新事件ID失败",
		})
		return
	}

	c.JSON(http.StatusOK, LatestEventResponse{
		LatestID: latestID,
	})
}

// checkEventsAPIToken 检查事件 API Token（强制鉴权）
// 如果未配置 api_token，返回 503 拒绝所有请求
// 返回 true 表示验证通过，false 表示验证失败（已返回错误响应）
func (h *Handler) checkEventsAPIToken(c *gin.Context) bool {
	h.cfgMu.RLock()
	apiToken := h.config.Events.APIToken
	h.cfgMu.RUnlock()

	// 未配置 token 时拒绝所有请求
	if apiToken == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "events API 未配置，请设置 EVENTS_API_TOKEN 环境变量",
		})
		return false
	}

	// 验证 Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "缺少 Authorization 请求头",
		})
		return false
	}

	// 支持 "Bearer <token>" 格式
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Authorization 格式错误，应为: Bearer <token>",
		})
		return false
	}

	token := strings.TrimPrefix(authHeader, bearerPrefix)
	// 使用恒定时间比较，防止时序攻击
	if subtle.ConstantTimeCompare([]byte(token), []byte(apiToken)) != 1 {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "API token 无效",
		})
		return false
	}

	return true
}
