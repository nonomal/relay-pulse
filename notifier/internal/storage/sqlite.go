package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStorage SQLite 存储实现
type SQLiteStorage struct {
	db *sql.DB
}

// NewSQLiteStorage 创建 SQLite 存储
func NewSQLiteStorage(dsn string) (*SQLiteStorage, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	// SQLite 配置
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Hour)

	return &SQLiteStorage{db: db}, nil
}

// Init 初始化数据库表
func (s *SQLiteStorage) Init(ctx context.Context) error {
	// 游标表
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS poll_cursor (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			last_event_id INTEGER NOT NULL DEFAULT 0,
			updated_at INTEGER NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("创建 poll_cursor 表失败: %w", err)
	}

	// 初始化游标（如果不存在）
	if _, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO poll_cursor (id, last_event_id, updated_at) VALUES (1, 0, ?)
	`, time.Now().Unix()); err != nil {
		return fmt.Errorf("初始化游标失败: %w", err)
	}

	// Telegram 用户表
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS telegram_chats (
			chat_id INTEGER PRIMARY KEY,
			username TEXT,
			first_name TEXT,
			status TEXT NOT NULL DEFAULT 'active',
			last_command_at INTEGER,
			command_count INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("创建 telegram_chats 表失败: %w", err)
	}

	// 订阅关系表
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS subscriptions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			chat_id INTEGER NOT NULL,
			provider TEXT NOT NULL,
			service TEXT NOT NULL,
			channel TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL,
			UNIQUE(chat_id, provider, service, channel),
			FOREIGN KEY (chat_id) REFERENCES telegram_chats(chat_id) ON DELETE CASCADE
		)
	`); err != nil {
		return fmt.Errorf("创建 subscriptions 表失败: %w", err)
	}

	// 订阅表索引
	if _, err := s.db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_subscriptions_psc ON subscriptions(provider, service, channel)
	`); err != nil {
		return fmt.Errorf("创建订阅索引失败: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_subscriptions_chat ON subscriptions(chat_id)
	`); err != nil {
		return fmt.Errorf("创建订阅索引失败: %w", err)
	}

	// 绑定 token 表
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS bind_tokens (
			token TEXT PRIMARY KEY,
			favorites TEXT NOT NULL,
			expires_at INTEGER NOT NULL,
			used_at INTEGER,
			created_at INTEGER NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("创建 bind_tokens 表失败: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_bind_tokens_expires ON bind_tokens(expires_at)
	`); err != nil {
		return fmt.Errorf("创建 bind_tokens 索引失败: %w", err)
	}

	// 投递记录表
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS deliveries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			event_id INTEGER NOT NULL,
			chat_id INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			message_id TEXT,
			error_message TEXT,
			retry_count INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			UNIQUE(event_id, chat_id)
		)
	`); err != nil {
		return fmt.Errorf("创建 deliveries 表失败: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_deliveries_pending ON deliveries(status, created_at) WHERE status = 'pending'
	`); err != nil {
		return fmt.Errorf("创建 deliveries 索引失败: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_deliveries_event ON deliveries(event_id)
	`); err != nil {
		return fmt.Errorf("创建 deliveries 索引失败: %w", err)
	}

	return nil
}

// Close 关闭数据库
func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

// ===== 游标管理 =====

// GetCursor 获取轮询游标
func (s *SQLiteStorage) GetCursor(ctx context.Context) (int64, error) {
	var lastEventID int64
	err := s.db.QueryRowContext(ctx, `SELECT last_event_id FROM poll_cursor WHERE id = 1`).Scan(&lastEventID)
	if err != nil {
		return 0, fmt.Errorf("获取游标失败: %w", err)
	}
	return lastEventID, nil
}

// UpdateCursor 更新轮询游标
func (s *SQLiteStorage) UpdateCursor(ctx context.Context, lastEventID int64) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE poll_cursor SET last_event_id = ?, updated_at = ? WHERE id = 1
	`, lastEventID, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("更新游标失败: %w", err)
	}
	return nil
}

// ===== Telegram 用户管理 =====

// UpsertChat 创建或更新 Telegram 用户
func (s *SQLiteStorage) UpsertChat(ctx context.Context, chat *TelegramChat) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO telegram_chats (chat_id, username, first_name, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(chat_id) DO UPDATE SET
			username = excluded.username,
			first_name = excluded.first_name,
			updated_at = excluded.updated_at
	`, chat.ChatID, chat.Username, chat.FirstName, ChatStatusActive, now, now)
	if err != nil {
		return fmt.Errorf("创建/更新用户失败: %w", err)
	}
	return nil
}

