package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"beeba.org/internal/config"
	contentdomain "beeba.org/internal/domain/content"
	"beeba.org/internal/domain/jobs"
	serversdomain "beeba.org/internal/domain/servers"
	mediadomain "beeba.org/internal/domain/media"
	emailclient "beeba.org/internal/email"
	"beeba.org/internal/repository/postgres"
	searchclient "beeba.org/internal/search/meilisearch"
	"beeba.org/internal/servercheck"
	"beeba.org/internal/security/antivirus"
	"beeba.org/internal/security/basisbee"
	"beeba.org/internal/security/images"
	"beeba.org/internal/security/secretbox"
	miniostorage "beeba.org/internal/storage/minio"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Worker struct {
	cfg     config.Config
	log     *slog.Logger
	db      *pgxpool.Pool
	jobs    postgres.JobRepository
	media   postgres.MediaRepository
	servers postgres.ServerRepository
	objects objectReader
	secrets secretbox.Box
	search  *searchclient.Client
	email   emailclient.Sender
	av      antivirusScanner
	id      string
}

type antivirusScanner interface {
	Scan(ctx context.Context, reader io.Reader) (antivirus.Result, error)
}

type objectReader interface {
	EnsureBucket(ctx context.Context, bucket string) error
	CopyObject(ctx context.Context, sourceBucket string, sourceKey string, destinationBucket string, destinationKey string) error
	GetObject(ctx context.Context, bucket string, key string) (io.ReadCloser, error)
	PutObject(ctx context.Context, bucket string, key string, reader io.Reader, size int64, contentType string) error
	RemoveObject(ctx context.Context, bucket string, key string) error
}

type EmbeddedPreviewBackfillReport struct {
	Scanned           int
	Created           int
	SkippedExisting   int
	SkippedNoPreview  int
	SkippedQuota      int
	Failed            int
	BackfillLimitUsed int
}

type embeddedPreviewFallbackOutcome string

const (
	embeddedPreviewFallbackCreated          embeddedPreviewFallbackOutcome = "created"
	embeddedPreviewFallbackSkippedExisting  embeddedPreviewFallbackOutcome = "skipped_existing"
	embeddedPreviewFallbackSkippedNoPreview embeddedPreviewFallbackOutcome = "skipped_no_preview"
	embeddedPreviewFallbackSkippedQuota     embeddedPreviewFallbackOutcome = "skipped_quota"
	embeddedPreviewFallbackFailed           embeddedPreviewFallbackOutcome = "failed"
)

func NewWorker(ctx context.Context, cfg config.Config, log *slog.Logger) (*Worker, error) {
	db, err := postgres.Open(ctx, cfg)
	if err != nil {
		return nil, err
	}

	secrets, err := secretbox.New(cfg.SecretBoxKey)
	if err != nil {
		db.Close()
		return nil, err
	}

	objects, err := miniostorage.New(cfg)
	if err != nil {
		db.Close()
		return nil, err
	}
	var search *searchclient.Client
	if cfg.MeilisearchURL != "" {
		search, err = searchclient.New(cfg.MeilisearchURL, cfg.MeilisearchAPIKey, cfg.MeilisearchContentIndex)
		if err != nil {
			db.Close()
			return nil, err
		}
	}
	var av antivirusScanner
	if cfg.ClamAVAddr != "" {
		av, err = antivirus.NewClamdScanner(cfg.ClamAVAddr, cfg.ClamAVTimeout)
		if err != nil {
			db.Close()
			return nil, err
		}
	}
	var emailSender emailclient.Sender
	if cfg.EmailDeliveryEnabled {
		emailSender, err = emailclient.NewResendSender(cfg)
		if err != nil {
			db.Close()
			return nil, err
		}
	}

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "worker"
	}

	return &Worker{
		cfg:     cfg,
		log:     log,
		db:      db,
		jobs:    postgres.NewJobRepository(db),
		media:   postgres.NewMediaRepository(db),
		servers: postgres.NewServerRepository(db, secrets),
		objects: objects,
		secrets: secrets,
		search:  search,
		email:   emailSender,
		av:      av,
		id:      fmt.Sprintf("%s-%d", hostname, time.Now().UnixNano()),
	}, nil
}

func (w *Worker) Close() {
	if w.db != nil {
		w.db.Close()
	}
}

func (w *Worker) Run(ctx context.Context) error {
	w.log.Info("worker_starting", slog.String("worker_id", w.id))
	defer w.Close()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		if err := w.processOnce(ctx); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			w.log.Error("worker_process_failed", slog.String("error", err.Error()))
		}

		select {
		case <-ctx.Done():
			w.log.Info("worker_stopping")
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (w *Worker) BackfillEmbeddedPreviewFallbacks(ctx context.Context, limit int) (EmbeddedPreviewBackfillReport, error) {
	limit = normalizeEmbeddedPreviewBackfillLimit(limit)
	report := EmbeddedPreviewBackfillReport{BackfillLimitUsed: limit}

	candidates, err := w.jobs.ListEmbeddedPreviewBackfillCandidates(ctx, limit)
	if err != nil {
		return report, err
	}
	report.Scanned = len(candidates)

	for _, file := range candidates {
		switch w.backfillEmbeddedPreviewFallback(ctx, file) {
		case embeddedPreviewFallbackCreated:
			report.Created++
		case embeddedPreviewFallbackSkippedExisting:
			report.SkippedExisting++
		case embeddedPreviewFallbackSkippedNoPreview:
			report.SkippedNoPreview++
		case embeddedPreviewFallbackSkippedQuota:
			report.SkippedQuota++
		default:
			report.Failed++
		}
	}
	return report, nil
}

func normalizeEmbeddedPreviewBackfillLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}

