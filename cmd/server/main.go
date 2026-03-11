package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"monitor/internal/announcements"
	"monitor/internal/api"
	"monitor/internal/automove"
	"monitor/internal/buildinfo"
	"monitor/internal/config"
	"monitor/internal/events"
	"monitor/internal/identity"
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

// resolveConfigDir 解析配置文件所在目录的绝对路径
func resolveConfigDir(configFile string) string {
	configDir := filepath.Dir(configFile)
	if configDir == "" || configDir == "." {
		if cwd, err := os.Getwd(); err == nil {
			return cwd
		}
	}
	return configDir
}

// newSelfTestManager 基于 Normalize 后的配置创建 TestJobManager
// 注意：cfg.SelfTest 的默认值和 Duration 解析已由 Normalize 完成
func newSelfTestManager(cfg *config.AppConfig, configFile string) *selftest.TestJobManager {
	selftest.SetTemplatesDir(filepath.Join(resolveConfigDir(configFile), "templates"))
	return selftest.NewTestJobManager(
		cfg.SelfTest.MaxConcurrent,
		cfg.SelfTest.MaxQueueSize,
		cfg.SelfTest.JobTimeoutDuration,
		cfg.SelfTest.ResultTTLDuration,
		cfg.SelfTest.RateLimitPerMinute,
		selftest.WithSlowLatencyByService(cfg.SlowLatencyByServiceDuration),
	)
}

// selfTestConfigChanged 检测 selftest 运行时配置是否需要重建 Manager
func selfTestConfigChanged(oldCfg, newCfg *config.AppConfig) bool {
	if oldCfg == nil || newCfg == nil {
		return true
	}
	o, n := oldCfg.SelfTest, newCfg.SelfTest
	if o.Enabled != n.Enabled {
		return true
	}
	if !o.Enabled && !n.Enabled {
		return false
	}
	return o.MaxConcurrent != n.MaxConcurrent ||
		o.MaxQueueSize != n.MaxQueueSize ||
		o.JobTimeoutDuration != n.JobTimeoutDuration ||
		o.ResultTTLDuration != n.ResultTTLDuration ||
		o.RateLimitPerMinute != n.RateLimitPerMinute ||
		!sameDurationMap(oldCfg.SlowLatencyByServiceDuration, newCfg.SlowLatencyByServiceDuration)
}

// announcementsConfigChanged 检测公告配置是否需要重建 Service
func announcementsConfigChanged(oldCfg, newCfg *config.AppConfig) bool {
	if oldCfg == nil || newCfg == nil {
		return true
	}
	oEnabled, nEnabled := oldCfg.Announcements.IsEnabled(), newCfg.Announcements.IsEnabled()
	if oEnabled != nEnabled {
		return true
	}
	if !oEnabled && !nEnabled {
		return false
	}
	o, n := oldCfg.Announcements, newCfg.Announcements
	return o.Owner != n.Owner ||
		o.Repo != n.Repo ||
		o.CategoryName != n.CategoryName ||
		o.PollIntervalDuration != n.PollIntervalDuration ||
		o.WindowHours != n.WindowHours ||
		o.MaxItems != n.MaxItems ||
		o.APIMaxAge != n.APIMaxAge ||
		oldCfg.GitHub.Token != newCfg.GitHub.Token ||
		oldCfg.GitHub.Proxy != newCfg.GitHub.Proxy ||
		oldCfg.GitHub.TimeoutDuration != newCfg.GitHub.TimeoutDuration
}

