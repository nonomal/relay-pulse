package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"monitor/internal/logger"
)

// ===== 配置管理表初始化 (PostgreSQL) =====

// initAdminConfigTables 初始化配置管理相关的数据库表
// 包括 10 张表：monitor_configs, monitor_secrets, monitor_config_audits,
// provider_policies, badge_definitions, badge_bindings,
// board_configs, board_items, global_settings, config_meta
func (s *PostgresStorage) initAdminConfigTables(ctx context.Context) error {
	statements := []string{
		// monitor_configs 监测项配置表
		`CREATE TABLE IF NOT EXISTS monitor_configs (
			id BIGSERIAL PRIMARY KEY,
			provider TEXT NOT NULL,
			service TEXT NOT NULL,
			channel TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL DEFAULT '',
			enabled BOOLEAN NOT NULL DEFAULT true,
			parent_key TEXT NOT NULL DEFAULT '',
			config_blob TEXT NOT NULL,
			schema_version INTEGER NOT NULL DEFAULT 1,
			version BIGINT NOT NULL DEFAULT 1,
			created_at BIGINT NOT NULL,
			updated_at BIGINT NOT NULL,
			deleted_at BIGINT
		)`,
		// 四元组唯一约束（仅对未删除记录）
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_monitor_configs_key
		 ON monitor_configs(provider, service, channel, model) WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_monitor_configs_parent ON monitor_configs(parent_key) WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_monitor_configs_enabled ON monitor_configs(enabled) WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_monitor_configs_updated ON monitor_configs(updated_at DESC)`,

		// monitor_secrets API Key 加密存储表
		`CREATE TABLE IF NOT EXISTS monitor_secrets (
			monitor_id BIGINT PRIMARY KEY,
			api_key_ciphertext BYTEA NOT NULL,
			api_key_nonce BYTEA NOT NULL,
			key_version INTEGER NOT NULL DEFAULT 1,
			enc_version INTEGER NOT NULL DEFAULT 1,
			created_at BIGINT NOT NULL,
			updated_at BIGINT NOT NULL
		)`,

		// monitor_config_audits 配置审计表
		`CREATE TABLE IF NOT EXISTS monitor_config_audits (
			id BIGSERIAL PRIMARY KEY,
			monitor_id BIGINT NOT NULL,
			provider TEXT NOT NULL,
			service TEXT NOT NULL,
			channel TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			action TEXT NOT NULL,
			before_blob TEXT,
			after_blob TEXT,
			before_version BIGINT,
			after_version BIGINT,
			secret_changed BOOLEAN NOT NULL DEFAULT false,
			actor TEXT,
			actor_ip TEXT,
			user_agent TEXT,
			request_id TEXT,
			batch_id TEXT,
			reason TEXT,
			created_at BIGINT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_monitor ON monitor_config_audits(monitor_id)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_time ON monitor_config_audits(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_request ON monitor_config_audits(request_id) WHERE request_id IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_audit_batch ON monitor_config_audits(batch_id) WHERE batch_id IS NOT NULL`,

		// provider_policies Provider 策略表
		`CREATE TABLE IF NOT EXISTS provider_policies (
			id BIGSERIAL PRIMARY KEY,
			policy_type TEXT NOT NULL CHECK (policy_type IN ('disabled','hidden','risk')),
			provider TEXT NOT NULL,
			reason TEXT,
			risks TEXT,
			created_at BIGINT NOT NULL,
			updated_at BIGINT NOT NULL
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_provider_policies ON provider_policies(policy_type, provider)`,

		// badge_definitions 徽标定义表
		`CREATE TABLE IF NOT EXISTS badge_definitions (
			id TEXT PRIMARY KEY,
			kind TEXT NOT NULL CHECK (kind IN ('source','sponsor','risk','feature','info')),
			weight INTEGER NOT NULL DEFAULT 0,
			label_i18n TEXT NOT NULL,
			tooltip_i18n TEXT,
			icon TEXT,
			color TEXT,
			category TEXT,
			svg_source TEXT,
			created_at BIGINT NOT NULL,
			updated_at BIGINT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_badge_defs_kind ON badge_definitions(kind)`,
		// 迁移：为已存在的表添加新列
		`ALTER TABLE badge_definitions ADD COLUMN IF NOT EXISTS category TEXT`,
		`ALTER TABLE badge_definitions ADD COLUMN IF NOT EXISTS svg_source TEXT`,

		// badge_bindings 徽标绑定表
		`CREATE TABLE IF NOT EXISTS badge_bindings (
			id BIGSERIAL PRIMARY KEY,
			badge_id TEXT NOT NULL,
			scope TEXT NOT NULL CHECK (scope IN ('global','provider','service','channel')),
			provider TEXT,
			service TEXT,
			channel TEXT,
			tooltip_override TEXT,
			created_at BIGINT NOT NULL,
			updated_at BIGINT NOT NULL,
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
			created_at BIGINT NOT NULL,
			updated_at BIGINT NOT NULL
		)`,

		// board_items Board 成员关联表
		`CREATE TABLE IF NOT EXISTS board_items (
			id BIGSERIAL PRIMARY KEY,
			board TEXT NOT NULL,
			monitor_id BIGINT NOT NULL,
			sort_order INTEGER NOT NULL DEFAULT 0,
			created_at BIGINT NOT NULL,
			updated_at BIGINT NOT NULL
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_board_items ON board_items(board, monitor_id)`,
		`CREATE INDEX IF NOT EXISTS idx_board_items_order ON board_items(board, sort_order, id)`,

		// global_settings 全局键值配置表
		`CREATE TABLE IF NOT EXISTS global_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			schema_version INTEGER NOT NULL DEFAULT 1,
			version BIGINT NOT NULL DEFAULT 1,
			created_at BIGINT NOT NULL,
			updated_at BIGINT NOT NULL
		)`,

		// config_meta 配置版本元数据表
		`CREATE TABLE IF NOT EXISTS config_meta (
			scope TEXT PRIMARY KEY,
			version BIGINT NOT NULL DEFAULT 1,
			updated_at BIGINT NOT NULL
		)`,
	}

	for i, stmt := range statements {
		if _, err := s.pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("初始化配置管理表: 执行语句 %d 失败: %w", i+1, err)
		}
	}

	// 迁移：更新 badge_definitions 的 kind 约束（支持 'source' 类型）
	if err := s.migrateBadgeKindConstraint(ctx); err != nil {
		return fmt.Errorf("迁移 badge_definitions 约束失败: %w", err)
	}

	// 初始化默认数据
	now := time.Now().Unix()
	if err := s.seedConfigMeta(ctx, now); err != nil {
		return err
	}
	if err := s.seedBoardConfigs(ctx, now); err != nil {
		return err
	}

	return nil
}

// seedConfigMeta 初始化 config_meta 默认数据
func (s *PostgresStorage) seedConfigMeta(ctx context.Context, now int64) error {
	defaultScopes := []ConfigScope{
		ConfigScopeMonitors,
		ConfigScopePolicies,
		ConfigScopeBadges,
		ConfigScopeBoards,
		ConfigScopeSettings,
	}

	for _, scope := range defaultScopes {
		var exists int
		err := s.pool.QueryRow(ctx,
			`SELECT 1 FROM config_meta WHERE scope = $1`, string(scope)).Scan(&exists)
		if err == nil {
			continue // 已存在
		}
		if err != pgx.ErrNoRows {
			return fmt.Errorf("检查 config_meta 失败: %w", err)
		}

		_, err = s.pool.Exec(ctx,
			`INSERT INTO config_meta (scope, version, updated_at) VALUES ($1, 1, $2)`,
			string(scope), now)
		if err != nil {
			return fmt.Errorf("初始化 config_meta 失败: %w", err)
		}
	}
	return nil
}

// seedBoardConfigs 初始化 board_configs 默认数据
func (s *PostgresStorage) seedBoardConfigs(ctx context.Context, now int64) error {
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
		err := s.pool.QueryRow(ctx,
			`SELECT 1 FROM board_configs WHERE board = $1`, b.board).Scan(&exists)
		if err == nil {
			continue // 已存在
		}
		if err != pgx.ErrNoRows {
			return fmt.Errorf("检查 board_configs 失败: %w", err)
		}

		_, err = s.pool.Exec(ctx,
			`INSERT INTO board_configs (board, display_name, description, sort_order, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			b.board, b.displayName, b.description, b.sortOrder, now, now)
		if err != nil {
			return fmt.Errorf("初始化 board_configs 失败: %w", err)
		}
	}
	return nil
}

// migrateBadgeKindConstraint 迁移 badge_definitions 的 kind 约束
// 确保约束包含 'source' 类型（用于已有数据库的在线迁移）
func (s *PostgresStorage) migrateBadgeKindConstraint(ctx context.Context) error {
	// 检查约束定义是否已包含 'source'
	var constraintDef string
	err := s.pool.QueryRow(ctx, `
		SELECT pg_get_constraintdef(c.oid)
		FROM pg_constraint c
		JOIN pg_class t ON c.conrelid = t.oid
		WHERE t.relname = 'badge_definitions'
		  AND c.conname = 'badge_definitions_kind_check'
	`).Scan(&constraintDef)

	if err != nil {
		if err == pgx.ErrNoRows {
			// 约束不存在（新表由 CREATE TABLE 创建时已包含正确约束）
			return nil
		}
		return fmt.Errorf("查询约束定义失败: %w", err)
	}

	// 如果约束已包含 'source'，无需迁移
	if strings.Contains(constraintDef, "'source'") {
		return nil
	}

	// 旧约束不包含 'source'，执行迁移
	logger.Info("storage", "迁移 badge_definitions.kind 约束: 添加 'source' 类型支持")

	// 删除旧约束并添加新约束
	_, err = s.pool.Exec(ctx, `
		ALTER TABLE badge_definitions DROP CONSTRAINT IF EXISTS badge_definitions_kind_check;
		ALTER TABLE badge_definitions ADD CONSTRAINT badge_definitions_kind_check
			CHECK (kind IN ('source','sponsor','risk','feature','info'));
	`)
	if err != nil {
		return fmt.Errorf("更新约束失败: %w", err)
	}

	logger.Info("storage", "badge_definitions.kind 约束迁移完成")
	return nil
}

// ===== MonitorConfig CRUD (PostgreSQL) =====

// CreateMonitorConfig 创建监测项配置
func (s *PostgresStorage) CreateMonitorConfig(config *MonitorConfig) error {
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

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("创建监测项配置: 开启事务失败: %w", err)
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
		INSERT INTO monitor_configs (
			provider, service, channel, model, name, enabled, parent_key,
			config_blob, schema_version, version, created_at, updated_at, deleted_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id`,
		config.Provider, config.Service, config.Channel, config.Model,
		config.Name, config.Enabled, config.ParentKey, config.ConfigBlob,
		config.SchemaVersion, config.Version, config.CreatedAt, config.UpdatedAt, config.DeletedAt,
	).Scan(&config.ID)
	if err != nil {
		if strings.Contains(err.Error(), "uq_monitor_configs_key") {
			return fmt.Errorf("创建监测项配置: 四元组已存在 (%s/%s/%s/%s)",
				config.Provider, config.Service, config.Channel, config.Model)
		}
		return fmt.Errorf("创建监测项配置: 插入失败: %w", err)
	}

	// 递增 monitors scope 版本
	if err := s.incrementConfigVersionTx(ctx, tx, ConfigScopeMonitors); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("创建监测项配置: 提交事务失败: %w", err)
	}

	return nil
}

// GetMonitorConfig 根据 ID 获取监测项配置
func (s *PostgresStorage) GetMonitorConfig(id int64) (*MonitorConfig, error) {
	ctx := s.effectiveCtx()

	var config MonitorConfig
	var deletedAt *int64
	err := s.pool.QueryRow(ctx, `
		SELECT id, provider, service, channel, model, name, enabled, parent_key,
			   config_blob, schema_version, version, created_at, updated_at, deleted_at
		FROM monitor_configs WHERE id = $1`, id).Scan(
		&config.ID, &config.Provider, &config.Service, &config.Channel, &config.Model,
		&config.Name, &config.Enabled, &config.ParentKey, &config.ConfigBlob,
		&config.SchemaVersion, &config.Version, &config.CreatedAt, &config.UpdatedAt, &deletedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("获取监测项配置失败: %w", err)
	}
	config.DeletedAt = deletedAt
	return &config, nil
}

// UpdateMonitorConfig 更新监测项配置（乐观锁）
func (s *PostgresStorage) UpdateMonitorConfig(config *MonitorConfig) error {
	if config == nil || config.ID == 0 {
		return errors.New("config 或 ID 不能为空")
	}

	ctx := s.effectiveCtx()
	now := time.Now().Unix()
	newVersion := config.Version + 1

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("更新监测项配置: 开启事务失败: %w", err)
	}
	defer tx.Rollback(ctx)

	result, err := tx.Exec(ctx, `
		UPDATE monitor_configs SET
			name = $1, enabled = $2, parent_key = $3, config_blob = $4,
			version = $5, updated_at = $6
		WHERE id = $7 AND version = $8 AND deleted_at IS NULL`,
		config.Name, config.Enabled, config.ParentKey, config.ConfigBlob,
		newVersion, now, config.ID, config.Version,
	)
	if err != nil {
		return fmt.Errorf("更新监测项配置: 执行失败: %w", err)
	}

	if result.RowsAffected() == 0 {
		return errors.New("更新失败: 记录不存在或版本冲突")
	}

	config.Version = newVersion
	config.UpdatedAt = now

	// 递增版本
	if err := s.incrementConfigVersionTx(ctx, tx, ConfigScopeMonitors); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("更新监测项配置: 提交事务失败: %w", err)
	}

	return nil
}

// DeleteMonitorConfig 软删除监测项配置
func (s *PostgresStorage) DeleteMonitorConfig(id int64) error {
	ctx := s.effectiveCtx()
	now := time.Now().Unix()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("删除监测项配置: 开启事务失败: %w", err)
	}
	defer tx.Rollback(ctx)

	result, err := tx.Exec(ctx, `
		UPDATE monitor_configs SET deleted_at = $1, updated_at = $1
		WHERE id = $2 AND deleted_at IS NULL`, now, id)
	if err != nil {
		return fmt.Errorf("删除监测项配置: 执行失败: %w", err)
	}

	if result.RowsAffected() == 0 {
		return nil // 已删除或不存在
	}

	// 递增版本
	if err := s.incrementConfigVersionTx(ctx, tx, ConfigScopeMonitors); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("删除监测项配置: 提交事务失败: %w", err)
	}

	return nil
}

// RestoreMonitorConfig 恢复软删除的监测项配置
func (s *PostgresStorage) RestoreMonitorConfig(id int64) error {
	ctx := s.effectiveCtx()
	now := time.Now().Unix()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("恢复监测项配置: 开启事务失败: %w", err)
	}
	defer tx.Rollback(ctx)

	result, err := tx.Exec(ctx, `
		UPDATE monitor_configs SET deleted_at = NULL, updated_at = $1
		WHERE id = $2 AND deleted_at IS NOT NULL`, now, id)
	if err != nil {
		return fmt.Errorf("恢复监测项配置: 执行失败: %w", err)
	}

	if result.RowsAffected() == 0 {
		return errors.New("监测项不存在或未被删除")
	}

	// 递增版本
	if err := s.incrementConfigVersionTx(ctx, tx, ConfigScopeMonitors); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("恢复监测项配置: 提交事务失败: %w", err)
	}

	return nil
}

// ListMonitorConfigs 列出监测项配置
func (s *PostgresStorage) ListMonitorConfigs(filter *MonitorConfigFilter) ([]*MonitorConfig, int, error) {
	ctx := s.effectiveCtx()

	// 构建 WHERE 子句
	var conditions []string
	var args []any
	argIdx := 1

	if filter != nil {
		if filter.Provider != "" {
			conditions = append(conditions, fmt.Sprintf("provider = $%d", argIdx))
			args = append(args, filter.Provider)
			argIdx++
		}
		if filter.Service != "" {
			conditions = append(conditions, fmt.Sprintf("service = $%d", argIdx))
			args = append(args, filter.Service)
			argIdx++
		}
		if filter.Channel != "" {
			conditions = append(conditions, fmt.Sprintf("channel = $%d", argIdx))
			args = append(args, filter.Channel)
			argIdx++
		}
		if filter.Model != "" {
			conditions = append(conditions, fmt.Sprintf("model = $%d", argIdx))
			args = append(args, filter.Model)
			argIdx++
		}
		if filter.Enabled != nil {
			conditions = append(conditions, fmt.Sprintf("enabled = $%d", argIdx))
			args = append(args, *filter.Enabled)
			argIdx++
		}
		if filter.Search != "" {
			conditions = append(conditions, fmt.Sprintf(
				"(provider ILIKE $%d OR service ILIKE $%d OR channel ILIKE $%d OR name ILIKE $%d)",
				argIdx, argIdx, argIdx, argIdx))
			args = append(args, "%"+filter.Search+"%")
			argIdx++
		}
		if !filter.IncludeDeleted {
			conditions = append(conditions, "deleted_at IS NULL")
		}
	} else {
		conditions = append(conditions, "deleted_at IS NULL")
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 查询总数
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM monitor_configs %s", whereClause)
	var total int
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("统计监测项配置失败: %w", err)
	}

	// 构建分页
	limit, offset := normalizeLimitOffset(filter)
	query := fmt.Sprintf(`
		SELECT id, provider, service, channel, model, name, enabled, parent_key,
			   config_blob, schema_version, version, created_at, updated_at, deleted_at
		FROM monitor_configs %s
		ORDER BY updated_at DESC`, whereClause)

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询监测项配置失败: %w", err)
	}
	defer rows.Close()

	var configs []*MonitorConfig
	for rows.Next() {
		var config MonitorConfig
		var deletedAt *int64
		if err := rows.Scan(
			&config.ID, &config.Provider, &config.Service, &config.Channel, &config.Model,
			&config.Name, &config.Enabled, &config.ParentKey, &config.ConfigBlob,
			&config.SchemaVersion, &config.Version, &config.CreatedAt, &config.UpdatedAt, &deletedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("扫描监测项配置失败: %w", err)
		}
		config.DeletedAt = deletedAt
		configs = append(configs, &config)
	}

	return configs, total, nil
}

// ImportMonitorConfigs 批量导入监测项配置（事务操作）
func (s *PostgresStorage) ImportMonitorConfigs(configs []*MonitorConfig) (*ImportResult, error) {
	result := &ImportResult{}
	if len(configs) == 0 {
		return result, nil
	}

	ctx := s.effectiveCtx()
	now := time.Now().Unix()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return result, fmt.Errorf("导入监测项配置: 开启事务失败: %w", err)
	}
	defer tx.Rollback(ctx)

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

		err := tx.QueryRow(ctx, `
			INSERT INTO monitor_configs (
				provider, service, channel, model, name, enabled, parent_key,
				config_blob, schema_version, version, created_at, updated_at, deleted_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			RETURNING id`,
			cfg.Provider, cfg.Service, cfg.Channel, cfg.Model,
			cfg.Name, cfg.Enabled, cfg.ParentKey, cfg.ConfigBlob,
			cfg.SchemaVersion, cfg.Version, cfg.CreatedAt, cfg.UpdatedAt, cfg.DeletedAt,
		).Scan(&cfg.ID)
		if err != nil {
			if strings.Contains(err.Error(), "uq_monitor_configs_key") {
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
		result.Created++
	}

	if result.Created > 0 {
		if err := s.incrementConfigVersionTx(ctx, tx, ConfigScopeMonitors); err != nil {
			result.Errors = append(result.Errors, err.Error())
			return result, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		result.Created = 0
		return result, fmt.Errorf("导入监测项配置: 提交事务失败: %w", err)
	}
	return result, nil
}

// ===== MonitorSecret CRUD (PostgreSQL) =====

// SetMonitorSecret 设置或更新监测项密钥
// 实现 AdminStorage 接口
func (s *PostgresStorage) SetMonitorSecret(monitorID int64, ciphertext, nonce []byte, keyVersion, encVersion int) error {
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

	_, err := s.pool.Exec(ctx, `
		INSERT INTO monitor_secrets (monitor_id, api_key_ciphertext, api_key_nonce, key_version, enc_version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (monitor_id) DO UPDATE SET
			api_key_ciphertext = EXCLUDED.api_key_ciphertext,
			api_key_nonce = EXCLUDED.api_key_nonce,
			key_version = EXCLUDED.key_version,
			enc_version = EXCLUDED.enc_version,
			updated_at = EXCLUDED.updated_at`,
		monitorID, ciphertext, nonce, keyVersion, encVersion, now, now,
	)
	if err != nil {
		return fmt.Errorf("设置监测项密钥失败: %w", err)
	}
	return nil
}

// SaveMonitorSecret 保存或更新监测项密钥（使用结构体参数）
func (s *PostgresStorage) SaveMonitorSecret(secret *MonitorSecret) error {
	if secret == nil || secret.MonitorID == 0 {
		return errors.New("secret 或 MonitorID 不能为空")
	}

	ctx := s.effectiveCtx()
	now := time.Now().Unix()

	_, err := s.pool.Exec(ctx, `
		INSERT INTO monitor_secrets (monitor_id, api_key_ciphertext, api_key_nonce, key_version, enc_version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (monitor_id) DO UPDATE SET
			api_key_ciphertext = EXCLUDED.api_key_ciphertext,
			api_key_nonce = EXCLUDED.api_key_nonce,
			key_version = EXCLUDED.key_version,
			enc_version = EXCLUDED.enc_version,
			updated_at = EXCLUDED.updated_at`,
		secret.MonitorID, secret.APIKeyCiphertext, secret.APIKeyNonce,
		secret.KeyVersion, secret.EncVersion, now, now,
	)
	if err != nil {
		return fmt.Errorf("保存监测项密钥失败: %w", err)
	}

	return nil
}

// GetMonitorSecret 获取监测项密钥
func (s *PostgresStorage) GetMonitorSecret(monitorID int64) (*MonitorSecret, error) {
	ctx := s.effectiveCtx()

	var secret MonitorSecret
	err := s.pool.QueryRow(ctx, `
		SELECT monitor_id, api_key_ciphertext, api_key_nonce, key_version, enc_version, created_at, updated_at
		FROM monitor_secrets WHERE monitor_id = $1`, monitorID).Scan(
		&secret.MonitorID, &secret.APIKeyCiphertext, &secret.APIKeyNonce,
		&secret.KeyVersion, &secret.EncVersion, &secret.CreatedAt, &secret.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("获取监测项密钥失败: %w", err)
	}
	return &secret, nil
}

// DeleteMonitorSecret 删除监测项密钥
func (s *PostgresStorage) DeleteMonitorSecret(monitorID int64) error {
	ctx := s.effectiveCtx()
	_, err := s.pool.Exec(ctx, `DELETE FROM monitor_secrets WHERE monitor_id = $1`, monitorID)
	if err != nil {
		return fmt.Errorf("删除监测项密钥失败: %w", err)
	}
	return nil
}

// ===== MonitorConfigAudit CRUD (PostgreSQL) =====

// CreateMonitorConfigAudit 创建审计记录
func (s *PostgresStorage) CreateMonitorConfigAudit(audit *MonitorConfigAudit) error {
	if audit == nil {
		return errors.New("audit 不能为空")
	}

	ctx := s.effectiveCtx()
	now := time.Now().Unix()
	audit.CreatedAt = now

	err := s.pool.QueryRow(ctx, `
		INSERT INTO monitor_config_audits (
			monitor_id, provider, service, channel, model, action,
			before_blob, after_blob, before_version, after_version, secret_changed,
			actor, actor_ip, user_agent, request_id, batch_id, reason, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		RETURNING id`,
		audit.MonitorID, audit.Provider, audit.Service, audit.Channel, audit.Model,
		audit.Action, audit.BeforeBlob, audit.AfterBlob, audit.BeforeVersion, audit.AfterVersion,
		audit.SecretChanged, audit.Actor, audit.ActorIP, audit.UserAgent,
		audit.RequestID, audit.BatchID, audit.Reason, audit.CreatedAt,
	).Scan(&audit.ID)
	if err != nil {
		return fmt.Errorf("创建审计记录失败: %w", err)
	}

	return nil
}

// ListMonitorConfigAudits 列出审计记录
func (s *PostgresStorage) ListMonitorConfigAudits(filter *AuditFilter) ([]*MonitorConfigAudit, int, error) {
	ctx := s.effectiveCtx()

	var conditions []string
	var args []any
	argIdx := 1

	if filter != nil {
		if filter.MonitorID > 0 {
			conditions = append(conditions, fmt.Sprintf("monitor_id = $%d", argIdx))
			args = append(args, filter.MonitorID)
			argIdx++
		}
		if filter.Provider != "" {
			conditions = append(conditions, fmt.Sprintf("provider = $%d", argIdx))
			args = append(args, filter.Provider)
			argIdx++
		}
		if filter.Service != "" {
			conditions = append(conditions, fmt.Sprintf("service = $%d", argIdx))
			args = append(args, filter.Service)
			argIdx++
		}
		if filter.Action != "" {
			conditions = append(conditions, fmt.Sprintf("action = $%d", argIdx))
			args = append(args, filter.Action)
			argIdx++
		}
		if filter.Since > 0 {
			conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIdx))
			args = append(args, filter.Since)
			argIdx++
		}
		if filter.Until > 0 {
			conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIdx))
			args = append(args, filter.Until)
			argIdx++
		}
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 统计总数
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM monitor_config_audits %s", whereClause)
	var total int
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("统计审计记录失败: %w", err)
	}

	// 分页
	limit := 50
	offset := 0
	if filter != nil {
		if filter.Limit > 0 && filter.Limit <= 200 {
			limit = filter.Limit
		}
		if filter.Offset > 0 {
			offset = filter.Offset
		}
	}

	query := fmt.Sprintf(`
		SELECT id, monitor_id, provider, service, channel, model, action,
			   before_blob, after_blob, before_version, after_version, secret_changed,
			   actor, actor_ip, user_agent, request_id, batch_id, reason, created_at
		FROM monitor_config_audits %s
		ORDER BY created_at DESC
		LIMIT %d OFFSET %d`, whereClause, limit, offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询审计记录失败: %w", err)
	}
	defer rows.Close()

	var audits []*MonitorConfigAudit
	for rows.Next() {
		var a MonitorConfigAudit
		if err := rows.Scan(
			&a.ID, &a.MonitorID, &a.Provider, &a.Service, &a.Channel, &a.Model, &a.Action,
			&a.BeforeBlob, &a.AfterBlob, &a.BeforeVersion, &a.AfterVersion, &a.SecretChanged,
			&a.Actor, &a.ActorIP, &a.UserAgent, &a.RequestID, &a.BatchID, &a.Reason, &a.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("扫描审计记录失败: %w", err)
		}
		audits = append(audits, &a)
	}

	return audits, total, nil
}

// ===== ProviderPolicy CRUD (PostgreSQL) =====

// ListProviderPolicies 列出所有 Provider 策略
func (s *PostgresStorage) ListProviderPolicies() ([]*ProviderPolicy, error) {
	ctx := s.effectiveCtx()

	rows, err := s.pool.Query(ctx, `
		SELECT id, policy_type, provider, reason, risks, created_at, updated_at
		FROM provider_policies ORDER BY provider, policy_type`)
	if err != nil {
		return nil, fmt.Errorf("查询 Provider 策略失败: %w", err)
	}
	defer rows.Close()

	var policies []*ProviderPolicy
	for rows.Next() {
		var p ProviderPolicy
		if err := rows.Scan(
			&p.ID, &p.PolicyType, &p.Provider, &p.Reason, &p.Risks, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描 Provider 策略失败: %w", err)
		}
		policies = append(policies, &p)
	}

	return policies, nil
}

// CreateProviderPolicy 创建 Provider 策略
func (s *PostgresStorage) CreateProviderPolicy(policy *ProviderPolicy) error {
	if policy == nil {
		return errors.New("policy 不能为空")
	}

	ctx := s.effectiveCtx()
	now := time.Now().Unix()
	policy.CreatedAt = now
	policy.UpdatedAt = now

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("创建 Provider 策略: 开启事务失败: %w", err)
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
		INSERT INTO provider_policies (policy_type, provider, reason, risks, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`,
		policy.PolicyType, policy.Provider, policy.Reason, policy.Risks,
		policy.CreatedAt, policy.UpdatedAt,
	).Scan(&policy.ID)
	if err != nil {
		if strings.Contains(err.Error(), "uq_provider_policies") {
			return fmt.Errorf("策略已存在: %s/%s", policy.PolicyType, policy.Provider)
		}
		return fmt.Errorf("创建 Provider 策略失败: %w", err)
	}

	if err := s.incrementConfigVersionTx(ctx, tx, ConfigScopePolicies); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("创建 Provider 策略: 提交事务失败: %w", err)
	}

	return nil
}

// UpdateProviderPolicy 更新 Provider 策略
func (s *PostgresStorage) UpdateProviderPolicy(policy *ProviderPolicy) error {
	if policy == nil || policy.ID == 0 {
		return errors.New("policy 或 ID 不能为空")
	}

	ctx := s.effectiveCtx()
	now := time.Now().Unix()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("更新 Provider 策略: 开启事务失败: %w", err)
	}
	defer tx.Rollback(ctx)

	result, err := tx.Exec(ctx, `
		UPDATE provider_policies SET reason = $1, risks = $2, updated_at = $3
		WHERE id = $4`,
		policy.Reason, policy.Risks, now, policy.ID,
	)
	if err != nil {
		return fmt.Errorf("更新 Provider 策略失败: %w", err)
	}

	if result.RowsAffected() == 0 {
		return errors.New("策略不存在")
	}

	policy.UpdatedAt = now

	if err := s.incrementConfigVersionTx(ctx, tx, ConfigScopePolicies); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("更新 Provider 策略: 提交事务失败: %w", err)
	}

	return nil
}

// DeleteProviderPolicy 删除 Provider 策略
func (s *PostgresStorage) DeleteProviderPolicy(id int64) error {
	ctx := s.effectiveCtx()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("删除 Provider 策略: 开启事务失败: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `DELETE FROM provider_policies WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("删除 Provider 策略失败: %w", err)
	}

	if err := s.incrementConfigVersionTx(ctx, tx, ConfigScopePolicies); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("删除 Provider 策略: 提交事务失败: %w", err)
	}

	return nil
}

// ===== BadgeDefinition CRUD (PostgreSQL) =====

// ListBadgeDefinitions 列出所有徽标定义
func (s *PostgresStorage) ListBadgeDefinitions() ([]*BadgeDefinition, error) {
	ctx := s.effectiveCtx()

	rows, err := s.pool.Query(ctx, `
		SELECT id, kind, weight, label_i18n, tooltip_i18n, icon, color, category, svg_source, created_at, updated_at
		FROM badge_definitions ORDER BY weight DESC, id`)
	if err != nil {
		return nil, fmt.Errorf("查询徽标定义失败: %w", err)
	}
	defer rows.Close()

	var defs []*BadgeDefinition
	for rows.Next() {
		var d BadgeDefinition
		var category, svgSource *string
		if err := rows.Scan(
			&d.ID, &d.Kind, &d.Weight, &d.LabelI18n, &d.TooltipI18n, &d.Icon, &d.Color,
			&category, &svgSource, &d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描徽标定义失败: %w", err)
		}
		if category != nil {
			d.Category = *category
		}
		if svgSource != nil {
			d.SVGSource = *svgSource
		}
		defs = append(defs, &d)
	}

	return defs, nil
}

// CreateBadgeDefinition 创建徽标定义
func (s *PostgresStorage) CreateBadgeDefinition(def *BadgeDefinition) error {
	if def == nil || def.ID == "" {
		return errors.New("徽标 ID 不能为空")
	}

	ctx := s.effectiveCtx()
	now := time.Now().Unix()
	def.CreatedAt = now
	def.UpdatedAt = now

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("创建徽标定义: 开启事务失败: %w", err)
	}
	defer tx.Rollback(ctx)

	// 处理可选的 category 和 svg_source 字段
	var category, svgSource *string
	if def.Category != "" {
		category = &def.Category
	}
	if def.SVGSource != "" {
		svgSource = &def.SVGSource
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO badge_definitions (id, kind, weight, label_i18n, tooltip_i18n, icon, color, category, svg_source, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		def.ID, def.Kind, def.Weight, def.LabelI18n, def.TooltipI18n, def.Icon, def.Color,
		category, svgSource, def.CreatedAt, def.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "badge_definitions_pkey") {
			return fmt.Errorf("徽标 ID 已存在: %s", def.ID)
		}
		return fmt.Errorf("创建徽标定义失败: %w", err)
	}

	if err := s.incrementConfigVersionTx(ctx, tx, ConfigScopeBadges); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("创建徽标定义: 提交事务失败: %w", err)
	}

	return nil
}

// UpdateBadgeDefinition 更新徽标定义
func (s *PostgresStorage) UpdateBadgeDefinition(def *BadgeDefinition) error {
	if def == nil || def.ID == "" {
		return errors.New("徽标 ID 不能为空")
	}

	ctx := s.effectiveCtx()
	now := time.Now().Unix()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("更新徽标定义: 开启事务失败: %w", err)
	}
	defer tx.Rollback(ctx)

	// 处理可选的 category 和 svg_source 字段
	var category, svgSource *string
	if def.Category != "" {
		category = &def.Category
	}
	if def.SVGSource != "" {
		svgSource = &def.SVGSource
	}

	result, err := tx.Exec(ctx, `
		UPDATE badge_definitions SET
			kind = $1, weight = $2, label_i18n = $3, tooltip_i18n = $4,
			icon = $5, color = $6, category = $7, svg_source = $8, updated_at = $9
		WHERE id = $10`,
		def.Kind, def.Weight, def.LabelI18n, def.TooltipI18n,
		def.Icon, def.Color, category, svgSource, now, def.ID,
	)
	if err != nil {
		return fmt.Errorf("更新徽标定义失败: %w", err)
	}

	if result.RowsAffected() == 0 {
		return errors.New("徽标不存在")
	}

	def.UpdatedAt = now

	if err := s.incrementConfigVersionTx(ctx, tx, ConfigScopeBadges); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("更新徽标定义: 提交事务失败: %w", err)
	}

	return nil
}

