package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"monitor/internal/logger"

	_ "modernc.org/sqlite" // 纯Go实现的SQLite驱动
)

// SQLiteStorage SQLite存储实现
type SQLiteStorage struct {
	db  *sql.DB
	ctx context.Context
}

// NewSQLiteStorage 创建SQLite存储
func NewSQLiteStorage(dbPath string) (*SQLiteStorage, error) {
	// 使用WAL模式和其他参数解决并发锁问题
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_timeout=5000&_busy_timeout=5000", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	// 设置连接池参数（WAL模式支持更好的并发）
	db.SetMaxOpenConns(1) // SQLite建议单个写连接
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Hour)

	return &SQLiteStorage{db: db, ctx: context.Background()}, nil
}

// WithContext 返回绑定指定 context 的存储实例
func (s *SQLiteStorage) WithContext(ctx context.Context) Storage {
	if ctx == nil {
		return s
	}
	return &SQLiteStorage{
		db:  s.db,
		ctx: ctx,
	}
}

// effectiveCtx 返回有效的 context
func (s *SQLiteStorage) effectiveCtx() context.Context {
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}

// Init 初始化数据库表
func (s *SQLiteStorage) Init() error {
	ctx := s.effectiveCtx()
	schema := `
	CREATE TABLE IF NOT EXISTS probe_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		provider TEXT NOT NULL,
		service TEXT NOT NULL,
		channel TEXT NOT NULL DEFAULT '',
		status INTEGER NOT NULL,
		sub_status TEXT NOT NULL DEFAULT '',
		latency INTEGER NOT NULL,
		timestamp INTEGER NOT NULL
	);
	`

	_, err := s.db.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("初始化数据库失败: %w", err)
	}

	// 兼容旧数据库：添加缺失的列
	if err := s.ensureSubStatusColumn(); err != nil {
		return err
	}
	if err := s.ensureChannelColumn(); err != nil {
		return err
	}
	if err := s.ensureHttpCodeColumn(); err != nil {
		return err
	}

	// 在列迁移完成后创建索引
	//
	// 索引设计说明：
	// - 复合索引专为核心查询优化：GetLatest() 和 GetHistory()
	// - 所有业务查询都包含完整的 (provider, service, channel) 等值条件
	// - timestamp DESC 支持时间范围查询和排序，避免额外排序开销
	// - 包含查询所需的大部分字段（status, sub_status, latency），尽量减少回表
	// - 列顺序遵循 B-Tree 最佳实践：等值列在前，范围/排序列在后
	//
	// 性能优化：
	// - SQLite 不支持 INCLUDE 子句，使用复合索引模拟覆盖索引效果
	// - 虽然索引会变大，但 SQLite 查询优化器可以利用索引减少数据页访问
	// - 对于小型数据集（<1GB），性能提升明显
	//
	// ⚠️ 维护注意事项：
	// - 如果未来新增"不带 channel 的高频查询"，需要重新评估索引策略
	// - CleanOldRecords() 的全表扫描是可接受的（低频维护操作）
	// - SQLite 对大数据量（>1GB）性能有限，建议迁移到 PostgreSQL
	//
	// 性能验证：EXPLAIN QUERY PLAN SELECT ... WHERE provider=? AND service=? AND channel=? AND timestamp>=?
	indexSQL := `
	CREATE INDEX IF NOT EXISTS idx_probe_history_psc_ts_cover
	ON probe_history(provider, service, channel, timestamp DESC, status, sub_status, latency, http_code);
	`
	if _, err := s.db.ExecContext(ctx, indexSQL); err != nil {
		return fmt.Errorf("创建覆盖索引失败: %w", err)
	}

	return nil
}

// ensureSubStatusColumn 在旧表上添加 sub_status 列（向后兼容）
func (s *SQLiteStorage) ensureSubStatusColumn() error {
	ctx := s.effectiveCtx()
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(probe_history)`)
	if err != nil {
		return fmt.Errorf("查询表结构失败: %w", err)
	}
	defer rows.Close()

	hasColumn := false
	for rows.Next() {
		var (
			cid          int
			name         string
			colType      string
			notNull      int
			defaultValue sql.NullString
			pk           int
		)
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &pk); err != nil {
			return fmt.Errorf("扫描表结构失败: %w", err)
		}
		if name == "sub_status" {
			hasColumn = true
			break
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("遍历表结构失败: %w", err)
	}

	if hasColumn {
		return nil // 列已存在，无需添加
	}

	// 添加列
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE probe_history ADD COLUMN sub_status TEXT NOT NULL DEFAULT ''`); err != nil {
		return fmt.Errorf("添加 sub_status 列失败: %w", err)
	}

	logger.Info("storage", "已为 probe_history 表添加 sub_status 列")
	return nil
}

