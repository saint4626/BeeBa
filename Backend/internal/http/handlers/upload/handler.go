package upload

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"beeba.org/internal/config"
	contentdomain "beeba.org/internal/domain/content"
	uploaddomain "beeba.org/internal/domain/upload"
	"beeba.org/internal/http/middleware/authz"
	"beeba.org/internal/security/secretbox"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Store interface {
	Create(ctx context.Context, input uploaddomain.CreateInput) (uploaddomain.Created, error)
	OwnerStorageUsage(ctx context.Context, ownerID string, limitBytes int64) (contentdomain.OwnerStorageUsage, error)
}

type ObjectStore interface {
	EnsureBucket(ctx context.Context, bucket string) error
	PutObject(ctx context.Context, bucket string, key string, reader io.Reader, size int64, contentType string) error
	RemoveObject(ctx context.Context, bucket string, key string) error
}

type Handler struct {
	cfg     config.Config
	store   Store
	objects ObjectStore
	secrets secretbox.Box
}

func New(cfg config.Config, store Store, objects ObjectStore, secrets secretbox.Box) Handler {
	return Handler{cfg: cfg, store: store, objects: objects, secrets: secrets}
}

func (h Handler) Create(c fiber.Ctx) error {
	if h.store == nil || h.objects == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "upload dependencies unavailable")
	}

	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "file is required")
	}
	if err := validateBeeFile(fileHeader, h.cfg.MaxUploadBytes); err != nil {
		return err
	}

	title := strings.TrimSpace(c.FormValue("title"))
	if title == "" {
		return fiber.NewError(fiber.StatusBadRequest, "title is required")
	}
	category := strings.TrimSpace(c.FormValue("category"))
	if category == "" {
		return fiber.NewError(fiber.StatusBadRequest, "category is required")
	}
	unlockPassword := strings.TrimSpace(c.FormValue("unlock_password"))
	if unlockPassword == "" {
		return fiber.NewError(fiber.StatusBadRequest, "unlock_password is required")
	}
	if len(unlockPassword) > 256 {
		return fiber.NewError(fiber.StatusBadRequest, "unlock_password is too long")
	}
	visibility := strings.ToLower(strings.TrimSpace(c.FormValue("visibility")))
	if visibility == "" {
		visibility = "public"
	}
	if visibility != "public" && visibility != "private" {
		return fiber.NewError(fiber.StatusBadRequest, "visibility must be public or private")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "failed to open uploaded file")
	}
	defer file.Close()

	prefix := make([]byte, 512)
	n, readErr := io.ReadFull(file, prefix)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) && !errors.Is(readErr, io.EOF) {
		return fiber.NewError(fiber.StatusBadRequest, "failed to read uploaded file")
	}
	prefix = prefix[:n]
	detectedMIME := http.DetectContentType(prefix)

	ctx, cancel := context.WithTimeout(c.Context(), h.cfg.UploadStorageTimeout)
	defer cancel()
	if err := h.ensureStorageQuota(ctx, session.User.ID, fileHeader.Size); err != nil {
		return err
	}

	if err := h.objects.EnsureBucket(ctx, h.cfg.QuarantineBucket); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to prepare quarantine storage")
	}

	storageKey := fmt.Sprintf("uploads/%s/%s.bee", session.User.ID, uuid.NewString())
	hasher := sha256.New()
	reader := io.TeeReader(io.MultiReader(bytes.NewReader(prefix), file), hasher)
	if err := h.objects.PutObject(ctx, h.cfg.QuarantineBucket, storageKey, reader, fileHeader.Size, "application/octet-stream"); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to upload file to quarantine storage")
	}
	hash := hex.EncodeToString(hasher.Sum(nil))
	encryptedPassword, err := h.secrets.EncryptString(unlockPassword)
	if err != nil {
		_ = h.objects.RemoveObject(ctx, h.cfg.QuarantineBucket, storageKey)
		return fiber.NewError(fiber.StatusInternalServerError, "failed to secure unlock password")
	}

	created, err := h.store.Create(ctx, uploaddomain.CreateInput{
		AuthorID:                 session.User.ID,
		CategorySlug:             category,
		Title:                    title,
		Slug:                     uniqueSlug(title),
		Description:              strings.TrimSpace(c.FormValue("description")),
		NSFW:                     parseBool(c.FormValue("nsfw")),
		Visibility:               visibility,
		Bucket:                   h.cfg.QuarantineBucket,
		StorageKey:               storageKey,
		OriginalFilename:         fileHeader.Filename,
		SafeFilename:             safeFilename(fileHeader.Filename),
		FileSize:                 fileHeader.Size,
		FileHashSHA256:           hash,
		MimeTypeDetected:         detectedMIME,
		UnlockPasswordCiphertext: encryptedPassword,
		StorageQuotaBytes:        h.cfg.UserStorageQuotaBytes,
	})
	if errors.Is(err, contentdomain.ErrStorageQuotaExceeded) {
		_ = h.objects.RemoveObject(ctx, h.cfg.QuarantineBucket, storageKey)
		return fiber.NewError(fiber.StatusRequestEntityTooLarge, "account storage limit exceeded")
	}
	if errors.Is(err, pgx.ErrNoRows) {
		_ = h.objects.RemoveObject(ctx, h.cfg.QuarantineBucket, storageKey)
		return fiber.NewError(fiber.StatusBadRequest, "category is invalid")
	}
	if errors.Is(err, uploaddomain.ErrDuplicateFile) {
		_ = h.objects.RemoveObject(ctx, h.cfg.QuarantineBucket, storageKey)
		return fiber.NewError(fiber.StatusConflict, "this .bee file has already been uploaded")
	}
	if err != nil {
		_ = h.objects.RemoveObject(ctx, h.cfg.QuarantineBucket, storageKey)
		return fiber.NewError(fiber.StatusInternalServerError, "failed to persist upload metadata")
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"data": created,
	})
}

