package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ===== 配置管理表初始化 =====

// initAdminConfigTables 初始化配置管理相关的数据库表
// 包括 10 张表：monitor_configs, monitor_secrets, monitor_config_audits,
// provider_policies, badge_definitions, badge_bindings,
// board_configs, board_items, global_settings, config_meta
func (s *SQLiteStorage) initAdminConfigTables(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("初始化配置管理表: 开启事务失败: %w", err)
	}
	defer tx.Rollback()

	statements := []string{
		// monitor_configs 监测项配置表
		`CREATE TABLE IF NOT EXISTS monitor_configs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider TEXT NOT NULL,
			service TEXT NOT NULL,
			channel TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 1,
			parent_key TEXT NOT NULL DEFAULT '',
			config_blob TEXT NOT NULL,
			schema_version INTEGER NOT NULL DEFAULT 1,
			version INTEGER NOT NULL DEFAULT 1,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			deleted_at INTEGER
		)`,
		// 四元组唯一约束（仅对未删除记录）
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_monitor_configs_key
		 ON monitor_configs(provider, service, channel, model) WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_monitor_configs_parent ON monitor_configs(parent_key) WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_monitor_configs_enabled ON monitor_configs(enabled) WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_monitor_configs_updated ON monitor_configs(updated_at DESC)`,

		// monitor_secrets API Key 加密存储表
		`CREATE TABLE IF NOT EXISTS monitor_secrets (
			monitor_id INTEGER PRIMARY KEY,
			api_key_ciphertext BLOB NOT NULL,
			api_key_nonce BLOB NOT NULL,
			key_version INTEGER NOT NULL DEFAULT 1,
			enc_version INTEGER NOT NULL DEFAULT 1,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,

		// monitor_config_audits 配置审计表
		`CREATE TABLE IF NOT EXISTS monitor_config_audits (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			monitor_id INTEGER NOT NULL,
			provider TEXT NOT NULL,
			service TEXT NOT NULL,
			channel TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			action TEXT NOT NULL,
			before_blob TEXT,
			after_blob TEXT,
			before_version INTEGER,
			after_version INTEGER,
			secret_changed INTEGER NOT NULL DEFAULT 0,
			actor TEXT,
			actor_ip TEXT,
			user_agent TEXT,
			request_id TEXT,
			batch_id TEXT,
			reason TEXT,
			created_at INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_monitor ON monitor_config_audits(monitor_id)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_time ON monitor_config_audits(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_request ON monitor_config_audits(request_id) WHERE request_id IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_audit_batch ON monitor_config_audits(batch_id) WHERE batch_id IS NOT NULL`,

		// provider_policies Provider 策略表
		`CREATE TABLE IF NOT EXISTS provider_policies (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			policy_type TEXT NOT NULL CHECK (policy_type IN ('disabled','hidden','risk')),
			provider TEXT NOT NULL,
			reason TEXT,
			risks TEXT,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_provider_policies ON provider_policies(policy_type, provider)`,

		// badge_definitions 徽标定义表
		`CREATE TABLE IF NOT EXISTS badge_definitions (
			id TEXT PRIMARY KEY,
			kind TEXT NOT NULL CHECK (kind IN ('sponsor','risk','feature','info')),
			weight INTEGER NOT NULL DEFAULT 0,
			label_i18n TEXT NOT NULL,
			tooltip_i18n TEXT,
			icon TEXT,
			color TEXT,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_badge_defs_kind ON badge_definitions(kind)`,

		// badge_bindings 徽标绑定表
		`CREATE TABLE IF NOT EXISTS badge_bindings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			badge_id TEXT NOT NULL,
			scope TEXT NOT NULL CHECK (scope IN ('global','provider','service','channel')),
			provider TEXT,
			service TEXT,
			channel TEXT,
			tooltip_override TEXT,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			CHECK (
				(scope='global'   AND provider IS NULL AND service IS NULL AND channel IS NULL) OR
				(scope='provider' AND provider IS NOT NULL AND service IS NULL AND channel IS NULL) OR
				(scope='service'  AND provider IS NOT NULL AND service IS NOT NULL AND channel IS NULL) OR
				(scope='channel'  AND provider IS NOT NULL AND service IS NOT NULL AND channel IS NOT NULL)
			)
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_badge_bindings
		 ON badge_bindings(badge_id, scope, COALESCE(provider,''), COALESCE(service,''), COALESCE(channel,''))`,

		// board_configs Board 配置表
		`CREATE TABLE IF NOT EXISTS board_configs (
			board TEXT PRIMARY KEY,
			display_name TEXT NOT NULL,
			description TEXT,
			sort_order INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,

		// board_items Board 成员关联表
		`CREATE TABLE IF NOT EXISTS board_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			board TEXT NOT NULL,
			monitor_id INTEGER NOT NULL,
			sort_order INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_board_items ON board_items(board, monitor_id)`,
		`CREATE INDEX IF NOT EXISTS idx_board_items_order ON board_items(board, sort_order, id)`,

		// global_settings 全局键值配置表
		`CREATE TABLE IF NOT EXISTS global_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			schema_version INTEGER NOT NULL DEFAULT 1,
			version INTEGER NOT NULL DEFAULT 1,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,

		// config_meta 配置版本元数据表
		`CREATE TABLE IF NOT EXISTS config_meta (
			scope TEXT PRIMARY KEY,
			version INTEGER NOT NULL DEFAULT 1,
			updated_at INTEGER NOT NULL
		)`,
	}

	for i, stmt := range statements {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("初始化配置管理表: 执行语句 %d 失败: %w", i+1, err)
		}
	}

	// 初始化默认数据
	now := time.Now().Unix()
	if err := seedConfigMeta(ctx, tx, now); err != nil {
		return err
	}
	if err := seedBoardConfigs(ctx, tx, now); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("初始化配置管理表: 提交事务失败: %w", err)
	}
	return nil
}

