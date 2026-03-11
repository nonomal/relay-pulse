package storage

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"monitor/internal/config"
	"monitor/internal/logger"
)

// Cleaner 历史数据清理任务调度器
// 负责定期清理过期的探测记录，避免数据库无限增长
type Cleaner struct {
	storage     RetentionStorage
	cfgMu       sync.RWMutex
	config      *config.RetentionConfig
	running     atomic.Bool
	stopCh      chan struct{}
	reloadCh    chan struct{} // 配置热更新时唤醒挂起的调度 timer
	lifecycleMu sync.Mutex    // 保护 started/stoppedCh，确保 Stop() 阻塞等待 goroutine 退出
	started     bool
	stoppedCh   chan struct{}
	stopOnce    sync.Once
}

// NewCleaner 创建清理任务调度器
func NewCleaner(storage RetentionStorage, cfg *config.RetentionConfig) *Cleaner {
	return &Cleaner{
		storage:  storage,
		config:   cfg,
		stopCh:   make(chan struct{}),
		reloadCh: make(chan struct{}, 1),
	}
}

// currentConfig 获取当前配置的值拷贝（并发安全）
func (c *Cleaner) currentConfig() config.RetentionConfig {
	c.cfgMu.RLock()
	defer c.cfgMu.RUnlock()
	if c.config == nil {
		return config.RetentionConfig{}
	}
	return *c.config
}

// UpdateRetentionConfig 热更新 retention 配置，并唤醒调度循环重新计算下次清理时间
func (c *Cleaner) UpdateRetentionConfig(cfg *config.RetentionConfig) {
	if cfg == nil {
		return
	}
	c.cfgMu.Lock()
	c.config = cfg
	c.cfgMu.Unlock()

	// 非阻塞通知：唤醒 Start 中挂起的 timer
	select {
	case c.reloadCh <- struct{}{}:
	default:
	}
	logger.Info("cleaner", "retention 配置已更新",
		"days", cfg.Days,
		"cleanup_interval", cfg.CleanupInterval)
}

// beginLoop 标记 Start goroutine 已进入主循环（防止并发启动）
func (c *Cleaner) beginLoop() bool {
	c.lifecycleMu.Lock()
	defer c.lifecycleMu.Unlock()
	if c.started {
		return false
	}
	c.started = true
	c.stoppedCh = make(chan struct{})
	return true
}

// endLoop 标记 Start goroutine 已退出
func (c *Cleaner) endLoop() {
	c.lifecycleMu.Lock()
	ch := c.stoppedCh
	c.stoppedCh = nil
	c.started = false
	c.lifecycleMu.Unlock()
	if ch != nil {
		close(ch)
	}
}

// Start 启动清理任务（阻塞，应在 goroutine 中调用）
func (c *Cleaner) Start(ctx context.Context) {
	if !c.beginLoop() {
		return
	}
	defer c.endLoop()

	cfg := c.currentConfig()
	if !cfg.IsEnabled() {
		logger.Info("cleaner", "数据清理已禁用")
		return
	}

	startup := true
	for {
		// 每轮循环重新读取配置（支持热更新后生效）
		cfg = c.currentConfig()
		if !cfg.IsEnabled() {
			logger.Info("cleaner", "清理任务已禁用，停止调度")
			return
		}

		wait := applyJitter(cfg.CleanupIntervalDuration, cfg.Jitter)
		if startup {
			wait = applyJitter(cfg.StartupDelayDuration, cfg.Jitter)
			logger.Info("cleaner", "清理任务将在延迟后启动",
				"delay", wait,
				"retention_days", cfg.Days,
				"cleanup_interval", cfg.CleanupIntervalDuration)
		}

		timer := time.NewTimer(wait)
		select {
		case <-timer.C:
			c.runCleanup(ctx)
			startup = false
		case <-c.reloadCh:
			drainTimer(timer)
			logger.Info("cleaner", "清理配置已变更，重新计算调度")
			// 下一轮循环将读取新配置
		case <-ctx.Done():
			drainTimer(timer)
			logger.Info("cleaner", "清理任务收到取消信号，正在退出")
			return
		case <-c.stopCh:
			drainTimer(timer)
			logger.Info("cleaner", "清理任务收到停止信号，正在退出")
			return
		}
	}
}

