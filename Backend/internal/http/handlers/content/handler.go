package content

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode"

	domain "beeba.org/internal/domain/content"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	defaultLimit = 24
	maxLimit     = 50
)

var ErrUnavailable = errors.New("content repository unavailable")

type Store interface {
	ListPublished(ctx context.Context, filter domain.ListFilter) (domain.Page, error)
	GetPublished(ctx context.Context, contentID string) (domain.PublicDetail, error)
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

	filter, err := parseFilter(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()

	page, err := h.store.ListPublished(ctx, filter)
	if err != nil {
		if strings.Contains(err.Error(), "unsupported content sort") {
			return fiber.NewError(fiber.StatusBadRequest, "Unsupported content sort")
		}
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to load content")
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
		return fiber.NewError(fiber.StatusServiceUnavailable, ErrUnavailable.Error())
	}
	contentID := strings.TrimSpace(c.Params("contentID"))
	if _, err := uuid.Parse(contentID); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "content id is invalid")
	}

	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()

	item, err := h.store.GetPublished(ctx, contentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "Content item not found")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to load content")
	}
	return c.JSON(fiber.Map{"data": item})
}

func parseFilter(c fiber.Ctx) (domain.ListFilter, error) {
	limit, err := parseLimit(c.Query("limit"))
	if err != nil {
		return domain.ListFilter{}, err
	}
	if limit <= 0 {
		return domain.ListFilter{}, errors.New("limit must be greater than 0")
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	cursor, err := decodeCursor(c.Query("cursor"))
	if err != nil {
		return domain.ListFilter{}, err
	}

	sort := strings.TrimSpace(c.Query("sort", "newest"))
	switch sort {
	case "newest", "likes", "downloads":
	default:
		return domain.ListFilter{}, errors.New("sort must be one of newest, likes, downloads")
	}

	includeNSFW, err := parseBool(c.Query("include_nsfw"))
	if err != nil {
		return domain.ListFilter{}, err
	}

	return domain.ListFilter{
		CategorySlug: strings.TrimSpace(c.Query("category")),
		Author:       strings.TrimSpace(c.Query("author")),
		Query:        strings.TrimSpace(c.Query("q")),
		Tags:         parseTags(c.Query("tags")),
		IncludeNSFW:  includeNSFW,
		Sort:         sort,
		Limit:        limit,
		Cursor:       cursor,
	}, nil
}

func parseTags(raw string) []string {
	parts := strings.Split(raw, ",")
	tags := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		tag := strings.ToLower(strings.TrimSpace(part))
		if tag == "" || !validTagSlug(tag) {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
	}
	return tags
}

func validTagSlug(value string) bool {
	if len(value) > 64 {
		return false
	}
	for _, r := range value {
		if unicode.IsLower(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

func parseLimit(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultLimit, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil {
		return 0, errors.New("limit must be an integer")
	}
	return limit, nil
}

func parseBool(raw string) (bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, errors.New("include_nsfw must be a boolean")
	}
	return value, nil
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
