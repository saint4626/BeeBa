ALTER TABLE content_files
  DROP CONSTRAINT IF EXISTS content_files_unlock_password_ciphertext_not_blank,
  DROP COLUMN IF EXISTS unlock_password_ciphertext;

DROP INDEX IF EXISTS content_items_visibility_idx;

ALTER TABLE content_items
  DROP CONSTRAINT IF EXISTS content_items_visibility_allowed,
  DROP COLUMN IF EXISTS visibility;
