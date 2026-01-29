package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// =====================================================
// ServiceStorage 实现
// =====================================================

// CreateService 创建服务
func (s *PostgresStorage) CreateService(ctx context.Context, service *Service) error {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		INSERT INTO services (id, name, icon_svg, default_template_id, status, sort_order, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := s.pool.Exec(ctx, query,
		service.ID, service.Name, service.IconSVG, service.DefaultTemplateID,
		service.Status, service.SortOrder, service.CreatedAt, service.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("创建服务失败: %w", err)
	}
	return nil
}

// GetServiceByID 按 ID 获取服务
func (s *PostgresStorage) GetServiceByID(ctx context.Context, id string) (*Service, error) {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		SELECT id, name, icon_svg, default_template_id, status, sort_order, created_at, updated_at
		FROM services WHERE id = $1
	`
	service := &Service{}
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&service.ID, &service.Name, &service.IconSVG, &service.DefaultTemplateID,
		&service.Status, &service.SortOrder, &service.CreatedAt, &service.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("获取服务失败: %w", err)
	}
	return service, nil
}

// UpdateService 更新服务
func (s *PostgresStorage) UpdateService(ctx context.Context, service *Service) error {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		UPDATE services SET
			name = $2, icon_svg = $3, default_template_id = $4, status = $5, sort_order = $6, updated_at = $7
		WHERE id = $1
	`
	_, err := s.pool.Exec(ctx, query,
		service.ID, service.Name, service.IconSVG, service.DefaultTemplateID,
		service.Status, service.SortOrder, service.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("更新服务失败: %w", err)
	}
	return nil
}

// DeleteService 删除服务
func (s *PostgresStorage) DeleteService(ctx context.Context, id string) error {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `DELETE FROM services WHERE id = $1`
	_, err := s.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("删除服务失败: %w", err)
	}
	return nil
}

// ListServices 列出服务
func (s *PostgresStorage) ListServices(ctx context.Context, opts *ListServicesOptions) ([]*Service, error) {
	if ctx == nil {
		ctx = s.ctx
	}
	if opts == nil {
		opts = &ListServicesOptions{}
	}

	var conditions []string
	var args []interface{}
	argIdx := 1

	if opts.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, opts.Status)
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, name, icon_svg, default_template_id, status, sort_order, created_at, updated_at
		FROM services %s
		ORDER BY sort_order ASC, created_at ASC
	`, whereClause)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询服务列表失败: %w", err)
	}
	defer rows.Close()

	var services []*Service
	for rows.Next() {
		service := &Service{}
		if err := rows.Scan(
			&service.ID, &service.Name, &service.IconSVG, &service.DefaultTemplateID,
			&service.Status, &service.SortOrder, &service.CreatedAt, &service.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描服务数据失败: %w", err)
		}
		services = append(services, service)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历服务数据失败: %w", err)
	}

	return services, nil
}

// =====================================================
// TemplateStorage 实现
// =====================================================

// CreateTemplate 创建模板
func (s *PostgresStorage) CreateTemplate(ctx context.Context, template *MonitorTemplate) error {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		INSERT INTO monitor_templates (
			service_id, name, slug, description, is_default,
			request_method, base_request_headers, base_request_body, base_response_checks,
			timeout_ms, slow_latency_ms, retry_policy,
			created_by, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id
	`
	err := s.pool.QueryRow(ctx, query,
		template.ServiceID, template.Name, template.Slug, template.Description, template.IsDefault,
		template.RequestMethod, template.BaseRequestHeaders, template.BaseRequestBody, template.BaseResponseChecks,
		template.TimeoutMs, template.SlowLatencyMs, template.RetryPolicy,
		template.CreatedBy, template.CreatedAt, template.UpdatedAt,
	).Scan(&template.ID)
	if err != nil {
		return fmt.Errorf("创建模板失败: %w", err)
	}
	return nil
}

