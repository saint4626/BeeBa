package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"beeba.org/internal/domain/jobs"
	searchdomain "beeba.org/internal/domain/search"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrAdminJobNotRetryable = errors.New("admin job is not retryable")

type JobRepository struct {
	db *pgxpool.Pool
}

func NewJobRepository(db *pgxpool.Pool) JobRepository {
	return JobRepository{db: db}
}

func (r JobRepository) Claim(ctx context.Context, queueName string, workerID string) (jobs.Job, error) {
	var job jobs.Job
	err := r.db.QueryRow(ctx, `
WITH picked AS (
  SELECT id
  FROM worker_jobs
  WHERE queue_name = $1
    AND status = 'pending'
    AND (next_retry_at IS NULL OR next_retry_at <= now())
  ORDER BY created_at
  FOR UPDATE SKIP LOCKED
  LIMIT 1
)
UPDATE worker_jobs job
SET
  status = 'running',
  attempt_count = job.attempt_count + 1,
  locked_by = $2,
  locked_at = now(),
  updated_at = now()
FROM picked
WHERE job.id = picked.id
RETURNING
  job.id::text,
  job.queue_name,
  job.job_type,
  job.payload,
  job.attempt_count,
  job.max_attempts,
  job.created_at`,
		queueName,
		workerID,
	).Scan(
		&job.ID,
		&job.QueueName,
		&job.JobType,
		&job.Payload,
		&job.AttemptCount,
		&job.MaxAttempts,
		&job.CreatedAt,
	)
	if err != nil {
		return jobs.Job{}, err
	}
	return job, nil
}

