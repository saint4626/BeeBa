package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	authdomain "beeba.org/internal/domain/auth"
	"beeba.org/internal/domain/users"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrUserConflict = errors.New("user email or username already exists")

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) UserRepository {
	return UserRepository{db: db}
}

func (r UserRepository) Create(ctx context.Context, input users.CreateInput) (users.PublicUser, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return users.PublicUser{}, fmt.Errorf("begin create user: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	var user users.PublicUser
	err = tx.QueryRow(ctx, `
INSERT INTO users (email, username, display_name, password_hash)
VALUES ($1, $2, $3, $4)
RETURNING id::text, email, username, display_name, avatar_image_id::text, email_verified_at, created_at, updated_at`,
		input.Email,
		input.Username,
		input.DisplayName,
		input.PasswordHash,
	).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&user.DisplayName,
		&user.AvatarImageID,
		&user.EmailVerifiedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return users.PublicUser{}, ErrUserConflict
		}
		return users.PublicUser{}, fmt.Errorf("insert user: %w", err)
	}

	if _, err = tx.Exec(ctx, `
INSERT INTO user_roles (user_id, role_id)
SELECT $1::uuid, id
FROM roles
WHERE slug = 'user'`, user.ID); err != nil {
		return users.PublicUser{}, fmt.Errorf("assign user role: %w", err)
	}

	if input.EmailVerificationTokenHash != "" {
		if err = insertEmailVerificationToken(ctx, tx, user.ID, user.Email, input.EmailVerificationTokenHash, input.EmailVerificationTokenExpiresAt); err != nil {
			return users.PublicUser{}, err
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return users.PublicUser{}, fmt.Errorf("commit create user: %w", err)
	}

	return user, nil
}

func (r UserRepository) VerifyEmail(ctx context.Context, tokenHash string, ipAddress string, userAgent string) (users.EmailVerificationResult, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return users.EmailVerificationResult{}, fmt.Errorf("begin email verify: %w", err)
	}
	defer tx.Rollback(ctx)

	var tokenID string
	var user users.PublicUser
	err = tx.QueryRow(ctx, `
SELECT
  evt.id::text,
  u.id::text,
  u.email,
  u.username,
  u.display_name,
  avatar.id::text,
  u.email_verified_at,
  u.created_at,
  u.updated_at
FROM email_verification_tokens evt
JOIN users u ON u.id = evt.user_id
LEFT JOIN user_images avatar ON avatar.id = u.avatar_image_id AND avatar.processing_status = 'processed'
WHERE evt.token_hash = $1
  AND evt.used_at IS NULL
  AND evt.expires_at > now()
  AND u.deleted_at IS NULL
  AND u.banned_at IS NULL
FOR UPDATE OF evt, u`, tokenHash).Scan(
		&tokenID,
		&user.ID,
		&user.Email,
		&user.Username,
		&user.DisplayName,
		&user.AvatarImageID,
		&user.EmailVerifiedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return users.EmailVerificationResult{}, pgx.ErrNoRows
	}
	if err != nil {
		return users.EmailVerificationResult{}, fmt.Errorf("query email verification token: %w", err)
	}

	alreadyVerified := user.EmailVerifiedAt != nil
	err = tx.QueryRow(ctx, `
WITH updated AS (
  UPDATE users
  SET email_verified_at = COALESCE(email_verified_at, now()), updated_at = now()
  WHERE id = $1::uuid
  RETURNING *
)
SELECT updated.id::text, updated.email, updated.username, updated.display_name, avatar.id::text, updated.email_verified_at, updated.created_at, updated.updated_at
FROM updated
LEFT JOIN user_images avatar ON avatar.id = updated.avatar_image_id AND avatar.processing_status = 'processed'`, user.ID).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&user.DisplayName,
		&user.AvatarImageID,
		&user.EmailVerifiedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return users.EmailVerificationResult{}, fmt.Errorf("update email verified: %w", err)
	}

	if _, err := tx.Exec(ctx, `
UPDATE email_verification_tokens
SET used_at = now()
WHERE id = $1::uuid`, tokenID); err != nil {
		return users.EmailVerificationResult{}, fmt.Errorf("mark email verification used: %w", err)
	}

	if err := insertUserEmailAudit(ctx, tx, user.ID, "user.email.verify", map[string]any{
		"email_verified_at": nil,
	}, map[string]any{
		"email_verified_at": user.EmailVerifiedAt,
		"already_verified":  alreadyVerified,
	}, ipAddress, userAgent); err != nil {
		return users.EmailVerificationResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return users.EmailVerificationResult{}, fmt.Errorf("commit email verify: %w", err)
	}
	return users.EmailVerificationResult{User: user, AlreadyVerified: alreadyVerified}, nil
}

