package files

import (
	"encoding/json"
	"time"
)

type AdminFile struct {
	ID                 string          `json:"id"`
	ContentID          string          `json:"content_id"`
	ContentTitle       string          `json:"content_title"`
	ContentStatus      string          `json:"content_status"`
	ContentVisibility  string          `json:"content_visibility"`
	AuthorID           string          `json:"author_id"`
	AuthorUsername     string          `json:"author_username"`
	Bucket             string          `json:"bucket"`
	OriginalFilename   string          `json:"original_filename"`
	SafeFilename       string          `json:"safe_filename"`
	FileSize           int64           `json:"file_size"`
	FileHashSHA256     string          `json:"file_hash_sha256"`
	MimeTypeDetected   *string         `json:"mime_type_detected,omitempty"`
	Extension          string          `json:"extension"`
	ScanStatus         string          `json:"scan_status"`
	ScanResult         json.RawMessage `json:"scan_result"`
	UploadedBy         string          `json:"uploaded_by"`
	UploadedByUsername string          `json:"uploaded_by_username"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

type AdminListFilter struct {
	Query      string
	ScanStatus string
	Bucket     string
	ContentID  string
	Hash       string
	Limit      int
	Cursor     AdminCursor
}

type AdminPage struct {
	Items      []AdminFile
	NextCursor AdminCursor
}

type AdminCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
}

type AdminRescanInput struct {
	ActorUserID string
	FileID      string
	Reason      string
	IPAddress   string
	UserAgent   string
}

type AdminRescanResult struct {
	File  AdminFile `json:"file"`
	JobID string    `json:"job_id"`
}
