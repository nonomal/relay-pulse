package config

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"monitor/internal/logger"
)

const (
	// defaultDBWatchInterval 默认数据库轮询间隔
	defaultDBWatchInterval = 10 * time.Second

	// maxConsistencyRetries 版本一致性校验最大重试次数
	maxConsistencyRetries = 3
)

// DBWatcher 数据库配置监听器
// 通过轮询 config_meta 版本号触发数据库配置热更新
// 适用于 config_source=database 模式
type DBWatcher struct {
	loader   *Loader
	onReload func(*AppConfig)

	interval    time.Duration
	jitterRatio float64 // 抖动比例（0-1），用于打散多实例轮询
	rng         *rand.Rand

	mu              sync.RWMutex // 保护 lastVersions 和 baselineOK
	lastVersions    *ConfigVersionsRecord
	baselineOK      bool // 标记基线是否成功建立
	forceReloadNext bool // 基线失败后，下次成功读取时强制重载

	startOnce sync.Once
	stopOnce  sync.Once
	stopCh    chan struct{}
}

// DBWatcherOption 配置选项函数
type DBWatcherOption func(*DBWatcher)

// WithDBWatchInterval 设置轮询间隔
func WithDBWatchInterval(interval time.Duration) DBWatcherOption {
	return func(w *DBWatcher) {
		if interval > 0 {
			w.interval = interval
		}
	}
}

// WithDBWatchJitter 设置抖动比例（0-1）
// 例如 0.2 表示在基准间隔 ±20% 范围内随机抖动
func WithDBWatchJitter(ratio float64) DBWatcherOption {
	return func(w *DBWatcher) {
		if ratio >= 0 && ratio <= 1 {
			w.jitterRatio = ratio
		}
	}
}

