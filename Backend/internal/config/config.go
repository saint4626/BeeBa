package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHTTPAddr         = ":8080"
	defaultEnvironment      = "development"
	defaultServiceName      = "beeba-api"
	defaultPublicBaseURL    = "http://localhost:4321"
	defaultCORSOrigins      = "http://localhost:4321,http://127.0.0.1:4321"
	defaultMaxUploadBytes   = 2 * 1024 * 1024 * 1024
	defaultMaxImageBytes    = 8 * 1024 * 1024
	defaultReadinessTimeout = 2 * time.Second
	defaultShutdownTimeout  = 10 * time.Second
	defaultUploadTimeout    = 20 * time.Minute
	defaultReadTimeout      = 30 * time.Second
	defaultWriteTimeout     = 2 * time.Minute
	defaultIdleTimeout      = 2 * time.Minute
	defaultRequestBodyLimit = int(defaultMaxUploadBytes + 1024*1024)
	defaultQuarantineBucket = "beeba-content-quarantine"
	defaultPrivateBucket    = "beeba-content-private"
	defaultPublicBucket     = "beeba-content-public"
	defaultPreviewBucket    = "beeba-content-previews"
	defaultBackupBucket     = "beeba-content-backups"
	developmentSecretBoxKey = "zcSZAHVJniJYGFVutzyqiz8hAWYhoxB83+gy9lJpabM="
	defaultContentIndex     = "beeba_content"
	defaultClamAVTimeout    = 20 * time.Minute
	defaultProxyHeader      = "X-Forwarded-For"
)

type RateLimitRule struct {
	Limit  int64
	Window time.Duration
}

type RateLimitConfig struct {
	Search             RateLimitRule
	DownloadPublic     RateLimitRule
	DownloadOwner      RateLimitRule
	UploadContent      RateLimitRule
	UploadAvatar       RateLimitRule
	UploadContentImage RateLimitRule
	AuthRegister       RateLimitRule
	AuthLogin          RateLimitRule
	AuthEmailVerify    RateLimitRule
	AuthRefresh        RateLimitRule
	AuthEmailResend    RateLimitRule
	SocialComments     RateLimitRule
	SocialLikes        RateLimitRule
	SocialReports      RateLimitRule
	AdminAction        RateLimitRule
}

var defaultRateLimits = RateLimitConfig{
	Search:             RateLimitRule{Limit: 120, Window: time.Minute},
	DownloadPublic:     RateLimitRule{Limit: 60, Window: time.Hour},
	DownloadOwner:      RateLimitRule{Limit: 120, Window: time.Hour},
	UploadContent:      RateLimitRule{Limit: 30, Window: time.Hour},
	UploadAvatar:       RateLimitRule{Limit: 20, Window: time.Hour},
	UploadContentImage: RateLimitRule{Limit: 40, Window: time.Hour},
	AuthRegister:       RateLimitRule{Limit: 10, Window: time.Hour},
	AuthLogin:          RateLimitRule{Limit: 20, Window: 15 * time.Minute},
	AuthEmailVerify:    RateLimitRule{Limit: 30, Window: time.Hour},
	AuthRefresh:        RateLimitRule{Limit: 120, Window: 15 * time.Minute},
	AuthEmailResend:    RateLimitRule{Limit: 5, Window: time.Hour},
	SocialComments:     RateLimitRule{Limit: 10, Window: time.Hour},
	SocialLikes:        RateLimitRule{Limit: 60, Window: time.Hour},
	SocialReports:      RateLimitRule{Limit: 5, Window: time.Hour},
	AdminAction:        RateLimitRule{Limit: 240, Window: time.Hour},
}

type Config struct {
	Environment             string
	ServiceName             string
	HTTPAddr                string
	PublicBaseURL           string
	CORSAllowedOrigins      []string
	DatabaseURL             string
	RedisURL                string
	MeilisearchURL          string
	MeilisearchAPIKey       string
	MeilisearchContentIndex string
	ClamAVAddr              string
	ClamAVRequired          bool
	ClamAVTimeout           time.Duration
	MinIOEndpoint           string
	MinIOAccessKey          string
	MinIOSecretKey          string
	MinIOUseSSL             bool
	QuarantineBucket        string
	PrivateBucket           string
	PublicBucket            string
	PreviewBucket           string
	BackupBucket            string
	SecretBoxKey            string
	MaxUploadBytes          int64
	MaxImageBytes           int64
	RequestBodyLimit        int
	ReadinessTimeout        time.Duration
	ShutdownTimeout         time.Duration
	UploadStorageTimeout    time.Duration
	HTTPReadTimeout         time.Duration
	HTTPWriteTimeout        time.Duration
	HTTPIdleTimeout         time.Duration
	TrustProxy              bool
	TrustedProxyProxies     []string
	TrustProxyPrivate       bool
	TrustProxyLoopback      bool
	ProxyHeader             string
	RateLimits              RateLimitConfig
}

