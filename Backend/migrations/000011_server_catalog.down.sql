ALTER TABLE worker_jobs DROP CONSTRAINT worker_jobs_queue_name_allowed;
DELETE FROM worker_jobs WHERE queue_name = 'server_check_queue';
ALTER TABLE worker_jobs ADD CONSTRAINT worker_jobs_queue_name_allowed CHECK (queue_name IN (
  'file_scan_queue',
  'image_processing_queue',
  'search_index_queue',
  'email_queue',
  'moderation_queue',
  'cleanup_queue'
));

DROP TABLE IF EXISTS server_tags;
DROP TABLE IF EXISTS server_checks;
DROP TABLE IF EXISTS server_listings;
