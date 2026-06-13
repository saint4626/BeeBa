package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"beeba.org/internal/config"
	contentdomain "beeba.org/internal/domain/content"
	mediadomain "beeba.org/internal/domain/media"
	"beeba.org/internal/http/middleware/authz"
	"beeba.org/internal/security/images"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Store interface {
	CreateUserAvatar(ctx context.Context, input mediadomain.ImageUploadInput) (mediadomain.UploadedImage, error)
	CreateContentImage(ctx context.Context, input mediadomain.ImageUploadInput) (mediadomain.UploadedImage, error)
	OwnerStorageUsage(ctx context.Context, ownerID string, limitBytes int64) (contentdomain.OwnerStorageUsage, error)
	ListOwnedContentImages(ctx context.Context, ownerUserID string, contentID string) ([]mediadomain.UploadedImage, error)
	UpdateOwnedContentImage(ctx context.Context, input mediadomain.ContentImageUpdate) (mediadomain.UploadedImage, error)
	DeleteOwnedContentImage(ctx context.Context, ownerUserID string, contentID string, imageID string) error
	GetPublicObject(ctx context.Context, imageID string) (mediadomain.Object, error)
	GetOwnedObject(ctx context.Context, ownerUserID string, imageID string) (mediadomain.Object, error)
}

type ObjectStore interface {
	EnsureBucket(ctx context.Context, bucket string) error
	PutObject(ctx context.Context, bucket string, key string, reader io.Reader, size int64, contentType string) error
	GetObject(ctx context.Context, bucket string, key string) (io.ReadCloser, error)
	RemoveObject(ctx context.Context, bucket string, key string) error
}

type Handler struct {
	cfg     config.Config
	store   Store
	objects ObjectStore
}

type updateContentImageRequest struct {
	AltText   *string `json:"alt_text"`
	IsPrimary *bool   `json:"is_primary"`
	SortOrder *int    `json:"sort_order"`
}

func New(cfg config.Config, store Store, objects ObjectStore) Handler {
	return Handler{cfg: cfg, store: store, objects: objects}
}

func (h Handler) UploadAvatar(c fiber.Ctx) error {
	if h.store == nil || h.objects == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "media dependencies unavailable")
	}
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "file is required")
	}
	altText, err := parseAltText(c.FormValue("alt_text"))
	if err != nil {
		return err
	}
	metadata, err := readImageHeader(fileHeader, h.cfg.MaxImageBytes)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	if err := h.objects.EnsureBucket(ctx, h.cfg.PreviewBucket); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to prepare media storage")
	}

	imageID := uuid.NewString()
	storageKey := fmt.Sprintf("pending/users/%s/avatar/%s%s", session.User.ID, imageID, extensionForMIME(metadata.MIMEType))
	if err := h.objects.PutObject(ctx, h.cfg.PreviewBucket, storageKey, bytes.NewReader(metadata.Bytes), metadata.DecodedSize, metadata.MIMEType); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to store avatar image")
	}

	uploaded, err := h.store.CreateUserAvatar(ctx, mediadomain.ImageUploadInput{
		OwnerUserID:       session.User.ID,
		Bucket:            h.cfg.PreviewBucket,
		StorageKey:        storageKey,
		OriginalFilename:  fileHeader.Filename,
		AltText:           altText,
		Width:             metadata.Width,
		Height:            metadata.Height,
		FileSize:          metadata.DecodedSize,
		FileHashSHA256:    metadata.SHA256,
		MimeTypeDetected:  metadata.MIMEType,
		StorageQuotaBytes: h.cfg.UserStorageQuotaBytes,
	})
	if errors.Is(err, contentdomain.ErrStorageQuotaExceeded) {
		_ = h.objects.RemoveObject(ctx, h.cfg.PreviewBucket, storageKey)
		return fiber.NewError(fiber.StatusRequestEntityTooLarge, "account storage limit exceeded")
	}
	if err != nil {
		_ = h.objects.RemoveObject(ctx, h.cfg.PreviewBucket, storageKey)
		return fiber.NewError(fiber.StatusInternalServerError, "failed to persist avatar metadata")
	}
	for _, object := range uploaded.ReplacedObjects {
		if object.Bucket == "" || object.StorageKey == "" || object.StorageKey == storageKey {
			continue
		}
		_ = h.objects.RemoveObject(ctx, object.Bucket, object.StorageKey)
	}
	uploaded.URL = mediaURL(c, uploaded.ID)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": uploaded})
}

