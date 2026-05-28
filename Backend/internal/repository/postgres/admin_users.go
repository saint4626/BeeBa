package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"beeba.org/internal/domain/users"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrAdminUserProtected = fmt.Errorf("admin user is protected")

var allowedAdminRoles = map[string]struct{}{
	"user":      {},
	"moderator": {},
	"admin":     {},
	"owner":     {},
}

func (r UserRepository) ListAdminUsers(ctx context.Context, filter users.AdminListFilter) ([]users.AdminUser, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	args := []any{limit}
	conditions := []string{"u.deleted_at IS NULL"}
	if filter.Query != "" {
		args = append(args, "%"+strings.ToLower(filter.Query)+"%")
		conditions = append(conditions, fmt.Sprintf("(lower(u.email) LIKE $%d OR lower(u.username) LIKE $%d OR lower(COALESCE(u.display_name, '')) LIKE $%d)", len(args), len(args), len(args)))
	}
	if filter.Role != "" {
		args = append(args, filter.Role)
		conditions = append(conditions, fmt.Sprintf(`EXISTS (
  SELECT 1
  FROM user_roles filter_ur
  JOIN roles filter_r ON filter_r.id = filter_ur.role_id
  WHERE filter_ur.user_id = u.id AND filter_r.slug = $%d
)`, len(args)))
	}

	rows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT
  u.id::text,
  u.email,
  u.username,
  u.display_name,
  u.avatar_image_id::text,
  u.email_verified_at,
  COALESCE(array_agg(DISTINCT r.slug ORDER BY r.slug) FILTER (WHERE r.slug IS NOT NULL), '{}') AS roles,
  u.banned_at,
  u.deleted_at,
  COUNT(DISTINCT ci.id)::int AS content_count,
  u.created_at,
  u.updated_at
FROM users u
LEFT JOIN user_roles ur ON ur.user_id = u.id
LEFT JOIN roles r ON r.id = ur.role_id
LEFT JOIN content_items ci ON ci.author_id = u.id AND ci.deleted_at IS NULL
WHERE %s
GROUP BY u.id
ORDER BY u.created_at DESC, u.id DESC
LIMIT $1`, joinConditions(conditions)), args...)
	if err != nil {
		return nil, fmt.Errorf("query admin users: %w", err)
	}
	defer rows.Close()

	items := make([]users.AdminUser, 0, limit)
	for rows.Next() {
		var item users.AdminUser
		err := rows.Scan(
			&item.ID,
			&item.Email,
			&item.Username,
			&item.DisplayName,
			&item.AvatarImageID,
			&item.EmailVerifiedAt,
			&item.Roles,
			&item.BannedAt,
			&item.DeletedAt,
			&item.ContentCount,
			&item.CreatedAt,
			&item.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan admin user: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read admin users: %w", err)
	}
	return items, nil
}

func (r UserRepository) BanUser(ctx context.Context, input users.AdminUserActionInput) (users.AdminUser, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return users.AdminUser{}, fmt.Errorf("begin ban user: %w", err)
	}
	defer tx.Rollback(ctx)

	before, err := loadAdminUserForUpdate(ctx, tx, input.TargetUserID)
	if err != nil {
		return users.AdminUser{}, err
	}
	if containsRole(before.Roles, "owner") {
		return users.AdminUser{}, ErrAdminUserProtected
	}

	var bannedAt pgtype.Timestamptz
	err = tx.QueryRow(ctx, `
UPDATE users
SET banned_at = COALESCE(banned_at, now()), updated_at = now()
WHERE id = $1::uuid
  AND deleted_at IS NULL
RETURNING banned_at`, input.TargetUserID).Scan(&bannedAt)
	if err != nil {
		return users.AdminUser{}, fmt.Errorf("ban user: %w", err)
	}

	if _, err := tx.Exec(ctx, `
UPDATE auth_sessions
SET revoked_at = now(), updated_at = now()
WHERE user_id = $1::uuid
  AND revoked_at IS NULL`, input.TargetUserID); err != nil {
		return users.AdminUser{}, fmt.Errorf("revoke banned user sessions: %w", err)
	}

	if err := insertUserAudit(ctx, tx, input.ActorUserID, input.TargetUserID, "user.ban", before, map[string]any{
		"banned_at": nullablePgTime(bannedAt),
		"reason":    input.Reason,
	}, input.IPAddress, input.UserAgent); err != nil {
		return users.AdminUser{}, err
	}

	updated, err := loadAdminUser(ctx, tx, input.TargetUserID)
	if err != nil {
		return users.AdminUser{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return users.AdminUser{}, fmt.Errorf("commit ban user: %w", err)
	}
	return updated, nil
}

func (r UserRepository) UnbanUser(ctx context.Context, input users.AdminUserActionInput) (users.AdminUser, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return users.AdminUser{}, fmt.Errorf("begin unban user: %w", err)
	}
	defer tx.Rollback(ctx)

	before, err := loadAdminUserForUpdate(ctx, tx, input.TargetUserID)
	if err != nil {
		return users.AdminUser{}, err
	}

	if _, err := tx.Exec(ctx, `
UPDATE users
SET banned_at = NULL, updated_at = now()
WHERE id = $1::uuid
  AND deleted_at IS NULL`, input.TargetUserID); err != nil {
		return users.AdminUser{}, fmt.Errorf("unban user: %w", err)
	}

	if err := insertUserAudit(ctx, tx, input.ActorUserID, input.TargetUserID, "user.unban", before, map[string]any{
		"banned_at": nil,
		"reason":    input.Reason,
	}, input.IPAddress, input.UserAgent); err != nil {
		return users.AdminUser{}, err
	}

	updated, err := loadAdminUser(ctx, tx, input.TargetUserID)
	if err != nil {
		return users.AdminUser{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return users.AdminUser{}, fmt.Errorf("commit unban user: %w", err)
	}
	return updated, nil
}

func (r UserRepository) SetUserRoles(ctx context.Context, input users.AdminUserRolesInput) (users.AdminUser, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return users.AdminUser{}, fmt.Errorf("begin set user roles: %w", err)
	}
	defer tx.Rollback(ctx)

	before, err := loadAdminUserForUpdate(ctx, tx, input.TargetUserID)
	if err != nil {
		return users.AdminUser{}, err
	}
	if (containsRole(before.Roles, "owner") || containsRole(input.Roles, "owner")) && !input.AllowOwnerRole {
		return users.AdminUser{}, ErrAdminUserProtected
	}
	if containsRole(before.Roles, "owner") && !containsRole(input.Roles, "owner") {
		var ownerCount int
		if err := tx.QueryRow(ctx, `
SELECT COUNT(*)::int
FROM users u
JOIN user_roles ur ON ur.user_id = u.id
JOIN roles r ON r.id = ur.role_id
WHERE r.slug = 'owner'
  AND u.deleted_at IS NULL
  AND u.banned_at IS NULL`).Scan(&ownerCount); err != nil {
			return users.AdminUser{}, fmt.Errorf("count owners: %w", err)
		}
		if ownerCount <= 1 {
			return users.AdminUser{}, ErrAdminUserProtected
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM user_roles WHERE user_id = $1::uuid`, input.TargetUserID); err != nil {
		return users.AdminUser{}, fmt.Errorf("delete user roles: %w", err)
	}
	for _, role := range input.Roles {
		if _, ok := allowedAdminRoles[role]; !ok {
			return users.AdminUser{}, pgx.ErrNoRows
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO user_roles (user_id, role_id, granted_by)
SELECT $1::uuid, id, $2::uuid
FROM roles
WHERE slug = $3`, input.TargetUserID, input.ActorUserID, role); err != nil {
			return users.AdminUser{}, fmt.Errorf("insert user role %q: %w", role, err)
		}
	}

	if err := insertUserAudit(ctx, tx, input.ActorUserID, input.TargetUserID, "user.roles.update", before, map[string]any{
		"roles": input.Roles,
	}, input.IPAddress, input.UserAgent); err != nil {
		return users.AdminUser{}, err
	}

	updated, err := loadAdminUser(ctx, tx, input.TargetUserID)
	if err != nil {
		return users.AdminUser{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return users.AdminUser{}, fmt.Errorf("commit set user roles: %w", err)
	}
	return updated, nil
}

func loadAdminUserForUpdate(ctx context.Context, tx pgx.Tx, userID string) (users.AdminUser, error) {
	var lockedID string
	if err := tx.QueryRow(ctx, `SELECT id::text FROM users WHERE id = $1::uuid AND deleted_at IS NULL FOR UPDATE`, userID).Scan(&lockedID); err != nil {
		return users.AdminUser{}, err
	}
	return loadAdminUser(ctx, tx, userID)
}

func loadAdminUser(ctx context.Context, tx pgx.Tx, userID string) (users.AdminUser, error) {
	var item users.AdminUser
	err := tx.QueryRow(ctx, `
SELECT
  u.id::text,
  u.email,
  u.username,
  u.display_name,
  u.avatar_image_id::text,
  u.email_verified_at,
  COALESCE(array_agg(DISTINCT r.slug ORDER BY r.slug) FILTER (WHERE r.slug IS NOT NULL), '{}') AS roles,
  u.banned_at,
  u.deleted_at,
  COUNT(DISTINCT ci.id)::int AS content_count,
  u.created_at,
  u.updated_at
FROM users u
LEFT JOIN user_roles ur ON ur.user_id = u.id
LEFT JOIN roles r ON r.id = ur.role_id
LEFT JOIN content_items ci ON ci.author_id = u.id AND ci.deleted_at IS NULL
WHERE u.id = $1::uuid
GROUP BY u.id
LIMIT 1`, userID).Scan(
		&item.ID,
		&item.Email,
		&item.Username,
		&item.DisplayName,
		&item.AvatarImageID,
		&item.EmailVerifiedAt,
		&item.Roles,
		&item.BannedAt,
		&item.DeletedAt,
		&item.ContentCount,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return users.AdminUser{}, err
	}
	return item, nil
}

func insertUserAudit(ctx context.Context, tx pgx.Tx, actorUserID string, targetUserID string, action string, before users.AdminUser, after map[string]any, ipAddress string, userAgent string) error {
	afterJSON, err := json.Marshal(after)
	if err != nil {
		return fmt.Errorf("encode %s audit json: %w", action, err)
	}
	_, err = tx.Exec(ctx, `
INSERT INTO audit_logs (actor_user_id, action, entity_type, entity_id, before_json, after_json, ip_address, user_agent)
VALUES (
  $1::uuid,
  $2,
  'user',
  $3::uuid,
  jsonb_build_object('roles', $4::text[], 'banned_at', $5::text),
  $6::jsonb,
  NULLIF($7, '')::inet,
  $8
)`, actorUserID, action, targetUserID, before.Roles, timePtrString(before.BannedAt), string(afterJSON), ipAddress, userAgent)
	if err != nil {
		return fmt.Errorf("insert %s audit log: %w", action, err)
	}
	return nil
}

func containsRole(roles []string, role string) bool {
	for _, current := range roles {
		if current == role {
			return true
		}
	}
	return false
}

func timePtrString(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.Format(timeFormatRFC3339Micro)
}

func nullablePgTime(value pgtype.Timestamptz) any {
	if !value.Valid {
		return nil
	}
	return value.Time.Format(timeFormatRFC3339Micro)
}