// GetChat 获取 Telegram 用户
func (s *SQLiteStorage) GetChat(ctx context.Context, chatID int64) (*TelegramChat, error) {
	chat := &TelegramChat{}
	var lastCommandAt sql.NullInt64

	err := s.db.QueryRowContext(ctx, `
		SELECT chat_id, username, first_name, status, last_command_at, command_count, created_at, updated_at
		FROM telegram_chats WHERE chat_id = ?
	`, chatID).Scan(
		&chat.ChatID, &chat.Username, &chat.FirstName, &chat.Status,
		&lastCommandAt, &chat.CommandCount, &chat.CreatedAt, &chat.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	if lastCommandAt.Valid {
		chat.LastCommandAt = lastCommandAt.Int64
	}

	return chat, nil
}

// UpdateChatStatus 更新用户状态
func (s *SQLiteStorage) UpdateChatStatus(ctx context.Context, chatID int64, status string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE telegram_chats SET status = ?, updated_at = ? WHERE chat_id = ?
	`, status, time.Now().Unix(), chatID)
	if err != nil {
		return fmt.Errorf("更新用户状态失败: %w", err)
	}
	return nil
}

// UpdateChatCommandTime 更新用户命令时间
func (s *SQLiteStorage) UpdateChatCommandTime(ctx context.Context, chatID int64) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `
		UPDATE telegram_chats SET last_command_at = ?, command_count = command_count + 1, updated_at = ? WHERE chat_id = ?
	`, now, now, chatID)
	if err != nil {
		return fmt.Errorf("更新命令时间失败: %w", err)
	}
	return nil
}

// ===== 订阅管理 =====

// AddSubscription 添加订阅
func (s *SQLiteStorage) AddSubscription(ctx context.Context, sub *Subscription) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO subscriptions (chat_id, provider, service, channel, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(chat_id, provider, service, channel) DO NOTHING
	`, sub.ChatID, sub.Provider, sub.Service, sub.Channel, now)
	if err != nil {
		return fmt.Errorf("添加订阅失败: %w", err)
	}
	return nil
}

// RemoveSubscription 移除订阅
func (s *SQLiteStorage) RemoveSubscription(ctx context.Context, chatID int64, provider, service, channel string) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM subscriptions WHERE chat_id = ? AND provider = ? AND service = ? AND channel = ?
	`, chatID, provider, service, channel)
	if err != nil {
		return fmt.Errorf("移除订阅失败: %w", err)
	}
	return nil
}

// GetSubscriptionsByChatID 获取用户的所有订阅
func (s *SQLiteStorage) GetSubscriptionsByChatID(ctx context.Context, chatID int64) ([]*Subscription, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, chat_id, provider, service, channel, created_at
		FROM subscriptions WHERE chat_id = ? ORDER BY created_at DESC
	`, chatID)
	if err != nil {
		return nil, fmt.Errorf("查询订阅失败: %w", err)
	}
	defer rows.Close()

	var subs []*Subscription
	for rows.Next() {
		sub := &Subscription{}
		if err := rows.Scan(&sub.ID, &sub.ChatID, &sub.Provider, &sub.Service, &sub.Channel, &sub.CreatedAt); err != nil {
			return nil, fmt.Errorf("扫描订阅失败: %w", err)
		}
		subs = append(subs, sub)
	}

	return subs, nil
}

// GetSubscribersByMonitor 获取监测项的所有订阅者
func (s *SQLiteStorage) GetSubscribersByMonitor(ctx context.Context, provider, service, channel string) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.chat_id FROM subscriptions s
		JOIN telegram_chats t ON s.chat_id = t.chat_id
		WHERE s.provider = ? AND s.service = ? AND s.channel = ? AND t.status = 'active'
	`, provider, service, channel)
	if err != nil {
		return nil, fmt.Errorf("查询订阅者失败: %w", err)
	}
	defer rows.Close()

	var chatIDs []int64
	for rows.Next() {
		var chatID int64
		if err := rows.Scan(&chatID); err != nil {
			return nil, fmt.Errorf("扫描订阅者失败: %w", err)
		}
		chatIDs = append(chatIDs, chatID)
	}

	return chatIDs, nil
}

// CountSubscriptions 统计用户订阅数
func (s *SQLiteStorage) CountSubscriptions(ctx context.Context, chatID int64) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM subscriptions WHERE chat_id = ?`, chatID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("统计订阅数失败: %w", err)
	}
	return count, nil
}

// ClearSubscriptions 清空用户所有订阅
func (s *SQLiteStorage) ClearSubscriptions(ctx context.Context, chatID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM subscriptions WHERE chat_id = ?`, chatID)
	if err != nil {
		return fmt.Errorf("清空订阅失败: %w", err)
	}
	return nil
}

// ===== 绑定 Token 管理 =====

// CreateBindToken 创建绑定 token
func (s *SQLiteStorage) CreateBindToken(ctx context.Context, token *BindToken) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO bind_tokens (token, favorites, expires_at, created_at)
		VALUES (?, ?, ?, ?)
	`, token.Token, token.Favorites, token.ExpiresAt, token.CreatedAt)
	if err != nil {
		return fmt.Errorf("创建绑定 token 失败: %w", err)
	}
	return nil
}

