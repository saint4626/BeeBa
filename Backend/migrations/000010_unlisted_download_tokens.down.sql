DROP INDEX IF EXISTS content_items_unlisted_download_token_hash_unique;

ALTER TABLE content_items
  DROP CONSTRAINT IF EXISTS content_items_unlisted_download_token_pair,
  DROP CONSTRAINT IF EXISTS content_items_unlisted_download_token_ciphertext_not_blank,
  DROP CONSTRAINT IF EXISTS content_items_unlisted_download_token_hash_format,
  DROP COLUMN IF EXISTS unlisted_download_token_created_at,
  DROP COLUMN IF EXISTS unlisted_download_token_ciphertext,
  DROP COLUMN IF EXISTS unlisted_download_token_hash;
