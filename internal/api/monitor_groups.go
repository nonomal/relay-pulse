package api

import (
	"context"
	"strings"
	"time"

	"monitor/internal/config"
	"monitor/internal/logger"
	"monitor/internal/storage"
)

// StatusPoint 状态点（当前状态快照）
type StatusPoint struct {
	Status    int   `json:"status"`
	Latency   int   `json:"latency"`
	Timestamp int64 `json:"timestamp"`
}

// MonitorLayer 监测层（单个 model 的探测结果）
type MonitorLayer struct {
	Model         string              `json:"model"`
	LayerOrder    int                 `json:"layer_order"` // 0=父，1+=子（按配置顺序）
	CurrentStatus StatusPoint         `json:"current_status"`
	Timeline      []storage.TimePoint `json:"timeline"`
}

// MonitorGroup 监测组（父子/多模型结构的聚合单元）
type MonitorGroup struct {
	Provider      string                 `json:"provider"`
	ProviderName  string                 `json:"provider_name,omitempty"`
	ProviderSlug  string                 `json:"provider_slug"`
	ProviderURL   string                 `json:"provider_url"`
	Service       string                 `json:"service"`
	ServiceName   string                 `json:"service_name,omitempty"`
	Category      string                 `json:"category"`
	Sponsor       string                 `json:"sponsor"`
	SponsorURL    string                 `json:"sponsor_url"`
	SponsorLevel  config.SponsorLevel    `json:"sponsor_level,omitempty"`
	Risks         []config.RiskBadge     `json:"risks,omitempty"`
	Badges        []config.ResolvedBadge `json:"badges,omitempty"`
	PriceMin      *float64               `json:"price_min,omitempty"`
	PriceMax      *float64               `json:"price_max,omitempty"`
	ListedDays    *int                   `json:"listed_days,omitempty"`
	Channel       string                 `json:"channel"`
	ChannelName   string                 `json:"channel_name,omitempty"`
	Board         string                 `json:"board"`
	ColdReason    string                 `json:"cold_reason,omitempty"`
	ProbeURL      string                 `json:"probe_url,omitempty"`
	TemplateName  string                 `json:"template_name,omitempty"`
	IntervalMs    int64                  `json:"interval_ms"`
	SlowLatencyMs int64                  `json:"slow_latency_ms"`

	CurrentStatus int            `json:"current_status"` // 组级最差状态：0>2>1>-1
	Layers        []MonitorLayer `json:"layers"`
}

// filterMonitorsForGroups 过滤监测项（不去重，保留配置顺序）
func (h *Handler) filterMonitorsForGroups(monitors []config.ServiceConfig, provider, service, board string, boardsEnabled, includeHidden bool) []config.ServiceConfig {
	var filtered []config.ServiceConfig

	for _, task := range monitors {
		// 始终过滤已禁用的监测项（不探测、不存储、不展示）
		if task.Disabled {
			continue
		}

		// 过滤隐藏的监测项（除非显式要求包含）
		if !includeHidden && task.Hidden {
			continue
		}

		// 板块过滤（仅当 boards 功能启用时生效）
		if boardsEnabled && board != "all" {
			if board != task.Board {
				continue
			}
		}

		normalizedTaskProvider := strings.ToLower(strings.TrimSpace(task.Provider))

		// 过滤（统一使用 provider 名称匹配）
		if provider != "all" && provider != normalizedTaskProvider {
			continue
		}
		if service != "all" && service != task.Service {
			continue
		}

		filtered = append(filtered, task)
	}

	return filtered
}

// toStatusPoint 将 CurrentStatus 转换为 StatusPoint
func toStatusPoint(current *CurrentStatus) StatusPoint {
	if current == nil {
		return StatusPoint{Status: -1}
	}
	return StatusPoint{
		Status:    current.Status,
		Latency:   current.Latency,
		Timestamp: current.Timestamp,
	}
}

// statusSeverity 返回状态的严重程度（数值越大越严重）
func statusSeverity(status int) int {
	switch status {
	case 0: // 红色（不可用）
		return 3
	case 2: // 黄色（降级）
		return 2
	case 1: // 绿色（正常）
		return 1
	default: // -1（无数据）或未知
		return 0
	}
}

