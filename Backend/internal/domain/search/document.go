package search

import "time"

type ContentDocument struct {
	ID              string     `json:"id"`
	Slug            string     `json:"slug"`
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	CategorySlug    string     `json:"category_slug"`
	CategoryName    string     `json:"category_name"`
	AuthorID        string     `json:"author_id"`
	AuthorUsername  string     `json:"author_username"`
	AuthorName      *string    `json:"author_name,omitempty"`
	Tags            []string   `json:"tags"`
	TagNames        []string   `json:"tag_names"`
	NSFW            bool       `json:"nsfw"`
	PreviewImageID  *string    `json:"preview_image_id,omitempty"`
	LikesCount      int        `json:"likes_count"`
	DownloadsCount  int        `json:"downloads_count"`
	CommentsCount   int        `json:"comments_count"`
	PublishedAt     *time.Time `json:"published_at,omitempty"`
	PublishedAtUnix int64      `json:"published_at_unix"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type ContentQuery struct {
	Query        string
	CategorySlug string
	Tags         []string
	IncludeNSFW  bool
	Sort         string
	Limit        int
	Offset       int
}

type ContentSearchResult struct {
	IDs                []string
	EstimatedTotalHits int
	Limit              int
	Offset             int
}
