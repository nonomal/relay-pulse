package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"

	"monitor/internal/config"
	"monitor/internal/logger"
	"monitor/internal/storage"
)

// TimeFilter 每日时段过滤器（UTC 时区）
// 用于过滤特定时间段内的探测记录，如工作时间 09:00-17:00
// 支持跨午夜的时间范围，如 22:00-04:00（表示 22:00 到次日 04:00）
type TimeFilter struct {
	StartHour     int  // 开始小时 (0-23)
	StartMinute   int  // 开始分钟 (0 或 30)
	EndHour       int  // 结束小时 (0-24，24:00 表示午夜)
	EndMinute     int  // 结束分钟 (0 或 30)
	CrossMidnight bool // 是否跨午夜（start > end）
}

// timeFilterRegex 时段格式正则：HH:MM-HH:MM
var timeFilterRegex = regexp.MustCompile(`^(\d{2}):(\d{2})-(\d{2}):(\d{2})$`)

// Contains 检查给定 UTC 时间是否在时段范围内（左闭右开区间）
// 支持跨午夜的时间范围，如 22:00-04:00
func (f *TimeFilter) Contains(t time.Time) bool {
	h, m, _ := t.UTC().Clock()
	startMinutes := f.StartHour*60 + f.StartMinute
	endMinutes := f.EndHour*60 + f.EndMinute
	currentMinutes := h*60 + m

	if f.CrossMidnight {
		// 跨午夜：22:00-04:00 表示 [22:00, 24:00) ∪ [00:00, 04:00)
		return currentMinutes >= startMinutes || currentMinutes < endMinutes
	}
	// 正常范围：09:00-17:00 表示 [09:00, 17:00)
	return currentMinutes >= startMinutes && currentMinutes < endMinutes
}

// String 返回时段的字符串表示
func (f *TimeFilter) String() string {
	return fmt.Sprintf("%02d:%02d-%02d:%02d", f.StartHour, f.StartMinute, f.EndHour, f.EndMinute)
}

// ParseTimeFilter 解析时段参数
// 返回 nil 表示无过滤（全天）
// 格式：HH:MM-HH:MM，分钟必须为 00 或 30，支持 24:00 表示午夜
// 支持跨午夜的时间范围，如 22:00-04:00（表示 22:00 到次日 04:00）
func ParseTimeFilter(param string) (*TimeFilter, error) {
	if param == "" {
		return nil, nil
	}

	// 正则校验格式
	matches := timeFilterRegex.FindStringSubmatch(param)
	if len(matches) != 5 {
		return nil, fmt.Errorf("无效的时段格式: %s（应为 HH:MM-HH:MM）", param)
	}

	startH, _ := strconv.Atoi(matches[1])
	startM, _ := strconv.Atoi(matches[2])
	endH, _ := strconv.Atoi(matches[3])
	endM, _ := strconv.Atoi(matches[4])

	// 粒度校验：分钟必须为 00 或 30
	if (startM != 0 && startM != 30) || (endM != 0 && endM != 30) {
		return nil, fmt.Errorf("分钟必须为 00 或 30: %s", param)
	}

	// 范围校验：开始 0-23，结束 0-24
	if startH < 0 || startH > 23 {
		return nil, fmt.Errorf("开始小时必须在 0-23 范围内: %s", param)
	}
	if endH < 0 || endH > 24 {
		return nil, fmt.Errorf("结束小时必须在 0-24 范围内: %s", param)
	}
	// 24:00 只允许 24:00，不允许 24:30
	if endH == 24 && endM != 0 {
		return nil, fmt.Errorf("24 点只允许 24:00: %s", param)
	}

	// 判断是否跨午夜
	startTotal := startH*60 + startM
	endTotal := endH*60 + endM
	crossMidnight := startTotal >= endTotal

	// 开始和结束相同时无效（无时段）
	if startTotal == endTotal {
		return nil, fmt.Errorf("开始时间不能等于结束时间: %s", param)
	}

	return &TimeFilter{
		StartHour:     startH,
		StartMinute:   startM,
		EndHour:       endH,
		EndMinute:     endM,
		CrossMidnight: crossMidnight,
	}, nil
}

// statusCache API 响应缓存，防止高频查询打爆数据库
type statusCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	ttl     time.Duration
	maxSize int                // 最大缓存条目数，防止内存泄漏
	sf      singleflight.Group // 防止缓存击穿
}

