package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// =====================================================
// v0 到 v1 数据迁移
// =====================================================

// v0ConfigBlob v0 版本的 config_blob JSON 结构
// 使用 interface{} 类型以兼容各种格式
type v0ConfigBlob struct {
	URL             string      `json:"url"`
	Method          string      `json:"method"`
	Headers         interface{} `json:"headers"` // 可能是 map[string]string 或其他格式
	Body            interface{} `json:"body"`    // 可能是字符串或对象
	SuccessContains string      `json:"success_contains"`
	Interval        string      `json:"interval"`
	Timeout         string      `json:"timeout"`
	SlowLatency     string      `json:"slow_latency"`
	ProviderName    string      `json:"provider_name"`
	ServiceName     string      `json:"service_name"`
	ChannelName     string      `json:"channel_name"`
	ProviderURL     string      `json:"provider_url"`
	Category        string      `json:"category"`
}

// MigrationStats 迁移统计
type MigrationStats struct {
	Total     int      `json:"total"`
	Migrated  int      `json:"migrated"`
	Skipped   int      `json:"skipped"`
	Failed    int      `json:"failed"`
	FailedIDs []int64  `json:"failed_ids,omitempty"`
	Errors    []string `json:"errors,omitempty"`
}

// MigrateFromV0 从 v0 迁移数据到 v1
// 该方法是幂等的，可以安全地多次调用
func (s *PostgresStorage) MigrateFromV0(ctx context.Context) error {
	if ctx == nil {
		ctx = s.ctx
	}

	stats, err := s.migrateFromV0Internal(ctx)
	if err != nil {
		return fmt.Errorf("迁移失败: %w (统计: %+v)", err, stats)
	}

	if stats.Failed > 0 {
		return fmt.Errorf("迁移完成但有 %d 条失败: %v", stats.Failed, stats.Errors)
	}

	return nil
}

// MigrateFromV0WithStats 从 v0 迁移数据到 v1，返回详细统计
func (s *PostgresStorage) MigrateFromV0WithStats(ctx context.Context) (*MigrationStats, error) {
	if ctx == nil {
		ctx = s.ctx
	}
	return s.migrateFromV0Internal(ctx)
}

// migrateFromV0Internal 内部迁移实现
func (s *PostgresStorage) migrateFromV0Internal(ctx context.Context) (*MigrationStats, error) {
	stats := &MigrationStats{}

	// 分批迁移，每批 100 条
	const batchSize = 100
	var lastID int64 = 0

	for {
		// 查询一批未迁移的 monitor_configs（按 old_id 检查映射）
		query := `
			SELECT mc.id, mc.provider, mc.service, mc.channel, mc.model, mc.name,
				   mc.enabled, mc.config_blob, mc.created_at, mc.updated_at
			FROM monitor_configs mc
			LEFT JOIN monitor_id_mapping mim ON mim.old_id = mc.id
			WHERE mc.deleted_at IS NULL
			  AND mim.old_id IS NULL
			  AND mc.id > $1
			ORDER BY mc.id
			LIMIT $2
		`
		rows, err := s.pool.Query(ctx, query, lastID, batchSize)
		if err != nil {
			return stats, fmt.Errorf("查询 monitor_configs 失败: %w", err)
		}

		var batch []v0MonitorConfig
		for rows.Next() {
			var mc v0MonitorConfig
			if err := rows.Scan(
				&mc.ID, &mc.Provider, &mc.Service, &mc.Channel, &mc.Model, &mc.Name,
				&mc.Enabled, &mc.ConfigBlob, &mc.CreatedAt, &mc.UpdatedAt,
			); err != nil {
				rows.Close()
				return stats, fmt.Errorf("扫描 monitor_configs 失败: %w", err)
			}
			batch = append(batch, mc)
			lastID = mc.ID
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return stats, fmt.Errorf("遍历 monitor_configs 失败: %w", err)
		}

		// 没有更多数据，退出循环
		if len(batch) == 0 {
			break
		}

		stats.Total += len(batch)

		// 处理这一批
		for _, mc := range batch {
			if err := s.migrateOneMonitorConfig(ctx, &mc); err != nil {
				stats.Failed++
				stats.FailedIDs = append(stats.FailedIDs, mc.ID)
				stats.Errors = append(stats.Errors, fmt.Sprintf("id=%d: %v", mc.ID, err))
				continue
			}
			stats.Migrated++
		}
	}

	return stats, nil
}

// v0MonitorConfig v0 版本的 monitor_configs 记录
type v0MonitorConfig struct {
	ID         int64
	Provider   string
	Service    string
	Channel    string
	Model      string
	Name       string
	Enabled    bool
	ConfigBlob string
	CreatedAt  int64
	UpdatedAt  int64
}

