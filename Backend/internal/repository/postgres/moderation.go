package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"beeba.org/internal/domain/jobs"
	"beeba.org/internal/domain/moderation"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ModerationRepository struct {
	db *pgxpool.Pool
}

func NewModerationRepository(db *pgxpool.Pool) ModerationRepository {
	return ModerationRepository{db: db}
}

func (r ModerationRepository) ListQueue(ctx context.Context, filter moderation.QueueFilter) ([]moderation.QueueItem, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	args := []any{limit}
	conditions := []string{
		"ci.deleted_at IS NULL",
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		conditions = append(conditions, fmt.Sprintf("ci.status = $%d", len(args)))
	} else {
		conditions = append(conditions, "ci.status IN ('pending_moderation', 'scan_failed')")
	}

	sql := fmt.Sprintf(`
SELECT
  ci.id::text,
  file.id::text,
  ci.title,
  ci.description,
  ci.status,
  ci.visibility,
  ci.nsfw,
  c.slug,
  c.name,
  u.id::text,
  u.username,
  u.display_name,
  file.scan_status,
  file.file_size,
  file.file_hash_sha256,
  file.original_filename,
  ci.created_at,
  ci.updated_at
FROM content_items ci
JOIN categories c ON c.id = ci.category_id
JOIN users u ON u.id = ci.author_id
LEFT JOIN LATERAL (
  SELECT id, scan_status, file_size, file_hash_sha256, original_filename
  FROM content_files
  WHERE content_id = ci.id
  ORDER BY created_at DESC
  LIMIT 1
) file ON true
WHERE %s
ORDER BY ci.updated_at DESC, ci.id DESC
LIMIT $1`, joinConditions(conditions))

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query moderation queue: %w", err)
	}
	defer rows.Close()

	items := make([]moderation.QueueItem, 0, limit)
	for rows.Next() {
		var item moderation.QueueItem
		err := rows.Scan(
			&item.ContentID,
			&item.FileID,
			&item.Title,
			&item.Description,
			&item.Status,
			&item.Visibility,
			&item.NSFW,
			&item.CategorySlug,
			&item.CategoryName,
			&item.AuthorID,
			&item.AuthorUsername,
			&item.AuthorDisplayName,
			&item.ScanStatus,
			&item.FileSize,
			&item.FileHashSHA256,
			&item.OriginalFilename,
			&item.CreatedAt,
			&item.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan moderation queue: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read moderation queue: %w", err)
	}
	return items, nil
}

func (r ModerationRepository) GetApprovalCandidate(ctx context.Context, contentID string) (moderation.ApprovalCandidate, error) {
	var candidate moderation.ApprovalCandidate
	err := r.db.QueryRow(ctx, `
SELECT
  ci.id::text,
  cf.id::text,
  ci.visibility,
  ci.status,
  cf.scan_status,
  cf.bucket,
  cf.storage_key
FROM content_items ci
JOIN content_files cf ON cf.content_id = ci.id
WHERE ci.id = $1::uuid
  AND ci.deleted_at IS NULL
ORDER BY cf.created_at DESC
LIMIT 1`, contentID).Scan(
		&candidate.ContentID,
		&candidate.FileID,
		&candidate.Visibility,
		&candidate.Status,
		&candidate.ScanStatus,
		&candidate.Bucket,
		&candidate.StorageKey,
	)
	if err != nil {
		return moderation.ApprovalCandidate{}, err
	}
	return candidate, nil
}

func joinConditions(conditions []string) string {
	result := ""
	for index, condition := range conditions {
		if index > 0 {
			result += " AND "
		}
		result += condition
	}
	return result
}

