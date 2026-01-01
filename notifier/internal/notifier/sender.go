package notifier

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"sync"
	"time"

	"notifier/internal/config"
	"notifier/internal/poller"
	"notifier/internal/qq"
	"notifier/internal/storage"
	"notifier/internal/telegram"
)

// Sender 通知发送器（多平台）
type Sender struct {
	cfg      *config.Config
	storage  storage.Storage
	tgClient *telegram.Client
	qqClient *qq.Client

	// 限流
	rateLimiter *time.Ticker
	mu          sync.Mutex
	running     bool
	stopChan    chan struct{}
}

// NewSender 创建发送器
func NewSender(cfg *config.Config, store storage.Storage) *Sender {
	s := &Sender{
		cfg:         cfg,
		storage:     store,
		rateLimiter: time.NewTicker(time.Second / time.Duration(cfg.Limits.RateLimitPerSecond)),
		stopChan:    make(chan struct{}),
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
	s.mu.Unlock()

	slog.Info("通知发送器启动",
		"rate_limit", s.cfg.Limits.RateLimitPerSecond,
		"telegram_enabled", s.tgClient != nil,
		"qq_enabled", s.qqClient != nil,
	)

	// 启动重试处理
	go s.retryLoop(ctx)

	<-ctx.Done()
	return ctx.Err()
}

// Stop 停止发送器
func (s *Sender) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		close(s.stopChan)
		s.rateLimiter.Stop()
		s.running = false
	}
}

// HandleEvent 处理事件（由 Poller 调用）
func (s *Sender) HandleEvent(ctx context.Context, event *poller.Event) error {
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

// sendNotification 发送单条通知（多平台路由）
func (s *Sender) sendNotification(ctx context.Context, delivery *storage.Delivery, event *poller.Event) {
	// 等待限流
	select {
	case <-ctx.Done():
		return
	case <-s.rateLimiter.C:
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

%s%s

时间: %s`,
		emoji, statusText,
		location,
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

	return fmt.Sprintf("%s %s\n\n%s%s\n\n时间: %s", emoji, statusText, location, details, eventTime)
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
	// 等待限流
	select {
	case <-ctx.Done():
		return
	case <-s.rateLimiter.C:
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