func (r UserRepository) RequestEmailVerification(ctx context.Context, input users.EmailVerificationRequestInput) (users.EmailVerificationResult, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return users.EmailVerificationResult{}, fmt.Errorf("begin email verification request: %w", err)
	}
	defer tx.Rollback(ctx)

	var user users.PublicUser
	err = tx.QueryRow(ctx, `
SELECT u.id::text, u.email, u.username, u.display_name, avatar.id::text, u.email_verified_at, u.created_at, u.updated_at
FROM users u
LEFT JOIN user_images avatar ON avatar.id = u.avatar_image_id AND avatar.processing_status = 'processed'
WHERE u.id = $1::uuid
  AND u.deleted_at IS NULL
  AND u.banned_at IS NULL
FOR UPDATE OF u`, input.UserID).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&user.DisplayName,
		&user.AvatarImageID,
		&user.EmailVerifiedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return users.EmailVerificationResult{}, pgx.ErrNoRows
	}
	if err != nil {
		return users.EmailVerificationResult{}, fmt.Errorf("query user for email verification request: %w", err)
	}
	if user.EmailVerifiedAt == nil {
		if _, err := tx.Exec(ctx, `
UPDATE email_verification_tokens
SET used_at = now()
WHERE user_id = $1::uuid
  AND used_at IS NULL`, user.ID); err != nil {
			return users.EmailVerificationResult{}, fmt.Errorf("expire old email verification tokens: %w", err)
		}
		if err := insertEmailVerificationToken(ctx, tx, user.ID, user.Email, input.TokenHash, input.ExpiresAt); err != nil {
			return users.EmailVerificationResult{}, err
		}
	}

	if err := insertUserEmailAudit(ctx, tx, user.ID, "user.email.verification.request", map[string]any{}, map[string]any{
		"already_verified": user.EmailVerifiedAt != nil,
	}, input.IPAddress, input.UserAgent); err != nil {
		return users.EmailVerificationResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return users.EmailVerificationResult{}, fmt.Errorf("commit email verification request: %w", err)
	}
	return users.EmailVerificationResult{User: user, AlreadyVerified: user.EmailVerifiedAt != nil}, nil
}

func (r UserRepository) FindByEmailForLogin(ctx context.Context, email string) (authdomain.UserWithPassword, error) {
	var result authdomain.UserWithPassword
	err := r.db.QueryRow(ctx, `
SELECT u.id::text, u.email, u.username, u.display_name, avatar.id::text, u.email_verified_at, u.created_at, u.updated_at, u.password_hash, u.banned_at, u.deleted_at
FROM users u
LEFT JOIN user_images avatar ON avatar.id = u.avatar_image_id AND avatar.processing_status = 'processed'
WHERE u.email = $1
LIMIT 1`, email).Scan(
		&result.User.ID,
		&result.User.Email,
		&result.User.Username,
		&result.User.DisplayName,
		&result.User.AvatarImageID,
		&result.User.EmailVerifiedAt,
		&result.User.CreatedAt,
		&result.User.UpdatedAt,
		&result.PasswordHash,
		&result.BannedAt,
		&result.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return authdomain.UserWithPassword{}, pgx.ErrNoRows
	}
	if err != nil {
		return authdomain.UserWithPassword{}, fmt.Errorf("query user by email: %w", err)
	}
	return result, nil
}

func (r UserRepository) GetPublicProfile(ctx context.Context, username string) (users.PublicProfile, error) {
	var profile users.PublicProfile
	err := r.db.QueryRow(ctx, `
SELECT
  u.id::text,
  u.username,
  u.display_name,
  avatar.id::text,
  COUNT(ci.id)::int AS published_count,
  COALESCE(SUM(ci.likes_count), 0)::int AS likes_count,
  COALESCE(SUM(ci.downloads_count), 0)::int AS downloads_count,
  u.created_at,
  u.updated_at
FROM users u
LEFT JOIN user_images avatar ON avatar.id = u.avatar_image_id AND avatar.processing_status = 'processed'
LEFT JOIN content_items ci ON ci.author_id = u.id
  AND ci.status = 'published'
  AND ci.visibility = 'public'
  AND ci.deleted_at IS NULL
  AND ci.hidden_at IS NULL
  AND ci.published_at IS NOT NULL
WHERE lower(u.username) = lower($1)
  AND u.deleted_at IS NULL
  AND u.banned_at IS NULL
GROUP BY u.id, avatar.id
LIMIT 1`, username).Scan(
		&profile.ID,
		&profile.Username,
		&profile.DisplayName,
		&profile.AvatarImageID,
		&profile.PublishedCount,
		&profile.LikesCount,
		&profile.DownloadsCount,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return users.PublicProfile{}, pgx.ErrNoRows
	}
	if err != nil {
		return users.PublicProfile{}, fmt.Errorf("query public profile: %w", err)
	}
	return profile, nil
}

