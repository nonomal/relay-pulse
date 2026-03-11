package notifier

import (
	"context"

	"notifier/internal/poller"
	"notifier/internal/storage"
)

// ChannelSender 定义平台渠道的发送能力。
// 每个渠道（Telegram/QQ/未来新增）实现此接口后注册到 Sender 即可。
type ChannelSender interface {
	// Platform 返回渠道标识，与 storage.PlatformXxx 常量一致。
	Platform() string

	// FormatMessage 将事件格式化为该渠道的消息文本。
	FormatMessage(event *poller.Event) string

	// Send 向指定 chatID 发送消息，返回平台侧消息 ID。
	Send(ctx context.Context, chatID int64, message string) (messageID string, err error)

	// HandleSendError 对发送错误做平台特定处理（如 Telegram 的 blocked 检测）。
	// 返回 true 表示已处理（终态），false 表示应走通用重试。
	HandleSendError(ctx context.Context, delivery *storage.Delivery, err error) (handled bool)
}