// ensureChannelColumn 在旧表上添加 channel 列（向后兼容）
func (s *SQLiteStorage) ensureChannelColumn() error {
	ctx := s.effectiveCtx()
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(probe_history)`)
	if err != nil {
		return fmt.Errorf("查询表结构失败: %w", err)
	}
	defer rows.Close()

	hasColumn := false
	for rows.Next() {
		var (
			cid          int
			name         string
			colType      string
			notNull      int
			defaultValue sql.NullString
			pk           int
		)
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &pk); err != nil {
			return fmt.Errorf("扫描表结构失败: %w", err)
		}
		if name == "channel" {
			hasColumn = true
			break
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("遍历表结构失败: %w", err)
	}

	if hasColumn {
		return nil // 列已存在，无需添加
	}

	// 添加列
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE probe_history ADD COLUMN channel TEXT NOT NULL DEFAULT ''`); err != nil {
		return fmt.Errorf("添加 channel 列失败: %w", err)
	}

	logger.Info("storage", "已为 probe_history 表添加 channel 列")
	return nil
}

// ensureHttpCodeColumn 在旧表上添加 http_code 列（向后兼容）
func (s *SQLiteStorage) ensureHttpCodeColumn() error {
	ctx := s.effectiveCtx()
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(probe_history)`)
	if err != nil {
		return fmt.Errorf("查询表结构失败: %w", err)
	}
	defer rows.Close()

	hasColumn := false
	for rows.Next() {
		var (
			cid          int
			name         string
			colType      string
			notNull      int
			defaultValue sql.NullString
			pk           int
		)
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &pk); err != nil {
			return fmt.Errorf("扫描表结构失败: %w", err)
		}
		if name == "http_code" {
			hasColumn = true
			break
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("遍历表结构失败: %w", err)
	}

	if hasColumn {
		return nil // 列已存在，无需添加
	}

	// 添加列
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE probe_history ADD COLUMN http_code INTEGER NOT NULL DEFAULT 0`); err != nil {
		return fmt.Errorf("添加 http_code 列失败: %w", err)
	}

	logger.Info("storage", "已为 probe_history 表添加 http_code 列")
	return nil
}

