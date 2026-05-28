package admin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"beeba.org/internal/domain/files"
	"beeba.org/internal/http/middleware/authz"
	"beeba.org/internal/repository/postgres"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	defaultFileLimit = 50
	maxFileLimit     = 100
)

var sha256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type FileStore interface {
	ListAdminFiles(ctx context.Context, filter files.AdminListFilter) (files.AdminPage, error)
	RescanAdminFile(ctx context.Context, input files.AdminRescanInput) (files.AdminRescanResult, error)
}

type FilesHandler struct {
	store FileStore
}

func NewFilesHandler(store FileStore) FilesHandler {
	return FilesHandler{store: store}
}

type rescanFileRequest struct {
	Reason string `json:"reason"`
}

func (h FilesHandler) List(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "file administration unavailable")
	}
	filter, err := parseFileFilter(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()

	page, err := h.store.ListAdminFiles(ctx, filter)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to load files")
	}

	return c.JSON(fiber.Map{
		"data": page.Items,
		"pagination": fiber.Map{
			"next_cursor": encodeFileCursor(page.NextCursor),
			"limit":       filter.Limit,
		},
	})
}

func (h FilesHandler) Rescan(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "file administration unavailable")
	}
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}
	fileID := strings.TrimSpace(c.Params("fileID"))
	if _, err := uuid.Parse(fileID); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "file id is invalid")
	}

	var request rescanFileRequest
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

	result, err := h.store.RescanAdminFile(ctx, files.AdminRescanInput{
		ActorUserID: session.User.ID,
		FileID:      fileID,
		Reason:      reason,
		IPAddress:   c.IP(),
		UserAgent:   c.Get("User-Agent"),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "File not found")
	}
	if errors.Is(err, postgres.ErrAdminFileNotRescannable) {
		return fiber.NewError(fiber.StatusConflict, "File is not rescannable")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to rescan file")
	}

	return c.JSON(fiber.Map{"data": result})
}

func parseFileFilter(c fiber.Ctx) (files.AdminListFilter, error) {
	limit, err := parseFileLimit(c.Query("limit"))
	if err != nil {
		return files.AdminListFilter{}, err
	}
	cursor, err := decodeFileCursor(c.Query("cursor"))
	if err != nil {
		return files.AdminListFilter{}, err
	}

	query := strings.TrimSpace(c.Query("q"))
	if len(query) > 200 {
		return files.AdminListFilter{}, errors.New("q must be 200 characters or fewer")
	}
	scanStatus := strings.TrimSpace(c.Query("scan_status"))
	if scanStatus != "" && !allowedFileScanStatus(scanStatus) {
		return files.AdminListFilter{}, errors.New("scan_status is invalid")
	}
	bucket := strings.TrimSpace(c.Query("bucket"))
	if len(bucket) > 120 {
		return files.AdminListFilter{}, errors.New("bucket must be 120 characters or fewer")
	}
	contentID := strings.TrimSpace(c.Query("content_id"))
	if contentID != "" {
		if _, err := uuid.Parse(contentID); err != nil {
			return files.AdminListFilter{}, errors.New("content_id is invalid")
		}
	}
	hash := strings.ToLower(strings.TrimSpace(c.Query("hash")))
	if hash != "" && !sha256Pattern.MatchString(hash) {
		return files.AdminListFilter{}, errors.New("hash must be a lowercase sha-256 hex digest")
	}

	return files.AdminListFilter{
		Query:      query,
		ScanStatus: scanStatus,
		Bucket:     bucket,
		ContentID:  contentID,
		Hash:       hash,
		Limit:      limit,
		Cursor:     cursor,
	}, nil
}

func parseFileLimit(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return defaultFileLimit, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, errors.New("limit must be a number")
	}
	if value <= 0 {
		return 0, errors.New("limit must be greater than 0")
	}
	if value > maxFileLimit {
		return maxFileLimit, nil
	}
	return value, nil
}

func encodeFileCursor(cursor files.AdminCursor) string {
	if cursor.ID == "" || cursor.CreatedAt.IsZero() {
		return ""
	}
	payload, err := json.Marshal(cursor)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeFileCursor(raw string) (files.AdminCursor, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return files.AdminCursor{}, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return files.AdminCursor{}, errors.New("cursor is invalid")
	}
	var cursor files.AdminCursor
	if err := json.Unmarshal(payload, &cursor); err != nil {
		return files.AdminCursor{}, errors.New("cursor is invalid")
	}
	if cursor.ID == "" || cursor.CreatedAt.IsZero() {
		return files.AdminCursor{}, errors.New("cursor is invalid")
	}
	if _, err := uuid.Parse(cursor.ID); err != nil {
		return files.AdminCursor{}, errors.New("cursor is invalid")
	}
	return cursor, nil
}

func allowedFileScanStatus(status string) bool {
	switch status {
	case "pending", "running", "clean", "suspicious", "infected", "failed", "skipped":
		return true
	default:
		return false
	}
}