type cacheEntry struct {
	data     []byte
	expireAt time.Time
}

func newStatusCache(ttl time.Duration, maxSize int) *statusCache {
	return &statusCache{
		entries: make(map[string]*cacheEntry),
		ttl:     ttl,
		maxSize: maxSize,
	}
}

// get 获取缓存，过期则删除并返回 miss
func (c *statusCache) get(key string) ([]byte, bool) {
	now := time.Now()
	c.mu.RLock()
	entry := c.entries[key]
	c.mu.RUnlock()

	if entry == nil {
		return nil, false
	}

	if now.After(entry.expireAt) {
		// 懒清理：删除过期 key
		c.mu.Lock()
		if cur := c.entries[key]; cur == entry {
			delete(c.entries, key)
		}
		c.mu.Unlock()
		return nil, false
	}

	return entry.data, true
}

// set 存入缓存（拷贝数据，防止 buffer 复用问题）
func (c *statusCache) set(key string, data []byte) {
	buf := make([]byte, len(data))
	copy(buf, data)

	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()

	// 容量限制：超出时清理过期条目
	if len(c.entries) >= c.maxSize {
		for k, v := range c.entries {
			if now.After(v.expireAt) {
				delete(c.entries, k)
			}
		}
	}

	// 仍然超出则跳过写入（防止 DoS）
	if len(c.entries) >= c.maxSize {
		return
	}

	c.entries[key] = &cacheEntry{
		data:     buf,
		expireAt: now.Add(c.ttl),
	}
}

// clear 清空所有缓存（配置热更新时调用）
func (c *statusCache) clear() {
	c.mu.Lock()
	c.entries = make(map[string]*cacheEntry)
	c.mu.Unlock()
}

// load 获取缓存，未命中时用 singleflight 合并并发请求
func (c *statusCache) load(key string, loader func() ([]byte, error)) ([]byte, error) {
	// 先检查缓存
	if data, ok := c.get(key); ok {
		return data, nil
	}

	// singleflight: 同 key 多请求只执行一次 loader
	v, err, _ := c.sf.Do(key, func() (interface{}, error) {
		// double check：可能在等待期间已被其他 goroutine 填充
		if data, ok := c.get(key); ok {
			return data, nil
		}

		fresh, err := loader()
		if err != nil {
			return nil, err // 错误不缓存
		}

		c.set(key, fresh)
		return fresh, nil
	})

	if err != nil {
		return nil, err
	}
	return v.([]byte), nil
}

// Handler API处理器
type Handler struct {
	storage storage.Storage
	config  *config.AppConfig
	cfgMu   sync.RWMutex // 保护config的并发访问
	cache   *statusCache // API 响应缓存
}

// NewHandler 创建处理器
func NewHandler(store storage.Storage, cfg *config.AppConfig) *Handler {
	return &Handler{
		storage: store,
		config:  cfg,
		cache:   newStatusCache(10*time.Second, 100), // 10 秒缓存，最多 100 条
	}
}

// CurrentStatus API返回的当前状态（不暴露数据库主键）
type CurrentStatus struct {
	Status    int   `json:"status"`
	Latency   int   `json:"latency"`
	Timestamp int64 `json:"timestamp"`
}

// MonitorResult API返回结构
type MonitorResult struct {
	Provider     string              `json:"provider"`
	ProviderSlug string              `json:"provider_slug"` // URL slug（用于生成专属页面链接）
	ProviderURL  string              `json:"provider_url"`  // 服务商官网链接
	Service      string              `json:"service"`
	Category     string              `json:"category"`                // 分类：commercial（商业站）或 public（公益站）
	Sponsor      string              `json:"sponsor"`                 // 赞助者
	SponsorURL   string              `json:"sponsor_url"`             // 赞助者链接
	SponsorLevel config.SponsorLevel `json:"sponsor_level,omitempty"` // 赞助商等级：basic/advanced/enterprise
	Risks        []config.RiskBadge  `json:"risks,omitempty"`         // 风险徽标数组
	PriceMin     *float64            `json:"price_min,omitempty"`     // 参考倍率下限
	PriceMax     *float64            `json:"price_max,omitempty"`     // 参考倍率
	ListedDays   *int                `json:"listed_days,omitempty"`   // 收录天数（从 listed_since 计算）
	Channel      string              `json:"channel"`                 // 业务通道标识
	Current      *CurrentStatus      `json:"current_status"`
	Timeline     []storage.TimePoint `json:"timeline"`
}