func Load() (Config, error) {
	cfg := Config{
		Environment:             envString("BEEBA_ENV", defaultEnvironment),
		ServiceName:             envString("BEEBA_SERVICE_NAME", defaultServiceName),
		HTTPAddr:                envString("BEEBA_HTTP_ADDR", defaultHTTPAddr),
		PublicBaseURL:           envString("BEEBA_PUBLIC_BASE_URL", defaultPublicBaseURL),
		CORSAllowedOrigins:      envCSV("BEEBA_CORS_ALLOWED_ORIGINS", defaultCORSOrigins),
		DatabaseURL:             envString("BEEBA_DATABASE_URL", ""),
		RedisURL:                envString("BEEBA_REDIS_URL", ""),
		MeilisearchURL:          envString("BEEBA_MEILISEARCH_URL", ""),
		MeilisearchAPIKey:       envString("BEEBA_MEILISEARCH_API_KEY", ""),
		MeilisearchContentIndex: envString("BEEBA_MEILISEARCH_CONTENT_INDEX", defaultContentIndex),
		ClamAVAddr:              envString("BEEBA_CLAMAV_ADDR", ""),
		ClamAVRequired:          envBool("BEEBA_CLAMAV_REQUIRED", false),
		ClamAVTimeout:           envDuration("BEEBA_CLAMAV_TIMEOUT", defaultClamAVTimeout),
		MinIOEndpoint:           envString("BEEBA_MINIO_ENDPOINT", ""),
		MinIOAccessKey:          envString("BEEBA_MINIO_ACCESS_KEY", ""),
		MinIOSecretKey:          envString("BEEBA_MINIO_SECRET_KEY", ""),
		MinIOUseSSL:             envBool("BEEBA_MINIO_USE_SSL", false),
		QuarantineBucket:        envString("BEEBA_MINIO_QUARANTINE_BUCKET", defaultQuarantineBucket),
		PrivateBucket:           envString("BEEBA_MINIO_PRIVATE_BUCKET", defaultPrivateBucket),
		PublicBucket:            envString("BEEBA_MINIO_PUBLIC_BUCKET", defaultPublicBucket),
		PreviewBucket:           envString("BEEBA_MINIO_PREVIEW_BUCKET", defaultPreviewBucket),
		BackupBucket:            envString("BEEBA_MINIO_BACKUP_BUCKET", defaultBackupBucket),
		SecretBoxKey:            envString("BEEBA_SECRET_BOX_KEY", ""),
		MaxUploadBytes:          envInt64("BEEBA_MAX_UPLOAD_BYTES", defaultMaxUploadBytes),
		MaxImageBytes:           envInt64("BEEBA_MAX_IMAGE_UPLOAD_BYTES", defaultMaxImageBytes),
		RequestBodyLimit:        envInt("BEEBA_REQUEST_BODY_LIMIT", defaultRequestBodyLimit),
		ReadinessTimeout:        envDuration("BEEBA_READINESS_TIMEOUT", defaultReadinessTimeout),
		ShutdownTimeout:         envDuration("BEEBA_SHUTDOWN_TIMEOUT", defaultShutdownTimeout),
		UploadStorageTimeout:    envDuration("BEEBA_UPLOAD_STORAGE_TIMEOUT", defaultUploadTimeout),
		HTTPReadTimeout:         envDuration("BEEBA_HTTP_READ_TIMEOUT", defaultReadTimeout),
		HTTPWriteTimeout:        envDuration("BEEBA_HTTP_WRITE_TIMEOUT", defaultWriteTimeout),
		HTTPIdleTimeout:         envDuration("BEEBA_HTTP_IDLE_TIMEOUT", defaultIdleTimeout),
		TrustProxy:              envBool("BEEBA_TRUST_PROXY", false),
		TrustedProxyProxies:     envCSV("BEEBA_TRUST_PROXY_PROXIES", ""),
		TrustProxyPrivate:       envBool("BEEBA_TRUST_PROXY_PRIVATE", false),
		TrustProxyLoopback:      envBool("BEEBA_TRUST_PROXY_LOOPBACK", false),
		ProxyHeader:             envString("BEEBA_PROXY_HEADER", defaultProxyHeader),
		RateLimits:              loadRateLimits(),
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) IsProduction() bool {
	return c.Environment == "production"
}

func (c Config) validate() error {
	if strings.TrimSpace(c.HTTPAddr) == "" {
		return fmt.Errorf("BEEBA_HTTP_ADDR must not be empty")
	}
	if c.MaxUploadBytes <= 0 {
		return fmt.Errorf("BEEBA_MAX_UPLOAD_BYTES must be greater than 0")
	}
	if c.MaxImageBytes <= 0 {
		return fmt.Errorf("BEEBA_MAX_IMAGE_UPLOAD_BYTES must be greater than 0")
	}
	if c.RequestBodyLimit <= 0 {
		return fmt.Errorf("BEEBA_REQUEST_BODY_LIMIT must be greater than 0")
	}
	if int64(c.RequestBodyLimit) < c.MaxUploadBytes {
		return fmt.Errorf("BEEBA_REQUEST_BODY_LIMIT must be greater than or equal to BEEBA_MAX_UPLOAD_BYTES")
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("BEEBA_SHUTDOWN_TIMEOUT must be greater than 0")
	}
	if c.UploadStorageTimeout <= 0 {
		return fmt.Errorf("BEEBA_UPLOAD_STORAGE_TIMEOUT must be greater than 0")
	}
	if c.HTTPReadTimeout <= 0 {
		return fmt.Errorf("BEEBA_HTTP_READ_TIMEOUT must be greater than 0")
	}
	if c.HTTPWriteTimeout <= 0 {
		return fmt.Errorf("BEEBA_HTTP_WRITE_TIMEOUT must be greater than 0")
	}
	if c.HTTPIdleTimeout <= 0 {
		return fmt.Errorf("BEEBA_HTTP_IDLE_TIMEOUT must be greater than 0")
	}
	if c.ClamAVTimeout <= 0 {
		return fmt.Errorf("BEEBA_CLAMAV_TIMEOUT must be greater than 0")
	}
	if err := c.RateLimits.validate(); err != nil {
		return err
	}
	if c.IsProduction() && strings.TrimSpace(c.ClamAVAddr) == "" {
		return fmt.Errorf("BEEBA_CLAMAV_ADDR is required in production")
	}
	if _, err := url.ParseRequestURI(c.PublicBaseURL); err != nil {
		return fmt.Errorf("BEEBA_PUBLIC_BASE_URL is invalid: %w", err)
	}
	for _, origin := range c.CORSAllowedOrigins {
		if _, err := url.ParseRequestURI(origin); err != nil {
			return fmt.Errorf("BEEBA_CORS_ALLOWED_ORIGINS contains invalid origin %q: %w", origin, err)
		}
	}
	if c.IsProduction() {
		if c.SecretBoxKey == developmentSecretBoxKey {
			return fmt.Errorf("BEEBA_SECRET_BOX_KEY must not use the development key in production")
		}
		required := map[string]string{
			"BEEBA_DATABASE_URL":              c.DatabaseURL,
			"BEEBA_REDIS_URL":                 c.RedisURL,
			"BEEBA_MEILISEARCH_URL":           c.MeilisearchURL,
			"BEEBA_MEILISEARCH_API_KEY":       c.MeilisearchAPIKey,
			"BEEBA_MEILISEARCH_CONTENT_INDEX": c.MeilisearchContentIndex,
			"BEEBA_CLAMAV_ADDR":               c.ClamAVAddr,
			"BEEBA_MINIO_ENDPOINT":            c.MinIOEndpoint,
			"BEEBA_MINIO_ACCESS_KEY":          c.MinIOAccessKey,
			"BEEBA_MINIO_SECRET_KEY":          c.MinIOSecretKey,
			"BEEBA_PUBLIC_BASE_URL":           c.PublicBaseURL,
			"BEEBA_CORS_ALLOWED_ORIGINS":      strings.Join(c.CORSAllowedOrigins, ","),
			"BEEBA_MINIO_PRIVATE_BUCKET":      c.PrivateBucket,
			"BEEBA_MINIO_PUBLIC_BUCKET":       c.PublicBucket,
			"BEEBA_MINIO_PREVIEW_BUCKET":      c.PreviewBucket,
			"BEEBA_MINIO_BACKUP_BUCKET":       c.BackupBucket,
			"BEEBA_MINIO_QUARANTINE_BUCKET":   c.QuarantineBucket,
			"BEEBA_SECRET_BOX_KEY":            c.SecretBoxKey,
		}
		for key, value := range required {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("%s is required in production", key)
			}
		}
	}
	return nil
}