func (w *Worker) processOnce(ctx context.Context) error {
	job, err := w.jobs.Claim(ctx, "file_scan_queue", w.id)
	if errors.Is(err, pgx.ErrNoRows) {
		return w.processAutoPublishQueue(ctx)
	}
	if err != nil {
		return fmt.Errorf("claim file scan job: %w", err)
	}

	w.log.Info("worker_job_claimed", slog.String("job_id", job.ID), slog.String("job_type", job.JobType))
	if job.JobType != "scan_content_file" {
		return w.jobs.MarkFailed(ctx, job, "unsupported job type")
	}

	if err := w.processFileScan(ctx, job); err != nil {
		if markErr := w.jobs.MarkFailed(ctx, job, err.Error()); markErr != nil {
			return fmt.Errorf("process job: %w; mark failed: %w", err, markErr)
		}
		return err
	}
	return nil
}

func (w *Worker) processAutoPublishQueue(ctx context.Context) error {
	candidate, err := w.jobs.GetAutoPublishCandidate(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return w.processImageQueue(ctx)
	}
	if err != nil {
		return fmt.Errorf("load auto-publish candidate: %w", err)
	}

	if err := w.publishCleanCandidate(ctx, candidate); err != nil {
		return err
	}
	w.log.Info("worker_clean_content_auto_published",
		slog.String("content_id", candidate.ContentID),
		slog.String("file_id", candidate.FileID),
	)
	return nil
}

func (w *Worker) processImageQueue(ctx context.Context) error {
	job, err := w.jobs.Claim(ctx, "image_processing_queue", w.id)
	if errors.Is(err, pgx.ErrNoRows) {
		return w.processSearchQueue(ctx)
	}
	if err != nil {
		return fmt.Errorf("claim image processing job: %w", err)
	}

	w.log.Info("worker_job_claimed", slog.String("job_id", job.ID), slog.String("job_type", job.JobType))
	if job.JobType != "process_image" {
		return w.jobs.MarkFailed(ctx, job, "unsupported job type")
	}

	if err := w.processImage(ctx, job); err != nil {
		var payload jobs.ImageProcessPayload
		if decodeErr := json.Unmarshal(job.Payload, &payload); decodeErr == nil && payload.Kind != "" && payload.ImageID != "" {
			_ = w.media.FailImageProcessing(ctx, payload.Kind, payload.ImageID)
		}
		if markErr := w.jobs.MarkFailed(ctx, job, err.Error()); markErr != nil {
			return fmt.Errorf("process image: %w; mark failed: %w", err, markErr)
		}
		return err
	}
	return nil
}

func (w *Worker) processSearchQueue(ctx context.Context) error {
	job, err := w.jobs.Claim(ctx, "search_index_queue", w.id)
	if errors.Is(err, pgx.ErrNoRows) {
		return w.processEmailQueue(ctx)
	}
	if err != nil {
		return fmt.Errorf("claim search index job: %w", err)
	}

	w.log.Info("worker_job_claimed", slog.String("job_id", job.ID), slog.String("job_type", job.JobType))
	if job.JobType != "sync_content_search" {
		return w.jobs.MarkFailed(ctx, job, "unsupported job type")
	}
	if err := w.processSearchIndex(ctx, job); err != nil {
		if markErr := w.jobs.MarkFailed(ctx, job, err.Error()); markErr != nil {
			return fmt.Errorf("process search index: %w; mark failed: %w", err, markErr)
		}
		return err
	}
	return nil
}

func (w *Worker) processEmailQueue(ctx context.Context) error {
	job, err := w.jobs.Claim(ctx, "email_queue", w.id)
	if errors.Is(err, pgx.ErrNoRows) {
		return w.processServerCheckQueue(ctx)
	}
	if err != nil {
		return fmt.Errorf("claim email job: %w", err)
	}

	w.log.Info("worker_job_claimed", slog.String("job_id", job.ID), slog.String("job_type", job.JobType))
	var processErr error
	switch job.JobType {
	case "send_email_verification":
		processErr = w.processEmailVerification(ctx, job)
	case "send_password_change_confirmation":
		processErr = w.processPasswordChangeConfirmation(ctx, job)
	case "send_email_change_confirmation":
		processErr = w.processEmailChangeConfirmation(ctx, job)
	default:
		return w.jobs.MarkFailed(ctx, job, "unsupported job type")
	}
	if processErr == nil {
		return nil
	}
	if markErr := w.jobs.MarkFailed(ctx, job, processErr.Error()); markErr != nil {
		return fmt.Errorf("process email job: %w; mark failed: %w", processErr, markErr)
	}
	return processErr
}

func (w *Worker) processServerCheckQueue(ctx context.Context) error {
	job, err := w.jobs.Claim(ctx, "server_check_queue", w.id)
	if errors.Is(err, pgx.ErrNoRows) {
		w.log.Debug("worker_idle", slog.String("queue", "file_scan_queue,image_processing_queue,search_index_queue,email_queue,server_check_queue"))
		return err
	}
	if err != nil {
		return fmt.Errorf("claim server check job: %w", err)
	}

	w.log.Info("worker_job_claimed", slog.String("job_id", job.ID), slog.String("job_type", job.JobType))
	if job.JobType != "check_basis_server" {
		return w.jobs.MarkFailed(ctx, job, "unsupported job type")
	}
	if err := w.processServerCheck(ctx, job); err != nil {
		if markErr := w.jobs.MarkFailed(ctx, job, err.Error()); markErr != nil {
			return fmt.Errorf("process server check: %w; mark failed: %w", err, markErr)
		}
		return err
	}
	return nil
}

