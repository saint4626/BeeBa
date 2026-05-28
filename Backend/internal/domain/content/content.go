package content

import "time"

type PublicItem struct {
	ID             string     `json:"id"`
	Slug           string     `json:"slug"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	Status         string     `json:"status"`
	Visibility     string     `json:"visibility"`
	NSFW           bool       `json:"nsfw"`
	Category       Category   `json:"category"`
	Author         Author     `json:"author"`
	Tags           []Tag      `json:"tags"`
	PreviewImageID *string    `json:"preview_image_id,omitempty"`
	LikesCount     int        `json:"likes_count"`
	DownloadsCount int        `json:"downloads_count"`
	CommentsCount  int        `json:"comments_count"`
	UnlockPassword *string    `json:"unlock_password,omitempty"`
	PublishedAt    *time.Time `json:"published_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type PublicDetail struct {
	PublicItem
	File    *PublicFile `json:"file,omitempty"`
	Gallery []Image     `json:"gallery"`
}

type PublicFile struct {
	FileSize         int64  `json:"file_size"`
	FileHashSHA256   string `json:"file_hash_sha256"`
	OriginalFilename string `json:"original_filename"`
}

type Image struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	AltText   string    `json:"alt_text"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	IsPrimary bool      `json:"is_primary"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
}

type OwnerItem struct {
	ID             string                   `json:"id"`
	Slug           string                   `json:"slug"`
	Title          string                   `json:"title"`
	Description    string                   `json:"description"`
	Status         string                   `json:"status"`
	Visibility     string                   `json:"visibility"`
	NSFW           bool                     `json:"nsfw"`
	Category       Category                 `json:"category"`
	Tags           []Tag                    `json:"tags"`
	File           *OwnerFile               `json:"file,omitempty"`
	UnlockPassword *string                  `json:"unlock_password,omitempty"`
	Moderation     *OwnerModerationFeedback `json:"moderation,omitempty"`
	PublishedAt    *time.Time               `json:"published_at,omitempty"`
	HiddenAt       *time.Time               `json:"hidden_at,omitempty"`
	CreatedAt      time.Time                `json:"created_at"`
	UpdatedAt      time.Time                `json:"updated_at"`
}

type OwnerModerationFeedback struct {
	Action    string    `json:"action"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

type OwnerFile struct {
	ID               string `json:"id"`
	ScanStatus       string `json:"scan_status"`
	FileSize         int64  `json:"file_size"`
	FileHashSHA256   string `json:"file_hash_sha256"`
	OriginalFilename string `json:"original_filename"`
	SafeFilename     string `json:"safe_filename"`
}

type Category struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type Author struct {
	ID            string  `json:"id"`
	Username      string  `json:"username"`
	DisplayName   *string `json:"display_name,omitempty"`
	AvatarImageID *string `json:"avatar_image_id,omitempty"`
}

type Tag struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type ListFilter struct {
	CategorySlug string
	Author       string
	Query        string
	Tags         []string
	IncludeNSFW  bool
	Sort         string
	Limit        int
	Cursor       Cursor
}

type OwnerListFilter struct {
	Query      string
	Status     string
	Visibility string
	Limit      int
	Cursor     Cursor
}

type OwnerUpdateInput struct {
	Title       *string
	Description *string
	Visibility  *string
	NSFW        *bool
	Tags        *[]TagInput
}

type TagInput struct {
	Slug string
	Name string
}

type Cursor struct {
	SortValue string `json:"sort_value"`
	ID        string `json:"id"`
}

type Page struct {
	Items      []PublicItem
	NextCursor *Cursor
}

type OwnerPage struct {
	Items      []OwnerItem
	NextCursor *Cursor
}
