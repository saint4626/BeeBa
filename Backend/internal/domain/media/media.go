package media

import "time"

type ImageUploadInput struct {
	OwnerUserID      string
	ContentID        string
	Bucket           string
	StorageKey       string
	OriginalFilename string
	AltText          string
	Width            int
	Height           int
	FileSize         int64
	FileHashSHA256   string
	MimeTypeDetected string
	IsPrimary        bool
	SortOrder        int
}

type UploadedImage struct {
	ID               string    `json:"id"`
	Kind             string    `json:"kind"`
	ContentID        *string   `json:"content_id,omitempty"`
	UserID           *string   `json:"user_id,omitempty"`
	URL              string    `json:"url"`
	AltText          string    `json:"alt_text"`
	Width            int       `json:"width"`
	Height           int       `json:"height"`
	FileSize         int64     `json:"file_size"`
	FileHashSHA256   string    `json:"file_hash_sha256"`
	MimeTypeDetected string    `json:"mime_type_detected"`
	ProcessingStatus string    `json:"processing_status"`
	IsPrimary        bool      `json:"is_primary,omitempty"`
	SortOrder        int       `json:"sort_order,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	ReplacedObjects  []Object  `json:"-"`
}

type ContentImageUpdate struct {
	OwnerUserID string
	ContentID   string
	ImageID     string
	AltText     *string
	IsPrimary   *bool
	SortOrder   *int
}

type Object struct {
	ID          string
	Bucket      string
	StorageKey  string
	ContentType string
	FileSize    int64
}

type ImageForProcessing struct {
	ID               string
	Kind             string
	UserID           string
	ContentID        string
	Bucket           string
	StorageKey       string
	OriginalFilename string
	MimeTypeDetected string
}
