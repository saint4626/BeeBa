package admin

import (
	"context"
	"errors"
	"strings"
	"time"

	domain "beeba.org/internal/domain/servers"
	"beeba.org/internal/http/middleware/authz"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ServerAdminStore interface {
	Hide(ctx context.Context, actorUserID string, serverID string, reason string, ipAddress string, userAgent string) (domain.PublicDetail, error)
	Restore(ctx context.Context, actorUserID string, serverID string, reason string, ipAddress string, userAgent string) (domain.PublicDetail, error)
}

type ServerAdminHandler struct {
	store ServerAdminStore
}

func NewServerAdminHandler(store ServerAdminStore) ServerAdminHandler {
	return ServerAdminHandler{store: store}
}

func (h ServerAdminHandler) HideServer(c fiber.Ctx) error {
	return h.reviewServer(c, "hide")
}

func (h ServerAdminHandler) RestoreServer(c fiber.Ctx) error {
	return h.reviewServer(c, "restore")
}

func (h ServerAdminHandler) reviewServer(c fiber.Ctx, action string) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "server administration unavailable")
	}
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}
	serverID := strings.TrimSpace(c.Params("serverID"))
	if _, err := uuid.Parse(serverID); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "server id is invalid")
	}
	var req reviewRequest
	if len(c.Body()) > 0 {
		if err := c.Bind().Body(&req); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "request body is invalid")
		}
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if action == "hide" && req.Reason == "" {
		return fiber.NewError(fiber.StatusBadRequest, "reason is required")
	}
	if len([]rune(req.Reason)) > 500 {
		return fiber.NewError(fiber.StatusBadRequest, "reason must be 500 characters or fewer")
	}

	ctx, cancel := context.WithTimeout(c.Context(), 10*time.Second)
	defer cancel()
	var item domain.PublicDetail
	var err error
	switch action {
	case "hide":
		item, err = h.store.Hide(ctx, session.User.ID, serverID, req.Reason, c.IP(), c.Get("User-Agent"))
	case "restore":
		item, err = h.store.Restore(ctx, session.User.ID, serverID, req.Reason, c.IP(), c.Get("User-Agent"))
	default:
		return fiber.NewError(fiber.StatusInternalServerError, "unsupported server review action")
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "server not found")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to moderate server")
	}
	return c.JSON(fiber.Map{"data": item})
}