// seedConfigMeta 初始化 config_meta 默认数据
func seedConfigMeta(ctx context.Context, tx *sql.Tx, now int64) error {
	defaultScopes := []ConfigScope{
		ConfigScopeMonitors,
		ConfigScopePolicies,
		ConfigScopeBadges,
		ConfigScopeBoards,
		ConfigScopeSettings,
	}

	for _, scope := range defaultScopes {
		var exists int
		err := tx.QueryRowContext(ctx,
			`SELECT 1 FROM config_meta WHERE scope = ?`, string(scope)).Scan(&exists)
		if err == nil {
			continue // 已存在
		}
		if err != sql.ErrNoRows {
			return fmt.Errorf("检查 config_meta 失败: %w", err)
		}

		_, err = tx.ExecContext(ctx,
			`INSERT INTO config_meta (scope, version, updated_at) VALUES (?, 1, ?)`,
			string(scope), now)
		if err != nil {
			return fmt.Errorf("初始化 config_meta 失败: %w", err)
		}
	}
	return nil
}

// seedBoardConfigs 初始化 board_configs 默认数据
func seedBoardConfigs(ctx context.Context, tx *sql.Tx, now int64) error {
	defaultBoards := []struct {
		board       string
		displayName string
		description string
		sortOrder   int
	}{
		{"hot", "热门", "高关注监测项", 1},
		{"secondary", "常规", "常规监测项", 2},
		{"cold", "冷门", "低频监测项", 3},
	}

	for _, b := range defaultBoards {
		var exists int
		err := tx.QueryRowContext(ctx,
			`SELECT 1 FROM board_configs WHERE board = ?`, b.board).Scan(&exists)
		if err == nil {
			continue // 已存在
		}
		if err != sql.ErrNoRows {
			return fmt.Errorf("检查 board_configs 失败: %w", err)
		}

		_, err = tx.ExecContext(ctx,
			`INSERT INTO board_configs (board, display_name, description, sort_order, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			b.board, b.displayName, b.description, b.sortOrder, now, now)
		if err != nil {
			return fmt.Errorf("初始化 board_configs 失败: %w", err)
		}
	}
	return nil
}

// ===== MonitorConfig CRUD =====

// CreateMonitorConfig 创建监测项配置
func (s *SQLiteStorage) CreateMonitorConfig(config *MonitorConfig) error {
	if config == nil {
		return errors.New("config 不能为空")
	}
	if strings.TrimSpace(config.Provider) == "" {
		return errors.New("provider 不能为空")
	}
	if strings.TrimSpace(config.Service) == "" {
		return errors.New("service 不能为空")
	}

	ctx := s.effectiveCtx()
	now := time.Now().Unix()

	config.Version = 1
	config.SchemaVersion = 1
	config.CreatedAt = now
	config.UpdatedAt = now
	config.DeletedAt = nil

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("创建监测项配置: 开启事务失败: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		INSERT INTO monitor_configs (
			provider, service, channel, model, name, enabled, parent_key,
			config_blob, schema_version, version, created_at, updated_at, deleted_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)
	`, config.Provider, config.Service, config.Channel, config.Model,
		config.Name, boolToInt(config.Enabled), config.ParentKey,
		config.ConfigBlob, config.SchemaVersion, config.Version,
		config.CreatedAt, config.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return fmt.Errorf("监测项已存在: %s/%s/%s/%s",
				config.Provider, config.Service, config.Channel, config.Model)
		}
		return fmt.Errorf("创建监测项配置失败: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("获取插入 ID 失败: %w", err)
	}
	config.ID = id

	// 递增配置版本
	if err := incrementConfigVersionTx(ctx, tx, ConfigScopeMonitors, now); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("创建监测项配置: 提交事务失败: %w", err)
	}
	return nil
}

// ImportMonitorConfigs 批量导入监测项配置（事务操作）
func (s *SQLiteStorage) ImportMonitorConfigs(configs []*MonitorConfig) (*ImportResult, error) {
	result := &ImportResult{}
	if len(configs) == 0 {
		return result, nil
	}

	ctx := s.effectiveCtx()
	now := time.Now().Unix()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("导入监测项配置: 开启事务失败: %w", err)
	}
	defer tx.Rollback()

	for i, cfg := range configs {
		if cfg == nil {
			result.Errors = append(result.Errors, fmt.Sprintf("monitors[%d]: 配置为空", i))
			return result, errors.New("导入失败: 配置为空")
		}

		cfg.Provider = strings.TrimSpace(cfg.Provider)
		cfg.Service = strings.TrimSpace(cfg.Service)
		cfg.Channel = strings.TrimSpace(cfg.Channel)
		cfg.Model = strings.TrimSpace(cfg.Model)
		cfg.Name = strings.TrimSpace(cfg.Name)
		cfg.ParentKey = strings.TrimSpace(cfg.ParentKey)

		if cfg.Provider == "" || cfg.Service == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("monitors[%d]: provider/service 不能为空", i))
			return result, errors.New("导入失败: provider/service 不能为空")
		}

		cfg.Version = 1
		cfg.SchemaVersion = 1
		cfg.CreatedAt = now
		cfg.UpdatedAt = now
		cfg.DeletedAt = nil

		execResult, err := tx.ExecContext(ctx, `
			INSERT INTO monitor_configs (
				provider, service, channel, model, name, enabled, parent_key,
				config_blob, schema_version, version, created_at, updated_at, deleted_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)
		`, cfg.Provider, cfg.Service, cfg.Channel, cfg.Model,
			cfg.Name, boolToInt(cfg.Enabled), cfg.ParentKey,
			cfg.ConfigBlob, cfg.SchemaVersion, cfg.Version,
			cfg.CreatedAt, cfg.UpdatedAt)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				result.Skipped++
				continue
			}
			key := cfg.Provider + "/" + cfg.Service
			if cfg.Channel != "" {
				key += "/" + cfg.Channel
			}
			if cfg.Model != "" {
				key += "/" + cfg.Model
			}
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", key, err))
			return result, fmt.Errorf("导入失败: %w", err)
		}
		if id, err := execResult.LastInsertId(); err == nil {
			cfg.ID = id
		}
		result.Created++
	}

	if result.Created > 0 {
		if err := incrementConfigVersionTx(ctx, tx, ConfigScopeMonitors, now); err != nil {
			result.Errors = append(result.Errors, err.Error())
			return result, err
		}
	}

	if err := tx.Commit(); err != nil {
		result.Created = 0
		return result, fmt.Errorf("导入监测项配置: 提交事务失败: %w", err)
	}
	return result, nil
}

