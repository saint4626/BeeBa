package audit

import (
	"encoding/json"
	"time"
)

type LogEntry struct {
	ID            string          `json:"id"`
	ActorUserID   *string         `json:"actor_user_id,omitempty"`
	ActorUsername *string         `json:"actor_username,omitempty"`
	ActorEmail    *string         `json:"actor_email,omitempty"`
	Action        string          `json:"action"`
	EntityType    string          `json:"entity_type"`
	EntityID      *string         `json:"entity_id,omitempty"`
	Before        json.RawMessage `json:"before_json"`
	After         json.RawMessage `json:"after_json"`
	IPAddress     *string         `json:"ip_address,omitempty"`
	UserAgent     *string         `json:"user_agent,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

type ListFilter struct {
	Action      string
	EntityType  string
	ActorUserID string
	EntityID    string
	Limit       int
	Cursor      Cursor
}

type Page struct {
	Items      []LogEntry
	NextCursor Cursor
}

type Cursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
}
