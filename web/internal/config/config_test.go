package config_test

import (
	"encoding/base64"
	"testing"

	"bootcamp/web/internal/config"
)

func TestLoad_valid(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("UPLOAD_SERVICE_URL", "http://upload")
	t.Setenv("UPLOAD_ADMIN_TOKEN", "secret")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DatabaseURL != "postgres://localhost/test" {
		t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, "postgres://localhost/test")
	}
	want := base64.StdEncoding.EncodeToString([]byte("admin:secret"))
	if cfg.UploadAdminToken != want {
		t.Errorf("UploadAdminToken = %q, want %q", cfg.UploadAdminToken, want)
	}
}

func TestLoad_missingDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("UPLOAD_SERVICE_URL", "http://upload")
	t.Setenv("UPLOAD_ADMIN_TOKEN", "secret")

	if _, err := config.Load(); err == nil {
		t.Fatal("expected error for missing DATABASE_URL, got nil")
	}
}

func TestLoad_missingUploadServiceURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("UPLOAD_SERVICE_URL", "")
	t.Setenv("UPLOAD_ADMIN_TOKEN", "secret")

	if _, err := config.Load(); err == nil {
		t.Fatal("expected error for missing UPLOAD_SERVICE_URL, got nil")
	}
}

func TestLoad_uploadServiceURLTrailingSlashStripped(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("UPLOAD_SERVICE_URL", "http://upload/")
	t.Setenv("UPLOAD_ADMIN_TOKEN", "secret")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.UploadServiceURL != "http://upload" {
		t.Errorf("UploadServiceURL = %q, want trailing slash stripped", cfg.UploadServiceURL)
	}
}

func TestLoad_cookieSecureDefaultTrue(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("UPLOAD_SERVICE_URL", "http://upload")
	t.Setenv("UPLOAD_ADMIN_TOKEN", "secret")
	t.Setenv("COOKIE_SECURE", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.CookieSecure {
		t.Error("CookieSecure should default to true")
	}
}

func TestLoad_cookieSecureFalse(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("UPLOAD_SERVICE_URL", "http://upload")
	t.Setenv("UPLOAD_ADMIN_TOKEN", "secret")
	t.Setenv("COOKIE_SECURE", "false")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CookieSecure {
		t.Error("CookieSecure should be false when COOKIE_SECURE=false")
	}
}

func TestLoad_featureToggleDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("UPLOAD_SERVICE_URL", "http://upload")
	t.Setenv("UPLOAD_ADMIN_TOKEN", "secret")
	t.Setenv("ALLOW_PRIVATE_UPLOADS", "")
	t.Setenv("ALLOW_SINGLE_USE_LINKS", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.AllowPrivateUploads {
		t.Error("AllowPrivateUploads should default to true")
	}
	if !cfg.AllowSingleUseLinks {
		t.Error("AllowSingleUseLinks should default to true")
	}
}