// GetStatus 获取监测状态
func (h *Handler) GetStatus(c *gin.Context) {
	// 参数解析
	period := c.DefaultQuery("period", "24h")
	align := c.DefaultQuery("align", "")                 // 时间对齐模式：空=动态滑动窗口, "hour"=整点对齐
	timeFilterParam := c.DefaultQuery("time_filter", "") // 每日时段过滤：HH:MM-HH:MM（UTC）
	qProvider := strings.ToLower(strings.TrimSpace(c.DefaultQuery("provider", "all")))
	qService := c.DefaultQuery("service", "all")
	// include_hidden 参数：用于内部调试，默认不包含隐藏的监测项
	includeHidden := strings.EqualFold(strings.TrimSpace(c.DefaultQuery("include_hidden", "false")), "true")

	// 验证 period 参数
	if _, err := h.parsePeriod(period); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("无效的时间范围: %s", period),
		})
		return
	}

	// 验证 align 参数
	if align != "" && align != "hour" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("无效的对齐模式: %s (支持: hour)", align),
		})
		return
	}

	// 验证 time_filter 参数
	var timeFilter *TimeFilter
	if timeFilterParam != "" {
		// 时段过滤仅支持 7d 和 30d 周期
		if period == "24h" || period == "1d" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "时段过滤仅支持 7d 和 30d 周期",
			})
			return
		}

		var err error
		timeFilter, err = ParseTimeFilter(timeFilterParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
	}

	// 构建缓存 key（使用明确的分隔符避免碰撞）
	cacheKey := fmt.Sprintf("p=%s|align=%s|tf=%s|prov=%s|svc=%s|hidden=%t", period, align, timeFilterParam, qProvider, qService, includeHidden)

	// 使用缓存（singleflight 防止缓存击穿）
	// 注意：使用独立 context，避免单个请求取消影响其他等待的请求
	data, err := h.cache.load(cacheKey, func() ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return h.queryAndSerialize(ctx, period, align, timeFilter, qProvider, qService, includeHidden)
	})

	if err != nil {
		logger.FromContext(c.Request.Context(), "api").Error("GetStatus 失败", "cache_key", cacheKey, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("查询失败: %v", err),
		})
		return
	}

	// CDN 缓存头：Cloudflare 遵守 s-maxage，浏览器遵守 max-age
	c.Header("Cache-Control", "public, max-age=10, s-maxage=10")
	c.Header("Content-Type", "application/json; charset=utf-8")
	c.Writer.Write(data)
}

// queryAndSerialize 查询数据库并序列化为 JSON（缓存 miss 时调用）
func (h *Handler) queryAndSerialize(ctx context.Context, period, align string, timeFilter *TimeFilter, qProvider, qService string, includeHidden bool) ([]byte, error) {
	// 解析时间范围（支持对齐模式）
	startTime, endTime := h.parseTimeRange(period, align)

	// 获取配置副本（线程安全）
	h.cfgMu.RLock()
	monitors := h.config.Monitors
	degradedWeight := h.config.DegradedWeight
	enableConcurrent := h.config.EnableConcurrentQuery
	concurrentLimit := h.config.ConcurrentQueryLimit
	slowLatencyMs := int(h.config.SlowLatencyDuration / time.Millisecond)
	sponsorPin := h.config.SponsorPin
	h.cfgMu.RUnlock()

	// 构建 slug -> provider 映射（slug作为provider的路由别名）
	slugToProvider := make(map[string]string)
	for _, task := range monitors {
		normalizedProvider := strings.ToLower(strings.TrimSpace(task.Provider))
		slugToProvider[task.ProviderSlug] = normalizedProvider
	}

	// 将查询参数（可能是slug或provider）映射回真实的provider
	realProvider := qProvider
	if mappedProvider, exists := slugToProvider[qProvider]; exists {
		realProvider = mappedProvider
	}

	// 过滤并去重监测项
	filtered := h.filterMonitors(monitors, realProvider, qService, includeHidden)

	// 根据配置选择串行或并发查询
	var response []MonitorResult
	var err error
	var mode string

	if enableConcurrent {
		mode = "concurrent"
		response, err = h.getStatusConcurrent(ctx, filtered, startTime, endTime, period, degradedWeight, timeFilter, concurrentLimit)
	} else {
		mode = "serial"
		response, err = h.getStatusSerial(ctx, filtered, startTime, endTime, period, degradedWeight, timeFilter)
	}

	if err != nil {
		return nil, err
	}

	logger.Info("api", "GetStatus 查询完成", "mode", mode, "monitors", len(filtered), "period", period, "align", align, "count", len(response))

	// 序列化为 JSON
	meta := gin.H{
		"period":          period,
		"count":           len(response),
		"slow_latency_ms": slowLatencyMs,
		"sponsor_pin": gin.H{
			"enabled":    sponsorPin.IsEnabled(),
			"max_pinned": sponsorPin.MaxPinned,
			"min_uptime": sponsorPin.MinUptime,
			"min_level":  sponsorPin.MinLevel,
		},
	}
	// 仅在使用对齐模式时返回额外的时间范围信息
	if align != "" {
		meta["align"] = align
		meta["start_time"] = startTime.UTC().Format(time.RFC3339)
		meta["end_time"] = endTime.UTC().Format(time.RFC3339)
	}
	// 返回时段过滤信息
	if timeFilter != nil {
		meta["time_filter"] = timeFilter.String()
		meta["timezone"] = "UTC"
	}

	result := gin.H{
		"meta": meta,
		"data": response,
	}

	return json.Marshal(result)
}

