package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// =====================================================
// ApplicationStorage 实现
// =====================================================

// CreateApplication 创建申请
func (s *PostgresStorage) CreateApplication(ctx context.Context, app *MonitorApplication) error {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		INSERT INTO monitor_applications (
			applicant_user_id, service_id, template_id, template_snapshot,
			provider_name, channel_name, vendor_type, website_url, request_url,
			api_key_encrypted, api_key_nonce, api_key_version,
			status, reject_reason, reviewer_user_id, reviewed_at, last_test_session_id,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
		RETURNING id
	`
	err := s.pool.QueryRow(ctx, query,
		app.ApplicantUserID, app.ServiceID, app.TemplateID, app.TemplateSnapshot,
		app.ProviderName, app.ChannelName, app.VendorType, app.WebsiteURL, app.RequestURL,
		app.APIKeyEncrypted, app.APIKeyNonce, app.APIKeyVersion,
		app.Status, app.RejectReason, app.ReviewerUserID, app.ReviewedAt, app.LastTestSessionID,
		app.CreatedAt, app.UpdatedAt,
	).Scan(&app.ID)
	if err != nil {
		return fmt.Errorf("创建申请失败: %w", err)
	}
	return nil
}

// GetApplicationByID 按 ID 获取申请
func (s *PostgresStorage) GetApplicationByID(ctx context.Context, id int) (*MonitorApplication, error) {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		SELECT id, applicant_user_id, service_id, template_id, template_snapshot,
			provider_name, channel_name, vendor_type, website_url, request_url,
			api_key_encrypted, api_key_nonce, api_key_version,
			status, reject_reason, reviewer_user_id, reviewed_at, last_test_session_id,
			created_at, updated_at
		FROM monitor_applications WHERE id = $1
	`
	app := &MonitorApplication{}
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&app.ID, &app.ApplicantUserID, &app.ServiceID, &app.TemplateID, &app.TemplateSnapshot,
		&app.ProviderName, &app.ChannelName, &app.VendorType, &app.WebsiteURL, &app.RequestURL,
		&app.APIKeyEncrypted, &app.APIKeyNonce, &app.APIKeyVersion,
		&app.Status, &app.RejectReason, &app.ReviewerUserID, &app.ReviewedAt, &app.LastTestSessionID,
		&app.CreatedAt, &app.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("获取申请失败: %w", err)
	}
	return app, nil
}

