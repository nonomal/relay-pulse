package change

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PgxStore 基于 pgxpool 的 Store 实现（适用于 PostgreSQL）。
type PgxStore struct {
	pool *pgxpool.Pool
}

// NewPgxStore 创建基于 pgxpool 的 Store。
func NewPgxStore(pool *pgxpool.Pool) *PgxStore {
	return &PgxStore{pool: pool}
}

// InitTable 创建 change_requests 表和索引。
func (s *PgxStore) InitTable(ctx context.Context) error {
	schema := `
	CREATE TABLE IF NOT EXISTS change_requests (
		id                BIGSERIAL PRIMARY KEY,
		public_id         TEXT NOT NULL UNIQUE,
		status            TEXT NOT NULL DEFAULT 'pending',

		target_provider   TEXT NOT NULL,
		target_service    TEXT NOT NULL,
		target_channel    TEXT NOT NULL,
		target_key        TEXT NOT NULL,
		apply_mode        TEXT NOT NULL,

		auth_fingerprint  TEXT NOT NULL,
		auth_last4        TEXT NOT NULL,

		current_snapshot  TEXT NOT NULL,
		proposed_changes  TEXT NOT NULL,

		new_key_encrypted   TEXT,
		new_key_fingerprint TEXT,
		new_key_last4       TEXT,

		requires_test     BOOLEAN NOT NULL DEFAULT FALSE,
		test_job_id       TEXT,
		test_passed_at    BIGINT,
		test_latency_ms   INTEGER,
		test_http_code    INTEGER,

		admin_note        TEXT,
		reviewed_at       BIGINT,
		applied_at        BIGINT,

		submitter_ip_hash TEXT,
		locale            TEXT,

		created_at        BIGINT NOT NULL,
		updated_at        BIGINT NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_change_status ON change_requests(status, created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_change_target ON change_requests(target_key);
	`
	_, err := s.pool.Exec(ctx, schema)
	return err
}

const pgxChangeAllColumns = `id, public_id, status,
	target_provider, target_service, target_channel, target_key, apply_mode,
	auth_fingerprint, auth_last4,
	current_snapshot, proposed_changes,
	new_key_encrypted, new_key_fingerprint, new_key_last4,
	requires_test, test_job_id, test_passed_at, test_latency_ms, test_http_code,
	admin_note, reviewed_at, applied_at,
	submitter_ip_hash, locale,
	created_at, updated_at`

// Save 保存新变更请求
func (s *PgxStore) Save(ctx context.Context, r *ChangeRequest) error {
	query := `
	INSERT INTO change_requests (
		public_id, status,
		target_provider, target_service, target_channel, target_key, apply_mode,
		auth_fingerprint, auth_last4,
		current_snapshot, proposed_changes,
		new_key_encrypted, new_key_fingerprint, new_key_last4,
		requires_test, test_job_id, test_passed_at, test_latency_ms, test_http_code,
		admin_note, reviewed_at, applied_at,
		submitter_ip_hash, locale,
		created_at, updated_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26)
	RETURNING id`

	err := s.pool.QueryRow(ctx, query,
		r.PublicID, r.Status,
		r.TargetProvider, r.TargetService, r.TargetChannel, r.TargetKey, r.ApplyMode,
		r.AuthFingerprint, r.AuthLast4,
		r.CurrentSnapshot, r.ProposedChanges,
		pgxNullStr(r.NewKeyEncrypted), pgxNullStr(r.NewKeyFingerprint), pgxNullStr(r.NewKeyLast4),
		r.RequiresTest, pgxNullStr(r.TestJobID), pgxNullInt64(r.TestPassedAt), pgxNullInt(r.TestLatency), pgxNullInt(r.TestHTTPCode),
		pgxNullStr(r.AdminNote), r.ReviewedAt, r.AppliedAt,
		pgxNullStr(r.SubmitterIPHash), pgxNullStr(r.Locale),
		r.CreatedAt, r.UpdatedAt,
	).Scan(&r.ID)
	if err != nil {
		return fmt.Errorf("保存变更请求失败: %w", err)
	}
	return nil
}

// GetByPublicID 按公开 ID 查询
func (s *PgxStore) GetByPublicID(ctx context.Context, publicID string) (*ChangeRequest, error) {
	return s.scanOne(ctx, "SELECT "+pgxChangeAllColumns+" FROM change_requests WHERE public_id = $1", publicID)
}

// List 列表查询
func (s *PgxStore) List(ctx context.Context, status string, limit, offset int) ([]*ChangeRequest, int, error) {
	var countQuery, listQuery string
	var args []any

	if status != "" && status != "all" {
		countQuery = "SELECT COUNT(*) FROM change_requests WHERE status = $1"
		listQuery = "SELECT " + pgxChangeAllColumns + " FROM change_requests WHERE status = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3"
		args = []any{status}
	} else {
		countQuery = "SELECT COUNT(*) FROM change_requests"
		listQuery = "SELECT " + pgxChangeAllColumns + " FROM change_requests ORDER BY created_at DESC LIMIT $1 OFFSET $2"
	}

	var total int
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("统计变更请求数失败: %w", err)
	}

	listArgs := append(args, limit, offset)
	rows, err := s.pool.Query(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询变更请求列表失败: %w", err)
	}
	defer rows.Close()

	var results []*ChangeRequest
	for rows.Next() {
		cr, err := pgxScanChangeRequest(rows)
		if err != nil {
			return nil, 0, err
		}
		results = append(results, cr)
	}
	return results, total, rows.Err()
}

