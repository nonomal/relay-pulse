// Package validator 提供订阅目标验证功能
// 通过调用 relay-pulse 主服务的 /api/status/query 接口验证 provider/service/channel 是否存在
package validator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// 默认配置
const (
	defaultPositiveTTL = 2 * time.Minute  // 正向缓存 TTL
	defaultNegativeTTL = 30 * time.Second // 负向缓存 TTL
	defaultHTTPTimeout = 5 * time.Second  // HTTP 请求超时
	maxCandidates      = 8                // 候选列表最大数量
	maxResponseSize    = 1 << 20          // 响应体最大 1MB
	maxCacheEntries    = 500              // 缓存最大条目数（防止内存膨胀）
)

// NotFoundLevel 表示未找到的层级
type NotFoundLevel string

const (
	NotFoundProvider NotFoundLevel = "provider"
	NotFoundService  NotFoundLevel = "service"
	NotFoundChannel  NotFoundLevel = "channel"
)

// NotFoundError 表示目标不存在
type NotFoundError struct {
	Level      NotFoundLevel // 不存在的层级
	Provider   string        // 查询的 provider
	Service    string        // 查询的 service
	Channel    string        // 查询的 channel
	Candidates []string      // 候选列表（service 或 channel）
}

func (e *NotFoundError) Error() string {
	switch e.Level {
	case NotFoundProvider:
		return "provider 不存在"
	case NotFoundService:
		return "service 不存在"
	case NotFoundChannel:
		return "channel 不存在"
	default:
		return "目标不存在"
	}
}

// UnavailableError 表示验证服务不可用
type UnavailableError struct {
	Cause error
}

func (e *UnavailableError) Error() string {
	return fmt.Sprintf("验证服务不可用: %v", e.Cause)
}

func (e *UnavailableError) Unwrap() error {
	return e.Cause
}

// ColdBoardError 表示目标为冷板（board=cold），不允许新增订阅
type ColdBoardError struct {
	Provider string
	Service  string
	Channel  string
}

func (e *ColdBoardError) Error() string {
	if e.Channel != "" {
		return fmt.Sprintf("%s/%s/%s 已被移入冷板，不支持订阅", e.Provider, e.Service, e.Channel)
	}
	if e.Service != "" {
		return fmt.Sprintf("%s/%s 当前无可订阅的热板监测项", e.Provider, e.Service)
	}
	if e.Provider != "" {
		return fmt.Sprintf("%s 当前无可订阅的热板监测项", e.Provider)
	}
	return "目标为冷板，禁止订阅"
}

// CanonicalTarget 表示规范化的订阅目标
type CanonicalTarget struct {
	Provider string // 规范化的 provider 名称
	Service  string // 规范化的 service 名称
	Channel  string // 规范化的 channel 名称（空表示订阅所有 channel）
	Board    string // hot/cold（用于拒绝冷板订阅）
}

// Validator 订阅目标验证器接口
type Validator interface {
	// ValidateAdd 验证订阅目标是否存在
	// 返回规范化的目标或错误（NotFoundError/UnavailableError）
	ValidateAdd(ctx context.Context, provider, service, channel string) (*CanonicalTarget, error)
}

// RelayPulseValidator 基于 relay-pulse API 的验证器实现
type RelayPulseValidator struct {
	statusQueryURL string
	httpClient     *http.Client

	positiveTTL time.Duration
	negativeTTL time.Duration

	mu sync.Mutex

	// 缓存：key 使用 lowercase 标识
	providerCache map[string]*providerCacheEntry // "prov:<provider>" -> entry
	serviceCache  map[string]*serviceCacheEntry  // "svc:<provider>/<service>" -> entry

	// singleflight: 防止并发击穿
	inflight map[string]*inflightCall
}

type providerCacheEntry struct {
	expireAt time.Time
	provider string   // canonical provider 名称
	services []string // 可用的 service 列表
	notFound bool     // 是否为负向缓存
}

// channelEntry channel 信息（包含 board 状态）
type channelEntry struct {
	name  string // channel 名称
	board string // hot/cold
}

type serviceCacheEntry struct {
	expireAt time.Time
	provider string         // canonical provider 名称
	service  string         // canonical service 名称
	channels []channelEntry // 可用的 channel 列表（包含 board 信息）
	notFound bool           // 是否为负向缓存
	level    NotFoundLevel
}

type inflightCall struct {
	done chan struct{}
	val  any
	err  error
}