// sameDurationMap 比较两个 map[string]time.Duration 是否相同
func sameDurationMap(a, b map[string]time.Duration) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		if bv, ok := b[k]; !ok || bv != av {
			return false
		}
	}
	return true
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

	// 启动历史数据清理任务
	var cleaner *storage.Cleaner
	if cfg.Storage.Retention.IsEnabled() {
		cleaner = storage.NewCleaner(store, &cfg.Storage.Retention)
		go cleaner.Start(ctx)
		logger.Info("main", "历史数据清理任务已启动",
			"retention_days", cfg.Storage.Retention.Days,
			"cleanup_interval", cfg.Storage.Retention.CleanupInterval)
	}

	// 启动历史数据归档任务（仅 PostgreSQL 支持）
	var archiver *storage.Archiver
	if cfg.Storage.Archive.IsEnabled() {
		// 检查存储是否支持归档（仅 PostgreSQL 支持）
		if _, ok := store.(storage.ArchiveStorage); !ok {
			logger.Warn("main", "归档功能已启用但当前存储不支持（仅 PostgreSQL 支持），归档任务将不会执行",
				"storage_type", cfg.Storage.Type)
		} else {
			archiver = storage.NewArchiver(store, &cfg.Storage.Archive)
			go archiver.Start(ctx)
			logger.Info("main", "历史数据归档任务已启动",
				"archive_days", cfg.Storage.Archive.ArchiveDays,
				"backfill_days", cfg.Storage.Archive.BackfillDays,
				"output_dir", cfg.Storage.Archive.OutputDir,
				"format", cfg.Storage.Archive.Format)
		}
	}

	// 创建调度器（支持通过 config.yaml 配置 interval）
	interval := cfg.IntervalDuration
	if interval <= 0 {
		interval = time.Minute
	}
	userIDMgr := identity.NewUserIDManager()
	sched := scheduler.NewScheduler(store, interval, userIDMgr)

	// 创建事件服务（如果启用）
	eventSvc, err := events.NewService(events.ServiceConfig{
		DetectorConfig: events.DetectorConfig{
			DownThreshold: cfg.Events.DownThreshold,
			UpThreshold:   cfg.Events.UpThreshold,
		},
		ChannelDetectorConfig: events.ChannelDetectorConfig{
			DownThreshold: cfg.Events.ChannelDownThreshold,
		},
		Mode:             cfg.Events.Mode,
		ChannelCountMode: cfg.Events.ChannelCountMode,
		Enabled:          cfg.Events.Enabled,
	}, store)
	if err != nil {
		logger.Error("main", "创建事件服务失败", "error", err)
		os.Exit(1)
	}
	if eventSvc.IsEnabled() {
		sched.SetEventService(eventSvc)
		// 初始化活跃模型索引
		eventSvc.UpdateActiveModels(cfg.Monitors, cfg.Boards.Enabled)
		logger.Info("main", "事件服务已启用",
			"mode", eventSvc.GetMode(),
			"down_threshold", cfg.Events.DownThreshold,
			"up_threshold", cfg.Events.UpThreshold,
			"channel_down_threshold", cfg.Events.ChannelDownThreshold,
			"channel_count_mode", cfg.Events.ChannelCountMode)
	}

	sched.Start(ctx, cfg)

	// 创建自动移板服务（始终创建，内部根据 enabled 决定是否实际评估）
	autoMover := automove.NewService(store, cfg)
	autoMover.Start(ctx)
	if cfg.Boards.Enabled && cfg.Boards.AutoMove.Enabled {
		logger.Info("main", "自动移板服务已启用",
			"threshold_down", cfg.Boards.AutoMove.ThresholdDown,
			"threshold_up", cfg.Boards.AutoMove.ThresholdUp,
			"check_interval", cfg.Boards.AutoMove.CheckInterval,
			"min_probes", cfg.Boards.AutoMove.MinProbes)
	}

	// 创建API服务器
	server := api.NewServer(store, cfg, "8080", autoMover)

	// runtimeMu 保护热更新回调与关闭序列之间对 mutable 组件实例的并发访问
	var runtimeMu sync.Mutex

	// 提前注册公告 API 路由（使用支持动态替换的 Handler），避免热启用后无路由入口
	announcementsHandler := announcements.NewHandler(nil)
	server.RegisterAnnouncementsHandler(announcementsHandler.GetAnnouncements)

	// 记录当前已应用的配置快照，用于热更新时判断是否需要重建组件
	selfTestAppliedCfg := cfg
	// announcementsAppliedCfg 初始为 nil：仅在服务创建成功或明确禁用时设置，
	// 避免启动失败时标记为"已应用"导致后续热更新不再重试
	var announcementsAppliedCfg *config.AppConfig

	// 初始化自助测试管理器（如果启用）
	var selfTestMgr *selftest.TestJobManager
	if cfg.SelfTest.Enabled {
		selfTestMgr = newSelfTestManager(cfg, configFile)
		server.GetHandler().SetSelfTestManager(selfTestMgr)
		logger.Info("main", "自助测试功能已启用",
			"max_concurrent", cfg.SelfTest.MaxConcurrent,
			"max_queue_size", cfg.SelfTest.MaxQueueSize,
			"job_timeout", cfg.SelfTest.JobTimeoutDuration,
			"result_ttl", cfg.SelfTest.ResultTTLDuration,
			"rate_limit", cfg.SelfTest.RateLimitPerMinute)
	}

	// 初始化公告服务（如果启用）
	var announcementsSvc *announcements.Service
	if cfg.Announcements.IsEnabled() {
		var err error
		announcementsSvc, err = announcements.NewService(cfg.Announcements, cfg.GitHub)
		if err != nil {
			logger.Error("main", "创建公告服务失败", "error", err)
			// 公告服务失败不影响主服务启动，仅警告
			// 注意：不设置 announcementsAppliedCfg，下次热更新将重试创建
		} else {
			announcementsHandler.SetService(announcementsSvc)
			announcementsSvc.Start(ctx)
			announcementsAppliedCfg = cfg
			logger.Info("main", "公告服务已启用",
				"owner", cfg.Announcements.Owner,
				"repo", cfg.Announcements.Repo,
				"category", cfg.Announcements.CategoryName,
				"poll_interval", cfg.Announcements.PollInterval,
				"window_hours", cfg.Announcements.WindowHours)
		}
	} else {
		announcementsAppliedCfg = cfg // 明确禁用也标记为已应用
	}

	// 启动配置监听器（热更新）
	watcher, err := config.NewWatcher(loader, configFile, func(newCfg *config.AppConfig) {
		// 关闭中不再处理热更新
		if ctx.Err() != nil {
			return
		}

		// 序列化热更新回调，防止与关闭序列竞态
		runtimeMu.Lock()
		defer runtimeMu.Unlock()

		if ctx.Err() != nil {
			return
		}

		// === 已有热更新支持的组件 ===
		sched.UpdateConfig(newCfg)
		server.UpdateConfig(newCfg)
		autoMover.UpdateConfig(newCfg)

		// 重新运行 channel 迁移（支持运行时添加 channel）
		if err := store.MigrateChannelData(buildChannelMigrationMappings(newCfg.Monitors)); err != nil {
			logger.Warn("main", "热更新时 channel 迁移失败", "error", err)
		}
		// 注意：不再调用 TriggerNow()，rebuildTasks 已安排错峰首次执行
		// 避免与 rebuildTasks 的首轮调度产生竞态导致重复探测

		// === Cleaner: ApplyConfig（在线更新配置 + 唤醒调度） ===
		switch {
		case newCfg.Storage.Retention.IsEnabled() && cleaner == nil:
			cleaner = storage.NewCleaner(store, &newCfg.Storage.Retention)
			go cleaner.Start(ctx)
			logger.Info("main", "历史数据清理任务已在热更新后启动",
				"retention_days", newCfg.Storage.Retention.Days,
				"cleanup_interval", newCfg.Storage.Retention.CleanupInterval)
		case newCfg.Storage.Retention.IsEnabled() && cleaner != nil:
			cleaner.UpdateRetentionConfig(&newCfg.Storage.Retention)
		case !newCfg.Storage.Retention.IsEnabled() && cleaner != nil:
			cleaner.Stop()
			cleaner = nil
			logger.Info("main", "历史数据清理任务已在热更新后停用")
		}

		// === Archiver: ApplyConfig（在线更新配置 + 唤醒调度） ===
		switch {
		case newCfg.Storage.Archive.IsEnabled() && archiver == nil:
			if _, ok := store.(storage.ArchiveStorage); !ok {
				logger.Warn("main", "热更新后启用了归档功能，但当前存储不支持（仅 PostgreSQL 支持）",
					"storage_type", newCfg.Storage.Type)
			} else {
				archiver = storage.NewArchiver(store, &newCfg.Storage.Archive)
				go archiver.Start(ctx)
				logger.Info("main", "历史数据归档任务已在热更新后启动",
					"archive_days", newCfg.Storage.Archive.ArchiveDays,
					"backfill_days", newCfg.Storage.Archive.BackfillDays,
					"output_dir", newCfg.Storage.Archive.OutputDir,
					"format", newCfg.Storage.Archive.Format)
			}
		case newCfg.Storage.Archive.IsEnabled() && archiver != nil:
			archiver.UpdateArchiveConfig(&newCfg.Storage.Archive)
		case !newCfg.Storage.Archive.IsEnabled() && archiver != nil:
			archiver.Stop()
			archiver = nil
			logger.Info("main", "历史数据归档任务已在热更新后停用")
		}

		// === SelfTest: RecreateOnChange（drain + 重建） ===
		if selfTestConfigChanged(selfTestAppliedCfg, newCfg) {
			if selfTestMgr != nil {
				selfTestMgr.StopWithDrain(10 * time.Second)
			}

			if newCfg.SelfTest.Enabled {
				selfTestMgr = newSelfTestManager(newCfg, configFile)
				server.GetHandler().SetSelfTestManager(selfTestMgr)
				logger.Info("main", "自助测试配置热更新已生效",
					"max_concurrent", newCfg.SelfTest.MaxConcurrent,
					"max_queue_size", newCfg.SelfTest.MaxQueueSize,
					"job_timeout", newCfg.SelfTest.JobTimeoutDuration,
					"result_ttl", newCfg.SelfTest.ResultTTLDuration,
					"rate_limit", newCfg.SelfTest.RateLimitPerMinute)
			} else {
				selfTestMgr = nil
				server.GetHandler().SetSelfTestManager(nil)
				logger.Info("main", "自助测试功能已在热更新后停用")
			}
			selfTestAppliedCfg = newCfg
		}

		// === Announcements: RecreateOnChange（stop + 重建） ===
		if announcementsConfigChanged(announcementsAppliedCfg, newCfg) {
			if !newCfg.Announcements.IsEnabled() {
				announcementsHandler.SetService(nil)
				if announcementsSvc != nil {
					announcementsSvc.Stop()
					announcementsSvc = nil
					logger.Info("main", "公告服务已在热更新后停用")
				}
				announcementsAppliedCfg = newCfg
			} else {
				newSvc, err := announcements.NewService(newCfg.Announcements, newCfg.GitHub)
				if err != nil {
					logger.Error("main", "热更新创建公告服务失败，继续使用旧实例", "error", err)
				} else {
					// 先启动新实例，再切换引用，最后停止旧实例（减少不可用窗口）
					newSvc.Start(ctx)
					oldSvc := announcementsSvc
					announcementsSvc = newSvc
					announcementsHandler.SetService(newSvc)
					if oldSvc != nil {
						oldSvc.Stop()
					}
					announcementsAppliedCfg = newCfg
					logger.Info("main", "公告服务配置热更新已生效",
						"owner", newCfg.Announcements.Owner,
						"repo", newCfg.Announcements.Repo,
						"category", newCfg.Announcements.CategoryName,
						"poll_interval", newCfg.Announcements.PollInterval,
						"window_hours", newCfg.Announcements.WindowHours)
				}
			}
		}
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

	// 停止自动移板服务
	autoMover.Stop()
	logger.Info("main", "自动移板服务已关闭")

	// 在 runtimeMu 保护下捕获当前实例引用，防止与热更新回调竞态
	runtimeMu.Lock()
	currentSelfTestMgr := selfTestMgr
	selfTestMgr = nil
	currentAnnouncementsSvc := announcementsSvc
	announcementsSvc = nil
	currentCleaner := cleaner
	cleaner = nil
	currentArchiver := archiver
	archiver = nil
	// 清空 handler 引用，避免关闭后的请求访问已销毁的实例
	server.GetHandler().SetSelfTestManager(nil)
	announcementsHandler.SetService(nil)
	runtimeMu.Unlock()

	// 停止自助测试管理器（如果启用）
	if currentSelfTestMgr != nil {
		currentSelfTestMgr.Stop()
		logger.Info("main", "自助测试管理器已关闭")
	}

	// 停止公告服务（如果启用）
	if currentAnnouncementsSvc != nil {
		currentAnnouncementsSvc.Stop()
		logger.Info("main", "公告服务已关闭")
	}

	// 停止清理和归档任务
	if currentCleaner != nil {
		currentCleaner.Stop()
		logger.Info("main", "历史数据清理任务已关闭")
	}
	if currentArchiver != nil {
		currentArchiver.Stop()
		logger.Info("main", "历史数据归档任务已关闭")
	}

	// 停止HTTP服务器
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Stop(shutdownCtx); err != nil {
		logger.Warn("main", "HTTP服务器关闭错误", "error", err)
	}

	logger.Info("main", "服务已安全退出")
}
