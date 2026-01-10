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
	Board     string `json:"board,omitempty"`      // hot/cold（用于订阅校验等场景）
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
	boardsEnabled := h.config.Boards.Enabled
	h.cfgMu.RUnlock()

	// 使用带 context 的 storage
	store := h.storage.WithContext(ctx)

	results := make([]StatusQueryResult, 0, len(queries))
	for _, q := range queries {
		// 展开查询目标（基于配置匹配）
		targets, queryErr := expandQueryTargets(monitors, boardsEnabled, q)
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

				// 聚合该 channel 下的所有 model，取最差状态
				worstStatus := -1
				var worstRecord *storage.ProbeRecord
				for _, model := range ch.models {
					// 检查 context 是否已取消
					select {
					case <-ctx.Done():
						return nil, fmt.Errorf("查询超时或取消: %w", ctx.Err())
					default:
					}

					latest, err := store.GetLatest(target.provider, target.service, ch.name, model)
					if err != nil {
						return nil, fmt.Errorf("查询失败(provider=%s service=%s channel=%s model=%s): %w",
							target.provider, target.service, ch.name, model, err)
					}

					status := -1
					if latest != nil {
						status = latest.Status
					}

					newWorst := pickWorstStatus(worstStatus, status)
					if newWorst != worstStatus {
						worstStatus = newWorst
						worstRecord = latest
						continue
					}

					// 同一严重程度时，优先选择更新时间更近的记录作为展示信息
					if latest != nil && worstRecord != nil && latest.Status == worstRecord.Status && latest.Timestamp > worstRecord.Timestamp {
						worstRecord = latest
					}
				}

				chResult := StatusQueryChannel{
					Name:   ch.name, // 返回原始标识
					Status: statusIntToString(worstStatus),
					Board:  ch.board,
				}
				if worstRecord != nil {
					chResult.LatencyMs = worstRecord.Latency
					chResult.UpdatedAt = time.Unix(worstRecord.Timestamp, 0).UTC().Format(time.RFC3339)
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
	name   string   // 原始配置值（用于数据库查询和 API 返回）
	board  string   // hot/cold（channel 级别的 board 状态）
	models []string // 该 channel 下的所有 model（原始配置值）
}

// serviceTarget 服务目标（内部使用）
type serviceTarget struct {
	provider string        // 原始配置值
	service  string        // 原始配置值
	channels []channelInfo // 通道列表
}

// expandQueryTargets 根据配置展开查询目标
// 支持 service/channel 为空时查询所有匹配项
// boardsEnabled: 是否启用热板/冷板功能，启用时计算 channel 级别的 board 状态
func expandQueryTargets(monitors []config.ServiceConfig, boardsEnabled bool, q StatusQuery) ([]serviceTarget, *StatusQueryErrorObject) {
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

	// 第四步：为每个 service 展开 channel（并聚合每个 channel 的 models）
	var targets []serviceTarget
	for _, svcLower := range targetServices {
		svcConfigs := serviceMap[svcLower]

		// 收集该 service 下的 channel（并聚合每个 channel 的 models）
		channelMap := make(map[string]*channelInfo)             // key: lowercase channel
		modelSetByChannel := make(map[string]map[string]string) // chLower -> modelLower -> original model

		// board 统计：记录每个 channel 下是否有 hot/cold model
		type boardStat struct {
			hasHot  bool
			hasCold bool
		}
		boardStatByChannel := make(map[string]*boardStat) // chLower -> board stat

		for _, m := range svcConfigs {
			chLower := strings.ToLower(strings.TrimSpace(m.Channel))
			if _, ok := channelMap[chLower]; !ok {
				channelMap[chLower] = &channelInfo{
					name:  m.Channel,
					board: "hot", // 默认 hot
				}
			}

			// 统计 board（仅当 boardsEnabled 时有意义）
			// 注意：跳过 Disabled 的监测项，避免 disabled 的 hot model 污染 cold 判定
			if !m.Disabled {
				if _, ok := boardStatByChannel[chLower]; !ok {
					boardStatByChannel[chLower] = &boardStat{}
				}
				boardLower := strings.ToLower(strings.TrimSpace(m.Board))
				if boardLower == "cold" {
					boardStatByChannel[chLower].hasCold = true
				} else {
					// 空值或 "hot" 都视为 hot
					boardStatByChannel[chLower].hasHot = true
				}
			}

			modelLower := strings.ToLower(strings.TrimSpace(m.Model))
			if _, ok := modelSetByChannel[chLower]; !ok {
				modelSetByChannel[chLower] = make(map[string]string)
			}
			if _, ok := modelSetByChannel[chLower][modelLower]; !ok {
				modelSetByChannel[chLower][modelLower] = strings.TrimSpace(m.Model)
			}
		}

		// 为每个 channel 填充 models 列表和 board 状态
		for chLower, ch := range channelMap {
			for _, model := range modelSetByChannel[chLower] {
				ch.models = append(ch.models, model)
			}
			if len(ch.models) == 0 {
				// 兼容旧数据：model 为空时使用空字符串查询
				ch.models = []string{""}
			} else {
				sort.Strings(ch.models)
			}

			// 计算 channel 的 board：仅当 boards 启用且该 channel 全为 cold 时，视为 cold
			if boardsEnabled {
				if st, ok := boardStatByChannel[chLower]; ok {
					if st.hasCold && !st.hasHot {
						ch.board = "cold"
					} else {
						ch.board = "hot"
					}
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
			channels = []channelInfo{*ch}
		} else {
			for _, ch := range channelMap {
				channels = append(channels, *ch)
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

// statusIntToString 将状态码转换为字符串
// 无数据/缺失（-1）时返回 "down"（符合 API 规范：只允许 up/down/degraded）
func statusIntToString(status int) string {
	switch status {
	case 1:
		return "up"
	case 2:
		return "degraded"
	default:
		return "down"
	}
}
