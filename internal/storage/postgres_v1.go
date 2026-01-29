package storage

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// =====================================================
// v1.0 PostgreSQL 存储实现
// =====================================================

//go:embed ddl_v1.sql
var ddlV1SQL string

// InitV1Tables 初始化 v1.0 表结构
// 该方法是幂等的，可以安全地多次调用
func (s *PostgresStorage) InitV1Tables(ctx context.Context) error {
	// 使用传入的 ctx，如果为 nil 则使用实例的 ctx
	if ctx == nil {
		ctx = s.ctx
	}

	// 分割 DDL 为独立语句执行
	// 注意：DDL 中使用 IF NOT EXISTS，所以是幂等的
	statements := splitSQLStatements(ddlV1SQL)

	for i, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "--") {
			continue
		}

		_, err := s.pool.Exec(ctx, stmt)
		if err != nil {
			return fmt.Errorf("执行 DDL 语句 #%d 失败: %w\nSQL: %s", i+1, err, truncateSQL(stmt, 200))
		}
	}

	return nil
}

// splitSQLStatements 将 SQL 文件分割为独立语句
// 处理分号在字符串内的情况
func splitSQLStatements(sql string) []string {
	var statements []string
	var current strings.Builder
	inString := false
	inDollarQuote := false
	dollarTag := ""

	runes := []rune(sql)
	for i := 0; i < len(runes); i++ {
		char := runes[i]
		current.WriteRune(char)

		// 处理单引号字符串
		// SQL 中 '' 表示转义的单引号，不改变 inString 状态
		if char == '\'' && !inDollarQuote {
			if i+1 < len(runes) && runes[i+1] == '\'' {
				// 这是 '' 转义序列，写入下一个引号并跳过
				current.WriteRune(runes[i+1])
				i++
				// 不改变 inString 状态
			} else {
				// 普通单引号，切换字符串状态
				inString = !inString
			}
		}

		// 处理 $$ 或 $tag$ 美元引用
		if char == '$' && !inString {
			remaining := string(runes[i:])
			if !inDollarQuote {
				// 查找开始的美元引用
				tag := extractDollarTag(remaining)
				if tag != "" {
					inDollarQuote = true
					dollarTag = tag
				}
			} else {
				// 检查是否是结束的美元引用
				if strings.HasPrefix(remaining, dollarTag) {
					inDollarQuote = false
					dollarTag = ""
				}
			}
		}

		// 分号分割（不在字符串或美元引用内）
		if char == ';' && !inString && !inDollarQuote {
			stmt := strings.TrimSpace(current.String())
			if stmt != "" && stmt != ";" {
				statements = append(statements, stmt)
			}
			current.Reset()
		}
	}

	// 处理最后一个语句（可能没有分号）
	if stmt := strings.TrimSpace(current.String()); stmt != "" {
		statements = append(statements, stmt)
	}

	return statements
}

// extractDollarTag 提取美元引用标签（如 $$ 或 $tag$）
func extractDollarTag(s string) string {
	if len(s) < 2 || s[0] != '$' {
		return ""
	}

	// 查找第二个 $
	for i := 1; i < len(s); i++ {
		if s[i] == '$' {
			return s[:i+1]
		}
		// 标签只能包含字母、数字、下划线
		if !isTagChar(s[i]) {
			return ""
		}
	}
	return ""
}

// isTagChar 检查字符是否可以作为美元引用标签的一部分
func isTagChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '_'
}

// truncateSQL 截断 SQL 语句用于错误日志
func truncateSQL(sql string, maxLen int) string {
	sql = strings.TrimSpace(sql)
	if len(sql) <= maxLen {
		return sql
	}
	return sql[:maxLen] + "..."
}

// =====================================================
// UserStorage 实现
// =====================================================

// CreateUser 创建用户
func (s *PostgresStorage) CreateUser(ctx context.Context, user *User) error {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		INSERT INTO users (id, github_id, username, avatar_url, email, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := s.pool.Exec(ctx, query,
		user.ID, user.GitHubID, user.Username, user.AvatarURL, user.Email,
		user.Role, user.Status, user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("创建用户失败: %w", err)
	}
	return nil
}

// GetUserByID 按 ID 获取用户
func (s *PostgresStorage) GetUserByID(ctx context.Context, id string) (*User, error) {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		SELECT id, github_id, username, avatar_url, email, role, status, created_at, updated_at
		FROM users WHERE id = $1
	`
	user := &User{}
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&user.ID, &user.GitHubID, &user.Username, &user.AvatarURL, &user.Email,
		&user.Role, &user.Status, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("获取用户失败: %w", err)
	}
	return user, nil
}

// GetUserByGitHubID 按 GitHub ID 获取用户
func (s *PostgresStorage) GetUserByGitHubID(ctx context.Context, githubID int64) (*User, error) {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		SELECT id, github_id, username, avatar_url, email, role, status, created_at, updated_at
		FROM users WHERE github_id = $1
	`
	user := &User{}
	err := s.pool.QueryRow(ctx, query, githubID).Scan(
		&user.ID, &user.GitHubID, &user.Username, &user.AvatarURL, &user.Email,
		&user.Role, &user.Status, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("获取用户失败: %w", err)
	}
	return user, nil
}

// GetUserByUsername 按用户名获取用户
func (s *PostgresStorage) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		SELECT id, github_id, username, avatar_url, email, role, status, created_at, updated_at
		FROM users WHERE username = $1
	`
	user := &User{}
	err := s.pool.QueryRow(ctx, query, username).Scan(
		&user.ID, &user.GitHubID, &user.Username, &user.AvatarURL, &user.Email,
		&user.Role, &user.Status, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("获取用户失败: %w", err)
	}
	return user, nil
}

// UpdateUser 更新用户
func (s *PostgresStorage) UpdateUser(ctx context.Context, user *User) error {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		UPDATE users SET
			username = $2, avatar_url = $3, email = $4, role = $5, status = $6, updated_at = $7
		WHERE id = $1
	`
	_, err := s.pool.Exec(ctx, query,
		user.ID, user.Username, user.AvatarURL, user.Email,
		user.Role, user.Status, user.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("更新用户失败: %w", err)
	}
	return nil
}

