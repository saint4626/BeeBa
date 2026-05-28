CREATE TABLE auth_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  access_token_hash text NOT NULL UNIQUE,
  refresh_token_hash text NOT NULL UNIQUE,
  user_agent text,
  ip_address inet,
  access_expires_at timestamptz NOT NULL,
  refresh_expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT auth_sessions_access_hash_format CHECK (access_token_hash ~ '^[a-f0-9]{64}$'),
  CONSTRAINT auth_sessions_refresh_hash_format CHECK (refresh_token_hash ~ '^[a-f0-9]{64}$'),
  CONSTRAINT auth_sessions_expiry_order CHECK (refresh_expires_at > access_expires_at)
);

CREATE INDEX auth_sessions_user_id_idx ON auth_sessions (user_id);
CREATE INDEX auth_sessions_access_token_hash_idx ON auth_sessions (access_token_hash);
CREATE INDEX auth_sessions_refresh_token_hash_idx ON auth_sessions (refresh_token_hash);
CREATE INDEX auth_sessions_active_user_idx
  ON auth_sessions (user_id, refresh_expires_at DESC)
  WHERE revoked_at IS NULL;