func loadRateLimits() RateLimitConfig {
	return RateLimitConfig{
		Search:             envRateLimit("BEEBA_RATE_LIMIT_SEARCH", defaultRateLimits.Search),
		DownloadPublic:     envRateLimit("BEEBA_RATE_LIMIT_DOWNLOAD_PUBLIC", defaultRateLimits.DownloadPublic),
		DownloadOwner:      envRateLimit("BEEBA_RATE_LIMIT_DOWNLOAD_OWNER", defaultRateLimits.DownloadOwner),
		UploadContent:      envRateLimit("BEEBA_RATE_LIMIT_UPLOAD_CONTENT", defaultRateLimits.UploadContent),
		UploadAvatar:       envRateLimit("BEEBA_RATE_LIMIT_UPLOAD_AVATAR", defaultRateLimits.UploadAvatar),
		UploadContentImage: envRateLimit("BEEBA_RATE_LIMIT_UPLOAD_CONTENT_IMAGE", defaultRateLimits.UploadContentImage),
		AuthRegister:       envRateLimit("BEEBA_RATE_LIMIT_AUTH_REGISTER", defaultRateLimits.AuthRegister),
		AuthLogin:          envRateLimit("BEEBA_RATE_LIMIT_AUTH_LOGIN", defaultRateLimits.AuthLogin),
		AuthEmailVerify:    envRateLimit("BEEBA_RATE_LIMIT_AUTH_EMAIL_VERIFY", defaultRateLimits.AuthEmailVerify),
		AuthRefresh:        envRateLimit("BEEBA_RATE_LIMIT_AUTH_REFRESH", defaultRateLimits.AuthRefresh),
		AuthEmailResend:    envRateLimit("BEEBA_RATE_LIMIT_AUTH_EMAIL_RESEND", defaultRateLimits.AuthEmailResend),
		SocialComments:     envRateLimit("BEEBA_RATE_LIMIT_SOCIAL_COMMENTS", defaultRateLimits.SocialComments),
		SocialLikes:        envRateLimit("BEEBA_RATE_LIMIT_SOCIAL_LIKES", defaultRateLimits.SocialLikes),
		SocialReports:      envRateLimit("BEEBA_RATE_LIMIT_SOCIAL_REPORTS", defaultRateLimits.SocialReports),
		AdminAction:        envRateLimit("BEEBA_RATE_LIMIT_ADMIN_ACTION", defaultRateLimits.AdminAction),
	}
}

