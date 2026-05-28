package downloads

type Target struct {
	ContentID                string
	FileID                   string
	AuthorID                 string
	Visibility               string
	Bucket                   string
	StorageKey               string
	Filename                 string
	FileSize                 int64
	UnlockPasswordCiphertext *string
}

type EventInput struct {
	ContentID string
	FileID    string
	UserID    *string
	IPAddress string
	UserAgent string
}
