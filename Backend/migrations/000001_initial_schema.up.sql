CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email text NOT NULL,
  username text NOT NULL,
  display_name text,
  password_hash text NOT NULL,
  email_verified_at timestamptz,
  banned_at timestamptz,
  deleted_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT users_email_normalized CHECK (email = lower(email)),
  CONSTRAINT users_email_not_blank CHECK (length(trim(email)) > 0),
  CONSTRAINT users_username_not_blank CHECK (length(trim(username)) > 0)
);

CREATE UNIQUE INDEX users_email_unique_active ON users (email) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX users_username_unique_active ON users (lower(username)) WHERE deleted_at IS NULL;

CREATE TABLE roles (
  id smallserial PRIMARY KEY,
  slug text NOT NULL UNIQUE,
  name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT roles_slug_allowed CHECK (slug IN ('user', 'moderator', 'admin', 'owner'))
);

INSERT INTO roles (slug, name)
VALUES
  ('user', 'User'),
  ('moderator', 'Moderator'),
  ('admin', 'Admin'),
  ('owner', 'Owner');

CREATE TABLE user_roles (
  user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  role_id smallint NOT NULL REFERENCES roles (id) ON DELETE RESTRICT,
  granted_by uuid REFERENCES users (id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, role_id)
);

CREATE TABLE categories (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  slug text NOT NULL UNIQUE,
  name text NOT NULL,
  description text NOT NULL DEFAULT '',
  icon_key text,
  sort_order integer NOT NULL DEFAULT 0,
  is_active boolean NOT NULL DEFAULT true,
  published_count integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT categories_slug_not_blank CHECK (length(trim(slug)) > 0),
  CONSTRAINT categories_name_not_blank CHECK (length(trim(name)) > 0),
  CONSTRAINT categories_published_count_nonnegative CHECK (published_count >= 0)
);

INSERT INTO categories (slug, name, description, sort_order)
VALUES
  ('worlds', 'Worlds', 'Basis worlds and scenes for VR exploration.', 10),
  ('avatars', 'Avatars', 'Basis-ready avatars and character packages.', 20),
  ('props', 'Props', 'Reusable props and interactive objects.', 30),
  ('prefabs', 'Prefabs', 'Developer-friendly prefabs, templates, and integration assets.', 40);

CREATE TABLE content_items (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  author_id uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  category_id uuid NOT NULL REFERENCES categories (id) ON DELETE RESTRICT,
  title text NOT NULL,
  slug text NOT NULL,
  description text NOT NULL DEFAULT '',
  status text NOT NULL DEFAULT 'draft',
  nsfw boolean NOT NULL DEFAULT false,
  github_url text,
  documentation_url text,
  version text,
  changelog text,
  likes_count integer NOT NULL DEFAULT 0,
  downloads_count integer NOT NULL DEFAULT 0,
  comments_count integer NOT NULL DEFAULT 0,
  published_at timestamptz,
  hidden_at timestamptz,
  deleted_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT content_items_title_not_blank CHECK (length(trim(title)) > 0),
  CONSTRAINT content_items_slug_not_blank CHECK (length(trim(slug)) > 0),
  CONSTRAINT content_items_status_allowed CHECK (status IN (
    'draft',
    'uploaded',
    'pending_scan',
    'scan_failed',
    'pending_moderation',
    'approved',
    'published',
    'rejected',
    'hidden',
    'deleted'
  )),
  CONSTRAINT content_items_counts_nonnegative CHECK (
    likes_count >= 0 AND downloads_count >= 0 AND comments_count >= 0
  )
);

CREATE UNIQUE INDEX content_items_category_slug_unique_active
  ON content_items (category_id, slug)
  WHERE deleted_at IS NULL;
CREATE INDEX content_items_author_id_idx ON content_items (author_id);
CREATE INDEX content_items_category_id_idx ON content_items (category_id);
CREATE INDEX content_items_status_idx ON content_items (status);
CREATE INDEX content_items_nsfw_idx ON content_items (nsfw);
CREATE INDEX content_items_created_at_idx ON content_items (created_at DESC);
CREATE INDEX content_items_published_at_idx ON content_items (published_at DESC);
CREATE INDEX content_items_likes_count_idx ON content_items (likes_count DESC);
CREATE INDEX content_items_downloads_count_idx ON content_items (downloads_count DESC);