// MigrateChannelData 根据配置将 channel 为空的旧数据迁移到指定 channel
func (s *SQLiteStorage) MigrateChannelData(mappings []ChannelMigrationMapping) error {
	ctx := s.effectiveCtx()
	var pending int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM probe_history WHERE channel = ''`).Scan(&pending); err != nil {
		return fmt.Errorf("检测 channel 迁移需求失败: %w", err)
	}

	if pending == 0 {
		return nil
	}

	if len(mappings) == 0 {
		logger.Info("storage", "检测到 channel 为空的历史记录，但未提供迁移映射", "pending", pending)
		return nil
	}

	logger.Info("storage", "检测到 channel 为空的历史记录，开始迁移", "pending", pending)

	var totalUpdated int64
	for _, mapping := range mappings {
		if mapping.Channel == "" {
			continue
		}

		result, err := s.db.ExecContext(
			ctx,
			`UPDATE probe_history SET channel = ? WHERE channel = '' AND provider = ? AND service = ?`,
			mapping.Channel, mapping.Provider, mapping.Service,
		)
		if err != nil {
			return fmt.Errorf("迁移 channel 数据失败 (provider=%s service=%s): %w", mapping.Provider, mapping.Service, err)
		}

		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("获取迁移影响行数失败 (provider=%s service=%s): %w", mapping.Provider, mapping.Service, err)
		}

		if affected > 0 {
			totalUpdated += affected
			logger.Info("storage", "已迁移记录",
				"count", affected, "channel", mapping.Channel, "provider", mapping.Provider, "service", mapping.Service)
		}
	}

	if totalUpdated == 0 {
		logger.Info("storage", "channel 迁移：没有匹配的记录需要更新（可能缺少配置或 channel 仍为空）")
		return nil
	}

	remaining := int64(pending) - totalUpdated
	if remaining > 0 {
		logger.Info("storage", "channel 迁移完成", "updated", totalUpdated, "remaining", remaining)
	} else {
		logger.Info("storage", "channel 迁移完成", "updated", totalUpdated)
	}

	return nil
}

// Close 关闭数据库
func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

// SaveRecord 保存探测记录
func (s *SQLiteStorage) SaveRecord(record *ProbeRecord) error {
	ctx := s.effectiveCtx()
	query := `
		INSERT INTO probe_history (provider, service, channel, status, sub_status, http_code, latency, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := s.db.ExecContext(ctx, query,
		record.Provider,
		record.Service,
		record.Channel,
		record.Status,
		string(record.SubStatus),
		record.HttpCode,
		record.Latency,
		record.Timestamp,
	)

	if err != nil {
		return fmt.Errorf("保存记录失败: %w", err)
	}

	id, _ := result.LastInsertId()
	record.ID = id
	return nil
}

// GetLatest 获取最新记录
func (s *SQLiteStorage) GetLatest(provider, service, channel string) (*ProbeRecord, error) {
	ctx := s.effectiveCtx()
	query := `
		SELECT id, provider, service, channel, status, sub_status, http_code, latency, timestamp
		FROM probe_history
		WHERE provider = ? AND service = ? AND channel = ?
		ORDER BY timestamp DESC
		LIMIT 1
	`

	var record ProbeRecord
	var subStatusStr string
	err := s.db.QueryRowContext(ctx, query, provider, service, channel).Scan(
		&record.ID,
		&record.Provider,
		&record.Service,
		&record.Channel,
		&record.Status,
		&subStatusStr,
		&record.HttpCode,
		&record.Latency,
		&record.Timestamp,
	)

	if err == sql.ErrNoRows {
		return nil, nil // 没有记录不算错误
	}

	if err != nil {
		return nil, fmt.Errorf("查询最新记录失败: %w", err)
	}

	record.SubStatus = SubStatus(subStatusStr)
	return &record, nil
}

// GetHistory 获取历史记录
func (s *SQLiteStorage) GetHistory(provider, service, channel string, since time.Time) ([]*ProbeRecord, error) {
	ctx := s.effectiveCtx()
	// 使用 ORDER BY timestamp DESC 以利用索引（索引是 timestamp DESC）
	// 返回前在 Go 代码中反转为时间升序
	query := `
		SELECT id, provider, service, channel, status, sub_status, http_code, latency, timestamp
		FROM probe_history
		WHERE provider = ? AND service = ? AND channel = ? AND timestamp >= ?
		ORDER BY timestamp DESC
	`

	rows, err := s.db.QueryContext(ctx, query, provider, service, channel, since.Unix())
	if err != nil {
		return nil, fmt.Errorf("查询历史记录失败: %w", err)
	}
	defer rows.Close()

	var records []*ProbeRecord
	for rows.Next() {
		var record ProbeRecord
		var subStatusStr string
		err := rows.Scan(
			&record.ID,
			&record.Provider,
			&record.Service,
			&record.Channel,
			&record.Status,
			&subStatusStr,
			&record.HttpCode,
			&record.Latency,
			&record.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描记录失败: %w", err)
		}
		record.SubStatus = SubStatus(subStatusStr)
		records = append(records, &record)
	}

	// 检查迭代过程中是否发生错误
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("迭代记录失败: %w", err)
	}

	// DESC 取数利用索引，返回前翻转为时间升序
	reverseRecords(records)

	return records, nil
}

// CleanOldRecords 清理旧记录
func (s *SQLiteStorage) CleanOldRecords(days int) error {
	ctx := s.effectiveCtx()
	cutoff := time.Now().AddDate(0, 0, -days).Unix()
	query := `DELETE FROM probe_history WHERE timestamp < ?`

	result, err := s.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return fmt.Errorf("清理旧记录失败: %w", err)
	}

	deleted, _ := result.RowsAffected()
	if deleted > 0 {
		logger.Info("storage", "已清理旧记录", "deleted", deleted, "days", days)
	}

	return nil
}
