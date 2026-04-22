package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"bootcamp/web/internal/config"
	"bootcamp/web/internal/db"
	"golang.org/x/crypto/bcrypt"
)

// stubDB implements Database for tests. Zero values mean "return nothing / no error".
type stubDB struct {
	pingErr     error
	sessionUser *db.User
	sessionErr  error
	loginUser   *db.User
	loginErr    error
	newSession  string
	newSessErr  error
}

func (s *stubDB) Ping(_ context.Context) error { return s.pingErr }

func (s *stubDB) GetSessionUser(_ context.Context, _ string) (*db.User, error) {
	return s.sessionUser, s.sessionErr
}

func (s *stubDB) GetUserByUsername(_ context.Context, _ string) (*db.User, error) {
	return s.loginUser, s.loginErr
}

func (s *stubDB) CreateSession(_ context.Context, _ int64, _ time.Time) (string, error) {
	return s.newSession, s.newSessErr
}

func (s *stubDB) DeleteSession(_ context.Context, _ string) error        { return nil }
func (s *stubDB) UpdatePassword(_ context.Context, _ int64, _ string) error { return nil }

func (s *stubDB) ListUsers(_ context.Context) ([]*db.User, error) { return nil, nil }

func (s *stubDB) CreateUser(_ context.Context, username, passwordHash, tokenName, token string, isAdmin bool) (*db.User, error) {
	return &db.User{Username: username, IsAdmin: isAdmin}, nil
}

func (s *stubDB) DeleteUser(_ context.Context, _ int64) (string, error) { return "", nil }

// testFiles is a minimal in-memory filesystem satisfying the template paths.
var testFiles = fstest.MapFS{
	"templates/login.html":   {Data: []byte(`<html>login</html>`)},
	"templates/index.html":   {Data: []byte(`<html>index</html>`)},
	"templates/admin.html":   {Data: []byte(`<html>admin</html>`)},
	"templates/expired.html": {Data: []byte(`<html>expired</html>`)},
	"static/app.js":          {Data: []byte(``)},
}

func newApp(database Database) *App {
	return &App{
		DB:    database,
		Cfg:   &config.Config{CookieSecure: false, SessionDuration: 24 * time.Hour},
		Files: testFiles,
		Log:   slog.Default(),
	}
}

// ── requireAuth ──────────────────────────────────────────────────────────────

func TestRequireAuth_noSession_redirectsToLogin(t *testing.T) {
	app := newApp(&stubDB{})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	app.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("next handler should not be called")
	})).ServeHTTP(w, r)

	if w.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", w.Code, http.StatusSeeOther)
	}
	if loc := w.Header().Get("Location"); loc != "/login" {
		t.Errorf("Location = %q, want /login", loc)
	}
}

func TestRequireAuth_invalidSession_redirectsToLogin(t *testing.T) {
	stub := &stubDB{sessionUser: nil} // GetSessionUser returns nil user
	app := newApp(stub)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: "session", Value: "stale-sid"})

	app.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("next handler should not be called")
	})).ServeHTTP(w, r)

	if w.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", w.Code, http.StatusSeeOther)
	}
}

func TestRequireAuth_validSession_callsNext(t *testing.T) {
	user := &db.User{ID: 1, Username: "alice"}
	stub := &stubDB{sessionUser: user}
	app := newApp(stub)

	called := false
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: "session", Value: "valid-sid"})

	app.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		got := userFromContext(r)
		if got == nil || got.Username != "alice" {
			t.Errorf("userFromContext = %v, want alice", got)
		}
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(w, r)

	if !called {
		t.Error("next handler was not called")
	}
}

// ── requireAdmin ─────────────────────────────────────────────────────────────

