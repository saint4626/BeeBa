CREATE INDEX server_listings_check_due_idx
  ON server_listings (last_checked_at ASC NULLS FIRST, created_at ASC, id ASC)
  WHERE deleted_at IS NULL
    AND status IN ('pending_verification', 'published', 'offline', 'failed');

CREATE INDEX worker_jobs_server_check_active_server_idx
  ON worker_jobs ((payload->>'server_id'))
  WHERE queue_name = 'server_check_queue'
    AND status IN ('pending', 'running');
