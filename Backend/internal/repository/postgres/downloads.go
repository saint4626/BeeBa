package postgres

import (
	"context"
	"fmt"

	"beeba.org/internal/domain/downloads"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DownloadRepository struct {
	db *pgxpool.Pool
}

func NewDownloadRepository(db *pgxpool.Pool) DownloadRepository {
	return DownloadRepository{db: db}
}

func (r DownloadRepository) GetPublicTarget(ctx context.Context, contentID string, quarantineBucket string) (downloads.Target, error) {
	return r.getTarget(ctx, `
  AND ci.visibility = 'public'
  AND cf.bucket <> $2`, contentID, "", quarantineBucket)
}

func (r DownloadRepository) GetOwnerTarget(ctx context.Context, contentID string, userID string, quarantineBucket string) (downloads.Target, error) {
	return r.getTarget(ctx, `
  AND ci.author_id = $3::uuid
  AND cf.bucket <> $2`, contentID, userID, quarantineBucket)
}

func (r DownloadRepository) RecordEvent(ctx context.Context, input downloads.EventInput) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, `
INSERT INTO download_events (content_id, file_id, user_id, ip_address, user_agent)
VALUES ($1::uuid, $2::uuid, NULLIF($3, '')::uuid, NULLIF($4, '')::inet, NULLIF($5, ''))`,
		input.ContentID,
		input.FileID,
		stringValue(input.UserID),
		input.IPAddress,
		input.UserAgent,
	); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
UPDATE content_items
SET downloads_count = downloads_count + 1, updated_at = now()
WHERE id = $1::uuid`, input.ContentID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r DownloadRepository) getTarget(ctx context.Context, extraCondition string, contentID string, userID string, quarantineBucket string) (downloads.Target, error) {
	sql := fmt.Sprintf(`
SELECT
  ci.id::text,
  cf.id::text,
  ci.author_id::text,
  ci.visibility,
  cf.bucket,
  cf.storage_key,
  cf.safe_filename,
  cf.file_size,
  cf.unlock_password_ciphertext
FROM content_items ci
JOIN content_files cf ON cf.content_id = ci.id
WHERE ci.id = $1::uuid
  AND ci.status = 'published'
  AND ci.deleted_at IS NULL
  AND ci.hidden_at IS NULL
  AND cf.scan_status = 'clean'
%s
ORDER BY cf.created_at DESC
LIMIT 1`, extraCondition)

	args := []any{contentID, quarantineBucket}
	if userID != "" {
		args = append(args, userID)
	}

	var target downloads.Target
	err := r.db.QueryRow(ctx, sql, args...).Scan(
		&target.ContentID,
		&target.FileID,
		&target.AuthorID,
		&target.Visibility,
		&target.Bucket,
		&target.StorageKey,
		&target.Filename,
		&target.FileSize,
		&target.UnlockPasswordCiphertext,
	)
	if err != nil {
		return downloads.Target{}, err
	}
	return target, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