func (r ModerationRepository) Approve(ctx context.Context, input moderation.ApproveInput) (moderation.Approved, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return moderation.Approved{}, fmt.Errorf("begin approve transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
UPDATE content_files
SET bucket = $2, storage_key = $3, updated_at = now()
WHERE id = $1::uuid
  AND scan_status = 'clean'`,
		input.FileID,
		input.NewBucket,
		input.NewStorageKey,
	)
	if err != nil {
		return moderation.Approved{}, fmt.Errorf("update approved file storage: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return moderation.Approved{}, pgx.ErrNoRows
	}

	var approved moderation.Approved
	err = tx.QueryRow(ctx, `
UPDATE content_items
SET status = 'published', published_at = COALESCE(published_at, now()), updated_at = now()
WHERE id = $1::uuid
  AND status IN ('pending_moderation', 'approved')
RETURNING id::text, status, visibility, published_at`,
		input.ContentID,
	).Scan(&approved.ContentID, &approved.Status, &approved.Visibility, &approved.PublishedAt)
	if err != nil {
		return moderation.Approved{}, fmt.Errorf("publish approved content: %w", err)
	}

	_, err = tx.Exec(ctx, `
INSERT INTO moderation_actions (actor_user_id, content_id, action, reason, before_json, after_json, ip_address, user_agent)
VALUES ($1::uuid, $2::uuid, 'approve', '', '{}'::jsonb, jsonb_build_object('file_id', $3::uuid), NULLIF($4, '')::inet, $5)`,
		input.ActorUserID,
		input.ContentID,
		input.FileID,
		input.IPAddress,
		input.UserAgent,
	)
	if err != nil {
		return moderation.Approved{}, fmt.Errorf("insert moderation action: %w", err)
	}

	_, err = tx.Exec(ctx, `
INSERT INTO audit_logs (actor_user_id, action, entity_type, entity_id, before_json, after_json, ip_address, user_agent)
VALUES (
  $1::uuid,
  'content.approve',
  'content_item',
  $2::uuid,
  '{}'::jsonb,
  jsonb_build_object('status', 'published', 'file_id', $3::uuid),
  NULLIF($4, '')::inet,
  $5
)`,
		input.ActorUserID,
		input.ContentID,
		input.FileID,
		input.IPAddress,
		input.UserAgent,
	)
	if err != nil {
		return moderation.Approved{}, fmt.Errorf("insert approval audit log: %w", err)
	}

	if err := enqueueSearchIndexTx(ctx, tx, input.ContentID, "upsert_content"); err != nil {
		return moderation.Approved{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return moderation.Approved{}, fmt.Errorf("commit approve transaction: %w", err)
	}

	approved.FileID = input.FileID
	return approved, nil
}

func (r ModerationRepository) Reject(ctx context.Context, input moderation.ReviewInput) (moderation.Reviewed, error) {
	return r.reviewContent(ctx, reviewContentInput{
		ContentID:        input.ContentID,
		ActorUserID:      input.ActorUserID,
		Reason:           input.Reason,
		IPAddress:        input.IPAddress,
		UserAgent:        input.UserAgent,
		Action:           "reject",
		AuditAction:      "content.reject",
		NewStatus:        "rejected",
		AllowedStatuses:  []string{"pending_moderation", "approved", "scan_failed"},
		SetHiddenAt:      false,
		ClearPublishedAt: true,
		SearchAction:     "delete_content",
	})
}

func (r ModerationRepository) Hide(ctx context.Context, input moderation.ReviewInput) (moderation.Reviewed, error) {
	return r.reviewContent(ctx, reviewContentInput{
		ContentID:       input.ContentID,
		ActorUserID:     input.ActorUserID,
		Reason:          input.Reason,
		IPAddress:       input.IPAddress,
		UserAgent:       input.UserAgent,
		Action:          "hide",
		AuditAction:     "content.hide",
		NewStatus:       "hidden",
		AllowedStatuses: []string{"published", "approved"},
		SetHiddenAt:     true,
		SearchAction:    "delete_content",
	})
}

func (r ModerationRepository) Restore(ctx context.Context, input moderation.ReviewInput) (moderation.Reviewed, error) {
	return r.reviewContent(ctx, reviewContentInput{
		ContentID:        input.ContentID,
		ActorUserID:      input.ActorUserID,
		Reason:           input.Reason,
		IPAddress:        input.IPAddress,
		UserAgent:        input.UserAgent,
		Action:           "restore",
		AuditAction:      "content.restore",
		NewStatus:        "published",
		AllowedStatuses:  []string{"hidden"},
		SetHiddenAt:      false,
		SetPublishedAt:   true,
		RequireCleanScan: true,
		SearchAction:     "upsert_content",
	})
}

func (r ModerationRepository) ListCommentQueue(ctx context.Context, filter moderation.QueueFilter) ([]moderation.CommentQueueItem, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	args := []any{limit}
	conditions := []string{"cm.deleted_at IS NULL"}
	if filter.Status != "" {
		args = append(args, filter.Status)
		conditions = append(conditions, fmt.Sprintf("cm.status = $%d", len(args)))
	} else {
		conditions = append(conditions, "cm.status = 'visible'")
	}

	rows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT
  cm.id::text,
  cm.content_id::text,
  ci.title,
  cm.body,
  cm.status,
  author.id::text,
  author.username,
  author.display_name,
  content_author.id::text,
  content_author.username,
  content_author.display_name,
  cm.parent_id::text,
  cm.created_at,
  cm.updated_at
FROM comments cm
JOIN content_items ci ON ci.id = cm.content_id
JOIN users author ON author.id = cm.user_id
JOIN users content_author ON content_author.id = ci.author_id
WHERE %s
ORDER BY cm.updated_at DESC, cm.id DESC
LIMIT $1`, joinConditions(conditions)), args...)
	if err != nil {
		return nil, fmt.Errorf("query comment moderation queue: %w", err)
	}
	defer rows.Close()

	items := make([]moderation.CommentQueueItem, 0, limit)
	for rows.Next() {
		var item moderation.CommentQueueItem
		err := rows.Scan(
			&item.CommentID,
			&item.ContentID,
			&item.ContentTitle,
			&item.Body,
			&item.Status,
			&item.AuthorID,
			&item.AuthorUsername,
			&item.AuthorDisplayName,
			&item.ContentAuthorID,
			&item.ContentAuthorUsername,
			&item.ContentAuthorName,
			&item.ParentID,
			&item.CreatedAt,
			&item.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan comment moderation queue: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read comment moderation queue: %w", err)
	}
	return items, nil
}

