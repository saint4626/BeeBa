package admin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"beeba.org/internal/domain/audit"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const (
	defaultAuditLimit = 50
	maxAuditLimit     = 100
)

type AuditStore interface {
	ListAuditLogs(ctx context.Context, filter audit.ListFilter) (audit.Page, error)
}

type AuditHandler struct {
	store AuditStore
}

func NewAuditHandler(store AuditStore) AuditHandler {
	return AuditHandler{store: store}
}

func (h AuditHandler) List(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "audit log unavailable")
	}

	filter, err := parseAuditFilter(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()

	page, err := h.store.ListAuditLogs(ctx, filter)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to load audit log")
	}

	return c.JSON(fiber.Map{
		"data": page.Items,
		"pagination": fiber.Map{
			"next_cursor": encodeAuditCursor(page.NextCursor),
			"limit":       filter.Limit,
		},
	})
}

func parseAuditFilter(c fiber.Ctx) (audit.ListFilter, error) {
	limit, err := parseAuditLimit(c.Query("limit"))
	if err != nil {
		return audit.ListFilter{}, err
	}
	cursor, err := decodeAuditCursor(c.Query("cursor"))
	if err != nil {
		return audit.ListFilter{}, err
	}

	actorUserID := strings.TrimSpace(c.Query("actor_user_id"))
	if actorUserID != "" {
		if _, err := uuid.Parse(actorUserID); err != nil {
			return audit.ListFilter{}, errors.New("actor_user_id is invalid")
		}
	}
	entityID := strings.TrimSpace(c.Query("entity_id"))
	if entityID != "" {
		if _, err := uuid.Parse(entityID); err != nil {
			return audit.ListFilter{}, errors.New("entity_id is invalid")
		}
	}

	action := strings.TrimSpace(c.Query("action"))
	entityType := strings.TrimSpace(c.Query("entity_type"))
	if len(action) > 120 {
		return audit.ListFilter{}, errors.New("action must be 120 characters or fewer")
	}
	if len(entityType) > 80 {
		return audit.ListFilter{}, errors.New("entity_type must be 80 characters or fewer")
	}

	return audit.ListFilter{
		Action:      action,
		EntityType:  entityType,
		ActorUserID: actorUserID,
		EntityID:    entityID,
		Limit:       limit,
		Cursor:      cursor,
	}, nil
}

func parseAuditLimit(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return defaultAuditLimit, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, errors.New("limit must be a number")
	}
	if value <= 0 {
		return 0, errors.New("limit must be greater than 0")
	}
	if value > maxAuditLimit {
		return maxAuditLimit, nil
	}
	return value, nil
}

func encodeAuditCursor(cursor audit.Cursor) string {
	if cursor.ID == "" || cursor.CreatedAt.IsZero() {
		return ""
	}
	payload, err := json.Marshal(cursor)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeAuditCursor(raw string) (audit.Cursor, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return audit.Cursor{}, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return audit.Cursor{}, errors.New("cursor is invalid")
	}
	var cursor audit.Cursor
	if err := json.Unmarshal(payload, &cursor); err != nil {
		return audit.Cursor{}, errors.New("cursor is invalid")
	}
	if cursor.ID == "" || cursor.CreatedAt.IsZero() {
		return audit.Cursor{}, errors.New("cursor is invalid")
	}
	if _, err := uuid.Parse(cursor.ID); err != nil {
		return audit.Cursor{}, errors.New("cursor is invalid")
	}
	return cursor, nil
}