func (h Handler) UploadContentImage(c fiber.Ctx) error {
	if h.store == nil || h.objects == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "media dependencies unavailable")
	}
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}
	contentID, err := parseUUIDParam(c.Params("contentID"), "content id")
	if err != nil {
		return err
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "file is required")
	}
	altText, err := parseAltText(c.FormValue("alt_text"))
	if err != nil {
		return err
	}
	metadata, err := readImageHeader(fileHeader, h.cfg.MaxImageBytes)
	if err != nil {
		return err
	}
	sortOrder, err := parseSortOrder(c.FormValue("sort_order"))
	if err != nil {
		return err
	}
	isPrimary, err := parsePrimary(c.FormValue("is_primary"))
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	if err := h.ensureStorageQuota(ctx, session.User.ID, metadata.DecodedSize); err != nil {
		return err
	}
	if err := h.objects.EnsureBucket(ctx, h.cfg.PreviewBucket); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to prepare media storage")
	}

	imageID := uuid.NewString()
	storageKey := fmt.Sprintf("pending/content/%s/images/%s%s", contentID, imageID, extensionForMIME(metadata.MIMEType))
	if err := h.objects.PutObject(ctx, h.cfg.PreviewBucket, storageKey, bytes.NewReader(metadata.Bytes), metadata.DecodedSize, metadata.MIMEType); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to store content image")
	}

	uploaded, err := h.store.CreateContentImage(ctx, mediadomain.ImageUploadInput{
		OwnerUserID:       session.User.ID,
		ContentID:         contentID,
		Bucket:            h.cfg.PreviewBucket,
		StorageKey:        storageKey,
		OriginalFilename:  fileHeader.Filename,
		AltText:           altText,
		Width:             metadata.Width,
		Height:            metadata.Height,
		FileSize:          metadata.DecodedSize,
		FileHashSHA256:    metadata.SHA256,
		MimeTypeDetected:  metadata.MIMEType,
		IsPrimary:         isPrimary,
		SortOrder:         sortOrder,
		StorageQuotaBytes: h.cfg.UserStorageQuotaBytes,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		_ = h.objects.RemoveObject(ctx, h.cfg.PreviewBucket, storageKey)
		return fiber.NewError(fiber.StatusNotFound, "content not found")
	}
	if errors.Is(err, contentdomain.ErrStorageQuotaExceeded) {
		_ = h.objects.RemoveObject(ctx, h.cfg.PreviewBucket, storageKey)
		return fiber.NewError(fiber.StatusRequestEntityTooLarge, "account storage limit exceeded")
	}
	if err != nil {
		_ = h.objects.RemoveObject(ctx, h.cfg.PreviewBucket, storageKey)
		return fiber.NewError(fiber.StatusInternalServerError, "failed to persist content image metadata")
	}
	uploaded.URL = mediaURL(c, uploaded.ID)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": uploaded})
}

func (h Handler) ListContentImages(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "media dependencies unavailable")
	}
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}
	contentID, err := parseUUIDParam(c.Params("contentID"), "content id")
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()
	images, err := h.store.ListOwnedContentImages(ctx, session.User.ID, contentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "content not found")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to load content images")
	}
	return c.JSON(fiber.Map{"data": images})
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

func (h Handler) UpdateContentImage(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "media dependencies unavailable")
	}
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}
	contentID, err := parseUUIDParam(c.Params("contentID"), "content id")
	if err != nil {
		return err
	}
	imageID, err := parseUUIDParam(c.Params("imageID"), "image id")
	if err != nil {
		return err
	}
	var req updateContentImageRequest
	if err := c.Bind().Body(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "request body is invalid")
	}
	if req.AltText != nil {
		value, err := parseAltText(*req.AltText)
		if err != nil {
			return err
		}
		req.AltText = &value
	}
	if req.SortOrder != nil && (*req.SortOrder < 0 || *req.SortOrder > 1000) {
		return fiber.NewError(fiber.StatusBadRequest, "sort_order must be between 0 and 1000")
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()
	image, err := h.store.UpdateOwnedContentImage(ctx, mediadomain.ContentImageUpdate{
		OwnerUserID: session.User.ID,
		ContentID:   contentID,
		ImageID:     imageID,
		AltText:     req.AltText,
		IsPrimary:   req.IsPrimary,
		SortOrder:   req.SortOrder,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "image not found")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to update content image")
	}
	return c.JSON(fiber.Map{"data": image})
}

func (h Handler) DeleteContentImage(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "media dependencies unavailable")
	}
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}
	contentID, err := parseUUIDParam(c.Params("contentID"), "content id")
	if err != nil {
		return err
	}
	imageID, err := parseUUIDParam(c.Params("imageID"), "image id")
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()
	if err := h.store.DeleteOwnedContentImage(ctx, session.User.ID, contentID, imageID); errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "image not found")
	} else if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to delete content image")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h Handler) Public(c fiber.Ctx) error {
	if h.store == nil || h.objects == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "media dependencies unavailable")
	}
	imageID := strings.TrimSpace(c.Params("imageID"))
	if _, err := uuid.Parse(imageID); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "image id is invalid")
	}

	ctx, cancel := context.WithTimeout(c.Context(), 10*time.Second)
	defer cancel()
	object, err := h.store.GetPublicObject(ctx, imageID)
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "image not found")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to load image metadata")
	}
	body, err := h.objects.GetObject(ctx, object.Bucket, object.StorageKey)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to open image")
	}
	defer body.Close()
	data, err := io.ReadAll(io.LimitReader(body, h.cfg.MaxImageBytes+1))
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to read image")
	}
	if int64(len(data)) > h.cfg.MaxImageBytes {
		return fiber.NewError(fiber.StatusInternalServerError, "image exceeds configured serving limit")
	}

	c.Set("Content-Type", object.ContentType)
	c.Set("Cache-Control", "public, max-age=3600")
	c.Set("Content-Length", strconv.Itoa(len(data)))
	return c.Send(data)
}