func (r UserRepository) UpdateProfile(ctx context.Context, userID string, input users.ProfileUpdateInput) (users.PublicUser, error) {
	var user users.PublicUser
	err := r.db.QueryRow(ctx, `
WITH updated AS (
  UPDATE users
  SET
    display_name = $2,
    updated_at = now()
  WHERE id = $1::uuid
    AND deleted_at IS NULL
    AND banned_at IS NULL
  RETURNING *
)
SELECT updated.id::text, updated.email, updated.username, updated.display_name, avatar.id::text, updated.email_verified_at, updated.created_at, updated.updated_at
FROM updated
LEFT JOIN user_images avatar ON avatar.id = updated.avatar_image_id AND avatar.processing_status = 'processed'`,
		userID,
		input.DisplayName,
	).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&user.DisplayName,
		&user.AvatarImageID,
		&user.EmailVerifiedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return users.PublicUser{}, pgx.ErrNoRows
	}
	if err != nil {
		return users.PublicUser{}, fmt.Errorf("update user profile: %w", err)
	}
	return user, nil
}

func (r UserRepository) GetPasswordHash(ctx context.Context, userID string) (string, error) {
	var hash string
	err := r.db.QueryRow(ctx, `
SELECT password_hash
FROM users
WHERE id = $1::uuid
  AND deleted_at IS NULL
  AND banned_at IS NULL
LIMIT 1`, userID).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", pgx.ErrNoRows
	}
	if err != nil {
		return "", fmt.Errorf("query password hash: %w", err)
	}
	return hash, nil
}

func (r UserRepository) ChangePassword(ctx context.Context, userID string, input users.PasswordChangeInput) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin password change: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
UPDATE users
SET password_hash = $2, updated_at = now()
WHERE id = $1::uuid
  AND deleted_at IS NULL
  AND banned_at IS NULL`, userID, input.NewPasswordHash)
	if err != nil {
		return fmt.Errorf("update password hash: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	if _, err := tx.Exec(ctx, `
UPDATE auth_sessions
SET revoked_at = now(), updated_at = now()
WHERE user_id = $1::uuid
  AND id <> $2::uuid
  AND revoked_at IS NULL`, userID, input.CurrentSessionID); err != nil {
		return fmt.Errorf("revoke other sessions after password change: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit password change: %w", err)
	}
	return nil
}

func (r UserRepository) CreateSession(ctx context.Context, input authdomain.SessionInput) error {
	_, err := r.db.Exec(ctx, `
INSERT INTO auth_sessions (
  user_id,
  access_token_hash,
  refresh_token_hash,
  user_agent,
  ip_address,
  access_expires_at,
  refresh_expires_at
)
VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, '')::inet, $6, $7)`,
		input.UserID,
		input.AccessTokenHash,
		input.RefreshTokenHash,
		input.UserAgent,
		input.IPAddress,
		input.AccessExpiresAt,
		input.RefreshExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("insert auth session: %w", err)
	}
	return nil
}

func (r UserRepository) FindByAccessTokenHash(ctx context.Context, tokenHash string, now time.Time) (authdomain.SessionWithUser, error) {
	return r.findSessionByTokenHash(ctx, "access_token_hash", tokenHash, now)
}

func (r UserRepository) FindByRefreshTokenHash(ctx context.Context, tokenHash string, now time.Time) (authdomain.SessionWithUser, error) {
	return r.findSessionByTokenHash(ctx, "refresh_token_hash", tokenHash, now)
}

func (r UserRepository) RotateSession(ctx context.Context, sessionID string, currentRefreshTokenHash string, input authdomain.SessionInput) error {
	tag, err := r.db.Exec(ctx, `
UPDATE auth_sessions
SET
  access_token_hash = $3,
  refresh_token_hash = $4,
  access_expires_at = $5,
  refresh_expires_at = $6,
  updated_at = now()
