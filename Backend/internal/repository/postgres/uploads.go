package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"beeba.org/internal/domain/content"
	"beeba.org/internal/domain/upload"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UploadRepository struct {
	db *pgxpool.Pool
}

func NewUploadRepository(db *pgxpool.Pool) UploadRepository {
	return UploadRepository{db: db}
}

func (r UploadRepository) OwnerStorageUsage(ctx context.Context, ownerID string, limitBytes int64) (content.OwnerStorageUsage, error) {
	return ownerStorageUsage(ctx, r.db, ownerID, limitBytes)
}

func (r UploadRepository) Create(ctx context.Context, input upload.CreateInput) (upload.Created, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return upload.Created{}, fmt.Errorf("begin upload transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var created upload.Created
	if err := ensureOwnerStorageQuota(ctx, tx, input.AuthorID, input.FileSize, input.StorageQuotaBytes); err != nil {
		return upload.Created{}, err
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1), hashtext($2))`, input.AuthorID, input.FileHashSHA256); err != nil {
		return upload.Created{}, fmt.Errorf("lock duplicate upload check: %w", err)
	}

	var existingContentID string
	err = tx.QueryRow(ctx, `
SELECT cf.content_id::text
FROM content_files cf
JOIN content_items ci ON ci.id = cf.content_id
WHERE cf.uploaded_by = $1::uuid
  AND cf.file_hash_sha256 = $2
  AND ci.status <> 'deleted'
LIMIT 1`,
		input.AuthorID,
		input.FileHashSHA256,
	).Scan(&existingContentID)
	if err == nil {
		return upload.Created{}, upload.ErrDuplicateFile
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return upload.Created{}, fmt.Errorf("check duplicate upload: %w", err)
	}

	err = tx.QueryRow(ctx, `
WITH selected_category AS (
  SELECT id FROM categories WHERE slug = $2 AND is_active = true
),
created_content AS (
  INSERT INTO content_items (
    author_id,
    category_id,
    title,
    slug,
    description,
    status,
    nsfw,
    visibility
  )
  SELECT $1::uuid, selected_category.id, $3, $4, $5, 'pending_scan', $6, $7
  FROM selected_category
  RETURNING id, status, visibility, created_at
)
SELECT id::text, status, visibility, created_at
FROM created_content`,
		input.AuthorID,
		input.CategorySlug,
		input.Title,
		input.Slug,
		input.Description,
		input.NSFW,
		input.Visibility,
	).Scan(&created.ContentID, &created.Status, &created.Visibility, &created.CreatedAt)
	if err != nil {
		return upload.Created{}, fmt.Errorf("insert content upload: %w", err)
	}

	err = tx.QueryRow(ctx, `
INSERT INTO content_files (
  content_id,
  bucket,
  storage_key,
  original_filename,
  safe_filename,
  file_size,
  file_hash_sha256,
  mime_type_detected,
  extension,
  scan_status,
  unlock_password_ciphertext,
  uploaded_by
)
VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), '.bee', 'pending', $9, $10::uuid)
RETURNING id::text, scan_status`,
		created.ContentID,
		input.Bucket,
		input.StorageKey,
		input.OriginalFilename,
		input.SafeFilename,
		input.FileSize,
		input.FileHashSHA256,
		input.MimeTypeDetected,
		input.UnlockPasswordCiphertext,
		input.AuthorID,
	).Scan(&created.FileID, &created.ScanStatus)
	if err != nil {
		return upload.Created{}, fmt.Errorf("insert content file: %w", err)
	}

	payload, err := json.Marshal(map[string]string{
		"content_id":  created.ContentID,
		"file_id":     created.FileID,
		"bucket":      input.Bucket,
		"storage_key": input.StorageKey,
	})
	if err != nil {
		return upload.Created{}, fmt.Errorf("marshal scan job payload: %w", err)
	}

	err = tx.QueryRow(ctx, `
INSERT INTO worker_jobs (queue_name, job_type, payload)
VALUES ('file_scan_queue', 'scan_content_file', $1::jsonb)
RETURNING id::text`, string(payload)).Scan(&created.JobID)
	if err != nil {
		return upload.Created{}, fmt.Errorf("insert scan worker job: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return upload.Created{}, fmt.Errorf("commit upload transaction: %w", err)
	}

	created.StorageBucket = input.Bucket
	created.StorageKey = input.StorageKey
	created.FileHashSHA256 = input.FileHashSHA256
	created.FileSize = input.FileSize
	created.OriginalFilename = input.OriginalFilename
	return created, nil
}
