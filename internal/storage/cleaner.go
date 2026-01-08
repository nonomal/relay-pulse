package storage

import (
	"context"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"monitor/internal/config"
	"monitor/internal/logger"
)

// Cleaner 历史数据清理任务调度器
// 负责定期清理过期的探测记录，避免数据库无限增长
type Cleaner struct {
	storage  Storage
	config   *config.RetentionConfig
	running  atomic.Bool
	stopCh   chan struct{}
	stopOnce sync.Once
}

// NewCleaner 创建清理任务调度器
func NewCleaner(storage Storage, cfg *config.RetentionConfig) *Cleaner {
	return &Cleaner{
		storage: storage,
		config:  cfg,
		stopCh:  make(chan struct{}),
	}
}

// Start 启动清理任务（阻塞，应在 goroutine 中调用）
func (c *Cleaner) Start(ctx context.Context) {
	if !c.config.IsEnabled() {
		logger.Info("cleaner", "数据清理已禁用")
		return
	}

	// 启动延迟 + jitter
	delay := c.config.StartupDelayDuration
	if c.config.Jitter > 0 {
		jitter := time.Duration(float64(delay) * c.config.Jitter * (rand.Float64()*2 - 1))
		delay += jitter
	}
	logger.Info("cleaner", "清理任务将在延迟后启动",
		"delay", delay,
		"retention_days", c.config.Days,
		"cleanup_interval", c.config.CleanupIntervalDuration)

	select {
	case <-time.After(delay):
	case <-ctx.Done():
		return
	case <-c.stopCh:
		return
	}

	// 首次立即执行一次
	c.runCleanup(ctx)

	// 定时执行 + jitter
	for {
		interval := c.config.CleanupIntervalDuration
		if c.config.Jitter > 0 {
			jitter := time.Duration(float64(interval) * c.config.Jitter * (rand.Float64()*2 - 1))
			interval += jitter
		}

		select {
		case <-time.After(interval):
			c.runCleanup(ctx)
		case <-ctx.Done():
			logger.Info("cleaner", "清理任务收到取消信号，正在退出")
			return
		case <-c.stopCh:
			logger.Info("cleaner", "清理任务收到停止信号，正在退出")
			return
		}
	}
}

// Stop 停止清理任务（幂等，可重复调用）
func (c *Cleaner) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
}

// runCleanup 执行一轮清理
func (c *Cleaner) runCleanup(ctx context.Context) {
	// 防止重入
	if !c.running.CompareAndSwap(false, true) {
		logger.Info("cleaner", "清理任务仍在运行，跳过本轮")
		return
	}
	defer c.running.Store(false)

	cutoff := time.Now().UTC().AddDate(0, 0, -c.config.Days)
	startTime := time.Now()
	var totalDeleted int64
	batchCount := 0
	backoff := 50 * time.Millisecond

	for batchCount < c.config.MaxBatchesPerRun {
		select {
		case <-ctx.Done():
			logger.Info("cleaner", "清理任务被取消",
				"deleted", totalDeleted,
				"batches", batchCount)
			return
		default:
		}

		deleted, err := c.storage.PurgeOldRecords(ctx, cutoff, c.config.BatchSize)
		if err != nil {
			// 优雅关闭时 context 被取消，降级为 Info 避免噪声
			if ctx.Err() != nil {
				logger.Info("cleaner", "清理任务被取消",
					"deleted", totalDeleted,
					"batches", batchCount)
				return
			}

			// SQLite 锁冲突时指数退避重试
			if strings.Contains(err.Error(), "database is locked") {
				logger.Warn("cleaner", "数据库锁冲突，等待重试",
					"backoff", backoff,
					"deleted_so_far", totalDeleted)
				time.Sleep(backoff)
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

		if deleted < int64(c.config.BatchSize) {
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