// UpdateApplication 更新申请
func (s *PostgresStorage) UpdateApplication(ctx context.Context, app *MonitorApplication) error {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		UPDATE monitor_applications SET
			template_snapshot = $2, provider_name = $3, channel_name = $4,
			vendor_type = $5, website_url = $6, request_url = $7,
			api_key_encrypted = $8, api_key_nonce = $9, api_key_version = $10,
			status = $11, reject_reason = $12, reviewer_user_id = $13, reviewed_at = $14,
			last_test_session_id = $15, updated_at = $16
		WHERE id = $1
	`
	_, err := s.pool.Exec(ctx, query,
		app.ID, app.TemplateSnapshot, app.ProviderName, app.ChannelName,
		app.VendorType, app.WebsiteURL, app.RequestURL,
		app.APIKeyEncrypted, app.APIKeyNonce, app.APIKeyVersion,
		app.Status, app.RejectReason, app.ReviewerUserID, app.ReviewedAt,
		app.LastTestSessionID, app.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("更新申请失败: %w", err)
	}
	return nil
}

// DeleteApplication 删除申请
func (s *PostgresStorage) DeleteApplication(ctx context.Context, id int) error {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `DELETE FROM monitor_applications WHERE id = $1`
	_, err := s.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("删除申请失败: %w", err)
	}
	return nil
}

// ListApplications 列出申请
func (s *PostgresStorage) ListApplications(ctx context.Context, opts *ListApplicationsOptions) ([]*MonitorApplication, int, error) {
	if ctx == nil {
		ctx = s.ctx
	}
	if opts == nil {
		opts = &ListApplicationsOptions{}
	}

	var conditions []string
	var args []interface{}
	argIdx := 1

	if opts.ApplicantUserID != nil {
		conditions = append(conditions, fmt.Sprintf("applicant_user_id = $%d", argIdx))
		args = append(args, *opts.ApplicantUserID)
		argIdx++
	}
	if opts.ServiceID != "" {
		conditions = append(conditions, fmt.Sprintf("service_id = $%d", argIdx))
		args = append(args, opts.ServiceID)
		argIdx++
	}
	if opts.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *opts.Status)
		argIdx++
	}
	if opts.VendorType != nil {
		conditions = append(conditions, fmt.Sprintf("vendor_type = $%d", argIdx))
		args = append(args, *opts.VendorType)
		argIdx++
	}
	if opts.Search != "" {
		conditions = append(conditions, fmt.Sprintf("provider_name ILIKE $%d", argIdx))
		args = append(args, "%"+opts.Search+"%")
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 查询总数
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM monitor_applications %s", whereClause)
	var total int
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("查询申请总数失败: %w", err)
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
		SELECT id, applicant_user_id, service_id, template_id, template_snapshot,
			provider_name, channel_name, vendor_type, website_url, request_url,
			api_key_encrypted, api_key_nonce, api_key_version,
			status, reject_reason, reviewer_user_id, reviewed_at, last_test_session_id,
			created_at, updated_at
		FROM monitor_applications %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询申请列表失败: %w", err)
	}
	defer rows.Close()

	var apps []*MonitorApplication
	for rows.Next() {
		app := &MonitorApplication{}
		if err := rows.Scan(
			&app.ID, &app.ApplicantUserID, &app.ServiceID, &app.TemplateID, &app.TemplateSnapshot,
			&app.ProviderName, &app.ChannelName, &app.VendorType, &app.WebsiteURL, &app.RequestURL,
			&app.APIKeyEncrypted, &app.APIKeyNonce, &app.APIKeyVersion,
			&app.Status, &app.RejectReason, &app.ReviewerUserID, &app.ReviewedAt, &app.LastTestSessionID,
			&app.CreatedAt, &app.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("扫描申请数据失败: %w", err)
		}
		apps = append(apps, app)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("遍历申请数据失败: %w", err)
	}

	return apps, total, nil
}

// GetApplicationWithDetails 获取申请详情（包含关联数据）
func (s *PostgresStorage) GetApplicationWithDetails(ctx context.Context, id int) (*MonitorApplication, error) {
	app, err := s.GetApplicationByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, nil
	}

	// 获取最后一次测试会话
	if app.LastTestSessionID != nil {
		session, err := s.GetTestSessionByID(ctx, *app.LastTestSessionID)
		if err != nil {
			return nil, fmt.Errorf("获取测试会话失败: %w", err)
		}
		app.TestSession = session
	}

	return app, nil
}

// =====================================================
// ApplicationTestSession 实现
// =====================================================

// CreateTestSession 创建测试会话
func (s *PostgresStorage) CreateTestSession(ctx context.Context, session *ApplicationTestSession) error {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		INSERT INTO application_test_sessions (
			application_id, template_snapshot, status, summary, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`
	err := s.pool.QueryRow(ctx, query,
		session.ApplicationID, session.TemplateSnapshot, session.Status, session.Summary,
		session.CreatedAt, session.UpdatedAt,
	).Scan(&session.ID)
	if err != nil {
		return fmt.Errorf("创建测试会话失败: %w", err)
	}
	return nil
}

// GetTestSessionByID 按 ID 获取测试会话
func (s *PostgresStorage) GetTestSessionByID(ctx context.Context, id int) (*ApplicationTestSession, error) {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		SELECT id, application_id, template_snapshot, status, summary, created_at, updated_at
		FROM application_test_sessions WHERE id = $1
	`
	session := &ApplicationTestSession{}
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&session.ID, &session.ApplicationID, &session.TemplateSnapshot, &session.Status, &session.Summary,
		&session.CreatedAt, &session.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("获取测试会话失败: %w", err)
	}

	// 获取测试结果
	results, err := s.ListTestResults(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("获取测试结果失败: %w", err)
	}
	// 转换 []*ApplicationTestResult 为 []ApplicationTestResult
	session.Results = make([]ApplicationTestResult, len(results))
	for i, r := range results {
		session.Results[i] = *r
	}

	return session, nil
}

// UpdateTestSession 更新测试会话
func (s *PostgresStorage) UpdateTestSession(ctx context.Context, session *ApplicationTestSession) error {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		UPDATE application_test_sessions SET
			status = $2, summary = $3, updated_at = $4
		WHERE id = $1
	`
	_, err := s.pool.Exec(ctx, query,
		session.ID, session.Status, session.Summary, session.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("更新测试会话失败: %w", err)
	}
	return nil
}