// DeleteBadgeDefinition 删除徽标定义
func (s *PostgresStorage) DeleteBadgeDefinition(id string) error {
	ctx := s.effectiveCtx()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("删除徽标定义: 开启事务失败: %w", err)
	}
	defer tx.Rollback(ctx)

	// 先删除相关绑定
	_, err = tx.Exec(ctx, `DELETE FROM badge_bindings WHERE badge_id = $1`, id)
	if err != nil {
		return fmt.Errorf("删除徽标绑定失败: %w", err)
	}

	_, err = tx.Exec(ctx, `DELETE FROM badge_definitions WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("删除徽标定义失败: %w", err)
	}

	if err := s.incrementConfigVersionTx(ctx, tx, ConfigScopeBadges); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("删除徽标定义: 提交事务失败: %w", err)
	}

	return nil
}

// ===== BadgeBinding CRUD (PostgreSQL) =====

// ListBadgeBindings 列出徽标绑定
func (s *PostgresStorage) ListBadgeBindings(filter *BadgeBindingFilter) ([]*BadgeBinding, error) {
	ctx := s.effectiveCtx()

	var conditions []string
	var args []any
	argIdx := 1

	if filter != nil {
		if filter.BadgeID != "" {
			conditions = append(conditions, fmt.Sprintf("badge_id = $%d", argIdx))
			args = append(args, filter.BadgeID)
			argIdx++
		}
		if filter.Scope != "" {
			conditions = append(conditions, fmt.Sprintf("scope = $%d", argIdx))
			args = append(args, filter.Scope)
			argIdx++
		}
		if filter.Provider != "" {
			conditions = append(conditions, fmt.Sprintf("provider = $%d", argIdx))
			args = append(args, filter.Provider)
			argIdx++
		}
		if filter.Service != "" {
			conditions = append(conditions, fmt.Sprintf("service = $%d", argIdx))
			args = append(args, filter.Service)
			argIdx++
		}
		if filter.Channel != "" {
			conditions = append(conditions, fmt.Sprintf("channel = $%d", argIdx))
			args = append(args, filter.Channel)
			argIdx++
		}
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, badge_id, scope, provider, service, channel, tooltip_override, created_at, updated_at
		FROM badge_bindings %s
		ORDER BY scope, provider, service, channel`, whereClause)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询徽标绑定失败: %w", err)
	}
	defer rows.Close()

	var bindings []*BadgeBinding
	for rows.Next() {
		var b BadgeBinding
		if err := rows.Scan(
			&b.ID, &b.BadgeID, &b.Scope, &b.Provider, &b.Service, &b.Channel,
			&b.TooltipOverride, &b.CreatedAt, &b.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描徽标绑定失败: %w", err)
		}
		bindings = append(bindings, &b)
	}

	return bindings, nil
}

