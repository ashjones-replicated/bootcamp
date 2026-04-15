package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	BindAddress      string
	DatabaseURL      string
	UploadServiceURL string
	UploadAdminToken string
	AdminPassword    string
	CookieSecure     bool
	SessionDuration  time.Duration
	ReplicatedSDKURL string
	PodNamespace     string
}

func Load() (*Config, error) {
	c := &Config{
		BindAddress:      getenv("BIND_ADDRESS", ":8080"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		UploadServiceURL: strings.TrimRight(os.Getenv("UPLOAD_SERVICE_URL"), "/"),
		UploadAdminToken: base64.StdEncoding.EncodeToString([]byte("admin:" + os.Getenv("UPLOAD_ADMIN_TOKEN"))),
		AdminPassword:    os.Getenv("ADMIN_PASSWORD"),
		CookieSecure:     getenvBool("COOKIE_SECURE", true),
		SessionDuration:  24 * time.Hour,
		ReplicatedSDKURL: os.Getenv("REPLICATED_SDK_URL"),
		PodNamespace:     getenv("POD_NAMESPACE", "default"),
	}
	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if c.UploadServiceURL == "" {
		return nil, fmt.Errorf("UPLOAD_SERVICE_URL is required")
	}
	if c.UploadAdminToken == "" {
		return nil, fmt.Errorf("UPLOAD_ADMIN_TOKEN is required")
	}
	return c, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