func (h Handler) Owner(c fiber.Ctx) error {
	if h.store == nil || h.objects == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "media dependencies unavailable")
	}
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}
	imageID := strings.TrimSpace(c.Params("imageID"))
	if _, err := uuid.Parse(imageID); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "image id is invalid")
	}

	ctx, cancel := context.WithTimeout(c.Context(), 10*time.Second)
	defer cancel()
	object, err := h.store.GetOwnedObject(ctx, session.User.ID, imageID)
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "image not found")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to load image metadata")
	}
	return h.sendObject(c, ctx, object, "private, max-age=60")
}

func (h Handler) sendObject(c fiber.Ctx, ctx context.Context, object mediadomain.Object, cacheControl string) error {
	body, err := h.objects.GetObject(ctx, object.Bucket, object.StorageKey)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to open image")
	}
	defer body.Close()
	data, err := io.ReadAll(io.LimitReader(body, h.cfg.MaxImageBytes+1))
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to read image")
	}
	if int64(len(data)) > h.cfg.MaxImageBytes {
		return fiber.NewError(fiber.StatusInternalServerError, "image exceeds configured serving limit")
	}

	c.Set("Content-Type", object.ContentType)
	c.Set("Cache-Control", cacheControl)
	c.Set("Content-Length", strconv.Itoa(len(data)))
	return c.Send(data)
}

func readImageHeader(fileHeader *multipart.FileHeader, maxBytes int64) (images.Metadata, error) {
	if fileHeader.Size <= 0 {
		return images.Metadata{}, fiber.NewError(fiber.StatusBadRequest, "image must not be empty")
	}
	if fileHeader.Size > maxBytes {
		return images.Metadata{}, fiber.NewError(fiber.StatusRequestEntityTooLarge, "image exceeds maximum upload size")
	}
	filename := strings.TrimSpace(fileHeader.Filename)
	if filename == "" {
		return images.Metadata{}, fiber.NewError(fiber.StatusBadRequest, "filename is required")
	}
	ext := strings.ToLower(path.Ext(path.Base(filepath.ToSlash(filename))))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		return images.Metadata{}, fiber.NewError(fiber.StatusBadRequest, "only JPEG and PNG images are accepted")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return images.Metadata{}, fiber.NewError(fiber.StatusBadRequest, "failed to open uploaded image")
	}
	defer file.Close()
	metadata, err := images.ReadAndValidate(file, maxBytes)
	if err != nil {
		if strings.Contains(err.Error(), "exceeds maximum") {
			return images.Metadata{}, fiber.NewError(fiber.StatusRequestEntityTooLarge, err.Error())
		}
		return images.Metadata{}, fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	if ext == ".jpg" || ext == ".jpeg" {
		if metadata.MIMEType != images.MIMEJPEG {
			return images.Metadata{}, fiber.NewError(fiber.StatusBadRequest, "image extension does not match decoded format")
		}
	}
	if ext == ".png" && metadata.MIMEType != images.MIMEPNG {
		return images.Metadata{}, fiber.NewError(fiber.StatusBadRequest, "image extension does not match decoded format")
	}
	return metadata, nil
}

func parseUUIDParam(raw string, name string) (string, error) {
	value := strings.TrimSpace(raw)
	if _, err := uuid.Parse(value); err != nil {
		return "", fiber.NewError(fiber.StatusBadRequest, name+" is invalid")
	}
	return value, nil
}

func parseSortOrder(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fiber.NewError(fiber.StatusBadRequest, "sort_order must be an integer")
	}
	if value < 0 || value > 1000 {
		return 0, fiber.NewError(fiber.StatusBadRequest, "sort_order must be between 0 and 1000")
	}
	return value, nil
}

func parseAltText(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if len(value) > 500 {
		return "", fiber.NewError(fiber.StatusBadRequest, "alt_text must be 500 characters or fewer")
	}
	return value, nil
}

func parsePrimary(raw string) (bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fiber.NewError(fiber.StatusBadRequest, "is_primary must be a boolean")
	}
	return value, nil
}

var nonStorageNameChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func extensionForMIME(mimeType string) string {
	switch mimeType {
	case images.MIMEPNG:
		return ".png"
	default:
		return ".jpg"
	}
}

func mediaURL(c fiber.Ctx, imageID string) string {
	return fmt.Sprintf("/api/v1/media/%s", imageID)
}

func safeImageFilename(filename string) string {
	base := path.Base(filepath.ToSlash(filename))
	base = strings.TrimSpace(base)
	base = nonStorageNameChars.ReplaceAllString(base, "-")
	base = strings.Trim(base, ".-")
	if base == "" {
		return "image"
	}
	return base
}
