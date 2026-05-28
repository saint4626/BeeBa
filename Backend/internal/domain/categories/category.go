package categories

import "time"

type Category struct {
	ID             string    `json:"id"`
	Slug           string    `json:"slug"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	IconKey        *string   `json:"icon_key,omitempty"`
	SortOrder      int       `json:"sort_order"`
	IsActive       bool      `json:"is_active"`
	PublishedCount int       `json:"published_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type AdminUpdateInput struct {
	Name        *string
	Description *string
	IconKey     *string
	SortOrder   *int
	IsActive    *bool
}
