package auth

import (
	"context"
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"beeba.org/internal/config"
	authdomain "beeba.org/internal/domain/auth"
	"beeba.org/internal/domain/users"
	"beeba.org/internal/http/middleware/authz"
	"beeba.org/internal/repository/postgres"
	"beeba.org/internal/security/password"
	"beeba.org/internal/security/tokens"
	"beeba.org/internal/security/turnstile"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5"
)

type UserStore interface {
	Create(ctx context.Context, input users.CreateInput) (users.PublicUser, error)
	VerifyEmail(ctx context.Context, tokenHash string, ipAddress string, userAgent string) (users.EmailVerificationResult, error)
	RequestEmailVerification(ctx context.Context, input users.EmailVerificationRequestInput) (users.EmailVerificationResult, error)
	ConfirmPasswordChange(ctx context.Context, tokenHash string, ipAddress string, userAgent string) (users.PasswordChangeConfirmResult, error)
	ConfirmEmailChange(ctx context.Context, tokenHash string, ipAddress string, userAgent string) (users.EmailChangeConfirmResult, error)
	FindByEmailForLogin(ctx context.Context, email string) (authdomain.UserWithPassword, error)
	CreateSession(ctx context.Context, input authdomain.SessionInput) error
	FindByAccessTokenHash(ctx context.Context, tokenHash string, now time.Time) (authdomain.SessionWithUser, error)
	FindByRefreshTokenHash(ctx context.Context, tokenHash string, now time.Time) (authdomain.SessionWithUser, error)
	RotateSession(ctx context.Context, sessionID string, currentRefreshTokenHash string, input authdomain.SessionInput) error
	RevokeByRefreshTokenHash(ctx context.Context, tokenHash string) error
}

type Handler struct {
	cfg       config.Config
	users     UserStore
	turnstile *turnstile.Verifier
}

func New(cfg config.Config, users UserStore) Handler {
	var verifier *turnstile.Verifier
	if strings.TrimSpace(cfg.TurnstileSecretKey) != "" {
		verifier = turnstile.New(cfg.TurnstileSecretKey, cfg.TurnstileVerifyURL)
	}
	return Handler{cfg: cfg, users: users, turnstile: verifier}
}

