package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"beeba.org/internal/domain/files"
	"beeba.org/internal/domain/jobs"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrAdminFileNotRescannable = errors.New("admin file is not rescannable")

type AdminFileRepository struct {
	db *pgxpool.Pool
}

func NewAdminFileRepository(db *pgxpool.Pool) AdminFileRepository {
	return AdminFileRepository{db: db}
}

func (r AdminFileRepository) ListAdminFiles(ctx context.Context, filter files.AdminListFilter) (files.AdminPage, error) {
	args := []any{}
	conditions := []string{"1 = 1"}

	if filter.Query != "" {
		args = append(args, "%"+strings.ToLower(filter.Query)+"%")
		conditions = append(conditions, fmt.Sprintf("(lower(cf.original_filename) LIKE $%d OR lower(cf.safe_filename) LIKE $%d OR lower(ci.title) LIKE $%d)", len(args), len(args), len(args)))
	}
	if filter.ScanStatus != "" {
		args = append(args, filter.ScanStatus)
		conditions = append(conditions, fmt.Sprintf("cf.scan_status = $%d", len(args)))
	}
	if filter.Bucket != "" {
		args = append(args, filter.Bucket)
		conditions = append(conditions, fmt.Sprintf("cf.bucket = $%d", len(args)))
	}
	if filter.ContentID != "" {
		args = append(args, filter.ContentID)
		conditions = append(conditions, fmt.Sprintf("cf.content_id = $%d::uuid", len(args)))
	}
	if filter.Hash != "" {
		args = append(args, filter.Hash)
		conditions = append(conditions, fmt.Sprintf("cf.file_hash_sha256 = $%d", len(args)))
	}
	if !filter.Cursor.CreatedAt.IsZero() && filter.Cursor.ID != "" {
		args = append(args, filter.Cursor.CreatedAt, filter.Cursor.ID)
		conditions = append(conditions, fmt.Sprintf("(cf.created_at, cf.id) < ($%d::timestamptz, $%d::uuid)", len(args)-1, len(args)))
	}

	args = append(args, filter.Limit+1)
	limitArg := len(args)

	rows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT
  cf.id::text,
  cf.content_id::text,
  ci.title,
  ci.status,
  ci.visibility,
  ci.author_id::text,
  author.username,
  cf.bucket,
  cf.original_filename,
  cf.safe_filename,
  cf.file_size,
  cf.file_hash_sha256,
  cf.mime_type_detected,
  cf.extension,
  cf.scan_status,
  cf.scan_result::text,
  cf.uploaded_by::text,
  uploader.username,
  cf.created_at,
  cf.updated_at
FROM content_files cf
JOIN content_items ci ON ci.id = cf.content_id
JOIN users author ON author.id = ci.author_id
JOIN users uploader ON uploader.id = cf.uploaded_by
WHERE %s
ORDER BY cf.created_at DESC, cf.id DESC
LIMIT $%d`, joinConditions(conditions), limitArg), args...)
	if err != nil {
		return files.AdminPage{}, fmt.Errorf("query admin files: %w", err)
	}
	defer rows.Close()

	items := make([]files.AdminFile, 0, filter.Limit)
	for rows.Next() {
		item, err := scanAdminFile(rows)
		if err != nil {
			return files.AdminPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return files.AdminPage{}, fmt.Errorf("iterate admin files: %w", err)
	}

	page := files.AdminPage{Items: items}
	if len(items) > filter.Limit {
		page.Items = items[:filter.Limit]
		last := page.Items[len(page.Items)-1]
		page.NextCursor = files.AdminCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return page, nil
}

func (r AdminFileRepository) RescanAdminFile(ctx context.Context, input files.AdminRescanInput) (files.AdminRescanResult, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return files.AdminRescanResult{}, fmt.Errorf("begin rescan file: %w", err)
	}
	defer tx.Rollback(ctx)

	before, storageKey, err := loadAdminFileForUpdate(ctx, tx, input.FileID)
	if err != nil {
		return files.AdminRescanResult{}, err
	}
	if before.ScanStatus != "failed" && before.ScanStatus != "suspicious" && before.ScanStatus != "infected" {
		return files.AdminRescanResult{}, ErrAdminFileNotRescannable
	}

	if _, err := tx.Exec(ctx, `
UPDATE content_files
SET scan_status = 'pending', scan_result = '{}'::jsonb, updated_at = now()
WHERE id = $1::uuid`, input.FileID); err != nil {
		return files.AdminRescanResult{}, fmt.Errorf("reset file scan: %w", err)
	}
	if _, err := tx.Exec(ctx, `
UPDATE content_items
SET status = 'pending_scan', published_at = NULL, hidden_at = NULL, updated_at = now()
WHERE id = $1::uuid`, before.ContentID); err != nil {
		return files.AdminRescanResult{}, fmt.Errorf("reset content scan: %w", err)
	}

	payload, err := json.Marshal(jobs.FileScanPayload{
		ContentID:  before.ContentID,
		FileID:     before.ID,
		Bucket:     before.Bucket,
		StorageKey: storageKey,
	})
	if err != nil {
		return files.AdminRescanResult{}, fmt.Errorf("marshal rescan payload: %w", err)
	}

	var jobID string
	if err := tx.QueryRow(ctx, `
INSERT INTO worker_jobs (queue_name, job_type, payload)
VALUES ('file_scan_queue', 'scan_content_file', $1::jsonb)
RETURNING id::text`, string(payload)).Scan(&jobID); err != nil {
		return files.AdminRescanResult{}, fmt.Errorf("insert rescan job: %w", err)
	}

	beforeJSON, err := json.Marshal(before)
	if err != nil {
		return files.AdminRescanResult{}, fmt.Errorf("marshal rescan before: %w", err)
	}
	afterJSON, err := json.Marshal(map[string]any{
		"scan_status": "pending",
		"content_id":  before.ContentID,
		"job_id":      jobID,
		"reason":      input.Reason,
	})
	if err != nil {
		return files.AdminRescanResult{}, fmt.Errorf("marshal rescan after: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO audit_logs (actor_user_id, action, entity_type, entity_id, before_json, after_json, ip_address, user_agent)
VALUES ($1::uuid, 'file.rescan', 'content_file', $2::uuid, $3::jsonb, $4::jsonb, NULLIF($5, '')::inet, NULLIF($6, ''))`,
		input.ActorUserID,
		input.FileID,
		string(beforeJSON),
		string(afterJSON),
		input.IPAddress,
		input.UserAgent,
	); err != nil {
		return files.AdminRescanResult{}, fmt.Errorf("insert rescan audit: %w", err)
	}

	after, _, err := loadAdminFileForUpdate(ctx, tx, input.FileID)
	if err != nil {
		return files.AdminRescanResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return files.AdminRescanResult{}, fmt.Errorf("commit rescan file: %w", err)
	}
	return files.AdminRescanResult{File: after, JobID: jobID}, nil
}

