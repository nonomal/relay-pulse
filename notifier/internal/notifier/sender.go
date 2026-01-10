package notifier

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"notifier/internal/config"
	"notifier/internal/poller"
	"notifier/internal/qq"
	"notifier/internal/storage"
	"notifier/internal/telegram"
)

// aggregateKey 事件聚合键：按 provider/service/channel/event_type 分组
type aggregateKey struct {
	Provider  string
	Service   string
	Channel   string
	EventType string
}

// eventAggregate 事件聚合缓冲区
type eventAggregate struct {
	firstAt time.Time           // 首个事件到达时间
	timer   *time.Timer         // 聚合窗口定时器
	base    *poller.Event       // 基准事件（用于构建通知）
	models  map[string]struct{} // 收集的所有 model
}

// Sender 通知发送器（多平台）
type Sender struct {
	cfg      *config.Config
	storage  storage.Storage
	tgClient *telegram.Client
	qqClient *qq.Client

	// 平台独立限流器
	tgRateLimiter *time.Ticker
	qqRateLimiter *time.Ticker
	qqJitterMin   time.Duration
	qqJitterMax   time.Duration

	mu       sync.Mutex
	running  bool
	stopChan chan struct{}
	baseCtx  context.Context // 保存启动时的 context，供定时器回调使用

	// 事件聚合：按 provider/service/channel/event_type 分组
	// 同一监测组下多个 model 的事件会在时间窗口内合并为一条通知
	aggWindow time.Duration
	aggMu     sync.Mutex
	aggBuf    map[aggregateKey]*eventAggregate
}

// DefaultAggregateWindow 默认事件聚合窗口时长
// 同一监测组（provider/service/channel）下的多个 model 事件
// 在此时间窗口内会合并为一条通知
//
// 窗口计算依据：
// - 组内探测间隔：2秒/model，6 个 model 需要 10 秒启动完
// - 慢响应时间：可能高达 10 秒
// - poller 轮询间隔：5 秒
// - 最坏情况：10s（启动间隔）+ 10s（慢响应）+ 5s（轮询对齐）= 25s
// - 取 30 秒留有余量
const DefaultAggregateWindow = 30 * time.Second

// NewSender 创建发送器
func NewSender(cfg *config.Config, store storage.Storage) *Sender {
	// 计算限流间隔
	tgRPS := cfg.Limits.TelegramRateLimitPerSecond
	if tgRPS <= 0 {
		tgRPS = 25
	}
	qqRPS := cfg.Limits.QQRateLimitPerSecond
	if qqRPS <= 0 {
		qqRPS = 2
	}

	s := &Sender{
		cfg:           cfg,
		storage:       store,
		tgRateLimiter: time.NewTicker(time.Second / time.Duration(tgRPS)),
		qqRateLimiter: time.NewTicker(time.Second / time.Duration(qqRPS)),
		qqJitterMin:   cfg.Limits.QQJitterMin,
		qqJitterMax:   cfg.Limits.QQJitterMax,
		stopChan:      make(chan struct{}),
		aggWindow:     DefaultAggregateWindow,
		aggBuf:        make(map[aggregateKey]*eventAggregate),
	}

	// 按配置初始化客户端
	if cfg.HasTelegramToken() {
		s.tgClient = telegram.NewClient(cfg.Telegram.BotToken)
	}
	if cfg.HasQQ() {
		s.qqClient = qq.NewClient(cfg.QQ.OneBotHTTPURL, cfg.QQ.AccessToken)
	}

	return s
}

// Start 启动发送器
func (s *Sender) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("发送器已在运行")
	}
	s.running = true
	s.stopChan = make(chan struct{})
	s.baseCtx = ctx
	s.mu.Unlock()

	slog.Info("通知发送器启动",
		"telegram_rate_limit", s.cfg.Limits.TelegramRateLimitPerSecond,
		"qq_rate_limit", s.cfg.Limits.QQRateLimitPerSecond,
		"qq_jitter_min", s.qqJitterMin,
		"qq_jitter_max", s.qqJitterMax,
		"telegram_enabled", s.tgClient != nil,
		"qq_enabled", s.qqClient != nil,
		"aggregate_window", s.aggWindow,
	)

	// 启动重试处理
	go s.retryLoop(ctx)

	<-ctx.Done()
	return ctx.Err()
}

