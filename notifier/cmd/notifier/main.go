package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"notifier/internal/api"
	"notifier/internal/config"
	"notifier/internal/notifier"
	"notifier/internal/poller"
	"notifier/internal/storage"
	"notifier/internal/telegram"
)

var (
	configPath = flag.String("config", "config.yaml", "配置文件路径")
	version    = "dev"
)

func main() {
	flag.Parse()

	// 初始化日志
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("relay-pulse-notifier 启动中", "version", version)

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("加载配置失败", "error", err)
		os.Exit(1)
	}

	slog.Info("配置加载成功",
		"events_url", cfg.RelayPulse.EventsURL,
		"poll_interval", cfg.RelayPulse.PollInterval,
		"bot_username", cfg.Telegram.BotUsername,
		"telegram_enabled", cfg.HasTelegramToken(),
	)

	// 创建上下文，支持优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 初始化存储层
	store, err := storage.NewSQLiteStorage(cfg.Database.DSN)
	if err != nil {
		slog.Error("初始化存储失败", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	if err := store.Init(ctx); err != nil {
		slog.Error("初始化数据库表失败", "error", err)
		os.Exit(1)
	}
	slog.Info("存储层初始化成功")

	// 初始化 HTTP API 服务器
	apiServer := api.NewServer(cfg, store)

	// 启动 HTTP API 服务器
	go func() {
		if err := apiServer.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("API 服务器错误", "error", err)
			cancel()
		}
	}()

	// 变量声明（用于优雅关闭）
	var bot *telegram.Bot
	var sender *notifier.Sender
	var eventPoller *poller.Poller

	// 仅在配置了 Telegram Token 时启动 Bot、Sender 和 Poller
	if cfg.HasTelegramToken() {
		// 初始化并启动 Telegram Bot
		bot = telegram.NewBot(cfg, store)
		go func() {
			if err := bot.Start(ctx); err != nil && ctx.Err() == nil {
				slog.Error("Telegram Bot 错误", "error", err)
				cancel()
			}
		}()

		// 初始化通知发送器
		sender = notifier.NewSender(cfg, store)
		go func() {
			if err := sender.Start(ctx); err != nil && ctx.Err() == nil {
				slog.Error("通知发送器错误", "error", err)
				cancel()
			}
		}()

		// 初始化并启动事件轮询器
		eventPoller = poller.NewPoller(cfg, store, sender.HandleEvent)
		go func() {
			if err := eventPoller.Start(ctx); err != nil && ctx.Err() == nil {
				slog.Error("事件轮询器错误", "error", err)
				cancel()
			}
		}()
	} else {
		slog.Warn("未配置 TELEGRAM_BOT_TOKEN，Bot/Poller/Sender 功能已禁用",
			"hint", "仅 API 服务器可用（bind-token 接口）")
	}

	// 监听关闭信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		slog.Info("收到关闭信号", "signal", sig)
	case <-ctx.Done():
	}

	// 优雅关闭
	slog.Info("服务正在关闭...")

	// 停止各组件（仅在初始化时才需要停止）
	if eventPoller != nil {
		eventPoller.Stop()
	}
	if sender != nil {
		sender.Stop()
	}
	if bot != nil {
		bot.Stop()
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := apiServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("API 服务器关闭失败", "error", err)
	}

	slog.Info("服务已关闭")
}
