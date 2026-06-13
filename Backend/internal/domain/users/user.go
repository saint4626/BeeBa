package users

import "time"

type PublicUser struct {
	ID              string     `json:"id"`
	Email           string     `json:"email"`
	Username        string     `json:"username"`
	DisplayName     *string    `json:"display_name,omitempty"`
	AvatarImageID   *string    `json:"avatar_image_id,omitempty"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type PublicProfile struct {
	ID             string    `json:"id"`
	Username       string    `json:"username"`
	DisplayName    *string   `json:"display_name,omitempty"`
	AvatarImageID  *string   `json:"avatar_image_id,omitempty"`
	PublishedCount int       `json:"published_count"`
	LikesCount     int       `json:"likes_count"`
	DownloadsCount int       `json:"downloads_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type CreateInput struct {
	Email                           string
	Username                        string
	DisplayName                     *string
	PasswordHash                    string
	EmailVerificationTokenHash      string
	EmailVerificationToken          string
	EmailVerificationTokenExpiresAt time.Time
	EmailDailyLimit                 int
}

type ProfileUpdateInput struct {
	DisplayName *string
}

type PasswordChangeInput struct {
	NewPasswordHash  string
	CurrentSessionID string
}

type PasswordChangeRequestInput struct {
	UserID          string
	TokenHash       string
	Token           string
	NewPasswordHash string
	ExpiresAt       time.Time
	EmailDailyLimit int
	IPAddress       string
	UserAgent       string
}

type PasswordChangeConfirmResult struct {
	User PublicUser `json:"user"`
}

type EmailChangeRequestInput struct {
	UserID          string
	NewEmail        string
	TokenHash       string
	Token           string
	ExpiresAt       time.Time
	EmailDailyLimit int
	IPAddress       string
	UserAgent       string
}

type EmailChangeRequestResult struct {
	User         PublicUser `json:"user"`
	PendingEmail string     `json:"pending_email"`
}

type EmailChangeConfirmResult struct {
	User          PublicUser `json:"user"`
	PreviousEmail string     `json:"previous_email"`
	NewEmail      string     `json:"new_email"`
}

type EmailVerificationResult struct {
	User            PublicUser `json:"user"`
	AlreadyVerified bool       `json:"already_verified"`
}

type EmailVerificationRequestInput struct {
	UserID          string
	TokenHash       string
	Token           string
	ExpiresAt       time.Time
	EmailDailyLimit int
	IPAddress       string
	UserAgent       string
}

type AdminListFilter struct {
	Query string
	Role  string
	Limit int
}

type AdminUser struct {
	ID              string     `json:"id"`
	Email           string     `json:"email"`
	Username        string     `json:"username"`
	DisplayName     *string    `json:"display_name,omitempty"`
	AvatarImageID   *string    `json:"avatar_image_id,omitempty"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	Roles           []string   `json:"roles"`
	BannedAt        *time.Time `json:"banned_at,omitempty"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty"`
	ContentCount    int        `json:"content_count"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type AdminUserActionInput struct {
	ActorUserID  string
	TargetUserID string
	Reason       string
	IPAddress    string
	UserAgent    string
}

type AdminUserRolesInput struct {
	ActorUserID    string
	TargetUserID   string
	Roles          []string
	AllowOwnerRole bool
	IPAddress      string
	UserAgent      string
}