// Stop 停止发送器
func (s *Sender) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.stopChan)
	s.mu.Unlock()

	// 在释放 mu 锁后再 flush，避免死锁
	// 先 flush 再停止 rateLimiter，否则 sendNotification 会阻塞在等待 tick
	s.flushAllAggregates()

	// 最后停止限流器
	if s.tgRateLimiter != nil {
		s.tgRateLimiter.Stop()
	}
	if s.qqRateLimiter != nil {
		s.qqRateLimiter.Stop()
	}
}

// flushAllAggregates 刷新所有聚合缓冲区（用于优雅关闭）
func (s *Sender) flushAllAggregates() {
	s.aggMu.Lock()
	keys := make([]aggregateKey, 0, len(s.aggBuf))
	for k, agg := range s.aggBuf {
		if agg.timer != nil {
			agg.timer.Stop()
		}
		keys = append(keys, k)
	}
	s.aggMu.Unlock()

	// 立即 flush 所有待发送的聚合事件
	for _, key := range keys {
		s.flushAggregate(key)
	}
}

// HandleEvent 处理事件（由 Poller 调用）
// 事件会先进入聚合缓冲区，等待时间窗口结束后合并发送
// 这样同一监测组下多个 model 的 DOWN/UP 事件会合并为一条通知
func (s *Sender) HandleEvent(ctx context.Context, event *poller.Event) error {
	if event == nil {
		return nil
	}

	key := aggregateKey{
		Provider:  event.Provider,
		Service:   event.Service,
		Channel:   event.Channel,
		EventType: event.Type,
	}

	now := time.Now()

	s.aggMu.Lock()
	agg := s.aggBuf[key]
	if agg == nil {
		// 首个事件：创建聚合缓冲区并启动定时器
		baseCopy := *event
		agg = &eventAggregate{
			firstAt: now,
			base:    &baseCopy,
			models:  make(map[string]struct{}),
		}
		agg.timer = time.AfterFunc(s.aggWindow, func() {
			s.flushAggregate(key)
		})
		s.aggBuf[key] = agg

		slog.Debug("事件聚合开始",
			"provider", event.Provider,
			"service", event.Service,
			"channel", event.Channel,
			"type", event.Type,
			"window", s.aggWindow,
		)
	}

	// 收集 model（去重）
	if model := strings.TrimSpace(event.Model); model != "" {
		agg.models[model] = struct{}{}
	}
	s.aggMu.Unlock()

	return nil
}

// flushAggregate 刷新聚合缓冲区，发送合并后的通知
func (s *Sender) flushAggregate(key aggregateKey) {
	s.aggMu.Lock()
	agg := s.aggBuf[key]
	if agg == nil {
		s.aggMu.Unlock()
		return
	}
	delete(s.aggBuf, key)
	if agg.timer != nil {
		agg.timer.Stop()
	}
	s.aggMu.Unlock()

	// 收集并排序 models
	models := make([]string, 0, len(agg.models))
	for m := range agg.models {
		if strings.TrimSpace(m) != "" {
			models = append(models, m)
		}
	}
	sort.Strings(models)

	// 构建合并后的事件
	merged := *agg.base
	if merged.Meta == nil {
		merged.Meta = make(map[string]any)
	} else {
		// 深拷贝 Meta 防止并发问题
		metaCopy := make(map[string]any, len(merged.Meta)+1)
		for k, v := range merged.Meta {
			metaCopy[k] = v
		}
		merged.Meta = metaCopy
	}
	if len(models) > 0 {
		merged.Meta["models"] = models
	}

	slog.Info("事件聚合完成",
		"provider", key.Provider,
		"service", key.Service,
		"channel", key.Channel,
		"type", key.EventType,
		"models", models,
	)

	// 获取发送用的 context
	sendCtx := s.getSendContext()
	if err := s.dispatchEvent(sendCtx, &merged); err != nil {
		slog.Error("聚合事件发送失败",
			"provider", key.Provider,
			"service", key.Service,
			"channel", key.Channel,
			"type", key.EventType,
			"error", err,
		)
	}
}

