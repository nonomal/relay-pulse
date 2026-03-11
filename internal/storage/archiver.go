package storage

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"monitor/internal/config"
	"monitor/internal/logger"
)

// Archiver 历史数据归档任务
// 负责将过期数据导出到文件（CSV.gz），用于备份
type Archiver struct {
	storage     Storage
	cfgMu       sync.RWMutex
	config      *config.ArchiveConfig
	running     atomic.Bool
	stopCh      chan struct{}
	reloadCh    chan struct{} // 配置热更新时唤醒挂起的调度 timer
	lifecycleMu sync.Mutex    // 保护 started/stoppedCh，确保 Stop() 阻塞等待 goroutine 退出
	started     bool
	stoppedCh   chan struct{}
	stopOnce    sync.Once
}

// NewArchiver 创建归档任务
func NewArchiver(storage Storage, cfg *config.ArchiveConfig) *Archiver {
	return &Archiver{
		storage:  storage,
		config:   cfg,
		stopCh:   make(chan struct{}),
		reloadCh: make(chan struct{}, 1),
	}
}

// currentConfig 获取当前配置的值拷贝（并发安全）
func (a *Archiver) currentConfig() config.ArchiveConfig {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	if a.config == nil {
		return config.ArchiveConfig{}
	}
	return *a.config
}

// UpdateArchiveConfig 热更新 archive 配置，并唤醒调度循环重新计算下次归档时间
func (a *Archiver) UpdateArchiveConfig(cfg *config.ArchiveConfig) {
	if cfg == nil {
		return
	}
	a.cfgMu.Lock()
	a.config = cfg
	a.cfgMu.Unlock()

	// 非阻塞通知：唤醒 Start 中挂起的 timer
	select {
	case a.reloadCh <- struct{}{}:
	default:
	}
	logger.Info("archiver", "archive 配置已更新",
		"archive_days", cfg.ArchiveDays,
		"output_dir", cfg.OutputDir)
}

// beginLoop 标记 Start goroutine 已进入主循环（防止并发启动）
func (a *Archiver) beginLoop() bool {
	a.lifecycleMu.Lock()
	defer a.lifecycleMu.Unlock()
	if a.started {
		return false
	}
	a.started = true
	a.stoppedCh = make(chan struct{})
	return true
}

// endLoop 标记 Start goroutine 已退出
func (a *Archiver) endLoop() {
	a.lifecycleMu.Lock()
	ch := a.stoppedCh
	a.stoppedCh = nil
	a.started = false
	a.lifecycleMu.Unlock()
	if ch != nil {
		close(ch)
	}
}

// Start 启动归档任务（阻塞，应在 goroutine 中调用）
// 每天凌晨执行一次归档
func (a *Archiver) Start(ctx context.Context) {
	if !a.beginLoop() {
		return
	}
	defer a.endLoop()

	cfg := a.currentConfig()
	if !cfg.IsEnabled() {
		logger.Info("archiver", "数据归档已禁用")
		return
	}

	// 检查存储是否支持归档
	archiveStorage, ok := a.storage.(ArchiveStorage)
	if !ok {
		logger.Warn("archiver", "当前存储不支持归档功能（仅 PostgreSQL 支持）")
		return
	}

	// 确保输出目录存在
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		logger.Error("archiver", "创建归档目录失败", "error", err, "dir", cfg.OutputDir)
		return
	}

	logger.Info("archiver", "归档任务已启动",
		"output_dir", cfg.OutputDir,
		"archive_days", cfg.ArchiveDays,
		"backfill_days", cfg.BackfillDays,
		"format", cfg.Format)

	// 首次立即尝试归档
	a.runArchive(ctx, archiveStorage)

	for {
		// 每轮循环重新读取配置（支持热更新后生效）
		cfg = a.currentConfig()
		if !cfg.IsEnabled() {
			logger.Info("archiver", "归档任务已禁用，停止调度")
			return
		}

		nextRun := a.nextArchiveTimeFor(cfg)
		waitDuration := time.Until(nextRun)
		if waitDuration < 0 {
			waitDuration = 0
		}

		logger.Info("archiver", "下次归档时间",
			"next_run", nextRun.Format(time.RFC3339),
			"schedule_hour_utc", scheduleHourUTC(cfg),
			"wait", waitDuration)

		timer := time.NewTimer(waitDuration)
		select {
		case <-timer.C:
			a.runArchive(ctx, archiveStorage)
		case <-a.reloadCh:
			drainTimer(timer)
			logger.Info("archiver", "归档配置已变更，重新计算下次归档时间")
		case <-ctx.Done():
			drainTimer(timer)
			logger.Info("archiver", "归档任务收到取消信号，正在退出")
			return
		case <-a.stopCh:
			drainTimer(timer)
			logger.Info("archiver", "归档任务收到停止信号，正在退出")
			return
		}
	}
}

