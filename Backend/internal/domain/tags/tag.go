package tags

import "time"

type Tag struct {
	ID             string    `json:"id"`
	Slug           string    `json:"slug"`
	Name           string    `json:"name"`
	IsSystem       bool      `json:"is_system"`
	PublishedCount int       `json:"published_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type AdminCreateInput struct {
	Slug     string
	Name     string
	IsSystem bool
}

type AdminUpdateInput struct {
	Name     *string
	IsSystem *bool
}