WHERE id = $1::uuid
  AND refresh_token_hash = $2
  AND revoked_at IS NULL
  AND refresh_expires_at > now()`,
		sessionID,
		currentRefreshTokenHash,
		input.AccessTokenHash,
		input.RefreshTokenHash,
		input.AccessExpiresAt,
		input.RefreshExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("rotate auth session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r UserRepository) RevokeByRefreshTokenHash(ctx context.Context, tokenHash string) error {
	tag, err := r.db.Exec(ctx, `
UPDATE auth_sessions
SET revoked_at = now(), updated_at = now()
WHERE refresh_token_hash = $1
  AND revoked_at IS NULL`, tokenHash)
	if err != nil {
		return fmt.Errorf("revoke auth session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func insertEmailVerificationToken(ctx context.Context, tx pgx.Tx, userID string, email string, tokenHash string, expiresAt time.Time) error {
	var tokenID string
	if err := tx.QueryRow(ctx, `
INSERT INTO email_verification_tokens (user_id, email, token_hash, expires_at)
VALUES ($1::uuid, $2, $3, $4)
RETURNING id::text`, userID, email, tokenHash, expiresAt).Scan(&tokenID); err != nil {
		return fmt.Errorf("insert email verification token: %w", err)
	}
	payload, err := json.Marshal(map[string]string{
		"user_id":  userID,
		"email":    email,
		"token_id": tokenID,
		"template": "email_verification",
	})
	if err != nil {
		return fmt.Errorf("marshal email verification job: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO worker_jobs (queue_name, job_type, payload)
VALUES ('email_queue', 'send_email_verification', $1::jsonb)`, string(payload)); err != nil {
		return fmt.Errorf("insert email verification job: %w", err)
	}
	return nil
}

func insertUserEmailAudit(ctx context.Context, tx pgx.Tx, userID string, action string, before any, after any, ipAddress string, userAgent string) error {
	beforeJSON, err := json.Marshal(before)
	if err != nil {
		return err
	}
	afterJSON, err := json.Marshal(after)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO audit_logs (actor_user_id, action, entity_type, entity_id, before_json, after_json, ip_address, user_agent)
VALUES ($1::uuid, $2, 'user', $1::uuid, $3::jsonb, $4::jsonb, NULLIF($5, '')::inet, NULLIF($6, ''))`,
		userID, action, string(beforeJSON), string(afterJSON), ipAddress, userAgent,
	); err != nil {
		return fmt.Errorf("insert user email audit: %w", err)
	}
	return nil
}

func (r UserRepository) findSessionByTokenHash(ctx context.Context, column string, tokenHash string, now time.Time) (authdomain.SessionWithUser, error) {
	if column != "access_token_hash" && column != "refresh_token_hash" {
		return authdomain.SessionWithUser{}, fmt.Errorf("invalid auth token column")
	}

	expiryColumn := "access_expires_at"
	if column == "refresh_token_hash" {
		expiryColumn = "refresh_expires_at"
	}

	sql := fmt.Sprintf(`
SELECT
  s.id::text,
  u.id::text,
  u.email,
  u.username,
  u.display_name,
  avatar.id::text,
  u.email_verified_at,
  u.created_at,
  u.updated_at,
  COALESCE(array_agg(r.slug ORDER BY r.slug) FILTER (WHERE r.slug IS NOT NULL), '{}') AS roles,
  u.banned_at,
  u.deleted_at
FROM auth_sessions s
JOIN users u ON u.id = s.user_id
LEFT JOIN user_images avatar ON avatar.id = u.avatar_image_id AND avatar.processing_status = 'processed'
LEFT JOIN user_roles ur ON ur.user_id = u.id
LEFT JOIN roles r ON r.id = ur.role_id
WHERE s.%s = $1
  AND s.revoked_at IS NULL
  AND s.%s > $2
GROUP BY s.id, u.id, avatar.id
LIMIT 1`, column, expiryColumn)

	var result authdomain.SessionWithUser
	err := r.db.QueryRow(ctx, sql, tokenHash, now).Scan(
		&result.SessionID,
		&result.User.ID,
		&result.User.Email,
		&result.User.Username,
		&result.User.DisplayName,
		&result.User.AvatarImageID,
		&result.User.EmailVerifiedAt,
		&result.User.CreatedAt,
		&result.User.UpdatedAt,
		&result.Roles,
		&result.BannedAt,
		&result.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return authdomain.SessionWithUser{}, pgx.ErrNoRows
	}
	if err != nil {
		return authdomain.SessionWithUser{}, fmt.Errorf("query auth session: %w", err)
	}
	return result, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
