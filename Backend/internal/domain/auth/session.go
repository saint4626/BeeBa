package auth

import (
	"time"

	"beeba.org/internal/domain/users"
)

type SessionInput struct {
	UserID           string
	AccessTokenHash  string
	RefreshTokenHash string
	UserAgent        string
	IPAddress        string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
}

type TokenPair struct {
	AccessToken      string           `json:"access_token"`
	RefreshToken     string           `json:"refresh_token"`
	TokenType        string           `json:"token_type"`
	ExpiresIn        int64            `json:"expires_in"`
	RefreshExpiresIn int64            `json:"refresh_expires_in"`
	User             users.PublicUser `json:"user"`
}

type SessionInfo struct {
	ExpiresIn        int64            `json:"expires_in"`
	RefreshExpiresIn int64            `json:"refresh_expires_in"`
	User             users.PublicUser `json:"user"`
}

func (p TokenPair) SessionInfo() SessionInfo {
	return SessionInfo{
		ExpiresIn:        p.ExpiresIn,
		RefreshExpiresIn: p.RefreshExpiresIn,
		User:             p.User,
	}
}

type UserWithPassword struct {
	User         users.PublicUser
	PasswordHash string
	BannedAt     *time.Time
	DeletedAt    *time.Time
}

type SessionWithUser struct {
	SessionID string
	User      users.PublicUser
	Roles     []string
	BannedAt  *time.Time
	DeletedAt *time.Time
}
