package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"beeba.org/internal/domain/media"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MediaRepository struct {
	db *pgxpool.Pool
}

func NewMediaRepository(db *pgxpool.Pool) MediaRepository {
	return MediaRepository{db: db}
}

func (r MediaRepository) CreateUserAvatar(ctx context.Context, input media.ImageUploadInput) (media.UploadedImage, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return media.UploadedImage{}, fmt.Errorf("begin avatar upload: %w", err)
	}
	defer tx.Rollback(ctx)

	replacedObjects, err := userAvatarObjectsForReplacement(ctx, tx, input.OwnerUserID)
	if err != nil {
		return media.UploadedImage{}, err
	}

	var image media.UploadedImage
	image.Kind = "user_avatar"
	image.UserID = &input.OwnerUserID
	err = tx.QueryRow(ctx, `
INSERT INTO user_images (
  user_id, bucket, storage_key, original_filename, alt_text, width, height,
  file_size, file_hash_sha256, mime_type_detected, processing_status
)
VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'pending')
RETURNING id::text, alt_text, width, height, file_size, file_hash_sha256, mime_type_detected, processing_status, created_at`,
		input.OwnerUserID,
		input.Bucket,
		input.StorageKey,
		input.OriginalFilename,
		input.AltText,
		input.Width,
		input.Height,
		input.FileSize,
		input.FileHashSHA256,
		input.MimeTypeDetected,
	).Scan(
		&image.ID,
		&image.AltText,
		&image.Width,
		&image.Height,
		&image.FileSize,
		&image.FileHashSHA256,
		&image.MimeTypeDetected,
		&image.ProcessingStatus,
		&image.CreatedAt,
	)
	if err != nil {
		return media.UploadedImage{}, fmt.Errorf("insert user avatar: %w", err)
	}

	_, err = tx.Exec(ctx, `UPDATE users SET avatar_image_id = $2::uuid, updated_at = now() WHERE id = $1::uuid`, input.OwnerUserID, image.ID)
	if err != nil {
		return media.UploadedImage{}, fmt.Errorf("attach user avatar: %w", err)
	}

	if len(replacedObjects) > 0 {
		if err := softDeleteReplacedUserAvatars(ctx, tx, input.OwnerUserID, image.ID); err != nil {
			return media.UploadedImage{}, err
		}
	}

	if err := enqueueImageJob(ctx, tx, "user_avatar", image.ID); err != nil {
		return media.UploadedImage{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return media.UploadedImage{}, fmt.Errorf("commit avatar upload: %w", err)
	}
	image.ReplacedObjects = replacedObjects
	return image, nil
}

func (r MediaRepository) CreateContentImage(ctx context.Context, input media.ImageUploadInput) (media.UploadedImage, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return media.UploadedImage{}, fmt.Errorf("begin content image upload: %w", err)
	}
	defer tx.Rollback(ctx)

	var ownsContent bool
	err = tx.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1 FROM content_items
  WHERE id = $1::uuid
    AND author_id = $2::uuid
    AND deleted_at IS NULL
    AND status <> 'deleted'
)`, input.ContentID, input.OwnerUserID).Scan(&ownsContent)
	if err != nil {
		return media.UploadedImage{}, fmt.Errorf("check content ownership: %w", err)
	}
	if !ownsContent {
		return media.UploadedImage{}, pgx.ErrNoRows
	}

	if input.IsPrimary {
		_, err = tx.Exec(ctx, `UPDATE content_images SET is_primary = false, updated_at = now() WHERE content_id = $1::uuid AND is_primary = true`, input.ContentID)
		if err != nil {
			return media.UploadedImage{}, fmt.Errorf("clear previous primary image: %w", err)
		}
	}

	var image media.UploadedImage
	image.Kind = "content_image"
	image.ContentID = &input.ContentID
	err = tx.QueryRow(ctx, `
