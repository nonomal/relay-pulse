package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// =====================================================
// MonitorStorageV1 实现
// =====================================================

// CreateMonitor 创建监测项
func (s *PostgresStorage) CreateMonitor(ctx context.Context, monitor *Monitor) error {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		INSERT INTO monitors (
			provider, provider_name, service, service_name, channel, channel_name, model,
			template_id, url, method, headers, body, success_contains,
			interval, timeout, slow_latency, enabled, board_id,
			owner_user_id, vendor_type, website_url, application_id,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24)
		RETURNING id
	`
	err := s.pool.QueryRow(ctx, query,
		monitor.Provider, monitor.ProviderName, monitor.Service, monitor.ServiceName,
		monitor.Channel, monitor.ChannelName, monitor.Model,
		monitor.TemplateID, monitor.URL, monitor.Method, monitor.Headers, monitor.Body, monitor.SuccessContains,
		monitor.Interval, monitor.Timeout, monitor.SlowLatency, monitor.Enabled, monitor.BoardID,
		monitor.OwnerUserID, monitor.VendorType, monitor.WebsiteURL, monitor.ApplicationID,
		monitor.CreatedAt, monitor.UpdatedAt,
	).Scan(&monitor.ID)
	if err != nil {
		return fmt.Errorf("创建监测项失败: %w", err)
	}
	return nil
}

// GetMonitorByID 按 ID 获取监测项
func (s *PostgresStorage) GetMonitorByID(ctx context.Context, id int) (*Monitor, error) {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		SELECT id, provider, provider_name, service, service_name, channel, channel_name, model,
			template_id, url, method, headers, body, success_contains,
			interval, timeout, slow_latency, enabled, board_id,
			owner_user_id, vendor_type, website_url, application_id,
			created_at, updated_at
		FROM monitors WHERE id = $1
	`
	monitor := &Monitor{}
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&monitor.ID, &monitor.Provider, &monitor.ProviderName, &monitor.Service, &monitor.ServiceName,
		&monitor.Channel, &monitor.ChannelName, &monitor.Model,
		&monitor.TemplateID, &monitor.URL, &monitor.Method, &monitor.Headers, &monitor.Body, &monitor.SuccessContains,
		&monitor.Interval, &monitor.Timeout, &monitor.SlowLatency, &monitor.Enabled, &monitor.BoardID,
		&monitor.OwnerUserID, &monitor.VendorType, &monitor.WebsiteURL, &monitor.ApplicationID,
		&monitor.CreatedAt, &monitor.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("获取监测项失败: %w", err)
	}
	return monitor, nil
}

// GetMonitorByKey 按唯一键获取监测项
func (s *PostgresStorage) GetMonitorByKey(ctx context.Context, provider, service, channel string) (*Monitor, error) {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		SELECT id, provider, provider_name, service, service_name, channel, channel_name, model,
			template_id, url, method, headers, body, success_contains,
			interval, timeout, slow_latency, enabled, board_id,
			owner_user_id, vendor_type, website_url, application_id,
			created_at, updated_at
		FROM monitors WHERE provider = $1 AND service = $2 AND channel = $3
	`
	monitor := &Monitor{}
	err := s.pool.QueryRow(ctx, query, provider, service, channel).Scan(
		&monitor.ID, &monitor.Provider, &monitor.ProviderName, &monitor.Service, &monitor.ServiceName,
		&monitor.Channel, &monitor.ChannelName, &monitor.Model,
		&monitor.TemplateID, &monitor.URL, &monitor.Method, &monitor.Headers, &monitor.Body, &monitor.SuccessContains,
		&monitor.Interval, &monitor.Timeout, &monitor.SlowLatency, &monitor.Enabled, &monitor.BoardID,
		&monitor.OwnerUserID, &monitor.VendorType, &monitor.WebsiteURL, &monitor.ApplicationID,
		&monitor.CreatedAt, &monitor.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("获取监测项失败: %w", err)
	}
	return monitor, nil
}

// UpdateMonitor 更新监测项
func (s *PostgresStorage) UpdateMonitor(ctx context.Context, monitor *Monitor) error {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		UPDATE monitors SET
			provider_name = $2, service_name = $3, channel_name = $4, model = $5,
			template_id = $6, url = $7, method = $8, headers = $9, body = $10, success_contains = $11,
			interval = $12, timeout = $13, slow_latency = $14, enabled = $15, board_id = $16,
			vendor_type = $17, website_url = $18, updated_at = $19
		WHERE id = $1
	`
	_, err := s.pool.Exec(ctx, query,
		monitor.ID, monitor.ProviderName, monitor.ServiceName, monitor.ChannelName, monitor.Model,
		monitor.TemplateID, monitor.URL, monitor.Method, monitor.Headers, monitor.Body, monitor.SuccessContains,
		monitor.Interval, monitor.Timeout, monitor.SlowLatency, monitor.Enabled, monitor.BoardID,
		monitor.VendorType, monitor.WebsiteURL, monitor.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("更新监测项失败: %w", err)
	}
	return nil
}