// Update 更新变更请求
func (s *PgxStore) Update(ctx context.Context, r *ChangeRequest) error {
	query := `
	UPDATE change_requests SET
		status = $1, apply_mode = $2,
		current_snapshot = $3, proposed_changes = $4,
		new_key_encrypted = $5, new_key_fingerprint = $6, new_key_last4 = $7,
		requires_test = $8, test_job_id = $9, test_passed_at = $10, test_latency_ms = $11, test_http_code = $12,
		admin_note = $13, reviewed_at = $14, applied_at = $15,
		updated_at = $16
	WHERE id = $17`

	_, err := s.pool.Exec(ctx, query,
		r.Status, r.ApplyMode,
		r.CurrentSnapshot, r.ProposedChanges,
		pgxNullStr(r.NewKeyEncrypted), pgxNullStr(r.NewKeyFingerprint), pgxNullStr(r.NewKeyLast4),
		r.RequiresTest, pgxNullStr(r.TestJobID), pgxNullInt64(r.TestPassedAt), pgxNullInt(r.TestLatency), pgxNullInt(r.TestHTTPCode),
		pgxNullStr(r.AdminNote), r.ReviewedAt, r.AppliedAt,
		r.UpdatedAt,
		r.ID,
	)
	if err != nil {
		return fmt.Errorf("更新变更请求失败: %w", err)
	}
	return nil
}

// CountByIPToday 统计今天的提交数
func (s *PgxStore) CountByIPToday(ctx context.Context, ipHash string) (int, error) {
	start, end := todayRange()
	var count int
	err := s.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM change_requests WHERE submitter_ip_hash = $1 AND created_at >= $2 AND created_at < $3",
		ipHash, start, end,
	).Scan(&count)
	return count, err
}

// DeleteByPublicID 按公开 ID 删除
func (s *PgxStore) DeleteByPublicID(ctx context.Context, publicID string) error {
	tag, err := s.pool.Exec(ctx, "DELETE FROM change_requests WHERE public_id = $1", publicID)
	if err != nil {
		return fmt.Errorf("删除变更请求失败: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("变更请求不存在")
	}
	return nil
}

// === 内部辅助 ===

func pgxScanChangeRequest(row pgx.Row) (*ChangeRequest, error) {
	var cr ChangeRequest
	var newKeyEnc, newKeyFP, newKeyL4 *string
	var testJobID *string
	var testPassedAt *int64
	var testLatency, testHTTPCode *int
	var adminNote *string
	var reviewedAt, appliedAt *int64
	var ipHash, locale *string

	err := row.Scan(
		&cr.ID, &cr.PublicID, &cr.Status,
		&cr.TargetProvider, &cr.TargetService, &cr.TargetChannel, &cr.TargetKey, &cr.ApplyMode,
		&cr.AuthFingerprint, &cr.AuthLast4,
		&cr.CurrentSnapshot, &cr.ProposedChanges,
		&newKeyEnc, &newKeyFP, &newKeyL4,
		&cr.RequiresTest, &testJobID, &testPassedAt, &testLatency, &testHTTPCode,
		&adminNote, &reviewedAt, &appliedAt,
		&ipHash, &locale,
		&cr.CreatedAt, &cr.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if newKeyEnc != nil {
		cr.NewKeyEncrypted = *newKeyEnc
	}
	if newKeyFP != nil {
		cr.NewKeyFingerprint = *newKeyFP
	}
	if newKeyL4 != nil {
		cr.NewKeyLast4 = *newKeyL4
	}
	if testJobID != nil {
		cr.TestJobID = *testJobID
	}
	if testPassedAt != nil {
		cr.TestPassedAt = *testPassedAt
	}
	if testLatency != nil {
		cr.TestLatency = *testLatency
	}
	if testHTTPCode != nil {
		cr.TestHTTPCode = *testHTTPCode
	}
	if adminNote != nil {
		cr.AdminNote = *adminNote
	}
	cr.ReviewedAt = reviewedAt
	cr.AppliedAt = appliedAt
	if ipHash != nil {
		cr.SubmitterIPHash = *ipHash
	}
	if locale != nil {
		cr.Locale = *locale
	}

	return &cr, nil
}

func (s *PgxStore) scanOne(ctx context.Context, query string, args ...any) (*ChangeRequest, error) {
	row := s.pool.QueryRow(ctx, query, args...)
	cr, err := pgxScanChangeRequest(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("查询变更请求失败: %w", err)
	}
	return cr, nil
}

func pgxNullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func pgxNullInt64(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}

func pgxNullInt(v int) *int {
	if v == 0 {
		return nil
	}
	return &v
}
