package api

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"monitor/internal/config"
	"monitor/internal/logger"
	"monitor/internal/storage"
)

// ===== 请求/响应结构体 =====

// StatusQueryRequest 批量状态查询请求（用于 POST /api/status/batch）
type StatusQueryRequest struct {
	Queries []StatusQuery `json:"queries"`
}

// StatusQuery 单个查询条件
type StatusQuery struct {
	Provider string `json:"provider"`
	Service  string `json:"service,omitempty"`
	Channel  string `json:"channel,omitempty"`
}

// StatusQueryResponse 状态查询响应
type StatusQueryResponse struct {
	AsOf    string              `json:"as_of"`
	Results []StatusQueryResult `json:"results"`
}

// StatusQueryResult 单个查询的返回结果
type StatusQueryResult struct {
	Query    StatusQuery             `json:"query"`
	Provider string                  `json:"provider,omitempty"` // 原始标识
	Services []StatusQueryService    `json:"services,omitempty"`
	Error    *StatusQueryErrorObject `json:"error,omitempty"`
}

// StatusQueryErrorObject 查询错误对象
type StatusQueryErrorObject struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// StatusQueryService 单个 service 的结果
type StatusQueryService struct {
	Name     string               `json:"name"` // 原始标识
	Channels []StatusQueryChannel `json:"channels"`
}

// StatusQueryChannel 单个 channel 的结果
type StatusQueryChannel struct {
	Name      string `json:"name"`                 // 原始标识（可能为空字符串）
	Status    string `json:"status"`               // up/down/degraded
	LatencyMs int    `json:"latency_ms,omitempty"` // 毫秒
	UpdatedAt string `json:"updated_at,omitempty"` // RFC3339 格式
}

// ===== 常量 =====

const (
	maxQueryGET  = 20 // GET 请求最多 20 组查询
	maxQueryPOST = 50 // POST 请求最多 50 组查询
)

// ===== Handler 方法 =====

// GetStatusQuery GET /api/status/query
// 支持两种查询方式：
// - 单查：provider（必填）, service（可选）, channel（可选）
// - 多查：q=provider/service/channel 可重复，最多 20 组
func (h *Handler) GetStatusQuery(c *gin.Context) {
	rawQs := c.QueryArray("q")

	var queries []StatusQuery
	if len(rawQs) > 0 {
		// 多查模式
		if len(rawQs) > maxQueryGET {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("q 参数最多支持 %d 组查询", maxQueryGET)})
			return
		}
		for _, raw := range rawQs {
			q, err := parsePackedQuery(raw)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			queries = append(queries, q)
		}
	} else {
		// 单查模式
		provider := strings.TrimSpace(c.Query("provider"))
		if provider == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "provider 为必填参数（或使用 q=provider/service/channel）"})
			return
		}
		queries = []StatusQuery{
			{
				Provider: provider,
				Service:  strings.TrimSpace(c.Query("service")),
				Channel:  strings.TrimSpace(c.Query("channel")),
			},
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resp, err := h.executeStatusQuery(ctx, queries)
	if err != nil {
		logger.FromContext(c.Request.Context(), "api").Error("GetStatusQuery 失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("查询失败: %v", err)})
		return
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, resp)
}

// PostStatusBatch POST /api/status/batch
// Body: {"queries":[{"provider":"X","service":"Y","channel":"Z"}, ...]}
// 最多支持 50 组查询
func (h *Handler) PostStatusBatch(c *gin.Context) {
	var req StatusQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("无效的 JSON: %v", err)})
		return
	}

	if len(req.Queries) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "queries 不能为空"})
		return
	}
	if len(req.Queries) > maxQueryPOST {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("queries 最多支持 %d 组查询", maxQueryPOST)})
		return
	}

	// 规范化输入
	for i := range req.Queries {
		req.Queries[i].Provider = strings.TrimSpace(req.Queries[i].Provider)
		req.Queries[i].Service = strings.TrimSpace(req.Queries[i].Service)
		req.Queries[i].Channel = strings.TrimSpace(req.Queries[i].Channel)
		if req.Queries[i].Provider == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "provider 为必填字段"})
			return
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()

	resp, err := h.executeStatusQuery(ctx, req.Queries)
	if err != nil {
		logger.FromContext(c.Request.Context(), "api").Error("PostStatusBatch 失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("查询失败: %v", err)})
		return
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, resp)
}