// filterMonitors 过滤并去重监测项
func (h *Handler) filterMonitors(monitors []config.ServiceConfig, provider, service string, includeHidden bool) []config.ServiceConfig {
	var filtered []config.ServiceConfig
	seen := make(map[string]bool)

	for _, task := range monitors {
		// 始终过滤已禁用的监测项（不探测、不存储、不展示）
		if task.Disabled {
			continue
		}

		// 过滤隐藏的监测项（除非显式要求包含）
		if !includeHidden && task.Hidden {
			continue
		}

		normalizedTaskProvider := strings.ToLower(strings.TrimSpace(task.Provider))

		// 过滤（统一使用 provider 名称匹配）
		if provider != "all" && provider != normalizedTaskProvider {
			continue
		}
		if service != "all" && service != task.Service {
			continue
		}

		// 去重（使用 provider + service + channel 组合）
		key := task.Provider + "/" + task.Service + "/" + task.Channel
		if seen[key] {
			continue
		}
		seen[key] = true

		filtered = append(filtered, task)
	}

	return filtered
}

// getStatusSerial 串行查询（原有逻辑）
func (h *Handler) getStatusSerial(ctx context.Context, monitors []config.ServiceConfig, since, endTime time.Time, period string, degradedWeight float64, timeFilter *TimeFilter) ([]MonitorResult, error) {
	var response []MonitorResult
	store := h.storage.WithContext(ctx)

	for _, task := range monitors {
		// 获取最新记录
		latest, err := store.GetLatest(task.Provider, task.Service, task.Channel)
		if err != nil {
			return nil, fmt.Errorf("查询失败 %s/%s/%s: %w", task.Provider, task.Service, task.Channel, err)
		}

		// 获取历史记录
		history, err := store.GetHistory(task.Provider, task.Service, task.Channel, since)
		if err != nil {
			return nil, fmt.Errorf("查询历史失败 %s/%s/%s: %w", task.Provider, task.Service, task.Channel, err)
		}

		// 构建响应
		result := h.buildMonitorResult(task, latest, history, endTime, period, degradedWeight, timeFilter)
		response = append(response, result)
	}

	return response, nil
}