CREATE TABLE content_files (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  content_id uuid NOT NULL REFERENCES content_items (id) ON DELETE CASCADE,
  bucket text NOT NULL,
  storage_key text NOT NULL,
  original_filename text NOT NULL,
  safe_filename text NOT NULL,
  file_size bigint NOT NULL,
  file_hash_sha256 text NOT NULL,
  mime_type_detected text,
  extension text NOT NULL,
  scan_status text NOT NULL DEFAULT 'pending',
  scan_result jsonb NOT NULL DEFAULT '{}'::jsonb,
  uploaded_by uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT content_files_file_size_positive CHECK (file_size > 0),
  CONSTRAINT content_files_extension_bee CHECK (extension = '.bee'),
  CONSTRAINT content_files_hash_sha256_format CHECK (file_hash_sha256 ~ '^[a-f0-9]{64}$'),
  CONSTRAINT content_files_scan_status_allowed CHECK (scan_status IN (
    'pending',
    'running',
    'clean',
    'suspicious',
    'infected',
    'failed',
    'skipped'
  ))
);

CREATE UNIQUE INDEX content_files_storage_object_unique ON content_files (bucket, storage_key);
CREATE INDEX content_files_content_id_idx ON content_files (content_id);
CREATE INDEX content_files_file_hash_sha256_idx ON content_files (file_hash_sha256);
CREATE INDEX content_files_scan_status_idx ON content_files (scan_status);

CREATE TABLE content_images (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  content_id uuid NOT NULL REFERENCES content_items (id) ON DELETE CASCADE,
  bucket text NOT NULL,
  storage_key text NOT NULL,
  original_filename text NOT NULL,
  alt_text text NOT NULL DEFAULT '',
  width integer NOT NULL,
  height integer NOT NULL,
  file_size bigint NOT NULL,
  sort_order integer NOT NULL DEFAULT 0,
  is_primary boolean NOT NULL DEFAULT false,
  processing_status text NOT NULL DEFAULT 'pending',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT content_images_dimensions_positive CHECK (width > 0 AND height > 0),
  CONSTRAINT content_images_file_size_positive CHECK (file_size > 0),
  CONSTRAINT content_images_processing_status_allowed CHECK (
    processing_status IN ('pending', 'running', 'processed', 'failed')
  )
);

CREATE UNIQUE INDEX content_images_storage_object_unique ON content_images (bucket, storage_key);
CREATE UNIQUE INDEX content_images_one_primary_per_content
  ON content_images (content_id)
  WHERE is_primary;
CREATE INDEX content_images_content_id_idx ON content_images (content_id);

CREATE TABLE tags (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  slug text NOT NULL UNIQUE,
  name text NOT NULL,
  is_system boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT tags_slug_not_blank CHECK (length(trim(slug)) > 0),
  CONSTRAINT tags_name_not_blank CHECK (length(trim(name)) > 0)
);

CREATE TABLE content_tags (
  content_id uuid NOT NULL REFERENCES content_items (id) ON DELETE CASCADE,
  tag_id uuid NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (content_id, tag_id)
);

CREATE INDEX content_tags_tag_id_idx ON content_tags (tag_id);

CREATE TABLE likes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  content_id uuid NOT NULL REFERENCES content_items (id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, content_id)
);

CREATE INDEX likes_content_id_idx ON likes (content_id);

