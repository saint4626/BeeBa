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