// GetTemplateByID 按 ID 获取模板
func (s *PostgresStorage) GetTemplateByID(ctx context.Context, id int) (*MonitorTemplate, error) {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		SELECT id, service_id, name, slug, description, is_default,
			request_method, base_request_headers, base_request_body, base_response_checks,
			timeout_ms, slow_latency_ms, retry_policy,
			created_by, created_at, updated_at
		FROM monitor_templates WHERE id = $1
	`
	template := &MonitorTemplate{}
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&template.ID, &template.ServiceID, &template.Name, &template.Slug, &template.Description, &template.IsDefault,
		&template.RequestMethod, &template.BaseRequestHeaders, &template.BaseRequestBody, &template.BaseResponseChecks,
		&template.TimeoutMs, &template.SlowLatencyMs, &template.RetryPolicy,
		&template.CreatedBy, &template.CreatedAt, &template.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("获取模板失败: %w", err)
	}
	return template, nil
}

// GetTemplateBySlug 按服务 ID 和 slug 获取模板
func (s *PostgresStorage) GetTemplateBySlug(ctx context.Context, serviceID, slug string) (*MonitorTemplate, error) {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		SELECT id, service_id, name, slug, description, is_default,
			request_method, base_request_headers, base_request_body, base_response_checks,
			timeout_ms, slow_latency_ms, retry_policy,
			created_by, created_at, updated_at
		FROM monitor_templates WHERE service_id = $1 AND slug = $2
	`
	template := &MonitorTemplate{}
	err := s.pool.QueryRow(ctx, query, serviceID, slug).Scan(
		&template.ID, &template.ServiceID, &template.Name, &template.Slug, &template.Description, &template.IsDefault,
		&template.RequestMethod, &template.BaseRequestHeaders, &template.BaseRequestBody, &template.BaseResponseChecks,
		&template.TimeoutMs, &template.SlowLatencyMs, &template.RetryPolicy,
		&template.CreatedBy, &template.CreatedAt, &template.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("获取模板失败: %w", err)
	}
	return template, nil
}

// UpdateTemplate 更新模板
func (s *PostgresStorage) UpdateTemplate(ctx context.Context, template *MonitorTemplate) error {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		UPDATE monitor_templates SET
			name = $2, slug = $3, description = $4, is_default = $5,
			request_method = $6, base_request_headers = $7, base_request_body = $8, base_response_checks = $9,
			timeout_ms = $10, slow_latency_ms = $11, retry_policy = $12,
			updated_at = $13
		WHERE id = $1
	`
	_, err := s.pool.Exec(ctx, query,
		template.ID, template.Name, template.Slug, template.Description, template.IsDefault,
		template.RequestMethod, template.BaseRequestHeaders, template.BaseRequestBody, template.BaseResponseChecks,
		template.TimeoutMs, template.SlowLatencyMs, template.RetryPolicy,
		template.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("更新模板失败: %w", err)
	}
	return nil
}

// DeleteTemplate 删除模板
func (s *PostgresStorage) DeleteTemplate(ctx context.Context, id int) error {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `DELETE FROM monitor_templates WHERE id = $1`
	_, err := s.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("删除模板失败: %w", err)
	}
	return nil
}

// ListTemplates 列出模板
func (s *PostgresStorage) ListTemplates(ctx context.Context, opts *ListTemplatesOptions) ([]*MonitorTemplate, int, error) {
	if ctx == nil {
		ctx = s.ctx
	}
	if opts == nil {
		opts = &ListTemplatesOptions{}
	}

	var conditions []string
	var args []interface{}
	argIdx := 1

	if opts.ServiceID != "" {
		conditions = append(conditions, fmt.Sprintf("service_id = $%d", argIdx))
		args = append(args, opts.ServiceID)
		argIdx++
	}
	if opts.IsDefault != nil {
		conditions = append(conditions, fmt.Sprintf("is_default = $%d", argIdx))
		args = append(args, *opts.IsDefault)
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 查询总数
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM monitor_templates %s", whereClause)
	var total int
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("查询模板总数失败: %w", err)
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
		SELECT id, service_id, name, slug, description, is_default,
			request_method, base_request_headers, base_request_body, base_response_checks,
			timeout_ms, slow_latency_ms, retry_policy,
			created_by, created_at, updated_at
		FROM monitor_templates %s
		ORDER BY service_id ASC, is_default DESC, created_at ASC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询模板列表失败: %w", err)
	}
	defer rows.Close()

	var templates []*MonitorTemplate
	for rows.Next() {
		template := &MonitorTemplate{}
		if err := rows.Scan(
			&template.ID, &template.ServiceID, &template.Name, &template.Slug, &template.Description, &template.IsDefault,
			&template.RequestMethod, &template.BaseRequestHeaders, &template.BaseRequestBody, &template.BaseResponseChecks,
			&template.TimeoutMs, &template.SlowLatencyMs, &template.RetryPolicy,
			&template.CreatedBy, &template.CreatedAt, &template.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("扫描模板数据失败: %w", err)
		}
		templates = append(templates, template)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("遍历模板数据失败: %w", err)
	}

	// 如果需要包含模型列表
	if opts.WithModels && len(templates) > 0 {
		for _, template := range templates {
			models, err := s.ListTemplateModels(ctx, template.ID)
			if err != nil {
				return nil, 0, fmt.Errorf("获取模板模型失败: %w", err)
			}
			// 转换 []*MonitorTemplateModel 为 []MonitorTemplateModel
			template.Models = make([]MonitorTemplateModel, len(models))
			for i, m := range models {
				template.Models[i] = *m
			}
		}
	}

	return templates, total, nil
}

// =====================================================
// TemplateModel 实现
// =====================================================

// CreateTemplateModel 创建模板模型
func (s *PostgresStorage) CreateTemplateModel(ctx context.Context, model *MonitorTemplateModel) error {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		INSERT INTO monitor_template_models (
			template_id, model_key, display_name,
			request_body_overrides, response_checks_overrides,
			enabled, sort_order, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`
	err := s.pool.QueryRow(ctx, query,
		model.TemplateID, model.ModelKey, model.DisplayName,
		model.RequestBodyOverrides, model.ResponseChecksOverrides,
		model.Enabled, model.SortOrder, model.CreatedAt, model.UpdatedAt,
	).Scan(&model.ID)
	if err != nil {
		return fmt.Errorf("创建模板模型失败: %w", err)
	}
	return nil
}