CREATE TABLE comments (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  content_id uuid NOT NULL REFERENCES content_items (id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  parent_id uuid REFERENCES comments (id) ON DELETE CASCADE,
  body text NOT NULL,
  status text NOT NULL DEFAULT 'visible',
  hidden_by uuid REFERENCES users (id) ON DELETE SET NULL,
  hidden_at timestamptz,
  deleted_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT comments_body_not_blank CHECK (length(trim(body)) > 0),
  CONSTRAINT comments_status_allowed CHECK (
    status IN ('visible', 'hidden', 'deleted', 'pending_moderation')
  )
);

CREATE INDEX comments_content_id_idx ON comments (content_id);
CREATE INDEX comments_user_id_idx ON comments (user_id);
CREATE INDEX comments_status_idx ON comments (status);

CREATE TABLE reports (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  reporter_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
  content_id uuid REFERENCES content_items (id) ON DELETE CASCADE,
  comment_id uuid REFERENCES comments (id) ON DELETE CASCADE,
  reason text NOT NULL,
  details text,
  status text NOT NULL DEFAULT 'open',
  resolved_by uuid REFERENCES users (id) ON DELETE SET NULL,
  resolved_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT reports_reason_not_blank CHECK (length(trim(reason)) > 0),
  CONSTRAINT reports_target_required CHECK (
    (content_id IS NOT NULL AND comment_id IS NULL) OR
    (content_id IS NULL AND comment_id IS NOT NULL)
  ),
  CONSTRAINT reports_status_allowed CHECK (status IN ('open', 'in_review', 'resolved', 'rejected'))
);

CREATE INDEX reports_status_idx ON reports (status);
CREATE INDEX reports_content_id_idx ON reports (content_id);
CREATE INDEX reports_comment_id_idx ON reports (comment_id);

CREATE TABLE moderation_actions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  actor_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
  content_id uuid REFERENCES content_items (id) ON DELETE CASCADE,
  comment_id uuid REFERENCES comments (id) ON DELETE CASCADE,
  report_id uuid REFERENCES reports (id) ON DELETE SET NULL,
  action text NOT NULL,
  reason text,
  before_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  after_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  ip_address inet,
  user_agent text,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT moderation_actions_action_not_blank CHECK (length(trim(action)) > 0)
);

CREATE INDEX moderation_actions_actor_user_id_idx ON moderation_actions (actor_user_id);
CREATE INDEX moderation_actions_content_id_idx ON moderation_actions (content_id);
CREATE INDEX moderation_actions_created_at_idx ON moderation_actions (created_at DESC);

CREATE TABLE audit_logs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  actor_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
  action text NOT NULL,
  entity_type text NOT NULL,
  entity_id uuid,
  before_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  after_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  ip_address inet,
  user_agent text,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT audit_logs_action_not_blank CHECK (length(trim(action)) > 0),
  CONSTRAINT audit_logs_entity_type_not_blank CHECK (length(trim(entity_type)) > 0)
);

CREATE INDEX audit_logs_actor_user_id_idx ON audit_logs (actor_user_id);
CREATE INDEX audit_logs_entity_idx ON audit_logs (entity_type, entity_id);
CREATE INDEX audit_logs_created_at_idx ON audit_logs (created_at DESC);

CREATE TABLE api_tokens (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  name text NOT NULL,
  token_hash text NOT NULL UNIQUE,
  scopes text[] NOT NULL DEFAULT '{}',
  expires_at timestamptz,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT api_tokens_name_not_blank CHECK (length(trim(name)) > 0)
);

CREATE INDEX api_tokens_user_id_idx ON api_tokens (user_id);

CREATE TABLE download_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  content_id uuid NOT NULL REFERENCES content_items (id) ON DELETE CASCADE,
  file_id uuid NOT NULL REFERENCES content_files (id) ON DELETE CASCADE,
  user_id uuid REFERENCES users (id) ON DELETE SET NULL,
  ip_address inet,
  user_agent text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX download_events_content_id_idx ON download_events (content_id);
CREATE INDEX download_events_file_id_idx ON download_events (file_id);
CREATE INDEX download_events_created_at_idx ON download_events (created_at DESC);

CREATE TABLE worker_jobs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  queue_name text NOT NULL,
  job_type text NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  status text NOT NULL DEFAULT 'pending',
  attempt_count integer NOT NULL DEFAULT 0,
  max_attempts integer NOT NULL DEFAULT 5,
  last_error text,
  next_retry_at timestamptz,
  locked_by text,
  locked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT worker_jobs_queue_name_allowed CHECK (queue_name IN (
    'file_scan_queue',
    'image_processing_queue',
    'search_index_queue',
    'email_queue',
    'moderation_queue',
    'cleanup_queue'
  )),
  CONSTRAINT worker_jobs_status_allowed CHECK (
    status IN ('pending', 'running', 'succeeded', 'failed', 'dead')
  ),
  CONSTRAINT worker_jobs_attempts_valid CHECK (
    attempt_count >= 0 AND max_attempts > 0 AND attempt_count <= max_attempts
  )
);

CREATE INDEX worker_jobs_queue_status_retry_idx
  ON worker_jobs (queue_name, status, next_retry_at, created_at);
CREATE INDEX worker_jobs_created_at_idx ON worker_jobs (created_at DESC);
