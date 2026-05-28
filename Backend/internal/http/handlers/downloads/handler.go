package downloads

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"beeba.org/internal/config"
	downloaddomain "beeba.org/internal/domain/downloads"
	"beeba.org/internal/http/middleware/authz"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Store interface {
	GetPublicTarget(ctx context.Context, contentID string, quarantineBucket string) (downloaddomain.Target, error)
	GetOwnerTarget(ctx context.Context, contentID string, userID string, quarantineBucket string) (downloaddomain.Target, error)
	RecordEvent(ctx context.Context, input downloaddomain.EventInput) error
}

type ObjectStore interface {
	GetObject(ctx context.Context, bucket string, key string) (io.ReadCloser, error)
}

type Handler struct {
	cfg     config.Config
	store   Store
	objects ObjectStore
}

func New(cfg config.Config, store Store, objects ObjectStore) Handler {
	return Handler{cfg: cfg, store: store, objects: objects}
}

func (h Handler) Public(c fiber.Ctx) error {
	if h.store == nil || h.objects == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "download dependencies unavailable")
	}
	contentID := strings.TrimSpace(c.Params("contentID"))
	if _, err := uuid.Parse(contentID); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "content id is invalid")
	}

	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()

	target, err := h.store.GetPublicTarget(ctx, contentID, h.cfg.QuarantineBucket)
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "download not found")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to load download")
	}
	return h.stream(c, target, nil)
}

func (h Handler) Owner(c fiber.Ctx) error {
	if h.store == nil || h.objects == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "download dependencies unavailable")
	}
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}
	contentID := strings.TrimSpace(c.Params("contentID"))
	if _, err := uuid.Parse(contentID); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "content id is invalid")
	}

	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()

	target, err := h.store.GetOwnerTarget(ctx, contentID, session.User.ID, h.cfg.QuarantineBucket)
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "download not found")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to load owner download")
	}
	return h.stream(c, target, &session.User.ID)
}

func (h Handler) stream(c fiber.Ctx, target downloaddomain.Target, userID *string) error {
	object, err := h.objects.GetObject(c.Context(), target.Bucket, target.StorageKey)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to open download")
	}
	if err := h.store.RecordEvent(c.Context(), downloaddomain.EventInput{
		ContentID: target.ContentID,
		FileID:    target.FileID,
		UserID:    userID,
		IPAddress: c.IP(),
		UserAgent: c.Get("User-Agent"),
	}); err != nil {
		_ = object.Close()
		return fiber.NewError(fiber.StatusInternalServerError, "failed to record download")
	}
	c.Set(fiber.HeaderContentType, "application/octet-stream")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%q", target.Filename))
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("X-BeeBa-Content-ID", target.ContentID)
	c.Set("X-BeeBa-File-ID", target.FileID)
	return c.SendStream(object, int(target.FileSize))
}