// GetTemplateModelByID 按 ID 获取模板模型
func (s *PostgresStorage) GetTemplateModelByID(ctx context.Context, id int) (*MonitorTemplateModel, error) {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		SELECT id, template_id, model_key, display_name,
			request_body_overrides, response_checks_overrides,
			enabled, sort_order, created_at, updated_at
		FROM monitor_template_models WHERE id = $1
	`
	model := &MonitorTemplateModel{}
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&model.ID, &model.TemplateID, &model.ModelKey, &model.DisplayName,
		&model.RequestBodyOverrides, &model.ResponseChecksOverrides,
		&model.Enabled, &model.SortOrder, &model.CreatedAt, &model.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("获取模板模型失败: %w", err)
	}
	return model, nil
}

// UpdateTemplateModel 更新模板模型
func (s *PostgresStorage) UpdateTemplateModel(ctx context.Context, model *MonitorTemplateModel) error {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		UPDATE monitor_template_models SET
			model_key = $2, display_name = $3,
			request_body_overrides = $4, response_checks_overrides = $5,
			enabled = $6, sort_order = $7, updated_at = $8
		WHERE id = $1
	`
	_, err := s.pool.Exec(ctx, query,
		model.ID, model.ModelKey, model.DisplayName,
		model.RequestBodyOverrides, model.ResponseChecksOverrides,
		model.Enabled, model.SortOrder, model.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("更新模板模型失败: %w", err)
	}
	return nil
}

// DeleteTemplateModel 删除模板模型
func (s *PostgresStorage) DeleteTemplateModel(ctx context.Context, id int) error {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `DELETE FROM monitor_template_models WHERE id = $1`
	_, err := s.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("删除模板模型失败: %w", err)
	}
	return nil
}

// ListTemplateModels 列出模板的所有模型
func (s *PostgresStorage) ListTemplateModels(ctx context.Context, templateID int) ([]*MonitorTemplateModel, error) {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		SELECT id, template_id, model_key, display_name,
			request_body_overrides, response_checks_overrides,
			enabled, sort_order, created_at, updated_at
		FROM monitor_template_models
		WHERE template_id = $1
		ORDER BY sort_order ASC, created_at ASC
	`
	rows, err := s.pool.Query(ctx, query, templateID)
	if err != nil {
		return nil, fmt.Errorf("查询模板模型列表失败: %w", err)
	}
	defer rows.Close()

	var models []*MonitorTemplateModel
	for rows.Next() {
		model := &MonitorTemplateModel{}
		if err := rows.Scan(
			&model.ID, &model.TemplateID, &model.ModelKey, &model.DisplayName,
			&model.RequestBodyOverrides, &model.ResponseChecksOverrides,
			&model.Enabled, &model.SortOrder, &model.CreatedAt, &model.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描模板模型数据失败: %w", err)
		}
		models = append(models, model)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历模板模型数据失败: %w", err)
	}

	return models, nil
}

// GetTemplateWithModels 获取模板及其所有模型
func (s *PostgresStorage) GetTemplateWithModels(ctx context.Context, id int) (*MonitorTemplate, error) {
	template, err := s.GetTemplateByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, nil
	}

	models, err := s.ListTemplateModels(ctx, id)
	if err != nil {
		return nil, err
	}
	// 转换 []*MonitorTemplateModel 为 []MonitorTemplateModel
	template.Models = make([]MonitorTemplateModel, len(models))
	for i, m := range models {
		template.Models[i] = *m
	}

	return template, nil
}
