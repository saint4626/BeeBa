package admin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"beeba.org/internal/config"
	"beeba.org/internal/domain/moderation"
	"beeba.org/internal/http/middleware/authz"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ModerationStore interface {
	ListQueue(ctx context.Context, filter moderation.QueueFilter) ([]moderation.QueueItem, error)
	GetApprovalCandidate(ctx context.Context, contentID string) (moderation.ApprovalCandidate, error)
	Approve(ctx context.Context, input moderation.ApproveInput) (moderation.Approved, error)
	Reject(ctx context.Context, input moderation.ReviewInput) (moderation.Reviewed, error)
	Hide(ctx context.Context, input moderation.ReviewInput) (moderation.Reviewed, error)
	Restore(ctx context.Context, input moderation.ReviewInput) (moderation.Reviewed, error)
	ListCommentQueue(ctx context.Context, filter moderation.QueueFilter) ([]moderation.CommentQueueItem, error)
	ApproveComment(ctx context.Context, input moderation.CommentReviewInput) (moderation.CommentReviewed, error)
	HideComment(ctx context.Context, input moderation.CommentReviewInput) (moderation.CommentReviewed, error)
	ListReportQueue(ctx context.Context, filter moderation.QueueFilter) ([]moderation.ReportQueueItem, error)
	ReviewReport(ctx context.Context, input moderation.ReportReviewInput) (moderation.ReportReviewed, error)
}

type PromotionStore interface {
	EnsureBucket(ctx context.Context, bucket string) error
	CopyObject(ctx context.Context, sourceBucket string, sourceKey string, destinationBucket string, destinationKey string) error
	RemoveObject(ctx context.Context, bucket string, key string) error
}

type ModerationHandler struct {
	cfg     config.Config
	log     *slog.Logger
	store   ModerationStore
	objects PromotionStore
}

type reviewRequest struct {
	Reason string `json:"reason"`
}

func NewModerationHandler(cfg config.Config, log *slog.Logger, store ModerationStore, objects PromotionStore) ModerationHandler {
	return ModerationHandler{cfg: cfg, log: log, store: store, objects: objects}
}

func (h ModerationHandler) List(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "moderation dependencies unavailable")
	}

	status := strings.TrimSpace(c.Query("status"))
	if status != "" && !allowedModerationStatus(status) {
		return fiber.NewError(fiber.StatusBadRequest, "status is invalid")
	}
	limit, err := parseModerationLimit(c.Query("limit"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()

	items, err := h.store.ListQueue(ctx, moderation.QueueFilter{
		Status: status,
		Limit:  limit,
	})
	if err != nil {
		h.logError("list", "", err)
		return fiber.NewError(fiber.StatusInternalServerError, "failed to load moderation queue")
	}

	return c.JSON(fiber.Map{
		"data": items,
		"pagination": fiber.Map{
			"limit": limit,
		},
	})
}

func (h ModerationHandler) Approve(c fiber.Ctx) error {
	if h.store == nil || h.objects == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "moderation dependencies unavailable")
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

	candidate, err := h.store.GetApprovalCandidate(ctx, contentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "content not found")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to load content")
	}
	if candidate.Status != "pending_moderation" && candidate.Status != "approved" {
		return fiber.NewError(fiber.StatusConflict, "content is not ready for approval")
	}
	if candidate.ScanStatus != "clean" {
		return fiber.NewError(fiber.StatusConflict, "content file scan is not approved")
	}
	if candidate.Bucket == h.cfg.QuarantineBucket {
		if err := h.objects.EnsureBucket(ctx, h.cfg.PrivateBucket); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "failed to prepare approved storage")
		}
	}

	newBucket := candidate.Bucket
	newStorageKey := candidate.StorageKey
	if candidate.Bucket == h.cfg.QuarantineBucket {
		newBucket = h.cfg.PrivateBucket
		newStorageKey = fmt.Sprintf("content/%s/%s.bee", candidate.ContentID, candidate.FileID)
		if err := h.objects.CopyObject(ctx, candidate.Bucket, candidate.StorageKey, newBucket, newStorageKey); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "failed to promote content file")
		}
	}

	approved, err := h.store.Approve(ctx, moderation.ApproveInput{
		ContentID:     candidate.ContentID,
		FileID:        candidate.FileID,
		ActorUserID:   session.User.ID,
		NewBucket:     newBucket,
		NewStorageKey: newStorageKey,
		IPAddress:     c.IP(),
		UserAgent:     c.Get("User-Agent"),
	})
	if err != nil {
		if candidate.Bucket == h.cfg.QuarantineBucket {
			_ = h.objects.RemoveObject(ctx, newBucket, newStorageKey)
		}
		h.logError("approve", candidate.ContentID, err)
		return fiber.NewError(fiber.StatusInternalServerError, "failed to approve content")
	}

	if candidate.Bucket == h.cfg.QuarantineBucket {
		_ = h.objects.RemoveObject(ctx, candidate.Bucket, candidate.StorageKey)
	}

	return c.JSON(fiber.Map{"data": approved})
}