// ListUsers 列出用户
func (s *PostgresStorage) ListUsers(ctx context.Context, opts *ListUsersOptions) ([]*User, int, error) {
	if ctx == nil {
		ctx = s.ctx
	}
	if opts == nil {
		opts = &ListUsersOptions{}
	}

	// 构建查询条件
	var conditions []string
	var args []interface{}
	argIdx := 1

	if opts.Role != nil {
		conditions = append(conditions, fmt.Sprintf("role = $%d", argIdx))
		args = append(args, *opts.Role)
		argIdx++
	}
	if opts.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *opts.Status)
		argIdx++
	}
	if opts.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(username ILIKE $%d OR email ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+opts.Search+"%")
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 查询总数
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM users %s", whereClause)
	var total int
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("查询用户总数失败: %w", err)
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
		SELECT id, github_id, username, avatar_url, email, role, status, created_at, updated_at
		FROM users %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询用户列表失败: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		user := &User{}
		if err := rows.Scan(
			&user.ID, &user.GitHubID, &user.Username, &user.AvatarURL, &user.Email,
			&user.Role, &user.Status, &user.CreatedAt, &user.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("扫描用户数据失败: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("遍历用户数据失败: %w", err)
	}

	return users, total, nil
}

// =====================================================
// UserSession 实现
// =====================================================

// CreateSession 创建会话
func (s *PostgresStorage) CreateSession(ctx context.Context, session *UserSession) error {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		INSERT INTO user_sessions (id, user_id, token_hash, expires_at, created_at, last_seen_at, ip, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := s.pool.Exec(ctx, query,
		session.ID, session.UserID, session.TokenHash,
		session.ExpiresAt, session.CreatedAt, session.LastSeenAt,
		session.IP, session.UserAgent,
	)
	if err != nil {
		return fmt.Errorf("创建会话失败: %w", err)
	}
	return nil
}

// GetSessionByTokenHash 按 token hash 获取会话
func (s *PostgresStorage) GetSessionByTokenHash(ctx context.Context, tokenHash string) (*UserSession, error) {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `
		SELECT id, user_id, token_hash, expires_at, created_at, last_seen_at, revoked_at, ip, user_agent
		FROM user_sessions WHERE token_hash = $1 AND revoked_at IS NULL
	`
	session := &UserSession{}
	err := s.pool.QueryRow(ctx, query, tokenHash).Scan(
		&session.ID, &session.UserID, &session.TokenHash,
		&session.ExpiresAt, &session.CreatedAt, &session.LastSeenAt,
		&session.RevokedAt, &session.IP, &session.UserAgent,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("获取会话失败: %w", err)
	}
	return session, nil
}

// TouchSession 更新会话最后活跃时间并延长过期时间
// 注意：604800 = 7 天（与 SessionExtendedTTL 保持一致）
func (s *PostgresStorage) TouchSession(ctx context.Context, id string) error {
	if ctx == nil {
		ctx = s.ctx
	}

	// 同时更新 last_seen_at 和 expires_at（延长 7 天 = 604800 秒）
	query := `UPDATE user_sessions SET
		last_seen_at = EXTRACT(EPOCH FROM NOW())::BIGINT,
		expires_at = EXTRACT(EPOCH FROM NOW())::BIGINT + 604800
		WHERE id = $1`
	_, err := s.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("更新会话活跃时间失败: %w", err)
	}
	return nil
}

// RevokeSession 撤销会话
func (s *PostgresStorage) RevokeSession(ctx context.Context, id string) error {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `UPDATE user_sessions SET revoked_at = EXTRACT(EPOCH FROM NOW())::BIGINT WHERE id = $1`
	_, err := s.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("撤销会话失败: %w", err)
	}
	return nil
}

// DeleteSession 删除会话
func (s *PostgresStorage) DeleteSession(ctx context.Context, id string) error {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `DELETE FROM user_sessions WHERE id = $1`
	_, err := s.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("删除会话失败: %w", err)
	}
	return nil
}

// DeleteExpiredSessions 删除过期会话
func (s *PostgresStorage) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `DELETE FROM user_sessions WHERE expires_at < EXTRACT(EPOCH FROM NOW())::BIGINT`
	result, err := s.pool.Exec(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("删除过期会话失败: %w", err)
	}
	return result.RowsAffected(), nil
}

// DeleteUserSessions 删除用户的所有会话
func (s *PostgresStorage) DeleteUserSessions(ctx context.Context, userID string) (int64, error) {
	if ctx == nil {
		ctx = s.ctx
	}

	query := `DELETE FROM user_sessions WHERE user_id = $1`
	result, err := s.pool.Exec(ctx, query, userID)
	if err != nil {
		return 0, fmt.Errorf("删除用户会话失败: %w", err)
	}
	return result.RowsAffected(), nil
}
