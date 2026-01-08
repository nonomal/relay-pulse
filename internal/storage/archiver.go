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
	storage  Storage
	config   *config.ArchiveConfig
	running  atomic.Bool
	stopCh   chan struct{}
	stopOnce sync.Once
}

// NewArchiver 创建归档任务
func NewArchiver(storage Storage, cfg *config.ArchiveConfig) *Archiver {
	return &Archiver{
		storage: storage,
		config:  cfg,
		stopCh:  make(chan struct{}),
	}
}

// Start 启动归档任务（阻塞，应在 goroutine 中调用）
// 每天凌晨执行一次归档
func (a *Archiver) Start(ctx context.Context) {
	if !a.config.IsEnabled() {
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
	if err := os.MkdirAll(a.config.OutputDir, 0755); err != nil {
		logger.Error("archiver", "创建归档目录失败", "error", err, "dir", a.config.OutputDir)
		return
	}

	logger.Info("archiver", "归档任务已启动",
		"output_dir", a.config.OutputDir,
		"archive_days", a.config.ArchiveDays,
		"backfill_days", a.config.BackfillDays,
		"format", a.config.Format)

	// 首次立即尝试归档
	a.runArchive(ctx, archiveStorage)

	// 每天凌晨 3 点执行归档
	for {
		nextRun := a.nextArchiveTime()
		waitDuration := time.Until(nextRun)

		logger.Info("archiver", "下次归档时间",
			"next_run", nextRun.Format(time.RFC3339),
			"wait", waitDuration)

		select {
		case <-time.After(waitDuration):
			a.runArchive(ctx, archiveStorage)
		case <-ctx.Done():
			logger.Info("archiver", "归档任务收到取消信号，正在退出")
			return
		case <-a.stopCh:
			logger.Info("archiver", "归档任务收到停止信号，正在退出")
			return
		}
	}
}

// Stop 停止归档任务（幂等，可重复调用）
func (a *Archiver) Stop() {
	a.stopOnce.Do(func() {
		close(a.stopCh)
	})
}

// nextArchiveTime 计算下次归档时间（每天凌晨 3 点 UTC）
func (a *Archiver) nextArchiveTime() time.Time {
	now := time.Now().UTC()
	next := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, time.UTC)
	if now.After(next) {
		next = next.Add(24 * time.Hour)
	}
	return next
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

	now := time.Now().UTC()
	todayStart := now.Truncate(24 * time.Hour)

	// 计算归档日期范围
	// latestArchiveDate: 最新可归档日期（今天 - archive_days）
	// earliestArchiveDate: 最早回溯日期（latestArchiveDate - backfill_days + 1）
	latestArchiveDate := todayStart.AddDate(0, 0, -a.config.ArchiveDays)
	backfillDays := a.config.BackfillDays
	if backfillDays < 1 {
		backfillDays = 1
	}
	earliestArchiveDate := latestArchiveDate.AddDate(0, 0, -(backfillDays - 1))

	logger.Info("archiver", "本轮归档计划",
		"earliest_date", earliestArchiveDate.Format("2006-01-02"),
		"latest_date", latestArchiveDate.Format("2006-01-02"),
		"archive_days", a.config.ArchiveDays,
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

		if err := a.archiveOneDay(ctx, archiveStorage, d); err != nil {
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
	a.cleanupOldArchives()
}

// archiveOneDay 归档单日数据
func (a *Archiver) archiveOneDay(ctx context.Context, archiveStorage ArchiveStorage, archiveDate time.Time) error {
	archiveDate = archiveDate.UTC().Truncate(24 * time.Hour)
	dateStr := archiveDate.Format("2006-01-02")

	// 生成文件名
	filename := fmt.Sprintf("probe_history_%s.csv", dateStr)
	if a.config.Format == "csv.gz" {
		filename += ".gz"
	}
	fullPath := filepath.Join(a.config.OutputDir, filename)

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
	file, err := os.CreateTemp(a.config.OutputDir, filename+".tmp.*")
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
	if a.config.Format == "csv.gz" {
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
func (a *Archiver) cleanupOldArchives() {
	if a.config.KeepDaysValue <= 0 {
		return // -1 表示永久保留，0 不应该出现
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -a.config.KeepDaysValue)

	entries, err := os.ReadDir(a.config.OutputDir)
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
			fullPath := filepath.Join(a.config.OutputDir, name)
			if err := os.Remove(fullPath); err != nil {
				logger.Warn("archiver", "删除过期归档失败", "error", err, "file", name)
			} else {
				logger.Info("archiver", "已删除过期归档", "file", name)
			}
		}
	}
}
