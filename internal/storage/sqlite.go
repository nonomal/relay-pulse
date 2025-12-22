package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
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

	// 事件功能相关表
	if err := s.initEventTables(ctx); err != nil {
		return err
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

// GetLatestBatch 批量获取每个监测项的最新记录
//
// 实现说明：
// - 使用 CTE(keys) 承载入参列表
// - 使用窗口函数 ROW_NUMBER() 分组取最新一条（rn=1）
func (s *SQLiteStorage) GetLatestBatch(keys []MonitorKey) (map[MonitorKey]*ProbeRecord, error) {
	ctx := s.effectiveCtx()
	result := make(map[MonitorKey]*ProbeRecord, len(keys))
	if len(keys) == 0 {
		return result, nil
	}

	var b strings.Builder
	args := make([]any, 0, len(keys)*3)

	b.WriteString("WITH keys(provider, service, channel) AS (VALUES ")
	for i, k := range keys {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("(?, ?, ?)")
		args = append(args, k.Provider, k.Service, k.Channel)
	}
	b.WriteString(`),
ranked AS (
	SELECT
		p.id, p.provider, p.service, p.channel, p.status, p.sub_status, p.http_code, p.latency, p.timestamp,
		ROW_NUMBER() OVER (PARTITION BY p.provider, p.service, p.channel ORDER BY p.timestamp DESC, p.id DESC) AS rn
	FROM probe_history p
	JOIN keys k
		ON p.provider = k.provider AND p.service = k.service AND p.channel = k.channel
)
SELECT id, provider, service, channel, status, sub_status, http_code, latency, timestamp
FROM ranked
WHERE rn = 1
`)

	rows, err := s.db.QueryContext(ctx, b.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("批量查询最新记录失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		rec := &ProbeRecord{}
		var subStatusStr string
		if err := rows.Scan(
			&rec.ID,
			&rec.Provider,
			&rec.Service,
			&rec.Channel,
			&rec.Status,
			&subStatusStr,
			&rec.HttpCode,
			&rec.Latency,
			&rec.Timestamp,
		); err != nil {
			return nil, fmt.Errorf("扫描最新记录失败: %w", err)
		}
		rec.SubStatus = SubStatus(subStatusStr)
		result[MonitorKey{Provider: rec.Provider, Service: rec.Service, Channel: rec.Channel}] = rec
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("迭代最新记录失败: %w", err)
	}

	return result, nil
}

// GetHistoryBatch 批量获取多个监测项的历史记录（时间范围）
//
// 实现说明：
// - 使用 CTE(keys) + JOIN 过滤目标监测项集合
// - ORDER BY 按 (provider,service,channel,timestamp DESC) 输出
// - 返回前对每个 key 的切片做 reverse，保证时间升序（与 GetHistory 一致）
func (s *SQLiteStorage) GetHistoryBatch(keys []MonitorKey, since time.Time) (map[MonitorKey][]*ProbeRecord, error) {
	ctx := s.effectiveCtx()
	result := make(map[MonitorKey][]*ProbeRecord, len(keys))
	if len(keys) == 0 {
		return result, nil
	}

	var b strings.Builder
	args := make([]any, 0, len(keys)*3+1)

	b.WriteString("WITH keys(provider, service, channel) AS (VALUES ")
	for i, k := range keys {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("(?, ?, ?)")
		args = append(args, k.Provider, k.Service, k.Channel)
	}
	b.WriteString(")\n")
	b.WriteString(`
SELECT
	p.id, p.provider, p.service, p.channel, p.status, p.sub_status, p.http_code, p.latency, p.timestamp
FROM probe_history p
JOIN keys k
	ON p.provider = k.provider AND p.service = k.service AND p.channel = k.channel
WHERE p.timestamp >= ?
ORDER BY p.provider, p.service, p.channel, p.timestamp DESC
`)
	args = append(args, since.Unix())

	rows, err := s.db.QueryContext(ctx, b.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("批量查询历史记录失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		rec := &ProbeRecord{}
		var subStatusStr string
		if err := rows.Scan(
			&rec.ID,
			&rec.Provider,
			&rec.Service,
			&rec.Channel,
			&rec.Status,
			&subStatusStr,
			&rec.HttpCode,
			&rec.Latency,
			&rec.Timestamp,
		); err != nil {
			return nil, fmt.Errorf("扫描历史记录失败: %w", err)
		}
		rec.SubStatus = SubStatus(subStatusStr)
		key := MonitorKey{Provider: rec.Provider, Service: rec.Service, Channel: rec.Channel}
		result[key] = append(result[key], rec)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("迭代历史记录失败: %w", err)
	}

	// DESC 取数利用索引，返回前对每个 key 翻转为时间升序
	for k := range result {
		reverseRecords(result[k])
	}

	return result, nil
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

// ===== 状态订阅通知（事件）相关方法 =====

// initEventTables 初始化事件相关表
func (s *SQLiteStorage) initEventTables(ctx context.Context) error {
	// 服务状态表（状态机持久化）
	serviceStatesSchema := `
	CREATE TABLE IF NOT EXISTS service_states (
		provider TEXT NOT NULL,
		service TEXT NOT NULL,
		channel TEXT NOT NULL DEFAULT '',
		stable_available INTEGER NOT NULL DEFAULT -1,
		streak_count INTEGER NOT NULL DEFAULT 0,
		streak_status INTEGER NOT NULL DEFAULT -1,
		last_record_id INTEGER,
		last_timestamp INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (provider, service, channel)
	);
	`
	if _, err := s.db.ExecContext(ctx, serviceStatesSchema); err != nil {
		return fmt.Errorf("创建 service_states 表失败: %w", err)
	}

	// 状态事件表
	statusEventsSchema := `
	CREATE TABLE IF NOT EXISTS status_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		provider TEXT NOT NULL,
		service TEXT NOT NULL,
		channel TEXT NOT NULL DEFAULT '',
		event_type TEXT NOT NULL,
		from_status INTEGER NOT NULL,
		to_status INTEGER NOT NULL,
		trigger_record_id INTEGER NOT NULL,
		observed_at INTEGER NOT NULL,
		created_at INTEGER NOT NULL,
		meta TEXT
	);
	`
	if _, err := s.db.ExecContext(ctx, statusEventsSchema); err != nil {
		return fmt.Errorf("创建 status_events 表失败: %w", err)
	}

	// 创建索引
	eventsIndexSQL := `
	CREATE INDEX IF NOT EXISTS idx_status_events_psc_id
	ON status_events(provider, service, channel, id);
	`
	if _, err := s.db.ExecContext(ctx, eventsIndexSQL); err != nil {
		return fmt.Errorf("创建 status_events 索引失败: %w", err)
	}

	// 创建唯一约束索引（幂等性保障）
	uniqueIndexSQL := `
	CREATE UNIQUE INDEX IF NOT EXISTS idx_status_events_unique
	ON status_events(provider, service, channel, event_type, trigger_record_id);
	`
	if _, err := s.db.ExecContext(ctx, uniqueIndexSQL); err != nil {
		return fmt.Errorf("创建 status_events 唯一索引失败: %w", err)
	}

	return nil
}

// GetServiceState 获取服务状态机持久化状态
func (s *SQLiteStorage) GetServiceState(provider, service, channel string) (*ServiceState, error) {
	ctx := s.effectiveCtx()
	query := `
		SELECT provider, service, channel, stable_available, streak_count, streak_status, last_record_id, last_timestamp
		FROM service_states
		WHERE provider = ? AND service = ? AND channel = ?
	`

	var state ServiceState
	var lastRecordID sql.NullInt64

	err := s.db.QueryRowContext(ctx, query, provider, service, channel).Scan(
		&state.Provider,
		&state.Service,
		&state.Channel,
		&state.StableAvailable,
		&state.StreakCount,
		&state.StreakStatus,
		&lastRecordID,
		&state.LastTimestamp,
	)

	if err == sql.ErrNoRows {
		return nil, nil // 尚未初始化
	}
	if err != nil {
		return nil, fmt.Errorf("查询服务状态失败: %w", err)
	}

	if lastRecordID.Valid {
		state.LastRecordID = lastRecordID.Int64
	}

	return &state, nil
}

// UpsertServiceState 写入或更新服务状态机持久化状态
func (s *SQLiteStorage) UpsertServiceState(state *ServiceState) error {
	ctx := s.effectiveCtx()
	query := `
		INSERT INTO service_states (provider, service, channel, stable_available, streak_count, streak_status, last_record_id, last_timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(provider, service, channel) DO UPDATE SET
			stable_available = excluded.stable_available,
			streak_count = excluded.streak_count,
			streak_status = excluded.streak_status,
			last_record_id = excluded.last_record_id,
			last_timestamp = excluded.last_timestamp
	`

	_, err := s.db.ExecContext(ctx, query,
		state.Provider,
		state.Service,
		state.Channel,
		state.StableAvailable,
		state.StreakCount,
		state.StreakStatus,
		state.LastRecordID,
		state.LastTimestamp,
	)

	if err != nil {
		return fmt.Errorf("更新服务状态失败: %w", err)
	}

	return nil
}

// SaveStatusEvent 保存状态变更事件
func (s *SQLiteStorage) SaveStatusEvent(event *StatusEvent) error {
	ctx := s.effectiveCtx()

	// 序列化 meta 为 JSON
	var metaJSON sql.NullString
	if event.Meta != nil && len(event.Meta) > 0 {
		metaBytes, err := json.Marshal(event.Meta)
		if err != nil {
			return fmt.Errorf("序列化事件 meta 失败: %w", err)
		}
		metaJSON = sql.NullString{String: string(metaBytes), Valid: true}
	}

	query := `
		INSERT INTO status_events (provider, service, channel, event_type, from_status, to_status, trigger_record_id, observed_at, created_at, meta)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := s.db.ExecContext(ctx, query,
		event.Provider,
		event.Service,
		event.Channel,
		string(event.EventType),
		event.FromStatus,
		event.ToStatus,
		event.TriggerRecordID,
		event.ObservedAt,
		event.CreatedAt,
		metaJSON,
	)

	if err != nil {
		// 检查是否是唯一约束冲突（幂等处理）
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil // 重复事件，视为成功
		}
		return fmt.Errorf("保存状态事件失败: %w", err)
	}

	id, _ := result.LastInsertId()
	event.ID = id
	return nil
}

// GetStatusEvents 查询状态变更事件列表
func (s *SQLiteStorage) GetStatusEvents(sinceID int64, limit int, filters *EventFilters) ([]*StatusEvent, error) {
	ctx := s.effectiveCtx()

	var conditions []string
	var args []any

	// 游标条件
	conditions = append(conditions, "id > ?")
	args = append(args, sinceID)

	// 可选过滤条件
	if filters != nil {
		if filters.Provider != "" {
			conditions = append(conditions, "provider = ?")
			args = append(args, filters.Provider)
		}
		if filters.Service != "" {
			conditions = append(conditions, "service = ?")
			args = append(args, filters.Service)
		}
		if filters.Channel != "" {
			conditions = append(conditions, "channel = ?")
			args = append(args, filters.Channel)
		}
		if len(filters.Types) > 0 {
			placeholders := make([]string, len(filters.Types))
			for i, t := range filters.Types {
				placeholders[i] = "?"
				args = append(args, string(t))
			}
			conditions = append(conditions, "event_type IN ("+strings.Join(placeholders, ",")+")")
		}
	}

	// 限制条数
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	query := fmt.Sprintf(`
		SELECT id, provider, service, channel, event_type, from_status, to_status, trigger_record_id, observed_at, created_at, meta
		FROM status_events
		WHERE %s
		ORDER BY id ASC
		LIMIT ?
	`, strings.Join(conditions, " AND "))
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询状态事件失败: %w", err)
	}
	defer rows.Close()

	var events []*StatusEvent
	for rows.Next() {
		var event StatusEvent
		var eventTypeStr string
		var metaJSON sql.NullString

		err := rows.Scan(
			&event.ID,
			&event.Provider,
			&event.Service,
			&event.Channel,
			&eventTypeStr,
			&event.FromStatus,
			&event.ToStatus,
			&event.TriggerRecordID,
			&event.ObservedAt,
			&event.CreatedAt,
			&metaJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描状态事件失败: %w", err)
		}

		event.EventType = EventType(eventTypeStr)

		// 反序列化 meta
		if metaJSON.Valid && metaJSON.String != "" {
			var meta map[string]any
			if err := json.Unmarshal([]byte(metaJSON.String), &meta); err == nil {
				event.Meta = meta
			}
		}

		events = append(events, &event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("迭代状态事件失败: %w", err)
	}

	return events, nil
}

// GetLatestEventID 获取最新事件 ID
func (s *SQLiteStorage) GetLatestEventID() (int64, error) {
	ctx := s.effectiveCtx()
	query := `SELECT COALESCE(MAX(id), 0) FROM status_events`

	var latestID int64
	err := s.db.QueryRowContext(ctx, query).Scan(&latestID)
	if err != nil {
		return 0, fmt.Errorf("查询最新事件 ID 失败: %w", err)
	}

	return latestID, nil
}