INSERT INTO content_images (
  content_id, bucket, storage_key, original_filename, alt_text, width, height,
  file_size, file_hash_sha256, mime_type_detected, sort_order, is_primary,
  processing_status, uploaded_by
)
VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, 'pending', $13::uuid)
RETURNING id::text, alt_text, width, height, file_size, file_hash_sha256, mime_type_detected, processing_status, is_primary, sort_order, created_at`,
		input.ContentID,
		input.Bucket,
		input.StorageKey,
		input.OriginalFilename,
		input.AltText,
		input.Width,
		input.Height,
		input.FileSize,
		input.FileHashSHA256,
		input.MimeTypeDetected,
		input.SortOrder,
		input.IsPrimary,
		input.OwnerUserID,
	).Scan(
		&image.ID,
		&image.AltText,
		&image.Width,
		&image.Height,
		&image.FileSize,
		&image.FileHashSHA256,
		&image.MimeTypeDetected,
		&image.ProcessingStatus,
		&image.IsPrimary,
		&image.SortOrder,
		&image.CreatedAt,
	)
	if err != nil {
		return media.UploadedImage{}, fmt.Errorf("insert content image: %w", err)
	}

	if err := enqueueImageJob(ctx, tx, "content_image", image.ID); err != nil {
		return media.UploadedImage{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return media.UploadedImage{}, fmt.Errorf("commit content image upload: %w", err)
	}
	return image, nil
}

func (r MediaRepository) ListOwnedContentImages(ctx context.Context, ownerUserID string, contentID string) ([]media.UploadedImage, error) {
	if err := r.ensureContentOwner(ctx, ownerUserID, contentID); err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, `
SELECT id::text, alt_text, width, height, file_size, file_hash_sha256, mime_type_detected, processing_status, is_primary, sort_order, created_at
FROM content_images
WHERE content_id = $1::uuid
  AND deleted_at IS NULL
ORDER BY is_primary DESC, sort_order ASC, created_at ASC`, contentID)
	if err != nil {
		return nil, fmt.Errorf("query owned content images: %w", err)
	}
	defer rows.Close()

	images := make([]media.UploadedImage, 0)
	for rows.Next() {
		image := media.UploadedImage{
			Kind:      "content_image",
			ContentID: &contentID,
		}
		err := rows.Scan(
			&image.ID,
			&image.AltText,
			&image.Width,
			&image.Height,
			&image.FileSize,
			&image.FileHashSHA256,
			&image.MimeTypeDetected,
			&image.ProcessingStatus,
			&image.IsPrimary,
			&image.SortOrder,
			&image.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan owned content image: %w", err)
		}
		image.URL = "/api/v1/media/" + image.ID
		images = append(images, image)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read owned content images: %w", err)
	}
	return images, nil
}

func (r MediaRepository) GetOwnedObject(ctx context.Context, ownerUserID string, imageID string) (media.Object, error) {
	var object media.Object
	err := r.db.QueryRow(ctx, `
SELECT id::text, bucket, storage_key, mime_type_detected, file_size
FROM (
  SELECT id, bucket, storage_key, mime_type_detected, file_size
  FROM user_images
  WHERE id = $1::uuid
    AND user_id = $2::uuid
    AND processing_status = 'processed'
    AND deleted_at IS NULL
  UNION ALL
  SELECT image.id, image.bucket, image.storage_key, image.mime_type_detected, image.file_size
  FROM content_images image
  JOIN content_items content ON content.id = image.content_id
  WHERE image.id = $1::uuid
    AND content.author_id = $2::uuid
    AND image.processing_status = 'processed'
    AND image.deleted_at IS NULL
    AND content.deleted_at IS NULL
) owned_image
LIMIT 1`, imageID, ownerUserID).Scan(
		&object.ID,
		&object.Bucket,
		&object.StorageKey,
		&object.ContentType,
		&object.FileSize,
	)
	if err != nil {
		return media.Object{}, err
	}
	return object, nil
}

func (r MediaRepository) UpdateOwnedContentImage(ctx context.Context, input media.ContentImageUpdate) (media.UploadedImage, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return media.UploadedImage{}, fmt.Errorf("begin update content image: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := ensureContentOwnerTx(ctx, tx, input.OwnerUserID, input.ContentID); err != nil {
		return media.UploadedImage{}, err
	}
	if input.IsPrimary != nil && *input.IsPrimary {
		_, err = tx.Exec(ctx, `