func (r JobRepository) ListAdminJobs(ctx context.Context, filter jobs.AdminListFilter) (jobs.AdminPage, error) {
	args := []any{}
	conditions := []string{"1 = 1"}

	if filter.QueueName != "" {
		args = append(args, filter.QueueName)
		conditions = append(conditions, fmt.Sprintf("queue_name = $%d", len(args)))
	}
	if filter.JobType != "" {
		args = append(args, filter.JobType)
		conditions = append(conditions, fmt.Sprintf("job_type = $%d", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
	}
	if !filter.Cursor.CreatedAt.IsZero() && filter.Cursor.ID != "" {
		args = append(args, filter.Cursor.CreatedAt, filter.Cursor.ID)
		conditions = append(conditions, fmt.Sprintf("(created_at, id) < ($%d::timestamptz, $%d::uuid)", len(args)-1, len(args)))
	}

	args = append(args, filter.Limit+1)
	limitArg := len(args)

	rows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT
  id::text,
  queue_name,
  job_type,
  payload,
  status,
  attempt_count,
  max_attempts,
  last_error,
  next_retry_at,
  locked_by,
  locked_at,
  created_at,
  updated_at
FROM worker_jobs
WHERE %s
ORDER BY created_at DESC, id DESC
LIMIT $%d`, joinConditions(conditions), limitArg), args...)
	if err != nil {
		return jobs.AdminPage{}, fmt.Errorf("query admin jobs: %w", err)
	}
	defer rows.Close()

	items := make([]jobs.AdminJob, 0, filter.Limit)
	for rows.Next() {
		var job jobs.AdminJob
		if err := rows.Scan(
			&job.ID,
			&job.QueueName,
			&job.JobType,
			&job.Payload,
			&job.Status,
			&job.AttemptCount,
			&job.MaxAttempts,
			&job.LastError,
			&job.NextRetryAt,
			&job.LockedBy,
			&job.LockedAt,
			&job.CreatedAt,
			&job.UpdatedAt,
		); err != nil {
			return jobs.AdminPage{}, fmt.Errorf("scan admin job: %w", err)
		}
		job = redactAdminJob(job)
		items = append(items, job)
	}
	if err := rows.Err(); err != nil {
		return jobs.AdminPage{}, fmt.Errorf("iterate admin jobs: %w", err)
	}

	page := jobs.AdminPage{Items: items}
	if len(items) > filter.Limit {
		page.Items = items[:filter.Limit]
		last := page.Items[len(page.Items)-1]
		page.NextCursor = jobs.AdminCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return page, nil
}

func (r JobRepository) RetryAdminJob(ctx context.Context, input jobs.AdminRetryInput) (jobs.AdminJob, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return jobs.AdminJob{}, fmt.Errorf("begin retry job: %w", err)
	}
	defer tx.Rollback(ctx)

	before, err := loadAdminJobForUpdate(ctx, tx, input.JobID)
	if err != nil {
		return jobs.AdminJob{}, err
	}
	if before.Status != "failed" && before.Status != "dead" {
		return jobs.AdminJob{}, ErrAdminJobNotRetryable
	}

	var after jobs.AdminJob
	err = tx.QueryRow(ctx, `
UPDATE worker_jobs
SET
  status = 'pending',
  attempt_count = 0,
  last_error = NULL,
  next_retry_at = now(),
  locked_by = NULL,
  locked_at = NULL,
  updated_at = now()
WHERE id = $1::uuid
RETURNING
  id::text,
  queue_name,
  job_type,
  payload,
  status,
  attempt_count,
  max_attempts,
  last_error,
  next_retry_at,
  locked_by,
  locked_at,
  created_at,
  updated_at`, input.JobID).Scan(
		&after.ID,
		&after.QueueName,
		&after.JobType,
		&after.Payload,
		&after.Status,
		&after.AttemptCount,
		&after.MaxAttempts,
		&after.LastError,
		&after.NextRetryAt,
		&after.LockedBy,
		&after.LockedAt,
		&after.CreatedAt,
		&after.UpdatedAt,
	)
	if err != nil {
		return jobs.AdminJob{}, fmt.Errorf("retry job update: %w", err)
	}

	beforeJSON, err := json.Marshal(redactAdminJob(before))
	if err != nil {
		return jobs.AdminJob{}, fmt.Errorf("marshal retry before: %w", err)
	}
	afterJSON, err := json.Marshal(map[string]any{
		"status":        after.Status,
		"attempt_count": after.AttemptCount,
		"reason":        input.Reason,
	})
	if err != nil {
		return jobs.AdminJob{}, fmt.Errorf("marshal retry after: %w", err)
	}

	if _, err := tx.Exec(ctx, `
INSERT INTO audit_logs (actor_user_id, action, entity_type, entity_id, before_json, after_json, ip_address, user_agent)
VALUES ($1::uuid, 'job.retry', 'worker_job', $2::uuid, $3::jsonb, $4::jsonb, NULLIF($5, '')::inet, NULLIF($6, ''))`,
		input.ActorUserID,
		input.JobID,
		string(beforeJSON),
		string(afterJSON),
		input.IPAddress,
		input.UserAgent,
	); err != nil {
		return jobs.AdminJob{}, fmt.Errorf("insert retry audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return jobs.AdminJob{}, fmt.Errorf("commit retry job: %w", err)
	}
	return redactAdminJob(after), nil
}

func loadAdminJobForUpdate(ctx context.Context, tx pgx.Tx, jobID string) (jobs.AdminJob, error) {
	var job jobs.AdminJob
	err := tx.QueryRow(ctx, `
SELECT
  id::text,
  queue_name,
  job_type,
  payload,
  status,
  attempt_count,
  max_attempts,
  last_error,
  next_retry_at,
  locked_by,
  locked_at,
  created_at,
  updated_at
FROM worker_jobs
WHERE id = $1::uuid
FOR UPDATE`, jobID).Scan(
		&job.ID,
		&job.QueueName,
		&job.JobType,
		&job.Payload,
		&job.Status,
		&job.AttemptCount,
		&job.MaxAttempts,
		&job.LastError,
		&job.NextRetryAt,
		&job.LockedBy,
		&job.LockedAt,
		&job.CreatedAt,
		&job.UpdatedAt,
	)
	if err != nil {
		return jobs.AdminJob{}, err
	}
	return job, nil
}

func redactAdminJob(job jobs.AdminJob) jobs.AdminJob {
	if job.QueueName != "email_queue" || len(job.Payload) == 0 {
		return job
	}
	var payload map[string]any
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return job
	}
	if _, ok := payload["token"]; ok {
		payload["token"] = "[redacted]"
	}
	redacted, err := json.Marshal(payload)
	if err != nil {
		return job
	}
	job.Payload = redacted
	return job
}

func (r JobRepository) GetContentFileForScan(ctx context.Context, fileID string) (jobs.ContentFileForScan, error) {
	var file jobs.ContentFileForScan
	err := r.db.QueryRow(ctx, `
SELECT
  id::text,
  content_id::text,
  bucket,
  storage_key,
  file_size,
  file_hash_sha256,
  scan_status,
  unlock_password_ciphertext
FROM content_files
WHERE id = $1::uuid`, fileID).Scan(
		&file.FileID,
		&file.ContentID,
		&file.Bucket,
		&file.StorageKey,
		&file.FileSize,
		&file.FileHashSHA256,
		&file.ScanStatus,
		&file.UnlockPasswordCiphertext,
	)
	if err != nil {
		return jobs.ContentFileForScan{}, err
	}
	return file, nil
}

func (r JobRepository) GetAutoPublishCandidate(ctx context.Context) (jobs.ContentFileForScan, error) {
	var file jobs.ContentFileForScan
	err := r.db.QueryRow(ctx, `
SELECT
  cf.id::text,
  cf.content_id::text,
  cf.bucket,
  cf.storage_key,
  cf.file_size,
  cf.file_hash_sha256,
  cf.scan_status,
  COALESCE(cf.unlock_password_ciphertext, '')
FROM content_items ci
JOIN content_files cf ON cf.content_id = ci.id
WHERE ci.status = 'pending_moderation'
  AND ci.deleted_at IS NULL
  AND cf.scan_status = 'clean'
ORDER BY ci.updated_at ASC, ci.id ASC
LIMIT 1`).Scan(
		&file.FileID,
		&file.ContentID,
		&file.Bucket,
		&file.StorageKey,
		&file.FileSize,
		&file.FileHashSHA256,
		&file.ScanStatus,
		&file.UnlockPasswordCiphertext,
	)
	if err != nil {
		return jobs.ContentFileForScan{}, err
	}
	return file, nil
}

func (r JobRepository) ListEmbeddedPreviewBackfillCandidates(ctx context.Context, limit int) ([]jobs.ContentFileForScan, error) {
	rows, err := r.db.Query(ctx, `
SELECT
  cf.id::text,
  cf.content_id::text,
  cf.bucket,
  cf.storage_key,
  cf.file_size,
  cf.file_hash_sha256,
  cf.scan_status,
  COALESCE(cf.unlock_password_ciphertext, '')
FROM content_items ci
JOIN content_files cf ON cf.content_id = ci.id
WHERE ci.status = 'published'
  AND ci.deleted_at IS NULL
  AND ci.hidden_at IS NULL
  AND cf.scan_status = 'clean'
  AND COALESCE(cf.unlock_password_ciphertext, '') <> ''
  AND NOT EXISTS (
    SELECT 1
    FROM content_images image
    WHERE image.content_id = ci.id
      AND image.deleted_at IS NULL
  )
ORDER BY ci.published_at DESC NULLS LAST, ci.updated_at DESC, ci.id DESC
LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("query embedded preview backfill candidates: %w", err)
	}
	defer rows.Close()

	candidates := make([]jobs.ContentFileForScan, 0)
	for rows.Next() {
		var file jobs.ContentFileForScan
		if err := rows.Scan(
			&file.FileID,
			&file.ContentID,
			&file.Bucket,
			&file.StorageKey,
			&file.FileSize,
			&file.FileHashSHA256,
			&file.ScanStatus,
			&file.UnlockPasswordCiphertext,
		); err != nil {
			return nil, fmt.Errorf("scan embedded preview backfill candidate: %w", err)
		}
		candidates = append(candidates, file)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read embedded preview backfill candidates: %w", err)
	}
	return candidates, nil
}

func (r JobRepository) CompleteFileScan(ctx context.Context, jobID string, fileID string, contentID string, scanStatus string, contentStatus string, result map[string]any) error {
	payload, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal scan result: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin complete scan transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
UPDATE content_files
SET scan_status = $2, scan_result = $3::jsonb, updated_at = now()
WHERE id = $1::uuid`, fileID, scanStatus, string(payload))
	if err != nil {
		return fmt.Errorf("update content file scan status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	tag, err = tx.Exec(ctx, `
UPDATE content_items
SET status = $2, updated_at = now()
WHERE id = $1::uuid`, contentID, contentStatus)
	if err != nil {
		return fmt.Errorf("update content scan status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	if err := r.markSucceededTx(ctx, tx, jobID); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit complete scan transaction: %w", err)
	}
	return nil
}

func (r JobRepository) PublishCleanContent(ctx context.Context, fileID string, contentID string, newBucket string, newStorageKey string) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin publish clean content transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := publishCleanContentTx(ctx, tx, fileID, contentID, newBucket, newStorageKey, "auto_publish_reconcile"); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit publish clean content transaction: %w", err)
	}
	return nil
}

func (r JobRepository) CompleteFileScanAndPublish(
	ctx context.Context,
	jobID string,
	fileID string,
	contentID string,
	scanStatus string,
	result map[string]any,
	newBucket string,
	newStorageKey string,
) error {
	payload, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal scan result: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin complete scan publish transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
UPDATE content_files
SET scan_status = $2, scan_result = $3::jsonb, bucket = $4, storage_key = $5, updated_at = now()
WHERE id = $1::uuid`, fileID, scanStatus, string(payload), newBucket, newStorageKey)
	if err != nil {
		return fmt.Errorf("update clean content file: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	if err := publishCleanContentTx(ctx, tx, fileID, contentID, newBucket, newStorageKey, "auto_publish"); err != nil {
		return err
	}

	if err := r.markSucceededTx(ctx, tx, jobID); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit complete scan publish transaction: %w", err)
	}
	return nil
}

