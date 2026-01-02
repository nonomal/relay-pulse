package qq

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"notifier/internal/storage"
)

// Bot QQ 命令处理器（OneBot v11 / NapCatQQ）
type Bot struct {
	client  *Client
	storage storage.Storage

	maxSubscriptionsPerUser int
	eventsURL               string
	callbackSecret          string // Webhook 签名密钥

	handlers map[string]commandHandler

	// selfID 机器人 QQ 号（从消息事件中获取）
	selfID   int64
	selfIDMu sync.RWMutex
}

type commandHandler func(ctx context.Context, e *OneBotEvent, args string) error

// Options QQ Bot 初始化选项
type Options struct {
	MaxSubscriptionsPerUser int
	EventsURL               string
	CallbackSecret          string // Webhook 签名密钥（可选）
}

// NewBot 创建 QQ Bot
func NewBot(client *Client, store storage.Storage, opts Options) *Bot {
	b := &Bot{
		client:                  client,
		storage:                 store,
		maxSubscriptionsPerUser: opts.MaxSubscriptionsPerUser,
		eventsURL:               opts.EventsURL,
		callbackSecret:          opts.CallbackSecret,
		handlers:                make(map[string]commandHandler),
	}

	// 注册命令处理器
	b.handlers["list"] = b.handleList
	b.handlers["add"] = b.handleAdd
	b.handlers["remove"] = b.handleRemove
	b.handlers["clear"] = b.handleClear
	b.handlers["status"] = b.handleStatus
	b.handlers["help"] = b.handleHelp

	return b
}

// HandleCallback HTTP 回调处理（接收 OneBot 上报），快速 ACK，异步处理命令
func (b *Bot) HandleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// 限制请求体大小（1MB）
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		b.writeOK(w)
		return
	}

	// 签名校验（如果配置了 CallbackSecret）
	if b.callbackSecret != "" {
		signature := r.Header.Get("X-Signature")
		if signature == "" {
			slog.Warn("OneBot 回调缺少签名", "remote_addr", r.RemoteAddr)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if !b.verifySignature(body, signature) {
			slog.Warn("OneBot 回调签名校验失败", "remote_addr", r.RemoteAddr)
			w.WriteHeader(http.StatusForbidden)
			return
		}
	}

	var event OneBotEvent
	if err := json.Unmarshal(body, &event); err != nil {
		slog.Warn("OneBot 回调解析失败", "error", err)
		b.writeOK(w)
		return
	}

	// 快速响应，避免阻塞 NapCatQQ
	b.writeOK(w)

	// 异步处理消息
	go func(ev OneBotEvent) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		b.handleMessage(ctx, &ev)
	}(event)
}

// verifySignature 校验 HMAC-SHA1 签名
func (b *Bot) verifySignature(body []byte, signature string) bool {
	// 签名格式：sha1=<hex>
	if !strings.HasPrefix(signature, "sha1=") {
		return false
	}
	expectedMAC := signature[5:]

	mac := hmac.New(sha1.New, []byte(b.callbackSecret))
	mac.Write(body)
	actualMAC := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expectedMAC), []byte(actualMAC))
}

