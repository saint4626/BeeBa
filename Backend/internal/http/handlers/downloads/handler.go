package downloads

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"beeba.org/internal/config"
	downloaddomain "beeba.org/internal/domain/downloads"
	"beeba.org/internal/http/middleware/authz"
	"beeba.org/internal/security/tokens"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Store interface {
	GetPublicTarget(ctx context.Context, contentID string, quarantineBucket string) (downloaddomain.Target, error)
	GetUnlistedTarget(ctx context.Context, contentID string, tokenHash string, quarantineBucket string) (downloaddomain.Target, error)
	GetOwnerTarget(ctx context.Context, contentID string, userID string, quarantineBucket string) (downloaddomain.Target, error)
	RecordEvent(ctx context.Context, input downloaddomain.EventInput) error
}

type ObjectStore interface {
	GetObject(ctx context.Context, bucket string, key string) (io.ReadCloser, error)
	GetObjectRange(ctx context.Context, bucket string, key string, start int64, end int64) (io.ReadCloser, error)
}

type Handler struct {
	cfg     config.Config
	store   Store
	objects ObjectStore
}

func New(cfg config.Config, store Store, objects ObjectStore) Handler {
	return Handler{cfg: cfg, store: store, objects: objects}
}

func (h Handler) Public(c fiber.Ctx) error {
	if h.store == nil || h.objects == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "download dependencies unavailable")
	}
	contentID := strings.TrimSpace(c.Params("contentID"))
	if _, err := uuid.Parse(contentID); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "content id is invalid")
	}

	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()

	target, err := h.publicTarget(ctx, contentID, strings.TrimSpace(c.Query("access")))
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "download not found")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to load download")
	}
	return h.stream(c, target, nil)
}

func (h Handler) publicTarget(ctx context.Context, contentID string, accessToken string) (downloaddomain.Target, error) {
	if accessToken != "" {
		target, err := h.store.GetUnlistedTarget(ctx, contentID, tokens.Hash(accessToken), h.cfg.QuarantineBucket)
		if err == nil {
			return target, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return downloaddomain.Target{}, err
		}
	}
	return h.store.GetPublicTarget(ctx, contentID, h.cfg.QuarantineBucket)
}

func (h Handler) Owner(c fiber.Ctx) error {
	if h.store == nil || h.objects == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "download dependencies unavailable")
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

	target, err := h.store.GetOwnerTarget(ctx, contentID, session.User.ID, h.cfg.QuarantineBucket)
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "download not found")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to load owner download")
	}
	return h.stream(c, target, &session.User.ID)
}

func (h Handler) stream(c fiber.Ctx, target downloaddomain.Target, userID *string) error {
	if target.FileSize <= 0 {
		return fiber.NewError(fiber.StatusInternalServerError, "download has invalid file size")
	}

	c.Set(fiber.HeaderContentType, "application/octet-stream")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%q", target.Filename))
	c.Set("Accept-Ranges", "bytes")
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("X-BeeBa-Content-ID", target.ContentID)
	c.Set("X-BeeBa-File-ID", target.FileID)

	byteRange, err := parseRangeHeader(c.Get(fiber.HeaderRange), target.FileSize)
	if err != nil {
		c.Set(fiber.HeaderContentRange, fmt.Sprintf("bytes */%d", target.FileSize))
		return fiber.NewError(fiber.StatusRequestedRangeNotSatisfiable, "requested range is not satisfiable")
	}

	if c.Method() == fiber.MethodHead {
		if byteRange.partial {
			c.Status(fiber.StatusPartialContent)
			c.Set(fiber.HeaderContentRange, byteRange.contentRange(target.FileSize))
			c.Set(fiber.HeaderContentLength, strconv.FormatInt(byteRange.length(), 10))
			return nil
		}
		c.Set(fiber.HeaderContentLength, strconv.FormatInt(target.FileSize, 10))
		return nil
	}

	var object io.ReadCloser
	if byteRange.partial {
		object, err = h.objects.GetObjectRange(c.Context(), target.Bucket, target.StorageKey, byteRange.start, byteRange.end)
	} else {
		object, err = h.objects.GetObject(c.Context(), target.Bucket, target.StorageKey)
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to open download")
	}

	if shouldRecordDownload(byteRange) {
		if err := h.store.RecordEvent(c.Context(), downloaddomain.EventInput{
			ContentID: target.ContentID,
			FileID:    target.FileID,
			UserID:    userID,
			IPAddress: c.IP(),
			UserAgent: c.Get("User-Agent"),
		}); err != nil {
			_ = object.Close()
			return fiber.NewError(fiber.StatusInternalServerError, "failed to record download")
		}
	}

	if byteRange.partial {
		c.Status(fiber.StatusPartialContent)
		c.Set(fiber.HeaderContentRange, byteRange.contentRange(target.FileSize))
		return c.SendStream(object, int(byteRange.length()))
	}
	return c.SendStream(object, int(target.FileSize))
}

type downloadRange struct {
	start   int64
	end     int64
	partial bool
}

func (r downloadRange) length() int64 {
	return r.end - r.start + 1
}

func (r downloadRange) contentRange(size int64) string {
	return fmt.Sprintf("bytes %d-%d/%d", r.start, r.end, size)
}

func parseRangeHeader(header string, size int64) (downloadRange, error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return downloadRange{}, nil
	}
	if !strings.HasPrefix(strings.ToLower(header), "bytes=") {
		return downloadRange{}, errors.New("unsupported range unit")
	}

	spec := strings.TrimSpace(header[len("bytes="):])
	if spec == "" || strings.Contains(spec, ",") {
		return downloadRange{}, errors.New("invalid range")
	}

	parts := strings.SplitN(spec, "-", 2)
	if len(parts) != 2 {
		return downloadRange{}, errors.New("invalid range")
	}

	startText := strings.TrimSpace(parts[0])
	endText := strings.TrimSpace(parts[1])
	if startText == "" && endText == "" {
		return downloadRange{}, errors.New("invalid range")
	}

	var start int64
	var end int64
	if startText == "" {
		suffixLength, err := strconv.ParseInt(endText, 10, 64)
		if err != nil || suffixLength <= 0 {
			return downloadRange{}, errors.New("invalid suffix range")
		}
		if suffixLength > size {
			suffixLength = size
		}
		start = size - suffixLength
		end = size - 1
	} else {
		parsedStart, err := strconv.ParseInt(startText, 10, 64)
		if err != nil || parsedStart < 0 {
			return downloadRange{}, errors.New("invalid range start")
		}
		start = parsedStart
		if endText == "" {
			end = size - 1
		} else {
			parsedEnd, err := strconv.ParseInt(endText, 10, 64)
			if err != nil || parsedEnd < 0 {
				return downloadRange{}, errors.New("invalid range end")
			}
			end = parsedEnd
		}
	}

	if start >= size || start > end {
		return downloadRange{}, errors.New("range outside file")
	}
	if end >= size {
		end = size - 1
	}
	return downloadRange{start: start, end: end, partial: true}, nil
}

func shouldRecordDownload(byteRange downloadRange) bool {
	return !byteRange.partial || byteRange.start == 0
}
