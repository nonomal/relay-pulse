package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"strings"
	"sync"
	"time"

	"notifier/internal/config"
	"notifier/internal/screenshot"
	"notifier/internal/storage"
	"notifier/internal/validator"
)

// Bot Telegram Bot
type Bot struct {
	client            *Client
	cfg               *config.Config
	storage           storage.Storage
	screenshotService *screenshot.Service
	validator         *validator.RelayPulseValidator
	handlers          map[string]CommandHandler

	mu       sync.Mutex
	running  bool
	stopChan chan struct{}
}

// CommandHandler 命令处理函数
type CommandHandler func(ctx context.Context, msg *Message, args string) error

// NewBot 创建 Bot
func NewBot(cfg *config.Config, store storage.Storage) *Bot {
	client := NewClient(cfg.Telegram.BotToken)

	// 初始化订阅验证器
	var v *validator.RelayPulseValidator
	if cfg.RelayPulse.EventsURL != "" {
		var err error
		v, err = validator.NewRelayPulseValidator(cfg.RelayPulse.EventsURL)
		if err != nil {
			slog.Warn("订阅验证器初始化失败", "error", err)
			v = nil
		}
	}

	b := &Bot{
		client:    client,
		cfg:       cfg,
		storage:   store,
		validator: v,
		handlers:  make(map[string]CommandHandler),
		stopChan:  make(chan struct{}),
	}

	// 注册命令处理器
	b.handlers["start"] = b.handleStart
	b.handlers["list"] = b.handleList
	b.handlers["add"] = b.handleAdd
	b.handlers["remove"] = b.handleRemove
	b.handlers["clear"] = b.handleClear
	b.handlers["status"] = b.handleStatus
	b.handlers["help"] = b.handleHelp
	b.handlers["snap"] = b.handleSnap

	return b
}

// SetScreenshotService 设置截图服务（可选）
func (b *Bot) SetScreenshotService(svc *screenshot.Service) {
	b.screenshotService = svc
}

// Start 启动 Bot（Long Polling）
func (b *Bot) Start(ctx context.Context) error {
	b.mu.Lock()
	if b.running {
		b.mu.Unlock()
		return fmt.Errorf("Bot 已在运行")
	}
	b.running = true
	b.stopChan = make(chan struct{})
	b.mu.Unlock()

	// 验证 Bot Token
	me, err := b.client.GetMe(ctx)
	if err != nil {
		return fmt.Errorf("验证 Bot Token 失败: %w", err)
	}
	slog.Info("Telegram Bot 启动", "username", me.Username, "id", me.ID)

	var offset int64 = 0
	pollTimeout := 30 // Long Polling 超时秒数

	for {
		select {
		case <-ctx.Done():
			slog.Info("Bot 收到停止信号")
			return ctx.Err()
		case <-b.stopChan:
			slog.Info("Bot 停止")
			return nil
		default:
		}

		updates, err := b.client.GetUpdates(ctx, offset, pollTimeout)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			slog.Warn("获取更新失败", "error", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, update := range updates {
			offset = update.UpdateID + 1

			if update.Message != nil {
				go b.handleMessage(ctx, update.Message)
			}
		}
	}
}

// Stop 停止 Bot
func (b *Bot) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.running {
		close(b.stopChan)
		b.running = false
	}
}

// handleMessage 处理消息
func (b *Bot) handleMessage(ctx context.Context, msg *Message) {
	if msg.Text == "" || !strings.HasPrefix(msg.Text, "/") {
		return
	}

	// 解析命令
	parts := strings.SplitN(msg.Text, " ", 2)
	cmdPart := strings.TrimPrefix(parts[0], "/")

	// 移除 @BotUsername 后缀
	if idx := strings.Index(cmdPart, "@"); idx != -1 {
		cmdPart = cmdPart[:idx]
	}

	var args string
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}

	handler, ok := b.handlers[cmdPart]
	if !ok {
		b.sendReply(ctx, msg.Chat.ID, "未知命令。发送 /help 查看帮助。")
		return
	}

	// 更新用户信息
	if err := b.ensureUser(ctx, msg); err != nil {
		slog.Error("更新用户信息失败", "chat_id", msg.Chat.ID, "error", err)
	}

	// 更新命令时间
	if err := b.storage.UpdateChatCommandTime(ctx, storage.PlatformTelegram, msg.Chat.ID); err != nil {
		slog.Warn("更新命令时间失败", "error", err)
	}

	// 执行命令
	if err := handler(ctx, msg, args); err != nil {
		slog.Error("命令执行失败", "command", cmdPart, "chat_id", msg.Chat.ID, "error", err)
		b.sendReply(ctx, msg.Chat.ID, "命令执行出错，请稍后重试。")
	}
}