// pickWorstStatus 选择两个状态中更严重的一个
func pickWorstStatus(a, b int) int {
	if statusSeverity(b) > statusSeverity(a) {
		return b
	}
	return a
}

// buildMonitorGroupFromParent 从父通道配置构建 MonitorGroup 的元数据部分
func buildMonitorGroupFromParent(parent config.ServiceConfig, enableBadges bool) MonitorGroup {
	// 生成 slug：优先使用配置的 provider_slug，回退到 provider 小写
	slug := parent.ProviderSlug
	if slug == "" {
		slug = strings.ToLower(strings.TrimSpace(parent.Provider))
	}

	// 计算收录天数（从 listed_since 到今天）
	var listedDays *int
	if parent.ListedSince != "" {
		if listedDate, err := time.Parse("2006-01-02", parent.ListedSince); err == nil {
			days := int(time.Since(listedDate).Hours() / 24)
			if days < 0 {
				days = 0 // 防止未来日期导致负数
			}
			listedDays = &days
		}
	}

	// 当徽标系统禁用时，清空所有徽标相关字段（与 buildMonitorResult 一致）
	sponsorLevel := parent.SponsorLevel
	risks := parent.Risks
	badges := parent.ResolvedBadges
	intervalMs := parent.IntervalDuration.Milliseconds()
	if !enableBadges {
		sponsorLevel = ""
		risks = nil
		badges = nil
		intervalMs = 0
	}

	return MonitorGroup{
		Provider:      parent.Provider,
		ProviderName:  parent.ProviderName,
		ProviderSlug:  slug,
		ProviderURL:   parent.ProviderURL,
		Service:       parent.Service,
		ServiceName:   parent.ServiceName,
		Category:      parent.Category,
		Sponsor:       parent.Sponsor,
		SponsorURL:    parent.SponsorURL,
		SponsorLevel:  sponsorLevel,
		Risks:         risks,
		Badges:        badges,
		PriceMin:      parent.PriceMin,
		PriceMax:      parent.PriceMax,
		ListedDays:    listedDays,
		Channel:       parent.Channel,
		ChannelName:   parent.ChannelName,
		Board:         parent.Board,
		ColdReason:    parent.ColdReason,
		ProbeURL:      sanitizeProbeURL(parent.URL),
		TemplateName:  parent.BodyTemplateName,
		IntervalMs:    intervalMs,
		SlowLatencyMs: parent.SlowLatencyDuration.Milliseconds(),
		CurrentStatus: -1,
		Layers:        make([]MonitorLayer, 0),
	}
}

