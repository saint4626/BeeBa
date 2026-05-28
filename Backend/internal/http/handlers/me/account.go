package me

import (
	"context"
	"errors"
	"strings"
	"time"

	"beeba.org/internal/domain/users"
	"beeba.org/internal/http/middleware/authz"
	"beeba.org/internal/security/password"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5"
)

type AccountStore interface {
	UpdateProfile(ctx context.Context, userID string, input users.ProfileUpdateInput) (users.PublicUser, error)
	GetPasswordHash(ctx context.Context, userID string) (string, error)
	ChangePassword(ctx context.Context, userID string, input users.PasswordChangeInput) error
}

type AccountHandler struct {
	store AccountStore
}

func NewAccountHandler(store AccountStore) AccountHandler {
	return AccountHandler{store: store}
}

type updateProfileRequest struct {
	DisplayName *string `json:"display_name"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

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

	err = h.store.ChangePassword(ctx, session.User.ID, users.PasswordChangeInput{
		NewPasswordHash:  newHash,
		CurrentSessionID: session.SessionID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "User not found")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to change password")
	}

	return c.SendStatus(fiber.StatusNoContent)
}
