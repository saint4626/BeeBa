package me

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"beeba.org/internal/domain/users"
	"beeba.org/internal/http/middleware/authz"
	"beeba.org/internal/repository/postgres"
	"beeba.org/internal/security/password"
	"beeba.org/internal/security/tokens"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5"
)

type AccountStore interface {
	UpdateProfile(ctx context.Context, userID string, input users.ProfileUpdateInput) (users.PublicUser, error)
	GetPasswordHash(ctx context.Context, userID string) (string, error)
	ChangePassword(ctx context.Context, userID string, input users.PasswordChangeInput) error
	RequestPasswordChange(ctx context.Context, input users.PasswordChangeRequestInput) (users.PublicUser, error)
	RequestEmailChange(ctx context.Context, input users.EmailChangeRequestInput) (users.EmailChangeRequestResult, error)
}

type AccountHandler struct {
	store           AccountStore
	emailDailyLimit int
}

func NewAccountHandler(store AccountStore, emailDailyLimit int) AccountHandler {
	return AccountHandler{store: store, emailDailyLimit: emailDailyLimit}
}

type updateProfileRequest struct {
	DisplayName *string `json:"display_name"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type changeEmailRequest struct {
	CurrentPassword string `json:"current_password"`
	NewEmail        string `json:"new_email"`
}

const accountChangeTokenTTL = 30 * time.Minute

func (h AccountHandler) UpdateProfile(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "account repository unavailable")
	}

	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Authentication required")
	}

	var request updateProfileRequest
	if err := c.Bind().Body(&request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}
	if request.DisplayName == nil {
		return fiber.NewError(fiber.StatusBadRequest, "display_name is required")
	}

	displayName := strings.TrimSpace(*request.DisplayName)
	var normalized *string
	if displayName != "" {
		if len(displayName) > 80 {
			return fiber.NewError(fiber.StatusBadRequest, "display_name must be at most 80 characters")
		}
		normalized = &displayName
	}

	ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
	defer cancel()

	user, err := h.store.UpdateProfile(ctx, session.User.ID, users.ProfileUpdateInput{
		DisplayName: normalized,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "User not found")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to update profile")
	}

	return c.JSON(fiber.Map{"data": user})
}

func (h AccountHandler) ChangePassword(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "account repository unavailable")
	}

	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Authentication required")
	}

	var request changePasswordRequest
	if err := c.Bind().Body(&request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}
	if len(request.CurrentPassword) < 1 {
		return fiber.NewError(fiber.StatusBadRequest, "current_password is required")
	}
	if len(request.NewPassword) < 12 || len(request.NewPassword) > 128 {
		return fiber.NewError(fiber.StatusBadRequest, "new_password must be 12-128 characters")
	}
	if request.CurrentPassword == request.NewPassword {
		return fiber.NewError(fiber.StatusBadRequest, "new_password must be different from current_password")
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	currentHash, err := h.store.GetPasswordHash(ctx, session.User.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "User not found")
	}
	if errors.Is(err, postgres.ErrEmailDailyLimitReached) {
		return fiber.NewError(fiber.StatusTooManyRequests, "email confirmation is temporarily unavailable")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to change password")
	}

	ok, err = password.Verify(request.CurrentPassword, currentHash)
	if err != nil || !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Current password is incorrect")
	}

	newHash, err := password.Hash(request.NewPassword)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to change password")
	}
	confirmationToken, err := tokens.New("bb_pc_")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to change password")
	}

	user, err := h.store.RequestPasswordChange(ctx, users.PasswordChangeRequestInput{
		UserID:          session.User.ID,
		TokenHash:       tokens.Hash(confirmationToken),
		Token:           confirmationToken,
		NewPasswordHash: newHash,
		ExpiresAt:       time.Now().UTC().Add(accountChangeTokenTTL),
		EmailDailyLimit: h.emailDailyLimit,
		IPAddress:       c.IP(),
		UserAgent:       c.Get("User-Agent"),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "User not found")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to change password")
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"data": user,
		"meta": fiber.Map{
			"confirmation_sent": true,
		},
	})
}

func (h AccountHandler) ChangeEmail(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "account repository unavailable")
	}

	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Authentication required")
	}

	var request changeEmailRequest
	if err := c.Bind().Body(&request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}
	if len(request.CurrentPassword) < 1 {
		return fiber.NewError(fiber.StatusBadRequest, "current_password is required")
	}
	newEmail, err := normalizeStrictEmail(request.NewEmail)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	if strings.EqualFold(newEmail, session.User.Email) {
		return fiber.NewError(fiber.StatusBadRequest, "new_email must be different from current email")
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	currentHash, err := h.store.GetPasswordHash(ctx, session.User.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "User not found")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to request email change")
	}

	ok, err = password.Verify(request.CurrentPassword, currentHash)
	if err != nil || !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Current password is incorrect")
	}

	confirmationToken, err := tokens.New("bb_ec_")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to request email change")
	}
	result, err := h.store.RequestEmailChange(ctx, users.EmailChangeRequestInput{
		UserID:          session.User.ID,
		NewEmail:        newEmail,
		TokenHash:       tokens.Hash(confirmationToken),
		Token:           confirmationToken,
		ExpiresAt:       time.Now().UTC().Add(accountChangeTokenTTL),
		EmailDailyLimit: h.emailDailyLimit,
		IPAddress:       c.IP(),
		UserAgent:       c.Get("User-Agent"),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "User not found")
	}
	if errors.Is(err, postgres.ErrUserConflict) {
		return fiber.NewError(fiber.StatusConflict, "Email already exists")
	}
	if errors.Is(err, postgres.ErrEmailDailyLimitReached) {
		return fiber.NewError(fiber.StatusTooManyRequests, "email confirmation is temporarily unavailable")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to request email change")
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"data": result.User,
		"meta": fiber.Map{
			"confirmation_sent": true,
			"pending_email":     result.PendingEmail,
		},
	})
}

func normalizeStrictEmail(value string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(value))
	if email == "" {
		return "", errors.New("new_email is required")
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email {
		return "", errors.New("new_email is invalid")
	}
	return email, nil
}
