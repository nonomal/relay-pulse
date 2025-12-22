package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"monitor/internal/api"
	"monitor/internal/buildinfo"
	"monitor/internal/config"
	"monitor/internal/events"
	"monitor/internal/logger"
	"monitor/internal/scheduler"
	"monitor/internal/selftest"
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

	// 创建事件服务（如果启用）
	eventSvc, err := events.NewService(events.DetectorConfig{
		DownThreshold: cfg.Events.DownThreshold,
		UpThreshold:   cfg.Events.UpThreshold,
	}, store, cfg.Events.Enabled)
	if err != nil {
		logger.Error("main", "创建事件服务失败", "error", err)
		os.Exit(1)
	}
	if eventSvc.IsEnabled() {
		sched.SetEventService(eventSvc)
		logger.Info("main", "事件服务已启用",
			"down_threshold", cfg.Events.DownThreshold,
			"up_threshold", cfg.Events.UpThreshold)
	}

	sched.Start(ctx, cfg)

	// 创建API服务器
	server := api.NewServer(store, cfg, "8080")

	// 初始化自助测试管理器（如果启用）
	var selfTestMgr *selftest.TestJobManager
	if cfg.SelfTest.Enabled {
		// 设置 selftest 数据目录（用于动态读取 cc_base.json、cx_base.json 等模板）
		// 数据目录为配置文件所在目录下的 data/ 子目录
		configDir := filepath.Dir(configFile)
		if configDir == "" || configDir == "." {
			// 如果配置文件在当前目录，使用当前工作目录
			if cwd, err := os.Getwd(); err == nil {
				configDir = cwd
			}
		}
		dataDir := filepath.Join(configDir, "data")
		selftest.SetDataDir(dataDir)

		// 解析时间间隔
		jobTimeout, err := time.ParseDuration(cfg.SelfTest.JobTimeout)
		if err != nil || jobTimeout <= 0 {
			jobTimeout = 30 * time.Second
		}
		resultTTL, err := time.ParseDuration(cfg.SelfTest.ResultTTL)
		if err != nil || resultTTL <= 0 {
			resultTTL = 2 * time.Minute
		}

		// 应用默认值
		maxConcurrent := cfg.SelfTest.MaxConcurrent
		if maxConcurrent <= 0 {
			maxConcurrent = 10
		}
		maxQueueSize := cfg.SelfTest.MaxQueueSize
		if maxQueueSize <= 0 {
			maxQueueSize = 50
		}
		rateLimitPerMinute := cfg.SelfTest.RateLimitPerMinute
		if rateLimitPerMinute <= 0 {
			rateLimitPerMinute = 10
		}

		// 创建 TestJobManager（内部创建独立的安全 prober）
		selfTestMgr = selftest.NewTestJobManager(
			maxConcurrent,
			maxQueueSize,
			jobTimeout,
			resultTTL,
			rateLimitPerMinute,
		)

		// 注入到 handler
		server.GetHandler().SetSelfTestManager(selfTestMgr)

		logger.Info("main", "自助测试功能已启用",
			"max_concurrent", maxConcurrent,
			"max_queue_size", maxQueueSize,
			"job_timeout", jobTimeout,
			"result_ttl", resultTTL,
			"rate_limit", rateLimitPerMinute)
	}

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

	// 停止自助测试管理器（如果启用）
	if selfTestMgr != nil {
		selfTestMgr.Stop()
		logger.Info("main", "自助测试管理器已关闭")
	}

	// 停止HTTP服务器
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Stop(shutdownCtx); err != nil {
		logger.Warn("main", "HTTP服务器关闭错误", "error", err)
	}

	logger.Info("main", "服务已安全退出")
}