// DeleteMonitor 删除监测项
func (s *PostgresStorage) DeleteMonitor(ctx context.Context, id int) error {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `DELETE FROM monitors WHERE id = $1`
	_, err := s.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("删除监测项失败: %w", err)
	}
	return nil
}

// ListMonitors 列出监测项
func (s *PostgresStorage) ListMonitors(ctx context.Context, opts *ListMonitorsOptions) ([]*Monitor, int, error) {
	if ctx == nil {
		ctx = s.ctx
	}
	if opts == nil {
		opts = &ListMonitorsOptions{}
	}

	var conditions []string
	var args []interface{}
	argIdx := 1

	if opts.Provider != "" {
		conditions = append(conditions, fmt.Sprintf("provider = $%d", argIdx))
		args = append(args, opts.Provider)
		argIdx++
	}
	if opts.Service != "" {
		conditions = append(conditions, fmt.Sprintf("service = $%d", argIdx))
		args = append(args, opts.Service)
		argIdx++
	}
	if opts.Channel != "" {
		conditions = append(conditions, fmt.Sprintf("channel = $%d", argIdx))
		args = append(args, opts.Channel)
		argIdx++
	}
	if opts.BoardID != nil {
		conditions = append(conditions, fmt.Sprintf("board_id = $%d", argIdx))
		args = append(args, *opts.BoardID)
		argIdx++
	}
	if opts.OwnerUserID != nil {
		conditions = append(conditions, fmt.Sprintf("owner_user_id = $%d", argIdx))
		args = append(args, *opts.OwnerUserID)
		argIdx++
	}
	if opts.Enabled != nil {
		conditions = append(conditions, fmt.Sprintf("enabled = $%d", argIdx))
		args = append(args, *opts.Enabled)
		argIdx++
	}
	if opts.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(provider_name ILIKE $%d OR provider ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+opts.Search+"%")
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 查询总数
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM monitors %s", whereClause)
	var total int
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("查询监测项总数失败: %w", err)
	}

	// 查询列表
	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf(`
		SELECT id, provider, provider_name, service, service_name, channel, channel_name, model,
			template_id, url, method, headers, body, success_contains,
			interval, timeout, slow_latency, enabled, board_id,
			owner_user_id, vendor_type, website_url, application_id,
			created_at, updated_at
		FROM monitors %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询监测项列表失败: %w", err)
	}
	defer rows.Close()

	var monitors []*Monitor
	for rows.Next() {
		monitor := &Monitor{}
		if err := rows.Scan(
			&monitor.ID, &monitor.Provider, &monitor.ProviderName, &monitor.Service, &monitor.ServiceName,
			&monitor.Channel, &monitor.ChannelName, &monitor.Model,
			&monitor.TemplateID, &monitor.URL, &monitor.Method, &monitor.Headers, &monitor.Body, &monitor.SuccessContains,
			&monitor.Interval, &monitor.Timeout, &monitor.SlowLatency, &monitor.Enabled, &monitor.BoardID,
			&monitor.OwnerUserID, &monitor.VendorType, &monitor.WebsiteURL, &monitor.ApplicationID,
			&monitor.CreatedAt, &monitor.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("扫描监测项数据失败: %w", err)
		}
		monitors = append(monitors, monitor)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("遍历监测项数据失败: %w", err)
	}

	return monitors, total, nil
}

// monitorUpdatableFields 监测项可更新字段白名单
var monitorUpdatableFields = map[string]bool{
	"provider_name":    true,
	"service_name":     true,
	"channel_name":     true,
	"model":            true,
	"template_id":      true,
	"url":              true,
	"method":           true,
	"headers":          true,
	"body":             true,
	"success_contains": true,
	"interval":         true,
	"timeout":          true,
	"slow_latency":     true,
	"enabled":          true,
	"board_id":         true,
	"vendor_type":      true,
	"website_url":      true,
}

// BatchUpdateMonitors 批量更新监测项
func (s *PostgresStorage) BatchUpdateMonitors(ctx context.Context, ids []int, updates map[string]interface{}) (int64, error) {
	if ctx == nil {
		ctx = s.ctx
	}
	if len(ids) == 0 || len(updates) == 0 {
		return 0, nil
	}

	// 构建 SET 子句（使用白名单过滤）
	var setClauses []string
	var args []interface{}
	argIdx := 1

	for field, value := range updates {
		// 安全检查：只允许白名单中的字段
		if !monitorUpdatableFields[field] {
			return 0, fmt.Errorf("不允许更新字段: %s", field)
		}
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", field, argIdx))
		args = append(args, value)
		argIdx++
	}

	if len(setClauses) == 0 {
		return 0, nil
	}

	// 构建 IN 子句
	inPlaceholders := make([]string, len(ids))
	for i, id := range ids {
		inPlaceholders[i] = fmt.Sprintf("$%d", argIdx)
		args = append(args, id)
		argIdx++
	}

	query := fmt.Sprintf(`
		UPDATE monitors SET %s, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
		WHERE id IN (%s)
	`, strings.Join(setClauses, ", "), strings.Join(inPlaceholders, ", "))

	result, err := s.pool.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("批量更新监测项失败: %w", err)
	}
	return result.RowsAffected(), nil
}

// BatchDeleteMonitors 批量删除监测项
func (s *PostgresStorage) BatchDeleteMonitors(ctx context.Context, ids []int) (int64, error) {
	if ctx == nil {
		ctx = s.ctx
	}
	if len(ids) == 0 {
		return 0, nil
	}

	// 构建 IN 子句
	inPlaceholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		inPlaceholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(`DELETE FROM monitors WHERE id IN (%s)`, strings.Join(inPlaceholders, ", "))
	result, err := s.pool.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("批量删除监测项失败: %w", err)
	}
	return result.RowsAffected(), nil
}

// =====================================================
// MonitorIDMapping 实现
// =====================================================

// CreateIDMapping 创建 ID 映射
func (s *PostgresStorage) CreateIDMapping(ctx context.Context, mapping *MonitorIDMapping) error {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		INSERT INTO monitor_id_mapping (old_id, new_id, legacy_provider, legacy_service, legacy_channel, migrated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := s.pool.Exec(ctx, query,
		mapping.OldID, mapping.NewID, mapping.LegacyProvider, mapping.LegacyService, mapping.LegacyChannel, mapping.MigratedAt,
	)
	if err != nil {
		return fmt.Errorf("创建 ID 映射失败: %w", err)
	}
	return nil
}

// GetNewIDByOldID 按旧 ID 获取新 ID
func (s *PostgresStorage) GetNewIDByOldID(ctx context.Context, oldID int) (int, error) {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `SELECT new_id FROM monitor_id_mapping WHERE old_id = $1`
	var newID int
	err := s.pool.QueryRow(ctx, query, oldID).Scan(&newID)
	if err == pgx.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("获取新 ID 失败: %w", err)
	}
	return newID, nil
}

// GetOldIDByNewID 按新 ID 获取旧 ID
func (s *PostgresStorage) GetOldIDByNewID(ctx context.Context, newID int) (int, error) {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `SELECT old_id FROM monitor_id_mapping WHERE new_id = $1`
	var oldID int
	err := s.pool.QueryRow(ctx, query, newID).Scan(&oldID)
	if err == pgx.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("获取旧 ID 失败: %w", err)
	}
	return oldID, nil
}

// GetNewIDByLegacyKey 按旧唯一键获取新 ID
func (s *PostgresStorage) GetNewIDByLegacyKey(ctx context.Context, provider, service, channel string) (int, error) {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `SELECT new_id FROM monitor_id_mapping WHERE legacy_provider = $1 AND legacy_service = $2 AND legacy_channel = $3`
	var newID int
	err := s.pool.QueryRow(ctx, query, provider, service, channel).Scan(&newID)
	if err == pgx.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("获取新 ID 失败: %w", err)
	}
	return newID, nil
}
