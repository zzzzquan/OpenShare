package session

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/gorm"

	"openshare/backend/internal/bootstrap"
	"openshare/backend/internal/config"
	"openshare/backend/internal/model"
	"openshare/backend/internal/repository"
	"openshare/backend/pkg/database"
	"openshare/backend/pkg/identity"
)

func TestManagerCreateAndResolve(t *testing.T) {
	db := newSessionTestDB(t)
	admin := createActiveAdmin(t, db, "alice")
	now := time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC)

	manager := NewManager(db, config.SessionConfig{
		Name:            "openshare_session",
		Secret:          "test-secret",
		Path:            "/",
		MaxAgeSeconds:   3600,
		HTTPOnly:        true,
		Secure:          false,
		SameSite:        "lax",
		RenewWindowSecs: 300,
	}, repository.NewAdminSessionRepository())
	manager.clock = fakeClock{now: now}

	cookieValue, identity, err := manager.Create(context.Background(), admin)
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	if identity.AdminID != admin.ID {
		t.Fatalf("expected admin id %q, got %q", admin.ID, identity.AdminID)
	}

	result, err := manager.Resolve(context.Background(), cookieValue)
	if err != nil {
		t.Fatalf("resolve session failed: %v", err)
	}

	if result.Identity.Username != admin.Username {
		t.Fatalf("expected username %q, got %q", admin.Username, result.Identity.Username)
	}
	if result.Renewed {
		t.Fatal("did not expect renewal for a fresh session")
	}
}

func TestManagerRenewsNearExpiry(t *testing.T) {
	db := newSessionTestDB(t)
	admin := createActiveAdmin(t, db, "bob")
	baseTime := time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC)

	manager := NewManager(db, config.SessionConfig{
		Name:            "openshare_session",
		Secret:          "test-secret",
		Path:            "/",
		MaxAgeSeconds:   3600,
		HTTPOnly:        true,
		Secure:          false,
		SameSite:        "lax",
		RenewWindowSecs: 600,
	}, repository.NewAdminSessionRepository())
	manager.clock = fakeClock{now: baseTime}

	cookieValue, identity, err := manager.Create(context.Background(), admin)
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	manager.clock = fakeClock{now: identity.ExpiresAt.Add(-5 * time.Minute)}

	result, err := manager.Resolve(context.Background(), cookieValue)
	if err != nil {
		t.Fatalf("resolve session failed: %v", err)
	}

	if !result.Renewed {
		t.Fatal("expected session renewal near expiry")
	}
	if !result.Identity.ExpiresAt.After(identity.ExpiresAt) {
		t.Fatal("expected renewed expiry to move forward")
	}
}

func TestManagerRenewsWhenWindowCoversFullSessionLifetime(t *testing.T) {
	db := newSessionTestDB(t)
	admin := createActiveAdmin(t, db, "carol")
	baseTime := time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC)

	manager := NewManager(db, config.SessionConfig{
		Name:            "openshare_session",
		Secret:          "test-secret",
		Path:            "/",
		MaxAgeSeconds:   7 * 24 * 60 * 60,
		HTTPOnly:        true,
		Secure:          false,
		SameSite:        "lax",
		RenewWindowSecs: 7 * 24 * 60 * 60,
	}, repository.NewAdminSessionRepository())
	manager.clock = fakeClock{now: baseTime}

	cookieValue, identity, err := manager.Create(context.Background(), admin)
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	manager.clock = fakeClock{now: baseTime.Add(12 * time.Hour)}

	result, err := manager.Resolve(context.Background(), cookieValue)
	if err != nil {
		t.Fatalf("resolve session failed: %v", err)
	}

	if !result.Renewed {
		t.Fatal("expected session renewal when renew window covers the full lifetime")
	}
	if !result.Identity.ExpiresAt.After(identity.ExpiresAt) {
		t.Fatal("expected renewed expiry to move forward")
	}
}

func TestWriteAndClearCookie(t *testing.T) {
	db := newSessionTestDB(t)
	manager := NewManager(db, config.SessionConfig{
		Name:            "openshare_session",
		Secret:          "test-secret",
		Path:            "/admin",
		MaxAgeSeconds:   3600,
		HTTPOnly:        true,
		Secure:          true,
		SameSite:        "strict",
		RenewWindowSecs: 300,
	}, repository.NewAdminSessionRepository())

	recorder := httptest.NewRecorder()
	expiresAt := time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC)
	manager.WriteCookie(recorder, "cookie-value", expiresAt)

	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}

	cookie := cookies[0]
	if cookie.Name != "openshare_session" {
		t.Fatalf("unexpected cookie name %q", cookie.Name)
	}
	if cookie.Path != "/admin" {
		t.Fatalf("unexpected cookie path %q", cookie.Path)
	}
	if !cookie.HttpOnly {
		t.Fatal("expected HttpOnly cookie")
	}
	if !cookie.Secure {
		t.Fatal("expected Secure cookie")
	}

	recorder = httptest.NewRecorder()
	manager.ClearCookie(recorder)
	cookies = recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie after clear, got %d", len(cookies))
	}
	if cookies[0].MaxAge != -1 {
		t.Fatalf("expected MaxAge=-1 when clearing cookie, got %d", cookies[0].MaxAge)
	}
}

type fakeClock struct {
	now time.Time
}

func (c fakeClock) Now() time.Time {
	return c.now
}

func newSessionTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "openshare-session-test.db")
	db, err := database.NewSQLite(database.Options{
		Path:      dbPath,
		LogLevel:  "silent",
		EnableWAL: true,
		Pragmas: []database.Pragma{
			{Name: "foreign_keys", Value: "ON"},
			{Name: "busy_timeout", Value: "5000"},
		},
	})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}

	if err := bootstrap.EnsureSchema(db); err != nil {
		t.Fatalf("ensure schema failed: %v", err)
	}

	return db
}

func createActiveAdmin(t *testing.T, db *gorm.DB, username string) *model.Admin {
	t.Helper()

	id, err := identity.NewID()
	if err != nil {
		t.Fatalf("generate admin id failed: %v", err)
	}

	admin := &model.Admin{
		ID:           id,
		Username:     username,
		PasswordHash: "hash",
		Role:         string(model.AdminRoleAdmin),
		Status:       model.AdminStatusActive,
	}
	if err := db.Create(admin).Error; err != nil {
		t.Fatalf("create admin failed: %v", err)
	}

	return admin
}