// Stop 停止归档任务并等待 goroutine 退出（幂等，可重复调用）
func (a *Archiver) Stop() {
	a.stopOnce.Do(func() {
		close(a.stopCh)
	})
	// 等待 Start goroutine 退出，避免 Stop 后立即创建新实例导致并发
	a.lifecycleMu.Lock()
	ch := a.stoppedCh
	a.lifecycleMu.Unlock()
	if ch != nil {
		<-ch
	}
}

// nextArchiveTimeFor 计算下次归档时间（每天在配置的 UTC 小时执行，默认 3）
func (a *Archiver) nextArchiveTimeFor(cfg config.ArchiveConfig) time.Time {
	now := time.Now().UTC()
	hour := scheduleHourUTC(cfg)
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, time.UTC)
	if now.After(next) {
		next = next.Add(24 * time.Hour)
	}
	return next
}

// scheduleHourUTC 返回配置的归档执行时间（UTC 小时），默认 3
func scheduleHourUTC(cfg config.ArchiveConfig) int {
	if cfg.ScheduleHour != nil {
		return *cfg.ScheduleHour
	}
	return 3
}

// runArchive 执行一轮归档
// 支持历史日期补齐：每次运行会尝试补齐 backfill_days 窗口内缺失的归档文件
func (a *Archiver) runArchive(ctx context.Context, archiveStorage ArchiveStorage) {
	// 防止重入
	if !a.running.CompareAndSwap(false, true) {
		logger.Info("archiver", "归档任务仍在运行，跳过本轮")
		return
	}
	defer a.running.Store(false)

	cfg := a.currentConfig()
	if !cfg.IsEnabled() {
		return
	}

	// 确保输出目录存在（配置可能已变更）
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		logger.Error("archiver", "创建归档目录失败", "error", err, "dir", cfg.OutputDir)
		return
	}

	now := time.Now().UTC()
	todayStart := now.Truncate(24 * time.Hour)

	// 计算归档日期范围
	// latestArchiveDate: 最新可归档日期（今天 - archive_days）
	// earliestArchiveDate: 最早回溯日期（latestArchiveDate - backfill_days + 1）
	latestArchiveDate := todayStart.AddDate(0, 0, -cfg.ArchiveDays)
	backfillDays := cfg.BackfillDays
	if backfillDays < 1 {
		backfillDays = 1
	}
	earliestArchiveDate := latestArchiveDate.AddDate(0, 0, -(backfillDays - 1))

	logger.Info("archiver", "本轮归档计划",
		"earliest_date", earliestArchiveDate.Format("2006-01-02"),
		"latest_date", latestArchiveDate.Format("2006-01-02"),
		"archive_days", cfg.ArchiveDays,
		"backfill_days", backfillDays)

	// 遍历日期范围，逐日补齐缺失的归档
	for d := earliestArchiveDate; !d.After(latestArchiveDate); d = d.AddDate(0, 0, 1) {
		select {
		case <-ctx.Done():
			logger.Info("archiver", "归档任务被取消，正在退出")
			return
		case <-a.stopCh:
			logger.Info("archiver", "归档任务收到停止信号，正在退出")
			return
		default:
		}

		if err := a.archiveOneDay(ctx, archiveStorage, cfg, d); err != nil {
			// 优雅关闭时 context 被取消，降级为 Info 避免噪声
			if ctx.Err() != nil {
				logger.Info("archiver", "归档任务被取消", "date", d.Format("2006-01-02"))
				return
			}
			logger.Error("archiver", "归档任务失败", "error", err, "date", d.Format("2006-01-02"))
			// 继续尝试下一天，不中断整个归档流程
		}
	}

	// 清理过期的归档文件
	a.cleanupOldArchives(cfg)
}