func publishCleanContentTx(ctx context.Context, tx pgx.Tx, fileID string, contentID string, newBucket string, newStorageKey string, action string) error {
	tag, err := tx.Exec(ctx, `
UPDATE content_files
SET bucket = $2, storage_key = $3, updated_at = now()
WHERE id = $1::uuid
  AND content_id = $4::uuid
  AND scan_status = 'clean'`,
		fileID,
		newBucket,
		newStorageKey,
		contentID,
	)
	if err != nil {
		return fmt.Errorf("update published clean file storage: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	tag, err = tx.Exec(ctx, `
UPDATE content_items
SET status = 'published', published_at = COALESCE(published_at, now()), hidden_at = NULL, updated_at = now()
WHERE id = $1::uuid
  AND status IN ('pending_scan', 'pending_moderation')
  AND deleted_at IS NULL`, contentID)
	if err != nil {
		return fmt.Errorf("publish clean content: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	_, err = tx.Exec(ctx, `
INSERT INTO moderation_actions (actor_user_id, content_id, action, reason, before_json, after_json, ip_address, user_agent)
VALUES (NULL, $1::uuid, $3, 'clean scan and Basis validation', '{}'::jsonb, jsonb_build_object('status', 'published', 'file_id', $2::uuid), NULL, 'beeba-worker')`,
		contentID,
		fileID,
		action,
	)
	if err != nil {
		return fmt.Errorf("insert auto-publish moderation action: %w", err)
	}

	_, err = tx.Exec(ctx, `
INSERT INTO audit_logs (actor_user_id, action, entity_type, entity_id, before_json, after_json, ip_address, user_agent)
VALUES (
  NULL,
  $3,
  'content_item',
  $1::uuid,
  '{}'::jsonb,
  jsonb_build_object('status', 'published', 'file_id', $2::uuid, 'scan_status', 'clean'),
  NULL,
  'beeba-worker'
)`,
		contentID,
		fileID,
		"content."+action,
	)
	if err != nil {
		return fmt.Errorf("insert auto-publish audit log: %w", err)
	}

	if err := enqueueSearchIndexTx(ctx, tx, contentID, "upsert_content"); err != nil {
		return err
	}
	return nil
}

func (r JobRepository) EnqueueSearchIndex(ctx context.Context, contentID string, action string) error {
	payload, err := json.Marshal(jobs.SearchIndexPayload{
		ContentID: contentID,
		Action:    action,
	})
	if err != nil {
		return fmt.Errorf("marshal search index payload: %w", err)
	}
	_, err = r.db.Exec(ctx, `
INSERT INTO worker_jobs (queue_name, job_type, payload)
VALUES ('search_index_queue', 'sync_content_search', $1::jsonb)`, string(payload))
	if err != nil {
		return fmt.Errorf("insert search index job: %w", err)
	}
	return nil
}

func (r JobRepository) GetEmailVerificationTokenForSend(ctx context.Context, tokenID string) (jobs.EmailVerificationTokenForSend, error) {
	var token jobs.EmailVerificationTokenForSend
	err := r.db.QueryRow(ctx, `
SELECT id::text, user_id::text, email, expires_at, used_at
FROM email_verification_tokens
WHERE id = $1::uuid`, tokenID).Scan(
		&token.ID,
		&token.UserID,
		&token.Email,
		&token.ExpiresAt,
		&token.UsedAt,
	)
	if err != nil {
		return jobs.EmailVerificationTokenForSend{}, err
	}
	return token, nil
}

func (r JobRepository) GetPasswordChangeTokenForSend(ctx context.Context, tokenID string) (jobs.PasswordChangeTokenForSend, error) {
	var token jobs.PasswordChangeTokenForSend
	err := r.db.QueryRow(ctx, `
SELECT id::text, user_id::text, email, expires_at, used_at
FROM password_change_tokens
WHERE id = $1::uuid`, tokenID).Scan(
		&token.ID,
		&token.UserID,
		&token.Email,
		&token.ExpiresAt,
		&token.UsedAt,
	)
	if err != nil {
		return jobs.PasswordChangeTokenForSend{}, err
	}
	return token, nil
}

func (r JobRepository) GetEmailChangeTokenForSend(ctx context.Context, tokenID string) (jobs.EmailChangeTokenForSend, error) {
	var token jobs.EmailChangeTokenForSend
	err := r.db.QueryRow(ctx, `
SELECT id::text, user_id::text, new_email, expires_at, used_at
FROM email_change_tokens
WHERE id = $1::uuid`, tokenID).Scan(
		&token.ID,
		&token.UserID,
		&token.NewEmail,
		&token.ExpiresAt,
		&token.UsedAt,
	)
	if err != nil {
		return jobs.EmailChangeTokenForSend{}, err
	}
	return token, nil
}

func (r JobRepository) GetContentSearchDocument(ctx context.Context, contentID string) (searchdomain.ContentDocument, error) {
	var doc searchdomain.ContentDocument
	var tagsJSON string
	var tagNamesJSON string
	err := r.db.QueryRow(ctx, `
SELECT
  ci.id::text,
  ci.slug,
  ci.title,
  ci.description,
  c.slug,
  c.name,
  u.id::text,
  u.username,
  u.display_name,
  COALESCE(
    jsonb_agg(DISTINCT t.slug) FILTER (WHERE t.id IS NOT NULL),
    '[]'::jsonb
  )::text AS tags_json,
  COALESCE(
    jsonb_agg(DISTINCT t.name) FILTER (WHERE t.id IS NOT NULL),
    '[]'::jsonb
  )::text AS tag_names_json,
  ci.nsfw,
  primary_image.id::text,
  ci.likes_count,
  ci.downloads_count,
  ci.comments_count,
  ci.published_at,
  COALESCE(EXTRACT(EPOCH FROM ci.published_at)::bigint, 0),
  ci.updated_at
FROM content_items ci
JOIN categories c ON c.id = ci.category_id
JOIN users u ON u.id = ci.author_id
LEFT JOIN content_tags ct ON ct.content_id = ci.id
LEFT JOIN tags t ON t.id = ct.tag_id
LEFT JOIN LATERAL (
  SELECT id
  FROM content_images
  WHERE content_id = ci.id
    AND is_primary = true
    AND processing_status = 'processed'
    AND deleted_at IS NULL
  LIMIT 1
) primary_image ON true
WHERE ci.id = $1::uuid
  AND ci.status = 'published'
  AND ci.visibility = 'public'
  AND ci.deleted_at IS NULL
  AND ci.hidden_at IS NULL
  AND ci.published_at IS NOT NULL
GROUP BY ci.id, c.slug, c.name, u.id, u.username, u.display_name, primary_image.id`, contentID).Scan(
		&doc.ID,
		&doc.Slug,
		&doc.Title,
		&doc.Description,
		&doc.CategorySlug,
		&doc.CategoryName,
		&doc.AuthorID,
		&doc.AuthorUsername,
		&doc.AuthorName,
		&tagsJSON,
		&tagNamesJSON,
		&doc.NSFW,
		&doc.PreviewImageID,
		&doc.LikesCount,
		&doc.DownloadsCount,
		&doc.CommentsCount,
		&doc.PublishedAt,
		&doc.PublishedAtUnix,
		&doc.UpdatedAt,
	)
	if err != nil {
		return searchdomain.ContentDocument{}, err
	}
	if err := json.Unmarshal([]byte(tagsJSON), &doc.Tags); err != nil {
		return searchdomain.ContentDocument{}, fmt.Errorf("decode search tags: %w", err)
	}
	if err := json.Unmarshal([]byte(tagNamesJSON), &doc.TagNames); err != nil {
		return searchdomain.ContentDocument{}, fmt.Errorf("decode search tag names: %w", err)
	}
	return doc, nil
}

func (r JobRepository) MarkSucceeded(ctx context.Context, jobID string) error {
	tag, err := r.db.Exec(ctx, `
UPDATE worker_jobs
SET
  status = 'succeeded',
  payload = CASE
    WHEN queue_name = 'email_queue' THEN payload - 'token'
    ELSE payload
  END,
  locked_by = NULL,
  locked_at = NULL,
  updated_at = now()
WHERE id = $1::uuid`, jobID)
	if err != nil {
		return fmt.Errorf("mark job succeeded: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r JobRepository) MarkFailed(ctx context.Context, job jobs.Job, message string) error {
	status := "pending"
	var nextRetry any = time.Now().UTC().Add(backoff(job.AttemptCount))
	if job.AttemptCount >= job.MaxAttempts {
		status = "dead"
		nextRetry = nil
	}

	tag, err := r.db.Exec(ctx, `
UPDATE worker_jobs
SET
  status = $2,
  payload = CASE
    WHEN $2 = 'dead' AND queue_name = 'email_queue' THEN payload - 'token'
    ELSE payload
  END,
  last_error = $3,
  next_retry_at = $4,
  locked_by = NULL,
  locked_at = NULL,
  updated_at = now()
WHERE id = $1::uuid`,
		job.ID,
		status,
		message,
		nextRetry,
	)
	if err != nil {
		return fmt.Errorf("mark job failed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r JobRepository) markSucceededTx(ctx context.Context, tx pgx.Tx, jobID string) error {
	tag, err := tx.Exec(ctx, `
UPDATE worker_jobs
SET
  status = 'succeeded',
  payload = CASE
    WHEN queue_name = 'email_queue' THEN payload - 'token'
    ELSE payload
  END,
  locked_by = NULL,
  locked_at = NULL,
  updated_at = now()
WHERE id = $1::uuid`, jobID)
	if err != nil {
		return fmt.Errorf("mark job succeeded: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 6 {
		attempt = 6
	}
	return time.Duration(1<<uint(attempt-1)) * time.Minute
}
