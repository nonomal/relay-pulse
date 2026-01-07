package qq

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"notifier/internal/screenshot"
	"notifier/internal/storage"
	"notifier/internal/validator"
)

// 全局指令关键词
const statusCheckKeyword = "状态检查"

// Bot QQ 命令处理器（OneBot v11 / NapCatQQ）
type Bot struct {
	client            *Client
	storage           storage.Storage
	screenshotService *screenshot.Service
	validator         *validator.RelayPulseValidator

	maxSubscriptionsPerUser int
	eventsURL               string
	callbackSecret          string             // Webhook 签名密钥
	adminWhitelist          map[int64]struct{} // 管理员白名单（可越权执行管理命令）

	handlers map[string]commandHandler

	// 群聊"状态检查"防刷（同一群短时间内不重复响应）
	statusCheckCooldown    time.Duration
	lastStatusCheckByGroup map[int64]time.Time
	statusCheckMu          sync.Mutex

	// selfID 机器人 QQ 号（从消息事件中获取）
	selfID   int64
	selfIDMu sync.RWMutex
}

type commandHandler func(ctx context.Context, e *OneBotEvent, args string) error

// Options QQ Bot 初始化选项
type Options struct {
	MaxSubscriptionsPerUser int
	EventsURL               string
	CallbackSecret          string              // Webhook 签名密钥（可选）
	ScreenshotService       *screenshot.Service // 截图服务（可选）
	AdminWhitelist          []int64             // 管理员白名单 QQ 号（可越权执行管理命令，可选）
}

// NewBot 创建 QQ Bot
func NewBot(client *Client, store storage.Storage, opts Options) *Bot {
	// 初始化管理员白名单（转换为 map 实现 O(1) 查找）
	adminWhitelist := make(map[int64]struct{}, len(opts.AdminWhitelist))
	for _, id := range opts.AdminWhitelist {
		if id <= 0 {
			continue
		}
		adminWhitelist[id] = struct{}{}
	}

	// 初始化订阅验证器
	var v *validator.RelayPulseValidator
	if opts.EventsURL != "" {
		var err error
		v, err = validator.NewRelayPulseValidator(opts.EventsURL)
		if err != nil {
			slog.Warn("订阅验证器初始化失败", "error", err)
			v = nil
		}
	}

	b := &Bot{
		client:                  client,
		storage:                 store,
		screenshotService:       opts.ScreenshotService,
		validator:               v,
		maxSubscriptionsPerUser: opts.MaxSubscriptionsPerUser,
		eventsURL:               opts.EventsURL,
		callbackSecret:          opts.CallbackSecret,
		adminWhitelist:          adminWhitelist,
		handlers:                make(map[string]commandHandler),
		statusCheckCooldown:     30 * time.Second,
		lastStatusCheckByGroup:  make(map[int64]time.Time),
	}

	// 注册命令处理器
	b.handlers["list"] = b.handleList
	b.handlers["add"] = b.handleAdd
	b.handlers["remove"] = b.handleRemove
	b.handlers["clear"] = b.handleClear
	b.handlers["status"] = b.handleStatus
	b.handlers["help"] = b.handleHelp
	b.handlers["snap"] = b.handleSnap

	return b
}