func envRateLimit(prefix string, fallback RateLimitRule) RateLimitRule {
	return RateLimitRule{
		Limit:  envInt64(prefix+"_LIMIT", fallback.Limit),
		Window: envDuration(prefix+"_WINDOW", fallback.Window),
	}
}

func (r RateLimitConfig) validate() error {
	rules := map[string]RateLimitRule{
		"BEEBA_RATE_LIMIT_SEARCH":               r.Search,
		"BEEBA_RATE_LIMIT_DOWNLOAD_PUBLIC":      r.DownloadPublic,
		"BEEBA_RATE_LIMIT_DOWNLOAD_OWNER":       r.DownloadOwner,
		"BEEBA_RATE_LIMIT_UPLOAD_CONTENT":       r.UploadContent,
		"BEEBA_RATE_LIMIT_UPLOAD_AVATAR":        r.UploadAvatar,
		"BEEBA_RATE_LIMIT_UPLOAD_CONTENT_IMAGE": r.UploadContentImage,
		"BEEBA_RATE_LIMIT_AUTH_REGISTER":        r.AuthRegister,
		"BEEBA_RATE_LIMIT_AUTH_LOGIN":           r.AuthLogin,
		"BEEBA_RATE_LIMIT_AUTH_EMAIL_VERIFY":    r.AuthEmailVerify,
		"BEEBA_RATE_LIMIT_AUTH_REFRESH":         r.AuthRefresh,
		"BEEBA_RATE_LIMIT_AUTH_EMAIL_RESEND":    r.AuthEmailResend,
		"BEEBA_RATE_LIMIT_SOCIAL_COMMENTS":      r.SocialComments,
		"BEEBA_RATE_LIMIT_SOCIAL_LIKES":         r.SocialLikes,
		"BEEBA_RATE_LIMIT_SOCIAL_REPORTS":       r.SocialReports,
		"BEEBA_RATE_LIMIT_ADMIN_ACTION":         r.AdminAction,
	}

	for name, rule := range rules {
		if rule.Limit <= 0 {
			return fmt.Errorf("%s_LIMIT must be greater than 0", name)
		}
		if rule.Window <= 0 {
			return fmt.Errorf("%s_WINDOW must be greater than 0", name)
		}
	}
	return nil
}

func envString(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envCSV(key, fallback string) []string {
	raw := envString(key, fallback)
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

func envBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func envInt64(key string, fallback int64) int64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}
	return value
}

func envDuration(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return value
}
