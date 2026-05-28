package jobs

import (
	"encoding/json"
	"time"
)

type Job struct {
	ID           string          `json:"id"`
	QueueName    string          `json:"queue_name"`
	JobType      string          `json:"job_type"`
	Payload      json.RawMessage `json:"payload"`
	AttemptCount int             `json:"attempt_count"`
	MaxAttempts  int             `json:"max_attempts"`
	CreatedAt    time.Time       `json:"created_at"`
}

type AdminJob struct {
	ID           string          `json:"id"`
	QueueName    string          `json:"queue_name"`
	JobType      string          `json:"job_type"`
	Payload      json.RawMessage `json:"payload"`
	Status       string          `json:"status"`
	AttemptCount int             `json:"attempt_count"`
	MaxAttempts  int             `json:"max_attempts"`
	LastError    *string         `json:"last_error,omitempty"`
	NextRetryAt  *time.Time      `json:"next_retry_at,omitempty"`
	LockedBy     *string         `json:"locked_by,omitempty"`
	LockedAt     *time.Time      `json:"locked_at,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type AdminListFilter struct {
	QueueName string
	JobType   string
	Status    string
	Limit     int
	Cursor    AdminCursor
}

type AdminPage struct {
	Items      []AdminJob
	NextCursor AdminCursor
}

type AdminCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
}

type AdminRetryInput struct {
	ActorUserID string
	JobID       string
	Reason      string
	IPAddress   string
	UserAgent   string
}

type FileScanPayload struct {
	ContentID  string `json:"content_id"`
	FileID     string `json:"file_id"`
	Bucket     string `json:"bucket"`
	StorageKey string `json:"storage_key"`
}

type ImageProcessPayload struct {
	Kind    string `json:"kind"`
	ImageID string `json:"image_id"`
}

type SearchIndexPayload struct {
	ContentID string `json:"content_id"`
	Action    string `json:"action"`
}

type ContentFileForScan struct {
	FileID                   string
	ContentID                string
	Bucket                   string
	StorageKey               string
	FileSize                 int64
	FileHashSHA256           string
	ScanStatus               string
	UnlockPasswordCiphertext string
}
