package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadRateLimitDefaults(t *testing.T) {
	t.Setenv("BEEBA_ENV", "development")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.RateLimits.UploadContent.Limit != 30 {
		t.Fatalf("UploadContent limit = %d, want 30", cfg.RateLimits.UploadContent.Limit)
	}
	if cfg.RateLimits.AuthLogin.Window != 15*time.Minute {
		t.Fatalf("AuthLogin window = %s, want 15m", cfg.RateLimits.AuthLogin.Window)
	}
	if cfg.RateLimits.AuthAccountChange.Limit != 5 {
		t.Fatalf("AuthAccountChange limit = %d, want 5", cfg.RateLimits.AuthAccountChange.Limit)
	}
	if cfg.RateLimits.AdminAction.Limit != 240 {
		t.Fatalf("AdminAction limit = %d, want 240", cfg.RateLimits.AdminAction.Limit)
	}
}

func TestLoadRateLimitOverrides(t *testing.T) {
	t.Setenv("BEEBA_ENV", "development")
	t.Setenv("BEEBA_RATE_LIMIT_UPLOAD_CONTENT_LIMIT", "31")
	t.Setenv("BEEBA_RATE_LIMIT_UPLOAD_CONTENT_WINDOW", "2h")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.RateLimits.UploadContent.Limit != 31 {
		t.Fatalf("UploadContent limit = %d, want 31", cfg.RateLimits.UploadContent.Limit)
	}
	if cfg.RateLimits.UploadContent.Window != 2*time.Hour {
		t.Fatalf("UploadContent window = %s, want 2h", cfg.RateLimits.UploadContent.Window)
	}
}

func TestLoadAccountChangeRateLimitOverrides(t *testing.T) {
	t.Setenv("BEEBA_ENV", "development")
	t.Setenv("BEEBA_RATE_LIMIT_AUTH_ACCOUNT_CHANGE_LIMIT", "3")
	t.Setenv("BEEBA_RATE_LIMIT_AUTH_ACCOUNT_CHANGE_WINDOW", "30m")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.RateLimits.AuthAccountChange.Limit != 3 {
		t.Fatalf("AuthAccountChange limit = %d, want 3", cfg.RateLimits.AuthAccountChange.Limit)
	}
	if cfg.RateLimits.AuthAccountChange.Window != 30*time.Minute {
		t.Fatalf("AuthAccountChange window = %s, want 30m", cfg.RateLimits.AuthAccountChange.Window)
	}
}

func TestLoadUserStorageQuotaDefault(t *testing.T) {
	t.Setenv("BEEBA_ENV", "development")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	const want = int64(10 * 1024 * 1024 * 1024)
	if cfg.UserStorageQuotaBytes != want {
		t.Fatalf("UserStorageQuotaBytes = %d, want %d", cfg.UserStorageQuotaBytes, want)
	}
}

func TestLoadUserStorageQuotaOverride(t *testing.T) {
	t.Setenv("BEEBA_ENV", "development")
	t.Setenv("BEEBA_USER_STORAGE_QUOTA_BYTES", "1073741824")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.UserStorageQuotaBytes != 1073741824 {
		t.Fatalf("UserStorageQuotaBytes = %d, want 1073741824", cfg.UserStorageQuotaBytes)
	}
}

func TestLoadServerCheckDefaults(t *testing.T) {
	t.Setenv("BEEBA_ENV", "development")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.ServerChecks.SchedulerInterval != time.Minute {
		t.Fatalf("ServerChecks.SchedulerInterval = %s, want 1m", cfg.ServerChecks.SchedulerInterval)
	}
	if cfg.ServerChecks.PendingInterval != time.Minute {
		t.Fatalf("ServerChecks.PendingInterval = %s, want 1m", cfg.ServerChecks.PendingInterval)
	}
	if cfg.ServerChecks.OnlineInterval != 5*time.Minute {
		t.Fatalf("ServerChecks.OnlineInterval = %s, want 5m", cfg.ServerChecks.OnlineInterval)
	}
	if cfg.ServerChecks.OfflineInterval != 2*time.Minute {
		t.Fatalf("ServerChecks.OfflineInterval = %s, want 2m", cfg.ServerChecks.OfflineInterval)
	}
	if cfg.ServerChecks.BatchSize != 12 {
		t.Fatalf("ServerChecks.BatchSize = %d, want 12", cfg.ServerChecks.BatchSize)
	}
}