// getStatusConcurrent 并发查询（使用 errgroup + 并发限制）
func (h *Handler) getStatusConcurrent(ctx context.Context, monitors []config.ServiceConfig, since, endTime time.Time, period string, degradedWeight float64, timeFilter *TimeFilter, limit int) ([]MonitorResult, error) {
	// 使用请求的 context（支持取消）
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(limit) // 限制最大并发度
	store := h.storage.WithContext(gctx)

	// 预分配结果数组（保持顺序）
	results := make([]MonitorResult, len(monitors))

	for i, task := range monitors {
		i, task := i, task // 捕获循环变量
		g.Go(func() error {
			// 获取最新记录
			latest, err := store.GetLatest(task.Provider, task.Service, task.Channel)
			if err != nil {
				return fmt.Errorf("GetLatest %s/%s/%s: %w", task.Provider, task.Service, task.Channel, err)
			}

			// 获取历史记录
			history, err := store.GetHistory(task.Provider, task.Service, task.Channel, since)
			if err != nil {
				return fmt.Errorf("GetHistory %s/%s/%s: %w", task.Provider, task.Service, task.Channel, err)
			}

			// 构建响应（固定位置写入，保持顺序）
			results[i] = h.buildMonitorResult(task, latest, history, endTime, period, degradedWeight, timeFilter)
			return nil
		})
	}

	// 等待所有 goroutine 完成
	if err := g.Wait(); err != nil {
		return nil, err
	}

	return results, nil
}

// buildMonitorResult 构建单个监测项的响应结构
func (h *Handler) buildMonitorResult(task config.ServiceConfig, latest *storage.ProbeRecord, history []*storage.ProbeRecord, endTime time.Time, period string, degradedWeight float64, timeFilter *TimeFilter) MonitorResult {
	// 转换为时间轴数据
	timeline := h.buildTimeline(history, endTime, period, degradedWeight, timeFilter)

	// 转换为API响应格式（不暴露数据库主键）
	var current *CurrentStatus
	if latest != nil {
		current = &CurrentStatus{
			Status:    latest.Status,
			Latency:   latest.Latency,
			Timestamp: latest.Timestamp,
		}
	}

	// 生成 slug：优先使用配置的 provider_slug，回退到 provider 小写
	slug := task.ProviderSlug
	if slug == "" {
		slug = strings.ToLower(strings.TrimSpace(task.Provider))
	}

	// 计算收录天数（从 listed_since 到今天）
	var listedDays *int
	if task.ListedSince != "" {
		if listedDate, err := time.Parse("2006-01-02", task.ListedSince); err == nil {
			days := int(time.Since(listedDate).Hours() / 24)
			if days < 0 {
				days = 0 // 防止未来日期导致负数
			}
			listedDays = &days
		}
	}

	return MonitorResult{
		Provider:     task.Provider,
		ProviderSlug: slug,
		ProviderURL:  task.ProviderURL,
		Service:      task.Service,
		Category:     task.Category,
		Sponsor:      task.Sponsor,
		SponsorURL:   task.SponsorURL,
		SponsorLevel: task.SponsorLevel,
		Risks:        task.Risks,
		PriceMin:     task.PriceMin,
		PriceMax:     task.PriceMax,
		ListedDays:   listedDays,
		Channel:      task.Channel,
		Current:      current,
		Timeline:     timeline,
	}
}

// parsePeriod 解析时间范围（仅用于验证）
func (h *Handler) parsePeriod(period string) (time.Duration, error) {
	switch period {
	case "24h", "1d":
		return 24 * time.Hour, nil
	case "7d":
		return 7 * 24 * time.Hour, nil
	case "30d":
		return 30 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("不支持的时间范围")
	}
}

// parseTimeRange 解析时间范围，返回 (startTime, endTime)
// align 参数控制时间对齐模式：空=动态滑动窗口, "hour"=整点对齐
// 注意：7d/30d 模式自动使用 day 对齐，忽略 align 参数
func (h *Handler) parseTimeRange(period, align string) (startTime, endTime time.Time) {
	now := time.Now()

	// 根据 period 计算时间范围
	// 24h: 用户可选 align 模式
	// 7d/30d: 强制使用 day 对齐（包含今天不完整数据）
	switch period {
	case "24h", "1d":
		endTime = h.alignTimestamp(now, align)
		startTime = endTime.Add(-24 * time.Hour)
	case "7d":
		endTime = h.alignTimestamp(now, "day") // 自动按天对齐
		startTime = endTime.AddDate(0, 0, -7)
	case "30d":
		endTime = h.alignTimestamp(now, "day") // 自动按天对齐
		startTime = endTime.AddDate(0, 0, -30)
	default:
		endTime = h.alignTimestamp(now, align)
		startTime = endTime.Add(-24 * time.Hour)
	}

	return startTime, endTime
}