// getSendContext 获取用于发送通知的 context
// 如果服务已停止或 baseCtx 已取消，返回 Background context 确保 flush 能完成
func (s *Sender) getSendContext() context.Context {
	s.mu.Lock()
	running := s.running
	ctx := s.baseCtx
	s.mu.Unlock()

	// 服务已停止时，使用 Background 确保 flush 能完成发送
	if !running {
		return context.Background()
	}

	if ctx != nil && ctx.Err() == nil {
		return ctx
	}
	return context.Background()
}

// dispatchEvent 分发事件通知给所有订阅者
func (s *Sender) dispatchEvent(ctx context.Context, event *poller.Event) error {
	// 查找订阅者（返回 platform + chatID）
	subscribers, err := s.storage.GetSubscribersByMonitor(ctx, event.Provider, event.Service, event.Channel)
	if err != nil {
		return fmt.Errorf("查询订阅者失败: %w", err)
	}

	if len(subscribers) == 0 {
		return nil
	}

	slog.Info("分发事件通知",
		"event_id", event.ID,
		"provider", event.Provider,
		"service", event.Service,
		"subscribers", len(subscribers),
	)

	// 为每个订阅者创建投递记录并发送
	for _, ref := range subscribers {
		delivery := &storage.Delivery{
			EventID:  event.ID,
			Platform: ref.Platform,
			ChatID:   ref.ChatID,
			Status:   storage.DeliveryStatusPending,
		}

		// 创建投递记录（幂等）
		if err := s.storage.CreateDelivery(ctx, delivery); err != nil {
			slog.Warn("创建投递记录失败",
				"event_id", event.ID,
				"platform", ref.Platform,
				"chat_id", ref.ChatID,
				"error", err,
			)
			continue
		}

		// 异步发送
		go s.sendNotification(ctx, delivery, event)
	}

	return nil
}

// sleepWithContext 带 context 的 sleep，返回 true 表示正常完成，false 表示被取消
func (s *Sender) sleepWithContext(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-s.stopChan:
		// 服务正在关闭，跳过等待
		return true
	case <-t.C:
		return true
	}
}

// waitPlatformRateLimit 等待平台限流，返回 true 表示可以发送，false 表示应该放弃
func (s *Sender) waitPlatformRateLimit(ctx context.Context, platform string) bool {
	// 先检查 context 是否已取消
	select {
	case <-ctx.Done():
		return false
	case <-s.stopChan:
		// 服务正在关闭，跳过限流直接发送
		return true
	default:
	}

	// 根据平台选择限流器
	var limiter <-chan time.Time
	switch platform {
	case storage.PlatformTelegram:
		if s.tgRateLimiter == nil {
			return true
		}
		limiter = s.tgRateLimiter.C
	case storage.PlatformQQ:
		if s.qqRateLimiter == nil {
			return true
		}
		limiter = s.qqRateLimiter.C
	default:
		// 未知平台不做限流
		return true
	}

	// 等待限流
	select {
	case <-ctx.Done():
		return false
	case <-s.stopChan:
		// 服务正在关闭，跳过限流直接发送
		return true
	case <-limiter:
	}

	// QQ 额外抖动：进一步错峰，降低风控
	if platform == storage.PlatformQQ && s.qqJitterMax > 0 {
		min := s.qqJitterMin
		max := s.qqJitterMax
		if max < min {
			min, max = max, min
		}
		jitter := min
		if max > min {
			jitter += time.Duration(rand.Int63n(int64(max - min + 1)))
		}
		return s.sleepWithContext(ctx, jitter)
	}

	return true
}

