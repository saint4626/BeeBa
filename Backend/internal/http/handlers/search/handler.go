package search

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode"

	contentdomain "beeba.org/internal/domain/content"
	searchdomain "beeba.org/internal/domain/search"

	"github.com/gofiber/fiber/v3"
)

const (
	defaultLimit  = 24
	maxLimit      = 50
	maxQueryBytes = 200
)

type Searcher interface {
	SearchContent(ctx context.Context, query searchdomain.ContentQuery) (searchdomain.ContentSearchResult, error)
}

type Store interface {
	ListPublishedByIDs(ctx context.Context, ids []string) ([]contentdomain.PublicItem, error)
}

type Handler struct {
	search Searcher
	store  Store
}

func New(search Searcher, store Store) Handler {
	return Handler{search: search, store: store}
}

func (h Handler) Content(c fiber.Ctx) error {
	if h.search == nil || h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "search dependencies unavailable")
	}
	query, err := parseQuery(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
	defer cancel()

	result, err := h.search.SearchContent(ctx, query)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to search content")
	}
	items, err := h.store.ListPublishedByIDs(ctx, result.IDs)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to hydrate search results")
	}

	return c.JSON(fiber.Map{
		"data": items,
		"pagination": fiber.Map{
			"next_cursor": encodeCursor(nextOffset(result)),
			"limit":       query.Limit,
			"offset":      query.Offset,
			"total":       result.EstimatedTotalHits,
		},
	})
}

func parseQuery(c fiber.Ctx) (searchdomain.ContentQuery, error) {
	limit, err := parseLimit(c.Query("limit"))
	if err != nil {
		return searchdomain.ContentQuery{}, err
	}
	cursor, err := decodeCursor(c.Query("cursor"))
	if err != nil {
		return searchdomain.ContentQuery{}, err
	}
	sort := strings.TrimSpace(c.Query("sort", "newest"))
	switch sort {
	case "newest", "likes", "downloads":
	default:
		return searchdomain.ContentQuery{}, errors.New("sort must be one of newest, likes, downloads")
	}
	includeNSFW, err := parseBool(c.Query("include_nsfw"))
	if err != nil {
		return searchdomain.ContentQuery{}, err
	}
	category := strings.TrimSpace(c.Query("category"))
	switch category {
	case "", "worlds", "avatars", "props", "prefabs":
	default:
		return searchdomain.ContentQuery{}, errors.New("category must be one of worlds, avatars, props, prefabs")
	}
	rawQuery := strings.TrimSpace(c.Query("q"))
	if len(rawQuery) > maxQueryBytes {
		return searchdomain.ContentQuery{}, errors.New("q is too long")
	}
	return searchdomain.ContentQuery{
		Query:        rawQuery,
		CategorySlug: category,
		Tags:         parseTags(c.Query("tags")),
		IncludeNSFW:  includeNSFW,
		Sort:         sort,
		Limit:        limit,
		Offset:       cursor.Offset,
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
	if limit <= 0 {
		return 0, errors.New("limit must be greater than 0")
	}
	if limit > maxLimit {
		return maxLimit, nil
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

type cursor struct {
	Offset int `json:"offset"`
}

func decodeCursor(raw string) (cursor, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return cursor{}, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return cursor{}, errors.New("cursor is invalid")
	}
	var value cursor
	if err := json.Unmarshal(payload, &value); err != nil {
		return cursor{}, errors.New("cursor is invalid")
	}
	if value.Offset < 0 {
		return cursor{}, errors.New("cursor is invalid")
	}
	return value, nil
}

func encodeCursor(offset *int) *string {
	if offset == nil {
		return nil
	}
	payload, err := json.Marshal(cursor{Offset: *offset})
	if err != nil {
		return nil
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	return &encoded
}

func nextOffset(result searchdomain.ContentSearchResult) *int {
	next := result.Offset + len(result.IDs)
	if next >= result.EstimatedTotalHits || len(result.IDs) == 0 {
		return nil
	}
	return &next
}
