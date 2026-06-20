ALTER TABLE content_items
  ADD COLUMN unlisted_download_token_hash text,
  ADD COLUMN unlisted_download_token_ciphertext text,
  ADD COLUMN unlisted_download_token_created_at timestamptz,
  ADD CONSTRAINT content_items_unlisted_download_token_hash_format CHECK (
    unlisted_download_token_hash IS NULL OR unlisted_download_token_hash ~ '^[a-f0-9]{64}$'
  ),
  ADD CONSTRAINT content_items_unlisted_download_token_ciphertext_not_blank CHECK (
    unlisted_download_token_ciphertext IS NULL OR length(trim(unlisted_download_token_ciphertext)) > 0
  ),
  ADD CONSTRAINT content_items_unlisted_download_token_pair CHECK (
    (
      unlisted_download_token_hash IS NULL
      AND unlisted_download_token_ciphertext IS NULL
      AND unlisted_download_token_created_at IS NULL
    )
    OR
    (
      unlisted_download_token_hash IS NOT NULL
      AND unlisted_download_token_ciphertext IS NOT NULL
      AND unlisted_download_token_created_at IS NOT NULL
    )
  );

CREATE UNIQUE INDEX content_items_unlisted_download_token_hash_unique
  ON content_items (unlisted_download_token_hash)
  WHERE unlisted_download_token_hash IS NOT NULL;
