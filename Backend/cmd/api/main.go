package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"beeba.org/internal/app"
	"beeba.org/internal/config"
	"beeba.org/internal/observability/logging"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	logger := logging.New(cfg)
	startupCtx, startupCancel := context.WithTimeout(context.Background(), cfg.ReadinessTimeout)
	api, err := app.NewAPI(startupCtx, cfg, logger)
	startupCancel()
	if err != nil {
		logger.Error("api_init_failed", "error", err)
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- api.Run()
	}()

	select {
	case err := <-errCh:
		if err != nil {
			logger.Error("api_failed", "error", err)
			log.Fatal(err)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := api.Shutdown(shutdownCtx); err != nil {
			logger.Error("api_shutdown_failed", "error", err)
			log.Fatal(err)
		}
	}
}