func TestLoadServerCheckOverrides(t *testing.T) {
	t.Setenv("BEEBA_ENV", "development")
	t.Setenv("BEEBA_SERVER_CHECK_SCHEDULER_INTERVAL", "30s")
	t.Setenv("BEEBA_SERVER_CHECK_PENDING_INTERVAL", "45s")
	t.Setenv("BEEBA_SERVER_CHECK_ONLINE_INTERVAL", "10m")
	t.Setenv("BEEBA_SERVER_CHECK_OFFLINE_INTERVAL", "3m")
	t.Setenv("BEEBA_SERVER_CHECK_BATCH_SIZE", "7")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.ServerChecks.SchedulerInterval != 30*time.Second {
		t.Fatalf("ServerChecks.SchedulerInterval = %s, want 30s", cfg.ServerChecks.SchedulerInterval)
	}
	if cfg.ServerChecks.PendingInterval != 45*time.Second {
		t.Fatalf("ServerChecks.PendingInterval = %s, want 45s", cfg.ServerChecks.PendingInterval)
	}
	if cfg.ServerChecks.OnlineInterval != 10*time.Minute {
		t.Fatalf("ServerChecks.OnlineInterval = %s, want 10m", cfg.ServerChecks.OnlineInterval)
	}
	if cfg.ServerChecks.OfflineInterval != 3*time.Minute {
		t.Fatalf("ServerChecks.OfflineInterval = %s, want 3m", cfg.ServerChecks.OfflineInterval)
	}
	if cfg.ServerChecks.BatchSize != 7 {
		t.Fatalf("ServerChecks.BatchSize = %d, want 7", cfg.ServerChecks.BatchSize)
	}
}

func TestLoadRejectsInvalidServerCheckSchedule(t *testing.T) {
	t.Setenv("BEEBA_ENV", "development")
	t.Setenv("BEEBA_SERVER_CHECK_ONLINE_INTERVAL", "0")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want validation error")
	}
	if !strings.Contains(err.Error(), "BEEBA_SERVER_CHECK_ONLINE_INTERVAL") {
		t.Fatalf("Load() error = %q, want server check interval validation", err.Error())
	}
}

func TestHTTPTimeoutDefaultsAllowLargeMultipartUploads(t *testing.T) {
	t.Setenv("BEEBA_ENV", "development")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.HTTPReadTimeout < cfg.UploadStorageTimeout {
		t.Fatalf("HTTPReadTimeout = %s, want at least UploadStorageTimeout %s", cfg.HTTPReadTimeout, cfg.UploadStorageTimeout)
	}
	if cfg.HTTPWriteTimeout < cfg.UploadStorageTimeout {
		t.Fatalf("HTTPWriteTimeout = %s, want at least UploadStorageTimeout %s", cfg.HTTPWriteTimeout, cfg.UploadStorageTimeout)
	}
}

func TestLoadRejectsInvalidUserStorageQuota(t *testing.T) {
	t.Setenv("BEEBA_ENV", "development")
	t.Setenv("BEEBA_USER_STORAGE_QUOTA_BYTES", "0")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want validation error")
	}
	if !strings.Contains(err.Error(), "BEEBA_USER_STORAGE_QUOTA_BYTES") {
		t.Fatalf("Load() error = %q, want user storage quota validation", err.Error())
	}
}

func TestLoadRejectsInvalidRateLimit(t *testing.T) {
	t.Setenv("BEEBA_ENV", "development")
	t.Setenv("BEEBA_RATE_LIMIT_SEARCH_LIMIT", "0")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want validation error")
	}
	if !strings.Contains(err.Error(), "BEEBA_RATE_LIMIT_SEARCH_LIMIT") {
		t.Fatalf("Load() error = %q, want search limit validation", err.Error())
	}
}