// GetBindToken 获取绑定 token
func (s *SQLiteStorage) GetBindToken(ctx context.Context, token string) (*BindToken, error) {
	bt := &BindToken{}
	var usedAt sql.NullInt64

	err := s.db.QueryRowContext(ctx, `
		SELECT token, favorites, expires_at, used_at, created_at
		FROM bind_tokens WHERE token = ?
	`, token).Scan(&bt.Token, &bt.Favorites, &bt.ExpiresAt, &usedAt, &bt.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询绑定 token 失败: %w", err)
	}

	if usedAt.Valid {
		bt.UsedAt = usedAt.Int64
	}

	return bt, nil
}

// ConsumeBindToken 消费绑定 token
func (s *SQLiteStorage) ConsumeBindToken(ctx context.Context, token string) (*BindToken, error) {
	now := time.Now().Unix()

	// 使用事务确保原子性
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	bt := &BindToken{}
	var usedAt sql.NullInt64

	err = tx.QueryRowContext(ctx, `
		SELECT token, favorites, expires_at, used_at, created_at
		FROM bind_tokens WHERE token = ?
	`, token).Scan(&bt.Token, &bt.Favorites, &bt.ExpiresAt, &usedAt, &bt.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询绑定 token 失败: %w", err)
	}

	// 检查是否已过期
	if bt.ExpiresAt < now {
		return nil, fmt.Errorf("token 已过期")
	}

	// 检查是否已使用
	if usedAt.Valid {
		return nil, fmt.Errorf("token 已使用")
	}

	// 标记已使用
	_, err = tx.ExecContext(ctx, `UPDATE bind_tokens SET used_at = ? WHERE token = ?`, now, token)
	if err != nil {
		return nil, fmt.Errorf("标记 token 已使用失败: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("提交事务失败: %w", err)
	}

	bt.UsedAt = now
	return bt, nil
}

// CleanupExpiredTokens 清理过期 token
func (s *SQLiteStorage) CleanupExpiredTokens(ctx context.Context) (int64, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM bind_tokens WHERE expires_at < ?`, time.Now().Unix())
	if err != nil {
		return 0, fmt.Errorf("清理过期 token 失败: %w", err)
	}
	return result.RowsAffected()
}

// ===== 投递记录管理 =====

// CreateDelivery 创建投递记录
func (s *SQLiteStorage) CreateDelivery(ctx context.Context, delivery *Delivery) error {
	now := time.Now().Unix()
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO deliveries (event_id, chat_id, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(event_id, chat_id) DO NOTHING
	`, delivery.EventID, delivery.ChatID, DeliveryStatusPending, now, now)
	if err != nil {
		return fmt.Errorf("创建投递记录失败: %w", err)
	}

	id, _ := result.LastInsertId()
	delivery.ID = id
	delivery.CreatedAt = now
	delivery.UpdatedAt = now

	return nil
}

// UpdateDeliveryStatus 更新投递状态
func (s *SQLiteStorage) UpdateDeliveryStatus(ctx context.Context, id int64, status string, messageID string, errorMsg string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE deliveries SET status = ?, message_id = ?, error_message = ?, updated_at = ? WHERE id = ?
	`, status, messageID, errorMsg, time.Now().Unix(), id)
	if err != nil {
		return fmt.Errorf("更新投递状态失败: %w", err)
	}
	return nil
}

// GetPendingDeliveries 获取待发送的投递记录
func (s *SQLiteStorage) GetPendingDeliveries(ctx context.Context, limit int) ([]*Delivery, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, event_id, chat_id, status, message_id, error_message, retry_count, created_at, updated_at
		FROM deliveries WHERE status = 'pending' ORDER BY created_at ASC LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("查询待发送投递失败: %w", err)
	}
	defer rows.Close()

	var deliveries []*Delivery
	for rows.Next() {
		d := &Delivery{}
		var messageID, errorMessage sql.NullString
		if err := rows.Scan(&d.ID, &d.EventID, &d.ChatID, &d.Status, &messageID, &errorMessage, &d.RetryCount, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("扫描投递记录失败: %w", err)
		}
		if messageID.Valid {
			d.MessageID = messageID.String
		}
		if errorMessage.Valid {
			d.ErrorMessage = errorMessage.String
		}
		deliveries = append(deliveries, d)
	}

	return deliveries, nil
}

// IncrementRetryCount 增加重试次数
func (s *SQLiteStorage) IncrementRetryCount(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE deliveries SET retry_count = retry_count + 1, updated_at = ? WHERE id = ?
	`, time.Now().Unix(), id)
	if err != nil {
		return fmt.Errorf("增加重试次数失败: %w", err)
	}
	return nil
}

// CleanupOldDeliveries 清理旧的投递记录
func (s *SQLiteStorage) CleanupOldDeliveries(ctx context.Context, before time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM deliveries WHERE created_at < ? AND status IN ('sent', 'failed')
	`, before.Unix())
	if err != nil {
		return 0, fmt.Errorf("清理旧投递记录失败: %w", err)
	}
	return result.RowsAffected()
}
