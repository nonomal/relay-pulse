package config

import (
	"fmt"
	"strings"
	"time"
)

// StorageConfig 存储配置
type StorageConfig struct {
	Type string `yaml:"type" json:"type"` // "sqlite" 或 "postgres"

	// SQLite 配置
	SQLite SQLiteConfig `yaml:"sqlite" json:"sqlite"`

	// PostgreSQL 配置
	Postgres PostgresConfig `yaml:"postgres" json:"postgres"`

	// 历史数据保留与清理配置（默认禁用，需显式开启）
	Retention RetentionConfig `yaml:"retention" json:"retention"`

	// 历史数据归档配置（默认禁用）
	Archive ArchiveConfig `yaml:"archive" json:"archive"`
}

// SQLiteConfig SQLite 配置
type SQLiteConfig struct {
	Path string `yaml:"path" json:"path"` // 数据库文件路径
}

// PostgresConfig PostgreSQL 配置
type PostgresConfig struct {
	Host            string `yaml:"host" json:"host"`
	Port            int    `yaml:"port" json:"port"`
	User            string `yaml:"user" json:"user"`
	Password        string `yaml:"password" json:"-"` // 不输出到 JSON
	Database        string `yaml:"database" json:"database"`
	SSLMode         string `yaml:"sslmode" json:"sslmode"`
	MaxOpenConns    int    `yaml:"max_open_conns" json:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns" json:"max_idle_conns"`
	ConnMaxLifetime string `yaml:"conn_max_lifetime" json:"conn_max_lifetime"`
}

// RetentionConfig 历史数据保留与清理配置
type RetentionConfig struct {
	// 是否启用清理任务（默认 false，需要显式开启）
	Enabled *bool `yaml:"enabled" json:"enabled"`

	// 原始明细保留天数（默认 36）
	// 建议比用户可见的最大时间范围（30 天）多几天缓冲
	Days int `yaml:"days" json:"days"`

	// 清理任务执行间隔（默认 "1h"）
	CleanupInterval string `yaml:"cleanup_interval" json:"cleanup_interval"`

	// 每批删除的最大行数（默认 10000）
	BatchSize int `yaml:"batch_size" json:"batch_size"`

	// 单轮运行最多批次数（默认 100）
	// 用于限制单次清理耗时，避免长期占用写锁或造成抖动
	MaxBatchesPerRun int `yaml:"max_batches_per_run" json:"max_batches_per_run"`

	// 启动后延迟多久开始首次清理（默认 "1m"）
	// 用于避免服务启动抖动或多实例同时启动造成的峰值冲击
	StartupDelay string `yaml:"startup_delay" json:"startup_delay"`

	// 调度抖动比例（默认 0.2）
	// 取值范围 [0,1]，用于在 interval 基础上增加随机偏移，避免多实例同刻执行
	Jitter float64 `yaml:"jitter" json:"jitter"`

	// 解析后的时间间隔（内部使用，不序列化）
	CleanupIntervalDuration time.Duration `yaml:"-" json:"-"`
	StartupDelayDuration    time.Duration `yaml:"-" json:"-"`
}

// IsEnabled 返回是否启用清理任务
func (c *RetentionConfig) IsEnabled() bool {
	if c.Enabled == nil {
		return false // 默认禁用（需要显式开启）
	}
	return *c.Enabled
}

// Normalize 规范化 retention 配置（填充默认值并解析 duration）
func (c *RetentionConfig) Normalize() error {
	// 保留天数（默认 36）
	if c.Days == 0 {
		c.Days = 36
	}
	if c.Days < 1 {
		return fmt.Errorf("storage.retention.days 必须 >= 1，当前值: %d", c.Days)
	}

	// 清理间隔（默认 1h）
	if strings.TrimSpace(c.CleanupInterval) == "" {
		c.CleanupInterval = "1h"
	}
	d, err := time.ParseDuration(strings.TrimSpace(c.CleanupInterval))
	if err != nil {
		return fmt.Errorf("storage.retention.cleanup_interval 解析失败: %w", err)
	}
	if d <= 0 {
		return fmt.Errorf("storage.retention.cleanup_interval 必须 > 0")
	}
	c.CleanupIntervalDuration = d

	// 批大小（默认 10000）
	if c.BatchSize == 0 {
		c.BatchSize = 10000
	}
	if c.BatchSize < 1 {
		return fmt.Errorf("storage.retention.batch_size 必须 >= 1，当前值: %d", c.BatchSize)
	}

	// 单轮最多批次数（默认 100）
	if c.MaxBatchesPerRun == 0 {
		c.MaxBatchesPerRun = 100
	}
	if c.MaxBatchesPerRun < 1 {
		return fmt.Errorf("storage.retention.max_batches_per_run 必须 >= 1，当前值: %d", c.MaxBatchesPerRun)
	}

	// 启动延迟（默认 1m）
	if strings.TrimSpace(c.StartupDelay) == "" {
		c.StartupDelay = "1m"
	}
	d, err = time.ParseDuration(strings.TrimSpace(c.StartupDelay))
	if err != nil {
		return fmt.Errorf("storage.retention.startup_delay 解析失败: %w", err)
	}
	if d < 0 {
		return fmt.Errorf("storage.retention.startup_delay 必须 >= 0")
	}
	c.StartupDelayDuration = d

	// 抖动比例（默认 0.2）
	if c.Jitter == 0 {
		c.Jitter = 0.2
	}
	if c.Jitter < 0 || c.Jitter > 1 {
		return fmt.Errorf("storage.retention.jitter 必须在 [0,1] 范围内，当前值: %g", c.Jitter)
	}

	return nil
}