// alignTimestamp 根据对齐模式调整时间戳
// - align="hour": 向上取整到下一个 UTC 整点
// - align="day": 向上取整到下一天 00:00 UTC
// - 其他值: 保持原值（动态滑动窗口）
func (h *Handler) alignTimestamp(t time.Time, align string) time.Time {
	switch align {
	case "hour":
		// 向上取整到下一个整点（包含当前正在进行的小时）
		// 例如 17:48 → 18:00，这样最后一个 bucket 是 17:00-18:00
		truncated := t.UTC().Truncate(time.Hour)
		if truncated.Before(t.UTC()) {
			return truncated.Add(time.Hour)
		}
		return truncated
	case "day":
		// 向上取整到下一天 00:00 UTC（包含今天不完整的数据）
		// 例如 2024-01-15 12:30 → 2024-01-16 00:00，这样最后一个 bucket 是今天
		truncated := t.UTC().Truncate(24 * time.Hour)
		if truncated.Before(t.UTC()) {
			return truncated.Add(24 * time.Hour)
		}
		return truncated
	default:
		return t
	}
}

// bucketStats 用于聚合每个 bucket 内的探测数据
type bucketStats struct {
	total           int                  // 总探测次数
	weightedSuccess float64              // 累积成功权重（绿=1.0, 黄=degraded_weight, 红=0.0）
	latencySum      int64                // 延迟总和（仅统计可用状态）
	latencyCount    int                  // 有效延迟计数（仅 status > 0 的记录）
	last            *storage.ProbeRecord // 最新一条记录
	statusCounts    storage.StatusCounts // 各状态计数
}

// buildTimeline 构建固定长度的时间轴，计算每个 bucket 的可用率和平均延迟
// endTime 为时间窗口的结束时间（对齐模式下为整点，动态模式下为当前时间）
// timeFilter 为每日时段过滤器，nil 表示全天（不过滤）
func (h *Handler) buildTimeline(records []*storage.ProbeRecord, endTime time.Time, period string, degradedWeight float64, timeFilter *TimeFilter) []storage.TimePoint {
	// 根据 period 确定 bucket 策略
	bucketCount, bucketWindow, format := h.determineBucketStrategy(period)

	// 使用传入的 endTime 作为基准时间（支持对齐模式）
	baseTime := endTime

	// 初始化 buckets 和统计数据
	buckets := make([]storage.TimePoint, bucketCount)
	stats := make([]bucketStats, bucketCount)

	for i := 0; i < bucketCount; i++ {
		bucketTime := baseTime.Add(-time.Duration(bucketCount-i) * bucketWindow)
		buckets[i] = storage.TimePoint{
			Time:         bucketTime.Format(format),
			Timestamp:    bucketTime.Unix(),
			Status:       -1, // 缺失标记
			Latency:      0,
			Availability: -1, // 缺失标记
		}
	}

	// 聚合每个 bucket 的探测结果
	for _, record := range records {
		t := time.Unix(record.Timestamp, 0)

		// 时段过滤：跳过不在指定时间段内的记录
		if timeFilter != nil && !timeFilter.Contains(t) {
			continue
		}

		timeDiff := baseTime.Sub(t)

		// 跳过超出时间窗口的记录：
		// - timeDiff < 0: 记录在 endTime 之后（对齐模式下当前小时的数据）
		// - timeDiff >= bucketCount * bucketWindow: 记录太旧
		if timeDiff < 0 {
			continue // 记录在窗口之后，跳过
		}

		// 计算该记录属于哪个 bucket（从后往前）
		bucketIndex := int(timeDiff / bucketWindow)
		if bucketIndex >= bucketCount {
			continue // 超出范围，忽略
		}

		// 从前往后的索引
		actualIndex := bucketCount - 1 - bucketIndex
		if actualIndex < 0 || actualIndex >= bucketCount {
			continue
		}

		// 聚合统计
		stat := &stats[actualIndex]
		stat.total++
		stat.weightedSuccess += availabilityWeight(record.Status, degradedWeight)
		// 只统计可用状态（status > 0）的延迟
		if record.Status > 0 {
			stat.latencySum += int64(record.Latency)
			stat.latencyCount++
		}
		incrementStatusCount(&stat.statusCounts, record.Status, record.SubStatus)

		// 保留最新记录
		if stat.last == nil || record.Timestamp > stat.last.Timestamp {
			stat.last = record
		}
	}

	// 根据聚合结果计算可用率和平均延迟
	for i := 0; i < bucketCount; i++ {
		stat := &stats[i]
		buckets[i].StatusCounts = stat.statusCounts
		if stat.total == 0 {
			continue
		}

		// 计算可用率（使用权重）
		buckets[i].Availability = (stat.weightedSuccess / float64(stat.total)) * 100

		// 计算平均延迟（仅统计可用状态，四舍五入）
		if stat.latencyCount > 0 {
			avgLatency := float64(stat.latencySum) / float64(stat.latencyCount)
			buckets[i].Latency = int(avgLatency + 0.5)
		}

		// 使用最新记录的状态
		if stat.last != nil {
			buckets[i].Status = stat.last.Status
			// 注意：Timestamp 保持为 bucket 起始时间，不覆盖
			// 这样前端可以准确显示时间段（如 03:00-04:00）
		}
	}

	return buckets
}

