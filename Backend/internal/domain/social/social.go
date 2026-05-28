package social

import "time"

type Comment struct {
	ID          string    `json:"id"`
	ContentID   string    `json:"content_id"`
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	DisplayName *string   `json:"display_name,omitempty"`
	ParentID    *string   `json:"parent_id,omitempty"`
	Body        string    `json:"body"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CommentInput struct {
	ContentID string
	UserID    string
	ParentID  *string
	Body      string
}

type CommentFilter struct {
	Limit  int
	Cursor Cursor
}

type CommentPage struct {
	Items      []Comment
	NextCursor *Cursor
}

type LikeResult struct {
	ContentID  string `json:"content_id"`
	Liked      bool   `json:"liked"`
	LikesCount int    `json:"likes_count"`
}

type Report struct {
	ID        string    `json:"id"`
	ContentID string    `json:"content_id"`
	UserID    string    `json:"user_id"`
	Reason    string    `json:"reason"`
	Details   *string   `json:"details,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type ReportInput struct {
	ContentID string
	UserID    string
	Reason    string
	Details   string
}

type Cursor struct {
	SortValue string `json:"sort_value"`
	ID        string `json:"id"`
}