// NewRelayPulseValidator 创建验证器
// eventsURL: relay-pulse 的 events_url 配置（如 http://localhost:8080/api/events）
func NewRelayPulseValidator(eventsURL string) (*RelayPulseValidator, error) {
	statusQueryURL, err := deriveStatusQueryURL(eventsURL)
	if err != nil {
		return nil, err
	}

	return &RelayPulseValidator{
		statusQueryURL: statusQueryURL,
		httpClient: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
		positiveTTL:   defaultPositiveTTL,
		negativeTTL:   defaultNegativeTTL,
		providerCache: make(map[string]*providerCacheEntry),
		serviceCache:  make(map[string]*serviceCacheEntry),
		inflight:      make(map[string]*inflightCall),
	}, nil
}

// ValidateAdd 验证订阅目标是否存在
// 支持两种订阅模式：
// - service 级订阅：service!="" && channel=""，订阅该 service 下所有通道
// - 精确订阅：service!="" && channel!=""，订阅特定通道
// 注：provider 级订阅请使用 ValidateAndExpandProvider
func (v *RelayPulseValidator) ValidateAdd(ctx context.Context, provider, service, channel string) (*CanonicalTarget, error) {
	provider = strings.TrimSpace(provider)
	service = strings.TrimSpace(service)
	channel = strings.TrimSpace(channel)

	// "default" 归一化为空字符串
	if strings.EqualFold(channel, "default") {
		channel = ""
	}

	// provider 和 service 都是必填的
	if provider == "" {
		return nil, &NotFoundError{
			Level:    NotFoundProvider,
			Provider: provider,
			Service:  service,
			Channel:  channel,
		}
	}

	if service == "" {
		return nil, &NotFoundError{
			Level:    NotFoundService,
			Provider: provider,
			Service:  service,
			Channel:  channel,
		}
	}

	// 获取 service 信息（包含 channels 列表）
	svcEntry, err := v.getServiceEntry(ctx, provider, service)
	if err != nil {
		return nil, err
	}

	// 如果指定了 channel，验证其存在性
	if channel != "" {
		channelLower := strings.ToLower(channel)
		var canonicalChannel string
		var canonicalBoard string
		found := false

		for _, ch := range svcEntry.channels {
			if strings.ToLower(ch.name) == channelLower {
				canonicalChannel = ch.name
				canonicalBoard = strings.TrimSpace(ch.board)
				found = true
				break
			}
		}

		if !found {
			// 构建候选列表
			var candidates []string
			for _, ch := range svcEntry.channels {
				candidates = append(candidates, ch.name)
			}
			return nil, &NotFoundError{
				Level:      NotFoundChannel,
				Provider:   provider,
				Service:    service,
				Channel:    channel,
				Candidates: limitSlice(candidates, maxCandidates),
			}
		}

		// 冷板订阅拒绝：仅当 relay-pulse 返回 board=cold 时生效；缺失时按 hot 处理（兼容旧版本）
		if strings.EqualFold(canonicalBoard, "cold") {
			return nil, &ColdBoardError{
				Provider: svcEntry.provider,
				Service:  svcEntry.service,
				Channel:  canonicalChannel,
			}
		}

		return &CanonicalTarget{
			Provider: svcEntry.provider,
			Service:  svcEntry.service,
			Channel:  canonicalChannel,
			Board:    canonicalBoard,
		}, nil
	}

	// channel 为空：订阅该 service 的所有 channel
	return &CanonicalTarget{
		Provider: svcEntry.provider,
		Service:  svcEntry.service,
		Channel:  "",
	}, nil
}

// ValidateAndExpandProvider 验证 provider 并返回所有 service/channel 组合
// 用于 /add <provider> 展开订阅
// 任何 service 获取失败时返回错误（失败即中止）
func (v *RelayPulseValidator) ValidateAndExpandProvider(ctx context.Context, provider string) ([]CanonicalTarget, error) {
	provider = strings.TrimSpace(provider)
	if provider == "" {
		return nil, &NotFoundError{
			Level:    NotFoundProvider,
			Provider: provider,
		}
	}

	provEntry, err := v.getProviderEntry(ctx, provider)
	if err != nil {
		return nil, err
	}

	var targets []CanonicalTarget
	for _, svc := range provEntry.services {
		svcEntry, err := v.getServiceEntry(ctx, provider, svc)
		if err != nil {
			return nil, err // 失败即中止
		}
		if len(svcEntry.channels) == 0 {
			// service 无 channel，添加 service 级订阅
			targets = append(targets, CanonicalTarget{
				Provider: provEntry.provider,
				Service:  svcEntry.service,
				Channel:  "",
			})
		} else {
			// 展开每个 channel，跳过冷板
			for _, ch := range svcEntry.channels {
				// 冷板订阅拒绝：仅当 relay-pulse 返回 board=cold 时生效；缺失时按 hot 处理（兼容旧版本）
				if strings.EqualFold(strings.TrimSpace(ch.board), "cold") {
					continue
				}
				targets = append(targets, CanonicalTarget{
					Provider: provEntry.provider,
					Service:  svcEntry.service,
					Channel:  ch.name,
					Board:    strings.TrimSpace(ch.board),
				})
			}
		}
	}

	if len(targets) == 0 {
		// provider 存在但无可订阅热板项（全部为冷板）
		return nil, &ColdBoardError{Provider: provEntry.provider}
	}

	return targets, nil
}