func (h ModerationHandler) Reject(c fiber.Ctx) error {
	return h.review(c, "reject")
}

func (h ModerationHandler) Hide(c fiber.Ctx) error {
	return h.review(c, "hide")
}

func (h ModerationHandler) Restore(c fiber.Ctx) error {
	return h.review(c, "restore")
}

func (h ModerationHandler) ListComments(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "moderation dependencies unavailable")
	}
	status := strings.TrimSpace(c.Query("status"))
	if status != "" && !allowedCommentStatus(status) {
		return fiber.NewError(fiber.StatusBadRequest, "status is invalid")
	}
	limit, err := parseModerationLimit(c.Query("limit"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()
	items, err := h.store.ListCommentQueue(ctx, moderation.QueueFilter{Status: status, Limit: limit})
	if err != nil {
		h.logError("list_comments", "", err)
		return fiber.NewError(fiber.StatusInternalServerError, "failed to load comment moderation queue")
	}
	return c.JSON(fiber.Map{"data": items, "pagination": fiber.Map{"limit": limit}})
}

func (h ModerationHandler) ApproveComment(c fiber.Ctx) error {
	return h.reviewComment(c, "approve")
}

func (h ModerationHandler) HideComment(c fiber.Ctx) error {
	return h.reviewComment(c, "hide")
}

func (h ModerationHandler) reviewComment(c fiber.Ctx, action string) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "moderation dependencies unavailable")
	}
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}
	commentID := strings.TrimSpace(c.Params("commentID"))
	if _, err := uuid.Parse(commentID); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "comment id is invalid")
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
	if len(req.Reason) > 500 {
		return fiber.NewError(fiber.StatusBadRequest, "reason must be 500 characters or fewer")
	}

	ctx, cancel := context.WithTimeout(c.Context(), 10*time.Second)
	defer cancel()
	input := moderation.CommentReviewInput{
		CommentID:   commentID,
		ActorUserID: session.User.ID,
		Reason:      req.Reason,
		IPAddress:   c.IP(),
		UserAgent:   c.Get("User-Agent"),
	}
	var reviewed moderation.CommentReviewed
	var err error
	switch action {
	case "approve":
		reviewed, err = h.store.ApproveComment(ctx, input)
	case "hide":
		reviewed, err = h.store.HideComment(ctx, input)
	default:
		return fiber.NewError(fiber.StatusInternalServerError, "unsupported comment moderation action")
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusConflict, "comment is not in a valid state for this action")
	}
	if err != nil {
		h.logError("comment_"+action, commentID, err)
		return fiber.NewError(fiber.StatusInternalServerError, "failed to moderate comment")
	}
	return c.JSON(fiber.Map{"data": reviewed})
}

