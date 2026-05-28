package postgres

import (
	"context"
	"fmt"

	"beeba.org/internal/domain/social"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SocialRepository struct {
	db *pgxpool.Pool
}

func NewSocialRepository(db *pgxpool.Pool) SocialRepository {
	return SocialRepository{db: db}
}

func (r SocialRepository) ListComments(ctx context.Context, contentID string, filter social.CommentFilter) (social.CommentPage, error) {
	args := []any{contentID, filter.Limit + 1}
	conditions := []string{
		"cm.content_id = $1::uuid",
		"cm.status = 'visible'",
		"cm.deleted_at IS NULL",
		"ci.status = 'published'",
		"ci.visibility = 'public'",
		"ci.deleted_at IS NULL",
		"ci.hidden_at IS NULL",
	}
	if filter.Cursor.ID != "" && filter.Cursor.SortValue != "" {
		args = append(args, filter.Cursor.SortValue, filter.Cursor.ID)
		conditions = append(conditions, fmt.Sprintf("(cm.created_at, cm.id) < ($%d::timestamptz, $%d::uuid)", len(args)-1, len(args)))
	}

	rows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT
  cm.id::text,
  cm.content_id::text,
  u.id::text,
  u.username,
  u.display_name,
  cm.parent_id::text,
  cm.body,
  cm.status,
  cm.created_at,
  cm.updated_at
FROM comments cm
JOIN content_items ci ON ci.id = cm.content_id
JOIN users u ON u.id = cm.user_id
WHERE %s
ORDER BY cm.created_at DESC, cm.id DESC
LIMIT $2`, joinConditions(conditions)), args...)
	if err != nil {
		return social.CommentPage{}, fmt.Errorf("query comments: %w", err)
	}
	defer rows.Close()

	items := make([]social.Comment, 0, filter.Limit)
	for rows.Next() {
		comment, err := scanComment(rows)
		if err != nil {
			return social.CommentPage{}, err
		}
		items = append(items, comment)
	}
	if err := rows.Err(); err != nil {
		return social.CommentPage{}, fmt.Errorf("read comments: %w", err)
	}

	page := social.CommentPage{Items: items}
	if len(items) > filter.Limit {
		page.Items = items[:filter.Limit]
		last := page.Items[len(page.Items)-1]
		page.NextCursor = &social.Cursor{
			SortValue: last.CreatedAt.Format(timeFormatRFC3339Micro),
			ID:        last.ID,
		}
	}
	return page, nil
}

func (r SocialRepository) CreateComment(ctx context.Context, input social.CommentInput) (social.Comment, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return social.Comment{}, fmt.Errorf("begin comment transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var comment social.Comment
	err = tx.QueryRow(ctx, `
WITH target AS (
  SELECT id
  FROM content_items
  WHERE id = $1::uuid
    AND status = 'published'
    AND visibility = 'public'
    AND deleted_at IS NULL
    AND hidden_at IS NULL
),
parent AS (
  SELECT id
  FROM comments
  WHERE id = NULLIF($3, '')::uuid
    AND content_id = $1::uuid
    AND status = 'visible'
    AND deleted_at IS NULL
),
created AS (
  INSERT INTO comments (content_id, user_id, parent_id, body, status)
  SELECT target.id, $2::uuid, CASE WHEN NULLIF($3, '') IS NULL THEN NULL ELSE parent.id END, $4, 'visible'
  FROM target
  LEFT JOIN parent ON true
  WHERE NULLIF($3, '') IS NULL OR parent.id IS NOT NULL
  RETURNING id, content_id, user_id, parent_id, body, status, created_at, updated_at
)
SELECT
  created.id::text,
  created.content_id::text,
  u.id::text,
  u.username,
  u.display_name,
  created.parent_id::text,
  created.body,
  created.status,
  created.created_at,
  created.updated_at
FROM created
JOIN users u ON u.id = created.user_id`, input.ContentID, input.UserID, nullableString(input.ParentID), input.Body).Scan(
		&comment.ID,
		&comment.ContentID,
		&comment.UserID,
		&comment.Username,
		&comment.DisplayName,
		&comment.ParentID,
		&comment.Body,
		&comment.Status,
		&comment.CreatedAt,
		&comment.UpdatedAt,
	)
	if err != nil {
		return social.Comment{}, err
	}

	_, err = tx.Exec(ctx, `
UPDATE content_items
SET comments_count = comments_count + 1, updated_at = now()
WHERE id = $1::uuid`, input.ContentID)
	if err != nil {
		return social.Comment{}, fmt.Errorf("increment comment counter: %w", err)
	}
	if err := enqueueContentSearchTx(ctx, tx, input.ContentID, "upsert_content"); err != nil {
		return social.Comment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return social.Comment{}, fmt.Errorf("commit comment transaction: %w", err)
	}
	return comment, nil
}

func (r SocialRepository) Like(ctx context.Context, contentID string, userID string) (social.LikeResult, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return social.LikeResult{}, fmt.Errorf("begin like transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var likesCount int
	err = tx.QueryRow(ctx, `
SELECT likes_count
FROM content_items
WHERE id = $1::uuid
  AND status = 'published'
  AND visibility = 'public'
  AND deleted_at IS NULL
  AND hidden_at IS NULL
FOR UPDATE`, contentID).Scan(&likesCount)
	if err != nil {
		return social.LikeResult{}, err
	}

	tag, err := tx.Exec(ctx, `
INSERT INTO likes (content_id, user_id)
VALUES ($1::uuid, $2::uuid)
ON CONFLICT DO NOTHING`, contentID, userID)
	if err != nil {
		return social.LikeResult{}, fmt.Errorf("insert like: %w", err)
	}
	if tag.RowsAffected() > 0 {
		err = tx.QueryRow(ctx, `
UPDATE content_items
SET likes_count = likes_count + 1, updated_at = now()
WHERE id = $1::uuid
RETURNING likes_count`, contentID).Scan(&likesCount)
		if err != nil {
			return social.LikeResult{}, fmt.Errorf("increment like counter: %w", err)
		}
		if err := enqueueContentSearchTx(ctx, tx, contentID, "upsert_content"); err != nil {
			return social.LikeResult{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return social.LikeResult{}, fmt.Errorf("commit like transaction: %w", err)
	}
	return social.LikeResult{ContentID: contentID, Liked: true, LikesCount: likesCount}, nil
}

func (r SocialRepository) Unlike(ctx context.Context, contentID string, userID string) (social.LikeResult, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return social.LikeResult{}, fmt.Errorf("begin unlike transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var likesCount int
	err = tx.QueryRow(ctx, `
SELECT likes_count
FROM content_items
WHERE id = $1::uuid
  AND status = 'published'
  AND visibility = 'public'
  AND deleted_at IS NULL
  AND hidden_at IS NULL
FOR UPDATE`, contentID).Scan(&likesCount)
	if err != nil {
		return social.LikeResult{}, err
	}

	tag, err := tx.Exec(ctx, `DELETE FROM likes WHERE content_id = $1::uuid AND user_id = $2::uuid`, contentID, userID)
	if err != nil {
		return social.LikeResult{}, fmt.Errorf("delete like: %w", err)
	}
	if tag.RowsAffected() > 0 {
		err = tx.QueryRow(ctx, `
UPDATE content_items
SET likes_count = GREATEST(likes_count - 1, 0), updated_at = now()
WHERE id = $1::uuid
RETURNING likes_count`, contentID).Scan(&likesCount)
		if err != nil {
			return social.LikeResult{}, fmt.Errorf("decrement like counter: %w", err)
		}
		if err := enqueueContentSearchTx(ctx, tx, contentID, "upsert_content"); err != nil {
			return social.LikeResult{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return social.LikeResult{}, fmt.Errorf("commit unlike transaction: %w", err)
	}
	return social.LikeResult{ContentID: contentID, Liked: false, LikesCount: likesCount}, nil
}

func (r SocialRepository) ReportContent(ctx context.Context, input social.ReportInput) (social.Report, error) {
	var report social.Report
	err := r.db.QueryRow(ctx, `
WITH target AS (
  SELECT id
  FROM content_items
  WHERE id = $1::uuid
    AND status = 'published'
    AND visibility = 'public'
    AND deleted_at IS NULL
    AND hidden_at IS NULL
),
created AS (
  INSERT INTO reports (content_id, reporter_user_id, reason, details, status)
  SELECT target.id, $2::uuid, $3, NULLIF($4, ''), 'open'
  FROM target
  RETURNING id, content_id, reporter_user_id, reason, details, status, created_at
)
SELECT id::text, content_id::text, reporter_user_id::text, reason, details, status, created_at
FROM created`, input.ContentID, input.UserID, input.Reason, input.Details).Scan(
		&report.ID,
		&report.ContentID,
		&report.UserID,
		&report.Reason,
		&report.Details,
		&report.Status,
		&report.CreatedAt,
	)
	if err != nil {
		return social.Report{}, err
	}
	return report, nil
}

func nullableString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

type commentScanner interface {
	Scan(dest ...any) error
}

func scanComment(row commentScanner) (social.Comment, error) {
	var comment social.Comment
	if err := row.Scan(
		&comment.ID,
		&comment.ContentID,
		&comment.UserID,
		&comment.Username,
		&comment.DisplayName,
		&comment.ParentID,
		&comment.Body,
		&comment.Status,
		&comment.CreatedAt,
		&comment.UpdatedAt,
	); err != nil {
		return social.Comment{}, fmt.Errorf("scan comment: %w", err)
	}
	return comment, nil
}
