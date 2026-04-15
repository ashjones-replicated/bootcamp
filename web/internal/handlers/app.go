package handlers

import (
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"bootcamp/web/internal/config"
	"bootcamp/web/internal/db"
	"bootcamp/web/internal/replicated"
	"bootcamp/web/internal/upload"
)

// App holds application dependencies and registers HTTP routes.
type App struct {
	DB      *db.DB
	Upload  *upload.Client
	Cfg     *config.Config
	Files   fs.FS
	Log     *slog.Logger
	Updates *replicated.UpdatesClient // nil when SDK is not configured
	License *replicated.LicenseClient // nil when SDK is not configured
}

// checkLicenseExpiry wraps a handler and serves the expired page for all
// non-static, non-health requests when the license has passed its expiry date.
func (a *App) checkLicenseExpiry(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.License != nil &&
			!strings.HasPrefix(r.URL.Path, "/static/") &&
			r.URL.Path != "/healthz" {
			expired, err := a.License.IsExpired(r.Context())
			if err != nil {
				a.Log.Warn("license expiry check failed", "err", err)
			}
			if expired {
				w.WriteHeader(http.StatusPaymentRequired)
				serveTemplate(w, a.Files, "templates/expired.html")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) RegisterRoutes(mux *http.ServeMux) http.Handler {
	// Static assets (CSS, JS)
	sub, _ := fs.Sub(a.Files, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static", http.FileServer(http.FS(sub))))

	// Public routes
	mux.HandleFunc("GET /login", a.handleLoginPage)
	mux.HandleFunc("POST /login", a.handleLogin)
	mux.HandleFunc("POST /logout", a.handleLogout)
	mux.HandleFunc("GET /healthz", a.handleHealth)

	// Authenticated routes
	mux.Handle("GET /{$}", a.requireAuth(http.HandlerFunc(a.handleIndex)))
	mux.Handle("POST /upload", a.requireAuth(http.HandlerFunc(a.handleUpload)))
	mux.Handle("GET /api/files", a.requireAuth(http.HandlerFunc(a.handleListFiles)))
	mux.Handle("GET /api/me", a.requireAuth(http.HandlerFunc(a.handleMe)))
	mux.Handle("GET /api/updates", a.requireAuth(http.HandlerFunc(a.handleUpdates)))
	mux.Handle("GET /files/{name}", a.requireAuth(http.HandlerFunc(a.handleFileProxy)))
	mux.Handle("DELETE /files/{name}", a.requireAuth(http.HandlerFunc(a.handleFileDelete)))
	mux.Handle("POST /api/password", a.requireAuth(http.HandlerFunc(a.handleChangePassword)))

	// Admin routes
	admin := func(h http.Handler) http.Handler { return a.requireAuth(a.requireAdmin(h)) }
	mux.Handle("GET /admin", admin(http.HandlerFunc(a.handleAdminPage)))
	mux.Handle("GET /api/admin/users", admin(http.HandlerFunc(a.handleListUsers)))
	mux.Handle("POST /api/admin/users", admin(http.HandlerFunc(a.handleCreateUser)))
	mux.Handle("DELETE /api/admin/users/{id}", admin(http.HandlerFunc(a.handleDeleteUser)))
	mux.Handle("GET /api/admin/entitlements", admin(http.HandlerFunc(a.handleEntitlements)))
	mux.Handle("POST /api/admin/supportbundle", admin(http.HandlerFunc(a.handleGenerateSupportBundle)))

	return a.checkLicenseExpiry(mux)
}
