package admin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"beeba.org/internal/domain/jobs"
	"beeba.org/internal/http/middleware/authz"
	"beeba.org/internal/repository/postgres"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	defaultJobLimit = 50
	maxJobLimit     = 100
)

type JobStore interface {
	ListAdminJobs(ctx context.Context, filter jobs.AdminListFilter) (jobs.AdminPage, error)
	RetryAdminJob(ctx context.Context, input jobs.AdminRetryInput) (jobs.AdminJob, error)
}

type JobsHandler struct {
	store JobStore
}

func NewJobsHandler(store JobStore) JobsHandler {
	return JobsHandler{store: store}
}

type retryJobRequest struct {
	Reason string `json:"reason"`
}

func (h JobsHandler) List(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "job administration unavailable")
	}
	filter, err := parseJobFilter(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()

	page, err := h.store.ListAdminJobs(ctx, filter)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to load jobs")
	}

	return c.JSON(fiber.Map{
		"data": page.Items,
		"pagination": fiber.Map{
			"next_cursor": encodeJobCursor(page.NextCursor),
			"limit":       filter.Limit,
		},
	})
}

func (h JobsHandler) Retry(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "job administration unavailable")
	}
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}
	jobID := strings.TrimSpace(c.Params("jobID"))
	if _, err := uuid.Parse(jobID); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "job id is invalid")
	}

	var request retryJobRequest
	if err := c.Bind().Body(&request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}
	reason := strings.TrimSpace(request.Reason)
	if reason == "" {
		return fiber.NewError(fiber.StatusBadRequest, "reason is required")
	}
	if len(reason) > 500 {
		return fiber.NewError(fiber.StatusBadRequest, "reason must be 500 characters or fewer")
	}

	ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
	defer cancel()

	job, err := h.store.RetryAdminJob(ctx, jobs.AdminRetryInput{
		ActorUserID: session.User.ID,
		JobID:       jobID,
		Reason:      reason,
		IPAddress:   c.IP(),
		UserAgent:   c.Get("User-Agent"),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "Job not found")
	}
	if errors.Is(err, postgres.ErrAdminJobNotRetryable) {
		return fiber.NewError(fiber.StatusConflict, "Job is not retryable")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to retry job")
	}

	return c.JSON(fiber.Map{"data": job})
}

func parseJobFilter(c fiber.Ctx) (jobs.AdminListFilter, error) {
	limit, err := parseJobLimit(c.Query("limit"))
	if err != nil {
		return jobs.AdminListFilter{}, err
	}
	cursor, err := decodeJobCursor(c.Query("cursor"))
	if err != nil {
		return jobs.AdminListFilter{}, err
	}
	queue := strings.TrimSpace(c.Query("queue"))
	if queue != "" && !allowedJobQueue(queue) {
		return jobs.AdminListFilter{}, errors.New("queue is invalid")
	}
	status := strings.TrimSpace(c.Query("status"))
	if status != "" && !allowedJobStatus(status) {
		return jobs.AdminListFilter{}, errors.New("status is invalid")
	}
	jobType := strings.TrimSpace(c.Query("job_type"))
	if len(jobType) > 120 {
		return jobs.AdminListFilter{}, errors.New("job_type must be 120 characters or fewer")
	}
	return jobs.AdminListFilter{
		QueueName: queue,
		JobType:   jobType,
		Status:    status,
		Limit:     limit,
		Cursor:    cursor,
	}, nil
}

func parseJobLimit(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return defaultJobLimit, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, errors.New("limit must be a number")
	}
	if value <= 0 {
		return 0, errors.New("limit must be greater than 0")
	}
	if value > maxJobLimit {
		return maxJobLimit, nil
	}
	return value, nil
}

func encodeJobCursor(cursor jobs.AdminCursor) string {
	if cursor.ID == "" || cursor.CreatedAt.IsZero() {
		return ""
	}
	payload, err := json.Marshal(cursor)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeJobCursor(raw string) (jobs.AdminCursor, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return jobs.AdminCursor{}, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return jobs.AdminCursor{}, errors.New("cursor is invalid")
	}
	var cursor jobs.AdminCursor
	if err := json.Unmarshal(payload, &cursor); err != nil {
		return jobs.AdminCursor{}, errors.New("cursor is invalid")
	}
	if cursor.ID == "" || cursor.CreatedAt.IsZero() {
		return jobs.AdminCursor{}, errors.New("cursor is invalid")
	}
	if _, err := uuid.Parse(cursor.ID); err != nil {
		return jobs.AdminCursor{}, errors.New("cursor is invalid")
	}
	return cursor, nil
}

func allowedJobQueue(queue string) bool {
	switch queue {
	case "file_scan_queue", "image_processing_queue", "search_index_queue", "email_queue", "moderation_queue", "cleanup_queue":
		return true
	default:
		return false
	}
}

func allowedJobStatus(status string) bool {
	switch status {
	case "pending", "running", "succeeded", "failed", "dead":
		return true
	default:
		return false
	}
}