// ===== 内部方法 =====

// parsePackedQuery 解析紧凑格式的查询参数
// 格式：provider/service/channel（service 和 channel 可选）
func parsePackedQuery(raw string) (StatusQuery, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return StatusQuery{}, fmt.Errorf("q 不能为空")
	}

	parts := strings.Split(s, "/")
	if len(parts) > 3 {
		return StatusQuery{}, fmt.Errorf("q 格式错误: %s（应为 provider/service/channel）", raw)
	}

	q := StatusQuery{
		Provider: strings.TrimSpace(parts[0]),
	}
	if q.Provider == "" {
		return StatusQuery{}, fmt.Errorf("q 格式错误: %s（provider 不能为空）", raw)
	}
	if len(parts) >= 2 {
		q.Service = strings.TrimSpace(parts[1])
	}
	if len(parts) == 3 {
		q.Channel = strings.TrimSpace(parts[2])
	}
	return q, nil
}

// executeStatusQuery 执行状态查询核心逻辑
func (h *Handler) executeStatusQuery(ctx context.Context, queries []StatusQuery) (*StatusQueryResponse, error) {
	// 读取配置快照
	h.cfgMu.RLock()
	monitors := h.config.Monitors
	h.cfgMu.RUnlock()

	// 使用带 context 的 storage
	store := h.storage.WithContext(ctx)

	results := make([]StatusQueryResult, 0, len(queries))
	for _, q := range queries {
		// 展开查询目标（基于配置匹配）
		targets, queryErr := expandQueryTargets(monitors, q)
		if queryErr != nil {
			results = append(results, StatusQueryResult{
				Query: q,
				Error: queryErr,
			})
			continue
		}

		// 构建服务列表
		services := make([]StatusQueryService, 0, len(targets))
		for _, target := range targets {
			// 对 channels 按名称排序，保证输出稳定性
			sort.Slice(target.channels, func(i, j int) bool {
				return target.channels[i].name < target.channels[j].name
			})

			channelResults := make([]StatusQueryChannel, 0, len(target.channels))
			for _, ch := range target.channels {
				// 检查 context 是否已取消
				select {
				case <-ctx.Done():
					return nil, fmt.Errorf("查询超时或取消: %w", ctx.Err())
				default:
				}

				// 获取最新探测记录
				latest, err := store.GetLatest(target.provider, target.service, ch.name)
				if err != nil {
					return nil, fmt.Errorf("查询失败(provider=%s service=%s channel=%s): %w",
						target.provider, target.service, ch.name, err)
				}

				chResult := StatusQueryChannel{
					Name:   ch.name, // 返回原始标识
					Status: probeStatusToString(latest),
				}
				if latest != nil {
					chResult.LatencyMs = latest.Latency
					chResult.UpdatedAt = time.Unix(latest.Timestamp, 0).UTC().Format(time.RFC3339)
				}
				channelResults = append(channelResults, chResult)
			}

			services = append(services, StatusQueryService{
				Name:     target.service, // 返回原始标识
				Channels: channelResults,
			})
		}

		// 对 services 按名称排序，保证输出稳定性
		sort.Slice(services, func(i, j int) bool {
			return services[i].Name < services[j].Name
		})

		results = append(results, StatusQueryResult{
			Query:    q,
			Provider: targets[0].provider, // 返回原始标识
			Services: services,
		})
	}

	return &StatusQueryResponse{
		AsOf:    time.Now().UTC().Format(time.RFC3339),
		Results: results,
	}, nil
}