// CreateBadgeBinding 创建徽标绑定
func (s *PostgresStorage) CreateBadgeBinding(binding *BadgeBinding) error {
	if binding == nil || binding.BadgeID == "" {
		return errors.New("badge_id 不能为空")
	}

	ctx := s.effectiveCtx()
	now := time.Now().Unix()
	binding.CreatedAt = now
	binding.UpdatedAt = now

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("创建徽标绑定: 开启事务失败: %w", err)
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
		INSERT INTO badge_bindings (badge_id, scope, provider, service, channel, tooltip_override, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id`,
		binding.BadgeID, binding.Scope, binding.Provider, binding.Service,
		binding.Channel, binding.TooltipOverride, binding.CreatedAt, binding.UpdatedAt,
	).Scan(&binding.ID)
	if err != nil {
		if strings.Contains(err.Error(), "uq_badge_bindings") {
			return errors.New("徽标绑定已存在")
		}
		return fmt.Errorf("创建徽标绑定失败: %w", err)
	}

	if err := s.incrementConfigVersionTx(ctx, tx, ConfigScopeBadges); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("创建徽标绑定: 提交事务失败: %w", err)
	}

	return nil
}

// DeleteBadgeBinding 删除徽标绑定
func (s *PostgresStorage) DeleteBadgeBinding(id int64) error {
	ctx := s.effectiveCtx()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("删除徽标绑定: 开启事务失败: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `DELETE FROM badge_bindings WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("删除徽标绑定失败: %w", err)
	}

	if err := s.incrementConfigVersionTx(ctx, tx, ConfigScopeBadges); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("删除徽标绑定: 提交事务失败: %w", err)
	}

	return nil
}