// isWhitelisted 检查用户是否在管理员白名单中
func (b *Bot) isWhitelisted(userID int64) bool {
	if userID <= 0 {
		return false
	}
	_, ok := b.adminWhitelist[userID]
	return ok
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

// allowGroupStatusCheck 检查群聊状态检查是否允许（防刷）
func (b *Bot) allowGroupStatusCheck(groupID int64) bool {
	if groupID == 0 {
		return true
	}

	b.statusCheckMu.Lock()
	defer b.statusCheckMu.Unlock()

	now := time.Now()
	last, ok := b.lastStatusCheckByGroup[groupID]
	if ok && now.Sub(last) < b.statusCheckCooldown {
		return false
	}
	b.lastStatusCheckByGroup[groupID] = now
	return true
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

	// 提取纯文本
	text := strings.TrimSpace(extractPlainText(e))

	// 检查是否是机器人自己发的消息
	isSelfMessage := e.UserID != 0 && e.UserID == e.SelfID
	if isSelfMessage {
		// 仅允许 // 双斜杠命令（隐藏的自发命令入口，避免循环）
		if !strings.HasPrefix(text, "//") {
			return
		}
		// 将 // 转换为 / 以便后续统一处理
		text = text[1:]
	}

	// 私聊权限检查：仅接受好友消息（好友即白名单）
	if e.MessageType == "private" && e.SubType != "friend" && !isSelfMessage {
		slog.Debug("忽略非好友私聊消息", "user_id", e.UserID, "sub_type", e.SubType)
		return
	}

	// 群聊"状态检查"关键词：无需 @、无需 /，直接触发截图
	if e.MessageType == "group" && text == statusCheckKeyword {
		if !b.allowGroupStatusCheck(e.GroupID) {
			slog.Debug("群聊状态检查触发过于频繁，已忽略", "group_id", e.GroupID)
			return
		}

		chatID, ok := chatKey(e)
		if !ok {
			return
		}

		if err := b.ensureChat(ctx, e, chatID); err != nil {
			slog.Warn("确保用户记录失败", "chat_id", chatID, "error", err)
		}
		if err := b.storage.UpdateChatCommandTime(ctx, storage.PlatformQQ, chatID); err != nil {
			slog.Warn("更新命令时间失败", "chat_id", chatID, "error", err)
		}

		slog.Info("群聊触发状态检查", "group_id", e.GroupID, "user_id", e.UserID)
		if err := b.handleSnap(ctx, e, ""); err != nil {
			slog.Error("状态检查截图失败", "chat_id", chatID, "error", err)
			b.sendReply(ctx, e, "状态检查失败，请稍后重试。")
		}
		return
	}

	// 其他群聊命令必须 @机器人 才响应（自发命令跳过此检查）
	if e.MessageType == "group" && !isSelfMessage && !b.isMessageToBot(e) {
		return
	}

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

	// 群消息权限检查：管理命令需群管理员；白名单/自发命令可越权
	if e.MessageType == "group" && isAdminOnlyCommand(cmd) {
		if isSelfMessage {
			// 审计日志：机器人自发命令执行管理命令
			slog.Info("机器人自发命令执行管理命令", "command", cmd, "group_id", e.GroupID)
		} else if b.isWhitelisted(e.UserID) {
			// 审计日志：白名单越权执行管理命令
			slog.Info("管理员白名单越权执行管理命令", "command", cmd, "group_id", e.GroupID, "user_id", e.UserID)
		} else {
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
		b.sendReply(ctx, e, "你还没有订阅任何服务。\n\n使用 /add <provider> [service] 添加订阅。")
		return nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("当前订阅（%d 个）：\n\n", len(subs)))

	for i, sub := range subs {
		// 根据订阅级别显示不同格式
		if sub.Service == "" {
			// 旧版通配订阅（provider 级）
			sb.WriteString(fmt.Sprintf("%d. %s / *（旧版）\n", i+1, sub.Provider))
		} else if sub.Channel != "" {
			// 精确订阅（provider / service / channel）
			sb.WriteString(fmt.Sprintf("%d. %s / %s / %s\n", i+1, sub.Provider, sub.Service, sub.Channel))
		} else {
			// service 级订阅（provider / service）
			sb.WriteString(fmt.Sprintf("%d. %s / %s\n", i+1, sub.Provider, sub.Service))
		}
	}

	sb.WriteString("\n使用 /remove <provider> [service] [channel] 移除订阅。")

	b.sendReply(ctx, e, sb.String())
	return nil
}

// handleAdd 处理 /add 命令
// 支持三种订阅模式：
// - /add <provider> → 展开订阅该 provider 下所有 service/channel
// - /add <provider> <service> → 展开订阅该 service 下所有 channel
// - /add <provider> <service> <channel> → 精确订阅
func (b *Bot) handleAdd(ctx context.Context, e *OneBotEvent, args string) error {
	chatID, ok := chatKey(e)
	if !ok {
		return nil
	}

	parts := strings.Fields(args)
	if len(parts) < 1 {
		b.sendReply(ctx, e, "用法: /add <provider> [service] [channel]\n\n例如:\n/add 88code → 订阅 88code 所有服务\n/add 88code cc → 订阅 88code 的 cc 服务")
		return nil
	}

	provider := parts[0]
	service := ""
	if len(parts) > 1 {
		service = parts[1]
	}
	channel := ""
	if len(parts) > 2 {
		channel = parts[2]
	}

	// 先检查订阅数量
	count, err := b.storage.CountSubscriptions(ctx, storage.PlatformQQ, chatID)
	if err != nil {
		return err
	}
	maxSubs := b.maxSubscriptionsPerUser

	// 验证服务是否配置
	if b.validator == nil {
		b.sendReply(ctx, e, "当前无法验证订阅（验证服务未配置），为避免订阅无效服务，已拒绝本次订阅。请从网页一键导入收藏。")
		return nil
	}

	// Provider 级订阅：展开为所有 service/channel
	if service == "" {
		targets, err := b.validator.ValidateAndExpandProvider(ctx, provider)
		if err != nil {
			return b.handleAddError(ctx, e, err, provider, "", "")
		}

		// 检查配额
		if maxSubs > 0 && count+len(targets) > maxSubs {
			b.sendReply(ctx, e, fmt.Sprintf(
				"订阅配额不足。%s 有 %d 个订阅项，当前已用 %d/%d。\n\n请先移除部分订阅，或精确指定 service/channel。",
				provider, len(targets), count, maxSubs,
			))
			return nil
		}

		// 逐一添加
		added := 0
		for _, t := range targets {
			sub := &storage.Subscription{
				Platform: storage.PlatformQQ,
				ChatID:   chatID,
				Provider: t.Provider,
				Service:  t.Service,
				Channel:  t.Channel,
			}
			if err := b.storage.AddSubscription(ctx, sub); err == nil {
				added++
			}
		}

		b.sendReply(ctx, e, fmt.Sprintf(
			"已添加 %d 个订阅（%s 下所有服务通道）",
			added, provider,
		))
		return nil
	}

	// Service 级订阅：展开为所有 channel
	if channel == "" {
		targets, err := b.validator.ValidateAndExpandService(ctx, provider, service)
		if err != nil {
			return b.handleAddError(ctx, e, err, provider, service, "")
		}

		// 检查配额
		if maxSubs > 0 && count+len(targets) > maxSubs {
			b.sendReply(ctx, e, fmt.Sprintf(
				"订阅配额不足。%s / %s 有 %d 个订阅项，当前已用 %d/%d。\n\n请先移除部分订阅，或精确指定 channel。",
				provider, service, len(targets), count, maxSubs,
			))
			return nil
		}

		// 逐一添加
		added := 0
		for _, t := range targets {
			sub := &storage.Subscription{
				Platform: storage.PlatformQQ,
				ChatID:   chatID,
				Provider: t.Provider,
				Service:  t.Service,
				Channel:  t.Channel,
			}
			if err := b.storage.AddSubscription(ctx, sub); err == nil {
				added++
			}
		}

		b.sendReply(ctx, e, fmt.Sprintf(
			"已添加 %d 个订阅（%s / %s 下所有通道）",
			added, provider, service,
		))
		return nil
	}

	// 精确订阅
	if maxSubs > 0 && count >= maxSubs {
		b.sendReply(ctx, e, fmt.Sprintf(
			"订阅数量已达上限（%d/%d）。请先移除部分订阅。",
			count, maxSubs,
		))
		return nil
	}

	target, err := b.validator.ValidateAdd(ctx, provider, service, channel)
	if err != nil {
		return b.handleAddError(ctx, e, err, provider, service, channel)
	}

	sub := &storage.Subscription{
		Platform: storage.PlatformQQ,
		ChatID:   chatID,
		Provider: target.Provider,
		Service:  target.Service,
		Channel:  target.Channel,
	}

	if err := b.storage.AddSubscription(ctx, sub); err != nil {
		return err
	}

	b.sendReply(ctx, e, fmt.Sprintf(
		"已订阅 %s / %s / %s",
		target.Provider, target.Service, target.Channel,
	))
	return nil
}

// handleAddError 处理添加订阅时的错误
func (b *Bot) handleAddError(ctx context.Context, e *OneBotEvent, err error, provider, service, channel string) error {
	var nf *validator.NotFoundError
	if errors.As(err, &nf) {
		switch nf.Level {
		case validator.NotFoundProvider:
			b.sendReply(ctx, e, fmt.Sprintf(
				"未找到服务商 %s。\n\n请到 RelayPulse 网页复制正确的 provider/service/channel，或使用网页一键导入收藏。",
				provider,
			))
		case validator.NotFoundService:
			cands := validator.FormatCandidates(nf.Candidates, "service")
			if cands != "" {
				b.sendReply(ctx, e, fmt.Sprintf(
					"未找到 %s / %s。\n\n该服务商下可用的 service 例如：%s（仅显示前 %d 个）。",
					provider, service, cands, 8,
				))
			} else {
				b.sendReply(ctx, e, fmt.Sprintf(
					"未找到 %s / %s。\n\n请到 RelayPulse 网页确认 service 是否正确，或使用网页一键导入收藏。",
					provider, service,
				))
			}
		case validator.NotFoundChannel:
			cands := validator.FormatCandidates(nf.Candidates, "channel")
			if cands != "" {
				b.sendReply(ctx, e, fmt.Sprintf(
					"未找到 %s / %s / %s。\n\n该 service 下可用的 channel 例如：%s（仅显示前 %d 个）。",
					provider, service, channel, cands, 8,
				))
			} else {
				b.sendReply(ctx, e, fmt.Sprintf(
					"未找到 %s / %s / %s。",
					provider, service, channel,
				))
			}
		}
		return nil
	}

	var ue *validator.UnavailableError
	if errors.As(err, &ue) {
		b.sendReply(ctx, e, "当前无法验证订阅（状态服务暂不可用），为避免订阅无效服务，已拒绝本次订阅。请稍后再试，或从网页一键导入收藏。")
		return nil
	}

	return err
}

// handleRemove 处理 /remove 命令
// 支持三种移除模式：
// - /remove <provider> → 移除该 provider 下所有订阅（级联删除）
// - /remove <provider> <service> → 移除该 service 下所有通道
// - /remove <provider> <service> <channel> → 精确移除
func (b *Bot) handleRemove(ctx context.Context, e *OneBotEvent, args string) error {
	chatID, ok := chatKey(e)
	if !ok {
		return nil
	}

	parts := strings.Fields(args)
	if len(parts) < 1 {
		b.sendReply(ctx, e, "用法: /remove <provider> [service] [channel]\n\n例如:\n/remove 88code → 移除 88code 所有订阅\n/remove 88code cc → 移除 88code 的 cc 服务订阅")
		return nil
	}

	provider := parts[0]
	service := ""
	if len(parts) > 1 {
		service = parts[1]
	}
	channel := ""
	if len(parts) > 2 {
		channel = parts[2]
	}

	// "default" 归一化为空字符串（与 add 对齐）
	if strings.EqualFold(channel, "default") {
		channel = ""
	}

	if err := b.storage.RemoveSubscription(ctx, storage.PlatformQQ, chatID, provider, service, channel); err != nil {
		return err
	}

	// 根据删除级别显示不同消息
	if service == "" {
		// provider 级删除（级联）
		b.sendReply(ctx, e, fmt.Sprintf("已取消订阅 %s / *（包括该服务商下所有订阅）", provider))
	} else if channel != "" {
		// 精确删除
		b.sendReply(ctx, e, fmt.Sprintf("已取消订阅 %s / %s / %s", provider, service, channel))
	} else {
		// service 级删除
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
/add <provider> [service] [channel] - 添加订阅
/remove <provider> [service] [channel] - 移除订阅
/clear - 清空所有订阅
/snap - 截图订阅服务状态
/status - 查看服务状态
/help - 显示此帮助

手动添加订阅：
/add 88code → 订阅 88code 所有服务
/add 88code cc → 订阅 88code 的 cc 服务
/add duckcoding cc v1 → 精确订阅

移除订阅：
/remove 88code → 移除 88code 所有订阅
/remove 88code cc → 移除 88code 的 cc 订阅

全局指令（群聊无需@）：
状态检查 - 快速截图订阅服务状态

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

// handleSnap 处理 /snap 命令（截图订阅服务状态）
func (b *Bot) handleSnap(ctx context.Context, e *OneBotEvent, args string) error {
	chatID, ok := chatKey(e)
	if !ok {
		return nil
	}

	// 检查截图服务是否启用
	if b.screenshotService == nil {
		b.sendReply(ctx, e, "截图功能未启用。")
		return nil
	}

	// 获取订阅列表
	subs, err := b.storage.GetSubscriptionsByChatID(ctx, storage.PlatformQQ, chatID)
	if err != nil {
		slog.Error("获取订阅失败", "chat_id", chatID, "error", err)
		b.sendReply(ctx, e, "获取订阅信息失败，请稍后重试。")
		return nil
	}
	if len(subs) == 0 {
		b.sendReply(ctx, e, "你还没有订阅任何服务。\n\n使用 /add <provider> <service> 添加订阅后再试。")
		return nil
	}

	// 提取 provider 列表（去重）
	providers := extractUniqueProviders(subs)

	// 提取 service 列表（去重）
	services := extractUniqueServices(subs)

	// 构建专属标识（群名/昵称）
	ownerLabel := b.getOwnerLabel(ctx, e)

	// 发送提示
	b.sendReply(ctx, e, fmt.Sprintf("正在生成 [%s专属] 的状态截图...", ownerLabel))

	// 截图（使用独立的超时 ctx）
	snapCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 构建截图标题（群名 + 专属状态）
	title := ownerLabel + " 专属状态"
	pngData, err := b.screenshotService.CaptureWithOptions(snapCtx, providers, services, &screenshot.CaptureOptions{
		Title: title,
	})
	if err != nil {
		slog.Error("截图失败", "chat_id", chatID, "providers", providers, "error", err)
		// 区分错误类型
		if errors.Is(err, context.DeadlineExceeded) {
			b.sendReply(ctx, e, "截图超时，请稍后重试。")
		} else if errors.Is(err, screenshot.ErrConcurrencyLimit) {
			b.sendReply(ctx, e, "系统繁忙，请稍后重试。")
		} else {
			b.sendReply(ctx, e, "截图生成失败，请稍后重试。")
		}
		return nil
	}

	// 发送图片（使用独立的超时 ctx，因为回调 ctx 只有 10s）
	sendCtx, sendCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer sendCancel()
	if err := b.sendImage(sendCtx, e, pngData); err != nil {
		slog.Error("发送图片失败", "chat_id", chatID, "error", err)
		b.sendReply(ctx, e, "截图生成成功，但发送失败。请稍后重试。")
	}
	return nil
}

// extractUniqueProviders 从订阅列表中提取去重的 provider 列表
func extractUniqueProviders(subs []*storage.Subscription) []string {
	seen := make(map[string]struct{})
	var providers []string
	for _, sub := range subs {
		if _, ok := seen[sub.Provider]; !ok {
			seen[sub.Provider] = struct{}{}
			providers = append(providers, sub.Provider)
		}
	}
	return providers
}

// extractUniqueServices 从订阅列表中提取去重的 service 列表
func extractUniqueServices(subs []*storage.Subscription) []string {
	seen := make(map[string]struct{})
	var services []string
	for _, sub := range subs {
		if _, ok := seen[sub.Service]; !ok {
			seen[sub.Service] = struct{}{}
			services = append(services, sub.Service)
		}
	}
	return services
}

// getOwnerLabel 获取截图专属标识（群名或用户昵称）
func (b *Bot) getOwnerLabel(ctx context.Context, e *OneBotEvent) string {
	if e == nil {
		return "你"
	}

	switch e.MessageType {
	case "group":
		// 群聊：尝试获取群名称
		if e.GroupID != 0 {
			info, err := b.client.GetGroupInfo(ctx, e.GroupID)
			if err != nil {
				slog.Warn("获取群信息失败，回退到群号", "group_id", e.GroupID, "error", err)
				return fmt.Sprintf("群%d", e.GroupID)
			}
			if info.GroupName != "" {
				// 截断过长的群名（最多 20 字符）
				name := info.GroupName
				if len([]rune(name)) > 20 {
					name = string([]rune(name)[:20]) + "…"
				}
				return name
			}
			return fmt.Sprintf("群%d", e.GroupID)
		}
	case "private":
		// 私聊：使用发送者昵称
		if e.Sender != nil && e.Sender.Nickname != "" {
			// 截断过长的昵称（最多 20 字符）
			name := e.Sender.Nickname
			if len([]rune(name)) > 20 {
				name = string([]rune(name)[:20]) + "…"
			}
			return name
		}
	}

	return "你"
}

// sendImage 发送图片消息
func (b *Bot) sendImage(ctx context.Context, e *OneBotEvent, imageData []byte) error {
	if e == nil || len(imageData) == 0 {
		return nil
	}

	// Base64 编码图片数据
	base64Data := base64.StdEncoding.EncodeToString(imageData)

	var err error
	switch e.MessageType {
	case "group":
		if e.GroupID == 0 {
			return nil
		}
		_, err = b.client.SendGroupImageMessage(ctx, e.GroupID, base64Data)
	case "private":
		if e.UserID == 0 {
			return nil
		}
		_, err = b.client.SendPrivateImageMessage(ctx, e.UserID, base64Data)
	}
	return err
}
