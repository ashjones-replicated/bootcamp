package session_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bootcamp/web/internal/session"
)

func TestGet_noCookie(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if got := session.Get(r); got != "" {
		t.Errorf("Get with no cookie = %q, want empty", got)
	}
}

func TestGet_withCookie(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: "session", Value: "abc123"})
	if got := session.Get(r); got != "abc123" {
		t.Errorf("Get = %q, want abc123", got)
	}
}

func TestGet_wrongCookieName(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: "other", Value: "should-not-be-returned"})
	if got := session.Get(r); got != "" {
		t.Errorf("Get with wrong cookie name = %q, want empty", got)
	}
}

func TestSet_cookieAttributes(t *testing.T) {
	w := httptest.NewRecorder()
	expires := time.Now().Add(time.Hour)
	session.Set(w, "sid123", expires, true)

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	c := cookies[0]

	if c.Name != "session" {
		t.Errorf("Name = %q, want session", c.Name)
	}
	if c.Value != "sid123" {
		t.Errorf("Value = %q, want sid123", c.Value)
	}
	if !c.HttpOnly {
		t.Error("expected HttpOnly=true")
	}
	if !c.Secure {
		t.Error("expected Secure=true when secure param is true")
	}
	if c.SameSite != http.SameSiteStrictMode {
		t.Errorf("SameSite = %v, want SameSiteStrictMode", c.SameSite)
	}
}

func TestSet_insecureCookie(t *testing.T) {
	w := httptest.NewRecorder()
	session.Set(w, "x", time.Now().Add(time.Hour), false)
	c := w.Result().Cookies()[0]
	if c.Secure {
		t.Error("expected Secure=false when secure param is false")
	}
}

func TestClear_setsMaxAgeNegative(t *testing.T) {
	w := httptest.NewRecorder()
	session.Clear(w)

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	c := cookies[0]
	if c.Name != "session" {
		t.Errorf("Name = %q, want session", c.Name)
	}
	if c.MaxAge != -1 {
		t.Errorf("MaxAge = %d, want -1", c.MaxAge)
	}
}
