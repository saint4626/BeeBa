package logging

import (
	"log/slog"
	"os"

	"beeba.org/internal/config"
)

func New(cfg config.Config) *slog.Logger {
	level := slog.LevelInfo
	if cfg.Environment == "development" {
		level = slog.LevelDebug
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})

	return slog.New(handler).With(
		slog.String("service", cfg.ServiceName),
		slog.String("environment", cfg.Environment),
	)
}