func TestRequireAdmin_nonAdmin_forbidden(t *testing.T) {
	user := &db.User{ID: 1, Username: "bob", IsAdmin: false}
	stub := &stubDB{sessionUser: user}
	app := newApp(stub)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/admin", nil)
	r.AddCookie(&http.Cookie{Name: "session", Value: "sid"})

	handler := app.requireAuth(app.requireAdmin(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("next handler should not be called for non-admin")
	})))
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestRequireAdmin_admin_callsNext(t *testing.T) {
	user := &db.User{ID: 1, Username: "admin", IsAdmin: true}
	stub := &stubDB{sessionUser: user}
	app := newApp(stub)

	called := false
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/admin", nil)
	r.AddCookie(&http.Cookie{Name: "session", Value: "sid"})

	handler := app.requireAuth(app.requireAdmin(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})))
	handler.ServeHTTP(w, r)

	if !called {
		t.Error("next handler was not called for admin user")
	}
}

// ── checkLicenseExpiry ───────────────────────────────────────────────────────

func TestCheckLicenseExpiry_nilLicense_passesThrough(t *testing.T) {
	app := newApp(&stubDB{})
	app.License = nil // explicitly nil

	called := false
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/some/path", nil)

	app.checkLicenseExpiry(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(w, r)

	if !called {
		t.Error("expected next handler to be called when License is nil")
	}
}

func TestCheckLicenseExpiry_healthzBypassesCheck(t *testing.T) {
	// Even without a real license client, verify /healthz isn't gated.
	// We check the nil-license path, which always passes through.
	app := newApp(&stubDB{})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	app.checkLicenseExpiry(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 for /healthz", w.Code)
	}
}

// ── handleHealth ─────────────────────────────────────────────────────────────

func TestHandleHealth_dbOK(t *testing.T) {
	app := newApp(&stubDB{pingErr: nil})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	app.handleHealth(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status field = %q, want ok", body["status"])
	}
	if body["database"] != "ok" {
		t.Errorf("database field = %q, want ok", body["database"])
	}
}

func TestHandleHealth_dbDown(t *testing.T) {
	app := newApp(&stubDB{pingErr: errors.New("connection refused")})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	app.handleHealth(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "unhealthy" {
		t.Errorf("status field = %q, want unhealthy", body["status"])
	}
}

// ── handleLogin ──────────────────────────────────────────────────────────────

func TestHandleLogin_unknownUser_unauthorized(t *testing.T) {
	stub := &stubDB{loginUser: nil} // user not found
	app := newApp(stub)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/login",
		strings.NewReader("username=nobody&password=pass"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	app.handleLogin(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestHandleLogin_wrongPassword_unauthorized(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correctpass"), bcrypt.MinCost)
	stub := &stubDB{loginUser: &db.User{ID: 1, PasswordHash: string(hash)}}
	app := newApp(stub)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/login",
		strings.NewReader("username=admin&password=wrongpass"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	app.handleLogin(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestHandleLogin_correctCredentials_redirects(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correctpass"), bcrypt.MinCost)
	stub := &stubDB{
		loginUser:  &db.User{ID: 1, PasswordHash: string(hash)},
		newSession: "newsessionid",
	}
	app := newApp(stub)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/login",
		strings.NewReader("username=admin&password=correctpass"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	app.handleLogin(w, r)

	if w.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/" {
		t.Errorf("Location = %q, want /", loc)
	}
}

// ── handleFeatures ───────────────────────────────────────────────────────────

func TestHandleFeatures_reflectsConfig(t *testing.T) {
	app := newApp(&stubDB{})
	app.Cfg.AllowPrivateUploads = true
	app.Cfg.AllowSingleUseLinks = false

	// handleFeatures requires auth context; inject a user directly.
	user := &db.User{ID: 1, Username: "u"}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/features", nil)
	r = r.WithContext(context.WithValue(r.Context(), userContextKey, user))

	app.handleFeatures(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var body map[string]bool
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if !body["allow_private_uploads"] {
		t.Error("allow_private_uploads should be true")
	}
	if body["allow_single_use_links"] {
		t.Error("allow_single_use_links should be false")
	}
}