type registerRequest struct {
	Email          string  `json:"email"`
	Username       string  `json:"username"`
	DisplayName    *string `json:"display_name"`
	Password       string  `json:"password"`
	TurnstileToken string  `json:"turnstile_token"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

const refreshCookieName = "beeba_refresh_token"

type verifyEmailRequest struct {
	Token string `json:"token"`
}

func (h Handler) Register(c fiber.Ctx) error {
	if h.users == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "user repository unavailable")
	}

	var req registerRequest
	if err := c.Bind().Body(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	input, err := validateRegister(req)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	if err := h.verifyTurnstile(c, req.TurnstileToken); err != nil {
		return err
	}

	hash, err := password.Hash(req.Password)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to create user")
	}
	input.PasswordHash = hash
	verificationToken, err := tokens.New("bb_ev_")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to create user")
	}
	input.EmailVerificationTokenHash = tokens.Hash(verificationToken)
	input.EmailVerificationToken = verificationToken
	input.EmailVerificationTokenExpiresAt = time.Now().UTC().Add(24 * time.Hour)
	input.EmailDailyLimit = h.cfg.EmailDailyLimit

	ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
	defer cancel()

	user, err := h.users.Create(ctx, input)
	if err != nil {
		if errors.Is(err, postgres.ErrUserConflict) {
			return fiber.NewError(fiber.StatusConflict, "Email or username already exists")
		}
		if errors.Is(err, postgres.ErrEmailDailyLimitReached) {
			return fiber.NewError(fiber.StatusTooManyRequests, "email verification is temporarily unavailable")
		}
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to create user")
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"data": user,
		"meta": fiber.Map{
			"email_verification_required": true,
		},
	})
}

func (h Handler) VerifyEmail(c fiber.Ctx) error {
	if h.users == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "user repository unavailable")
	}
	var req verifyEmailRequest
	if err := c.Bind().Body(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}
	token := strings.TrimSpace(req.Token)
	if !strings.HasPrefix(token, "bb_ev_") {
		return fiber.NewError(fiber.StatusBadRequest, "token is invalid")
	}
	ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
	defer cancel()
	result, err := h.users.VerifyEmail(ctx, tokens.Hash(token), c.IP(), c.Get("User-Agent"))
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusBadRequest, "token is invalid or expired")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to verify email")
	}
	return c.JSON(fiber.Map{
		"data": result.User,
		"meta": fiber.Map{
			"already_verified": result.AlreadyVerified,
		},
	})
}

func (h Handler) ConfirmPasswordChange(c fiber.Ctx) error {
	if h.users == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "user repository unavailable")
	}
	var req verifyEmailRequest
	if err := c.Bind().Body(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}
	token := strings.TrimSpace(req.Token)
	if !strings.HasPrefix(token, "bb_pc_") {
		return fiber.NewError(fiber.StatusBadRequest, "token is invalid")
	}
	ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
	defer cancel()
	result, err := h.users.ConfirmPasswordChange(ctx, tokens.Hash(token), c.IP(), c.Get("User-Agent"))
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusBadRequest, "token is invalid or expired")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to confirm password change")
	}
	return c.JSON(fiber.Map{
		"data": result.User,
		"meta": fiber.Map{
			"sessions_revoked": true,
		},
	})
}

func (h Handler) ConfirmEmailChange(c fiber.Ctx) error {
	if h.users == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "user repository unavailable")
	}
	var req verifyEmailRequest
	if err := c.Bind().Body(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}
	token := strings.TrimSpace(req.Token)
	if !strings.HasPrefix(token, "bb_ec_") {
		return fiber.NewError(fiber.StatusBadRequest, "token is invalid")
	}
	ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
	defer cancel()
	result, err := h.users.ConfirmEmailChange(ctx, tokens.Hash(token), c.IP(), c.Get("User-Agent"))
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusBadRequest, "token is invalid or expired")
	}
	if errors.Is(err, postgres.ErrUserConflict) {
		return fiber.NewError(fiber.StatusConflict, "Email already exists")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to confirm email change")
	}
	return c.JSON(fiber.Map{
		"data": result.User,
		"meta": fiber.Map{
			"previous_email": result.PreviousEmail,
			"new_email":      result.NewEmail,
		},
	})
}

func (h Handler) ResendEmailVerification(c fiber.Ctx) error {
	if h.users == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "user repository unavailable")
	}
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}
	verificationToken, err := tokens.New("bb_ev_")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to request email verification")
	}
	ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
	defer cancel()
	result, err := h.users.RequestEmailVerification(ctx, users.EmailVerificationRequestInput{
		UserID:          session.User.ID,
		TokenHash:       tokens.Hash(verificationToken),
		Token:           verificationToken,
		ExpiresAt:       time.Now().UTC().Add(24 * time.Hour),
		EmailDailyLimit: h.cfg.EmailDailyLimit,
		IPAddress:       c.IP(),
		UserAgent:       c.Get("User-Agent"),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "User not found")
	}
	if errors.Is(err, postgres.ErrEmailDailyLimitReached) {
		return fiber.NewError(fiber.StatusTooManyRequests, "email verification is temporarily unavailable")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to request email verification")
	}
	return c.JSON(fiber.Map{
		"data": result.User,
		"meta": fiber.Map{
			"already_verified":  result.AlreadyVerified,
			"verification_sent": !result.AlreadyVerified,
		},
	})
}

func (h Handler) Login(c fiber.Ctx) error {
	if h.users == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "user repository unavailable")
	}

	var req loginRequest
	if err := c.Bind().Body(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if _, err := mail.ParseAddress(email); err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "Invalid email or password")
	}
	if req.Password == "" {
		return fiber.NewError(fiber.StatusUnauthorized, "Invalid email or password")
	}

	ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
	defer cancel()

	user, err := h.users.FindByEmailForLogin(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusUnauthorized, "Invalid email or password")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to login")
	}
	if user.DeletedAt != nil || user.BannedAt != nil {
		return fiber.NewError(fiber.StatusForbidden, "Account is not allowed to login")
	}

	ok, err := password.Verify(req.Password, user.PasswordHash)
	if err != nil || !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Invalid email or password")
	}

	pair, session, err := newSessionTokens(user.User, c.IP(), c.Get("User-Agent"))
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to login")
	}

	if err := h.users.CreateSession(ctx, session); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to login")
	}

	if err := setSessionCookies(c, pair); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to login")
	}
	return c.JSON(fiber.Map{
		"data": pair,
	})
}

func (h Handler) verifyTurnstile(c fiber.Ctx, token string) error {
	if !h.cfg.TurnstileRequired && h.turnstile == nil {
		return nil
	}
	if strings.TrimSpace(token) == "" {
		return fiber.NewError(fiber.StatusBadRequest, "human verification is required")
	}
	if h.turnstile == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "human verification is unavailable")
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()
	result, err := h.turnstile.Verify(ctx, token, c.IP())
	if err != nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "human verification is unavailable")
	}
	if !result.Success {
		return fiber.NewError(fiber.StatusForbidden, "human verification failed")
	}
	return nil
}

func (h Handler) Me(c fiber.Ctx) error {
	session, ok := authz.CurrentSession(c)
	if !ok {
		var err error
		session, err = h.sessionFromBearer(c)
		if err != nil {
			return err
		}
	}

	return c.JSON(fiber.Map{
		"data": session.User,
	})
}

func (h Handler) Refresh(c fiber.Ctx) error {
	if h.users == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "user repository unavailable")
	}

	var req refreshRequest
	if len(c.Body()) > 0 {
		if err := c.Bind().Body(&req); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
		}
	}
	refreshToken := strings.TrimSpace(req.RefreshToken)
	usesRefreshCookie := false
	if refreshToken == "" {
		refreshToken = strings.TrimSpace(c.Cookies(refreshCookieName))
		usesRefreshCookie = refreshToken != ""
	}
	if refreshToken == "" {
		return fiber.NewError(fiber.StatusBadRequest, "refresh_token is required")
	}
	if usesRefreshCookie && !authz.ValidCSRF(c) {
		return fiber.NewError(fiber.StatusForbidden, "Invalid CSRF token")
	}

	ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
	defer cancel()

	session, err := h.users.FindByRefreshTokenHash(ctx, tokens.Hash(refreshToken), time.Now().UTC())
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusUnauthorized, "Invalid refresh token")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to refresh session")
	}
	if session.DeletedAt != nil || session.BannedAt != nil {
		return fiber.NewError(fiber.StatusForbidden, "Account is not allowed to refresh")
	}

	pair, input, err := newSessionTokens(session.User, c.IP(), c.Get("User-Agent"))
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to refresh session")
	}
	if err := h.users.RotateSession(ctx, session.SessionID, tokens.Hash(refreshToken), input); err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "Invalid refresh token")
	}

	if err := setSessionCookies(c, pair); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to refresh session")
	}
	return c.JSON(fiber.Map{
		"data": pair,
	})
}

func (h Handler) Logout(c fiber.Ctx) error {
	if h.users == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "user repository unavailable")
	}

	var req logoutRequest
	if len(c.Body()) > 0 {
		if err := c.Bind().Body(&req); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
		}
	}
	refreshToken := strings.TrimSpace(req.RefreshToken)
	usesRefreshCookie := false
	if refreshToken == "" {
		refreshToken = strings.TrimSpace(c.Cookies(refreshCookieName))
		usesRefreshCookie = refreshToken != ""
	}
	if refreshToken == "" {
		return fiber.NewError(fiber.StatusBadRequest, "refresh_token is required")
	}
	if usesRefreshCookie && !authz.ValidCSRF(c) {
		return fiber.NewError(fiber.StatusForbidden, "Invalid CSRF token")
	}

	ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
	defer cancel()

	if err := h.users.RevokeByRefreshTokenHash(ctx, tokens.Hash(refreshToken)); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to logout")
	}

	clearSessionCookies(c)
	return c.SendStatus(fiber.StatusNoContent)
}

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9_-]{2,31}$`)

