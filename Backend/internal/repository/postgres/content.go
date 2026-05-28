package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"beeba.org/internal/domain/content"
	"beeba.org/internal/domain/jobs"
	"beeba.org/internal/security/secretbox"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ContentRepository struct {
	db      *pgxpool.Pool
	secrets *secretbox.Box
}

func NewContentRepository(db *pgxpool.Pool, secrets secretbox.Box) ContentRepository {
	return ContentRepository{db: db, secrets: &secrets}
}

func (r ContentRepository) ListPublished(ctx context.Context, filter content.ListFilter) (content.Page, error) {
	sortExpr, cursorCondition, err := contentSort(filter.Sort)
	if err != nil {
		return content.Page{}, err
	}

	args := []any{filter.Limit + 1}
	conditions := []string{
		"ci.status = 'published'",
		"ci.visibility = 'public'",
		"ci.deleted_at IS NULL",
		"ci.hidden_at IS NULL",
		"ci.published_at IS NOT NULL",
	}

	if !filter.IncludeNSFW {
		conditions = append(conditions, "ci.nsfw = false")
	}

	if filter.CategorySlug != "" {
		args = append(args, filter.CategorySlug)
		conditions = append(conditions, fmt.Sprintf("c.slug = $%d", len(args)))
	}

	if filter.Author != "" {
		args = append(args, strings.ToLower(filter.Author))
		conditions = append(conditions, fmt.Sprintf("lower(u.username) = $%d", len(args)))
	}

	if filter.Query != "" {
		args = append(args, "%"+strings.ToLower(filter.Query)+"%")
		conditions = append(conditions, fmt.Sprintf("(lower(ci.title) LIKE $%d OR lower(ci.description) LIKE $%d)", len(args), len(args)))
	}

	if len(filter.Tags) > 0 {
		args = append(args, filter.Tags)
		conditions = append(conditions, fmt.Sprintf(`EXISTS (
  SELECT 1
  FROM content_tags filter_ct
  JOIN tags filter_t ON filter_t.id = filter_ct.tag_id
  WHERE filter_ct.content_id = ci.id AND filter_t.slug = ANY($%d::text[])
)`, len(args)))
	}

	if filter.Cursor.ID != "" && filter.Cursor.SortValue != "" {
		args = append(args, filter.Cursor.SortValue, filter.Cursor.ID)
		conditions = append(conditions, fmt.Sprintf(cursorCondition, len(args)-1, len(args)))
	}

	sql := fmt.Sprintf(`
SELECT
  ci.id::text,
  ci.slug,
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
  author_avatar.id::text,
  COALESCE(
    jsonb_agg(DISTINCT jsonb_build_object('slug', t.slug, 'name', t.name))
      FILTER (WHERE t.id IS NOT NULL),
    '[]'::jsonb
  )::text AS tags_json,
  primary_image.id::text,
  ci.likes_count,
  ci.downloads_count,
  ci.comments_count,
  file.unlock_password_ciphertext,
  ci.published_at,
  ci.created_at,
  ci.updated_at
FROM content_items ci
JOIN categories c ON c.id = ci.category_id
JOIN users u ON u.id = ci.author_id
LEFT JOIN user_images author_avatar ON author_avatar.id = u.avatar_image_id AND author_avatar.processing_status = 'processed'
LEFT JOIN content_tags ct ON ct.content_id = ci.id
LEFT JOIN tags t ON t.id = ct.tag_id
LEFT JOIN LATERAL (
  SELECT id
  FROM content_images
  WHERE content_id = ci.id AND is_primary = true AND processing_status = 'processed' AND deleted_at IS NULL
  LIMIT 1
) primary_image ON true
LEFT JOIN LATERAL (
  SELECT unlock_password_ciphertext
  FROM content_files
  WHERE content_id = ci.id AND scan_status = 'clean'
  ORDER BY created_at DESC
  LIMIT 1
) file ON true
WHERE %s
GROUP BY ci.id, c.slug, c.name, u.id, u.username, u.display_name, author_avatar.id, primary_image.id, file.unlock_password_ciphertext
ORDER BY %s DESC, ci.id DESC
LIMIT $1`, strings.Join(conditions, " AND "), sortExpr)

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return content.Page{}, fmt.Errorf("query published content: %w", err)
	}
	defer rows.Close()

	items := make([]content.PublicItem, 0, filter.Limit)
	for rows.Next() {
		var item content.PublicItem
		var tagsJSON string
		var encryptedPassword *string
		if err := rows.Scan(
			&item.ID,
			&item.Slug,
			&item.Title,
			&item.Description,
			&item.Status,
			&item.Visibility,
			&item.NSFW,
			&item.Category.Slug,
			&item.Category.Name,
			&item.Author.ID,
			&item.Author.Username,
			&item.Author.DisplayName,
			&item.Author.AvatarImageID,
			&tagsJSON,
			&item.PreviewImageID,
			&item.LikesCount,
			&item.DownloadsCount,
			&item.CommentsCount,
			&encryptedPassword,
			&item.PublishedAt,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return content.Page{}, fmt.Errorf("scan published content: %w", err)
		}
		if err := decodeJSON([]byte(tagsJSON), &item.Tags); err != nil {
			return content.Page{}, fmt.Errorf("decode content tags: %w", err)
		}
		if encryptedPassword != nil && r.secrets != nil {
			password, err := r.secrets.DecryptString(*encryptedPassword)
			if err != nil {
				return content.Page{}, fmt.Errorf("decrypt public content unlock password: %w", err)
			}
			item.UnlockPassword = &password
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return content.Page{}, fmt.Errorf("read published content: %w", err)
	}

	page := content.Page{Items: items}
	if len(items) > filter.Limit {
		page.Items = items[:filter.Limit]
		last := page.Items[len(page.Items)-1]
		page.NextCursor = nextCursor(last, filter.Sort)
	}

	return page, nil
}

