package tags

import (
	"context"
	"errors"
	"time"

	domain "beeba.org/internal/domain/tags"

	"github.com/gofiber/fiber/v3"
)

var ErrUnavailable = errors.New("tags repository unavailable")

type Store interface {
	ListPublic(ctx context.Context) ([]domain.Tag, error)
}

type Handler struct {
	store Store
}

func New(store Store) Handler {
	return Handler{store: store}
}

func (h Handler) List(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, ErrUnavailable.Error())
	}

	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()

	items, err := h.store.ListPublic(ctx)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to load tags")
	}

	return c.JSON(fiber.Map{"data": items})
}