// sendNotification 发送单条通知（多平台路由）
func (s *Sender) sendNotification(ctx context.Context, delivery *storage.Delivery, event *poller.Event) {
	// 等待平台限流
	if !s.waitPlatformRateLimit(ctx, delivery.Platform) {
		return
	}

	var (
		messageID string
		err       error
	)

	switch delivery.Platform {
	case storage.PlatformTelegram:
		messageID, err = s.sendTelegram(ctx, delivery, event)
	case storage.PlatformQQ:
		messageID, err = s.sendQQ(ctx, delivery, event)
	default:
		err = fmt.Errorf("unknown platform: %s", delivery.Platform)
	}

	if err != nil {
		s.handleSendError(ctx, delivery, err)
		return
	}

	// 发送成功
	if err := s.storage.UpdateDeliveryStatus(ctx, delivery.ID, storage.DeliveryStatusSent, messageID, ""); err != nil {
		slog.Error("更新投递状态失败", "error", err)
	}
}

// sendTelegram 发送 Telegram 消息
func (s *Sender) sendTelegram(ctx context.Context, delivery *storage.Delivery, event *poller.Event) (string, error) {
	if s.tgClient == nil {
		return "", fmt.Errorf("telegram client not configured")
	}

	msg := s.formatMessageTelegram(event)
	result, err := s.tgClient.SendMessageHTML(ctx, delivery.ChatID, msg)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%d", result.MessageID), nil
}

// sendQQ 发送 QQ 消息
func (s *Sender) sendQQ(ctx context.Context, delivery *storage.Delivery, event *poller.Event) (string, error) {
	if s.qqClient == nil {
		return "", fmt.Errorf("qq client not configured")
	}

	text := s.formatMessageQQ(event)
	var mid int64
	var err error

	// 负数 chatID 表示群聊，正数表示私聊
	if delivery.ChatID < 0 {
		mid, err = s.qqClient.SendGroupMessage(ctx, -delivery.ChatID, text)
	} else {
		mid, err = s.qqClient.SendPrivateMessage(ctx, delivery.ChatID, text)
	}

	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%d", mid), nil
}

// extractModels 从事件中提取所有 model 信息
// 优先从 Meta["models"] 读取（聚合后的事件），回退到 event.Model（单个事件）
func extractModels(event *poller.Event) []string {
	if event == nil {
		return nil
	}

	seen := make(map[string]struct{})
	var models []string

	// 从 Meta["models"] 读取（聚合后的事件会设置这个字段）
	if event.Meta != nil {
		if v, ok := event.Meta["models"]; ok {
			switch t := v.(type) {
			case []string:
				for _, m := range t {
					m = strings.TrimSpace(m)
					if m == "" {
						continue
					}
					if _, exists := seen[m]; exists {
						continue
					}
					seen[m] = struct{}{}
					models = append(models, m)
				}
			case []any:
				// JSON 解码后可能是 []any 类型
				for _, raw := range t {
					m, ok := raw.(string)
					if !ok {
						continue
					}
					m = strings.TrimSpace(m)
					if m == "" {
						continue
					}
					if _, exists := seen[m]; exists {
						continue
					}
					seen[m] = struct{}{}
					models = append(models, m)
				}
			}
		}
	}

	// 回退到单个 event.Model（兼容未聚合的事件）
	if m := strings.TrimSpace(event.Model); m != "" {
		if _, exists := seen[m]; !exists {
			seen[m] = struct{}{}
			models = append(models, m)
		}
	}

	// 保持稳定排序
	if len(models) > 1 {
		sort.Strings(models)
	}

	return models
}

// handleSendError 处理发送错误
func (s *Sender) handleSendError(ctx context.Context, delivery *storage.Delivery, sendErr error) {
	slog.Warn("发送通知失败",
		"delivery_id", delivery.ID,
		"platform", delivery.Platform,
		"chat_id", delivery.ChatID,
		"error", sendErr,
	)

	// Telegram 平台检查是否被封禁
	if delivery.Platform == storage.PlatformTelegram && telegram.IsForbiddenError(sendErr) {
		// 标记用户为 blocked
		if err := s.storage.UpdateChatStatus(ctx, delivery.Platform, delivery.ChatID, storage.ChatStatusBlocked); err != nil {
			slog.Error("更新用户状态失败", "error", err)
		}
		// 标记投递失败
		if err := s.storage.UpdateDeliveryStatus(ctx, delivery.ID, storage.DeliveryStatusFailed, "", "user blocked bot"); err != nil {
			slog.Error("更新投递状态失败", "error", err)
		}
		return
	}

	// 增加重试计数
	if err := s.storage.IncrementRetryCount(ctx, delivery.ID); err != nil {
		slog.Error("增加重试计数失败", "error", err)
	}

	// 更新错误信息
	if err := s.storage.UpdateDeliveryStatus(ctx, delivery.ID, storage.DeliveryStatusPending, "", sendErr.Error()); err != nil {
		slog.Error("更新投递状态失败", "error", err)
	}
}