// writeOK 快速返回成功响应
func (b *Bot) writeOK(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// getSelfID 获取机器人 QQ 号
func (b *Bot) getSelfID() int64 {
	b.selfIDMu.RLock()
	defer b.selfIDMu.RUnlock()
	return b.selfID
}

// setSelfID 设置机器人 QQ 号（仅首次设置生效）
func (b *Bot) setSelfID(id int64) {
	if id == 0 {
		return
	}
	b.selfIDMu.Lock()
	defer b.selfIDMu.Unlock()
	if b.selfID == 0 {
		b.selfID = id
		slog.Info("QQ Bot SelfID 已记录", "self_id", id)
	}
}

// isMessageToBot 检查消息是否指向机器人
// 私聊直接返回 true；群聊检查是否 @了机器人
func (b *Bot) isMessageToBot(e *OneBotEvent) bool {
	if e == nil {
		return false
	}

	// 私聊直接返回 true
	if e.MessageType == "private" {
		return true
	}

	// 群聊需要检查是否 @了机器人
	selfID := b.getSelfID()
	if selfID == 0 {
		// 尚未获取到 SelfID，暂时允许（首次消息可能是群消息）
		slog.Debug("群消息但尚未获取 SelfID，暂时允许响应", "group_id", e.GroupID)
		return true
	}

	// 尝试从消息段数组中检测 @
	if len(e.Message) > 0 {
		var segs []MessageSegment
		if err := json.Unmarshal(e.Message, &segs); err == nil {
			for _, seg := range segs {
				if seg.Type == "at" && seg.Data.QQ != "" {
					if qq, _ := strconv.ParseInt(seg.Data.QQ, 10, 64); qq == selfID {
						return true
					}
				}
			}
			// 成功解析消息段但未找到 @机器人
			return false
		}
		// 消息可能是字符串格式，继续尝试 RawMessage
	}

	// 兜底：检查 RawMessage 中的 CQ 码（格式：[CQ:at,qq=123456]）
	if e.RawMessage != "" {
		// 构造目标 CQ 码
		target := fmt.Sprintf("[CQ:at,qq=%d]", selfID)
		if strings.Contains(e.RawMessage, target) {
			return true
		}
	}

	return false
}

// handleMessage 处理消息
func (b *Bot) handleMessage(ctx context.Context, e *OneBotEvent) {
	if e == nil || e.PostType != "message" {
		return
	}

	// 记录机器人 QQ 号（从消息事件中获取）
	if e.SelfID != 0 {
		b.setSelfID(e.SelfID)
	}

	// 忽略自己发的消息
	if e.UserID != 0 && e.UserID == e.SelfID {
		return
	}

	// 私聊权限检查：仅接受好友消息（好友即白名单）
	if e.MessageType == "private" && e.SubType != "friend" {
		slog.Debug("忽略非好友私聊消息", "user_id", e.UserID, "sub_type", e.SubType)
		return
	}

	// 群聊必须 @机器人 才响应
	if e.MessageType == "group" && !b.isMessageToBot(e) {
		return
	}

	// 提取纯文本
	text := extractPlainText(e)
	if text == "" || !strings.HasPrefix(text, "/") {
		return
	}

	// 解析命令
	cmd, args := parseCommand(text)
	if cmd == "" {
		return
	}

	// 获取 chatID
	chatID, ok := chatKey(e)
	if !ok {
		return
	}

	// 确保用户记录存在
	if err := b.ensureChat(ctx, e, chatID); err != nil {
		slog.Warn("确保用户记录失败", "chat_id", chatID, "error", err)
	}

	// 更新命令时间
	if err := b.storage.UpdateChatCommandTime(ctx, storage.PlatformQQ, chatID); err != nil {
		slog.Warn("更新命令时间失败", "chat_id", chatID, "error", err)
	}

	// 查找命令处理器
	handler, found := b.handlers[cmd]
	if !found {
		b.sendReply(ctx, e, "未知命令。发送 /help 查看帮助。")
		return
	}

	// 群消息权限检查
	if e.MessageType == "group" && isAdminOnlyCommand(cmd) {
		isAdmin, err := b.isGroupAdmin(ctx, e.GroupID, e.UserID)
		if err != nil {
			slog.Warn("群管理员校验失败", "group_id", e.GroupID, "user_id", e.UserID, "error", err)
			b.sendReply(ctx, e, "权限校验失败，请稍后重试。")
			return
		}
		if !isAdmin {
			b.sendReply(ctx, e, "权限不足：群聊中仅管理员可执行 /add /remove /clear。")
			return
		}
	}

	// 执行命令
	if err := handler(ctx, e, args); err != nil {
		slog.Error("QQ 命令执行失败", "command", cmd, "chat_id", chatID, "error", err)
		b.sendReply(ctx, e, "命令执行出错，请稍后重试。")
	}
}

// isAdminOnlyCommand 判断是否是仅管理员可用的命令
func isAdminOnlyCommand(cmd string) bool {
	switch cmd {
	case "add", "remove", "clear":
		return true
	default:
		return false
	}
}

// isGroupAdmin 检查用户是否是群管理员（二次确认）
func (b *Bot) isGroupAdmin(ctx context.Context, groupID, userID int64) (bool, error) {
	if groupID == 0 || userID == 0 {
		return false, nil
	}

	member, err := b.client.GetGroupMemberInfo(ctx, groupID, userID)
	if err != nil {
		return false, err
	}

	switch member.Role {
	case "owner", "admin":
		return true, nil
	default:
		return false, nil
	}
}

// ensureChat 确保用户/群记录存在
func (b *Bot) ensureChat(ctx context.Context, e *OneBotEvent, chatID int64) error {
	username := ""
	firstName := "qq"

	if e != nil && e.Sender != nil {
		if e.Sender.Card != "" {
			username = e.Sender.Card
		} else {
			username = e.Sender.Nickname
		}
	}

	if e != nil && e.MessageType == "group" {
		firstName = fmt.Sprintf("qq_group:%d", e.GroupID)
	}

	return b.storage.UpsertChat(ctx, &storage.Chat{
		Platform:  storage.PlatformQQ,
		ChatID:    chatID,
		Username:  username,
		FirstName: firstName,
	})
}

// sendReply 发送回复
func (b *Bot) sendReply(ctx context.Context, e *OneBotEvent, text string) {
	if e == nil {
		return
	}

	switch e.MessageType {
	case "group":
		if e.GroupID == 0 {
			return
		}
		_, err := b.client.SendGroupMessage(ctx, e.GroupID, text)
		if err != nil {
			slog.Error("发送群消息失败", "group_id", e.GroupID, "error", err)
		}
	case "private":
		if e.UserID == 0 {
			return
		}
		_, err := b.client.SendPrivateMessage(ctx, e.UserID, text)
		if err != nil {
			slog.Error("发送私聊消息失败", "user_id", e.UserID, "error", err)
		}
	}
}

// handleList 处理 /list 命令
func (b *Bot) handleList(ctx context.Context, e *OneBotEvent, args string) error {
	chatID, ok := chatKey(e)
	if !ok {
		return nil
	}

	subs, err := b.storage.GetSubscriptionsByChatID(ctx, storage.PlatformQQ, chatID)
	if err != nil {
		return err
	}

	if len(subs) == 0 {
		b.sendReply(ctx, e, "你还没有订阅任何服务。\n\n使用 /add <provider> <service> [channel] 添加订阅。")
		return nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("当前订阅（%d 个）：\n\n", len(subs)))

	for i, sub := range subs {
		if sub.Channel != "" {
			sb.WriteString(fmt.Sprintf("%d. %s / %s / %s\n", i+1, sub.Provider, sub.Service, sub.Channel))
		} else {
			sb.WriteString(fmt.Sprintf("%d. %s / %s\n", i+1, sub.Provider, sub.Service))
		}
	}

	sb.WriteString("\n使用 /remove <provider> <service> [channel] 移除订阅。")

	b.sendReply(ctx, e, sb.String())
	return nil
}

// handleAdd 处理 /add 命令
func (b *Bot) handleAdd(ctx context.Context, e *OneBotEvent, args string) error {
	chatID, ok := chatKey(e)
	if !ok {
		return nil
	}

	parts := strings.Fields(args)
	if len(parts) < 2 {
		b.sendReply(ctx, e, "用法: /add <provider> <service> [channel]\n例如: /add 88code cc")
		return nil
	}

	provider := parts[0]
	service := parts[1]
	channel := ""
	if len(parts) > 2 {
		channel = parts[2]
	}

	// 检查订阅数量限制
	if b.maxSubscriptionsPerUser > 0 {
		count, err := b.storage.CountSubscriptions(ctx, storage.PlatformQQ, chatID)
		if err != nil {
			return err
		}
		if count >= b.maxSubscriptionsPerUser {
			b.sendReply(ctx, e, fmt.Sprintf("订阅数量已达上限（%d/%d）。请先移除部分订阅。", count, b.maxSubscriptionsPerUser))
			return nil
		}
	}

	// 添加订阅
	if err := b.storage.AddSubscription(ctx, &storage.Subscription{
		Platform: storage.PlatformQQ,
		ChatID:   chatID,
		Provider: provider,
		Service:  service,
		Channel:  channel,
	}); err != nil {
		return err
	}

	if channel != "" {
		b.sendReply(ctx, e, fmt.Sprintf("已订阅 %s / %s / %s", provider, service, channel))
	} else {
		b.sendReply(ctx, e, fmt.Sprintf("已订阅 %s / %s", provider, service))
	}
	return nil
}

// handleRemove 处理 /remove 命令
func (b *Bot) handleRemove(ctx context.Context, e *OneBotEvent, args string) error {
	chatID, ok := chatKey(e)
	if !ok {
		return nil
	}

	parts := strings.Fields(args)
	if len(parts) < 2 {
		b.sendReply(ctx, e, "用法: /remove <provider> <service> [channel]\n例如: /remove 88code cc")
		return nil
	}

	provider := parts[0]
	service := parts[1]
	channel := ""
	if len(parts) > 2 {
		channel = parts[2]
	}

	if err := b.storage.RemoveSubscription(ctx, storage.PlatformQQ, chatID, provider, service, channel); err != nil {
		return err
	}

	if channel != "" {
		b.sendReply(ctx, e, fmt.Sprintf("已取消订阅 %s / %s / %s", provider, service, channel))
	} else {
		b.sendReply(ctx, e, fmt.Sprintf("已取消订阅 %s / %s", provider, service))
	}
	return nil
}

// handleClear 处理 /clear 命令
func (b *Bot) handleClear(ctx context.Context, e *OneBotEvent, args string) error {
	chatID, ok := chatKey(e)
	if !ok {
		return nil
	}

	if err := b.storage.ClearSubscriptions(ctx, storage.PlatformQQ, chatID); err != nil {
		return err
	}

	b.sendReply(ctx, e, "已清空所有订阅。")
	return nil
}

// handleStatus 处理 /status 命令
func (b *Bot) handleStatus(ctx context.Context, e *OneBotEvent, args string) error {
	chatID, ok := chatKey(e)
	if !ok {
		return nil
	}

	count, err := b.storage.CountSubscriptions(ctx, storage.PlatformQQ, chatID)
	if err != nil {
		return err
	}

	max := b.maxSubscriptionsPerUser
	msg := "服务状态\n\n"

	if max > 0 {
		msg += fmt.Sprintf("订阅数量: %d/%d\n", count, max)
	} else {
		msg += fmt.Sprintf("订阅数量: %d\n", count)
	}

	msg += "状态: 运行中\n"

	if b.eventsURL != "" {
		msg += fmt.Sprintf("数据源: %s\n", b.eventsURL)
	}

	b.sendReply(ctx, e, strings.TrimSpace(msg))
	return nil
}

// handleHelp 处理 /help 命令
func (b *Bot) handleHelp(ctx context.Context, e *OneBotEvent, args string) error {
	help := `RelayPulse QQ 通知帮助

命令列表：
/list - 查看当前订阅
/add <provider> <service> [channel] - 添加订阅
/remove <provider> <service> [channel] - 移除订阅
/clear - 清空所有订阅
/status - 查看服务状态
/help - 显示此帮助

权限说明：
1) 群聊：仅管理员可执行 /add /remove /clear
2) 私聊：好友可直接使用所有命令`

	b.sendReply(ctx, e, help)
	return nil
}

// parseCommand 解析命令和参数
func parseCommand(text string) (cmd string, args string) {
	parts := strings.SplitN(strings.TrimSpace(text), " ", 2)
	cmd = strings.TrimPrefix(parts[0], "/")
	cmd = strings.TrimSpace(cmd)

	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}

	return cmd, args
}