// ValidateAndExpandService 验证 service 并返回所有 channel
// 用于 /add <provider> <service> 展开订阅
func (v *RelayPulseValidator) ValidateAndExpandService(ctx context.Context, provider, service string) ([]CanonicalTarget, error) {
	provider = strings.TrimSpace(provider)
	service = strings.TrimSpace(service)

	if provider == "" {
		return nil, &NotFoundError{
			Level:    NotFoundProvider,
			Provider: provider,
		}
	}

	if service == "" {
		return nil, &NotFoundError{
			Level:    NotFoundService,
			Provider: provider,
		}
	}

	svcEntry, err := v.getServiceEntry(ctx, provider, service)
	if err != nil {
		return nil, err
	}

	var targets []CanonicalTarget
	if len(svcEntry.channels) == 0 {
		// service 无 channel，添加 service 级订阅
		targets = append(targets, CanonicalTarget{
			Provider: svcEntry.provider,
			Service:  svcEntry.service,
			Channel:  "",
		})
	} else {
		// 展开每个 channel，跳过冷板
		for _, ch := range svcEntry.channels {
			// 冷板订阅拒绝：仅当 relay-pulse 返回 board=cold 时生效；缺失时按 hot 处理（兼容旧版本）
			if strings.EqualFold(strings.TrimSpace(ch.board), "cold") {
				continue
			}
			targets = append(targets, CanonicalTarget{
				Provider: svcEntry.provider,
				Service:  svcEntry.service,
				Channel:  ch.name,
				Board:    strings.TrimSpace(ch.board),
			})
		}
	}

	if len(targets) == 0 {
		// service 存在但无可订阅热板项（全部为冷板）
		return nil, &ColdBoardError{Provider: svcEntry.provider, Service: svcEntry.service}
	}

	return targets, nil
}

// getServiceEntry 获取 service 缓存条目（带 singleflight）
func (v *RelayPulseValidator) getServiceEntry(ctx context.Context, provider, service string) (*serviceCacheEntry, error) {
	key := "svc:" + strings.ToLower(provider) + "/" + strings.ToLower(service)

	// 检查缓存
	v.mu.Lock()
	if entry, ok := v.serviceCache[key]; ok && time.Now().Before(entry.expireAt) {
		v.mu.Unlock()
		if entry.notFound {
			// 负向缓存：尝试获取候选列表，但不忽略错误
			candidates, err := v.getProviderServices(ctx, provider)
			if err != nil {
				// API 不可用时返回 UnavailableError，保持语义一致
				var ue *UnavailableError
				if errors.As(err, &ue) {
					return nil, err
				}
				// NotFoundError（provider 不存在）时继续返回原错误，不带候选
			}
			return nil, &NotFoundError{
				Level:      entry.level,
				Provider:   provider,
				Service:    service,
				Candidates: limitSlice(candidates, maxCandidates), // 候选列表截断
			}
		}
		return entry, nil
	}

	// singleflight: 检查是否有进行中的请求
	if call, ok := v.inflight[key]; ok {
		v.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, &UnavailableError{Cause: ctx.Err()}
		case <-call.done:
			if call.err != nil {
				return nil, call.err
			}
			return call.val.(*serviceCacheEntry), nil
		}
	}

	// 创建新的 inflight 调用
	call := &inflightCall{done: make(chan struct{})}
	v.inflight[key] = call
	v.mu.Unlock()

	// 执行 API 调用
	entry, err := v.fetchServiceEntry(ctx, provider, service)

	// 更新缓存和 inflight
	v.mu.Lock()
	v.evictExpiredCacheLocked() // 写入前清理过期缓存
	if err == nil {
		entry.expireAt = time.Now().Add(v.positiveTTL)
		v.serviceCache[key] = entry
	} else {
		var nf *NotFoundError
		if errors.As(err, &nf) {
			// 负向缓存
			v.serviceCache[key] = &serviceCacheEntry{
				expireAt: time.Now().Add(v.negativeTTL),
				notFound: true,
				level:    nf.Level,
			}
		}
	}
	call.val = entry
	call.err = err
	close(call.done)
	delete(v.inflight, key)
	v.mu.Unlock()

	if err != nil {
		return nil, err
	}
	return entry, nil
}

