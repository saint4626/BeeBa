package postgres

import (
	"context"
	"fmt"

	"beeba.org/internal/domain/content"

	"github.com/jackc/pgx/v5"
)

type storageUsageQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func ownerStorageUsage(ctx context.Context, querier storageUsageQuerier, ownerID string, limitBytes int64) (content.OwnerStorageUsage, error) {
	var usedBytes int64
	if err := querier.QueryRow(ctx, `
SELECT
  COALESCE((
    SELECT sum(cf.file_size)
    FROM content_files cf
    WHERE cf.uploaded_by = $1::uuid
  ), 0)
  + COALESCE((
    SELECT sum(ci.file_size)
    FROM content_images ci
    JOIN content_items item ON item.id = ci.content_id
    WHERE (ci.uploaded_by = $1::uuid OR item.author_id = $1::uuid)
      AND ci.deleted_at IS NULL
  ), 0)
  + COALESCE((
    SELECT sum(ui.file_size)
    FROM user_images ui
    WHERE ui.user_id = $1::uuid
      AND ui.deleted_at IS NULL
  ), 0)`, ownerID).Scan(&usedBytes); err != nil {
		return content.OwnerStorageUsage{}, fmt.Errorf("query owner storage usage: %w", err)
	}

	return content.OwnerStorageUsage{
		UsedBytes:  usedBytes,
		LimitBytes: limitBytes,
	}, nil
}

func lockOwnerStorageQuota(ctx context.Context, tx pgx.Tx, ownerID string) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1), hashtext('storage_quota'))`, ownerID); err != nil {
		return fmt.Errorf("lock owner storage quota: %w", err)
	}
	return nil
}

func ensureOwnerStorageQuota(ctx context.Context, tx pgx.Tx, ownerID string, incomingDeltaBytes int64, limitBytes int64) error {
	if incomingDeltaBytes <= 0 {
		return nil
	}
	if err := lockOwnerStorageQuota(ctx, tx, ownerID); err != nil {
		return err
	}
	usage, err := ownerStorageUsage(ctx, tx, ownerID, limitBytes)
	if err != nil {
		return err
	}
	if usage.LimitBytes > 0 && incomingDeltaBytes > usage.LimitBytes-usage.UsedBytes {
		return content.ErrStorageQuotaExceeded
	}
	return nil
}