// chatKey 获取聊天唯一标识
// 群聊使用负的 GroupID，私聊使用正的 UserID
func chatKey(e *OneBotEvent) (int64, bool) {
	if e == nil {
		return 0, false
	}

	switch e.MessageType {
	case "private":
		if e.UserID == 0 {
			return 0, false
		}
		return e.UserID, true
	case "group":
		if e.GroupID == 0 {
			return 0, false
		}
		// 使用负数区分群聊，避免与私聊 UserID 冲突
		return -e.GroupID, true
	default:
		return 0, false
	}
}

// extractPlainText 从消息中提取纯文本
// 优先从 message segments 提取，raw_message 仅作为兜底（可能包含 CQ 码）
func extractPlainText(e *OneBotEvent) string {
	if e == nil {
		return ""
	}

	// 优先尝试从 message 字段提取（更安全，避免 CQ 码注入）
	if len(e.Message) > 0 {
		// 尝试解析为字符串
		var s string
		if err := json.Unmarshal(e.Message, &s); err == nil {
			return strings.TrimSpace(s)
		}

		// 尝试解析为消息段数组
		var segs []MessageSegment
		if err := json.Unmarshal(e.Message, &segs); err == nil && len(segs) > 0 {
			var sb strings.Builder
			for _, seg := range segs {
				if seg.Type == "text" && seg.Data.Text != "" {
					sb.WriteString(seg.Data.Text)
				}
			}
			if text := strings.TrimSpace(sb.String()); text != "" {
				return text
			}
		}
	}

	// 兜底：使用 raw_message（注意可能包含 CQ 码）
	if e.RawMessage != "" {
		return strings.TrimSpace(e.RawMessage)
	}

	return ""
}