// determineBucketStrategy 根据 period 确定 bucket 数量、窗口大小和时间格式
func (h *Handler) determineBucketStrategy(period string) (count int, window time.Duration, format string) {
	switch period {
	case "24h", "1d":
		return 24, time.Hour, "15:04"
	case "7d":
		return 7, 24 * time.Hour, "2006-01-02"
	case "30d":
		return 30, 24 * time.Hour, "2006-01-02"
	default:
		return 24, time.Hour, "15:04"
	}
}

// UpdateConfig 更新配置（热更新时调用）
func (h *Handler) UpdateConfig(cfg *config.AppConfig) {
	h.cfgMu.Lock()
	h.config = cfg
	h.cfgMu.Unlock()

	// 配置更新后清空缓存，确保禁用/隐藏状态变更立即生效
	h.cache.clear()
}

// availabilityWeight 根据状态码返回可用率权重
func availabilityWeight(status int, degradedWeight float64) float64 {
	switch status {
	case 1: // 绿色（正常）
		return 1.0
	case 2: // 黄色（降级：如慢响应等）
		return degradedWeight
	default: // 红色（不可用）或灰色（未配置）
		return 0.0
	}
}

// incrementStatusCount 统计每种状态及细分出现次数
func incrementStatusCount(counts *storage.StatusCounts, status int, subStatus storage.SubStatus) {
	switch status {
	case 1: // 绿色
		counts.Available++
	case 2: // 黄色
		counts.Degraded++
		// 黄色细分
		switch subStatus {
		case storage.SubStatusSlowLatency:
			counts.SlowLatency++
		case storage.SubStatusRateLimit:
			counts.RateLimit++
		}
	case 0: // 红色
		counts.Unavailable++
		// 红色细分
		switch subStatus {
		case storage.SubStatusRateLimit:
			// 限流现在视为红色不可用，但沿用 rate_limit 细分计数
			counts.RateLimit++
		case storage.SubStatusServerError:
			counts.ServerError++
		case storage.SubStatusClientError:
			counts.ClientError++
		case storage.SubStatusAuthError:
			counts.AuthError++
		case storage.SubStatusInvalidRequest:
			counts.InvalidRequest++
		case storage.SubStatusNetworkError:
			counts.NetworkError++
		case storage.SubStatusContentMismatch:
			counts.ContentMismatch++
		}
	default: // 灰色（3）或其他
		counts.Missing++
	}
}

// GetSitemap 生成 sitemap.xml
func (h *Handler) GetSitemap(c *gin.Context) {
	// 获取配置副本
	h.cfgMu.RLock()
	monitors := h.config.Monitors
	h.cfgMu.RUnlock()

	// 提取唯一的 provider slugs
	providerSlugs := h.extractUniqueProviderSlugs(monitors)

	// 构建 sitemap XML
	sitemap := h.buildSitemapXML(providerSlugs)

	c.Header("Content-Type", "application/xml; charset=utf-8")
	c.Header("Cache-Control", "public, max-age=3600") // 缓存 1 小时
	c.String(http.StatusOK, sitemap)
}

// extractUniqueProviderSlugs 从监测配置中提取唯一的 provider slugs（排除禁用和隐藏的）
func (h *Handler) extractUniqueProviderSlugs(monitors []config.ServiceConfig) []string {
	slugSet := make(map[string]bool)
	var slugs []string

	for _, task := range monitors {
		// 跳过已禁用的监测项
		if task.Disabled {
			continue
		}
		// 跳过隐藏的监测项
		if task.Hidden {
			continue
		}

		slug := task.ProviderSlug
		if slug == "" {
			slug = strings.ToLower(strings.TrimSpace(task.Provider))
		}

		if !slugSet[slug] {
			slugSet[slug] = true
			slugs = append(slugs, slug)
		}
	}

	return slugs
}