// ListTestSessions 列出申请的测试会话
func (s *PostgresStorage) ListTestSessions(ctx context.Context, applicationID int) ([]*ApplicationTestSession, error) {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		SELECT id, application_id, template_snapshot, status, summary, created_at, updated_at
		FROM application_test_sessions
		WHERE application_id = $1
		ORDER BY created_at DESC
	`
	rows, err := s.pool.Query(ctx, query, applicationID)
	if err != nil {
		return nil, fmt.Errorf("查询测试会话列表失败: %w", err)
	}
	defer rows.Close()

	var sessions []*ApplicationTestSession
	for rows.Next() {
		session := &ApplicationTestSession{}
		if err := rows.Scan(
			&session.ID, &session.ApplicationID, &session.TemplateSnapshot, &session.Status, &session.Summary,
			&session.CreatedAt, &session.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描测试会话数据失败: %w", err)
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历测试会话数据失败: %w", err)
	}

	return sessions, nil
}

// =====================================================
// ApplicationTestResult 实现
// =====================================================

// CreateTestResult 创建测试结果
func (s *PostgresStorage) CreateTestResult(ctx context.Context, result *ApplicationTestResult) error {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		INSERT INTO application_test_results (
			session_id, template_model_id, model_key,
			status, latency_ms, http_code, error_message,
			request_snapshot, response_snapshot, checked_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`
	err := s.pool.QueryRow(ctx, query,
		result.SessionID, result.TemplateModelID, result.ModelKey,
		result.Status, result.LatencyMs, result.HTTPCode, result.ErrorMessage,
		result.RequestSnapshot, result.ResponseSnapshot, result.CheckedAt,
	).Scan(&result.ID)
	if err != nil {
		return fmt.Errorf("创建测试结果失败: %w", err)
	}
	return nil
}

// ListTestResults 列出测试会话的结果
func (s *PostgresStorage) ListTestResults(ctx context.Context, sessionID int) ([]*ApplicationTestResult, error) {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		SELECT id, session_id, template_model_id, model_key,
			status, latency_ms, http_code, error_message,
			request_snapshot, response_snapshot, checked_at
		FROM application_test_results
		WHERE session_id = $1
		ORDER BY id ASC
	`
	rows, err := s.pool.Query(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("查询测试结果列表失败: %w", err)
	}
	defer rows.Close()

	var results []*ApplicationTestResult
	for rows.Next() {
		result := &ApplicationTestResult{}
		if err := rows.Scan(
			&result.ID, &result.SessionID, &result.TemplateModelID, &result.ModelKey,
			&result.Status, &result.LatencyMs, &result.HTTPCode, &result.ErrorMessage,
			&result.RequestSnapshot, &result.ResponseSnapshot, &result.CheckedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描测试结果数据失败: %w", err)
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历测试结果数据失败: %w", err)
	}

	return results, nil
}

// =====================================================
// AuditLogStorage 实现
// =====================================================

// CreateAuditLog 创建审计日志
func (s *PostgresStorage) CreateAuditLog(ctx context.Context, log *AdminAuditLog) error {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		INSERT INTO admin_audit_logs (
			user_id, action, resource_type, resource_id, changes, ip_address, user_agent, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`
	err := s.pool.QueryRow(ctx, query,
		log.UserID, log.Action, log.ResourceType, log.ResourceID, log.Changes,
		log.IPAddress, log.UserAgent, log.CreatedAt,
	).Scan(&log.ID)
	if err != nil {
		return fmt.Errorf("创建审计日志失败: %w", err)
	}
	return nil
}

// ListAuditLogs 列出审计日志
func (s *PostgresStorage) ListAuditLogs(ctx context.Context, opts *ListAuditLogsOptions) ([]*AdminAuditLog, int, error) {
	if ctx == nil {
		ctx = s.ctx
	}
	if opts == nil {
		opts = &ListAuditLogsOptions{}
	}

	var conditions []string
	var args []interface{}
	argIdx := 1

	if opts.UserID != nil {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", argIdx))
		args = append(args, *opts.UserID)
		argIdx++
	}
	if opts.Action != nil {
		conditions = append(conditions, fmt.Sprintf("action = $%d", argIdx))
		args = append(args, *opts.Action)
		argIdx++
	}
	if opts.ResourceType != nil {
		conditions = append(conditions, fmt.Sprintf("resource_type = $%d", argIdx))
		args = append(args, *opts.ResourceType)
		argIdx++
	}
	if opts.ResourceID != "" {
		conditions = append(conditions, fmt.Sprintf("resource_id = $%d", argIdx))
		args = append(args, opts.ResourceID)
		argIdx++
	}
	if opts.StartTime != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIdx))
		args = append(args, *opts.StartTime)
		argIdx++
	}
	if opts.EndTime != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIdx))
		args = append(args, *opts.EndTime)
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 查询总数
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM admin_audit_logs %s", whereClause)
	var total int
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("查询审计日志总数失败: %w", err)
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
		SELECT id, user_id, action, resource_type, resource_id, changes, ip_address, user_agent, created_at
		FROM admin_audit_logs %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询审计日志列表失败: %w", err)
	}
	defer rows.Close()

	var logs []*AdminAuditLog
	for rows.Next() {
		log := &AdminAuditLog{}
		if err := rows.Scan(
			&log.ID, &log.UserID, &log.Action, &log.ResourceType, &log.ResourceID, &log.Changes,
			&log.IPAddress, &log.UserAgent, &log.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("扫描审计日志数据失败: %w", err)
		}
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("遍历审计日志数据失败: %w", err)
	}

	return logs, total, nil
}
