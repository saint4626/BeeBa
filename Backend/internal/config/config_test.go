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