func (w *Worker) processServerCheck(ctx context.Context, job jobs.Job) error {
	var payload jobs.ServerCheckPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("decode server check payload: %w", err)
	}
	if payload.ServerID == "" {
		return fmt.Errorf("server check payload is incomplete")
	}
	target, err := w.servers.GetCheckTarget(ctx, payload.ServerID)
	if errors.Is(err, pgx.ErrNoRows) {
		if markErr := w.jobs.MarkSucceeded(ctx, job.ID); markErr != nil {
			return fmt.Errorf("mark stale server check job succeeded: %w", markErr)
		}
		w.log.Info("worker_server_check_skipped",
			slog.String("job_id", job.ID),
			slog.String("server_id", payload.ServerID),
			slog.String("reason", "server_not_available_for_check"),
		)
		return nil
	}
	if err != nil {
		return fmt.Errorf("load server check target: %w", err)
	}

	checkedAt := time.Now().UTC()
	info, probeErr := servercheck.Probe(ctx, target.Host, target.Port, 3*time.Second)
	result := serversdomain.CheckResult{
		Status:    serversdomain.CheckStatusOnline,
		CheckedAt: checkedAt,
	}
	if probeErr != nil {
		message := probeErr.Error()
		result.Status = serversdomain.CheckStatusOffline
		result.LastError = &message
	} else {
		result.OnlinePlayers = intPtr(info.OnlinePlayers)
		result.MaxPlayers = intPtr(info.MaxPlayers)
		result.ProtocolVersion = intPtr(info.ProtocolVersion)
		result.ServerName = stringPtr(strings.TrimSpace(info.ServerName))
		result.Motd = stringPtr(strings.TrimSpace(info.Motd))
		result.RoundTripMS = intPtr(info.RoundTripMS)
	}

	if err := w.servers.CompleteCheck(ctx, payload.ServerID, result); err != nil {
		return fmt.Errorf("complete server check: %w", err)
	}
	if err := w.jobs.MarkSucceeded(ctx, job.ID); err != nil {
		return fmt.Errorf("mark server check job succeeded: %w", err)
	}
	w.log.Info("worker_server_check_completed",
		slog.String("server_id", payload.ServerID),
		slog.String("status", result.Status),
	)
	return nil
}

func intPtr(value int) *int {
	return &value
}

func stringPtr(value string) *string {
	return &value
}