// buildMonitorGroups 构建父子/多模型 groups（仅包含 Model 非空的监测项）
func (h *Handler) buildMonitorGroups(
	ctx context.Context,
	monitors []config.ServiceConfig,
	since, endTime time.Time,
	period string,
	degradedWeight float64,
	timeFilter *TimeFilter,
	enableBadges bool,
	enableDBTimelineAgg bool,
	enableConcurrent bool,
	concurrentLimit int,
	enableBatchQuery bool,
	batchQueryMaxKeys int,
) ([]MonitorGroup, error) {
	if len(monitors) == 0 {
		return make([]MonitorGroup, 0), nil
	}

	// 筛选出有 model 的监测项
	layerTasks := make([]config.ServiceConfig, 0, len(monitors))
	for _, task := range monitors {
		if strings.TrimSpace(task.Model) == "" {
			continue
		}
		layerTasks = append(layerTasks, task)
	}
	if len(layerTasks) == 0 {
		return make([]MonitorGroup, 0), nil
	}

	// 查询每一层的 timeline/current（复用既有 batch/concurrent/serial 策略）
	var layerResults []MonitorResult
	var err error

	tryBatch := enableBatchQuery && (period == "7d" || period == "30d") && len(layerTasks) <= batchQueryMaxKeys
	if tryBatch {
		layerResults, err = h.getStatusBatch(ctx, layerTasks, since, endTime, period, degradedWeight, timeFilter, enableBadges, enableDBTimelineAgg)
		if err != nil {
			logger.Warn("api", "groups 批量查询失败，回退到并发/串行模式", "error", err, "monitors", len(layerTasks), "period", period)
		}
	}

	if err != nil || !tryBatch {
		if enableConcurrent {
			layerResults, err = h.getStatusConcurrent(ctx, layerTasks, since, endTime, period, degradedWeight, timeFilter, concurrentLimit, enableBadges)
		} else {
			layerResults, err = h.getStatusSerial(ctx, layerTasks, since, endTime, period, degradedWeight, timeFilter, enableBadges)
		}
	}
	if err != nil {
		return nil, err
	}

	// 构建 layerByKey 索引
	type layerData struct {
		current  StatusPoint
		timeline []storage.TimePoint
	}

	layerByKey := make(map[storage.MonitorKey]layerData, len(layerTasks))
	for i, task := range layerTasks {
		res := layerResults[i]
		layerByKey[storage.MonitorKey{
			Provider: task.Provider,
			Service:  task.Service,
			Channel:  task.Channel,
			Model:    task.Model,
		}] = layerData{
			current:  toStatusPoint(res.Current),
			timeline: res.Timeline,
		}
	}

	// 按 PSC 分组
	type groupBucket struct {
		hasParent bool
		parent    config.ServiceConfig
		children  []config.ServiceConfig
	}

	buckets := make(map[string]*groupBucket)
	order := make([]string, 0)

	for _, task := range layerTasks {
		psc := task.Provider + "/" + task.Service + "/" + task.Channel
		b := buckets[psc]
		if b == nil {
			b = &groupBucket{}
			buckets[psc] = b
			order = append(order, psc)
		}

		if strings.TrimSpace(task.Parent) == "" {
			// Parent=="" && Model!="" 视为父层
			if !b.hasParent {
				b.hasParent = true
				b.parent = task
			}
			continue
		}
		// Parent!="" 视为子层（按配置顺序追加）
		b.children = append(b.children, task)
	}

	// 构建 groups
	groups := make([]MonitorGroup, 0, len(order))
	for _, psc := range order {
		b := buckets[psc]
		if b == nil || !b.hasParent {
			// 无父层时无法构建组元数据（理论上 Validate 已保证父存在）
			continue
		}

		group := buildMonitorGroupFromParent(b.parent, enableBadges)

		layers := make([]MonitorLayer, 0, 1+len(b.children))

		// 父层（LayerOrder=0）
		parentKey := storage.MonitorKey{
			Provider: b.parent.Provider,
			Service:  b.parent.Service,
			Channel:  b.parent.Channel,
			Model:    b.parent.Model,
		}
		parentData, ok := layerByKey[parentKey]
		if !ok {
			parentData = layerData{
				current:  StatusPoint{Status: -1},
				timeline: h.buildTimeline(nil, endTime, period, degradedWeight, timeFilter),
			}
		}
		layers = append(layers, MonitorLayer{
			Model:         b.parent.Model,
			LayerOrder:    0,
			CurrentStatus: parentData.current,
			Timeline:      parentData.timeline,
		})

		// 子层（按配置顺序，LayerOrder=1..）
		for i, child := range b.children {
			key := storage.MonitorKey{
				Provider: child.Provider,
				Service:  child.Service,
				Channel:  child.Channel,
				Model:    child.Model,
			}
			d, ok := layerByKey[key]
			if !ok {
				d = layerData{
					current:  StatusPoint{Status: -1},
					timeline: h.buildTimeline(nil, endTime, period, degradedWeight, timeFilter),
				}
			}
			layers = append(layers, MonitorLayer{
				Model:         child.Model,
				LayerOrder:    i + 1,
				CurrentStatus: d.current,
				Timeline:      d.timeline,
			})
		}

		// 组级最差状态：0 > 2 > 1 > -1
		groupStatus := -1
		for _, layer := range layers {
			groupStatus = pickWorstStatus(groupStatus, layer.CurrentStatus.Status)
		}

		group.CurrentStatus = groupStatus
		group.Layers = layers
		groups = append(groups, group)
	}

	return groups, nil
}