// GetMonitorConfig 按 ID 获取监测项配置
func (s *SQLiteStorage) GetMonitorConfig(id int64) (*MonitorConfig, error) {
	ctx := s.effectiveCtx()
	config := &MonitorConfig{}
	var enabled int
	var deletedAt sql.NullInt64

	err := s.db.QueryRowContext(ctx, `
		SELECT id, provider, service, channel, model, name, enabled, parent_key,
		       config_blob, schema_version, version, created_at, updated_at, deleted_at
		FROM monitor_configs
		WHERE id = ?
	`, id).Scan(
		&config.ID, &config.Provider, &config.Service, &config.Channel, &config.Model,
		&config.Name, &enabled, &config.ParentKey,
		&config.ConfigBlob, &config.SchemaVersion, &config.Version,
		&config.CreatedAt, &config.UpdatedAt, &deletedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询监测项配置失败: %w", err)
	}

	config.Enabled = enabled != 0
	if deletedAt.Valid {
		config.DeletedAt = &deletedAt.Int64
	}
	return config, nil
}

// GetMonitorConfigByKey 按四元组获取监测项配置
func (s *SQLiteStorage) GetMonitorConfigByKey(provider, service, channel, model string) (*MonitorConfig, error) {
	ctx := s.effectiveCtx()
	config := &MonitorConfig{}
	var enabled int
	var deletedAt sql.NullInt64

	err := s.db.QueryRowContext(ctx, `
		SELECT id, provider, service, channel, model, name, enabled, parent_key,
		       config_blob, schema_version, version, created_at, updated_at, deleted_at
		FROM monitor_configs
		WHERE provider = ? AND service = ? AND channel = ? AND model = ? AND deleted_at IS NULL
	`, provider, service, channel, model).Scan(
		&config.ID, &config.Provider, &config.Service, &config.Channel, &config.Model,
		&config.Name, &enabled, &config.ParentKey,
		&config.ConfigBlob, &config.SchemaVersion, &config.Version,
		&config.CreatedAt, &config.UpdatedAt, &deletedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询监测项配置失败: %w", err)
	}

	config.Enabled = enabled != 0
	if deletedAt.Valid {
		config.DeletedAt = &deletedAt.Int64
	}
	return config, nil
}

// UpdateMonitorConfig 更新监测项配置（含乐观锁）
func (s *SQLiteStorage) UpdateMonitorConfig(config *MonitorConfig) error {
	if config == nil {
		return errors.New("config 不能为空")
	}
	if config.ID <= 0 {
		return errors.New("id 不能为空")
	}
	if config.Version <= 0 {
		return errors.New("version 不能为空")
	}

	ctx := s.effectiveCtx()
	now := time.Now().Unix()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("更新监测项配置: 开启事务失败: %w", err)
	}
	defer tx.Rollback()

	// 使用乐观锁更新
	result, err := tx.ExecContext(ctx, `
		UPDATE monitor_configs SET
			name = ?, enabled = ?, parent_key = ?,
			config_blob = ?, updated_at = ?, version = version + 1
		WHERE id = ? AND version = ? AND deleted_at IS NULL
	`, config.Name, boolToInt(config.Enabled), config.ParentKey,
		config.ConfigBlob, now, config.ID, config.Version)
	if err != nil {
		return fmt.Errorf("更新监测项配置失败: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %w", err)
	}
	if affected == 0 {
		return errors.New("更新失败: 记录不存在或版本冲突")
	}

	config.Version++
	config.UpdatedAt = now

	// 递增配置版本
	if err := incrementConfigVersionTx(ctx, tx, ConfigScopeMonitors, now); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("更新监测项配置: 提交事务失败: %w", err)
	}
	return nil
}

// DeleteMonitorConfig 软删除监测项配置
func (s *SQLiteStorage) DeleteMonitorConfig(id int64) error {
	ctx := s.effectiveCtx()
	now := time.Now().Unix()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("删除监测项配置: 开启事务失败: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		UPDATE monitor_configs SET
			deleted_at = ?, updated_at = ?, version = version + 1
		WHERE id = ? AND deleted_at IS NULL
	`, now, now, id)
	if err != nil {
		return fmt.Errorf("删除监测项配置失败: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %w", err)
	}
	if affected == 0 {
		return errors.New("删除失败: 记录不存在")
	}

	// 递增配置版本
	if err := incrementConfigVersionTx(ctx, tx, ConfigScopeMonitors, now); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("删除监测项配置: 提交事务失败: %w", err)
	}
	return nil
}

// RestoreMonitorConfig 恢复软删除的监测项配置
func (s *SQLiteStorage) RestoreMonitorConfig(id int64) error {
	ctx := s.effectiveCtx()
	now := time.Now().Unix()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("恢复监测项配置: 开启事务失败: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		UPDATE monitor_configs SET
			deleted_at = NULL, updated_at = ?, version = version + 1
		WHERE id = ? AND deleted_at IS NOT NULL
	`, now, id)
	if err != nil {
		return fmt.Errorf("恢复监测项配置失败: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %w", err)
	}
	if affected == 0 {
		return errors.New("恢复失败: 记录不存在或未被删除")
	}

	// 递增配置版本
	if err := incrementConfigVersionTx(ctx, tx, ConfigScopeMonitors, now); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("恢复监测项配置: 提交事务失败: %w", err)
	}
	return nil
}