func (w *Worker) processFileScan(ctx context.Context, job jobs.Job) error {
	var payload jobs.FileScanPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("decode file scan payload: %w", err)
	}
	if payload.FileID == "" || payload.ContentID == "" || payload.Bucket == "" || payload.StorageKey == "" {
		return fmt.Errorf("file scan payload is incomplete")
	}

	file, err := w.jobs.GetContentFileForScan(ctx, payload.FileID)
	if err != nil {
		return fmt.Errorf("load content file for scan: %w", err)
	}
	if file.ContentID != payload.ContentID || file.Bucket != payload.Bucket || file.StorageKey != payload.StorageKey {
		return w.jobs.CompleteFileScan(ctx, job.ID, payload.FileID, payload.ContentID, "suspicious", "scan_failed", map[string]any{
			"scanner": "metadata_integrity",
			"result":  "suspicious",
			"reason":  "payload_metadata_mismatch",
		})
	}

	object, err := w.objects.GetObject(ctx, file.Bucket, file.StorageKey)
	if err != nil {
		return fmt.Errorf("open quarantine object: %w", err)
	}

	hasher := sha256.New()
	size, err := io.Copy(hasher, object)
	if err != nil {
		object.Close()
		return fmt.Errorf("read quarantine object: %w", err)
	}
	object.Close()
	hash := hex.EncodeToString(hasher.Sum(nil))
	if size != file.FileSize || hash != file.FileHashSHA256 {
		return w.jobs.CompleteFileScan(ctx, job.ID, file.FileID, file.ContentID, "suspicious", "scan_failed", map[string]any{
			"scanner":       "metadata_integrity",
			"result":        "suspicious",
			"reason":        "object_hash_or_size_mismatch",
			"expected_size": file.FileSize,
			"actual_size":   size,
			"expected_hash": file.FileHashSHA256,
			"actual_hash":   hash,
		})
	}

	avResult, err := w.scanQuarantineObject(ctx, file)
	if errors.Is(err, antivirus.ErrInfected) {
		return w.jobs.CompleteFileScan(ctx, job.ID, file.FileID, file.ContentID, "infected", "scan_failed", map[string]any{
			"scanner":   "clamav",
			"result":    "infected",
			"signature": avResult.Signature,
			"reason":    "antivirus_detected_infected_content",
			"size":      size,
			"sha256":    hash,
		})
	}
	if err != nil {
		return fmt.Errorf("antivirus scan failed: %w", err)
	}

	object, err = w.objects.GetObject(ctx, file.Bucket, file.StorageKey)
	if err != nil {
		return fmt.Errorf("reopen quarantine object for basis validation: %w", err)
	}
	defer object.Close()

	unlockPassword, err := w.secrets.DecryptString(file.UnlockPasswordCiphertext)
	if err != nil {
		return fmt.Errorf("decrypt unlock password: %w", err)
	}

	beeResult, connector, err := basisbee.ValidateRemoteSDKBEEWithPassword(object, unlockPassword, w.cfg.MaxUploadBytes)
	if err != nil {
		return w.jobs.CompleteFileScan(ctx, job.ID, file.FileID, file.ContentID, "suspicious", "scan_failed", map[string]any{
			"scanner": "basis_bee_format",
			"result":  "suspicious",
			"reason":  err.Error(),
		})
	}

	if avResult.Result == "skipped" {
		if err := w.jobs.CompleteFileScan(ctx, job.ID, file.FileID, file.ContentID, "failed", "scan_failed", map[string]any{
			"scanner":           "clamav+basis_bee_format",
			"result":            "failed",
			"antivirus":         avResult,
			"basis_result":      "valid",
			"reason":            "antivirus_scan_skipped",
			"size":              size,
			"sha256":            hash,
			"connector_bytes":   beeResult.ConnectorBytes,
			"section_bytes":     beeResult.SectionBytes,
			"remote_sdk_format": beeResult.RemoteSDKFormat,
			"unique_version":    beeResult.UniqueVersion,
			"asset_name":        beeResult.AssetName,
			"asset_mode":        beeResult.AssetMode,
			"platform_count":    beeResult.PlatformCount,
		}); err != nil {
			return fmt.Errorf("complete skipped file scan: %w", err)
		}
		w.log.Info("worker_file_scan_completed",
			slog.String("job_id", job.ID),
			slog.String("file_id", file.FileID),
			slog.String("scan_status", "failed"),
		)
		return nil
	}

	scanResult := map[string]any{
		"scanner":           "clamav+basis_bee_format",
		"result":            "clean",
		"antivirus":         avResult,
		"basis_result":      "valid",
		"size":              size,
		"sha256":            hash,
		"connector_bytes":   beeResult.ConnectorBytes,
		"section_bytes":     beeResult.SectionBytes,
		"remote_sdk_format": beeResult.RemoteSDKFormat,
		"unique_version":    beeResult.UniqueVersion,
		"asset_name":        beeResult.AssetName,
		"asset_mode":        beeResult.AssetMode,
		"platform_count":    beeResult.PlatformCount,
		"tags":              connector.BasisBundleDescription.Tags,
		"metadata": map[string]any{
			"triangles_count":      connector.MetaData.TrianglesCount,
			"material_count":       connector.MetaData.MaterialCount,
			"bones_count":          connector.MetaData.BonesCount,
			"texture_memory_bytes": connector.MetaData.TextureMemoryBytes,
			"graphics_pipeline":    connector.MetaData.GraphicsPipeline,
		},
	}
	if err := w.autoPublishCleanFile(ctx, job.ID, file, scanResult); err != nil {
		return err
	}
	w.createEmbeddedPreviewFallback(ctx, file, connector)

	w.log.Info("worker_file_scan_completed",
		slog.String("job_id", job.ID),
		slog.String("file_id", file.FileID),
		slog.String("scan_status", "clean"),
		slog.String("content_status", "published"),
	)
	return nil
}

func (w *Worker) backfillEmbeddedPreviewFallback(ctx context.Context, file jobs.ContentFileForScan) embeddedPreviewFallbackOutcome {
	object, err := w.objects.GetObject(ctx, file.Bucket, file.StorageKey)
	if err != nil {
		w.log.Warn("embedded_preview_backfill_object_open_failed",
			slog.String("content_id", file.ContentID),
			slog.String("file_id", file.FileID),
			slog.String("error", err.Error()),
		)
		return embeddedPreviewFallbackFailed
	}

	unlockPassword, err := w.secrets.DecryptString(file.UnlockPasswordCiphertext)
	if err != nil {
		_ = object.Close()
		w.log.Warn("embedded_preview_backfill_password_decrypt_failed",
			slog.String("content_id", file.ContentID),
			slog.String("file_id", file.FileID),
			slog.String("error", err.Error()),
		)
		return embeddedPreviewFallbackFailed
	}

	_, connector, validateErr := basisbee.ValidateRemoteSDKBEEWithPassword(object, unlockPassword, w.cfg.MaxUploadBytes)
	closeErr := object.Close()
	if validateErr != nil {
		w.log.Warn("embedded_preview_backfill_basis_validate_failed",
			slog.String("content_id", file.ContentID),
			slog.String("file_id", file.FileID),
			slog.String("error", validateErr.Error()),
		)
		return embeddedPreviewFallbackFailed
	}
	if closeErr != nil {
		w.log.Warn("embedded_preview_backfill_object_close_failed",
			slog.String("content_id", file.ContentID),
			slog.String("file_id", file.FileID),
			slog.String("error", closeErr.Error()),
		)
		return embeddedPreviewFallbackFailed
	}

	return w.createEmbeddedPreviewFallback(ctx, file, connector)
}

