DROP INDEX IF EXISTS content_images_sort_idx;
DROP INDEX IF EXISTS content_images_processing_status_idx;
DROP INDEX IF EXISTS user_images_processing_status_idx;
DROP INDEX IF EXISTS user_images_user_id_idx;
DROP INDEX IF EXISTS user_images_storage_object_unique;

ALTER TABLE content_images
  DROP CONSTRAINT IF EXISTS content_images_hash_sha256_format,
  DROP COLUMN IF EXISTS uploaded_by,
  DROP COLUMN IF EXISTS mime_type_detected,
  DROP COLUMN IF EXISTS file_hash_sha256;

ALTER TABLE users
  DROP CONSTRAINT IF EXISTS users_avatar_image_id_fkey,
  DROP COLUMN IF EXISTS avatar_image_id;

DROP TABLE IF EXISTS user_images;