func (r ContentRepository) GetPublished(ctx context.Context, contentID string) (content.PublicDetail, error) {
	var item content.PublicDetail
	var tagsJSON string
	var galleryJSON string
	var encryptedPassword *string
	var fileSize *int64
	var fileHash *string
	var originalFilename *string
	err := r.db.QueryRow(ctx, `
SELECT
  ci.id::text,
  ci.slug,
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
  author_avatar.id::text,
  COALESCE(
    jsonb_agg(DISTINCT jsonb_build_object('slug', t.slug, 'name', t.name))
      FILTER (WHERE t.id IS NOT NULL),
    '[]'::jsonb
  )::text AS tags_json,
  primary_image.id::text,
  ci.likes_count,
  ci.downloads_count,
  ci.comments_count,
  file.unlock_password_ciphertext,
  file.file_size,
  file.file_hash_sha256,
  file.original_filename,
  ci.published_at,
  ci.created_at,
  ci.updated_at,
  COALESCE(gallery.images, '[]'::jsonb)::text AS gallery_json
FROM content_items ci
JOIN categories c ON c.id = ci.category_id
JOIN users u ON u.id = ci.author_id
LEFT JOIN user_images author_avatar ON author_avatar.id = u.avatar_image_id AND author_avatar.processing_status = 'processed'
LEFT JOIN content_tags ct ON ct.content_id = ci.id
LEFT JOIN tags t ON t.id = ct.tag_id
LEFT JOIN LATERAL (
  SELECT id
  FROM content_images
  WHERE content_id = ci.id AND is_primary = true AND processing_status = 'processed' AND deleted_at IS NULL
  LIMIT 1
) primary_image ON true
LEFT JOIN LATERAL (
  SELECT unlock_password_ciphertext, file_size, file_hash_sha256, original_filename
  FROM content_files
  WHERE content_id = ci.id AND scan_status = 'clean'
  ORDER BY created_at DESC
  LIMIT 1
) file ON true
LEFT JOIN LATERAL (
  SELECT jsonb_agg(
    jsonb_build_object(
      'id', id::text,
      'url', '/api/v1/media/' || id::text,
      'alt_text', alt_text,
      'width', width,
      'height', height,
      'is_primary', is_primary,
      'sort_order', sort_order,
      'created_at', created_at
    )
    ORDER BY is_primary DESC, sort_order ASC, created_at ASC
  ) AS images
  FROM content_images
  WHERE content_id = ci.id AND processing_status = 'processed' AND deleted_at IS NULL
) gallery ON true
WHERE ci.id = $1::uuid
  AND ci.status = 'published'
  AND ci.visibility = 'public'
  AND ci.deleted_at IS NULL
  AND ci.hidden_at IS NULL
  AND ci.published_at IS NOT NULL
GROUP BY ci.id, c.slug, c.name, u.id, u.username, u.display_name, author_avatar.id, primary_image.id, file.unlock_password_ciphertext, file.file_size, file.file_hash_sha256, file.original_filename, gallery.images`, contentID).Scan(
		&item.ID,
		&item.Slug,
		&item.Title,
		&item.Description,
		&item.Status,
		&item.Visibility,
		&item.NSFW,
		&item.Category.Slug,
		&item.Category.Name,
		&item.Author.ID,
		&item.Author.Username,
		&item.Author.DisplayName,
		&item.Author.AvatarImageID,
		&tagsJSON,
		&item.PreviewImageID,
		&item.LikesCount,
		&item.DownloadsCount,
		&item.CommentsCount,
		&encryptedPassword,
		&fileSize,
		&fileHash,
		&originalFilename,
		&item.PublishedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
		&galleryJSON,
	)
	if err != nil {
		return content.PublicDetail{}, err
	}
	if err := decodeJSON([]byte(tagsJSON), &item.Tags); err != nil {
		return content.PublicDetail{}, fmt.Errorf("decode content tags: %w", err)
	}
	if err := decodeJSON([]byte(galleryJSON), &item.Gallery); err != nil {
		return content.PublicDetail{}, fmt.Errorf("decode content gallery: %w", err)
	}
	if encryptedPassword != nil && r.secrets != nil {
		password, err := r.secrets.DecryptString(*encryptedPassword)
		if err != nil {
			return content.PublicDetail{}, fmt.Errorf("decrypt public content unlock password: %w", err)
		}
		item.UnlockPassword = &password
	}
	if fileSize != nil && fileHash != nil && originalFilename != nil {
		item.File = &content.PublicFile{
			FileSize:         *fileSize,
			FileHashSHA256:   *fileHash,
			OriginalFilename: *originalFilename,
		}
	}
	return item, nil
}