// ListMonitorConfigs 列表查询监测项配置
func (s *SQLiteStorage) ListMonitorConfigs(filter *MonitorConfigFilter) ([]*MonitorConfig, int, error) {
	ctx := s.effectiveCtx()
	whereClause, args := buildMonitorConfigWhere(filter)

	// 查询总数
	countQuery := "SELECT COUNT(1) FROM monitor_configs WHERE " + whereClause
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("查询监测项配置总数失败: %w", err)
	}

	// 分页查询
	limit, offset := normalizeLimitOffset(filter)

	var listQuery string
	var listArgs []interface{}

	if limit == 0 {
		// 无限制查询（用于导出等场景）
		listQuery = fmt.Sprintf(`
			SELECT id, provider, service, channel, model, name, enabled, parent_key,
			       config_blob, schema_version, version, created_at, updated_at, deleted_at
			FROM monitor_configs
			WHERE %s
			ORDER BY updated_at DESC, id DESC
		`, whereClause)
		listArgs = args
	} else {
		// 带分页查询
		listQuery = fmt.Sprintf(`
			SELECT id, provider, service, channel, model, name, enabled, parent_key,
			       config_blob, schema_version, version, created_at, updated_at, deleted_at
			FROM monitor_configs
			WHERE %s
			ORDER BY updated_at DESC, id DESC
			LIMIT ? OFFSET ?
		`, whereClause)
		listArgs = append(args, limit, offset)
	}

	rows, err := s.db.QueryContext(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询监测项配置列表失败: %w", err)
	}
	defer rows.Close()

	var configs []*MonitorConfig
	for rows.Next() {
		config := &MonitorConfig{}
		var enabled int
		var deletedAt sql.NullInt64

		if err := rows.Scan(
			&config.ID, &config.Provider, &config.Service, &config.Channel, &config.Model,
			&config.Name, &enabled, &config.ParentKey,
			&config.ConfigBlob, &config.SchemaVersion, &config.Version,
			&config.CreatedAt, &config.UpdatedAt, &deletedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("扫描监测项配置失败: %w", err)
		}

		config.Enabled = enabled != 0
		if deletedAt.Valid {
			config.DeletedAt = &deletedAt.Int64
		}
		configs = append(configs, config)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("迭代监测项配置失败: %w", err)
	}

	return configs, total, nil
}

// ===== MonitorSecret CRUD =====