// getProviderServices 获取 provider 下的 services 列表（用于候选提示）
// 内部调用 getProviderEntry 以复用缓存和 singleflight 逻辑
func (v *RelayPulseValidator) getProviderServices(ctx context.Context, provider string) ([]string, error) {
	entry, err := v.getProviderEntry(ctx, provider)
	if err != nil {
		return nil, err
	}
	return entry.services, nil
}

// getProviderEntry 获取 provider 缓存条目（包含 canonicalProvider）
func (v *RelayPulseValidator) getProviderEntry(ctx context.Context, provider string) (*providerCacheEntry, error) {
	key := "prov:" + strings.ToLower(provider)

	// 检查缓存
	v.mu.Lock()
	if entry, ok := v.providerCache[key]; ok && time.Now().Before(entry.expireAt) {
		v.mu.Unlock()
		if entry.notFound {
			return nil, &NotFoundError{Level: NotFoundProvider, Provider: provider}
		}
		return entry, nil
	}

	// singleflight
	if call, ok := v.inflight[key]; ok {
		v.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, &UnavailableError{Cause: ctx.Err()}
		case <-call.done:
			if call.err != nil {
				return nil, call.err
			}
			return call.val.(*providerCacheEntry), nil
		}
	}

	call := &inflightCall{done: make(chan struct{})}
	v.inflight[key] = call
	v.mu.Unlock()

	// 执行 API 调用
	services, canonicalProvider, err := v.fetchProviderServices(ctx, provider)

	var entry *providerCacheEntry
	v.mu.Lock()
	v.evictExpiredCacheLocked() // 写入前清理过期缓存
	if err == nil {
		entry = &providerCacheEntry{
			expireAt: time.Now().Add(v.positiveTTL),
			provider: canonicalProvider,
			services: services,
			notFound: false,
		}
		v.providerCache[key] = entry
	} else {
		var nf *NotFoundError
		if errors.As(err, &nf) {
			v.providerCache[key] = &providerCacheEntry{
				expireAt: time.Now().Add(v.negativeTTL),
				notFound: true,
			}
		}
	}
	call.val = entry
	call.err = err
	close(call.done)
	delete(v.inflight, key)
	v.mu.Unlock()

	if err != nil {
		return nil, err
	}
	return entry, nil
}

// fetchServiceEntry 从 API 获取 service 信息
func (v *RelayPulseValidator) fetchServiceEntry(ctx context.Context, provider, service string) (*serviceCacheEntry, error) {
	resp, err := v.callStatusQuery(ctx, provider, service, "")
	if err != nil {
		return nil, &UnavailableError{Cause: err}
	}

	if resp.Error != nil {
		// 只有 NOT_FOUND 错误码才转为 NotFoundError，其他错误视为服务不可用
		if !strings.EqualFold(resp.Error.Code, "NOT_FOUND") {
			return nil, &UnavailableError{Cause: fmt.Errorf("API 错误: %s - %s", resp.Error.Code, resp.Error.Message)}
		}

		level := parseNotFoundLevel(resp.Error.Message)
		nfErr := &NotFoundError{
			Level:    level,
			Provider: provider,
			Service:  service,
		}

		// 如果是 service 不存在，尝试获取候选列表
		if level == NotFoundService {
			candidates, _ := v.getProviderServices(ctx, provider)
			nfErr.Candidates = limitSlice(candidates, maxCandidates) // 候选列表截断
		}

		return nil, nfErr
	}

	// 解析响应
	entry := &serviceCacheEntry{
		provider: resp.Provider,
	}

	if len(resp.Services) > 0 {
		entry.service = resp.Services[0].Name
		for _, ch := range resp.Services[0].Channels {
			entry.channels = append(entry.channels, channelEntry{
				name:  ch.Name,
				board: strings.TrimSpace(ch.Board),
			})
		}
	}

	return entry, nil
}

