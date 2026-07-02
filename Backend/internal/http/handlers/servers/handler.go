package servers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	domain "beeba.org/internal/domain/servers"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	defaultServerLimit = 24
	maxServerLimit     = 50
)

type Store interface {
	ListPublished(ctx context.Context, filter domain.ListFilter) (domain.Page, error)
	GetPublished(ctx context.Context, serverID string) (domain.PublicDetail, error)
}

type Handler struct {
	store Store
}

func New(store Store) Handler {
	return Handler{store: store}
}

func (h Handler) List(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "server catalog unavailable")
	}
	filter, err := parseListFilter(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()
	page, err := h.store.ListPublished(ctx, filter)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to load servers")
	}
	return c.JSON(fiber.Map{
		"data": page.Items,
		"pagination": fiber.Map{
			"next_cursor": encodeCursor(page.NextCursor),
			"limit":       filter.Limit,
		},
	})
}

func (h Handler) Detail(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "server catalog unavailable")
	}
	serverID := strings.TrimSpace(c.Params("serverID"))
	if _, err := uuid.Parse(serverID); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "server id is invalid")
	}
	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()
	item, err := h.store.GetPublished(ctx, serverID)
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "server not found")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to load server")
	}
	return c.JSON(fiber.Map{"data": item})
}

func parseListFilter(c fiber.Ctx) (domain.ListFilter, error) {
	limit, err := parseLimit(c.Query("limit"))
	if err != nil {
		return domain.ListFilter{}, err
	}
	cursor, err := decodeCursor(c.Query("cursor"))
	if err != nil {
		return domain.ListFilter{}, err
	}
	sort := strings.TrimSpace(c.Query("sort"))
	if sort != "" && sort != "newest" && sort != "players" && sort != "updated" {
		return domain.ListFilter{}, errors.New("sort is invalid")
	}
	language, err := domain.NormalizeLanguage(c.Query("language"))
	if err != nil {
		return domain.ListFilter{}, err
	}
	tags := parseCSV(c.Query("tags"))
	return domain.ListFilter{
		Query:      strings.TrimSpace(c.Query("q")),
		Region:     strings.TrimSpace(c.Query("region")),
		Language:   language,
		Tags:       tags,
		OnlineOnly: c.Query("online") == "1" || strings.EqualFold(c.Query("online"), "true"),
		IncludeNSFW: c.Query("nsfw") == "1" ||
			strings.EqualFold(c.Query("nsfw"), "true") ||
			c.Query("include_nsfw") == "1" ||
			strings.EqualFold(c.Query("include_nsfw"), "true"),
		Sort:   sort,
		Limit:  limit,
		Cursor: cursor,
	}, nil
}

func parseLimit(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultServerLimit, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return 0, errors.New("limit must be a positive integer")
	}
	if limit > maxServerLimit {
		return maxServerLimit, nil
	}
	return limit, nil
}

func parseCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	return values
}

func encodeCursor(cursor *domain.Cursor) *string {
	if cursor == nil || cursor.ID == "" || cursor.SortValue == "" {
		return nil
	}
	payload, err := json.Marshal(cursor)
	if err != nil {
		return nil
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	return &encoded
}

func decodeCursor(raw string) (domain.Cursor, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return domain.Cursor{}, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return domain.Cursor{}, errors.New("cursor is invalid")
	}
	var cursor domain.Cursor
	if err := json.Unmarshal(payload, &cursor); err != nil {
		return domain.Cursor{}, errors.New("cursor is invalid")
	}
	if cursor.ID == "" || cursor.SortValue == "" {
		return domain.Cursor{}, errors.New("cursor is invalid")
	}
	return cursor, nil
}
