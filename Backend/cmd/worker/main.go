package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os/signal"
	"syscall"

	"beeba.org/internal/app"
	"beeba.org/internal/config"
	"beeba.org/internal/observability/logging"
)

func main() {
	backfillEmbeddedPreviews := flag.Bool("backfill-embedded-previews", false, "create preview fallbacks from embedded Basis connector images")
	backfillLimit := flag.Int("backfill-limit", 100, "maximum number of content items to inspect during embedded preview backfill")
	flag.Parse()

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

	if *backfillEmbeddedPreviews {
		defer worker.Close()
		report, err := worker.BackfillEmbeddedPreviewFallbacks(ctx, *backfillLimit)
		if err != nil {
			logger.Error("embedded_preview_backfill_failed", "error", err)
			log.Fatal(err)
		}
		logger.Info("embedded_preview_backfill_finished",
			"scanned", report.Scanned,
			"created", report.Created,
			"skipped_existing", report.SkippedExisting,
			"skipped_no_preview", report.SkippedNoPreview,
			"skipped_quota", report.SkippedQuota,
			"failed", report.Failed,
			"limit", report.BackfillLimitUsed,
		)
		return
	}

	if err := worker.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("worker_failed", "error", err)
		log.Fatal(err)
	}
}