// SetMonitorSecret 设置或更新监测项密钥
func (s *SQLiteStorage) SetMonitorSecret(monitorID int64, ciphertext, nonce []byte, keyVersion, encVersion int) error {
	if monitorID <= 0 {
		return errors.New("monitorID 不能为空")
	}
	if len(ciphertext) == 0 {
		return errors.New("ciphertext 不能为空")
	}
	if len(nonce) == 0 {
		return errors.New("nonce 不能为空")
	}
	if keyVersion <= 0 {
		return errors.New("keyVersion 必须 > 0")
	}
	if encVersion <= 0 {
		encVersion = 1
	}

	ctx := s.effectiveCtx()
	now := time.Now().Unix()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO monitor_secrets (monitor_id, api_key_ciphertext, api_key_nonce, key_version, enc_version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(monitor_id) DO UPDATE SET
			api_key_ciphertext = excluded.api_key_ciphertext,
			api_key_nonce = excluded.api_key_nonce,
			key_version = excluded.key_version,
			enc_version = excluded.enc_version,
			updated_at = excluded.updated_at
	`, monitorID, ciphertext, nonce, keyVersion, encVersion, now, now)
	if err != nil {
		return fmt.Errorf("设置监测项密钥失败: %w", err)
	}
	return nil
}

// GetMonitorSecret 获取监测项密钥
func (s *SQLiteStorage) GetMonitorSecret(monitorID int64) (*MonitorSecret, error) {
	ctx := s.effectiveCtx()
	secret := &MonitorSecret{}

	err := s.db.QueryRowContext(ctx, `
		SELECT monitor_id, api_key_ciphertext, api_key_nonce, key_version, enc_version, created_at, updated_at
		FROM monitor_secrets
		WHERE monitor_id = ?
	`, monitorID).Scan(
		&secret.MonitorID, &secret.APIKeyCiphertext, &secret.APIKeyNonce,
		&secret.KeyVersion, &secret.EncVersion, &secret.CreatedAt, &secret.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询监测项密钥失败: %w", err)
	}
	return secret, nil
}

// DeleteMonitorSecret 删除监测项密钥
func (s *SQLiteStorage) DeleteMonitorSecret(monitorID int64) error {
	ctx := s.effectiveCtx()

	result, err := s.db.ExecContext(ctx, `
		DELETE FROM monitor_secrets WHERE monitor_id = ?
	`, monitorID)
	if err != nil {
		return fmt.Errorf("删除监测项密钥失败: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %w", err)
	}
	if affected == 0 {
		return errors.New("删除失败: 密钥不存在")
	}
	return nil
}

// ===== MonitorConfigAudit CRUD =====

// CreateMonitorConfigAudit 创建审计记录
func (s *SQLiteStorage) CreateMonitorConfigAudit(audit *MonitorConfigAudit) error {
	if audit == nil {
		return errors.New("audit 不能为空")
	}

	ctx := s.effectiveCtx()
	now := time.Now().Unix()

	if audit.CreatedAt == 0 {
		audit.CreatedAt = now
	}

	result, err := s.db.ExecContext(ctx, `
		INSERT INTO monitor_config_audits (
			monitor_id, provider, service, channel, model, action,
			before_blob, after_blob, before_version, after_version, secret_changed,
			actor, actor_ip, user_agent, request_id, batch_id, reason, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, audit.MonitorID, audit.Provider, audit.Service, audit.Channel, audit.Model,
		string(audit.Action), audit.BeforeBlob, audit.AfterBlob,
		audit.BeforeVersion, audit.AfterVersion, boolToInt(audit.SecretChanged),
		audit.Actor, audit.ActorIP, audit.UserAgent, audit.RequestID, audit.BatchID,
		audit.Reason, audit.CreatedAt)
	if err != nil {
		return fmt.Errorf("创建审计记录失败: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("获取审计记录 ID 失败: %w", err)
	}
	audit.ID = id
	return nil
}

// ListMonitorConfigAudits 查询审计记录列表
func (s *SQLiteStorage) ListMonitorConfigAudits(filter *AuditFilter) ([]*MonitorConfigAudit, int, error) {
	ctx := s.effectiveCtx()
	whereClause, args := buildAuditWhere(filter)

	// 查询总数
	countQuery := "SELECT COUNT(1) FROM monitor_config_audits WHERE " + whereClause
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("查询审计记录总数失败: %w", err)
	}

	// 分页查询
	limit, offset := normalizeAuditLimitOffset(filter)
	listQuery := fmt.Sprintf(`
		SELECT id, monitor_id, provider, service, channel, model, action,
		       before_blob, after_blob, before_version, after_version, secret_changed,
		       actor, actor_ip, user_agent, request_id, batch_id, reason, created_at
		FROM monitor_config_audits
		WHERE %s
		ORDER BY created_at DESC, id DESC
		LIMIT ? OFFSET ?
	`, whereClause)

	listArgs := append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询审计记录列表失败: %w", err)
	}
	defer rows.Close()

	var audits []*MonitorConfigAudit
	for rows.Next() {
		audit := &MonitorConfigAudit{}
		var action string
		var beforeVersion, afterVersion sql.NullInt64
		var secretChanged int
		var actor, actorIP, userAgent, requestID, batchID, reason sql.NullString

		if err := rows.Scan(
			&audit.ID, &audit.MonitorID, &audit.Provider, &audit.Service, &audit.Channel, &audit.Model,
			&action, &audit.BeforeBlob, &audit.AfterBlob, &beforeVersion, &afterVersion, &secretChanged,
			&actor, &actorIP, &userAgent, &requestID, &batchID, &reason, &audit.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("扫描审计记录失败: %w", err)
		}

		audit.Action = AuditAction(action)
		audit.SecretChanged = secretChanged != 0
		if beforeVersion.Valid {
			audit.BeforeVersion = &beforeVersion.Int64
		}
		if afterVersion.Valid {
			audit.AfterVersion = &afterVersion.Int64
		}
		if actor.Valid {
			audit.Actor = actor.String
		}
		if actorIP.Valid {
			audit.ActorIP = actorIP.String
		}
		if userAgent.Valid {
			audit.UserAgent = userAgent.String
		}
		if requestID.Valid {
			audit.RequestID = requestID.String
		}
		if batchID.Valid {
			audit.BatchID = batchID.String
		}
		if reason.Valid {
			audit.Reason = reason.String
		}

		audits = append(audits, audit)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("迭代审计记录失败: %w", err)
	}

	return audits, total, nil
}

// ===== ConfigMeta CRUD =====

// GetConfigVersions 获取所有 scope 的配置版本
func (s *SQLiteStorage) GetConfigVersions() (*ConfigVersions, error) {
	ctx := s.effectiveCtx()

	rows, err := s.db.QueryContext(ctx, `
		SELECT scope, version, updated_at FROM config_meta ORDER BY scope
	`)
	if err != nil {
		return nil, fmt.Errorf("查询配置版本失败: %w", err)
	}
	defer rows.Close()

	versions := &ConfigVersions{}
	for rows.Next() {
		var scope string
		var version, updatedAt int64

		if err := rows.Scan(&scope, &version, &updatedAt); err != nil {
			return nil, fmt.Errorf("扫描配置版本失败: %w", err)
		}

		switch ConfigScope(scope) {
		case ConfigScopeMonitors:
			versions.Monitors = version
		case ConfigScopePolicies:
			versions.Policies = version
		case ConfigScopeBadges:
			versions.Badges = version
		case ConfigScopeBoards:
			versions.Boards = version
		case ConfigScopeSettings:
			versions.Settings = version
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("迭代配置版本失败: %w", err)
	}

	return versions, nil
}

// IncrementConfigVersion 递增指定 scope 的配置版本
func (s *SQLiteStorage) IncrementConfigVersion(scope ConfigScope) error {
	ctx := s.effectiveCtx()
	now := time.Now().Unix()

	result, err := s.db.ExecContext(ctx, `
		UPDATE config_meta SET version = version + 1, updated_at = ? WHERE scope = ?
	`, now, string(scope))
	if err != nil {
		return fmt.Errorf("递增配置版本失败: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %w", err)
	}
	if affected == 0 {
		// scope 不存在，插入新记录
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO config_meta (scope, version, updated_at) VALUES (?, 1, ?)
		`, string(scope), now)
		if err != nil {
			return fmt.Errorf("创建配置版本记录失败: %w", err)
		}
	}
	return nil
}

// incrementConfigVersionTx 在事务中递增配置版本
func incrementConfigVersionTx(ctx context.Context, tx *sql.Tx, scope ConfigScope, now int64) error {
	result, err := tx.ExecContext(ctx, `
		UPDATE config_meta SET version = version + 1, updated_at = ? WHERE scope = ?
	`, now, string(scope))
	if err != nil {
		return fmt.Errorf("递增配置版本失败: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %w", err)
	}
	if affected == 0 {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO config_meta (scope, version, updated_at) VALUES (?, 1, ?)
		`, string(scope), now)
		if err != nil {
			return fmt.Errorf("创建配置版本记录失败: %w", err)
		}
	}
	return nil
}

