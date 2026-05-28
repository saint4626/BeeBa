package health

import (
	"context"
	"time"

	"beeba.org/internal/config"

	"github.com/gofiber/fiber/v3"
)

type Handler struct {
	cfg      config.Config
	started  time.Time
	postgres Pinger
	redis    Pinger
}

type Pinger interface {
	Ping(ctx context.Context) error
}

func New(cfg config.Config, postgres Pinger, redis Pinger) Handler {
	return Handler{
		cfg:      cfg,
		started:  time.Now().UTC(),
		postgres: postgres,
		redis:    redis,
	}
}

func (h Handler) Healthz(c fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":      "ok",
		"service":     h.cfg.ServiceName,
		"environment": h.cfg.Environment,
		"started_at":  h.started.Format(time.RFC3339),
	})
}

func (h Handler) Readyz(c fiber.Ctx) error {
	postgresStatus := checkStatus(h.postgres != nil)
	redisStatus := checkStatus(h.redis != nil)

	if h.postgres != nil {
		ctx, cancel := context.WithTimeout(c.Context(), h.cfg.ReadinessTimeout)
		defer cancel()
		if err := h.postgres.Ping(ctx); err != nil {
			postgresStatus = "failed"
		}
	}
	if h.redis != nil {
		ctx, cancel := context.WithTimeout(c.Context(), h.cfg.ReadinessTimeout)
		defer cancel()
		if err := h.redis.Ping(ctx); err != nil {
			redisStatus = "failed"
		}
	}

	statusCode := fiber.StatusOK
	status := "ready"
	if postgresStatus == "failed" || redisStatus == "failed" {
		statusCode = fiber.StatusServiceUnavailable
		status = "not_ready"
	}

	return c.Status(statusCode).JSON(fiber.Map{
		"status":  status,
		"service": h.cfg.ServiceName,
		"checks": fiber.Map{
			"postgres": postgresStatus,
			"redis":    redisStatus,
		},
	})
}

func (h Handler) APIStatus(c fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"service":      h.cfg.ServiceName,
		"api_version":  "v1",
		"environment":  h.cfg.Environment,
		"max_upload_b": h.cfg.MaxUploadBytes,
	})
}

func checkStatus(enabled bool) string {
	if !enabled {
		return "not_configured"
	}
	return "ok"
}