func validateRegister(req registerRequest) (users.CreateInput, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if _, err := mail.ParseAddress(email); err != nil {
		return users.CreateInput{}, errors.New("email is invalid")
	}

	username := strings.TrimSpace(req.Username)
	if !usernamePattern.MatchString(username) {
		return users.CreateInput{}, errors.New("username must be 3-32 characters and use letters, numbers, underscore, or hyphen")
	}

	if len(req.Password) < 12 || len(req.Password) > 128 {
		return users.CreateInput{}, errors.New("password must be 12-128 characters")
	}

	var displayName *string
	if req.DisplayName != nil {
		trimmed := strings.TrimSpace(*req.DisplayName)
		if len(trimmed) > 80 {
			return users.CreateInput{}, errors.New("display_name must be at most 80 characters")
		}
		if trimmed != "" {
			displayName = &trimmed
		}
	}

	return users.CreateInput{
		Email:       email,
		Username:    username,
		DisplayName: displayName,
	}, nil
}

func (h Handler) sessionFromBearer(c fiber.Ctx) (authdomain.SessionWithUser, error) {
	if h.users == nil {
		return authdomain.SessionWithUser{}, fiber.NewError(fiber.StatusServiceUnavailable, "user repository unavailable")
	}

	token, ok := authz.TokenFromRequest(c)
	if !ok {
		return authdomain.SessionWithUser{}, fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}

	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()

	session, err := h.users.FindByAccessTokenHash(ctx, tokens.Hash(strings.TrimSpace(token)), time.Now().UTC())
	if errors.Is(err, pgx.ErrNoRows) {
		return authdomain.SessionWithUser{}, fiber.NewError(fiber.StatusUnauthorized, "Invalid bearer token")
	}
	if err != nil {
		return authdomain.SessionWithUser{}, fiber.NewError(fiber.StatusInternalServerError, "Failed to load session")
	}
	if session.DeletedAt != nil || session.BannedAt != nil {
		return authdomain.SessionWithUser{}, fiber.NewError(fiber.StatusForbidden, "Account is not allowed")
	}
	return session, nil
}

