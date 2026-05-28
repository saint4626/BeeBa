ALTER TABLE users
  ADD COLUMN avatar_image_id uuid;

CREATE TABLE user_images (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  bucket text NOT NULL,
  storage_key text NOT NULL,
  original_filename text NOT NULL,
  alt_text text NOT NULL DEFAULT '',
  width integer NOT NULL,
  height integer NOT NULL,
  file_size bigint NOT NULL,
  file_hash_sha256 text NOT NULL,
  mime_type_detected text NOT NULL,
  processing_status text NOT NULL DEFAULT 'pending',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT user_images_dimensions_positive CHECK (width > 0 AND height > 0),
  CONSTRAINT user_images_file_size_positive CHECK (file_size > 0),
  CONSTRAINT user_images_hash_sha256_format CHECK (file_hash_sha256 ~ '^[a-f0-9]{64}$'),
  CONSTRAINT user_images_processing_status_allowed CHECK (
    processing_status IN ('pending', 'running', 'processed', 'failed')
  )
);

ALTER TABLE users
  ADD CONSTRAINT users_avatar_image_id_fkey
  FOREIGN KEY (avatar_image_id) REFERENCES user_images (id) ON DELETE SET NULL;

ALTER TABLE content_images
  ADD COLUMN file_hash_sha256 text NOT NULL DEFAULT repeat('0', 64),
  ADD COLUMN mime_type_detected text NOT NULL DEFAULT 'application/octet-stream',
  ADD COLUMN uploaded_by uuid REFERENCES users (id) ON DELETE SET NULL;

ALTER TABLE content_images
  ADD CONSTRAINT content_images_hash_sha256_format CHECK (file_hash_sha256 ~ '^[a-f0-9]{64}$');

CREATE UNIQUE INDEX user_images_storage_object_unique ON user_images (bucket, storage_key);
CREATE INDEX user_images_user_id_idx ON user_images (user_id);
CREATE INDEX user_images_processing_status_idx ON user_images (processing_status);
CREATE INDEX content_images_processing_status_idx ON content_images (processing_status);
CREATE INDEX content_images_sort_idx ON content_images (content_id, is_primary DESC, sort_order, created_at);