// archiveOneDay 归档单日数据
func (a *Archiver) archiveOneDay(ctx context.Context, archiveStorage ArchiveStorage, cfg config.ArchiveConfig, archiveDate time.Time) error {
	archiveDate = archiveDate.UTC().Truncate(24 * time.Hour)
	dateStr := archiveDate.Format("2006-01-02")

	// 生成文件名
	filename := fmt.Sprintf("probe_history_%s.csv", dateStr)
	if cfg.Format == "csv.gz" {
		filename += ".gz"
	}
	fullPath := filepath.Join(cfg.OutputDir, filename)

	// 检查是否已归档
	if _, err := os.Stat(fullPath); err == nil {
		logger.Info("archiver", "该日期已归档，跳过",
			"date", dateStr,
			"file", filename)
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("检查归档文件失败: %w", err)
	}

	// 计算时间范围
	dayStart := archiveDate.Unix()
	dayEnd := archiveDate.Add(24 * time.Hour).Unix()

	startTime := time.Now()
	logger.Info("archiver", "开始归档",
		"date", dateStr,
		"file", filename)

	// 创建临时文件（使用 CreateTemp 避免文件名冲突）
	file, err := os.CreateTemp(cfg.OutputDir, filename+".tmp.*")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := file.Name()

	// 设置文件权限为 0644（CreateTemp 默认 0600，可能导致外部备份进程无法读取）
	if err := file.Chmod(0644); err != nil {
		file.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("设置文件权限失败: %w", err)
	}

	var writer io.Writer = file
	var gzipWriter *gzip.Writer

	// 如果需要压缩
	if cfg.Format == "csv.gz" {
		gzipWriter = gzip.NewWriter(file)
		writer = gzipWriter
	}

	// 同时计算校验和
	hash := sha256.New()
	writer = io.MultiWriter(writer, hash)

	// 导出数据
	rowCount, err := archiveStorage.ExportDayToWriter(ctx, dayStart, dayEnd, writer)
	if err != nil {
		if gzipWriter != nil {
			gzipWriter.Close()
		}
		file.Close()
		os.Remove(tmpPath)

		// 处理 advisory lock 冲突：其他实例正在归档该日期
		if errors.Is(err, ErrArchiveAdvisoryLockNotAcquired) {
			logger.Info("archiver", "其他实例正在归档该日期，跳过", "date", dateStr)
			return nil
		}

		return fmt.Errorf("导出数据失败: %w", err)
	}

	// 关闭 gzip writer（必须在 file.Close 之前）
	if gzipWriter != nil {
		if err := gzipWriter.Close(); err != nil {
			file.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("关闭 gzip writer 失败: %w", err)
		}
	}

	// 关闭文件
	if err := file.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("关闭文件失败: %w", err)
	}

	// 兜底检查：防止在重命名窗口被其他进程写入（例如混跑旧版本时）
	if _, err := os.Stat(fullPath); err == nil {
		os.Remove(tmpPath)
		logger.Info("archiver", "该日期已归档，跳过（可能由其他实例完成）",
			"date", dateStr,
			"file", filename)
		return nil
	} else if !os.IsNotExist(err) {
		os.Remove(tmpPath)
		return fmt.Errorf("检查归档文件失败: %w", err)
	}

	// 重命名临时文件为最终文件
	if err := os.Rename(tmpPath, fullPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("重命名文件失败: %w", err)
	}

	// 获取文件大小
	fileInfo, _ := os.Stat(fullPath)
	fileSize := int64(0)
	if fileInfo != nil {
		fileSize = fileInfo.Size()
	}

	checksum := hex.EncodeToString(hash.Sum(nil))
	elapsed := time.Since(startTime)

	logger.Info("archiver", "归档完成",
		"date", dateStr,
		"file", filename,
		"rows", rowCount,
		"size_bytes", fileSize,
		"checksum_sha256", checksum[:16]+"...", // 只显示前 16 字符
		"elapsed", elapsed)

	return nil
}

// cleanupOldArchives 清理过期的归档文件
func (a *Archiver) cleanupOldArchives(cfg config.ArchiveConfig) {
	if cfg.KeepDaysValue <= 0 {
		return // -1 表示永久保留，0 不应该出现
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -cfg.KeepDaysValue)

	entries, err := os.ReadDir(cfg.OutputDir)
	if err != nil {
		logger.Warn("archiver", "读取归档目录失败", "error", err)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// 只处理归档文件（probe_history_*.csv 或 probe_history_*.csv.gz）
		name := entry.Name()
		if !strings.HasPrefix(name, "probe_history_") {
			continue
		}
		if !strings.HasSuffix(name, ".csv") && !strings.HasSuffix(name, ".csv.gz") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			fullPath := filepath.Join(cfg.OutputDir, name)
			if err := os.Remove(fullPath); err != nil {
				logger.Warn("archiver", "删除过期归档失败", "error", err, "file", name)
			} else {
				logger.Info("archiver", "已删除过期归档", "file", name)
			}
		}
	}
}