func (w *Worker) createEmbeddedPreviewFallback(ctx context.Context, file jobs.ContentFileForScan, connector basisbee.Connector) embeddedPreviewFallbackOutcome {
	if strings.TrimSpace(connector.ImageBase64) == "" {
		return embeddedPreviewFallbackSkippedNoPreview
	}

	target, err := w.media.GetContentPreviewFallbackTarget(ctx, file.ContentID)
	if err != nil {
		w.log.Warn("embedded_preview_target_load_failed",
			slog.String("content_id", file.ContentID),
			slog.String("file_id", file.FileID),
			slog.String("error", err.Error()),
		)
		return embeddedPreviewFallbackFailed
	}
	if target.HasImages {
		return embeddedPreviewFallbackSkippedExisting
	}

	metadata, err := readEmbeddedPreviewImage(connector.ImageBase64, w.cfg.MaxImageBytes)
	if err != nil {
		w.log.Warn("embedded_preview_decode_failed",
			slog.String("content_id", file.ContentID),
			slog.String("file_id", file.FileID),
			slog.String("error", err.Error()),
		)
		return embeddedPreviewFallbackFailed
	}

	if err := w.objects.EnsureBucket(ctx, w.cfg.PreviewBucket); err != nil {
		w.log.Warn("embedded_preview_bucket_prepare_failed",
			slog.String("content_id", file.ContentID),
			slog.String("file_id", file.FileID),
			slog.String("error", err.Error()),
		)
		return embeddedPreviewFallbackFailed
	}

	pendingImageID := uuid.NewString()
	storageKey := fmt.Sprintf("pending/content/%s/images/%s%s", file.ContentID, pendingImageID, embeddedPreviewExtension(metadata.MIMEType))
	if err := w.objects.PutObject(ctx, w.cfg.PreviewBucket, storageKey, bytes.NewReader(metadata.Bytes), metadata.DecodedSize, metadata.MIMEType); err != nil {
		w.log.Warn("embedded_preview_store_failed",
			slog.String("content_id", file.ContentID),
			slog.String("file_id", file.FileID),
			slog.String("error", err.Error()),
		)
		return embeddedPreviewFallbackFailed
	}

	uploaded, err := w.media.CreateContentImageFallback(ctx, mediadomain.ImageUploadInput{
		OwnerUserID:       target.OwnerUserID,
		ContentID:         file.ContentID,
		Bucket:            w.cfg.PreviewBucket,
		StorageKey:        storageKey,
		OriginalFilename:  "basis-embedded-preview.png",
		AltText:           target.Title,
		Width:             metadata.Width,
		Height:            metadata.Height,
		FileSize:          metadata.DecodedSize,
		FileHashSHA256:    metadata.SHA256,
		MimeTypeDetected:  metadata.MIMEType,
		IsPrimary:         true,
		StorageQuotaBytes: w.cfg.UserStorageQuotaBytes,
	})
	if errors.Is(err, mediadomain.ErrContentImageFallbackExists) {
		_ = w.objects.RemoveObject(ctx, w.cfg.PreviewBucket, storageKey)
		return embeddedPreviewFallbackSkippedExisting
	}
	if errors.Is(err, contentdomain.ErrStorageQuotaExceeded) {
		_ = w.objects.RemoveObject(ctx, w.cfg.PreviewBucket, storageKey)
		w.log.Warn("embedded_preview_storage_quota_exceeded",
			slog.String("content_id", file.ContentID),
			slog.String("file_id", file.FileID),
			slog.Int64("image_size", metadata.DecodedSize),
		)
		return embeddedPreviewFallbackSkippedQuota
	}
	if err != nil {
		_ = w.objects.RemoveObject(ctx, w.cfg.PreviewBucket, storageKey)
		w.log.Warn("embedded_preview_metadata_store_failed",
			slog.String("content_id", file.ContentID),
			slog.String("file_id", file.FileID),
			slog.String("error", err.Error()),
		)
		return embeddedPreviewFallbackFailed
	}

	w.log.Info("embedded_preview_fallback_created",
		slog.String("content_id", file.ContentID),
		slog.String("file_id", file.FileID),
		slog.String("image_id", uploaded.ID),
	)
	return embeddedPreviewFallbackCreated
}

func readEmbeddedPreviewImage(raw string, maxBytes int64) (images.Metadata, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return images.Metadata{}, fmt.Errorf("embedded preview is empty")
	}
	if strings.HasPrefix(strings.ToLower(value), "data:") {
		comma := strings.IndexByte(value, ',')
		if comma < 0 {
			return images.Metadata{}, fmt.Errorf("embedded preview data URL is invalid")
		}
		value = value[comma+1:]
	}
	value = strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			return -1
		}
		return r
	}, value)
	decoder := base64.NewDecoder(base64.StdEncoding, strings.NewReader(value))
	return images.ReadAndValidate(decoder, maxBytes)
}

func embeddedPreviewExtension(mimeType string) string {
	if mimeType == images.MIMEJPEG {
		return ".jpg"
	}
	return ".png"
}

func (w *Worker) autoPublishCleanFile(ctx context.Context, jobID string, file jobs.ContentFileForScan, scanResult map[string]any) error {
	newBucket := file.Bucket
	newStorageKey := file.StorageKey
	promoted := false

	if file.Bucket == w.cfg.QuarantineBucket {
		if err := w.objects.EnsureBucket(ctx, w.cfg.PrivateBucket); err != nil {
			return fmt.Errorf("prepare approved storage: %w", err)
		}
		newBucket = w.cfg.PrivateBucket
		newStorageKey = fmt.Sprintf("content/%s/%s.bee", file.ContentID, file.FileID)
		if err := w.objects.CopyObject(ctx, file.Bucket, file.StorageKey, newBucket, newStorageKey); err != nil {
			return fmt.Errorf("promote clean content file: %w", err)
		}
		promoted = true
	}

	if err := w.jobs.CompleteFileScanAndPublish(ctx, jobID, file.FileID, file.ContentID, "clean", scanResult, newBucket, newStorageKey); err != nil {
		if promoted {
			_ = w.objects.RemoveObject(ctx, newBucket, newStorageKey)
		}
		return fmt.Errorf("auto-publish clean file scan: %w", err)
	}

	if promoted {
		_ = w.objects.RemoveObject(ctx, file.Bucket, file.StorageKey)
	}
	return nil
}

