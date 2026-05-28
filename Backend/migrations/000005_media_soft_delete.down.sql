DROP INDEX IF EXISTS user_images_deleted_at_idx;
DROP INDEX IF EXISTS content_images_deleted_at_idx;

DROP INDEX IF EXISTS content_images_one_primary_per_content;
CREATE UNIQUE INDEX content_images_one_primary_per_content
  ON content_images (content_id)
  WHERE is_primary;

ALTER TABLE user_images
  DROP COLUMN IF EXISTS deleted_at;

ALTER TABLE content_images
  DROP COLUMN IF EXISTS deleted_at;