func (h Handler) ensureStorageQuota(ctx context.Context, ownerID string, incomingBytes int64) error {
	usage, err := h.store.OwnerStorageUsage(ctx, ownerID, h.cfg.UserStorageQuotaBytes)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to check storage quota")
	}
	if usage.LimitBytes > 0 && incomingBytes > usage.LimitBytes-usage.UsedBytes {
		return fiber.NewError(fiber.StatusRequestEntityTooLarge, "account storage limit exceeded")
	}
	return nil
}

func validateBeeFile(fileHeader *multipart.FileHeader, maxSize int64) error {
	if fileHeader.Size <= 0 {
		return fiber.NewError(fiber.StatusBadRequest, "file must not be empty")
	}
	if fileHeader.Size > maxSize {
		return fiber.NewError(fiber.StatusRequestEntityTooLarge, "file exceeds maximum upload size")
	}

	filename := strings.TrimSpace(fileHeader.Filename)
	if filename == "" {
		return fiber.NewError(fiber.StatusBadRequest, "filename is required")
	}
	normalized := strings.ToLower(filepath.ToSlash(filename))
	base := path.Base(normalized)
	if strings.Contains(base, ".bee.") || path.Ext(base) != ".bee" {
		return fiber.NewError(fiber.StatusBadRequest, "only .bee files are accepted")
	}
	return nil
}

var nonFilenameChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
var nonSlugChars = regexp.MustCompile(`[^a-z0-9]+`)

func safeFilename(filename string) string {
	base := path.Base(filepath.ToSlash(filename))
	base = strings.TrimSpace(base)
	base = nonFilenameChars.ReplaceAllString(base, "-")
	base = strings.Trim(base, ".-")
	if base == "" {
		return "upload.bee"
	}
	return base
}

func uniqueSlug(title string) string {
	value := strings.ToLower(strings.TrimSpace(title))
	value = nonSlugChars.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		value = "upload"
	}
	return fmt.Sprintf("%s-%s", value, uuid.NewString()[:8])
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
