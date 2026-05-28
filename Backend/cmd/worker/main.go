package main

import (
	"context"
	"errors"
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
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	worker, err := app.NewWorker(ctx, cfg, logger)
	if err != nil {
		logger.Error("create_worker_failed", "error", err)
		log.Fatal(err)
	}

	if err := worker.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("worker_failed", "error", err)
		log.Fatal(err)
	}
}
