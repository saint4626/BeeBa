package me

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"beeba.org/internal/domain/content"
	"beeba.org/internal/http/middleware/authz"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5"
)

const (
	defaultOwnedLimit = 24
	maxOwnedLimit     = 50
)

type ContentStore interface {
	ListOwned(ctx context.Context, ownerID string, filter content.OwnerListFilter) (content.OwnerPage, error)
	OwnerStorageUsage(ctx context.Context, ownerID string, limitBytes int64) (content.OwnerStorageUsage, error)
	UpdateOwned(ctx context.Context, ownerID string, contentID string, input content.OwnerUpdateInput) (content.OwnerItem, error)
	DeleteOwned(ctx context.Context, ownerID string, contentID string) error
}

type ContentHandler struct {
	store                 ContentStore
	userStorageQuotaBytes int64
}

func NewContentHandler(store ContentStore, userStorageQuotaBytes int64) ContentHandler {
	return ContentHandler{store: store, userStorageQuotaBytes: userStorageQuotaBytes}
}

func (h ContentHandler) List(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "content repository unavailable")
	}
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}

	filter, err := parseOwnedFilter(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()

	page, err := h.store.ListOwned(ctx, session.User.ID, filter)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to load owned content")
	}
	storage, err := h.store.OwnerStorageUsage(ctx, session.User.ID, h.userStorageQuotaBytes)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to load storage usage")
	}

	return c.JSON(fiber.Map{
		"data": page.Items,
		"pagination": fiber.Map{
			"next_cursor": encodeOwnedCursor(page.NextCursor),
			"limit":       filter.Limit,
		},
		"storage": storage,
	})
}

type updateOwnedContentRequest struct {
	Title       *string  `json:"title"`
	Description *string  `json:"description"`
	Visibility  *string  `json:"visibility"`
	NSFW        *bool    `json:"nsfw"`
	Tags        []string `json:"tags"`
}

func (h ContentHandler) Update(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "content repository unavailable")
	}
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}
	contentID := strings.TrimSpace(c.Params("contentID"))
	if contentID == "" {
		return fiber.NewError(fiber.StatusBadRequest, "content id is required")
	}

	var request updateOwnedContentRequest
	if err := c.Bind().Body(&request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}

	input, err := normalizeUpdateRequest(request)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
	defer cancel()

	item, err := h.store.UpdateOwned(ctx, session.User.ID, contentID, input)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fiber.NewError(fiber.StatusNotFound, "owned content not found")
		}
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to update owned content")
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{"data": item})
}

func (h ContentHandler) Delete(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "content repository unavailable")
	}
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}
	contentID := strings.TrimSpace(c.Params("contentID"))
	if contentID == "" {
		return fiber.NewError(fiber.StatusBadRequest, "content id is required")
	}

	ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
	defer cancel()

	if err := h.store.DeleteOwned(ctx, session.User.ID, contentID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fiber.NewError(fiber.StatusNotFound, "owned content not found")
		}
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to delete owned content")
	}
	return c.SendStatus(http.StatusNoContent)
}

func parseOwnedFilter(c fiber.Ctx) (content.OwnerListFilter, error) {
	limit, err := parseOwnedLimit(c.Query("limit"))
	if err != nil {
		return content.OwnerListFilter{}, err
	}
	if limit <= 0 {
		return content.OwnerListFilter{}, errors.New("limit must be greater than 0")
	}
	if limit > maxOwnedLimit {
		limit = maxOwnedLimit
	}

	cursor, err := decodeOwnedCursor(c.Query("cursor"))
	if err != nil {
		return content.OwnerListFilter{}, err
	}

	status := strings.TrimSpace(c.Query("status"))
	if status != "" && !allowedOwnedStatus(status) {
		return content.OwnerListFilter{}, errors.New("status is invalid")
	}
	visibility := strings.TrimSpace(c.Query("visibility"))
	if visibility != "" && visibility != "public" && visibility != "private" {
		return content.OwnerListFilter{}, errors.New("visibility must be public or private")
	}

	return content.OwnerListFilter{
		Query:      strings.TrimSpace(c.Query("q")),
		Status:     status,
		Visibility: visibility,
		Limit:      limit,
		Cursor:     cursor,
	}, nil
}

func normalizeUpdateRequest(request updateOwnedContentRequest) (content.OwnerUpdateInput, error) {
	input := content.OwnerUpdateInput{}
	hasField := false
	if request.Title != nil {
		title := strings.TrimSpace(*request.Title)
		if title == "" {
			return content.OwnerUpdateInput{}, errors.New("title is required")
		}
		if len([]rune(title)) > 140 {
			return content.OwnerUpdateInput{}, errors.New("title must be 140 characters or fewer")
		}
		input.Title = &title
		hasField = true
	}
	if request.Description != nil {
		description := strings.TrimSpace(*request.Description)
		if len([]rune(description)) > 4000 {
			return content.OwnerUpdateInput{}, errors.New("description must be 4000 characters or fewer")
		}
		input.Description = &description
		hasField = true
	}
	if request.Visibility != nil {
		visibility := strings.ToLower(strings.TrimSpace(*request.Visibility))
		if visibility != "public" && visibility != "private" {
			return content.OwnerUpdateInput{}, errors.New("visibility must be public or private")
		}
		input.Visibility = &visibility
		hasField = true
	}
	if request.NSFW != nil {
		input.NSFW = request.NSFW
		hasField = true
	}
	if request.Tags != nil {
		tags, err := normalizeTags(request.Tags)
		if err != nil {
			return content.OwnerUpdateInput{}, err
		}
		input.Tags = &tags
		hasField = true
	}
	if !hasField {
		return content.OwnerUpdateInput{}, errors.New("at least one editable field is required")
	}
	return input, nil
}

func normalizeTags(rawTags []string) ([]content.TagInput, error) {
	if len(rawTags) > 16 {
		return nil, errors.New("tags must contain 16 items or fewer")
	}
	tags := make([]content.TagInput, 0, len(rawTags))
	seen := map[string]struct{}{}
	for _, raw := range rawTags {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if len([]rune(name)) > 40 {
			return nil, errors.New("each tag must be 40 characters or fewer")
		}
		slug := tagSlug(name)
		if slug == "" {
			return nil, errors.New("each tag must include a letter or number")
		}
		if _, ok := seen[slug]; ok {
			continue
		}
		seen[slug] = struct{}{}
		tags = append(tags, content.TagInput{Slug: slug, Name: name})
	}
	return tags, nil
}

func tagSlug(value string) string {
	var builder strings.Builder
	lastHyphen := false
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastHyphen = false
			continue
		}
		if !lastHyphen && builder.Len() > 0 {
			builder.WriteByte('-')
			lastHyphen = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func parseOwnedLimit(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultOwnedLimit, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil {
		return 0, errors.New("limit must be an integer")
	}
	return limit, nil
}

func allowedOwnedStatus(status string) bool {
	switch status {
	case "draft", "uploaded", "pending_scan", "scan_failed", "pending_moderation", "approved", "published", "rejected", "hidden", "deleted":
		return true
	default:
		return false
	}
}

func encodeOwnedCursor(cursor *content.Cursor) *string {
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

func decodeOwnedCursor(raw string) (content.Cursor, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return content.Cursor{}, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return content.Cursor{}, errors.New("cursor is invalid")
	}
	var cursor content.Cursor
	if err := json.Unmarshal(payload, &cursor); err != nil {
		return content.Cursor{}, errors.New("cursor is invalid")
	}
	if cursor.ID == "" || cursor.SortValue == "" {
		return content.Cursor{}, errors.New("cursor is invalid")
	}
	return cursor, nil
}