// migrateOneMonitorConfig 迁移单条 monitor_config 记录
func (s *PostgresStorage) migrateOneMonitorConfig(ctx context.Context, mc *v0MonitorConfig) error {
	// 解析 config_blob（宽松解析，允许各种格式）
	var blob v0ConfigBlob
	if mc.ConfigBlob != "" {
		if err := json.Unmarshal([]byte(mc.ConfigBlob), &blob); err != nil {
			// 解析失败时使用空 blob，不中止迁移
			blob = v0ConfigBlob{}
		}
	}

	// 准备 headers JSONB（兼容各种格式）
	var headersJSON []byte
	if blob.Headers != nil {
		var err error
		headersJSON, err = json.Marshal(blob.Headers)
		if err != nil {
			headersJSON = nil // 序列化失败时置空
		}
	}

	// 准备 body JSONB（兼容各种格式）
	var bodyJSON []byte
	if blob.Body != nil {
		switch v := blob.Body.(type) {
		case string:
			// 尝试解析为 JSON
			var bodyObj interface{}
			if err := json.Unmarshal([]byte(v), &bodyObj); err == nil {
				bodyJSON = []byte(v)
			} else {
				// 不是有效 JSON，存为字符串
				bodyJSON, _ = json.Marshal(v)
			}
		default:
			// 已经是对象，直接序列化
			bodyJSON, _ = json.Marshal(v)
		}
	}

	// 设置默认值
	method := blob.Method
	if method == "" {
		method = "POST"
	}
	interval := blob.Interval
	if interval == "" {
		interval = "1m"
	}
	timeout := blob.Timeout
	if timeout == "" {
		timeout = "10s"
	}
	slowLatency := blob.SlowLatency
	if slowLatency == "" {
		slowLatency = "5s"
	}

	// 使用事务确保原子性
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback(ctx)

	now := time.Now().Unix()

	// 检查是否已存在相同的 (provider, service, channel) 记录
	var existingID int
	var existingModel string
	checkQuery := `SELECT id, model FROM monitors WHERE provider = $1 AND service = $2 AND channel = $3`
	err = tx.QueryRow(ctx, checkQuery, mc.Provider, mc.Service, mc.Channel).Scan(&existingID, &existingModel)
	if err != nil && err != pgx.ErrNoRows {
		return fmt.Errorf("检查已存在记录失败: %w", err)
	}

	var newID int

	if err == pgx.ErrNoRows {
		// 不存在，插入新记录（保留原始 updated_at）
		insertQuery := `
			INSERT INTO monitors (
				provider, provider_name, service, service_name, channel, channel_name, model,
				url, method, headers, body, success_contains,
				interval_override, timeout_override, slow_latency_override, enabled,
				website_url, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
			RETURNING id
		`
		err = tx.QueryRow(ctx, insertQuery,
			mc.Provider, blob.ProviderName, mc.Service, blob.ServiceName, mc.Channel, blob.ChannelName, mc.Model,
			blob.URL, method, headersJSON, bodyJSON, blob.SuccessContains,
			interval, timeout, slowLatency, mc.Enabled,
			blob.ProviderURL, mc.CreatedAt, mc.UpdatedAt,
		).Scan(&newID)
		if err != nil {
			return fmt.Errorf("插入 monitors 失败: %w", err)
		}
	} else {
		// 已存在，合并 model 字段
		newID = existingID
		mcModel := strings.TrimSpace(mc.Model)
		if mcModel != "" {
			// 使用集合比较，避免子串误判
			existingModels := make(map[string]bool)
			if existingModel != "" {
				for _, m := range strings.Split(existingModel, ",") {
					existingModels[strings.TrimSpace(m)] = true
				}
			}

			if !existingModels[mcModel] {
				// 追加新 model
				newModel := existingModel
				if newModel != "" {
					newModel += "," + mcModel
				} else {
					newModel = mcModel
				}
				updateQuery := `UPDATE monitors SET model = $1, updated_at = $2 WHERE id = $3`
				_, err = tx.Exec(ctx, updateQuery, newModel, now, newID)
				if err != nil {
					return fmt.Errorf("更新 model 失败: %w", err)
				}
			}
		}
	}

	// 创建 ID 映射（使用 old_id 作为冲突键）
	mappingQuery := `
		INSERT INTO monitor_id_mapping (old_id, new_id, legacy_provider, legacy_service, legacy_channel, migrated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT DO NOTHING
	`
	result, err := tx.Exec(ctx, mappingQuery, mc.ID, newID, mc.Provider, mc.Service, mc.Channel, now)
	if err != nil {
		return fmt.Errorf("创建 ID 映射失败: %w", err)
	}
	// 检查是否实际插入了映射（幂等性检查）
	if result.RowsAffected() == 0 {
		// 映射已存在，说明已迁移过，跳过
		return nil
	}

	// 迁移 monitor_secrets（如果存在且新 ID 没有密钥）
	secretQuery := `
		INSERT INTO monitor_secrets (monitor_id, api_key_ciphertext, api_key_nonce, key_version, enc_version, created_at, updated_at)
		SELECT $1, api_key_ciphertext, api_key_nonce, key_version, enc_version, created_at, $2
		FROM monitor_secrets
		WHERE monitor_id = $3
		  AND NOT EXISTS (SELECT 1 FROM monitor_secrets WHERE monitor_id = $1)
	`
	_, err = tx.Exec(ctx, secretQuery, newID, now, mc.ID)
	if err != nil {
		return fmt.Errorf("迁移 monitor_secrets 失败: %w", err)
	}

	// 提交事务
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	return nil
}

// GetMigrationStatus 获取迁移状态
func (s *PostgresStorage) GetMigrationStatus(ctx context.Context) (*MigrationStats, error) {
	if ctx == nil {
		ctx = s.ctx
	}

	stats := &MigrationStats{}

	// 查询总数（未删除的 monitor_configs）
	totalQuery := `SELECT COUNT(*) FROM monitor_configs WHERE deleted_at IS NULL`
	if err := s.pool.QueryRow(ctx, totalQuery).Scan(&stats.Total); err != nil {
		return nil, fmt.Errorf("查询总数失败: %w", err)
	}

	// 查询已迁移数（按 old_id 检查）
	migratedQuery := `
		SELECT COUNT(*) FROM monitor_configs mc
		INNER JOIN monitor_id_mapping mim ON mim.old_id = mc.id
		WHERE mc.deleted_at IS NULL
	`
	if err := s.pool.QueryRow(ctx, migratedQuery).Scan(&stats.Migrated); err != nil {
		return nil, fmt.Errorf("查询已迁移数失败: %w", err)
	}

	// 计算未迁移数
	stats.Skipped = stats.Total - stats.Migrated

	return stats, nil
}
