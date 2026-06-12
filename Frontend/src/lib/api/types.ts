export interface Category {
  slug: "worlds" | "avatars" | "props" | "prefabs";
  name: string;
}

export interface AdminCategory {
  id: string;
  slug: "worlds" | "avatars" | "props" | "prefabs";
  name: string;
  description: string;
  icon_key?: string | null;
  sort_order: number;
  is_active: boolean;
  published_count: number;
  created_at: string;
  updated_at: string;
}

export interface PublicContentTag {
  slug: string;
  name: string;
}

export interface CatalogTag extends PublicContentTag {
  id: string;
  is_system: boolean;
  published_count: number;
  created_at: string;
  updated_at: string;
}

export interface PublicContentAuthor {
  id: string;
  username: string;
  display_name?: string | null;
  avatar_image_id?: string | null;
}

export interface PublicProfile {
  id: string;
  username: string;
  display_name?: string | null;
  avatar_image_id?: string | null;
  published_count: number;
  likes_count: number;
  downloads_count: number;
  created_at: string;
  updated_at: string;
}

export interface PublicContentItem {
  id: string;
  slug: string;
  title: string;
  description: string;
  status: "published";
  visibility: "public";
  nsfw: boolean;
  unlock_password?: string;
  category: Category;
  author: PublicContentAuthor;
  tags: PublicContentTag[];
  preview_image_id?: string | null;
  likes_count: number;
  downloads_count: number;
  comments_count: number;
  published_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface PublicContentImage {
  id: string;
  url: string;
  alt_text: string;
  width: number;
  height: number;
  is_primary: boolean;
  sort_order: number;
  created_at: string;
}

export interface PublicContentFile {
  file_size: number;
  file_hash_sha256: string;
  original_filename: string;
}

export interface PublicContentDetail extends PublicContentItem {
  file?: PublicContentFile;
  gallery: PublicContentImage[];
  liked_by_me: boolean;
}

export interface PublicComment {
  id: string;
  content_id: string;
  user_id: string;
  username: string;
  display_name?: string | null;
  parent_id?: string | null;
  body: string;
  status: "visible" | "pending_moderation" | "hidden" | "deleted";
  created_at: string;
  updated_at: string;
}

export interface LikeResult {
  content_id: string;
  liked: boolean;
  likes_count: number;
}

export interface ContentReport {
  id: string;
  content_id: string;
  user_id: string;
  reason: string;
  details?: string | null;
  status: "open" | "in_review" | "resolved" | "rejected";
  created_at: string;
}

export interface CursorPagination {
  next_cursor?: string | null;
  limit: number;
  offset?: number;
  total?: number;
}

export interface APIListResponse<T> {
  data: T[];
  pagination?: CursorPagination;
}

export interface APIErrorResponse {
  error: {
    code: string;
    message: string;
    request_id?: string;
  };
}

export interface CatalogFilter {
  category?: Category["slug"];
  author?: string;
  q?: string;
  tags?: string[];
  include_nsfw?: boolean;
  sort?: "newest" | "likes" | "downloads";
  cursor?: string;
  limit?: number;
}

export interface PublicUser {
  id: string;
  email: string;
  username: string;
  display_name?: string | null;
  avatar_image_id?: string | null;
  email_verified_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface TokenPair {
  access_token: string;
  refresh_token: string;
  token_type: "Bearer";
  expires_in: number;
  refresh_expires_in: number;
  user: PublicUser;
}

export interface ContentUploadCreated {
  content_id: string;
  file_id: string;
  job_id: string;
  status: "pending_scan";
  scan_status: "pending";
  storage_bucket: string;
  storage_key: string;
  file_hash_sha256: string;
  file_size: number;
  original_filename: string;
  visibility: "public" | "private";
  created_at: string;
}

export interface OwnerContentFile {
  id: string;
  scan_status: "pending" | "running" | "clean" | "suspicious" | "infected" | "failed" | "skipped";
  file_size: number;
  file_hash_sha256: string;
  original_filename: string;
  safe_filename: string;
}

export interface OwnerContentItem {
  id: string;
  slug: string;
  title: string;
  description: string;
  status:
    | "draft"
    | "uploaded"
    | "pending_scan"
    | "scan_failed"
    | "pending_moderation"
    | "approved"
    | "published"
    | "rejected"
    | "hidden"
    | "deleted";
  visibility: "public" | "private";
  nsfw: boolean;
  category: Category;
  tags: PublicContentTag[];
  file?: OwnerContentFile;
  unlock_password?: string;
  moderation?: OwnerModerationFeedback;
  published_at?: string | null;
  hidden_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface OwnerModerationFeedback {
  action: "reject" | "hide";
  reason: string;
  created_at: string;
}

export interface OwnerContentUpdateInput {
  title?: string;
  description?: string;
  visibility?: "public" | "private";
  nsfw?: boolean;
  tags?: string[];
}

export interface AdminStatus {
  status: "ok";
  actor: PublicUser;
  roles: string[];
}

export interface ModerationQueueItem {
  content_id: string;
  file_id?: string;
  title: string;
  description: string;
  status: string;
  visibility: "public" | "private";
  nsfw: boolean;
  category_slug: string;
  category_name: string;
  author_id: string;
  author_username: string;
  author_display_name?: string | null;
  scan_status?: string;
  file_size?: number;
  file_hash_sha256?: string;
  original_filename?: string;
  created_at: string;
  updated_at: string;
}

export interface ModeratedContent {
  content_id: string;
  status: "published" | "rejected" | "hidden";
  hidden_at?: string | null;
  reason?: string;
  updated_at: string;
}

export interface ModerationCommentItem {
  comment_id: string;
  content_id: string;
  content_title: string;
  body: string;
  status: "visible" | "hidden" | "deleted" | "pending_moderation";
  author_id: string;
  author_username: string;
  author_display_name?: string | null;
  content_author_id: string;
  content_author_username: string;
  content_author_display_name?: string | null;
  parent_id?: string | null;
  created_at: string;
  updated_at: string;
}

export interface ModeratedComment {
  comment_id: string;
  content_id: string;
  status: "visible" | "hidden" | "deleted" | "pending_moderation";
  comments_count: number;
  reason?: string;
  updated_at: string;
}

export interface ModerationReportItem {
  report_id: string;
  content_id: string;
  content_title: string;
  comment_id?: string | null;
  reason: string;
  details?: string | null;
  status: "open" | "in_review" | "resolved" | "rejected";
  reporter_user_id?: string | null;
  reporter_username?: string | null;
  reporter_display_name?: string | null;
  resolved_by?: string | null;
  resolved_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface ModeratedReport {
  report_id: string;
  status: "open" | "in_review" | "resolved" | "rejected";
  resolved_by?: string | null;
  resolved_at?: string | null;
  reason?: string;
  updated_at: string;
}

export interface AdminUser {
  id: string;
  email: string;
  username: string;
  display_name?: string | null;
  avatar_image_id?: string | null;
  email_verified_at?: string | null;
  roles: Array<"user" | "moderator" | "admin" | "owner">;
  banned_at?: string | null;
  deleted_at?: string | null;
  content_count: number;
  created_at: string;
  updated_at: string;
}

export interface AuditLogEntry {
  id: string;
  actor_user_id?: string | null;
  actor_username?: string | null;
  actor_email?: string | null;
  action: string;
  entity_type: string;
  entity_id?: string | null;
  before_json: Record<string, unknown> | unknown[] | null;
  after_json: Record<string, unknown> | unknown[] | null;
  ip_address?: string | null;
  user_agent?: string | null;
  created_at: string;
}

export interface AdminJob {
  id: string;
  queue_name: "file_scan_queue" | "image_processing_queue" | "search_index_queue" | "email_queue" | "moderation_queue" | "cleanup_queue";
  job_type: string;
  payload: Record<string, unknown> | unknown[] | null;
  status: "pending" | "running" | "succeeded" | "failed" | "dead";
  attempt_count: number;
  max_attempts: number;
  last_error?: string | null;
  next_retry_at?: string | null;
  locked_by?: string | null;
  locked_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface AdminFile {
  id: string;
  content_id: string;
  content_title: string;
  content_status: string;
  content_visibility: "public" | "private";
  author_id: string;
  author_username: string;
  bucket: string;
  original_filename: string;
  safe_filename: string;
  file_size: number;
  file_hash_sha256: string;
  mime_type_detected?: string | null;
  extension: ".bee";
  scan_status: "pending" | "running" | "clean" | "suspicious" | "infected" | "failed" | "skipped";
  scan_result: Record<string, unknown> | unknown[] | null;
  uploaded_by: string;
  uploaded_by_username: string;
  created_at: string;
  updated_at: string;
}

export interface AdminFileRescanResult {
  file: AdminFile;
  job_id: string;
}

export interface UploadedImage {
  id: string;
  kind: "user_avatar" | "content_image";
  content_id?: string;
  user_id?: string;
  url: string;
  alt_text: string;
  width: number;
  height: number;
  file_size: number;
  file_hash_sha256: string;
  mime_type_detected: "image/jpeg" | "image/png" | "image/webp";
  processing_status: "pending" | "running" | "processed" | "failed";
  is_primary?: boolean;
  sort_order?: number;
  created_at: string;
}