// ensureUser 确保用户存在
func (b *Bot) ensureUser(ctx context.Context, msg *Message) error {
	chat := &storage.Chat{
		Platform:  storage.PlatformTelegram,
		ChatID:    msg.Chat.ID,
		Username:  msg.Chat.Username,
		FirstName: msg.Chat.FirstName,
	}
	return b.storage.UpsertChat(ctx, chat)
}

// sendReply 发送回复
func (b *Bot) sendReply(ctx context.Context, chatID int64, text string) {
	if _, err := b.client.SendMessageHTML(ctx, chatID, text); err != nil {
		slog.Error("发送消息失败", "chat_id", chatID, "error", err)

		// 检查是否被封禁
		if IsForbiddenError(err) {
			if err := b.storage.UpdateChatStatus(ctx, storage.PlatformTelegram, chatID, storage.ChatStatusBlocked); err != nil {
				slog.Error("更新用户状态失败", "error", err)
			}
		}
	}
}

// handleStart 处理 /start 命令
func (b *Bot) handleStart(ctx context.Context, msg *Message, args string) error {
	if args == "" {
		// 普通 /start
		welcome := `欢迎使用 <b>RelayPulse 通知 Bot</b>！

我可以在你收藏的 LLM 中继服务状态变化时发送通知。

<b>命令列表：</b>
/list - 查看当前订阅
/add &lt;provider&gt; [service] [channel] - 添加订阅
/remove &lt;provider&gt; [service] [channel] - 移除订阅
/clear - 清空所有订阅
/snap - 截图订阅服务状态
/status - 查看服务状态
/help - 显示帮助

<b>快速开始：</b>
从 RelayPulse 网页点击"订阅通知"按钮，即可一键导入收藏列表。`

		b.sendReply(ctx, msg.Chat.ID, welcome)
		return nil
	}

	// 带 token 的 /start，从 bind-token API 获取收藏列表
	token := args

	// 消费 token
	bindToken, err := b.storage.ConsumeBindToken(ctx, token)
	if err != nil {
		slog.Warn("消费绑定 token 失败", "error", err)
		b.sendReply(ctx, msg.Chat.ID, "绑定链接无效或已过期，请重新从网页获取。")
		return nil
	}

	if bindToken == nil {
		b.sendReply(ctx, msg.Chat.ID, "绑定链接不存在，请重新从网页获取。")
		return nil
	}

	// 解析收藏列表并创建订阅
	favorites, err := parseBindTokenFavorites(bindToken.Favorites)
	if err != nil {
		slog.Error("解析收藏列表失败", "error", err)
		b.sendReply(ctx, msg.Chat.ID, "收藏数据格式错误，请联系管理员。")
		return nil
	}

	// 检查订阅数量限制
	currentCount, err := b.storage.CountSubscriptions(ctx, storage.PlatformTelegram, msg.Chat.ID)
	if err != nil {
		return err
	}

	maxSubs := b.cfg.Limits.MaxSubscriptionsPerUser
	availableSlots := maxSubs - currentCount
	if availableSlots <= 0 {
		b.sendReply(ctx, msg.Chat.ID, fmt.Sprintf(
			"订阅数量已达上限（%d/%d）。请先使用 /clear 清空或 /remove 移除部分订阅。",
			currentCount, maxSubs,
		))
		return nil
	}

	// 添加订阅
	added := 0
	for _, fav := range favorites {
		if added >= availableSlots {
			break
		}

		sub := &storage.Subscription{
			Platform: storage.PlatformTelegram,
			ChatID:   msg.Chat.ID,
			Provider: fav.Provider,
			Service:  fav.Service,
			Channel:  fav.Channel,
		}

		if err := b.storage.AddSubscription(ctx, sub); err != nil {
			slog.Warn("添加订阅失败", "error", err)
			continue
		}
		added++
	}

	reply := fmt.Sprintf(
		"成功导入 <b>%d</b> 个订阅！\n\n发送 /list 查看当前订阅列表。",
		added,
	)

	if len(favorites) > added {
		reply += fmt.Sprintf("\n\n⚠️ 部分订阅因数量限制未能添加（%d/%d）", added, len(favorites))
	}

	b.sendReply(ctx, msg.Chat.ID, reply)
	return nil
}