// formatMessageTelegram 格式化 Telegram 消息（HTML）
func (s *Sender) formatMessageTelegram(event *poller.Event) string {
	var emoji string
	var statusText string

	switch event.Type {
	case "UP":
		emoji = "🟢"
		statusText = "服务已恢复"
	case "DOWN":
		emoji = "🔴"
		statusText = "服务不可用"
	default:
		switch event.ToStatus {
		case 1:
			emoji = "🟢"
			statusText = "服务已恢复"
		case 2:
			emoji = "🟡"
			statusText = "服务波动"
		case 0:
			emoji = "🔴"
			statusText = "服务不可用"
		default:
			emoji = "⚪"
			statusText = "状态变更"
		}
	}

	// 转义 HTML 防止注入
	provider := html.EscapeString(event.Provider)
	service := html.EscapeString(event.Service)
	channel := html.EscapeString(event.Channel)

	location := fmt.Sprintf("<b>%s</b> / <b>%s</b>", provider, service)
	if channel != "" {
		location += fmt.Sprintf(" / <b>%s</b>", channel)
	}

	// 模型信息（多模型监测组会显示所有受影响的模型）
	var modelLine string
	models := extractModels(event)
	if len(models) == 1 {
		modelLine = fmt.Sprintf("\n模型: %s", html.EscapeString(models[0]))
	} else if len(models) > 1 {
		modelLine = fmt.Sprintf("\n模型: %s", html.EscapeString(strings.Join(models, ", ")))
	}

	var details string
	if subStatus, ok := event.Meta["sub_status"]; ok {
		details = fmt.Sprintf("\n原因: %s", html.EscapeString(fmt.Sprintf("%v", subStatus)))
	}

	eventTs := event.ObservedAt
	if eventTs == 0 {
		eventTs = event.CreatedAt
	}
	cst := time.FixedZone("CST", 8*60*60)
	eventTime := time.Unix(eventTs, 0).In(cst).Format("2006-01-02 15:04:05")

	return fmt.Sprintf(`%s <b>%s</b>

%s%s%s

时间: %s`,
		emoji, statusText,
		location,
		modelLine,
		details,
		eventTime,
	)
}

// formatMessageQQ 格式化 QQ 消息（纯文本）
func (s *Sender) formatMessageQQ(event *poller.Event) string {
	var emoji string
	var statusText string

	switch event.Type {
	case "UP":
		emoji = "🟢"
		statusText = "服务已恢复"
	case "DOWN":
		emoji = "🔴"
		statusText = "服务不可用"
	default:
		switch event.ToStatus {
		case 1:
			emoji = "🟢"
			statusText = "服务已恢复"
		case 2:
			emoji = "🟡"
			statusText = "服务波动"
		case 0:
			emoji = "🔴"
			statusText = "服务不可用"
		default:
			emoji = "⚪"
			statusText = "状态变更"
		}
	}

	location := fmt.Sprintf("%s / %s", event.Provider, event.Service)
	if event.Channel != "" {
		location += fmt.Sprintf(" / %s", event.Channel)
	}

	// 模型信息（多模型监测组会显示所有受影响的模型）
	var modelLine string
	models := extractModels(event)
	if len(models) == 1 {
		modelLine = fmt.Sprintf("\n模型: %s", models[0])
	} else if len(models) > 1 {
		modelLine = fmt.Sprintf("\n模型: %s", strings.Join(models, ", "))
	}

	var details string
	if subStatus, ok := event.Meta["sub_status"]; ok {
		details = fmt.Sprintf("\n原因: %v", subStatus)
	}

	eventTs := event.ObservedAt
	if eventTs == 0 {
		eventTs = event.CreatedAt
	}
	cst := time.FixedZone("CST", 8*60*60)
	eventTime := time.Unix(eventTs, 0).In(cst).Format("2006-01-02 15:04:05")

	return fmt.Sprintf("%s %s\n\n%s%s%s\n\n时间: %s", emoji, statusText, location, modelLine, details, eventTime)
}