func loadAdminFileForUpdate(ctx context.Context, tx pgx.Tx, fileID string) (files.AdminFile, string, error) {
	row := tx.QueryRow(ctx, `
SELECT
  cf.id::text,
  cf.content_id::text,
  ci.title,
  ci.status,
  ci.visibility,
  ci.author_id::text,
  author.username,
  cf.bucket,
  cf.storage_key,
  cf.original_filename,
  cf.safe_filename,
  cf.file_size,
  cf.file_hash_sha256,
  cf.mime_type_detected,
  cf.extension,
  cf.scan_status,
  cf.scan_result::text,
  cf.uploaded_by::text,
  uploader.username,
  cf.created_at,
  cf.updated_at
FROM content_files cf
JOIN content_items ci ON ci.id = cf.content_id
JOIN users author ON author.id = ci.author_id
JOIN users uploader ON uploader.id = cf.uploaded_by
WHERE cf.id = $1::uuid
FOR UPDATE`, fileID)

	var item files.AdminFile
	var storageKey string
	var scanResult string
	err := row.Scan(
		&item.ID,
		&item.ContentID,
		&item.ContentTitle,
		&item.ContentStatus,
		&item.ContentVisibility,
		&item.AuthorID,
		&item.AuthorUsername,
		&item.Bucket,
		&storageKey,
		&item.OriginalFilename,
		&item.SafeFilename,
		&item.FileSize,
		&item.FileHashSHA256,
		&item.MimeTypeDetected,
		&item.Extension,
		&item.ScanStatus,
		&scanResult,
		&item.UploadedBy,
		&item.UploadedByUsername,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return files.AdminFile{}, "", err
	}
	item.ScanResult = json.RawMessage(scanResult)
	return item, storageKey, nil
}

type adminFileScanner interface {
	Scan(dest ...any) error
}

func scanAdminFile(row adminFileScanner) (files.AdminFile, error) {
	var item files.AdminFile
	var scanResult string
	if err := row.Scan(
		&item.ID,
		&item.ContentID,
		&item.ContentTitle,
		&item.ContentStatus,
		&item.ContentVisibility,
		&item.AuthorID,
		&item.AuthorUsername,
		&item.Bucket,
		&item.OriginalFilename,
		&item.SafeFilename,
		&item.FileSize,
		&item.FileHashSHA256,
		&item.MimeTypeDetected,
		&item.Extension,
		&item.ScanStatus,
		&scanResult,
		&item.UploadedBy,
		&item.UploadedByUsername,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return files.AdminFile{}, fmt.Errorf("scan admin file: %w", err)
	}
	item.ScanResult = json.RawMessage(scanResult)
	return item, nil
}
