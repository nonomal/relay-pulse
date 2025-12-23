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
	"notifier/internal/storage"
	"notifier/internal/telegram"
)

// Sender 通知发送器
type Sender struct {
	cfg      *config.Config
	storage  storage.Storage
	tgClient *telegram.Client

	// 限流
	rateLimiter *time.Ticker
	mu          sync.Mutex
	running     bool
	stopChan    chan struct{}
}

// NewSender 创建发送器
func NewSender(cfg *config.Config, store storage.Storage) *Sender {
	return &Sender{
		cfg:         cfg,
		storage:     store,
		tgClient:    telegram.NewClient(cfg.Telegram.BotToken),
		rateLimiter: time.NewTicker(time.Second / time.Duration(cfg.Limits.RateLimitPerSecond)),
		stopChan:    make(chan struct{}),
	}
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

	slog.Info("通知发送器启动", "rate_limit", s.cfg.Limits.RateLimitPerSecond)

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
	// 查找订阅者
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
	for _, chatID := range subscribers {
		delivery := &storage.Delivery{
			EventID: event.ID,
			ChatID:  chatID,
			Status:  storage.DeliveryStatusPending,
		}

		// 创建投递记录（幂等）
		if err := s.storage.CreateDelivery(ctx, delivery); err != nil {
			slog.Warn("创建投递记录失败", "event_id", event.ID, "chat_id", chatID, "error", err)
			continue
		}

		// 异步发送
		go s.sendNotification(ctx, delivery, event)
	}

	return nil
}

// sendNotification 发送单条通知
func (s *Sender) sendNotification(ctx context.Context, delivery *storage.Delivery, event *poller.Event) {
	// 等待限流
	select {
	case <-ctx.Done():
		return
	case <-s.rateLimiter.C:
	}

	// 构建消息
	msg := s.formatMessage(event)

	// 发送消息
	result, err := s.tgClient.SendMessageHTML(ctx, delivery.ChatID, msg)
	if err != nil {
		slog.Warn("发送通知失败",
			"delivery_id", delivery.ID,
			"chat_id", delivery.ChatID,
			"error", err,
		)

		// 检查是否被封禁
		if telegram.IsForbiddenError(err) {
			// 标记用户为 blocked
			if err := s.storage.UpdateChatStatus(ctx, delivery.ChatID, storage.ChatStatusBlocked); err != nil {
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
		if err := s.storage.UpdateDeliveryStatus(ctx, delivery.ID, storage.DeliveryStatusPending, "", err.Error()); err != nil {
			slog.Error("更新投递状态失败", "error", err)
		}

		return
	}

	// 发送成功
	messageID := fmt.Sprintf("%d", result.MessageID)
	if err := s.storage.UpdateDeliveryStatus(ctx, delivery.ID, storage.DeliveryStatusSent, messageID, ""); err != nil {
		slog.Error("更新投递状态失败", "error", err)
	}
}

// formatMessage 格式化通知消息
func (s *Sender) formatMessage(event *poller.Event) string {
	var emoji string
	var statusText string

	switch event.NewStatus {
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
		statusText = "状态未知"
	}

	// 转义 HTML 防止注入
	provider := html.EscapeString(event.Provider)
	service := html.EscapeString(event.Service)
	channel := html.EscapeString(event.Channel)

	location := fmt.Sprintf("<b>%s</b> / <b>%s</b>", provider, service)
	if channel != "" {
		location += fmt.Sprintf(" / <b>%s</b>", channel)
	}

	msg := fmt.Sprintf(`%s <b>%s</b>

%s

延迟: %dms → %dms
时间: %s`,
		emoji, statusText,
		location,
		event.OldLatency, event.NewLatency,
		time.Unix(event.Timestamp, 0).Format("2006-01-02 15:04:05 MST"),
	)

	return msg
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
		// 注意：这里没有事件详情，只能发送简单的通知
		// 在实际场景中，可能需要在 deliveries 表中存储事件内容
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

	// 简单的重试消息（因为我们没有存储原始事件内容）
	msg := fmt.Sprintf("🔔 通知重试 (event_id: %d)\n\n如果您持续收到此消息，请检查订阅设置。", delivery.EventID)

	result, err := s.tgClient.SendMessageHTML(ctx, delivery.ChatID, msg)
	if err != nil {
		slog.Warn("重试投递失败",
			"delivery_id", delivery.ID,
			"chat_id", delivery.ChatID,
			"retry_count", delivery.RetryCount,
			"error", err,
		)

		if telegram.IsForbiddenError(err) {
			if err := s.storage.UpdateChatStatus(ctx, delivery.ChatID, storage.ChatStatusBlocked); err != nil {
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

	messageID := fmt.Sprintf("%d", result.MessageID)
	if err := s.storage.UpdateDeliveryStatus(ctx, delivery.ID, storage.DeliveryStatusSent, messageID, ""); err != nil {
		slog.Error("更新投递状态失败", "error", err)
	}
}