func (r ContentRepository) ListPublishedByIDs(ctx context.Context, ids []string) ([]content.PublicItem, error) {
	if len(ids) == 0 {
		return []content.PublicItem{}, nil
	}
	args := make([]any, 0, len(ids))
	values := make([]string, 0, len(ids))
	for index, id := range ids {
		args = append(args, id)
		values = append(values, fmt.Sprintf("($%d::uuid, %d)", len(args), index+1))
	}

	sql := fmt.Sprintf(`
WITH selected(id, ord) AS (
  VALUES %s
)
SELECT
  ci.id::text,
  ci.slug,
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
  author_avatar.id::text,
  COALESCE(
    jsonb_agg(DISTINCT jsonb_build_object('slug', t.slug, 'name', t.name))
      FILTER (WHERE t.id IS NOT NULL),
    '[]'::jsonb
  )::text AS tags_json,
  primary_image.id::text,
  ci.likes_count,
  ci.downloads_count,
  ci.comments_count,
  file.unlock_password_ciphertext,
  ci.published_at,
  ci.created_at,
  ci.updated_at,
  selected.ord
FROM selected
JOIN content_items ci ON ci.id = selected.id
JOIN categories c ON c.id = ci.category_id
JOIN users u ON u.id = ci.author_id
LEFT JOIN user_images author_avatar ON author_avatar.id = u.avatar_image_id AND author_avatar.processing_status = 'processed'
LEFT JOIN content_tags ct ON ct.content_id = ci.id
LEFT JOIN tags t ON t.id = ct.tag_id
LEFT JOIN LATERAL (
  SELECT id
  FROM content_images
  WHERE content_id = ci.id AND is_primary = true AND processing_status = 'processed' AND deleted_at IS NULL
  LIMIT 1
) primary_image ON true
LEFT JOIN LATERAL (
  SELECT unlock_password_ciphertext
  FROM content_files
  WHERE content_id = ci.id AND scan_status = 'clean'
  ORDER BY created_at DESC
  LIMIT 1
) file ON true
WHERE ci.status = 'published'
  AND ci.visibility = 'public'
  AND ci.deleted_at IS NULL
  AND ci.hidden_at IS NULL
  AND ci.published_at IS NOT NULL
GROUP BY selected.ord, ci.id, c.slug, c.name, u.id, u.username, u.display_name, author_avatar.id, primary_image.id, file.unlock_password_ciphertext
ORDER BY selected.ord ASC`, strings.Join(values, ","))

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query published content by ids: %w", err)
	}
	defer rows.Close()

	items := make([]content.PublicItem, 0, len(ids))
	for rows.Next() {
		var item content.PublicItem
		var tagsJSON string
		var encryptedPassword *string
		var ignoredOrder int
		if err := rows.Scan(
			&item.ID,
			&item.Slug,
			&item.Title,
			&item.Description,
			&item.Status,
			&item.Visibility,
			&item.NSFW,
			&item.Category.Slug,
			&item.Category.Name,
			&item.Author.ID,
			&item.Author.Username,
			&item.Author.DisplayName,
			&item.Author.AvatarImageID,
			&tagsJSON,
			&item.PreviewImageID,
			&item.LikesCount,
			&item.DownloadsCount,
			&item.CommentsCount,
			&encryptedPassword,
			&item.PublishedAt,
			&item.CreatedAt,
			&item.UpdatedAt,
			&ignoredOrder,
		); err != nil {
			return nil, fmt.Errorf("scan published content by ids: %w", err)
		}
		if err := decodeJSON([]byte(tagsJSON), &item.Tags); err != nil {
			return nil, fmt.Errorf("decode content tags: %w", err)
		}
		if encryptedPassword != nil && r.secrets != nil {
			password, err := r.secrets.DecryptString(*encryptedPassword)
			if err != nil {
				return nil, fmt.Errorf("decrypt public content unlock password: %w", err)
			}
			item.UnlockPassword = &password
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read published content by ids: %w", err)
	}
	return items, nil
}