func (w *Worker) publishCleanCandidate(ctx context.Context, file jobs.ContentFileForScan) error {
	newBucket := file.Bucket
	newStorageKey := file.StorageKey
	promoted := false

	if file.Bucket == w.cfg.QuarantineBucket {
		if err := w.objects.EnsureBucket(ctx, w.cfg.PrivateBucket); err != nil {
			return fmt.Errorf("prepare approved storage: %w", err)
		}
		newBucket = w.cfg.PrivateBucket
		newStorageKey = fmt.Sprintf("content/%s/%s.bee", file.ContentID, file.FileID)
		if err := w.objects.CopyObject(ctx, file.Bucket, file.StorageKey, newBucket, newStorageKey); err != nil {
			return fmt.Errorf("promote clean content file: %w", err)
		}
		promoted = true
	}

	if err := w.jobs.PublishCleanContent(ctx, file.FileID, file.ContentID, newBucket, newStorageKey); err != nil {
		if promoted {
			_ = w.objects.RemoveObject(ctx, newBucket, newStorageKey)
		}
		return fmt.Errorf("publish clean content candidate: %w", err)
	}

	if promoted {
		_ = w.objects.RemoveObject(ctx, file.Bucket, file.StorageKey)
	}
	return nil
}

func (w *Worker) scanQuarantineObject(ctx context.Context, file jobs.ContentFileForScan) (antivirus.Result, error) {
	if w.av == nil {
		if w.cfg.ClamAVRequired {
			return antivirus.Result{}, fmt.Errorf("clamav scanner is required but not configured")
		}
		return antivirus.Result{
			Scanner: "clamav",
			Result:  "skipped",
			Reason:  "clamav_not_configured",
		}, nil
	}
	object, err := w.objects.GetObject(ctx, file.Bucket, file.StorageKey)
	if err != nil {
		return antivirus.Result{}, fmt.Errorf("open quarantine object for antivirus: %w", err)
	}
	defer object.Close()
	return w.av.Scan(ctx, object)
}

func (w *Worker) processImage(ctx context.Context, job jobs.Job) error {
	var payload jobs.ImageProcessPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("decode image processing payload: %w", err)
	}
	if payload.Kind == "" || payload.ImageID == "" {
		return fmt.Errorf("image processing payload is incomplete")
	}

	imageMeta, err := w.media.GetImageForProcessing(ctx, payload.Kind, payload.ImageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if markErr := w.jobs.MarkSucceeded(ctx, job.ID); markErr != nil {
				return fmt.Errorf("mark deleted image job succeeded: %w", markErr)
			}
			w.log.Info("worker_image_skipped",
				slog.String("job_id", job.ID),
				slog.String("image_id", payload.ImageID),
				slog.String("kind", payload.Kind),
			)
			return nil
		}
		return fmt.Errorf("load image for processing: %w", err)
	}

	object, err := w.objects.GetObject(ctx, imageMeta.Bucket, imageMeta.StorageKey)
	if err != nil {
		return fmt.Errorf("open pending image object: %w", err)
	}
	processed, mimeType, width, height, extension, err := images.Reencode(object, imageProcessingProfile(imageMeta.Kind))
	closeErr := object.Close()
	if err != nil {
		return fmt.Errorf("process image: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close pending image object: %w", closeErr)
	}

	sum := sha256.Sum256(processed)
	hash := hex.EncodeToString(sum[:])
	processedKey := processedImageKey(imageMeta.Kind, imageMeta.UserID, imageMeta.ContentID, imageMeta.ID, extension)
	if err := w.objects.PutObject(ctx, imageMeta.Bucket, processedKey, bytes.NewReader(processed), int64(len(processed)), mimeType); err != nil {
		return fmt.Errorf("store processed image: %w", err)
	}

	if err := w.media.CompleteImageProcessing(ctx, imageMeta.Kind, imageMeta.ID, imageMeta.Bucket, processedKey, width, height, int64(len(processed)), hash, mimeType); err != nil {
		_ = w.objects.RemoveObject(ctx, imageMeta.Bucket, processedKey)
		return fmt.Errorf("complete image processing: %w", err)
	}
	if imageMeta.Kind == "content_image" && imageMeta.ContentID != "" {
		if err := w.jobs.EnqueueSearchIndex(ctx, imageMeta.ContentID, "upsert_content"); err != nil {
			w.log.Error("search_refresh_enqueue_failed",
				slog.String("content_id", imageMeta.ContentID),
				slog.String("image_id", imageMeta.ID),
				slog.String("error", err.Error()),
			)
		}
	}

	if imageMeta.StorageKey != processedKey {
		_ = w.objects.RemoveObject(ctx, imageMeta.Bucket, imageMeta.StorageKey)
	}
	if err := w.jobs.MarkSucceeded(ctx, job.ID); err != nil {
		return fmt.Errorf("mark image job succeeded: %w", err)
	}

	w.log.Info("worker_image_processed",
		slog.String("job_id", job.ID),
		slog.String("image_id", imageMeta.ID),
		slog.String("kind", imageMeta.Kind),
	)
	return nil
}