UPDATE content_images
SET is_primary = false, updated_at = now()
WHERE content_id = $1::uuid AND id <> $2::uuid AND is_primary = true AND deleted_at IS NULL`, input.ContentID, input.ImageID)
		if err != nil {
			return media.UploadedImage{}, fmt.Errorf("clear previous primary image: %w", err)
		}
	}

	var image media.UploadedImage
	image.Kind = "content_image"
	image.ContentID = &input.ContentID
	err = tx.QueryRow(ctx, `
UPDATE content_images
SET
  alt_text = COALESCE($3, alt_text),
  is_primary = COALESCE($4, is_primary),
  sort_order = COALESCE($5, sort_order),
  updated_at = now()
WHERE id = $1::uuid
  AND content_id = $2::uuid
  AND deleted_at IS NULL
RETURNING id::text, alt_text, width, height, file_size, file_hash_sha256, mime_type_detected, processing_status, is_primary, sort_order, created_at`,
		input.ImageID,
		input.ContentID,
		input.AltText,
		input.IsPrimary,
		input.SortOrder,
	).Scan(
		&image.ID,
		&image.AltText,
		&image.Width,
		&image.Height,
		&image.FileSize,
		&image.FileHashSHA256,
		&image.MimeTypeDetected,
		&image.ProcessingStatus,
		&image.IsPrimary,
		&image.SortOrder,
		&image.CreatedAt,
	)
	if err != nil {
		return media.UploadedImage{}, err
	}
	if err := enqueueSearchIndexTx(ctx, tx, input.ContentID, "upsert_content"); err != nil {
		return media.UploadedImage{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return media.UploadedImage{}, fmt.Errorf("commit update content image: %w", err)
	}
	image.URL = "/api/v1/media/" + image.ID
	return image, nil
}

func (r MediaRepository) DeleteOwnedContentImage(ctx context.Context, ownerUserID string, contentID string, imageID string) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin delete content image: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := ensureContentOwnerTx(ctx, tx, ownerUserID, contentID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
UPDATE content_images
SET deleted_at = COALESCE(deleted_at, now()), is_primary = false, updated_at = now()
WHERE id = $1::uuid
  AND content_id = $2::uuid
  AND deleted_at IS NULL`, imageID, contentID)
	if err != nil {
		return fmt.Errorf("delete content image: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	if err := enqueueSearchIndexTx(ctx, tx, contentID, "upsert_content"); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete content image: %w", err)
	}
	return nil
}

func (r MediaRepository) GetPublicObject(ctx context.Context, imageID string) (media.Object, error) {
	var object media.Object
	err := r.db.QueryRow(ctx, `
SELECT id, bucket, storage_key, mime_type_detected, file_size
FROM (
  SELECT ui.id::text AS id, ui.bucket, ui.storage_key, ui.mime_type_detected, ui.file_size
  FROM user_images ui
  WHERE ui.id = $1::uuid AND ui.processing_status = 'processed' AND ui.deleted_at IS NULL
  UNION ALL
  SELECT ci.id::text AS id, ci.bucket, ci.storage_key, ci.mime_type_detected, ci.file_size
  FROM content_images ci
  JOIN content_items item ON item.id = ci.content_id
  WHERE ci.id = $1::uuid
    AND ci.processing_status = 'processed'
    AND ci.deleted_at IS NULL
    AND item.status = 'published'
    AND item.visibility = 'public'
    AND item.deleted_at IS NULL
    AND item.hidden_at IS NULL
) image
LIMIT 1`, imageID).Scan(&object.ID, &object.Bucket, &object.StorageKey, &object.ContentType, &object.FileSize)
	if err != nil {
		return media.Object{}, err
	}
	return object, nil
}

