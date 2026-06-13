package upload

import (
	"errors"
	"time"
)

var ErrDuplicateFile = errors.New("duplicate content file")

type CreateInput struct {
	AuthorID                 string
	CategorySlug             string
	Title                    string
	Slug                     string
	Description              string
	NSFW                     bool
	Visibility               string
	Bucket                   string
	StorageKey               string
	OriginalFilename         string
	SafeFilename             string
	FileSize                 int64
	FileHashSHA256           string
	MimeTypeDetected         string
	UnlockPasswordCiphertext string
	StorageQuotaBytes        int64
}

type Created struct {
	ContentID        string    `json:"content_id"`
	FileID           string    `json:"file_id"`
	JobID            string    `json:"job_id"`
	Status           string    `json:"status"`
	ScanStatus       string    `json:"scan_status"`
	StorageBucket    string    `json:"storage_bucket"`
	StorageKey       string    `json:"storage_key"`
	FileHashSHA256   string    `json:"file_hash_sha256"`
	FileSize         int64     `json:"file_size"`
	OriginalFilename string    `json:"original_filename"`
	Visibility       string    `json:"visibility"`
	CreatedAt        time.Time `json:"created_at"`
}
