package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/gorm"

	"openshare/backend/internal/config"
	"openshare/backend/internal/model"
)

func TestPublicDownloadServesActiveFile(t *testing.T) {
	cfg := newRouterTestConfig(t)
	db := newRouterTestDB(t)
	file := createRepositoryFileForDownload(t, cfg, db, model.ResourceStatusActive, "lecture.pdf", []byte("download-content"))
	engine := New(db, cfg, newRouterSessionManager(db))

	request := httptest.NewRequest(http.MethodGet, "/api/public/files/"+file.ID+"/download", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/pdf" {
		t.Fatalf("unexpected content-type %q", got)
	}
	if got := recorder.Header().Get("Content-Disposition"); got == "" {
		t.Fatal("expected content-disposition header")
	}
	if recorder.Body.String() != "download-content" {
		t.Fatalf("unexpected response body %q", recorder.Body.String())
	}

	assertEventuallyDownloadCount(t, db, file.ID, 1)
}

func TestPublicDownloadRejectsOfflineFile(t *testing.T) {
	cfg := newRouterTestConfig(t)
	db := newRouterTestDB(t)
	file := createRepositoryFileForDownload(t, cfg, db, model.ResourceStatusOffline, "lecture.pdf", []byte("hidden"))
	engine := New(db, cfg, newRouterSessionManager(db))

	request := httptest.NewRequest(http.MethodGet, "/api/public/files/"+file.ID+"/download", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", recorder.Code)
	}
}

func TestPublicDownloadReturnsGoneWhenRepositoryFileMissing(t *testing.T) {
	cfg := newRouterTestConfig(t)
	db := newRouterTestDB(t)
	file := createRepositoryFileForDownload(t, cfg, db, model.ResourceStatusActive, "lecture.pdf", []byte("download-content"))
	if err := os.Remove(file.DiskPath); err != nil {
		t.Fatalf("remove repository file failed: %v", err)
	}
	engine := New(db, cfg, newRouterSessionManager(db))

	request := httptest.NewRequest(http.MethodGet, "/api/public/files/"+file.ID+"/download", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusGone {
		t.Fatalf("expected status 410, got %d, body=%s", recorder.Code, recorder.Body.String())
	}
}

func createRepositoryFileForDownload(t *testing.T, cfg config.Config, db *gorm.DB, status model.ResourceStatus, originalName string, content []byte) *model.File {
	t.Helper()

	now := time.Date(2026, 3, 12, 15, 0, 0, 0, time.UTC)
	storedName := mustNewID(t) + filepath.Ext(originalName)
	diskPath := filepath.Join(cfg.Storage.Root, cfg.Storage.Repository, storedName)
	if err := os.WriteFile(diskPath, content, 0o644); err != nil {
		t.Fatalf("write repository file failed: %v", err)
	}

	file := &model.File{
		ID:            mustNewID(t),
		Title:         "公开资料",
		OriginalName:  originalName,
		StoredName:    storedName,
		Extension:     filepath.Ext(originalName),
		MimeType:      "application/pdf",
		Size:          int64(len(content)),
		DiskPath:      diskPath,
		Status:        status,
		DownloadCount: 0,
		UploaderIP:    "127.0.0.1",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := db.Create(file).Error; err != nil {
		t.Fatalf("create download file failed: %v", err)
	}

	return file
}

func assertEventuallyDownloadCount(t *testing.T, db *gorm.DB, fileID string, expected int64) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var file model.File
		if err := db.Where("id = ?", fileID).Take(&file).Error; err == nil && file.DownloadCount == expected {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	var file model.File
	if err := db.Where("id = ?", fileID).Take(&file).Error; err != nil {
		t.Fatalf("reload file failed: %v", err)
	}
	t.Fatalf("expected download_count=%d, got %d", expected, file.DownloadCount)
}
