CREATE TABLE password_change_tokens (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  email text NOT NULL,
  token_hash text NOT NULL UNIQUE,
  password_hash text NOT NULL,
  expires_at timestamptz NOT NULL,
  used_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT password_change_tokens_email_normalized CHECK (email = lower(email)),
  CONSTRAINT password_change_tokens_email_not_blank CHECK (length(trim(email)) > 0),
  CONSTRAINT password_change_tokens_hash_format CHECK (token_hash ~ '^[a-f0-9]{64}$'),
  CONSTRAINT password_change_tokens_password_hash_not_blank CHECK (length(trim(password_hash)) > 0),
  CONSTRAINT password_change_tokens_expiry_valid CHECK (expires_at > created_at)
);

CREATE INDEX password_change_tokens_user_id_idx ON password_change_tokens (user_id);
CREATE INDEX password_change_tokens_pending_idx ON password_change_tokens (user_id, expires_at)
  WHERE used_at IS NULL;

CREATE TABLE email_change_tokens (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  new_email text NOT NULL,
  token_hash text NOT NULL UNIQUE,
  expires_at timestamptz NOT NULL,
  used_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT email_change_tokens_new_email_normalized CHECK (new_email = lower(new_email)),
  CONSTRAINT email_change_tokens_new_email_not_blank CHECK (length(trim(new_email)) > 0),
  CONSTRAINT email_change_tokens_hash_format CHECK (token_hash ~ '^[a-f0-9]{64}$'),
  CONSTRAINT email_change_tokens_expiry_valid CHECK (expires_at > created_at)
);

CREATE INDEX email_change_tokens_user_id_idx ON email_change_tokens (user_id);
CREATE INDEX email_change_tokens_pending_idx ON email_change_tokens (user_id, expires_at)
  WHERE used_at IS NULL;
CREATE INDEX email_change_tokens_pending_new_email_idx ON email_change_tokens (new_email, expires_at)
  WHERE used_at IS NULL;