func (r ContentRepository) ListOwned(ctx context.Context, ownerID string, filter content.OwnerListFilter) (content.OwnerPage, error) {
	args := []any{filter.Limit + 1, ownerID}
	conditions := []string{
		"ci.author_id = $2::uuid",
		"ci.deleted_at IS NULL",
	}

	if filter.Status != "" {
		args = append(args, filter.Status)
		conditions = append(conditions, fmt.Sprintf("ci.status = $%d", len(args)))
	}
	if filter.Visibility != "" {
		args = append(args, filter.Visibility)
		conditions = append(conditions, fmt.Sprintf("ci.visibility = $%d", len(args)))
	}
	if filter.Query != "" {
		args = append(args, "%"+strings.ToLower(filter.Query)+"%")
		conditions = append(conditions, fmt.Sprintf("(lower(ci.title) LIKE $%d OR lower(ci.description) LIKE $%d)", len(args), len(args)))
	}
	if filter.Cursor.ID != "" && filter.Cursor.SortValue != "" {
		args = append(args, filter.Cursor.SortValue, filter.Cursor.ID)
		conditions = append(conditions, fmt.Sprintf("(ci.created_at, ci.id) < ($%d::timestamptz, $%d::uuid)", len(args)-1, len(args)))
	}

	sql := fmt.Sprintf(`
SELECT
  ci.id::text,
  ci.slug,
  ci.title,
  ci.description,
  ci.status,
  ci.visibility,
  ci.nsfw,
  c.slug,
  c.name,
  COALESCE(
    jsonb_agg(DISTINCT jsonb_build_object('slug', t.slug, 'name', t.name))
      FILTER (WHERE t.id IS NOT NULL),
    '[]'::jsonb
  )::text AS tags_json,
  file.id::text,
  file.scan_status,
  file.file_size,
  file.file_hash_sha256,
  file.original_filename,
  file.safe_filename,
  file.unlock_password_ciphertext,
  moderation.action,
  moderation.reason,
  moderation.created_at,
  ci.published_at,
  ci.hidden_at,
  ci.created_at,
  ci.updated_at
FROM content_items ci
JOIN categories c ON c.id = ci.category_id
LEFT JOIN content_tags ct ON ct.content_id = ci.id
LEFT JOIN tags t ON t.id = ct.tag_id
LEFT JOIN LATERAL (
  SELECT id, scan_status, file_size, file_hash_sha256, original_filename, safe_filename, unlock_password_ciphertext
  FROM content_files
  WHERE content_id = ci.id
  ORDER BY created_at DESC
  LIMIT 1
) file ON true
LEFT JOIN LATERAL (
  SELECT action, reason, created_at
  FROM moderation_actions
  WHERE content_id = ci.id
    AND action IN ('reject', 'hide')
    AND reason IS NOT NULL
    AND reason <> ''
  ORDER BY created_at DESC
  LIMIT 1
) moderation ON true
WHERE %s
GROUP BY ci.id, c.slug, c.name, file.id, file.scan_status, file.file_size, file.file_hash_sha256, file.original_filename, file.safe_filename, file.unlock_password_ciphertext, moderation.action, moderation.reason, moderation.created_at
ORDER BY ci.created_at DESC, ci.id DESC
LIMIT $1`, strings.Join(conditions, " AND "))

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return content.OwnerPage{}, fmt.Errorf("query owned content: %w", err)
	}
	defer rows.Close()

	items := make([]content.OwnerItem, 0, filter.Limit)
	for rows.Next() {
		var item content.OwnerItem
		var tagsJSON string
		var fileID *string
		var fileScanStatus *string
		var fileSize *int64
		var fileHash *string
		var originalFilename *string
		var safeFilename *string
		var encryptedPassword *string
		var moderationAction *string
		var moderationReason *string
		var moderationCreatedAt *time.Time
		if err := rows.Scan(
			&item.ID,
			&item.Slug,
			&item.Title,
			&item.Description,
			&item.Status,
			&item.Visibility,
			&item.NSFW,
			&item.Category.Slug,
			&item.Category.Name,
			&tagsJSON,
			&fileID,
			&fileScanStatus,
			&fileSize,
			&fileHash,
			&originalFilename,
			&safeFilename,
			&encryptedPassword,
			&moderationAction,
			&moderationReason,
			&moderationCreatedAt,
			&item.PublishedAt,
			&item.HiddenAt,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return content.OwnerPage{}, fmt.Errorf("scan owned content: %w", err)
		}
		if err := decodeJSON([]byte(tagsJSON), &item.Tags); err != nil {
			return content.OwnerPage{}, fmt.Errorf("decode owned content tags: %w", err)
		}
		if fileID != nil && fileScanStatus != nil && fileSize != nil && fileHash != nil && originalFilename != nil && safeFilename != nil {
			item.File = &content.OwnerFile{
				ID:               *fileID,
				ScanStatus:       *fileScanStatus,
				FileSize:         *fileSize,
				FileHashSHA256:   *fileHash,
				OriginalFilename: *originalFilename,
				SafeFilename:     *safeFilename,
			}
		}
		if encryptedPassword != nil && r.secrets != nil {
			password, err := r.secrets.DecryptString(*encryptedPassword)
			if err != nil {
				return content.OwnerPage{}, fmt.Errorf("decrypt owned content unlock password: %w", err)
			}
			item.UnlockPassword = &password
		}
		if moderationAction != nil && moderationReason != nil && moderationCreatedAt != nil && (item.Status == "rejected" || item.Status == "hidden") {
			item.Moderation = &content.OwnerModerationFeedback{
				Action:    *moderationAction,
				Reason:    *moderationReason,
				CreatedAt: *moderationCreatedAt,
			}
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return content.OwnerPage{}, fmt.Errorf("read owned content: %w", err)
	}

	page := content.OwnerPage{Items: items}
	if len(items) > filter.Limit {
		page.Items = items[:filter.Limit]
		last := page.Items[len(page.Items)-1]
		page.NextCursor = &content.Cursor{
			SortValue: last.CreatedAt.Format(timeFormatRFC3339Micro),
			ID:        last.ID,
		}
	}

	return page, nil
}

