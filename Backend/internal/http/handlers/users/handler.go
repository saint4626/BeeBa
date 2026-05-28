package users

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	usersdomain "beeba.org/internal/domain/users"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5"
)

type Store interface {
	GetPublicProfile(ctx context.Context, username string) (usersdomain.PublicProfile, error)
}

type Handler struct {
	store Store
}

func New(store Store) Handler {
	return Handler{store: store}
}

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9_-]{2,31}$`)

func (h Handler) Profile(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "user repository unavailable")
	}
	username := strings.TrimSpace(c.Params("username"))
	if !usernamePattern.MatchString(username) {
		return fiber.NewError(fiber.StatusBadRequest, "username is invalid")
	}

	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()

	profile, err := h.store.GetPublicProfile(ctx, username)
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "User not found")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to load user profile")
	}

	return c.JSON(fiber.Map{"data": profile})
}