func setSessionCookies(c fiber.Ctx, pair authdomain.TokenPair) error {
	secure := requestIsSecure(c)
	csrfToken, err := tokens.New("bb_csrf_")
	if err != nil {
		return err
	}
	c.Cookie(&fiber.Cookie{
		Name:     authz.AccessCookieName,
		Value:    pair.AccessToken,
		Path:     "/",
		SameSite: "Lax",
		MaxAge:   int(pair.ExpiresIn),
		Secure:   secure,
		HTTPOnly: true,
	})
	c.Cookie(&fiber.Cookie{
		Name:     refreshCookieName,
		Value:    pair.RefreshToken,
		Path:     "/",
		SameSite: "Lax",
		MaxAge:   int(pair.RefreshExpiresIn),
		Secure:   secure,
		HTTPOnly: true,
	})
	c.Cookie(&fiber.Cookie{
		Name:     authz.CSRFCookieName,
		Value:    csrfToken,
		Path:     "/",
		SameSite: "Lax",
		MaxAge:   int(pair.RefreshExpiresIn),
		Secure:   secure,
		HTTPOnly: false,
	})
	return nil
}

func clearSessionCookies(c fiber.Ctx) {
	secure := requestIsSecure(c)
	expired := time.Unix(0, 0).UTC()
	c.Cookie(&fiber.Cookie{
		Name:     authz.AccessCookieName,
		Value:    "",
		Path:     "/",
		SameSite: "Lax",
		MaxAge:   -1,
		Expires:  expired,
		Secure:   secure,
		HTTPOnly: true,
	})
	c.Cookie(&fiber.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     "/",
		SameSite: "Lax",
		MaxAge:   -1,
		Expires:  expired,
		Secure:   secure,
		HTTPOnly: true,
	})
	c.Cookie(&fiber.Cookie{
		Name:     authz.CSRFCookieName,
		Value:    "",
		Path:     "/",
		SameSite: "Lax",
		MaxAge:   -1,
		Expires:  expired,
		Secure:   secure,
		HTTPOnly: false,
	})
}

func requestIsSecure(c fiber.Ctx) bool {
	return c.Secure() || strings.EqualFold(c.Scheme(), "https")
}

func newSessionTokens(user users.PublicUser, ipAddress string, userAgent string) (authdomain.TokenPair, authdomain.SessionInput, error) {
	accessToken, err := tokens.New("bb_at_")
	if err != nil {
		return authdomain.TokenPair{}, authdomain.SessionInput{}, err
	}
	refreshToken, err := tokens.New("bb_rt_")
	if err != nil {
		return authdomain.TokenPair{}, authdomain.SessionInput{}, err
	}

	now := time.Now().UTC()
	accessExpiresAt := now.Add(15 * time.Minute)
	refreshExpiresAt := now.Add(30 * 24 * time.Hour)

	return authdomain.TokenPair{
			AccessToken:      accessToken,
			RefreshToken:     refreshToken,
			TokenType:        "Bearer",
			ExpiresIn:        int64(time.Until(accessExpiresAt).Seconds()),
			RefreshExpiresIn: int64(time.Until(refreshExpiresAt).Seconds()),
			User:             user,
		}, authdomain.SessionInput{
			UserID:           user.ID,
			AccessTokenHash:  tokens.Hash(accessToken),
			RefreshTokenHash: tokens.Hash(refreshToken),
			UserAgent:        userAgent,
			IPAddress:        ipAddress,
			AccessExpiresAt:  accessExpiresAt,
			RefreshExpiresAt: refreshExpiresAt,
		}, nil
}