func (r ContentRepository) UpdateOwned(ctx context.Context, ownerID string, contentID string, input content.OwnerUpdateInput) (content.OwnerItem, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return content.OwnerItem{}, fmt.Errorf("begin owned content update transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var currentTitle string
	var currentDescription string
	var currentVisibility string
	var currentNSFW bool
	var currentStatus string
	var latestScanStatus *string
	err = tx.QueryRow(ctx, `
SELECT
  ci.title,
  ci.description,
  ci.visibility,
  ci.nsfw,
  ci.status,
  file.scan_status
FROM content_items ci
LEFT JOIN LATERAL (
  SELECT scan_status
  FROM content_files
  WHERE content_id = ci.id
  ORDER BY created_at DESC
  LIMIT 1
) file ON true
WHERE ci.id = $1::uuid
  AND ci.author_id = $2::uuid
  AND ci.deleted_at IS NULL
  AND ci.status <> 'deleted'
FOR UPDATE OF ci`, contentID, ownerID).Scan(
		&currentTitle,
		&currentDescription,
		&currentVisibility,
		&currentNSFW,
		&currentStatus,
		&latestScanStatus,
	)
	if err != nil {
		return content.OwnerItem{}, err
	}

	title := currentTitle
	if input.Title != nil {
		title = *input.Title
	}
	description := currentDescription
	if input.Description != nil {
		description = *input.Description
	}
	visibility := currentVisibility
	if input.Visibility != nil {
		visibility = *input.Visibility
	}
	nsfw := currentNSFW
	if input.NSFW != nil {
		nsfw = *input.NSFW
	}

	newStatus := currentStatus
	clearPublicationState := false
	if ownerUpdateRequiresModeration(currentStatus) {
		if latestScanStatus == nil || *latestScanStatus != "clean" {
			return content.OwnerItem{}, pgx.ErrNoRows
		}
		newStatus = "pending_moderation"
		clearPublicationState = true
	}

	_, err = tx.Exec(ctx, `
UPDATE content_items
SET
  title = $3,
  description = $4,
  visibility = $5,
  nsfw = $6,
  status = $7,
  published_at = CASE WHEN $8 THEN NULL ELSE published_at END,
  hidden_at = CASE WHEN $8 THEN NULL ELSE hidden_at END,
  updated_at = now()
WHERE id = $1::uuid
  AND author_id = $2::uuid`,
		contentID,
		ownerID,
		title,
		description,
		visibility,
		nsfw,
		newStatus,
		clearPublicationState,
	)
	if err != nil {
		return content.OwnerItem{}, fmt.Errorf("update owned content metadata: %w", err)
	}

	if input.Tags != nil {
		if err := replaceContentTagsTx(ctx, tx, contentID, *input.Tags); err != nil {
			return content.OwnerItem{}, err
		}
	}

	if currentStatus == "published" && currentVisibility == "public" {
		if err := enqueueContentSearchTx(ctx, tx, contentID, "delete_content"); err != nil {
			return content.OwnerItem{}, err
		}
	}

	item, err := getOwnedContentTx(ctx, tx, ownerID, contentID, r.secrets)
	if err != nil {
		return content.OwnerItem{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return content.OwnerItem{}, fmt.Errorf("commit owned content update transaction: %w", err)
	}

	return item, nil
}

func ownerUpdateRequiresModeration(status string) bool {
	switch status {
	case "approved", "published", "rejected", "hidden":
		return true
	default:
		return false
	}
}

func (r ContentRepository) DeleteOwned(ctx context.Context, ownerID string, contentID string) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin owned content delete transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var wasPublishedPublic bool
	err = tx.QueryRow(ctx, `
WITH target AS (
  SELECT id, status = 'published' AND visibility = 'public' AS was_published_public
  FROM content_items
  WHERE id = $1::uuid
    AND author_id = $2::uuid
    AND deleted_at IS NULL
    AND status <> 'deleted'
  FOR UPDATE
),
updated AS (
  UPDATE content_items
  SET
    status = 'deleted',
    published_at = NULL,
    hidden_at = NULL,
    deleted_at = now(),
    updated_at = now()
  FROM target
  WHERE content_items.id = target.id
  RETURNING target.was_published_public
)
SELECT was_published_public FROM updated`,
		contentID,
		ownerID,
	).Scan(&wasPublishedPublic)
	if err != nil {
		return err
	}

	if wasPublishedPublic {
		if err := enqueueContentSearchTx(ctx, tx, contentID, "delete_content"); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit owned content delete transaction: %w", err)
	}
	return nil
}

func replaceContentTagsTx(ctx context.Context, tx pgx.Tx, contentID string, tags []content.TagInput) error {
	if _, err := tx.Exec(ctx, `DELETE FROM content_tags WHERE content_id = $1::uuid`, contentID); err != nil {
		return fmt.Errorf("delete existing content tags: %w", err)
	}
	for _, tag := range tags {
		var tagID string
		err := tx.QueryRow(ctx, `
INSERT INTO tags (slug, name)
VALUES ($1, $2)
ON CONFLICT (slug) DO UPDATE SET updated_at = tags.updated_at
RETURNING id::text`, tag.Slug, tag.Name).Scan(&tagID)
		if err != nil {
			return fmt.Errorf("upsert content tag %q: %w", tag.Slug, err)
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO content_tags (content_id, tag_id)
VALUES ($1::uuid, $2::uuid)
ON CONFLICT DO NOTHING`, contentID, tagID); err != nil {
			return fmt.Errorf("link content tag %q: %w", tag.Slug, err)
		}
	}
	return nil
}

func getOwnedContentTx(ctx context.Context, tx pgx.Tx, ownerID string, contentID string, secrets *secretbox.Box) (content.OwnerItem, error) {
	row := tx.QueryRow(ctx, `
SELECT
  ci.id::text,
  ci.slug,
  ci.title,
  ci.description,
  ci.status,
  ci.visibility,
  ci.nsfw,
  c.slug,
  c.name,
  COALESCE(
    jsonb_agg(DISTINCT jsonb_build_object('slug', t.slug, 'name', t.name))
      FILTER (WHERE t.id IS NOT NULL),
    '[]'::jsonb
  )::text AS tags_json,
  file.id::text,
  file.scan_status,
  file.file_size,
  file.file_hash_sha256,
  file.original_filename,
  file.safe_filename,
  file.unlock_password_ciphertext,
  moderation.action,
  moderation.reason,
  moderation.created_at,
  ci.published_at,
  ci.hidden_at,
  ci.created_at,
  ci.updated_at
FROM content_items ci
JOIN categories c ON c.id = ci.category_id
LEFT JOIN content_tags ct ON ct.content_id = ci.id
LEFT JOIN tags t ON t.id = ct.tag_id
LEFT JOIN LATERAL (
  SELECT id, scan_status, file_size, file_hash_sha256, original_filename, safe_filename, unlock_password_ciphertext
  FROM content_files
  WHERE content_id = ci.id
  ORDER BY created_at DESC
  LIMIT 1
) file ON true
LEFT JOIN LATERAL (
  SELECT action, reason, created_at
  FROM moderation_actions
  WHERE content_id = ci.id
    AND action IN ('reject', 'hide')
    AND reason IS NOT NULL
    AND reason <> ''
  ORDER BY created_at DESC
  LIMIT 1
) moderation ON true
WHERE ci.id = $1::uuid
  AND ci.author_id = $2::uuid
  AND ci.deleted_at IS NULL
GROUP BY ci.id, c.slug, c.name, file.id, file.scan_status, file.file_size, file.file_hash_sha256, file.original_filename, file.safe_filename, file.unlock_password_ciphertext, moderation.action, moderation.reason, moderation.created_at`,
		contentID,
		ownerID,
	)
	return scanOwnedContentRow(row, secrets)
}

func scanOwnedContentRow(row pgx.Row, secrets *secretbox.Box) (content.OwnerItem, error) {
	var item content.OwnerItem
	var tagsJSON string
	var fileID *string
	var fileScanStatus *string
	var fileSize *int64
	var fileHash *string
	var originalFilename *string
	var safeFilename *string
	var encryptedPassword *string
	var moderationAction *string
	var moderationReason *string
	var moderationCreatedAt *time.Time
	if err := row.Scan(
		&item.ID,
		&item.Slug,
		&item.Title,
		&item.Description,
		&item.Status,
		&item.Visibility,
		&item.NSFW,
		&item.Category.Slug,
		&item.Category.Name,
		&tagsJSON,
		&fileID,
		&fileScanStatus,
		&fileSize,
		&fileHash,
		&originalFilename,
		&safeFilename,
		&encryptedPassword,
		&moderationAction,
		&moderationReason,
		&moderationCreatedAt,
		&item.PublishedAt,
		&item.HiddenAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return content.OwnerItem{}, err
	}
	if err := decodeJSON([]byte(tagsJSON), &item.Tags); err != nil {
		return content.OwnerItem{}, fmt.Errorf("decode owned content tags: %w", err)
	}
	if fileID != nil && fileScanStatus != nil && fileSize != nil && fileHash != nil && originalFilename != nil && safeFilename != nil {
		item.File = &content.OwnerFile{
			ID:               *fileID,
			ScanStatus:       *fileScanStatus,
			FileSize:         *fileSize,
			FileHashSHA256:   *fileHash,
			OriginalFilename: *originalFilename,
			SafeFilename:     *safeFilename,
		}
	}
	if encryptedPassword != nil && secrets != nil {
		password, err := secrets.DecryptString(*encryptedPassword)
		if err != nil {
			return content.OwnerItem{}, fmt.Errorf("decrypt owned content unlock password: %w", err)
		}
		item.UnlockPassword = &password
	}
	if moderationAction != nil && moderationReason != nil && moderationCreatedAt != nil && (item.Status == "rejected" || item.Status == "hidden") {
		item.Moderation = &content.OwnerModerationFeedback{
			Action:    *moderationAction,
			Reason:    *moderationReason,
			CreatedAt: *moderationCreatedAt,
		}
	}
	return item, nil
}

func enqueueContentSearchTx(ctx context.Context, tx pgx.Tx, contentID string, action string) error {
	payload, err := json.Marshal(jobs.SearchIndexPayload{
		ContentID: contentID,
		Action:    action,
	})
	if err != nil {
		return fmt.Errorf("marshal search index payload: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO worker_jobs (queue_name, job_type, payload)
VALUES ('search_index_queue', 'sync_content_search', $1::jsonb)`, string(payload)); err != nil {
		return fmt.Errorf("insert search index job: %w", err)
	}
	return nil
}

func contentSort(sort string) (expr string, cursorCondition string, err error) {
	switch sort {
	case "", "newest":
		return "ci.published_at", "(ci.published_at, ci.id) < ($%d::timestamptz, $%d::uuid)", nil
	case "likes":
		return "ci.likes_count", "(ci.likes_count, ci.id) < ($%d::integer, $%d::uuid)", nil
	case "downloads":
		return "ci.downloads_count", "(ci.downloads_count, ci.id) < ($%d::integer, $%d::uuid)", nil
	default:
		return "", "", fmt.Errorf("unsupported content sort %q", sort)
	}
}

func nextCursor(item content.PublicItem, sort string) *content.Cursor {
	cursor := &content.Cursor{ID: item.ID}
	switch sort {
	case "likes":
		cursor.SortValue = fmt.Sprintf("%d", item.LikesCount)
	case "downloads":
		cursor.SortValue = fmt.Sprintf("%d", item.DownloadsCount)
	default:
		if item.PublishedAt != nil {
			cursor.SortValue = item.PublishedAt.Format(timeFormatRFC3339Micro)
		}
	}
	return cursor
}

const timeFormatRFC3339Micro = "2006-01-02T15:04:05.999999Z07:00"
