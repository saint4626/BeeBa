CREATE TABLE server_listings (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_id uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  name text NOT NULL,
  slug text NOT NULL,
  description text NOT NULL DEFAULT '',
  host text NOT NULL,
  port integer NOT NULL DEFAULT 4296,
  password_ciphertext text,
  visibility text NOT NULL DEFAULT 'public',
  status text NOT NULL DEFAULT 'pending_verification',
  region text NOT NULL DEFAULT '',
  language text NOT NULL DEFAULT '',
  nsfw boolean NOT NULL DEFAULT false,
  rules text NOT NULL DEFAULT '',
  discord_url text,
  website_url text,
  check_status text NOT NULL DEFAULT 'pending',
  check_online_players integer,
  check_max_players integer,
  check_protocol_version integer,
  check_server_name text,
  check_motd text,
  check_round_trip_ms integer,
  check_error text,
  last_checked_at timestamptz,
  published_at timestamptz,
  hidden_at timestamptz,
  deleted_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT server_listings_name_not_blank CHECK (length(trim(name)) > 0),
  CONSTRAINT server_listings_slug_not_blank CHECK (length(trim(slug)) > 0),
  CONSTRAINT server_listings_host_not_blank CHECK (length(trim(host)) > 0),
  CONSTRAINT server_listings_port_range CHECK (port BETWEEN 1 AND 65535),
  CONSTRAINT server_listings_visibility_allowed CHECK (visibility IN ('public', 'unlisted')),
  CONSTRAINT server_listings_status_allowed CHECK (status IN (
    'draft',
    'pending_verification',
    'pending_moderation',
    'published',
    'offline',
    'failed',
    'hidden',
    'deleted'
  )),
  CONSTRAINT server_listings_check_status_allowed CHECK (check_status IN (
    'pending',
    'running',
    'online',
    'offline',
    'failed'
  )),
  CONSTRAINT server_listings_check_counts_nonnegative CHECK (
    (check_online_players IS NULL OR check_online_players >= 0) AND
    (check_max_players IS NULL OR check_max_players >= 0) AND
    (check_round_trip_ms IS NULL OR check_round_trip_ms >= 0)
  )
);

CREATE UNIQUE INDEX server_listings_owner_slug_unique_active
  ON server_listings (owner_id, slug)
  WHERE deleted_at IS NULL;
CREATE INDEX server_listings_owner_id_idx ON server_listings (owner_id);
CREATE INDEX server_listings_status_visibility_idx ON server_listings (status, visibility);
CREATE INDEX server_listings_check_status_idx ON server_listings (check_status);
CREATE INDEX server_listings_region_idx ON server_listings (lower(region)) WHERE deleted_at IS NULL;
CREATE INDEX server_listings_language_idx ON server_listings (lower(language)) WHERE deleted_at IS NULL;
CREATE INDEX server_listings_published_at_idx ON server_listings (published_at DESC, id DESC);
CREATE INDEX server_listings_updated_at_idx ON server_listings (updated_at DESC, id DESC);
CREATE INDEX server_listings_online_players_idx ON server_listings (check_online_players DESC NULLS LAST, id DESC);

CREATE TABLE server_checks (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  server_id uuid NOT NULL REFERENCES server_listings (id) ON DELETE CASCADE,
  status text NOT NULL,
  online_players integer,
  max_players integer,
  protocol_version integer,
  server_name text,
  motd text,
  round_trip_ms integer,
  error text,
  checked_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT server_checks_status_allowed CHECK (status IN ('pending', 'running', 'online', 'offline', 'failed')),
  CONSTRAINT server_checks_counts_nonnegative CHECK (
    (online_players IS NULL OR online_players >= 0) AND
    (max_players IS NULL OR max_players >= 0) AND
    (round_trip_ms IS NULL OR round_trip_ms >= 0)
  )
);

CREATE INDEX server_checks_server_checked_idx ON server_checks (server_id, checked_at DESC);
CREATE INDEX server_checks_status_idx ON server_checks (status);

CREATE TABLE server_tags (
  server_id uuid NOT NULL REFERENCES server_listings (id) ON DELETE CASCADE,
  tag_id uuid NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (server_id, tag_id)
);

CREATE INDEX server_tags_tag_id_idx ON server_tags (tag_id);

ALTER TABLE worker_jobs DROP CONSTRAINT worker_jobs_queue_name_allowed;
ALTER TABLE worker_jobs ADD CONSTRAINT worker_jobs_queue_name_allowed CHECK (queue_name IN (
  'file_scan_queue',
  'image_processing_queue',
  'search_index_queue',
  'email_queue',
  'moderation_queue',
  'cleanup_queue',
  'server_check_queue'
));