// ArchiveConfig 历史数据归档配置
// 归档数据仅用于备份，不提供在线查询
type ArchiveConfig struct {
	// 是否启用归档（默认 false，需要显式开启）
	Enabled *bool `yaml:"enabled" json:"enabled"`

	// 归档执行时间（UTC 小时，0-23，默认 3）
	// 例如：配置为 19 表示每天 UTC 19:00（北京时间次日 03:00）执行
	ScheduleHour *int `yaml:"schedule_hour" json:"schedule_hour"`

	// 归档输出目录（默认 "./archive"）
	OutputDir string `yaml:"output_dir" json:"output_dir"`

	// 归档格式（默认 "csv.gz"，可选 "csv"）
	Format string `yaml:"format" json:"format"`

	// 归档阈值天数（默认 35）
	// timestamp < now - archive_days 的整天数据将被归档
	ArchiveDays int `yaml:"archive_days" json:"archive_days"`

	// 归档补齐回溯天数（默认 7）
	// 每次归档会尝试补齐 [now-archive_days-backfill_days+1, now-archive_days] 区间内缺失的归档文件
	// 设置为 1 表示仅归档单日（兼容旧行为）
	BackfillDays int `yaml:"backfill_days" json:"backfill_days"`

	// 归档文件保留天数（默认 365）
	// 使用指针类型区分"未配置"(nil→365) 和"配置为0"(0→永久保留)
	KeepDays *int `yaml:"keep_days" json:"keep_days"`

	// KeepDaysValue 规范化后的实际值（供运行时使用）
	// -1 表示永久保留，>0 表示保留天数
	KeepDaysValue int `yaml:"-" json:"-"`
}

// IsEnabled 返回是否启用归档
func (c *ArchiveConfig) IsEnabled() bool {
	if c.Enabled == nil {
		return false // 默认禁用
	}
	return *c.Enabled
}

// Normalize 规范化 archive 配置（填充默认值）
func (c *ArchiveConfig) Normalize() error {
	// 归档执行时间校验（UTC 小时，0-23，默认 3）
	if c.ScheduleHour != nil {
		if *c.ScheduleHour < 0 || *c.ScheduleHour > 23 {
			return fmt.Errorf("storage.archive.schedule_hour 必须在 [0,23] 范围内，当前值: %d", *c.ScheduleHour)
		}
	}

	// 输出目录（默认 ./archive）
	if strings.TrimSpace(c.OutputDir) == "" {
		c.OutputDir = "./archive"
	}

	// 格式（默认 csv.gz）
	if strings.TrimSpace(c.Format) == "" {
		c.Format = "csv.gz"
	}
	format := strings.ToLower(strings.TrimSpace(c.Format))
	if format != "csv" && format != "csv.gz" {
		return fmt.Errorf("storage.archive.format 仅支持 csv 或 csv.gz，当前值: %s", c.Format)
	}
	c.Format = format

	// 归档阈值天数（默认 35）
	if c.ArchiveDays == 0 {
		c.ArchiveDays = 35
	}
	if c.ArchiveDays < 1 {
		return fmt.Errorf("storage.archive.archive_days 必须 >= 1，当前值: %d", c.ArchiveDays)
	}

	// 归档补齐回溯天数（默认 7）
	if c.BackfillDays == 0 {
		c.BackfillDays = 7
	}
	if c.BackfillDays < 1 {
		return fmt.Errorf("storage.archive.backfill_days 必须 >= 1，当前值: %d", c.BackfillDays)
	}
	if c.BackfillDays > 365 {
		return fmt.Errorf("storage.archive.backfill_days 必须 <= 365，当前值: %d", c.BackfillDays)
	}

	// 保留天数规范化
	// nil(未配置) → 365 天
	// 0 → 永久保留（用 -1 表示）
	// >0 → 使用配置值
	// <0 → 错误
	if c.KeepDays == nil {
		c.KeepDaysValue = 365 // 默认 365 天
	} else if *c.KeepDays == 0 {
		c.KeepDaysValue = -1 // 永久保留
	} else if *c.KeepDays > 0 {
		c.KeepDaysValue = *c.KeepDays
	} else {
		return fmt.Errorf("storage.archive.keep_days 必须 >= 0，当前值: %d", *c.KeepDays)
	}

	return nil
}
