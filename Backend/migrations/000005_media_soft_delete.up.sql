ALTER TABLE content_images
  ADD COLUMN deleted_at timestamptz;

ALTER TABLE user_images
  ADD COLUMN deleted_at timestamptz;

DROP INDEX IF EXISTS content_images_one_primary_per_content;
CREATE UNIQUE INDEX content_images_one_primary_per_content
  ON content_images (content_id)
  WHERE is_primary AND deleted_at IS NULL;

CREATE INDEX content_images_deleted_at_idx ON content_images (deleted_at);
CREATE INDEX user_images_deleted_at_idx ON user_images (deleted_at);
