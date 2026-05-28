package app

import (
	"context"
	"log/slog"
	"strings"

	"beeba.org/internal/config"
	beebahttp "beeba.org/internal/http"
	"beeba.org/internal/ratelimit"
	"beeba.org/internal/repository/postgres"
	searchclient "beeba.org/internal/search/meilisearch"
	"beeba.org/internal/security/secretbox"
	miniostorage "beeba.org/internal/storage/minio"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
)

type API struct {
	cfg    config.Config
	log    *slog.Logger
	router *fiber.App
	db     *pgxpool.Pool
	redis  ratelimit.Limiter
}

func NewAPI(ctx context.Context, cfg config.Config, log *slog.Logger) (*API, error) {
	var db *pgxpool.Pool
	var deps beebahttp.Dependencies

	if strings.TrimSpace(cfg.DatabaseURL) != "" {
		secretBox, err := secretbox.New(cfg.SecretBoxKey)
		if err != nil {
			return nil, err
		}
		deps.SecretBox = secretBox

		pool, err := postgres.Open(ctx, cfg)
		if err != nil {
			return nil, err
		}
		db = pool
		deps.ReadinessPinger = pool
		categoryRepository := postgres.NewCategoryRepository(pool)
		deps.CategoryStore = categoryRepository
		deps.AdminCategoryStore = categoryRepository
		tagRepository := postgres.NewTagRepository(pool)
		deps.TagStore = tagRepository
		deps.AdminTagStore = tagRepository
		contentRepository := postgres.NewContentRepository(pool, secretBox)
		deps.ContentStore = contentRepository
		deps.SearchStore = contentRepository
		deps.OwnedContentStore = contentRepository
		downloadRepository := postgres.NewDownloadRepository(pool)
		deps.DownloadStore = downloadRepository
		mediaRepository := postgres.NewMediaRepository(pool)
		deps.MediaStore = mediaRepository
		userRepository := postgres.NewUserRepository(pool)
		deps.UserStore = userRepository
		deps.PublicUserStore = userRepository
		deps.AccountStore = userRepository
		deps.AdminUserStore = userRepository
		uploadRepository := postgres.NewUploadRepository(pool)
		deps.UploadStore = uploadRepository
		moderationRepository := postgres.NewModerationRepository(pool)
		deps.ModerationStore = moderationRepository
		jobRepository := postgres.NewJobRepository(pool)
		deps.JobStore = jobRepository
		adminFileRepository := postgres.NewAdminFileRepository(pool)
		deps.FileStore = adminFileRepository
		auditRepository := postgres.NewAuditRepository(pool)
		deps.AuditStore = auditRepository
		socialRepository := postgres.NewSocialRepository(pool)
		deps.SocialStore = socialRepository
		if strings.TrimSpace(cfg.MinIOEndpoint) != "" {
			objectStore, err := miniostorage.New(cfg)
			if err != nil {
				return nil, err
			}
			deps.ObjectStore = objectStore
		}
		if strings.TrimSpace(cfg.MeilisearchURL) != "" {
			search, err := searchclient.New(cfg.MeilisearchURL, cfg.MeilisearchAPIKey, cfg.MeilisearchContentIndex)
			if err != nil {
				return nil, err
			}
			deps.SearchClient = search
		}
		if strings.TrimSpace(cfg.RedisURL) != "" {
			limiter, err := ratelimit.NewRedisLimiter(ctx, cfg.RedisURL)
			if err != nil {
				return nil, err
			}
			deps.RateLimiter = limiter
		}
	}

	return &API{
		cfg:    cfg,
		log:    log,
		db:     db,
		redis:  deps.RateLimiter,
		router: beebahttp.NewRouter(cfg, log, deps),
	}, nil
}

func (a *API) Run() error {
	a.log.Info("api_starting", slog.String("addr", a.cfg.HTTPAddr))
	return a.router.Listen(a.cfg.HTTPAddr, fiber.ListenConfig{
		DisableStartupMessage: true,
	})
}

func (a *API) Shutdown(ctx context.Context) error {
	a.log.Info("api_stopping")
	err := a.router.ShutdownWithContext(ctx)
	if a.db != nil {
		a.db.Close()
	}
	if a.redis != nil {
		_ = a.redis.Close()
	}
	return err
}