func (h ModerationHandler) ListReports(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "moderation dependencies unavailable")
	}
	status := strings.TrimSpace(c.Query("status"))
	if status != "" && !allowedReportStatus(status) {
		return fiber.NewError(fiber.StatusBadRequest, "status is invalid")
	}
	limit, err := parseModerationLimit(c.Query("limit"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()
	items, err := h.store.ListReportQueue(ctx, moderation.QueueFilter{Status: status, Limit: limit})
	if err != nil {
		h.logError("list_reports", "", err)
		return fiber.NewError(fiber.StatusInternalServerError, "failed to load report moderation queue")
	}
	return c.JSON(fiber.Map{"data": items, "pagination": fiber.Map{"limit": limit}})
}

type reportReviewRequest struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func (h ModerationHandler) ReviewReport(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "moderation dependencies unavailable")
	}
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}
	reportID := strings.TrimSpace(c.Params("reportID"))
	if _, err := uuid.Parse(reportID); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "report id is invalid")
	}
	var req reportReviewRequest
	if err := c.Bind().Body(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "request body is invalid")
	}
	req.Status = strings.TrimSpace(req.Status)
	req.Reason = strings.TrimSpace(req.Reason)
	if !allowedReportReviewStatus(req.Status) {
		return fiber.NewError(fiber.StatusBadRequest, "status must be in_review, resolved, or rejected")
	}
	if (req.Status == "resolved" || req.Status == "rejected") && req.Reason == "" {
		return fiber.NewError(fiber.StatusBadRequest, "reason is required")
	}
	if len(req.Reason) > 500 {
		return fiber.NewError(fiber.StatusBadRequest, "reason must be 500 characters or fewer")
	}

	ctx, cancel := context.WithTimeout(c.Context(), 10*time.Second)
	defer cancel()
	reviewed, err := h.store.ReviewReport(ctx, moderation.ReportReviewInput{
		ReportID:    reportID,
		ActorUserID: session.User.ID,
		Status:      req.Status,
		Reason:      req.Reason,
		IPAddress:   c.IP(),
		UserAgent:   c.Get("User-Agent"),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusConflict, "report is not in a valid state for this action")
	}
	if err != nil {
		h.logError("report_review", reportID, err)
		return fiber.NewError(fiber.StatusInternalServerError, "failed to review report")
	}
	return c.JSON(fiber.Map{"data": reviewed})
}

func (h ModerationHandler) review(c fiber.Ctx, action string) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "moderation dependencies unavailable")
	}
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}

	contentID := strings.TrimSpace(c.Params("contentID"))
	if _, err := uuid.Parse(contentID); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "content id is invalid")
	}

	var req reviewRequest
	if len(c.Body()) > 0 {
		if err := c.Bind().Body(&req); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "request body is invalid")
		}
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if action != "restore" && req.Reason == "" {
		return fiber.NewError(fiber.StatusBadRequest, "reason is required")
	}
	if len(req.Reason) > 500 {
		return fiber.NewError(fiber.StatusBadRequest, "reason must be 500 characters or fewer")
	}

	ctx, cancel := context.WithTimeout(c.Context(), 10*time.Second)
	defer cancel()

	input := moderation.ReviewInput{
		ContentID:   contentID,
		ActorUserID: session.User.ID,
		Reason:      req.Reason,
		IPAddress:   c.IP(),
		UserAgent:   c.Get("User-Agent"),
	}

	var reviewed moderation.Reviewed
	var err error
	switch action {
	case "reject":
		reviewed, err = h.store.Reject(ctx, input)
	case "hide":
		reviewed, err = h.store.Hide(ctx, input)
	case "restore":
		reviewed, err = h.store.Restore(ctx, input)
	default:
		return fiber.NewError(fiber.StatusInternalServerError, "unsupported moderation action")
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusConflict, "content is not in a valid state for this action")
	}
	if err != nil {
		h.logError(action, contentID, err)
		return fiber.NewError(fiber.StatusInternalServerError, "failed to moderate content")
	}

	return c.JSON(fiber.Map{"data": reviewed})
}

func parseModerationLimit(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 50, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil {
		return 0, errors.New("limit must be an integer")
	}
	if limit < 1 {
		return 0, errors.New("limit must be greater than 0")
	}
	if limit > 100 {
		return 100, nil
	}
	return limit, nil
}

func allowedModerationStatus(status string) bool {
	switch status {
	case "pending_scan", "scan_failed", "pending_moderation", "approved", "published", "rejected", "hidden":
		return true
	default:
		return false
	}
}

func allowedCommentStatus(status string) bool {
	switch status {
	case "visible", "hidden", "deleted", "pending_moderation":
		return true
	default:
		return false
	}
}

func allowedReportStatus(status string) bool {
	switch status {
	case "open", "in_review", "resolved", "rejected":
		return true
	default:
		return false
	}
}

func allowedReportReviewStatus(status string) bool {
	switch status {
	case "in_review", "resolved", "rejected":
		return true
	default:
		return false
	}
}

func (h ModerationHandler) logError(action string, contentID string, err error) {
	if h.log == nil {
		return
	}
	h.log.Error(
		"moderation_action_failed",
		slog.String("action", action),
		slog.String("content_id", contentID),
		slog.String("error", err.Error()),
	)
}