// ===== BoardConfig CRUD (PostgreSQL) =====

// ListBoardConfigs 列出所有 Board 配置
func (s *PostgresStorage) ListBoardConfigs() ([]*BoardConfig, error) {
	ctx := s.effectiveCtx()

	rows, err := s.pool.Query(ctx, `
		SELECT board, display_name, description, sort_order, created_at, updated_at
		FROM board_configs ORDER BY sort_order, board`)
	if err != nil {
		return nil, fmt.Errorf("查询 Board 配置失败: %w", err)
	}
	defer rows.Close()

	var boards []*BoardConfig
	for rows.Next() {
		var b BoardConfig
		if err := rows.Scan(
			&b.Board, &b.DisplayName, &b.Description, &b.SortOrder, &b.CreatedAt, &b.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描 Board 配置失败: %w", err)
		}
		boards = append(boards, &b)
	}

	return boards, nil
}

// ===== GlobalSetting CRUD (PostgreSQL) =====

// GetGlobalSetting 获取全局设置
func (s *PostgresStorage) GetGlobalSetting(key string) (*GlobalSetting, error) {
	ctx := s.effectiveCtx()

	var setting GlobalSetting
	err := s.pool.QueryRow(ctx, `
		SELECT key, value, schema_version, version, created_at, updated_at
		FROM global_settings WHERE key = $1`, key).Scan(
		&setting.Key, &setting.Value, &setting.SchemaVersion, &setting.Version,
		&setting.CreatedAt, &setting.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("获取全局设置失败: %w", err)
	}
	return &setting, nil
}

