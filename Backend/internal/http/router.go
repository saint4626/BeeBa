package http

import (
	"context"
	"log/slog"
	"strings"

	"beeba.org/internal/config"
	adminhandler "beeba.org/internal/http/handlers/admin"
	authhandler "beeba.org/internal/http/handlers/auth"
	categoryhandler "beeba.org/internal/http/handlers/categories"
	contenthandler "beeba.org/internal/http/handlers/content"
	downloadhandler "beeba.org/internal/http/handlers/downloads"
	"beeba.org/internal/http/handlers/health"
	mehandler "beeba.org/internal/http/handlers/me"
	mediahandler "beeba.org/internal/http/handlers/media"
	openapihandler "beeba.org/internal/http/handlers/openapi"
	searchhandler "beeba.org/internal/http/handlers/search"
	socialhandler "beeba.org/internal/http/handlers/social"
	taghandler "beeba.org/internal/http/handlers/tags"
	uploadhandler "beeba.org/internal/http/handlers/upload"
	userhandler "beeba.org/internal/http/handlers/users"
	"beeba.org/internal/http/httperror"
	"beeba.org/internal/http/middleware/authz"
	http_rate "beeba.org/internal/http/middleware/ratelimit"
	"beeba.org/internal/ratelimit"
	"beeba.org/internal/security/secretbox"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/helmet"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

type Dependencies struct {
	ReadinessPinger    Pinger
	CategoryStore      categoryhandler.Store
	AdminCategoryStore adminhandler.CategoryAdminStore
	TagStore           taghandler.Store
	AdminTagStore      adminhandler.TagAdminStore
	ContentStore       contenthandler.Store
	SearchStore        searchhandler.Store
	SearchClient       searchhandler.Searcher
	OwnedContentStore  mehandler.ContentStore
	AccountStore       mehandler.AccountStore
	DownloadStore      downloadhandler.Store
	MediaStore         mediahandler.Store
	SocialStore        socialhandler.Store
	PublicUserStore    userhandler.Store
	AdminUserStore     adminhandler.UserStore
	AuditStore         adminhandler.AuditStore
	JobStore           adminhandler.JobStore
	FileStore          adminhandler.FileStore
	UserStore          authhandler.UserStore
	UploadStore        uploadhandler.Store
	ModerationStore    adminhandler.ModerationStore
	ObjectStore        ObjectStore
	SecretBox          secretbox.Box
	RateLimiter        ratelimit.Limiter
}

type ObjectStore interface {
	uploadhandler.ObjectStore
	downloadhandler.ObjectStore
	mediahandler.ObjectStore
	adminhandler.PromotionStore
}

type Pinger interface {
	Ping(ctx context.Context) error
}

func NewRouter(cfg config.Config, log *slog.Logger, deps Dependencies) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      cfg.ServiceName,
		BodyLimit:    cfg.RequestBodyLimit,
		ErrorHandler: httperror.Handler,
		ReadTimeout:  cfg.HTTPReadTimeout,
		WriteTimeout: cfg.HTTPWriteTimeout,
		IdleTimeout:  cfg.HTTPIdleTimeout,
		TrustProxy:   cfg.TrustProxy,
		TrustProxyConfig: fiber.TrustProxyConfig{
			Proxies:  cfg.TrustedProxyProxies,
			Private:  cfg.TrustProxyPrivate,
			Loopback: cfg.TrustProxyLoopback,
		},
		ProxyHeader:        cfg.ProxyHeader,
		EnableIPValidation: cfg.TrustProxy,
	})

	app.Use(requestid.New())
	app.Use(recover.New())
	app.Use(helmet.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORSAllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID", "X-CSRF-Token"},
		AllowCredentials: true,
	}))
	app.Use(logger.New(logger.Config{
		Format: `{"time":"${time}","request_id":"${request_id}","method":"${method}","path":"${path}","status":${status},"latency":"${latency}","ip":"${ip}"}` + "\n",
		Stream: logWriter{log: log},
		CustomTags: map[string]logger.LogFunc{
			"request_id": func(output logger.Buffer, c fiber.Ctx, _ *logger.Data, _ string) (int, error) {
				return output.WriteString(requestid.FromContext(c))
			},
		},
	}))

	healthHandler := health.New(cfg, deps.ReadinessPinger, deps.RateLimiter)
	openAPIHandler := openapihandler.New()
	categoryHandler := categoryhandler.New(deps.CategoryStore)
	tagHandler := taghandler.New(deps.TagStore)
	contentHandler := contenthandler.New(deps.ContentStore)
	searchHandler := searchhandler.New(deps.SearchClient, deps.SearchStore)
	socialHandler := socialhandler.New(deps.SocialStore, deps.RateLimiter, cfg.RateLimits)
	downloadHandler := downloadhandler.New(cfg, deps.DownloadStore, deps.ObjectStore)
	mediaHandler := mediahandler.New(cfg, deps.MediaStore, deps.ObjectStore)
	userHandler := userhandler.New(deps.PublicUserStore)
	meContentHandler := mehandler.NewContentHandler(deps.OwnedContentStore)
	meAccountHandler := mehandler.NewAccountHandler(deps.AccountStore)
	authHandler := authhandler.New(deps.UserStore)
	uploadHandler := uploadhandler.New(cfg, deps.UploadStore, deps.ObjectStore, deps.SecretBox)
	adminStatusHandler := adminhandler.NewStatusHandler()
	adminModerationHandler := adminhandler.NewModerationHandler(cfg, log, deps.ModerationStore, deps.ObjectStore)
	adminUsersHandler := adminhandler.NewUsersHandler(deps.AdminUserStore)
	adminAuditHandler := adminhandler.NewAuditHandler(deps.AuditStore)
	adminJobsHandler := adminhandler.NewJobsHandler(deps.JobStore)
	adminFilesHandler := adminhandler.NewFilesHandler(deps.FileStore)
	adminTaxonomyHandler := adminhandler.NewTaxonomyHandler(deps.AdminCategoryStore, deps.AdminTagStore)
	rate := func(scope string, rule config.RateLimitRule) fiber.Handler {
		return http_rate.FixedWindow(deps.RateLimiter, scope, rule.Limit, rule.Window)
	}
	adminActionRate := rate("admin_action", cfg.RateLimits.AdminAction)

	app.Get("/healthz", healthHandler.Healthz)
	app.Get("/readyz", healthHandler.Readyz)
	app.Get("/openapi.yaml", openAPIHandler.YAML)

	v1 := app.Group("/api/v1")
	v1.Get("/status", healthHandler.APIStatus)
	v1.Get("/categories", categoryHandler.List)
	v1.Get("/tags", tagHandler.List)
	v1.Get("/users/:username", userHandler.Profile)
	v1.Get("/content", contentHandler.List)
	v1.Get("/content/:contentID/comments", socialHandler.ListComments)
	v1.Post("/content/:contentID/comments", authz.RequireBearer(deps.UserStore), socialHandler.CreateComment)
	v1.Post("/content/:contentID/like", authz.RequireBearer(deps.UserStore), socialHandler.Like)
	v1.Delete("/content/:contentID/like", authz.RequireBearer(deps.UserStore), socialHandler.Unlike)
	v1.Post("/content/:contentID/report", authz.RequireBearer(deps.UserStore), socialHandler.Report)
	v1.Get("/content/:contentID", contentHandler.Detail)
	v1.Get("/search", rate("search", cfg.RateLimits.Search), searchHandler.Content)
	v1.Get("/content/:contentID/download", rate("download_public", cfg.RateLimits.DownloadPublic), downloadHandler.Public)
	v1.Get("/media/:imageID", mediaHandler.Public)
	v1.Post("/content/uploads", rate("upload_content", cfg.RateLimits.UploadContent), authz.RequireBearer(deps.UserStore), uploadHandler.Create)
	authGroup := v1.Group("/auth")
	authGroup.Post("/register", rate("auth_register", cfg.RateLimits.AuthRegister), authHandler.Register)
	authGroup.Post("/login", rate("auth_login", cfg.RateLimits.AuthLogin), authHandler.Login)
	authGroup.Post("/email/verify", rate("auth_email_verify", cfg.RateLimits.AuthEmailVerify), authHandler.VerifyEmail)
	authGroup.Post("/refresh", rate("auth_refresh", cfg.RateLimits.AuthRefresh), authHandler.Refresh)
	authGroup.Post("/logout", authHandler.Logout)
	protectedAuthGroup := authGroup.Group("", authz.RequireBearer(deps.UserStore))
	protectedAuthGroup.Get("/me", authHandler.Me)
	protectedAuthGroup.Post("/email/resend", rate("auth_email_resend", cfg.RateLimits.AuthEmailResend), authHandler.ResendEmailVerification)
	meGroup := v1.Group("/me", authz.RequireBearer(deps.UserStore))
	meGroup.Patch("/profile", meAccountHandler.UpdateProfile)
	meGroup.Patch("/password", meAccountHandler.ChangePassword)
	meGroup.Get("/content", meContentHandler.List)
	meGroup.Patch("/content/:contentID", meContentHandler.Update)
	meGroup.Delete("/content/:contentID", meContentHandler.Delete)
	meGroup.Get("/content/:contentID/download", rate("download_owner", cfg.RateLimits.DownloadOwner), downloadHandler.Owner)
	meGroup.Get("/media/:imageID", mediaHandler.Owner)
	meGroup.Post("/avatar", rate("upload_avatar", cfg.RateLimits.UploadAvatar), mediaHandler.UploadAvatar)
	meGroup.Get("/content/:contentID/images", mediaHandler.ListContentImages)
	meGroup.Post("/content/:contentID/images", rate("upload_content_image", cfg.RateLimits.UploadContentImage), mediaHandler.UploadContentImage)
	meGroup.Patch("/content/:contentID/images/:imageID", mediaHandler.UpdateContentImage)
	meGroup.Delete("/content/:contentID/images/:imageID", mediaHandler.DeleteContentImage)

	adminGroup := v1.Group("/admin", authz.RequireBearer(deps.UserStore), authz.RequireRole("moderator", "admin", "owner"))
	adminGroup.Get("/status", adminStatusHandler.Status)
	adminGroup.Get("/moderation/content", adminModerationHandler.List)
	adminGroup.Get("/moderation/comments", adminModerationHandler.ListComments)
	adminGroup.Get("/moderation/reports", adminModerationHandler.ListReports)
	adminGroup.Post("/content/:contentID/approve", adminActionRate, adminModerationHandler.Approve)
	adminGroup.Post("/content/:contentID/reject", adminActionRate, adminModerationHandler.Reject)
	adminGroup.Post("/content/:contentID/hide", adminActionRate, adminModerationHandler.Hide)
	adminGroup.Post("/content/:contentID/restore", adminActionRate, adminModerationHandler.Restore)
	adminGroup.Post("/comments/:commentID/approve", adminActionRate, adminModerationHandler.ApproveComment)
	adminGroup.Post("/comments/:commentID/hide", adminActionRate, adminModerationHandler.HideComment)
	adminGroup.Post("/reports/:reportID/status", adminActionRate, adminModerationHandler.ReviewReport)
	adminOwnerGroup := v1.Group("/admin", authz.RequireBearer(deps.UserStore), authz.RequireRole("admin", "owner"))
	adminOwnerGroup.Get("/users", adminUsersHandler.List)
	adminOwnerGroup.Post("/users/:userID/ban", adminActionRate, adminUsersHandler.Ban)
	adminOwnerGroup.Post("/users/:userID/unban", adminActionRate, adminUsersHandler.Unban)
	adminOwnerGroup.Patch("/users/:userID/roles", adminActionRate, adminUsersHandler.SetRoles)
	adminOwnerGroup.Get("/audit-log", adminAuditHandler.List)
	adminOwnerGroup.Get("/jobs", adminJobsHandler.List)
	adminOwnerGroup.Post("/jobs/:jobID/retry", adminActionRate, adminJobsHandler.Retry)
	adminOwnerGroup.Get("/files", adminFilesHandler.List)
	adminOwnerGroup.Post("/files/:fileID/rescan", adminActionRate, adminFilesHandler.Rescan)
	adminOwnerGroup.Get("/categories", adminTaxonomyHandler.ListCategories)
	adminOwnerGroup.Patch("/categories/:categoryID", adminActionRate, adminTaxonomyHandler.UpdateCategory)
	adminOwnerGroup.Get("/tags", adminTaxonomyHandler.ListTags)
	adminOwnerGroup.Post("/tags", adminActionRate, adminTaxonomyHandler.CreateTag)
	adminOwnerGroup.Patch("/tags/:tagID", adminActionRate, adminTaxonomyHandler.UpdateTag)

	app.Use(httperror.NotFound)

	return app
}

type logWriter struct {
	log *slog.Logger
}

func (w logWriter) Write(p []byte) (int, error) {
	w.log.Info("http_request", slog.String("event", strings.TrimSpace(string(p))))
	return len(p), nil
}
