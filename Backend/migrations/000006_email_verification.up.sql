CREATE TABLE email_verification_tokens (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  email text NOT NULL,
  token_hash text NOT NULL UNIQUE,
  expires_at timestamptz NOT NULL,
  used_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT email_verification_tokens_email_normalized CHECK (email = lower(email)),
  CONSTRAINT email_verification_tokens_hash_format CHECK (token_hash ~ '^[a-f0-9]{64}$'),
  CONSTRAINT email_verification_tokens_expiry_valid CHECK (expires_at > created_at)
);

CREATE INDEX email_verification_tokens_user_id_idx ON email_verification_tokens (user_id);
CREATE INDEX email_verification_tokens_pending_idx ON email_verification_tokens (user_id, expires_at)
  WHERE used_at IS NULL;
