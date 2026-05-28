package social

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"beeba.org/internal/config"
	socialdomain "beeba.org/internal/domain/social"
	"beeba.org/internal/http/middleware/authz"
	"beeba.org/internal/ratelimit"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	defaultCommentLimit       = 24
	maxCommentLimit           = 50
	maxCommentBodyRunes       = 2000
	maxReportReasonRunes      = 120
	maxReportDetailsBodyRunes = 2000
)

type Store interface {
	ListComments(ctx context.Context, contentID string, filter socialdomain.CommentFilter) (socialdomain.CommentPage, error)
	CreateComment(ctx context.Context, input socialdomain.CommentInput) (socialdomain.Comment, error)
	Like(ctx context.Context, contentID string, userID string) (socialdomain.LikeResult, error)
	Unlike(ctx context.Context, contentID string, userID string) (socialdomain.LikeResult, error)
	ReportContent(ctx context.Context, input socialdomain.ReportInput) (socialdomain.Report, error)
}

type Handler struct {
	store      Store
	limiter    ratelimit.Limiter
	rateLimits config.RateLimitConfig
}

func New(store Store, limiter ratelimit.Limiter, rateLimits config.RateLimitConfig) Handler {
	return Handler{store: store, limiter: limiter, rateLimits: rateLimits}
}

func (h Handler) ListComments(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "social repository unavailable")
	}
	contentID, err := parseUUIDParam(c, "contentID")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	filter, err := parseCommentFilter(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()

	page, err := h.store.ListComments(ctx, contentID, filter)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to load comments")
	}
	return c.JSON(fiber.Map{
		"data": page.Items,
		"pagination": fiber.Map{
			"next_cursor": encodeCommentCursor(page.NextCursor),
			"limit":       filter.Limit,
		},
	})
}

type createCommentRequest struct {
	ParentID *string `json:"parent_id"`
	Body     string  `json:"body"`
}

func (h Handler) CreateComment(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "social repository unavailable")
	}
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}
	if err := h.allow(c, session.User.ID, "comments", h.rateLimits.SocialComments); err != nil {
		return err
	}
	contentID, err := parseUUIDParam(c, "contentID")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	var request createCommentRequest
	if err := c.Bind().Body(&request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	body := strings.TrimSpace(request.Body)
	if body == "" {
		return fiber.NewError(fiber.StatusBadRequest, "comment body is required")
	}
	if len([]rune(body)) > maxCommentBodyRunes {
		return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("comment body must be %d characters or fewer", maxCommentBodyRunes))
	}
	if request.ParentID != nil {
		parentID := strings.TrimSpace(*request.ParentID)
		if _, err := uuid.Parse(parentID); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "parent_id is invalid")
		}
		request.ParentID = &parentID
	}

	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()

	comment, err := h.store.CreateComment(ctx, socialdomain.CommentInput{
		ContentID: contentID,
		UserID:    session.User.ID,
		ParentID:  request.ParentID,
		Body:      body,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "Content item not found")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to create comment")
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": comment})
}

func (h Handler) Like(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "social repository unavailable")
	}
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}
	if err := h.allow(c, session.User.ID, "likes", h.rateLimits.SocialLikes); err != nil {
		return err
	}
	contentID, err := parseUUIDParam(c, "contentID")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()
	result, err := h.store.Like(ctx, contentID, session.User.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "Content item not found")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to like content")
	}
	return c.JSON(fiber.Map{"data": result})
}

func (h Handler) Unlike(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "social repository unavailable")
	}
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}
	contentID, err := parseUUIDParam(c, "contentID")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()
	result, err := h.store.Unlike(ctx, contentID, session.User.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "Content item not found")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to unlike content")
	}
	return c.JSON(fiber.Map{"data": result})
}

type reportContentRequest struct {
	Reason  string `json:"reason"`
	Details string `json:"details"`
}

func (h Handler) Report(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "social repository unavailable")
	}
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}
	if err := h.allow(c, session.User.ID, "reports", h.rateLimits.SocialReports); err != nil {
		return err
	}
	contentID, err := parseUUIDParam(c, "contentID")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	var request reportContentRequest
	if err := c.Bind().Body(&request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	reason := strings.TrimSpace(request.Reason)
	details := strings.TrimSpace(request.Details)
	if reason == "" {
		return fiber.NewError(fiber.StatusBadRequest, "report reason is required")
	}
	if len([]rune(reason)) > maxReportReasonRunes {
		return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("report reason must be %d characters or fewer", maxReportReasonRunes))
	}
	if len([]rune(details)) > maxReportDetailsBodyRunes {
		return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("report details must be %d characters or fewer", maxReportDetailsBodyRunes))
	}

	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()
	report, err := h.store.ReportContent(ctx, socialdomain.ReportInput{
		ContentID: contentID,
		UserID:    session.User.ID,
		Reason:    reason,
		Details:   details,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "Content item not found")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to report content")
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": report})
}

func (h Handler) allow(c fiber.Ctx, userID string, action string, rule config.RateLimitRule) error {
	if h.limiter == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "rate limiter unavailable")
	}
	ctx, cancel := context.WithTimeout(c.Context(), time.Second)
	defer cancel()
	key := fmt.Sprintf("rl:%s:%s", action, userID)
	ok, err := h.limiter.Allow(ctx, key, rule.Limit, rule.Window)
	if err != nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "rate limiter unavailable")
	}
	if !ok {
		return fiber.NewError(fiber.StatusTooManyRequests, "rate limit exceeded")
	}
	return nil
}

func parseUUIDParam(c fiber.Ctx, name string) (string, error) {
	value := strings.TrimSpace(c.Params(name))
	if _, err := uuid.Parse(value); err != nil {
		return "", fmt.Errorf("%s is invalid", strings.ToLower(name))
	}
	return value, nil
}

func parseCommentFilter(c fiber.Ctx) (socialdomain.CommentFilter, error) {
	limit, err := parseLimit(c.Query("limit"))
	if err != nil {
		return socialdomain.CommentFilter{}, err
	}
	cursor, err := decodeCommentCursor(c.Query("cursor"))
	if err != nil {
		return socialdomain.CommentFilter{}, err
	}
	return socialdomain.CommentFilter{Limit: limit, Cursor: cursor}, nil
}

func parseLimit(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultCommentLimit, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return 0, errors.New("limit must be a positive integer")
	}
	if limit > maxCommentLimit {
		return maxCommentLimit, nil
	}
	return limit, nil
}

func encodeCommentCursor(cursor *socialdomain.Cursor) *string {
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

func decodeCommentCursor(raw string) (socialdomain.Cursor, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return socialdomain.Cursor{}, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return socialdomain.Cursor{}, errors.New("cursor is invalid")
	}
	var cursor socialdomain.Cursor
	if err := json.Unmarshal(payload, &cursor); err != nil {
		return socialdomain.Cursor{}, errors.New("cursor is invalid")
	}
	if cursor.ID == "" || cursor.SortValue == "" {
		return socialdomain.Cursor{}, errors.New("cursor is invalid")
	}
	return cursor, nil
}