// ===== 辅助函数 =====

// buildMonitorConfigWhere 构建监测项配置查询条件
func buildMonitorConfigWhere(filter *MonitorConfigFilter) (string, []any) {
	clauses := []string{"1=1"}
	args := []any{}

	if filter == nil || !filter.IncludeDeleted {
		clauses = append(clauses, "deleted_at IS NULL")
	}

	if filter == nil {
		return strings.Join(clauses, " AND "), args
	}

	if v := strings.TrimSpace(filter.Provider); v != "" {
		clauses = append(clauses, "provider = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.Service); v != "" {
		clauses = append(clauses, "service = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.Channel); v != "" {
		clauses = append(clauses, "channel = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.Model); v != "" {
		clauses = append(clauses, "model = ?")
		args = append(args, v)
	}
	if filter.Enabled != nil {
		clauses = append(clauses, "enabled = ?")
		args = append(args, boolToInt(*filter.Enabled))
	}
	if v := strings.TrimSpace(filter.Search); v != "" {
		clauses = append(clauses, "(name LIKE ? OR provider LIKE ? OR service LIKE ? OR channel LIKE ?)")
		pattern := "%" + v + "%"
		args = append(args, pattern, pattern, pattern, pattern)
	}

	return strings.Join(clauses, " AND "), args
}

// buildAuditWhere 构建审计记录查询条件
func buildAuditWhere(filter *AuditFilter) (string, []any) {
	clauses := []string{"1=1"}
	args := []any{}

	if filter == nil {
		return strings.Join(clauses, " AND "), args
	}

	if filter.MonitorID > 0 {
		clauses = append(clauses, "monitor_id = ?")
		args = append(args, filter.MonitorID)
	}
	if v := strings.TrimSpace(filter.Provider); v != "" {
		clauses = append(clauses, "provider = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.Service); v != "" {
		clauses = append(clauses, "service = ?")
		args = append(args, v)
	}
	if filter.Action != "" {
		clauses = append(clauses, "action = ?")
		args = append(args, string(filter.Action))
	}
	if v := strings.TrimSpace(filter.Actor); v != "" {
		clauses = append(clauses, "actor = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.RequestID); v != "" {
		clauses = append(clauses, "request_id = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.BatchID); v != "" {
		clauses = append(clauses, "batch_id = ?")
		args = append(args, v)
	}
	if filter.Since > 0 {
		clauses = append(clauses, "created_at >= ?")
		args = append(args, filter.Since)
	}
	if filter.Until > 0 {
		clauses = append(clauses, "created_at <= ?")
		args = append(args, filter.Until)
	}

	return strings.Join(clauses, " AND "), args
}

// normalizeLimitOffset 规范化分页参数
// Limit = -1 表示无限制（内部使用，如导出）
func normalizeLimitOffset(filter *MonitorConfigFilter) (int, int) {
	limit := 100
	offset := 0

	if filter != nil {
		if filter.Limit == -1 {
			// 内部使用：无限制
			limit = 0 // SQL 中 LIMIT 0 或省略表示不限制
		} else if filter.Limit > 0 && filter.Limit <= 500 {
			limit = filter.Limit
		}
		if filter.Offset > 0 {
			offset = filter.Offset
		}
	}
	return limit, offset
}

// normalizeAuditLimitOffset 规范化审计记录分页参数
func normalizeAuditLimitOffset(filter *AuditFilter) (int, int) {
	limit := 100
	offset := 0

	if filter != nil {
		if filter.Limit > 0 && filter.Limit <= 500 {
			limit = filter.Limit
		}
		if filter.Offset > 0 {
			offset = filter.Offset
		}
	}
	return limit, offset
}

// boolToInt 将 bool 转换为 SQLite 兼容的 int
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ===== ProviderPolicy CRUD =====

// ListProviderPolicies 列出所有 Provider 策略
func (s *SQLiteStorage) ListProviderPolicies() ([]*ProviderPolicy, error) {
	ctx := s.effectiveCtx()

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, policy_type, provider, reason, risks, created_at, updated_at
		FROM provider_policies
		ORDER BY policy_type, provider
	`)
	if err != nil {
		return nil, fmt.Errorf("查询 Provider 策略失败: %w", err)
	}
	defer rows.Close()

	var policies []*ProviderPolicy
	for rows.Next() {
		p := &ProviderPolicy{}
		var reason, risks sql.NullString

		if err := rows.Scan(&p.ID, &p.PolicyType, &p.Provider, &reason, &risks, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("扫描 Provider 策略失败: %w", err)
		}

		if reason.Valid {
			p.Reason = reason.String
		}
		if risks.Valid {
			p.Risks = risks.String
		}
		policies = append(policies, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("迭代 Provider 策略失败: %w", err)
	}

	return policies, nil
}

// CreateProviderPolicy 创建 Provider 策略
func (s *SQLiteStorage) CreateProviderPolicy(policy *ProviderPolicy) error {
	if policy == nil {
		return errors.New("policy 不能为空")
	}
	if strings.TrimSpace(policy.Provider) == "" {
		return errors.New("provider 不能为空")
	}
	if policy.PolicyType == "" {
		return errors.New("policy_type 不能为空")
	}

	ctx := s.effectiveCtx()
	now := time.Now().Unix()

	policy.CreatedAt = now
	policy.UpdatedAt = now

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("创建 Provider 策略: 开启事务失败: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		INSERT INTO provider_policies (policy_type, provider, reason, risks, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, string(policy.PolicyType), policy.Provider, policy.Reason, policy.Risks, policy.CreatedAt, policy.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return fmt.Errorf("策略已存在: %s/%s", policy.PolicyType, policy.Provider)
		}
		return fmt.Errorf("创建 Provider 策略失败: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("获取插入 ID 失败: %w", err)
	}
	policy.ID = id

	// 递增配置版本
	if err := incrementConfigVersionTx(ctx, tx, ConfigScopePolicies, now); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("创建 Provider 策略: 提交事务失败: %w", err)
	}
	return nil
}

// DeleteProviderPolicy 删除 Provider 策略
func (s *SQLiteStorage) DeleteProviderPolicy(id int64) error {
	ctx := s.effectiveCtx()
	now := time.Now().Unix()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("删除 Provider 策略: 开启事务失败: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `DELETE FROM provider_policies WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("删除 Provider 策略失败: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %w", err)
	}
	if affected == 0 {
		return errors.New("删除失败: 策略不存在")
	}

	// 递增配置版本
	if err := incrementConfigVersionTx(ctx, tx, ConfigScopePolicies, now); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("删除 Provider 策略: 提交事务失败: %w", err)
	}
	return nil
}

// ===== BadgeDefinition CRUD =====

// ListBadgeDefinitions 列出所有徽标定义
func (s *SQLiteStorage) ListBadgeDefinitions() ([]*BadgeDefinition, error) {
	ctx := s.effectiveCtx()

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, kind, weight, label_i18n, tooltip_i18n, icon, color, created_at, updated_at
		FROM badge_definitions
		ORDER BY weight DESC, id
	`)
	if err != nil {
		return nil, fmt.Errorf("查询徽标定义失败: %w", err)
	}
	defer rows.Close()

	var badges []*BadgeDefinition
	for rows.Next() {
		b := &BadgeDefinition{}
		var tooltipI18n, icon, color sql.NullString

		if err := rows.Scan(&b.ID, &b.Kind, &b.Weight, &b.LabelI18n, &tooltipI18n, &icon, &color, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("扫描徽标定义失败: %w", err)
		}

		if tooltipI18n.Valid {
			b.TooltipI18n = tooltipI18n.String
		}
		if icon.Valid {
			b.Icon = icon.String
		}
		if color.Valid {
			b.Color = color.String
		}
		badges = append(badges, b)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("迭代徽标定义失败: %w", err)
	}

	return badges, nil
}

// CreateBadgeDefinition 创建徽标定义
func (s *SQLiteStorage) CreateBadgeDefinition(badge *BadgeDefinition) error {
	if badge == nil {
		return errors.New("badge 不能为空")
	}
	if strings.TrimSpace(badge.ID) == "" {
		return errors.New("id 不能为空")
	}
	if badge.Kind == "" {
		return errors.New("kind 不能为空")
	}
	if strings.TrimSpace(badge.LabelI18n) == "" {
		return errors.New("label_i18n 不能为空")
	}

	ctx := s.effectiveCtx()
	now := time.Now().Unix()

	badge.CreatedAt = now
	badge.UpdatedAt = now

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("创建徽标定义: 开启事务失败: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO badge_definitions (id, kind, weight, label_i18n, tooltip_i18n, icon, color, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, badge.ID, string(badge.Kind), badge.Weight, badge.LabelI18n, badge.TooltipI18n, badge.Icon, badge.Color, badge.CreatedAt, badge.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "PRIMARY KEY constraint failed") {
			return fmt.Errorf("徽标已存在: %s", badge.ID)
		}
		return fmt.Errorf("创建徽标定义失败: %w", err)
	}

	// 递增配置版本
	if err := incrementConfigVersionTx(ctx, tx, ConfigScopeBadges, now); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("创建徽标定义: 提交事务失败: %w", err)
	}
	return nil
}

// DeleteBadgeDefinition 删除徽标定义
func (s *SQLiteStorage) DeleteBadgeDefinition(id string) error {
	ctx := s.effectiveCtx()
	now := time.Now().Unix()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("删除徽标定义: 开启事务失败: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `DELETE FROM badge_definitions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("删除徽标定义失败: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %w", err)
	}
	if affected == 0 {
		return errors.New("删除失败: 徽标不存在")
	}

	// 递增配置版本
	if err := incrementConfigVersionTx(ctx, tx, ConfigScopeBadges, now); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("删除徽标定义: 提交事务失败: %w", err)
	}
	return nil
}

// ===== BadgeBinding CRUD =====

// BadgeBindingFilter 徽标绑定查询过滤器
type BadgeBindingFilter struct {
	BadgeID  string     `json:"badge_id,omitempty"`
	Scope    BadgeScope `json:"scope,omitempty"`
	Provider string     `json:"provider,omitempty"`
	Service  string     `json:"service,omitempty"`
	Channel  string     `json:"channel,omitempty"`
}

// ListBadgeBindings 列出徽标绑定
func (s *SQLiteStorage) ListBadgeBindings(filter *BadgeBindingFilter) ([]*BadgeBinding, error) {
	ctx := s.effectiveCtx()

	clauses := []string{"1=1"}
	args := []any{}

	if filter != nil {
		if v := strings.TrimSpace(filter.BadgeID); v != "" {
			clauses = append(clauses, "badge_id = ?")
			args = append(args, v)
		}
		if filter.Scope != "" {
			clauses = append(clauses, "scope = ?")
			args = append(args, string(filter.Scope))
		}
		if v := strings.TrimSpace(filter.Provider); v != "" {
			clauses = append(clauses, "provider = ?")
			args = append(args, v)
		}
		if v := strings.TrimSpace(filter.Service); v != "" {
			clauses = append(clauses, "service = ?")
			args = append(args, v)
		}
		if v := strings.TrimSpace(filter.Channel); v != "" {
			clauses = append(clauses, "channel = ?")
			args = append(args, v)
		}
	}

	query := fmt.Sprintf(`
		SELECT id, badge_id, scope, provider, service, channel, tooltip_override, created_at, updated_at
		FROM badge_bindings
		WHERE %s
		ORDER BY scope, provider, service, channel
	`, strings.Join(clauses, " AND "))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询徽标绑定失败: %w", err)
	}
	defer rows.Close()

	var bindings []*BadgeBinding
	for rows.Next() {
		b := &BadgeBinding{}
		var provider, service, channel, tooltipOverride sql.NullString

		if err := rows.Scan(&b.ID, &b.BadgeID, &b.Scope, &provider, &service, &channel, &tooltipOverride, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("扫描徽标绑定失败: %w", err)
		}

		if provider.Valid {
			b.Provider = provider.String
		}
		if service.Valid {
			b.Service = service.String
		}
		if channel.Valid {
			b.Channel = channel.String
		}
		if tooltipOverride.Valid {
			b.TooltipOverride = tooltipOverride.String
		}
		bindings = append(bindings, b)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("迭代徽标绑定失败: %w", err)
	}

	return bindings, nil
}

// CreateBadgeBinding 创建徽标绑定
func (s *SQLiteStorage) CreateBadgeBinding(binding *BadgeBinding) error {
	if binding == nil {
		return errors.New("binding 不能为空")
	}
	if strings.TrimSpace(binding.BadgeID) == "" {
		return errors.New("badge_id 不能为空")
	}
	if binding.Scope == "" {
		return errors.New("scope 不能为空")
	}

	ctx := s.effectiveCtx()
	now := time.Now().Unix()

	binding.CreatedAt = now
	binding.UpdatedAt = now

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("创建徽标绑定: 开启事务失败: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		INSERT INTO badge_bindings (badge_id, scope, provider, service, channel, tooltip_override, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, binding.BadgeID, string(binding.Scope), nullString(binding.Provider), nullString(binding.Service), nullString(binding.Channel), nullString(binding.TooltipOverride), binding.CreatedAt, binding.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return errors.New("徽标绑定已存在")
		}
		return fmt.Errorf("创建徽标绑定失败: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("获取插入 ID 失败: %w", err)
	}
	binding.ID = id

	// 递增配置版本
	if err := incrementConfigVersionTx(ctx, tx, ConfigScopeBadges, now); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("创建徽标绑定: 提交事务失败: %w", err)
	}
	return nil
}

