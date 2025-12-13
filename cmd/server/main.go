package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"monitor/internal/api"
	"monitor/internal/buildinfo"
	"monitor/internal/config"
	"monitor/internal/logger"
	"monitor/internal/scheduler"
	"monitor/internal/storage"
)

// buildChannelMigrationMappings 从配置构建 channel 迁移映射（同一 provider+service 取第一个非空 channel）
func buildChannelMigrationMappings(monitors []config.ServiceConfig) []storage.ChannelMigrationMapping {
	seen := make(map[string]bool)
	mappings := make([]storage.ChannelMigrationMapping, 0, len(monitors))

	for _, monitor := range monitors {
		// 跳过已禁用的监测项
		if monitor.Disabled {
			continue
		}
		// 跳过空 channel
		if monitor.Channel == "" {
			continue
		}

		key := monitor.Provider + "|" + monitor.Service
		if seen[key] {
			continue
		}
		seen[key] = true

		mappings = append(mappings, storage.ChannelMigrationMapping{
			Provider: monitor.Provider,
			Service:  monitor.Service,
			Channel:  monitor.Channel,
		})
	}

	return mappings
}

func main() {
	// 打印版本信息
	logger.Info("main", "Relay Pulse Monitor 启动",
		"version", buildinfo.GetVersion(),
		"git_commit", buildinfo.GetGitCommit(),
		"build_time", buildinfo.GetBuildTime())

	// 配置文件路径
	configFile := "config.yaml"
	if len(os.Args) > 1 {
		configFile = os.Args[1]
	}

	// 创建配置加载器
	loader := config.NewLoader()

	// 初始加载配置
	cfg, err := loader.Load(configFile)
	if err != nil {
		logger.Error("main", "无法加载配置文件", "error", err)
		os.Exit(1)
	}

	logger.Info("main", "配置加载完成",
		"monitors", len(cfg.Monitors),
		"interval", cfg.Interval,
		"max_concurrency", cfg.MaxConcurrency,
		"stagger_probes", cfg.StaggerProbes,
		"slow_latency", cfg.SlowLatency,
		"degraded_weight", cfg.DegradedWeight,
	)

	// 初始化存储（支持 SQLite 和 PostgreSQL）
	store, err := storage.New(&cfg.Storage)
	if err != nil {
		logger.Error("main", "初始化存储失败", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	if err := store.Init(); err != nil {
		logger.Error("main", "初始化数据库失败", "error", err)
		os.Exit(1)
	}

	// 自动迁移旧数据的 channel
	if err := store.MigrateChannelData(buildChannelMigrationMappings(cfg.Monitors)); err != nil {
		logger.Warn("main", "channel 数据迁移失败", "error", err)
	}

	storageType := cfg.Storage.Type
	if storageType == "" {
		storageType = "sqlite"
	}
	logger.Info("main", "存储已就绪", "type", storageType)

	// 创建上下文（用于优雅关闭）
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 创建调度器（支持通过 config.yaml 配置 interval）
	interval := cfg.IntervalDuration
	if interval <= 0 {
		interval = time.Minute
	}
	sched := scheduler.NewScheduler(store, interval)
	sched.Start(ctx, cfg)

	// 创建API服务器
	server := api.NewServer(store, cfg, "8080")

	// 启动配置监听器（热更新）
	watcher, err := config.NewWatcher(loader, configFile, func(newCfg *config.AppConfig) {
		// 配置热更新回调
		sched.UpdateConfig(newCfg)
		server.UpdateConfig(newCfg)
		// 重新运行 channel 迁移（支持运行时添加 channel）
		if err := store.MigrateChannelData(buildChannelMigrationMappings(newCfg.Monitors)); err != nil {
			logger.Warn("main", "热更新时 channel 迁移失败", "error", err)
		}
		// 注意：不再调用 TriggerNow()，rebuildTasks 已安排错峰首次执行
		// 避免与 rebuildTasks 的首轮调度产生竞态导致重复探测
	})

	if err != nil {
		logger.Warn("main", "配置监听器创建失败，热更新功能不可用", "error", err)
	} else {
		if err := watcher.Start(ctx); err != nil {
			logger.Warn("main", "配置监听器启动失败，热更新功能不可用", "error", err)
		} else {
			logger.Info("main", "配置热更新已启用")
		}
	}

	// 启动定期清理任务（保留30天数据）
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := store.CleanOldRecords(30); err != nil {
					logger.Warn("main", "清理旧记录失败", "error", err)
				}
			}
		}
	}()

	// 监听中断信号（优雅关闭）
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 启动HTTP服务器（阻塞）
	go func() {
		if err := server.Start(); err != nil {
			logger.Error("main", "HTTP服务器错误", "error", err)
			cancel()
			// 向信号通道发送信号，确保进程退出
			sigChan <- syscall.SIGTERM
		}
	}()

	// 等待中断信号
	<-sigChan
	logger.Info("main", "收到关闭信号，正在优雅退出")

	// 取消上下文
	cancel()

	// 停止调度器
	sched.Stop()

	// 停止HTTP服务器
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Stop(shutdownCtx); err != nil {
		logger.Warn("main", "HTTP服务器关闭错误", "error", err)
	}

	logger.Info("main", "服务已安全退出")
}
