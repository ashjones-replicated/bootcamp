package main

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"bootcamp/web/internal/config"
	"bootcamp/web/internal/db"
	"bootcamp/web/internal/handlers"
	"bootcamp/web/internal/metrics"
	"bootcamp/web/internal/upload"
	"golang.org/x/crypto/bcrypt"
)

//go:embed static templates
var staticFiles embed.FS

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config", "err", err)
		os.Exit(1)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		logger.Error("db", "err", err)
		os.Exit(1)
	}

	uploadClient := upload.New(cfg.UploadServiceURL, cfg.UploadAdminToken)

	if err := bootstrap(context.Background(), cfg, database); err != nil {
		logger.Error("bootstrap", "err", err)
		os.Exit(1)
	}

	app := &handlers.App{
		DB:     database,
		Upload: uploadClient,
		Cfg:    cfg,
		Files:  staticFiles,
		Log:    logger,
	}

	mux := http.NewServeMux()
	app.RegisterRoutes(mux)

	go func() {
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for range t.C {
			if err := database.DeleteExpiredSessions(context.Background()); err != nil {
				logger.Error("session cleanup", "err", err)
			}
		}
	}()

	if cfg.ReplicatedSDKURL != "" {
		reporter := metrics.NewReporter(cfg.ReplicatedSDKURL, database, uploadClient, logger)
		go func() {
			if err := reporter.Report(context.Background()); err != nil {
				logger.Warn("metrics report", "err", err)
			}
			t := time.NewTicker(time.Hour)
			defer t.Stop()
			for range t.C {
				if err := reporter.Report(context.Background()); err != nil {
					logger.Warn("metrics report", "err", err)
				}
			}
		}()
	}

	logger.Info("listening", "addr", cfg.BindAddress)
	if err := http.ListenAndServe(cfg.BindAddress, mux); err != nil {
		logger.Error("server", "err", err)
		os.Exit(1)
	}
}

func bootstrap(ctx context.Context, cfg *config.Config, database *db.DB) error {
	password := cfg.AdminPassword
	if password == "" {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return fmt.Errorf("generate password: %w", err)
		}
		password = hex.EncodeToString(b)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	existing, err := database.GetUserByUsername(ctx, "admin")
	if err != nil {
		return fmt.Errorf("lookup admin user: %w", err)
	}

	if existing == nil {
		// The upload service already has an "admin" token from its own initialisation
		// (UPLOAD_PRESETADMINPASSWORD). Reuse it rather than trying to create a duplicate.
		if _, err := database.CreateUser(ctx, "admin", string(hash), "admin", cfg.UploadAdminToken, true); err != nil {
			return fmt.Errorf("create admin user: %w", err)
		}
		if cfg.AdminPassword == "" {
			fmt.Printf("\n  Admin user created\n  username: admin\n  password: %s\n\n", password)
		}
		return nil
	}

	// Admin user already exists. If a preset password is configured, keep it in sync.
	if cfg.AdminPassword != "" {
		if err := database.UpdatePassword(ctx, existing.ID, string(hash)); err != nil {
			return fmt.Errorf("sync admin password: %w", err)
		}
	}
	return nil
}