// DeleteBadgeBinding 删除徽标绑定
func (s *SQLiteStorage) DeleteBadgeBinding(id int64) error {
	ctx := s.effectiveCtx()
	now := time.Now().Unix()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("删除徽标绑定: 开启事务失败: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `DELETE FROM badge_bindings WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("删除徽标绑定失败: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %w", err)
	}
	if affected == 0 {
		return errors.New("删除失败: 绑定不存在")
	}

	// 递增配置版本
	if err := incrementConfigVersionTx(ctx, tx, ConfigScopeBadges, now); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("删除徽标绑定: 提交事务失败: %w", err)
	}
	return nil
}

// ===== BoardConfig CRUD =====

// ListBoardConfigs 列出所有 Board 配置
func (s *SQLiteStorage) ListBoardConfigs() ([]*BoardConfig, error) {
	ctx := s.effectiveCtx()

	rows, err := s.db.QueryContext(ctx, `
		SELECT board, display_name, description, sort_order, created_at, updated_at
		FROM board_configs
		ORDER BY sort_order, board
	`)
	if err != nil {
		return nil, fmt.Errorf("查询 Board 配置失败: %w", err)
	}
	defer rows.Close()

	var configs []*BoardConfig
	for rows.Next() {
		c := &BoardConfig{}
		var desc sql.NullString

		if err := rows.Scan(&c.Board, &c.DisplayName, &desc, &c.SortOrder, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("扫描 Board 配置失败: %w", err)
		}

		if desc.Valid {
			c.Description = desc.String
		}
		configs = append(configs, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("迭代 Board 配置失败: %w", err)
	}

	return configs, nil
}

// ===== GlobalSetting CRUD =====

// GetGlobalSetting 获取全局设置
func (s *SQLiteStorage) GetGlobalSetting(key string) (*GlobalSetting, error) {
	ctx := s.effectiveCtx()
	setting := &GlobalSetting{}

	err := s.db.QueryRowContext(ctx, `
		SELECT key, value, schema_version, version, created_at, updated_at
		FROM global_settings
		WHERE key = ?
	`, key).Scan(&setting.Key, &setting.Value, &setting.SchemaVersion, &setting.Version, &setting.CreatedAt, &setting.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询全局设置失败: %w", err)
	}
	return setting, nil
}

// SetGlobalSetting 设置全局设置（upsert）
func (s *SQLiteStorage) SetGlobalSetting(key, value string) error {
	if strings.TrimSpace(key) == "" {
		return errors.New("key 不能为空")
	}

	ctx := s.effectiveCtx()
	now := time.Now().Unix()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("设置全局设置: 开启事务失败: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO global_settings (key, value, schema_version, version, created_at, updated_at)
		VALUES (?, ?, 1, 1, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			updated_at = excluded.updated_at,
			version = version + 1
	`, key, value, now, now)
	if err != nil {
		return fmt.Errorf("设置全局设置失败: %w", err)
	}

	// 递增配置版本
	if err := incrementConfigVersionTx(ctx, tx, ConfigScopeSettings, now); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("设置全局设置: 提交事务失败: %w", err)
	}
	return nil
}

// nullString 将空字符串转换为 SQL NULL
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}