func (r MediaRepository) ensureContentOwner(ctx context.Context, ownerUserID string, contentID string) error {
	var ownsContent bool
	err := r.db.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1 FROM content_items
  WHERE id = $1::uuid
    AND author_id = $2::uuid
    AND deleted_at IS NULL
    AND status <> 'deleted'
)`, contentID, ownerUserID).Scan(&ownsContent)
	if err != nil {
		return fmt.Errorf("check content ownership: %w", err)
	}
	if !ownsContent {
		return pgx.ErrNoRows
	}
	return nil
}

func ensureContentOwnerTx(ctx context.Context, tx pgx.Tx, ownerUserID string, contentID string) error {
	var ownsContent bool
	err := tx.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1 FROM content_items
  WHERE id = $1::uuid
    AND author_id = $2::uuid
    AND deleted_at IS NULL
    AND status <> 'deleted'
  FOR UPDATE
)`, contentID, ownerUserID).Scan(&ownsContent)
	if err != nil {
		return fmt.Errorf("check content ownership: %w", err)
	}
	if !ownsContent {
		return pgx.ErrNoRows
	}
	return nil
}

func (r MediaRepository) GetImageForProcessing(ctx context.Context, kind string, imageID string) (media.ImageForProcessing, error) {
	var image media.ImageForProcessing
	switch kind {
	case "user_avatar":
		err := r.db.QueryRow(ctx, `
UPDATE user_images
SET processing_status = 'running', updated_at = now()
WHERE id = $1::uuid AND processing_status IN ('pending', 'running') AND deleted_at IS NULL
RETURNING id::text, 'user_avatar', user_id::text, '', bucket, storage_key, original_filename, mime_type_detected`, imageID).Scan(
			&image.ID, &image.Kind, &image.UserID, &image.ContentID, &image.Bucket, &image.StorageKey, &image.OriginalFilename, &image.MimeTypeDetected,
		)
		if err != nil {
			return media.ImageForProcessing{}, err
		}
	case "content_image":
		err := r.db.QueryRow(ctx, `
UPDATE content_images
SET processing_status = 'running', updated_at = now()
WHERE id = $1::uuid AND processing_status IN ('pending', 'running')
RETURNING id::text, 'content_image', COALESCE(uploaded_by::text, ''), content_id::text, bucket, storage_key, original_filename, mime_type_detected`, imageID).Scan(
			&image.ID, &image.Kind, &image.UserID, &image.ContentID, &image.Bucket, &image.StorageKey, &image.OriginalFilename, &image.MimeTypeDetected,
		)
		if err != nil {
			return media.ImageForProcessing{}, err
		}
	default:
		return media.ImageForProcessing{}, fmt.Errorf("unsupported image kind %q", kind)
	}
	return image, nil
}