// retryLoop 重试失败的投递
func (s *Sender) retryLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.processRetries(ctx)
		}
	}
}

// processRetries 处理重试
func (s *Sender) processRetries(ctx context.Context) {
	deliveries, err := s.storage.GetPendingDeliveries(ctx, 100)
	if err != nil {
		slog.Error("获取待重试投递失败", "error", err)
		return
	}

	for _, delivery := range deliveries {
		// 检查重试次数
		if delivery.RetryCount >= s.cfg.Limits.MaxRetries {
			// 超过最大重试次数，标记为失败
			if err := s.storage.UpdateDeliveryStatus(ctx, delivery.ID, storage.DeliveryStatusFailed, "", "max retries exceeded"); err != nil {
				slog.Error("更新投递状态失败", "error", err)
			}
			continue
		}

		// 重新发送
		go s.retryDelivery(ctx, delivery)
	}
}

// retryDelivery 重试单条投递
func (s *Sender) retryDelivery(ctx context.Context, delivery *storage.Delivery) {
	// 等待平台限流
	if !s.waitPlatformRateLimit(ctx, delivery.Platform) {
		return
	}

	// 简单的重试消息
	msg := fmt.Sprintf("🔔 通知重试 (event_id: %d)\n\n如果您持续收到此消息，请检查订阅设置。", delivery.EventID)

	var err error
	var messageID string

	switch delivery.Platform {
	case storage.PlatformTelegram:
		if s.tgClient == nil {
			err = fmt.Errorf("telegram client not configured")
		} else {
			result, sendErr := s.tgClient.SendMessageHTML(ctx, delivery.ChatID, msg)
			if sendErr == nil && result != nil {
				messageID = fmt.Sprintf("%d", result.MessageID)
			}
			err = sendErr
		}

	case storage.PlatformQQ:
		if s.qqClient == nil {
			err = fmt.Errorf("qq client not configured")
		} else {
			var mid int64
			if delivery.ChatID < 0 {
				mid, err = s.qqClient.SendGroupMessage(ctx, -delivery.ChatID, msg)
			} else {
				mid, err = s.qqClient.SendPrivateMessage(ctx, delivery.ChatID, msg)
			}
			if err == nil {
				messageID = fmt.Sprintf("%d", mid)
			}
		}

	default:
		err = fmt.Errorf("unknown platform: %s", delivery.Platform)
	}

	if err != nil {
		slog.Warn("重试投递失败",
			"delivery_id", delivery.ID,
			"platform", delivery.Platform,
			"chat_id", delivery.ChatID,
			"retry_count", delivery.RetryCount,
			"error", err,
		)

		// Telegram 封禁检查
		if delivery.Platform == storage.PlatformTelegram && telegram.IsForbiddenError(err) {
			if err := s.storage.UpdateChatStatus(ctx, delivery.Platform, delivery.ChatID, storage.ChatStatusBlocked); err != nil {
				slog.Error("更新用户状态失败", "error", err)
			}
			if err := s.storage.UpdateDeliveryStatus(ctx, delivery.ID, storage.DeliveryStatusFailed, "", "user blocked bot"); err != nil {
				slog.Error("更新投递状态失败", "error", err)
			}
			return
		}

		if err := s.storage.IncrementRetryCount(ctx, delivery.ID); err != nil {
			slog.Error("增加重试计数失败", "error", err)
		}
		return
	}

	if err := s.storage.UpdateDeliveryStatus(ctx, delivery.ID, storage.DeliveryStatusSent, messageID, ""); err != nil {
		slog.Error("更新投递状态失败", "error", err)
	}
}
