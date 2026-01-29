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
			interval_override, timeout_override, slow_latency_override, enabled, board_id,
			metadata, owner_user_id, vendor_type, website_url, application_id,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25)
		RETURNING id
	`
	err := s.pool.QueryRow(ctx, query,
		monitor.Provider, monitor.ProviderName, monitor.Service, monitor.ServiceName,
		monitor.Channel, monitor.ChannelName, monitor.Model,
		monitor.TemplateID, monitor.URL, monitor.Method, monitor.Headers, monitor.Body, monitor.SuccessContains,
		monitor.IntervalOverride, monitor.TimeoutOverride, monitor.SlowLatencyOverride, monitor.Enabled, monitor.BoardID,
		monitor.Metadata, monitor.OwnerUserID, monitor.VendorType, monitor.WebsiteURL, monitor.ApplicationID,
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
			interval_override, timeout_override, slow_latency_override, enabled, board_id,
			metadata, owner_user_id, vendor_type, website_url, application_id,
			created_at, updated_at
		FROM monitors WHERE id = $1
	`
	monitor := &Monitor{}
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&monitor.ID, &monitor.Provider, &monitor.ProviderName, &monitor.Service, &monitor.ServiceName,
		&monitor.Channel, &monitor.ChannelName, &monitor.Model,
		&monitor.TemplateID, &monitor.URL, &monitor.Method, &monitor.Headers, &monitor.Body, &monitor.SuccessContains,
		&monitor.IntervalOverride, &monitor.TimeoutOverride, &monitor.SlowLatencyOverride, &monitor.Enabled, &monitor.BoardID,
		&monitor.Metadata, &monitor.OwnerUserID, &monitor.VendorType, &monitor.WebsiteURL, &monitor.ApplicationID,
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
			interval_override, timeout_override, slow_latency_override, enabled, board_id,
			metadata, owner_user_id, vendor_type, website_url, application_id,
			created_at, updated_at
		FROM monitors WHERE provider = $1 AND service = $2 AND channel = $3
	`
	monitor := &Monitor{}
	err := s.pool.QueryRow(ctx, query, provider, service, channel).Scan(
		&monitor.ID, &monitor.Provider, &monitor.ProviderName, &monitor.Service, &monitor.ServiceName,
		&monitor.Channel, &monitor.ChannelName, &monitor.Model,
		&monitor.TemplateID, &monitor.URL, &monitor.Method, &monitor.Headers, &monitor.Body, &monitor.SuccessContains,
		&monitor.IntervalOverride, &monitor.TimeoutOverride, &monitor.SlowLatencyOverride, &monitor.Enabled, &monitor.BoardID,
		&monitor.Metadata, &monitor.OwnerUserID, &monitor.VendorType, &monitor.WebsiteURL, &monitor.ApplicationID,
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
			interval_override = $12, timeout_override = $13, slow_latency_override = $14, enabled = $15, board_id = $16,
			metadata = $17, vendor_type = $18, website_url = $19, updated_at = $20
		WHERE id = $1
	`
	_, err := s.pool.Exec(ctx, query,
		monitor.ID, monitor.ProviderName, monitor.ServiceName, monitor.ChannelName, monitor.Model,
		monitor.TemplateID, monitor.URL, monitor.Method, monitor.Headers, monitor.Body, monitor.SuccessContains,
		monitor.IntervalOverride, monitor.TimeoutOverride, monitor.SlowLatencyOverride, monitor.Enabled, monitor.BoardID,
		monitor.Metadata, monitor.VendorType, monitor.WebsiteURL, monitor.UpdatedAt,
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
			interval_override, timeout_override, slow_latency_override, enabled, board_id,
			metadata, owner_user_id, vendor_type, website_url, application_id,
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
			&monitor.IntervalOverride, &monitor.TimeoutOverride, &monitor.SlowLatencyOverride, &monitor.Enabled, &monitor.BoardID,
			&monitor.Metadata, &monitor.OwnerUserID, &monitor.VendorType, &monitor.WebsiteURL, &monitor.ApplicationID,
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
	"provider_name":         true,
	"service_name":          true,
	"channel_name":          true,
	"model":                 true,
	"template_id":           true,
	"url":                   true,
	"method":                true,
	"headers":               true,
	"body":                  true,
	"success_contains":      true,
	"interval_override":     true,
	"timeout_override":      true,
	"slow_latency_override": true,
	"enabled":               true,
	"board_id":              true,
	"metadata":              true,
	"vendor_type":           true,
	"website_url":           true,
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

// =====================================================
// V1ConfigProvider 存储接口实现
// =====================================================

// V1MonitorWithTemplate 带模板数据的监测项（供 config.V1ConfigProvider 使用）
type V1MonitorWithTemplate struct {
	// 监测项基础信息
	ID           int
	Provider     string
	ProviderName string
	Service      string
	ServiceName  string
	Channel      string
	ChannelName  string
	Model        string

	// 模板关联
	TemplateID *int

	// 监测项配置
	URL             string
	Method          *string
	Headers         []byte
	Body            []byte
	SuccessContains *string

	// 时间配置覆盖
	IntervalOverride    *string
	TimeoutOverride     *string
	SlowLatencyOverride *string

	// 状态
	Enabled bool
	BoardID *int

	// 元数据
	Metadata    []byte
	OwnerUserID *string
	VendorType  string
	WebsiteURL  string

	// 时间戳
	CreatedAt int64
	UpdatedAt int64

	// 关联的模板数据
	TemplateServiceID          *string
	TemplateName               *string
	TemplateSlug               *string
	TemplateRequestMethod      *string
	TemplateBaseRequestHeaders []byte
	TemplateBaseRequestBody    []byte
	TemplateBaseResponseChecks []byte
	TemplateTimeoutMs          *int
	TemplateSlowLatencyMs      *int
}

// ListEnabledMonitorsWithTemplatesInternal 查询所有启用的监测项（含模板数据）
// 供 v1_config_adapter 调用
func (s *PostgresStorage) ListEnabledMonitorsWithTemplatesInternal(ctx context.Context) ([]*V1MonitorWithTemplate, error) {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		SELECT
			m.id, m.provider, m.provider_name, m.service, m.service_name,
			m.channel, m.channel_name, m.model,
			m.template_id,
			m.url, m.method, m.headers, m.body, m.success_contains,
			m.interval_override, m.timeout_override, m.slow_latency_override,
			m.enabled, m.board_id,
			m.metadata, m.owner_user_id, m.vendor_type, m.website_url,
			m.created_at, m.updated_at,
			-- 模板字段（LEFT JOIN）
			t.service_id, t.name, t.slug,
			t.request_method, t.base_request_headers, t.base_request_body, t.base_response_checks,
			t.timeout_ms, t.slow_latency_ms
		FROM monitors m
		LEFT JOIN monitor_templates t ON m.template_id = t.id
		WHERE m.enabled = true
		ORDER BY m.service, m.provider, m.channel
	`

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("查询 v1 监测项失败: %w", err)
	}
	defer rows.Close()

	var result []*V1MonitorWithTemplate
	for rows.Next() {
		m := &V1MonitorWithTemplate{}
		err := rows.Scan(
			&m.ID, &m.Provider, &m.ProviderName, &m.Service, &m.ServiceName,
			&m.Channel, &m.ChannelName, &m.Model,
			&m.TemplateID,
			&m.URL, &m.Method, &m.Headers, &m.Body, &m.SuccessContains,
			&m.IntervalOverride, &m.TimeoutOverride, &m.SlowLatencyOverride,
			&m.Enabled, &m.BoardID,
			&m.Metadata, &m.OwnerUserID, &m.VendorType, &m.WebsiteURL,
			&m.CreatedAt, &m.UpdatedAt,
			// 模板字段
			&m.TemplateServiceID, &m.TemplateName, &m.TemplateSlug,
			&m.TemplateRequestMethod, &m.TemplateBaseRequestHeaders,
			&m.TemplateBaseRequestBody, &m.TemplateBaseResponseChecks,
			&m.TemplateTimeoutMs, &m.TemplateSlowLatencyMs,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描 v1 监测项数据失败: %w", err)
		}
		result = append(result, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历 v1 监测项数据失败: %w", err)
	}

	return result, nil
}

// GetMonitorSecretByV1ID 按 v1 监测项 ID 获取 API Key 密文
// v1 monitors 表的 API Key 存储在 monitor_secrets 表，使用 monitors.id 作为外键
func (s *PostgresStorage) GetMonitorSecretByV1ID(ctx context.Context, monitorID int) (*MonitorSecretRecord, error) {
	if ctx == nil {
		ctx = s.ctx
	}

	// 注意：v1 使用 monitors.id 作为 monitor_id
	// 需要确保 monitor_secrets 表支持 v1 的 ID
	query := `
		SELECT monitor_id, api_key_ciphertext, api_key_nonce, key_version, enc_version
		FROM monitor_secrets
		WHERE monitor_id = $1
	`
	record := &MonitorSecretRecord{}
	err := s.pool.QueryRow(ctx, query, monitorID).Scan(
		&record.MonitorID,
		&record.APIKeyCiphertext,
		&record.APIKeyNonce,
		&record.KeyVersion,
		&record.EncVersion,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("获取 v1 监测项密钥失败: %w", err)
	}
	return record, nil
}

// MonitorSecretRecord API Key 密文记录（复用现有类型）
type MonitorSecretRecord struct {
	MonitorID        int64
	APIKeyCiphertext []byte
	APIKeyNonce      []byte
	KeyVersion       int
	EncVersion       int
}