func (r MediaRepository) CompleteImageProcessing(ctx context.Context, kind string, imageID string, bucket string, storageKey string, width int, height int, fileSize int64, hash string, mimeType string) error {
	var tag pgconn.CommandTag
	var err error
	switch kind {
	case "user_avatar":
		tag, err = r.db.Exec(ctx, `
UPDATE user_images
SET bucket = $2, storage_key = $3, width = $4, height = $5, file_size = $6,
    file_hash_sha256 = $7, mime_type_detected = $8, processing_status = 'processed', updated_at = now()
WHERE id = $1::uuid AND deleted_at IS NULL`, imageID, bucket, storageKey, width, height, fileSize, hash, mimeType)
	case "content_image":
		tag, err = r.db.Exec(ctx, `
UPDATE content_images
SET bucket = $2, storage_key = $3, width = $4, height = $5, file_size = $6,
    file_hash_sha256 = $7, mime_type_detected = $8, processing_status = 'processed', updated_at = now()
WHERE id = $1::uuid`, imageID, bucket, storageKey, width, height, fileSize, hash, mimeType)
	default:
		return fmt.Errorf("unsupported image kind %q", kind)
	}
	if err != nil {
		return fmt.Errorf("complete image processing: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r MediaRepository) FailImageProcessing(ctx context.Context, kind string, imageID string) error {
	var tag pgconn.CommandTag
	var err error
	switch kind {
	case "user_avatar":
		tag, err = r.db.Exec(ctx, `UPDATE user_images SET processing_status = 'failed', updated_at = now() WHERE id = $1::uuid AND deleted_at IS NULL`, imageID)
	case "content_image":
		tag, err = r.db.Exec(ctx, `UPDATE content_images SET processing_status = 'failed', updated_at = now() WHERE id = $1::uuid`, imageID)
	default:
		return fmt.Errorf("unsupported image kind %q", kind)
	}
	if err != nil {
		return fmt.Errorf("fail image processing: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func userAvatarObjectsForReplacement(ctx context.Context, tx pgx.Tx, userID string) ([]media.Object, error) {
	rows, err := tx.Query(ctx, `
SELECT id::text, bucket, storage_key, mime_type_detected, file_size
FROM user_images
WHERE user_id = $1::uuid
  AND deleted_at IS NULL
FOR UPDATE`, userID)
	if err != nil {
		return nil, fmt.Errorf("query replaced avatar objects: %w", err)
	}
	defer rows.Close()

	objects := make([]media.Object, 0)
	for rows.Next() {
		var object media.Object
		if err := rows.Scan(&object.ID, &object.Bucket, &object.StorageKey, &object.ContentType, &object.FileSize); err != nil {
			return nil, fmt.Errorf("scan replaced avatar object: %w", err)
		}
		objects = append(objects, object)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read replaced avatar objects: %w", err)
	}
	return objects, nil
}

func softDeleteReplacedUserAvatars(ctx context.Context, tx pgx.Tx, userID string, keepImageID string) error {
	_, err := tx.Exec(ctx, `
UPDATE user_images
SET deleted_at = COALESCE(deleted_at, now()), updated_at = now()
WHERE user_id = $1::uuid
  AND id <> $2::uuid
  AND deleted_at IS NULL`, userID, keepImageID)
	if err != nil {
		return fmt.Errorf("soft delete replaced avatars: %w", err)
	}

	_, err = tx.Exec(ctx, `
UPDATE worker_jobs
SET status = 'succeeded', locked_by = NULL, locked_at = NULL, last_error = NULL, updated_at = now()
WHERE queue_name = 'image_processing_queue'
  AND job_type = 'process_image'
  AND status = 'pending'
  AND payload->>'kind' = 'user_avatar'
  AND EXISTS (
    SELECT 1
    FROM user_images ui
    WHERE ui.id::text = worker_jobs.payload->>'image_id'
      AND ui.user_id = $1::uuid
      AND ui.id <> $2::uuid
      AND ui.deleted_at IS NOT NULL
  )`, userID, keepImageID)
	if err != nil {
		return fmt.Errorf("cancel replaced avatar jobs: %w", err)
	}
	return nil
}

func enqueueImageJob(ctx context.Context, tx pgx.Tx, kind string, imageID string) error {
	payload, err := json.Marshal(map[string]string{
		"kind":     kind,
		"image_id": imageID,
	})
	if err != nil {
		return fmt.Errorf("marshal image job payload: %w", err)
	}
	_, err = tx.Exec(ctx, `
INSERT INTO worker_jobs (queue_name, job_type, payload)
VALUES ('image_processing_queue', 'process_image', $1::jsonb)`, string(payload))
	if err != nil {
		return fmt.Errorf("insert image worker job: %w", err)
	}
	return nil
}