// channelInfo 通道信息（内部使用）
type channelInfo struct {
	name string // 原始配置值（用于数据库查询和 API 返回）
}

// serviceTarget 服务目标（内部使用）
type serviceTarget struct {
	provider string        // 原始配置值
	service  string        // 原始配置值
	channels []channelInfo // 通道列表
}

// expandQueryTargets 根据配置展开查询目标
// 支持 service/channel 为空时查询所有匹配项
func expandQueryTargets(monitors []config.ServiceConfig, q StatusQuery) ([]serviceTarget, *StatusQueryErrorObject) {
	queryProvider := strings.ToLower(strings.TrimSpace(q.Provider))
	queryService := strings.ToLower(strings.TrimSpace(q.Service))
	queryChannel := strings.ToLower(strings.TrimSpace(q.Channel))

	// 第一步：筛选匹配的 provider
	var providerMatches []config.ServiceConfig
	var originalProvider string
	for _, m := range monitors {
		if strings.ToLower(strings.TrimSpace(m.Provider)) == queryProvider {
			providerMatches = append(providerMatches, m)
			if originalProvider == "" {
				originalProvider = m.Provider
			}
		}
	}
	if len(providerMatches) == 0 {
		return nil, &StatusQueryErrorObject{Code: "NOT_FOUND", Message: "provider 不存在"}
	}

	// 第二步：按 service 分组
	serviceMap := make(map[string][]config.ServiceConfig) // key: lowercase service
	serviceOriginal := make(map[string]string)            // key: lowercase service -> original name

	for _, m := range providerMatches {
		svcLower := strings.ToLower(strings.TrimSpace(m.Service))
		serviceMap[svcLower] = append(serviceMap[svcLower], m)
		if _, ok := serviceOriginal[svcLower]; !ok {
			serviceOriginal[svcLower] = m.Service
		}
	}

	// 第三步：确定要查询的 service 列表
	var targetServices []string
	if queryService != "" {
		if _, ok := serviceMap[queryService]; !ok {
			return nil, &StatusQueryErrorObject{Code: "NOT_FOUND", Message: "service 不存在"}
		}
		targetServices = []string{queryService}
	} else {
		for svc := range serviceMap {
			targetServices = append(targetServices, svc)
		}
	}

	// 第四步：为每个 service 展开 channel
	var targets []serviceTarget
	for _, svcLower := range targetServices {
		svcConfigs := serviceMap[svcLower]

		// 收集该 service 下的 channel
		channelMap := make(map[string]channelInfo) // key: lowercase channel
		for _, m := range svcConfigs {
			chLower := strings.ToLower(strings.TrimSpace(m.Channel))
			if _, ok := channelMap[chLower]; !ok {
				channelMap[chLower] = channelInfo{
					name: m.Channel,
				}
			}
		}

		// 确定要查询的 channel 列表
		var channels []channelInfo
		if queryChannel != "" {
			ch, ok := channelMap[queryChannel]
			if !ok {
				return nil, &StatusQueryErrorObject{Code: "NOT_FOUND", Message: "channel 不存在"}
			}
			channels = []channelInfo{ch}
		} else {
			for _, ch := range channelMap {
				channels = append(channels, ch)
			}
		}

		targets = append(targets, serviceTarget{
			provider: originalProvider,
			service:  serviceOriginal[svcLower],
			channels: channels,
		})
	}

	return targets, nil
}

// probeStatusToString 将探测状态码转换为字符串
// 无数据时返回 "down"（符合 API 规范：只允许 up/down/degraded）
func probeStatusToString(latest *storage.ProbeRecord) string {
	if latest == nil {
		return "down"
	}
	switch latest.Status {
	case 1:
		return "up"
	case 2:
		return "degraded"
	default:
		return "down"
	}
}