// fetchProviderServices 从 API 获取 provider 下的 services 列表
func (v *RelayPulseValidator) fetchProviderServices(ctx context.Context, provider string) ([]string, string, error) {
	resp, err := v.callStatusQuery(ctx, provider, "", "")
	if err != nil {
		return nil, "", &UnavailableError{Cause: err}
	}

	if resp.Error != nil {
		// 只有 NOT_FOUND 错误码才转为 NotFoundError，其他错误视为服务不可用
		if !strings.EqualFold(resp.Error.Code, "NOT_FOUND") {
			return nil, "", &UnavailableError{Cause: fmt.Errorf("API 错误: %s - %s", resp.Error.Code, resp.Error.Message)}
		}
		return nil, "", &NotFoundError{
			Level:    NotFoundProvider,
			Provider: provider,
		}
	}

	var services []string
	for _, svc := range resp.Services {
		services = append(services, svc.Name)
	}

	// 注意：这里返回完整的 services 列表，不做截断
	// 截断仅用于候选提示（Candidates），不用于实际展开
	return services, resp.Provider, nil
}

// ===== API 客户端 =====

// statusQueryResponse API 响应结构
type statusQueryResponse struct {
	Provider string `json:"provider,omitempty"`
	Services []struct {
		Name     string `json:"name"`
		Channels []struct {
			Name  string `json:"name"`
			Board string `json:"board,omitempty"` // hot/cold
		} `json:"channels"`
	} `json:"services,omitempty"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// callStatusQuery 调用 /api/status/query 接口
func (v *RelayPulseValidator) callStatusQuery(ctx context.Context, provider, service, channel string) (*statusQueryResponse, error) {
	u, err := url.Parse(v.statusQueryURL)
	if err != nil {
		return nil, fmt.Errorf("无效的 status_query_url: %w", err)
	}

	q := u.Query()
	q.Set("provider", provider)
	if service != "" {
		q.Set("service", service)
	}
	if channel != "" {
		q.Set("channel", channel)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	// 注意：不手动设置 Accept-Encoding，让 http.Transport 自动处理 gzip 压缩和解压

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	// 解析外层响应
	var apiResp struct {
		Results []statusQueryResponse `json:"results"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if len(apiResp.Results) == 0 {
		return nil, fmt.Errorf("响应 results 为空")
	}

	return &apiResp.Results[0], nil
}

// ===== 辅助函数 =====

// deriveStatusQueryURL 从 events_url 推导 status/query URL
func deriveStatusQueryURL(eventsURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(eventsURL))
	if err != nil {
		return "", fmt.Errorf("events_url 无效: %w", err)
	}

	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("events_url 无效: 缺少 scheme 或 host")
	}

	// 从 /api/events 推导为 /api/status/query
	path := u.Path
	switch {
	case strings.HasSuffix(path, "/api/events"):
		path = strings.TrimSuffix(path, "/api/events") + "/api/status/query"
	case strings.HasSuffix(path, "/api/events/"):
		path = strings.TrimSuffix(path, "/api/events/") + "/api/status/query"
	default:
		// 无法推断时使用根路径
		path = "/api/status/query"
	}

	u.Path = path
	u.RawQuery = ""
	u.Fragment = ""

	return u.String(), nil
}

// parseNotFoundLevel 从错误消息解析不存在层级
func parseNotFoundLevel(msg string) NotFoundLevel {
	msg = strings.TrimSpace(msg)
	switch msg {
	case "provider 不存在":
		return NotFoundProvider
	case "service 不存在":
		return NotFoundService
	case "channel 不存在":
		return NotFoundChannel
	default:
		// 回退：从关键词推断
		lower := strings.ToLower(msg)
		if strings.Contains(lower, "provider") {
			return NotFoundProvider
		}
		if strings.Contains(lower, "channel") {
			return NotFoundChannel
		}
		return NotFoundService
	}
}

// limitSlice 限制切片长度
func limitSlice(s []string, max int) []string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

// evictExpiredCacheLocked 清理过期缓存条目（调用前必须持有 v.mu 锁）
// 如果清理后仍超过 maxCacheEntries，则清空所有缓存
func (v *RelayPulseValidator) evictExpiredCacheLocked() {
	totalEntries := len(v.providerCache) + len(v.serviceCache)
	if totalEntries < maxCacheEntries {
		return
	}

	now := time.Now()

	// 清理过期的 provider 缓存
	for key, entry := range v.providerCache {
		if now.After(entry.expireAt) {
			delete(v.providerCache, key)
		}
	}

	// 清理过期的 service 缓存
	for key, entry := range v.serviceCache {
		if now.After(entry.expireAt) {
			delete(v.serviceCache, key)
		}
	}

	// 如果清理后仍超过限制，清空所有缓存（简单粗暴但有效）
	totalEntries = len(v.providerCache) + len(v.serviceCache)
	if totalEntries >= maxCacheEntries {
		v.providerCache = make(map[string]*providerCacheEntry)
		v.serviceCache = make(map[string]*serviceCacheEntry)
	}
}