func (w *Worker) processSearchIndex(ctx context.Context, job jobs.Job) error {
	if w.search == nil {
		return fmt.Errorf("meilisearch client is not configured")
	}
	var payload jobs.SearchIndexPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("decode search index payload: %w", err)
	}
	if payload.ContentID == "" {
		return fmt.Errorf("search index payload is incomplete")
	}

	switch payload.Action {
	case "", "upsert_content":
		doc, err := w.jobs.GetContentSearchDocument(ctx, payload.ContentID)
		if errors.Is(err, pgx.ErrNoRows) {
			if err := w.search.DeleteContent(ctx, payload.ContentID); err != nil {
				return fmt.Errorf("delete stale content search document: %w", err)
			}
			break
		}
		if err != nil {
			return fmt.Errorf("load content search document: %w", err)
		}
		if err := w.search.UpsertContent(ctx, doc); err != nil {
			return fmt.Errorf("upsert content search document: %w", err)
		}
	case "delete_content":
		if err := w.search.DeleteContent(ctx, payload.ContentID); err != nil {
			return fmt.Errorf("delete content search document: %w", err)
		}
	default:
		return fmt.Errorf("unsupported search index action %q", payload.Action)
	}

	if err := w.jobs.MarkSucceeded(ctx, job.ID); err != nil {
		return fmt.Errorf("mark search job succeeded: %w", err)
	}
	w.log.Info("worker_search_index_synced",
		slog.String("job_id", job.ID),
		slog.String("content_id", payload.ContentID),
		slog.String("action", payload.Action),
	)
	return nil
}

func (w *Worker) processEmailVerification(ctx context.Context, job jobs.Job) error {
	var payload jobs.EmailVerificationPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("decode email verification payload: %w", err)
	}
	if payload.UserID == "" || payload.Email == "" || payload.TokenID == "" || payload.Token == "" {
		return fmt.Errorf("email verification payload is incomplete")
	}

	token, err := w.jobs.GetEmailVerificationTokenForSend(ctx, payload.TokenID)
	if errors.Is(err, pgx.ErrNoRows) {
		if markErr := w.jobs.MarkSucceeded(ctx, job.ID); markErr != nil {
			return fmt.Errorf("mark missing email token job succeeded: %w", markErr)
		}
		w.log.Info("worker_email_verification_skipped",
			slog.String("job_id", job.ID),
			slog.String("reason", "token_missing"),
		)
		return nil
	}
	if err != nil {
		return fmt.Errorf("load email verification token: %w", err)
	}
	if token.UserID != payload.UserID || token.Email != payload.Email {
		return fmt.Errorf("email verification token metadata mismatch")
	}
	if token.UsedAt != nil || !token.ExpiresAt.After(time.Now().UTC()) {
		if markErr := w.jobs.MarkSucceeded(ctx, job.ID); markErr != nil {
			return fmt.Errorf("mark stale email token job succeeded: %w", markErr)
		}
		w.log.Info("worker_email_verification_skipped",
			slog.String("job_id", job.ID),
			slog.String("token_id", payload.TokenID),
			slog.String("reason", "token_stale"),
		)
		return nil
	}
	if w.email == nil {
		if !w.cfg.EmailDeliveryEnabled {
			if markErr := w.jobs.MarkSucceeded(ctx, job.ID); markErr != nil {
				return fmt.Errorf("mark disabled email job succeeded: %w", markErr)
			}
			w.log.Info("worker_email_verification_skipped",
				slog.String("job_id", job.ID),
				slog.String("token_id", payload.TokenID),
				slog.String("reason", "email_delivery_disabled"),
			)
			return nil
		}
		return fmt.Errorf("email sender is not configured")
	}

	messageID, err := w.email.SendEmailVerification(ctx, emailclient.VerificationEmail{
		To:            payload.Email,
		Username:      payload.Username,
		TokenID:       payload.TokenID,
		Token:         payload.Token,
		PublicBaseURL: w.cfg.PublicBaseURL,
		ExpiresAt:     token.ExpiresAt,
	})
	if err != nil {
		return err
	}
	if err := w.jobs.MarkSucceeded(ctx, job.ID); err != nil {
		return fmt.Errorf("mark email job succeeded: %w", err)
	}
	w.log.Info("worker_email_verification_sent",
		slog.String("job_id", job.ID),
		slog.String("token_id", payload.TokenID),
		slog.String("resend_message_id", messageID),
	)
	return nil
}