// handleList 处理 /list 命令
func (b *Bot) handleList(ctx context.Context, msg *Message, args string) error {
	subs, err := b.storage.GetSubscriptionsByChatID(ctx, storage.PlatformTelegram, msg.Chat.ID)
	if err != nil {
		return err
	}

	if len(subs) == 0 {
		b.sendReply(ctx, msg.Chat.ID, "你还没有订阅任何服务。\n\n使用 /add 添加订阅，或从网页点击「订阅通知」一键导入。")
		return nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>当前订阅（%d 个）：</b>\n\n", len(subs)))

	for i, sub := range subs {
		// 转义 HTML 防止注入
		provider := html.EscapeString(sub.Provider)
		service := html.EscapeString(sub.Service)
		channel := html.EscapeString(sub.Channel)

		// 根据订阅级别显示不同格式
		if sub.Service == "" {
			// 旧版通配订阅（provider 级）
			sb.WriteString(fmt.Sprintf("%d. %s / *（旧版）\n", i+1, provider))
		} else if channel != "" {
			// 精确订阅（provider / service / channel）
			sb.WriteString(fmt.Sprintf("%d. %s / %s / %s\n", i+1, provider, service, channel))
		} else {
			// service 级订阅（provider / service）
			sb.WriteString(fmt.Sprintf("%d. %s / %s\n", i+1, provider, service))
		}
	}

	sb.WriteString("\n使用 /remove &lt;provider&gt; [service] [channel] 移除订阅")

	b.sendReply(ctx, msg.Chat.ID, sb.String())
	return nil
}

// handleAdd 处理 /add 命令
// 支持三种订阅模式：
// - /add <provider> → 展开订阅该 provider 下所有 service/channel
// - /add <provider> <service> → 展开订阅该 service 下所有 channel
// - /add <provider> <service> <channel> → 精确订阅
func (b *Bot) handleAdd(ctx context.Context, msg *Message, args string) error {
	parts := strings.Fields(args)
	if len(parts) < 1 {
		b.sendReply(ctx, msg.Chat.ID, "用法: /add &lt;provider&gt; [service] [channel]\n\n例如:\n/add 88code → 订阅 88code 所有服务\n/add 88code cc → 订阅 88code 的 cc 服务")
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
	count, err := b.storage.CountSubscriptions(ctx, storage.PlatformTelegram, msg.Chat.ID)
	if err != nil {
		return err
	}
	maxSubs := b.cfg.Limits.MaxSubscriptionsPerUser

	// 验证服务是否配置
	if b.validator == nil {
		b.sendReply(ctx, msg.Chat.ID, "当前无法验证订阅（验证服务未配置），为避免订阅无效服务，已拒绝本次订阅。请从网页一键导入收藏。")
		return nil
	}

	// Provider 级订阅：展开为所有 service/channel
	if service == "" {
		targets, err := b.validator.ValidateAndExpandProvider(ctx, provider)
		if err != nil {
			return b.handleAddError(ctx, msg.Chat.ID, err, provider, "", "")
		}

		// 检查配额（maxSubs==0 表示无限制）
		if maxSubs > 0 && count+len(targets) > maxSubs {
			b.sendReply(ctx, msg.Chat.ID, fmt.Sprintf(
				"订阅配额不足。<b>%s</b> 有 %d 个订阅项，当前已用 %d/%d。\n\n请先移除部分订阅，或精确指定 service/channel。",
				html.EscapeString(provider), len(targets), count, maxSubs,
			))
			return nil
		}

		// 逐一添加
		added := 0
		for _, t := range targets {
			sub := &storage.Subscription{
				Platform: storage.PlatformTelegram,
				ChatID:   msg.Chat.ID,
				Provider: t.Provider,
				Service:  t.Service,
				Channel:  t.Channel,
			}
			if err := b.storage.AddSubscription(ctx, sub); err == nil {
				added++
			}
		}

		b.sendReply(ctx, msg.Chat.ID, fmt.Sprintf(
			"已添加 <b>%d</b> 个订阅（%s 下所有服务通道）",
			added, html.EscapeString(provider),
		))
		return nil
	}

	// Service 级订阅：展开为所有 channel
	if channel == "" {
		targets, err := b.validator.ValidateAndExpandService(ctx, provider, service)
		if err != nil {
			return b.handleAddError(ctx, msg.Chat.ID, err, provider, service, "")
		}

		// 检查配额（maxSubs==0 表示无限制）
		if maxSubs > 0 && count+len(targets) > maxSubs {
			b.sendReply(ctx, msg.Chat.ID, fmt.Sprintf(
				"订阅配额不足。<b>%s / %s</b> 有 %d 个订阅项，当前已用 %d/%d。\n\n请先移除部分订阅，或精确指定 channel。",
				html.EscapeString(provider), html.EscapeString(service), len(targets), count, maxSubs,
			))
			return nil
		}

		// 逐一添加
		added := 0
		for _, t := range targets {
			sub := &storage.Subscription{
				Platform: storage.PlatformTelegram,
				ChatID:   msg.Chat.ID,
				Provider: t.Provider,
				Service:  t.Service,
				Channel:  t.Channel,
			}
			if err := b.storage.AddSubscription(ctx, sub); err == nil {
				added++
			}
		}

		b.sendReply(ctx, msg.Chat.ID, fmt.Sprintf(
			"已添加 <b>%d</b> 个订阅（%s / %s 下所有通道）",
			added, html.EscapeString(provider), html.EscapeString(service),
		))
		return nil
	}

	// 精确订阅
	if maxSubs > 0 && count >= maxSubs {
		b.sendReply(ctx, msg.Chat.ID, fmt.Sprintf(
			"订阅数量已达上限（%d/%d）。请先移除部分订阅。",
			count, maxSubs,
		))
		return nil
	}

	target, err := b.validator.ValidateAdd(ctx, provider, service, channel)
	if err != nil {
		return b.handleAddError(ctx, msg.Chat.ID, err, provider, service, channel)
	}

	sub := &storage.Subscription{
		Platform: storage.PlatformTelegram,
		ChatID:   msg.Chat.ID,
		Provider: target.Provider,
		Service:  target.Service,
		Channel:  target.Channel,
	}

	if err := b.storage.AddSubscription(ctx, sub); err != nil {
		return err
	}

	b.sendReply(ctx, msg.Chat.ID, fmt.Sprintf(
		"已订阅 <b>%s / %s / %s</b>",
		html.EscapeString(target.Provider),
		html.EscapeString(target.Service),
		html.EscapeString(target.Channel),
	))
	return nil
}

// handleAddError 处理添加订阅时的错误
func (b *Bot) handleAddError(ctx context.Context, chatID int64, err error, provider, service, channel string) error {
	providerEsc := html.EscapeString(provider)
	serviceEsc := html.EscapeString(service)
	channelEsc := html.EscapeString(channel)

	var nf *validator.NotFoundError
	if errors.As(err, &nf) {
		switch nf.Level {
		case validator.NotFoundProvider:
			b.sendReply(ctx, chatID, fmt.Sprintf(
				"未找到服务商 <b>%s</b>。\n\n请到 RelayPulse 网页复制正确的 provider/service/channel，或使用网页一键导入收藏。",
				providerEsc,
			))
		case validator.NotFoundService:
			cands := validator.FormatCandidates(nf.Candidates, "service")
			if cands != "" {
				b.sendReply(ctx, chatID, fmt.Sprintf(
					"未找到 <b>%s / %s</b>。\n\n该服务商下可用的 service 例如：<b>%s</b>（仅显示前 %d 个）。",
					providerEsc, serviceEsc, html.EscapeString(cands), 8,
				))
			} else {
				b.sendReply(ctx, chatID, fmt.Sprintf(
					"未找到 <b>%s / %s</b>。\n\n请到 RelayPulse 网页确认 service 是否正确，或使用网页一键导入收藏。",
					providerEsc, serviceEsc,
				))
			}
		case validator.NotFoundChannel:
			cands := validator.FormatCandidates(nf.Candidates, "channel")
			if cands != "" {
				b.sendReply(ctx, chatID, fmt.Sprintf(
					"未找到 <b>%s / %s / %s</b>。\n\n该 service 下可用的 channel 例如：<b>%s</b>（仅显示前 %d 个）。",
					providerEsc, serviceEsc, channelEsc, html.EscapeString(cands), 8,
				))
			} else {
				b.sendReply(ctx, chatID, fmt.Sprintf(
					"未找到 <b>%s / %s / %s</b>。",
					providerEsc, serviceEsc, channelEsc,
				))
			}
		}
		return nil
	}

	var ue *validator.UnavailableError
	if errors.As(err, &ue) {
		b.sendReply(ctx, chatID, "当前无法验证订阅（状态服务暂不可用），为避免订阅无效服务，已拒绝本次订阅。请稍后再试，或从网页一键导入收藏。")
		return nil
	}

	return err
}

// handleRemove 处理 /remove 命令
// 支持三种移除模式：
// - /remove <provider> → 移除该 provider 下所有订阅（级联删除）
// - /remove <provider> <service> → 移除该 service 下所有通道
// - /remove <provider> <service> <channel> → 精确移除
func (b *Bot) handleRemove(ctx context.Context, msg *Message, args string) error {
	parts := strings.Fields(args)
	if len(parts) < 1 {
		b.sendReply(ctx, msg.Chat.ID, "用法: /remove &lt;provider&gt; [service] [channel]\n\n例如:\n/remove 88code → 移除 88code 所有订阅\n/remove 88code cc → 移除 88code 的 cc 服务订阅")
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

	if err := b.storage.RemoveSubscription(ctx, storage.PlatformTelegram, msg.Chat.ID, provider, service, channel); err != nil {
		return err
	}

	// 转义 HTML 防止注入
	providerEsc := html.EscapeString(provider)
	serviceEsc := html.EscapeString(service)
	channelEsc := html.EscapeString(channel)

	// 根据删除级别显示不同消息
	if service == "" {
		// provider 级删除（级联）
		b.sendReply(ctx, msg.Chat.ID, fmt.Sprintf("已取消订阅 <b>%s / *</b>（包括该服务商下所有订阅）", providerEsc))
	} else if channel != "" {
		// 精确删除
		b.sendReply(ctx, msg.Chat.ID, fmt.Sprintf("已取消订阅 <b>%s / %s / %s</b>", providerEsc, serviceEsc, channelEsc))
	} else {
		// service 级删除
		b.sendReply(ctx, msg.Chat.ID, fmt.Sprintf("已取消订阅 <b>%s / %s</b>", providerEsc, serviceEsc))
	}
	return nil
}

// handleClear 处理 /clear 命令
func (b *Bot) handleClear(ctx context.Context, msg *Message, args string) error {
	if err := b.storage.ClearSubscriptions(ctx, storage.PlatformTelegram, msg.Chat.ID); err != nil {
		return err
	}

	b.sendReply(ctx, msg.Chat.ID, "已清空所有订阅。")
	return nil
}

// handleStatus 处理 /status 命令
func (b *Bot) handleStatus(ctx context.Context, msg *Message, args string) error {
	count, err := b.storage.CountSubscriptions(ctx, storage.PlatformTelegram, msg.Chat.ID)
	if err != nil {
		return err
	}

	status := fmt.Sprintf(`<b>服务状态</b>

订阅数量: %d/%d
服务版本: %s
状态: 运行中 ✅

数据源: %s`,
		count, b.cfg.Limits.MaxSubscriptionsPerUser,
		"dev", // TODO: 从外部传入版本号
		b.cfg.RelayPulse.EventsURL,
	)

	b.sendReply(ctx, msg.Chat.ID, status)
	return nil
}

// handleHelp 处理 /help 命令
func (b *Bot) handleHelp(ctx context.Context, msg *Message, args string) error {
	help := `<b>RelayPulse 通知 Bot 帮助</b>

<b>命令列表：</b>
/start - 开始使用 / 导入收藏
/list - 查看当前订阅
/add &lt;provider&gt; [service] [channel] - 添加订阅
/remove &lt;provider&gt; [service] [channel] - 移除订阅
/clear - 清空所有订阅
/snap - 截图订阅服务状态
/status - 查看服务状态
/help - 显示此帮助

<b>快速开始：</b>
1. 访问 RelayPulse 网站
2. 收藏你关注的服务
3. 点击"订阅通知"按钮
4. 跳转到此 Bot 自动导入

<b>手动添加订阅：</b>
/add 88code → 订阅 88code 所有服务
/add 88code cc → 订阅 88code 的 cc 服务
/add duckcoding cc v1 → 精确订阅

<b>移除订阅：</b>
/remove 88code → 移除 88code 所有订阅
/remove 88code cc → 移除 88code 的 cc 订阅`

	b.sendReply(ctx, msg.Chat.ID, help)
	return nil
}

// handleSnap 处理 /snap 命令（截图订阅服务状态）
func (b *Bot) handleSnap(ctx context.Context, msg *Message, args string) error {
	chatID := msg.Chat.ID

	// 检查截图服务是否启用
	if b.screenshotService == nil {
		b.sendReply(ctx, chatID, "截图功能未启用。")
		return nil
	}

	// 获取订阅列表
	subs, err := b.storage.GetSubscriptionsByChatID(ctx, storage.PlatformTelegram, chatID)
	if err != nil {
		slog.Error("获取订阅失败", "chat_id", chatID, "error", err)
		b.sendReply(ctx, chatID, "获取订阅信息失败，请稍后重试。")
		return nil
	}
	if len(subs) == 0 {
		b.sendReply(ctx, chatID, "你还没有订阅任何服务。\n\n使用 /add 添加订阅后再试。")
		return nil
	}

	// 提取 provider 列表（去重）
	providers := extractUniqueProviders(subs)

	// 提取 service 列表（去重）
	services := extractUniqueServices(subs)

	// 发送提示
	b.sendReply(ctx, chatID, fmt.Sprintf("正在生成 %d 个服务商的状态截图...", len(providers)))

	// 截图（使用独立的超时 ctx）
	snapCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pngData, err := b.screenshotService.Capture(snapCtx, providers, services)
	if err != nil {
		slog.Error("截图失败", "chat_id", chatID, "providers", providers, "error", err)
		// 区分错误类型
		if errors.Is(err, context.DeadlineExceeded) {
			b.sendReply(ctx, chatID, "截图超时，请稍后重试。")
		} else if errors.Is(err, screenshot.ErrConcurrencyLimit) {
			b.sendReply(ctx, chatID, "系统繁忙，请稍后重试。")
		} else {
			b.sendReply(ctx, chatID, "截图生成失败，请稍后重试。")
		}
		return nil
	}

	// 发送图片
	if _, err := b.client.SendPhoto(ctx, chatID, pngData, ""); err != nil {
		slog.Error("发送图片失败", "chat_id", chatID, "error", err)
		b.sendReply(ctx, chatID, "截图生成成功，但发送失败。请稍后重试。")
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

// Favorite 收藏项
type Favorite struct {
	Provider string
	Service  string
	Channel  string
}

// parseBindTokenFavorites 解析绑定 token 中的收藏列表
func parseBindTokenFavorites(favoritesJSON string) ([]Favorite, error) {
	var ids []string
	if err := json.Unmarshal([]byte(favoritesJSON), &ids); err != nil {
		return nil, err
	}

	var favorites []Favorite
	for _, id := range ids {
		// ID 格式: provider-service-channel 或 provider-service-default
		// 前端生成格式: `${provider}-${service}-${channel || 'default'}`
		parts := strings.SplitN(id, "-", 3)
		if len(parts) < 2 {
			continue
		}

		fav := Favorite{
			Provider: parts[0],
			Service:  parts[1],
		}
		// 第三部分是 channel，"default" 表示无 channel
		if len(parts) > 2 && parts[2] != "default" {
			fav.Channel = parts[2]
		}
		favorites = append(favorites, fav)
	}

	return favorites, nil
}