// buildSitemapXML 构建 sitemap.xml 内容
func (h *Handler) buildSitemapXML(providerSlugs []string) string {
	h.cfgMu.RLock()
	baseURL := h.config.PublicBaseURL
	h.cfgMu.RUnlock()
	languages := []struct {
		code string // hreflang 语言码
		path string // URL 路径前缀
	}{
		{"zh-Hans", ""}, // 中文默认无前缀
		{"en", "en"},    // 英文
		{"ru", "ru"},    // 俄文
		{"ja", "ja"},    // 日文
	}

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString("\n")
	sb.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"`)
	sb.WriteString("\n")
	sb.WriteString(`        xmlns:xhtml="http://www.w3.org/1999/xhtml">`)
	sb.WriteString("\n")

	// 生成首页 URL（4 个语言版本）
	for _, lang := range languages {
		sb.WriteString("  <url>\n")

		// 生成 loc
		if lang.path == "" {
			sb.WriteString(fmt.Sprintf("    <loc>%s/</loc>\n", baseURL))
		} else {
			sb.WriteString(fmt.Sprintf("    <loc>%s/%s/</loc>\n", baseURL, lang.path))
		}

		// 生成 hreflang 链接（指向所有语言版本）
		for _, altLang := range languages {
			var href string
			if altLang.path == "" {
				href = fmt.Sprintf("%s/", baseURL)
			} else {
				href = fmt.Sprintf("%s/%s/", baseURL, altLang.path)
			}
			sb.WriteString(fmt.Sprintf(`    <xhtml:link rel="alternate" hreflang="%s" href="%s"/>`+"\n", altLang.code, href))
		}

		// x-default 指向中文首页
		sb.WriteString(fmt.Sprintf(`    <xhtml:link rel="alternate" hreflang="x-default" href="%s/"/>`+"\n", baseURL))

		sb.WriteString("    <priority>1.0</priority>\n")
		sb.WriteString("    <changefreq>daily</changefreq>\n")
		sb.WriteString("  </url>\n")
	}

	// 生成服务商页面 URL（每个 provider 4 个语言版本）
	for _, slug := range providerSlugs {
		for _, lang := range languages {
			sb.WriteString("  <url>\n")

			// 生成 loc
			if lang.path == "" {
				sb.WriteString(fmt.Sprintf("    <loc>%s/p/%s</loc>\n", baseURL, slug))
			} else {
				sb.WriteString(fmt.Sprintf("    <loc>%s/%s/p/%s</loc>\n", baseURL, lang.path, slug))
			}

			// 生成 hreflang 链接（指向所有语言版本）
			for _, altLang := range languages {
				var href string
				if altLang.path == "" {
					href = fmt.Sprintf("%s/p/%s", baseURL, slug)
				} else {
					href = fmt.Sprintf("%s/%s/p/%s", baseURL, altLang.path, slug)
				}
				sb.WriteString(fmt.Sprintf(`    <xhtml:link rel="alternate" hreflang="%s" href="%s"/>`+"\n", altLang.code, href))
			}

			// x-default 指向中文版本
			sb.WriteString(fmt.Sprintf(`    <xhtml:link rel="alternate" hreflang="x-default" href="%s/p/%s"/>`+"\n", baseURL, slug))

			sb.WriteString("    <priority>0.8</priority>\n")
			sb.WriteString("    <changefreq>daily</changefreq>\n")
			sb.WriteString("  </url>\n")
		}
	}

	sb.WriteString("</urlset>\n")
	return sb.String()
}

// GetRobots 生成 robots.txt
func (h *Handler) GetRobots(c *gin.Context) {
	h.cfgMu.RLock()
	baseURL := h.config.PublicBaseURL
	h.cfgMu.RUnlock()

	robotsTxt := fmt.Sprintf(`User-agent: *
Allow: /
Disallow: /api/

Sitemap: %s/sitemap.xml
`, baseURL)

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Cache-Control", "public, max-age=86400") // 缓存 24 小时
	c.String(http.StatusOK, robotsTxt)
}