// NewDBWatcher 创建数据库配置监听器
func NewDBWatcher(loader *Loader, onReload func(*AppConfig), opts ...DBWatcherOption) *DBWatcher {
	w := &DBWatcher{
		loader:      loader,
		onReload:    onReload,
		interval:    defaultDBWatchInterval,
		jitterRatio: 0.1, // 默认 ±10% 抖动
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
		stopCh:      make(chan struct{}),
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// Start 启动数据库配置轮询
// 通过 ctx 控制生命周期，ctx 取消时自动停止
func (w *DBWatcher) Start(ctx context.Context) error {
	if err := w.validate(); err != nil {
		return err
	}

	w.startOnce.Do(func() {
		go w.loop(ctx)
	})

	return nil
}

// Stop 停止轮询（幂等）
func (w *DBWatcher) Stop() {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
}

// loop 轮询主循环
func (w *DBWatcher) loop(ctx context.Context) {
	logger.Info("config", "数据库配置轮询已启动",
		"interval", w.interval,
		"jitter", w.jitterRatio)

	// 首次读取用于建立版本基线
	w.establishBaseline()

	for {
		interval := w.nextInterval()
		select {
		case <-time.After(interval):
			w.pollOnce(ctx)
		case <-ctx.Done():
			logger.Info("config", "数据库配置轮询已停止（context 取消）")
			return
		case <-w.stopCh:
			logger.Info("config", "数据库配置轮询已停止")
			return
		}
	}
}

// establishBaseline 建立版本基线
func (w *DBWatcher) establishBaseline() {
	versions, err := w.readVersions()
	if err != nil {
		logger.Error("config", "建立数据库配置版本基线失败", "error", err)
		w.mu.Lock()
		w.forceReloadNext = true // 基线失败，下次成功时需要重载
		w.mu.Unlock()
		return
	}

	if versions == nil {
		logger.Warn("config", "数据库配置版本为空，跳过基线建立")
		w.mu.Lock()
		w.forceReloadNext = true
		w.mu.Unlock()
		return
	}

	w.mu.Lock()
	w.lastVersions = cloneVersions(versions)
	w.baselineOK = true
	w.forceReloadNext = false
	w.mu.Unlock()

	logger.Info("config", "数据库配置版本基线已建立",
		"monitors", versions.Monitors,
		"policies", versions.Policies,
		"badges", versions.Badges,
		"boards", versions.Boards,
		"settings", versions.Settings)
}

// pollOnce 执行一次轮询
func (w *DBWatcher) pollOnce(ctx context.Context) {
	versions, err := w.readVersions()
	if err != nil {
		logger.Error("config", "读取数据库配置版本失败", "error", err)
		return
	}

	if versions == nil {
		logger.Warn("config", "数据库配置版本为空，跳过本次轮询")
		return
	}

	w.mu.Lock()
	// 基线失败后首次成功读取，触发强制重载
	if w.forceReloadNext {
		w.lastVersions = cloneVersions(versions)
		w.baselineOK = true
		w.forceReloadNext = false
		w.mu.Unlock()
		logger.Info("config", "基线恢复成功，触发配置重载",
			"monitors", versions.Monitors,
			"policies", versions.Policies)
		w.reloadWithConsistency(ctx, versions)
		return
	}

	// 尚未建立基线（正常路径）
	if w.lastVersions == nil {
		w.lastVersions = cloneVersions(versions)
		w.baselineOK = true
		w.mu.Unlock()
		return
	}

	lastVersions := cloneVersions(w.lastVersions)
	w.mu.Unlock()

	// 版本未变化
	if versionsEqual(lastVersions, versions) {
		return
	}

	logger.Info("config", "检测到数据库配置版本变化，准备重载",
		"monitors", versionDiff(lastVersions.Monitors, versions.Monitors),
		"policies", versionDiff(lastVersions.Policies, versions.Policies),
		"badges", versionDiff(lastVersions.Badges, versions.Badges),
		"boards", versionDiff(lastVersions.Boards, versions.Boards),
		"settings", versionDiff(lastVersions.Settings, versions.Settings))

	w.reloadWithConsistency(ctx, versions)
}

// reloadWithConsistency 带版本一致性校验的重载
// 采用"两次读取"策略：读版本 → 读配置 → 再读版本 → 对比
// 若版本不一致（加载过程中发生了写入），最多重试 maxConsistencyRetries 次
//
// 注意：LoadFromDatabaseOnly 在最后一行才设置 currentConfig（原子指针赋值），
// 所以失败时不需要显式回滚。只有成功但版本不一致时才需要恢复旧配置。
func (w *DBWatcher) reloadWithConsistency(ctx context.Context, beforeVersions *ConfigVersionsRecord) {
	// 保存旧配置的引用（用于版本不一致时恢复）
	oldConfig := w.loader.currentConfig

	for attempt := 1; attempt <= maxConsistencyRetries; attempt++ {
		// 加载数据库配置
		// LoadFromDatabaseOnly 只在最后成功时才设置 currentConfig
		cfg, err := w.loader.LoadFromDatabaseOnly(ctx)
		if err != nil {
			// 加载失败，currentConfig 未被修改，无需回滚
			logger.Error("config", "数据库配置重载失败，保持当前配置",
				"error", err, "attempt", attempt)
			return
		}

		// 此时 loader.currentConfig 已经是新配置 cfg

		// 二次读取版本号
		afterVersions, err := w.readVersions()
		if err != nil {
			// 读取失败，恢复旧配置
			logger.Error("config", "二次读取配置版本失败，保持当前配置",
				"error", err, "attempt", attempt)
			w.loader.currentConfig = oldConfig
			return
		}

		// 校验一致性
		if versionsEqual(beforeVersions, afterVersions) {
			// 一致，更新成功
			w.mu.Lock()
			w.lastVersions = cloneVersions(afterVersions)
			w.mu.Unlock()

			logger.Info("config", "数据库配置热更新成功",
				"monitors", len(cfg.Monitors), "attempt", attempt)
			if w.onReload != nil {
				w.onReload(cfg)
			}
			return
		}

		// 不一致，恢复旧配置并使用最新版本号重试
		logger.Warn("config", "数据库配置版本在加载过程中发生变化，重试",
			"attempt", attempt, "max_retries", maxConsistencyRetries)
		w.loader.currentConfig = oldConfig
		beforeVersions = afterVersions // 使用最新版本号重试
	}

	// 所有重试都不一致，放弃本次更新（oldConfig 已在最后一次循环中恢复）
	logger.Warn("config", "数据库配置版本持续变化，放弃本次更新，保持当前配置",
		"retries", maxConsistencyRetries)
}

// readVersions 从数据库读取配置版本号
func (w *DBWatcher) readVersions() (*ConfigVersionsRecord, error) {
	if w.loader == nil || w.loader.dbProvider == nil {
		return nil, errors.New("数据库配置提供者未初始化")
	}
	return w.loader.dbProvider.storage.GetConfigVersions()
}

// nextInterval 计算下次轮询间隔（含抖动）
func (w *DBWatcher) nextInterval() time.Duration {
	if w.jitterRatio <= 0 {
		return w.interval
	}

	// 在 [interval*(1-jitter), interval*(1+jitter)] 范围内随机
	jitter := time.Duration(float64(w.interval) * w.jitterRatio * (w.rng.Float64()*2 - 1))
	next := w.interval + jitter
	if next <= 0 {
		return w.interval
	}
	return next
}

// validate 验证监听器配置
func (w *DBWatcher) validate() error {
	if w.loader == nil {
		return errors.New("配置加载器为空")
	}
	if w.loader.dbProvider == nil {
		return errors.New("数据库配置提供者为空")
	}
	if w.loader.currentConfig == nil {
		return errors.New("当前配置为空，无法进行热更新")
	}
	return nil
}

// ===== 辅助函数 =====

// versionsEqual 比较两个版本号记录是否相等
func versionsEqual(a, b *ConfigVersionsRecord) bool {
	if a == nil || b == nil {
		return false
	}
	return a.Monitors == b.Monitors &&
		a.Policies == b.Policies &&
		a.Badges == b.Badges &&
		a.Boards == b.Boards &&
		a.Settings == b.Settings
}

// cloneVersions 复制版本号记录
func cloneVersions(v *ConfigVersionsRecord) *ConfigVersionsRecord {
	if v == nil {
		return nil
	}
	c := *v
	return &c
}

// versionDiff 格式化版本号差异
func versionDiff(before, after int64) string {
	if before == after {
		return "unchanged"
	}
	return fmt.Sprintf("%d→%d", before, after)
}