func (r ModerationRepository) ApproveComment(ctx context.Context, input moderation.CommentReviewInput) (moderation.CommentReviewed, error) {
	return r.reviewComment(ctx, commentReviewInput{
		CommentID:       input.CommentID,
		ActorUserID:     input.ActorUserID,
		Reason:          "",
		IPAddress:       input.IPAddress,
		UserAgent:       input.UserAgent,
		Action:          "approve_comment",
		AuditAction:     "comment.approve",
		NewStatus:       "visible",
		AllowedStatuses: []string{"pending_moderation"},
	})
}

func (r ModerationRepository) HideComment(ctx context.Context, input moderation.CommentReviewInput) (moderation.CommentReviewed, error) {
	return r.reviewComment(ctx, commentReviewInput{
		CommentID:       input.CommentID,
		ActorUserID:     input.ActorUserID,
		Reason:          input.Reason,
		IPAddress:       input.IPAddress,
		UserAgent:       input.UserAgent,
		Action:          "hide_comment",
		AuditAction:     "comment.hide",
		NewStatus:       "hidden",
		AllowedStatuses: []string{"pending_moderation", "visible"},
	})
}

type commentReviewInput struct {
	CommentID       string
	ActorUserID     string
	Reason          string
	IPAddress       string
	UserAgent       string
	Action          string
	AuditAction     string
	NewStatus       string
	AllowedStatuses []string
}

