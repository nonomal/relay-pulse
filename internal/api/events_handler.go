package api

import (
	"crypto/subtle"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"monitor/internal/logger"
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

	// 解析查询参数（仅在参数缺省时使用默认值；显式传参必须是合法整数）
	sinceID := int64(0)
	if rawSinceID, ok := c.GetQuery("since_id"); ok {
		parsedSinceID, err := strconv.ParseInt(strings.TrimSpace(rawSinceID), 10, 64)
		if err != nil || parsedSinceID < 0 {
			apiError(c, http.StatusBadRequest, ErrCodeInvalidParam, "since_id 必须为大于等于 0 的整数")
			return
		}
		sinceID = parsedSinceID
	}

	limit := 20
	if rawLimit, ok := c.GetQuery("limit"); ok {
		parsedLimit, err := strconv.Atoi(strings.TrimSpace(rawLimit))
		if err != nil || parsedLimit <= 0 {
			apiError(c, http.StatusBadRequest, ErrCodeInvalidParam, "limit 必须为 1-100 的整数")
			return
		}
		if parsedLimit > 100 {
			parsedLimit = 100
		}
		limit = parsedLimit
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
				if t == "" {
					continue
				}
				if t != "DOWN" && t != "UP" {
					apiError(c, http.StatusBadRequest, ErrCodeInvalidParam, "types 仅支持 DOWN、UP（逗号分隔）")
					return
				}
				filters.Types = append(filters.Types, storage.EventType(t))
			}
		}
	}

	// 查询事件
	events, err := h.storage.GetStatusEvents(sinceID, limit+1, filters)
	if err != nil {
		logger.FromContext(c.Request.Context(), "api").Error("GetEvents 失败", "since_id", sinceID, "limit", limit, "error", err)
		apiError(c, http.StatusInternalServerError, ErrCodeInternalError, "查询事件失败，请稍后再试")
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
		logger.FromContext(c.Request.Context(), "api").Error("GetLatestEventID 失败", "error", err)
		apiError(c, http.StatusInternalServerError, ErrCodeInternalError, "查询最新事件 ID 失败，请稍后再试")
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
		apiError(c, http.StatusServiceUnavailable, ErrCodeServiceUnavailable, "events API 暂不可用")
		return false
	}

	// 验证 Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		apiError(c, http.StatusUnauthorized, ErrCodeUnauthorized, "缺少 Authorization 请求头")
		return false
	}

	// 支持 "Bearer <token>" 格式
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		apiError(c, http.StatusUnauthorized, ErrCodeUnauthorized, "Authorization 格式错误，应为 Bearer <token>")
		return false
	}

	token := strings.TrimPrefix(authHeader, bearerPrefix)
	// 使用恒定时间比较，防止时序攻击
	if subtle.ConstantTimeCompare([]byte(token), []byte(apiToken)) != 1 {
		apiError(c, http.StatusForbidden, ErrCodeForbidden, "API token 无效")
		return false
	}

	return true
}
