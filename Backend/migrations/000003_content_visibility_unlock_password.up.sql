ALTER TABLE content_items
  ADD COLUMN visibility text NOT NULL DEFAULT 'public',
  ADD CONSTRAINT content_items_visibility_allowed CHECK (visibility IN ('public', 'private'));

CREATE INDEX content_items_visibility_idx ON content_items (visibility);

ALTER TABLE content_files
  ADD COLUMN unlock_password_ciphertext text,
  ADD CONSTRAINT content_files_unlock_password_ciphertext_not_blank CHECK (
    unlock_password_ciphertext IS NULL OR length(trim(unlock_password_ciphertext)) > 0
  );