func (r ModerationRepository) reviewComment(ctx context.Context, input commentReviewInput) (moderation.CommentReviewed, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return moderation.CommentReviewed{}, fmt.Errorf("begin comment review transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var contentID string
	var beforeStatus string
	err = tx.QueryRow(ctx, `
SELECT content_id::text, status
FROM comments
WHERE id = $1::uuid
  AND deleted_at IS NULL
FOR UPDATE`, input.CommentID).Scan(&contentID, &beforeStatus)
	if err != nil {
		return moderation.CommentReviewed{}, err
	}
	if !stringInSlice(beforeStatus, input.AllowedStatuses) {
		return moderation.CommentReviewed{}, pgx.ErrNoRows
	}

	var reviewed moderation.CommentReviewed
	err = tx.QueryRow(ctx, `
UPDATE comments
SET status = $2, updated_at = now()
WHERE id = $1::uuid
RETURNING id::text, content_id::text, status, updated_at`, input.CommentID, input.NewStatus).Scan(
		&reviewed.CommentID,
		&reviewed.ContentID,
		&reviewed.Status,
		&reviewed.UpdatedAt,
	)
	if err != nil {
		return moderation.CommentReviewed{}, fmt.Errorf("update comment status: %w", err)
	}

	counterDelta := 0
	if beforeStatus != "visible" && input.NewStatus == "visible" {
		counterDelta = 1
	} else if beforeStatus == "visible" && input.NewStatus != "visible" {
		counterDelta = -1
	}
	if counterDelta != 0 {
		err = tx.QueryRow(ctx, `
UPDATE content_items
SET comments_count = GREATEST(comments_count + $2, 0), updated_at = now()
WHERE id = $1::uuid
RETURNING comments_count`, contentID, counterDelta).Scan(&reviewed.CommentsCount)
		if err != nil {
			return moderation.CommentReviewed{}, fmt.Errorf("update comment counter: %w", err)
		}
		if err := enqueueSearchIndexTx(ctx, tx, contentID, "upsert_content"); err != nil {
			return moderation.CommentReviewed{}, err
		}
	} else {
		err = tx.QueryRow(ctx, `SELECT comments_count FROM content_items WHERE id = $1::uuid`, contentID).Scan(&reviewed.CommentsCount)
		if err != nil {
			return moderation.CommentReviewed{}, fmt.Errorf("load comment counter: %w", err)
		}
	}

	_, err = tx.Exec(ctx, `
INSERT INTO moderation_actions (actor_user_id, content_id, comment_id, action, reason, before_json, after_json, ip_address, user_agent)
VALUES (
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4,
  NULLIF($5, ''),
  jsonb_build_object('status', $6::text),
  jsonb_build_object('status', $7::text, 'reason', NULLIF($5, '')::text),
  NULLIF($8, '')::inet,
  $9
)`, input.ActorUserID, contentID, input.CommentID, input.Action, input.Reason, beforeStatus, input.NewStatus, input.IPAddress, input.UserAgent)
	if err != nil {
		return moderation.CommentReviewed{}, fmt.Errorf("insert comment moderation action: %w", err)
	}

	_, err = tx.Exec(ctx, `
INSERT INTO audit_logs (actor_user_id, action, entity_type, entity_id, before_json, after_json, ip_address, user_agent)
VALUES (
  $1::uuid,
  $2,
  'comment',
  $3::uuid,
  jsonb_build_object('status', $4::text),
  jsonb_build_object('status', $5::text, 'reason', NULLIF($6, '')::text),
  NULLIF($7, '')::inet,
  $8
)`, input.ActorUserID, input.AuditAction, input.CommentID, beforeStatus, input.NewStatus, input.Reason, input.IPAddress, input.UserAgent)
	if err != nil {
		return moderation.CommentReviewed{}, fmt.Errorf("insert comment audit log: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return moderation.CommentReviewed{}, fmt.Errorf("commit comment review transaction: %w", err)
	}
	reviewed.Reason = input.Reason
	return reviewed, nil
}

func (r ModerationRepository) ListReportQueue(ctx context.Context, filter moderation.QueueFilter) ([]moderation.ReportQueueItem, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	args := []any{limit}
	conditions := []string{"1 = 1"}
	if filter.Status != "" {
		args = append(args, filter.Status)
		conditions = append(conditions, fmt.Sprintf("rp.status = $%d", len(args)))
	} else {
		conditions = append(conditions, "rp.status IN ('open', 'in_review')")
	}

	rows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT
  rp.id::text,
  rp.content_id::text,
  ci.title,
  rp.comment_id::text,
  rp.reason,
  rp.details,
  rp.status,
  reporter.id::text,
  reporter.username,
  reporter.display_name,
  rp.resolved_by::text,
  rp.resolved_at,
  rp.created_at,
  rp.updated_at
FROM reports rp
LEFT JOIN content_items ci ON ci.id = rp.content_id
LEFT JOIN users reporter ON reporter.id = rp.reporter_user_id
WHERE %s
ORDER BY rp.updated_at DESC, rp.id DESC
LIMIT $1`, joinConditions(conditions)), args...)
	if err != nil {
		return nil, fmt.Errorf("query report moderation queue: %w", err)
	}
	defer rows.Close()

	items := make([]moderation.ReportQueueItem, 0, limit)
	for rows.Next() {
		var item moderation.ReportQueueItem
		err := rows.Scan(
			&item.ReportID,
			&item.ContentID,
			&item.ContentTitle,
			&item.CommentID,
			&item.Reason,
			&item.Details,
			&item.Status,
			&item.ReporterUserID,
			&item.ReporterUsername,
			&item.ReporterDisplayName,
			&item.ResolvedBy,
			&item.ResolvedAt,
			&item.CreatedAt,
			&item.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan report moderation queue: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read report moderation queue: %w", err)
	}
	return items, nil
}

func (r ModerationRepository) ReviewReport(ctx context.Context, input moderation.ReportReviewInput) (moderation.ReportReviewed, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return moderation.ReportReviewed{}, fmt.Errorf("begin report review transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var beforeStatus string
	err = tx.QueryRow(ctx, `
SELECT status
FROM reports
WHERE id = $1::uuid
FOR UPDATE`, input.ReportID).Scan(&beforeStatus)
	if err != nil {
		return moderation.ReportReviewed{}, err
	}
	if beforeStatus == input.Status {
		return moderation.ReportReviewed{}, pgx.ErrNoRows
	}

	var reviewed moderation.ReportReviewed
	var resolvedBy *string
	var resolvedAt *time.Time
	if input.Status == "resolved" || input.Status == "rejected" {
		resolvedBy = &input.ActorUserID
		err = tx.QueryRow(ctx, `
UPDATE reports
SET status = $2, resolved_by = $3::uuid, resolved_at = now(), updated_at = now()
WHERE id = $1::uuid
RETURNING id::text, status, resolved_by::text, resolved_at, updated_at`, input.ReportID, input.Status, input.ActorUserID).Scan(
			&reviewed.ReportID,
			&reviewed.Status,
			&reviewed.ResolvedBy,
			&reviewed.ResolvedAt,
			&reviewed.UpdatedAt,
		)
	} else {
		err = tx.QueryRow(ctx, `
UPDATE reports
SET status = $2, resolved_by = NULL, resolved_at = NULL, updated_at = now()
WHERE id = $1::uuid
RETURNING id::text, status, resolved_by::text, resolved_at, updated_at`, input.ReportID, input.Status).Scan(
			&reviewed.ReportID,
			&reviewed.Status,
			&reviewed.ResolvedBy,
			&reviewed.ResolvedAt,
			&reviewed.UpdatedAt,
		)
	}
	if err != nil {
		return moderation.ReportReviewed{}, fmt.Errorf("update report status: %w", err)
	}
	resolvedAt = reviewed.ResolvedAt

	_, err = tx.Exec(ctx, `
INSERT INTO audit_logs (actor_user_id, action, entity_type, entity_id, before_json, after_json, ip_address, user_agent)
VALUES (
  $1::uuid,
  'report.review',
  'report',
  $2::uuid,
  jsonb_build_object('status', $3::text),
  jsonb_build_object('status', $4::text, 'reason', NULLIF($5, '')::text, 'resolved_by', $6::text, 'resolved_at', $7::text),
  NULLIF($8, '')::inet,
  $9
)`, input.ActorUserID, input.ReportID, beforeStatus, input.Status, input.Reason, nullableString(resolvedBy), nullableTimePtr(resolvedAt), input.IPAddress, input.UserAgent)
	if err != nil {
		return moderation.ReportReviewed{}, fmt.Errorf("insert report audit log: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return moderation.ReportReviewed{}, fmt.Errorf("commit report review transaction: %w", err)
	}
	reviewed.Reason = input.Reason
	return reviewed, nil
}

type reviewContentInput struct {
	ContentID        string
	ActorUserID      string
	Reason           string
	IPAddress        string
	UserAgent        string
	Action           string
	AuditAction      string
	NewStatus        string
	AllowedStatuses  []string
	SetHiddenAt      bool
	SetPublishedAt   bool
	ClearPublishedAt bool
	RequireCleanScan bool
	SearchAction     string
}

func (r ModerationRepository) reviewContent(ctx context.Context, input reviewContentInput) (moderation.Reviewed, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return moderation.Reviewed{}, fmt.Errorf("begin %s transaction: %w", input.Action, err)
	}
	defer tx.Rollback(ctx)

	var beforeStatus string
	var beforeHiddenAt pgtype.Timestamptz
	var beforePublishedAt pgtype.Timestamptz
	err = tx.QueryRow(ctx, `
SELECT status, hidden_at, published_at
FROM content_items
WHERE id = $1::uuid
  AND deleted_at IS NULL
FOR UPDATE`,
		input.ContentID,
	).Scan(&beforeStatus, &beforeHiddenAt, &beforePublishedAt)
	if err != nil {
		return moderation.Reviewed{}, err
	}
	if !stringInSlice(beforeStatus, input.AllowedStatuses) {
		return moderation.Reviewed{}, pgx.ErrNoRows
	}
	if input.RequireCleanScan {
		var scanStatus string
		err = tx.QueryRow(ctx, `
SELECT scan_status
FROM content_files
WHERE content_id = $1::uuid
ORDER BY created_at DESC
LIMIT 1`, input.ContentID).Scan(&scanStatus)
		if err != nil {
			return moderation.Reviewed{}, err
		}
		if scanStatus != "clean" {
			return moderation.Reviewed{}, pgx.ErrNoRows
		}
	}

	var reviewed moderation.Reviewed
	var reviewedHiddenAt pgtype.Timestamptz
	if input.SetHiddenAt {
		err = tx.QueryRow(ctx, `
UPDATE content_items
SET status = $2, hidden_at = COALESCE(hidden_at, now()), updated_at = now()
WHERE id = $1::uuid
RETURNING id::text, status, hidden_at, updated_at`,
			input.ContentID,
			input.NewStatus,
		).Scan(&reviewed.ContentID, &reviewed.Status, &reviewedHiddenAt, &reviewed.UpdatedAt)
	} else if input.SetPublishedAt {
		err = tx.QueryRow(ctx, `
UPDATE content_items
SET status = $2, hidden_at = NULL, published_at = COALESCE(published_at, now()), updated_at = now()
WHERE id = $1::uuid
RETURNING id::text, status, hidden_at, updated_at`,
			input.ContentID,
			input.NewStatus,
		).Scan(&reviewed.ContentID, &reviewed.Status, &reviewedHiddenAt, &reviewed.UpdatedAt)
	} else {
		publishedAtSQL := "published_at"
		if input.ClearPublishedAt {
			publishedAtSQL = "NULL"
		}
		err = tx.QueryRow(ctx, `
UPDATE content_items
SET status = $2, hidden_at = NULL, published_at = `+publishedAtSQL+`, updated_at = now()
WHERE id = $1::uuid
RETURNING id::text, status, hidden_at, updated_at`,
			input.ContentID,
			input.NewStatus,
		).Scan(&reviewed.ContentID, &reviewed.Status, &reviewedHiddenAt, &reviewed.UpdatedAt)
	}
	if err != nil {
		return moderation.Reviewed{}, fmt.Errorf("update %s content: %w", input.Action, err)
	}
	if reviewedHiddenAt.Valid {
		hiddenAt := reviewedHiddenAt.Time
		reviewed.HiddenAt = &hiddenAt
	}

	_, err = tx.Exec(ctx, `
INSERT INTO moderation_actions (actor_user_id, content_id, action, reason, before_json, after_json, ip_address, user_agent)
VALUES (
  $1::uuid,
  $2::uuid,
  $3,
  NULLIF($4, ''),
  jsonb_build_object('status', $5::text, 'hidden_at', $6::text, 'published_at', $7::text),
  jsonb_build_object('status', $8::text, 'reason', NULLIF($4, '')::text),
  NULLIF($9, '')::inet,
  $10
)`,
		input.ActorUserID,
		input.ContentID,
		input.Action,
		input.Reason,
		beforeStatus,
		nullableTimeValue(beforeHiddenAt),
		nullableTimeValue(beforePublishedAt),
		input.NewStatus,
		input.IPAddress,
		input.UserAgent,
	)
	if err != nil {
		return moderation.Reviewed{}, fmt.Errorf("insert %s moderation action: %w", input.Action, err)
	}

	_, err = tx.Exec(ctx, `
INSERT INTO audit_logs (actor_user_id, action, entity_type, entity_id, before_json, after_json, ip_address, user_agent)
VALUES (
  $1::uuid,
  $2,
  'content_item',
  $3::uuid,
  jsonb_build_object('status', $4::text, 'hidden_at', $5::text, 'published_at', $6::text),
  jsonb_build_object('status', $7::text, 'reason', NULLIF($8, '')::text),
  NULLIF($9, '')::inet,
  $10
)`,
		input.ActorUserID,
		input.AuditAction,
		input.ContentID,
		beforeStatus,
		nullableTimeValue(beforeHiddenAt),
		nullableTimeValue(beforePublishedAt),
		input.NewStatus,
		input.Reason,
		input.IPAddress,
		input.UserAgent,
	)
	if err != nil {
		return moderation.Reviewed{}, fmt.Errorf("insert %s audit log: %w", input.Action, err)
	}

	if input.SearchAction != "" {
		if err := enqueueSearchIndexTx(ctx, tx, input.ContentID, input.SearchAction); err != nil {
			return moderation.Reviewed{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return moderation.Reviewed{}, fmt.Errorf("commit %s transaction: %w", input.Action, err)
	}

	reviewed.Reason = input.Reason
	return reviewed, nil
}

func nullableTimeValue(value pgtype.Timestamptz) any {
	if !value.Valid {
		return nil
	}
	return value.Time.Format(timeFormatRFC3339Micro)
}

func nullableTimePtr(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.Format(timeFormatRFC3339Micro)
}

func stringInSlice(value string, values []string) bool {
	for _, candidate := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func enqueueSearchIndexTx(ctx context.Context, tx pgx.Tx, contentID string, action string) error {
	payload, err := json.Marshal(jobs.SearchIndexPayload{
		ContentID: contentID,
		Action:    action,
	})
	if err != nil {
		return fmt.Errorf("marshal search index payload: %w", err)
	}
	_, err = tx.Exec(ctx, `
INSERT INTO worker_jobs (queue_name, job_type, payload)
VALUES ('search_index_queue', 'sync_content_search', $1::jsonb)`, string(payload))
	if err != nil {
		return fmt.Errorf("insert search index job: %w", err)
	}
	return nil
}