func (w *Worker) processPasswordChangeConfirmation(ctx context.Context, job jobs.Job) error {
	var payload jobs.PasswordChangeConfirmationPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("decode password change payload: %w", err)
	}
	if payload.UserID == "" || payload.Email == "" || payload.TokenID == "" || payload.Token == "" {
		return fmt.Errorf("password change payload is incomplete")
	}

	token, err := w.jobs.GetPasswordChangeTokenForSend(ctx, payload.TokenID)
	if errors.Is(err, pgx.ErrNoRows) {
		if markErr := w.jobs.MarkSucceeded(ctx, job.ID); markErr != nil {
			return fmt.Errorf("mark missing password token job succeeded: %w", markErr)
		}
		w.log.Info("worker_password_change_email_skipped",
			slog.String("job_id", job.ID),
			slog.String("reason", "token_missing"),
		)
		return nil
	}
	if err != nil {
		return fmt.Errorf("load password change token: %w", err)
	}
	if token.UserID != payload.UserID || token.Email != payload.Email {
		return fmt.Errorf("password change token metadata mismatch")
	}
	if token.UsedAt != nil || !token.ExpiresAt.After(time.Now().UTC()) {
		if markErr := w.jobs.MarkSucceeded(ctx, job.ID); markErr != nil {
			return fmt.Errorf("mark stale password token job succeeded: %w", markErr)
		}
		w.log.Info("worker_password_change_email_skipped",
			slog.String("job_id", job.ID),
			slog.String("token_id", payload.TokenID),
			slog.String("reason", "token_stale"),
		)
		return nil
	}
	if err := w.ensureEmailSender(ctx, job.ID, payload.TokenID, "worker_password_change_email_skipped"); err != nil {
		return err
	}
	if w.email == nil {
		return nil
	}

	messageID, err := w.email.SendPasswordChangeConfirmation(ctx, emailclient.PasswordChangeEmail{
		To:            payload.Email,
		Username:      payload.Username,
		TokenID:       payload.TokenID,
		Token:         payload.Token,
		PublicBaseURL: w.cfg.PublicBaseURL,
		ExpiresAt:     token.ExpiresAt,
	})
	if err != nil {
		return err
	}
	if err := w.jobs.MarkSucceeded(ctx, job.ID); err != nil {
		return fmt.Errorf("mark password change email job succeeded: %w", err)
	}
	w.log.Info("worker_password_change_email_sent",
		slog.String("job_id", job.ID),
		slog.String("token_id", payload.TokenID),
		slog.String("resend_message_id", messageID),
	)
	return nil
}

func (w *Worker) processEmailChangeConfirmation(ctx context.Context, job jobs.Job) error {
	var payload jobs.EmailChangeConfirmationPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("decode email change payload: %w", err)
	}
	if payload.UserID == "" || payload.Email == "" || payload.TokenID == "" || payload.Token == "" {
		return fmt.Errorf("email change payload is incomplete")
	}

	token, err := w.jobs.GetEmailChangeTokenForSend(ctx, payload.TokenID)
	if errors.Is(err, pgx.ErrNoRows) {
		if markErr := w.jobs.MarkSucceeded(ctx, job.ID); markErr != nil {
			return fmt.Errorf("mark missing email change token job succeeded: %w", markErr)
		}
		w.log.Info("worker_email_change_skipped",
			slog.String("job_id", job.ID),
			slog.String("reason", "token_missing"),
		)
		return nil
	}
	if err != nil {
		return fmt.Errorf("load email change token: %w", err)
	}
	if token.UserID != payload.UserID || token.NewEmail != payload.Email {
		return fmt.Errorf("email change token metadata mismatch")
	}
	if token.UsedAt != nil || !token.ExpiresAt.After(time.Now().UTC()) {
		if markErr := w.jobs.MarkSucceeded(ctx, job.ID); markErr != nil {
			return fmt.Errorf("mark stale email change token job succeeded: %w", markErr)
		}
		w.log.Info("worker_email_change_skipped",
			slog.String("job_id", job.ID),
			slog.String("token_id", payload.TokenID),
			slog.String("reason", "token_stale"),
		)
		return nil
	}
	if err := w.ensureEmailSender(ctx, job.ID, payload.TokenID, "worker_email_change_skipped"); err != nil {
		return err
	}
	if w.email == nil {
		return nil
	}

	messageID, err := w.email.SendEmailChangeConfirmation(ctx, emailclient.EmailChangeEmail{
		To:            payload.Email,
		Username:      payload.Username,
		TokenID:       payload.TokenID,
		Token:         payload.Token,
		PublicBaseURL: w.cfg.PublicBaseURL,
		ExpiresAt:     token.ExpiresAt,
	})
	if err != nil {
		return err
	}
	if err := w.jobs.MarkSucceeded(ctx, job.ID); err != nil {
		return fmt.Errorf("mark email change job succeeded: %w", err)
	}
	w.log.Info("worker_email_change_sent",
		slog.String("job_id", job.ID),
		slog.String("token_id", payload.TokenID),
		slog.String("resend_message_id", messageID),
	)
	return nil
}

func (w *Worker) ensureEmailSender(ctx context.Context, jobID string, tokenID string, logEvent string) error {
	if w.email != nil {
		return nil
	}
	if !w.cfg.EmailDeliveryEnabled {
		if markErr := w.jobs.MarkSucceeded(ctx, jobID); markErr != nil {
			return fmt.Errorf("mark disabled email job succeeded: %w", markErr)
		}
		w.log.Info(logEvent,
			slog.String("job_id", jobID),
			slog.String("token_id", tokenID),
			slog.String("reason", "email_delivery_disabled"),
		)
		return nil
	}
	return fmt.Errorf("email sender is not configured")
}

func processedImageKey(kind string, userID string, contentID string, imageID string, extension string) string {
	switch kind {
	case "user_avatar":
		return fmt.Sprintf("users/%s/avatar/%s%s", userID, imageID, extension)
	case "content_image":
		return fmt.Sprintf("content/%s/images/%s%s", contentID, imageID, extension)
	default:
		return fmt.Sprintf("images/%s%s", imageID, extension)
	}
}

func imageProcessingProfile(kind string) images.ProcessingProfile {
	if kind == "user_avatar" {
		return images.AvatarProfile()
	}
	return images.ContentProfile()
}