// Stop 停止清理任务并等待 goroutine 退出（幂等，可重复调用）
func (c *Cleaner) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	// 等待 Start goroutine 退出，避免 Stop 后立即创建新实例导致并发
	c.lifecycleMu.Lock()
	ch := c.stoppedCh
	c.lifecycleMu.Unlock()
	if ch != nil {
		<-ch
	}
}

// runCleanup 执行一轮清理
func (c *Cleaner) runCleanup(ctx context.Context) {
	// 防止重入
	if !c.running.CompareAndSwap(false, true) {
		logger.Info("cleaner", "清理任务仍在运行，跳过本轮")
		return
	}
	defer c.running.Store(false)

	cfg := c.currentConfig()
	if !cfg.IsEnabled() {
		return
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -cfg.Days)
	startTime := time.Now()
	var totalDeleted int64
	batchCount := 0
	backoff := 50 * time.Millisecond

	for batchCount < cfg.MaxBatchesPerRun {
		select {
		case <-ctx.Done():
			logger.Info("cleaner", "清理任务被取消",
				"deleted", totalDeleted,
				"batches", batchCount)
			return
		case <-c.stopCh:
			logger.Info("cleaner", "清理任务收到停止信号",
				"deleted", totalDeleted,
				"batches", batchCount)
			return
		default:
		}

		deleted, err := c.storage.PurgeOldRecords(ctx, cutoff, cfg.BatchSize)
		if err != nil {
			// 优雅关闭时 context 被取消，降级为 Info 避免噪声
			if ctx.Err() != nil {
				logger.Info("cleaner", "清理任务被取消",
					"deleted", totalDeleted,
					"batches", batchCount)
				return
			}

			// SQLite 锁冲突时指数退避重试（使用类型断言而非字符串匹配）
			var sqliteErr *sqlite.Error
			if errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3.SQLITE_BUSY {
				logger.Warn("cleaner", "数据库锁冲突，等待重试",
					"backoff", backoff,
					"deleted_so_far", totalDeleted)
				retryTimer := time.NewTimer(backoff)
				select {
				case <-retryTimer.C:
				case <-ctx.Done():
					drainTimer(retryTimer)
					logger.Info("cleaner", "清理任务被取消",
						"deleted", totalDeleted,
						"batches", batchCount)
					return
				case <-c.stopCh:
					drainTimer(retryTimer)
					logger.Info("cleaner", "清理任务收到停止信号",
						"deleted", totalDeleted,
						"batches", batchCount)
					return
				}
				backoff = min(backoff*2, 5*time.Second)
				continue
			}

			logger.Error("cleaner", "清理任务失败",
				"error", err,
				"deleted", totalDeleted)
			return
		}

		backoff = 50 * time.Millisecond // 重置退避
		totalDeleted += deleted
		batchCount++

		if deleted < int64(cfg.BatchSize) {
			break // 没有更多数据
		}
	}

	elapsed := time.Since(startTime)
	if totalDeleted > 0 {
		logger.Info("cleaner", "历史数据清理完成",
			"deleted", totalDeleted,
			"batches", batchCount,
			"elapsed", elapsed,
			"cutoff", cutoff.Format(time.RFC3339))
	}
}

// applyJitter 对 base 时长施加随机抖动
func applyJitter(base time.Duration, ratio float64) time.Duration {
	if base <= 0 || ratio <= 0 {
		if base < 0 {
			return 0
		}
		return base
	}
	jitter := time.Duration(float64(base) * ratio * (rand.Float64()*2 - 1))
	result := base + jitter
	if result < 0 {
		return 0
	}
	return result
}

// drainTimer 安全停止并排空 timer
func drainTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}