// SetGlobalSetting 设置全局设置（upsert）
// 实现 AdminStorage 接口
func (s *PostgresStorage) SetGlobalSetting(key, value string) error {
	if strings.TrimSpace(key) == "" {
		return errors.New("key 不能为空")
	}

	ctx := s.effectiveCtx()
	now := time.Now().Unix()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("设置全局设置: 开启事务失败: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO global_settings (key, value, schema_version, version, created_at, updated_at)
		VALUES ($1, $2, 1, 1, $3, $4)
		ON CONFLICT (key) DO UPDATE SET
			value = EXCLUDED.value,
			version = global_settings.version + 1,
			updated_at = EXCLUDED.updated_at`,
		key, value, now, now,
	)
	if err != nil {
		return fmt.Errorf("设置全局设置失败: %w", err)
	}

	if err := s.incrementConfigVersionTx(ctx, tx, ConfigScopeSettings); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("设置全局设置: 提交事务失败: %w", err)
	}
	return nil
}

// SaveGlobalSetting 保存全局设置（使用结构体参数）
func (s *PostgresStorage) SaveGlobalSetting(setting *GlobalSetting) error {
	if setting == nil || setting.Key == "" {
		return errors.New("setting 或 key 不能为空")
	}

	ctx := s.effectiveCtx()
	now := time.Now().Unix()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("保存全局设置: 开启事务失败: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO global_settings (key, value, schema_version, version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (key) DO UPDATE SET
			value = EXCLUDED.value,
			schema_version = EXCLUDED.schema_version,
			version = global_settings.version + 1,
			updated_at = EXCLUDED.updated_at`,
		setting.Key, setting.Value, setting.SchemaVersion, 1, now, now,
	)
	if err != nil {
		return fmt.Errorf("保存全局设置失败: %w", err)
	}

	if err := s.incrementConfigVersionTx(ctx, tx, ConfigScopeSettings); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("保存全局设置: 提交事务失败: %w", err)
	}

	return nil
}

// ===== ConfigMeta (PostgreSQL) =====

// GetConfigVersions 获取所有配置版本
func (s *PostgresStorage) GetConfigVersions() (*ConfigVersions, error) {
	ctx := s.effectiveCtx()

	rows, err := s.pool.Query(ctx, `SELECT scope, version FROM config_meta`)
	if err != nil {
		return nil, fmt.Errorf("查询配置版本失败: %w", err)
	}
	defer rows.Close()

	versions := &ConfigVersions{}
	for rows.Next() {
		var scope string
		var version int64
		if err := rows.Scan(&scope, &version); err != nil {
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

	return versions, nil
}

// incrementConfigVersionTx 在事务中递增配置版本
func (s *PostgresStorage) incrementConfigVersionTx(ctx context.Context, tx pgx.Tx, scope ConfigScope) error {
	now := time.Now().Unix()
	_, err := tx.Exec(ctx, `
		UPDATE config_meta SET version = version + 1, updated_at = $1
		WHERE scope = $2`, now, string(scope))
	if err != nil {
		return fmt.Errorf("递增配置版本失败: %w", err)
	}
	return nil
}
