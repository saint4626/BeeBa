package moderation

import "time"

type ApprovalCandidate struct {
	ContentID  string
	FileID     string
	Visibility string
	Status     string
	ScanStatus string
	Bucket     string
	StorageKey string
}

type QueueFilter struct {
	Status string
	Limit  int
}

type QueueItem struct {
	ContentID         string    `json:"content_id"`
	FileID            *string   `json:"file_id,omitempty"`
	Title             string    `json:"title"`
	Description       string    `json:"description"`
	Status            string    `json:"status"`
	Visibility        string    `json:"visibility"`
	NSFW              bool      `json:"nsfw"`
	CategorySlug      string    `json:"category_slug"`
	CategoryName      string    `json:"category_name"`
	AuthorID          string    `json:"author_id"`
	AuthorUsername    string    `json:"author_username"`
	AuthorDisplayName *string   `json:"author_display_name,omitempty"`
	ScanStatus        *string   `json:"scan_status,omitempty"`
	FileSize          *int64    `json:"file_size,omitempty"`
	FileHashSHA256    *string   `json:"file_hash_sha256,omitempty"`
	OriginalFilename  *string   `json:"original_filename,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type ApproveInput struct {
	ContentID     string
	FileID        string
	ActorUserID   string
	NewBucket     string
	NewStorageKey string
	IPAddress     string
	UserAgent     string
}

type Approved struct {
	ContentID   string    `json:"content_id"`
	FileID      string    `json:"file_id"`
	Status      string    `json:"status"`
	Visibility  string    `json:"visibility"`
	PublishedAt time.Time `json:"published_at"`
}

type ReviewInput struct {
	ContentID   string
	ActorUserID string
	Reason      string
	IPAddress   string
	UserAgent   string
}

type Reviewed struct {
	ContentID string     `json:"content_id"`
	Status    string     `json:"status"`
	HiddenAt  *time.Time `json:"hidden_at,omitempty"`
	Reason    string     `json:"reason,omitempty"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type CommentQueueItem struct {
	CommentID             string    `json:"comment_id"`
	ContentID             string    `json:"content_id"`
	ContentTitle          string    `json:"content_title"`
	Body                  string    `json:"body"`
	Status                string    `json:"status"`
	AuthorID              string    `json:"author_id"`
	AuthorUsername        string    `json:"author_username"`
	AuthorDisplayName     *string   `json:"author_display_name,omitempty"`
	ContentAuthorID       string    `json:"content_author_id"`
	ContentAuthorUsername string    `json:"content_author_username"`
	ContentAuthorName     *string   `json:"content_author_display_name,omitempty"`
	ParentID              *string   `json:"parent_id,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type CommentReviewInput struct {
	CommentID   string
	ActorUserID string
	Reason      string
	IPAddress   string
	UserAgent   string
}

type CommentReviewed struct {
	CommentID     string    `json:"comment_id"`
	ContentID     string    `json:"content_id"`
	Status        string    `json:"status"`
	CommentsCount int       `json:"comments_count"`
	Reason        string    `json:"reason,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ReportQueueItem struct {
	ReportID            string     `json:"report_id"`
	ContentID           string     `json:"content_id"`
	ContentTitle        string     `json:"content_title"`
	CommentID           *string    `json:"comment_id,omitempty"`
	Reason              string     `json:"reason"`
	Details             *string    `json:"details,omitempty"`
	Status              string     `json:"status"`
	ReporterUserID      *string    `json:"reporter_user_id,omitempty"`
	ReporterUsername    *string    `json:"reporter_username,omitempty"`
	ReporterDisplayName *string    `json:"reporter_display_name,omitempty"`
	ResolvedBy          *string    `json:"resolved_by,omitempty"`
	ResolvedAt          *time.Time `json:"resolved_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type ReportReviewInput struct {
	ReportID    string
	ActorUserID string
	Status      string
	Reason      string
	IPAddress   string
	UserAgent   string
}

type ReportReviewed struct {
	ReportID   string     `json:"report_id"`
	Status     string     `json:"status"`
	ResolvedBy *string    `json:"resolved_by,omitempty"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
	Reason     string     `json:"reason,omitempty"`
	UpdatedAt  time.Time  `json:"updated_at"`
}
