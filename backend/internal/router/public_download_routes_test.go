package router

import (
	"archive/zip"
	"bytes"
	"encoding/json"
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

func TestPublicFileDetailReturnsMetadata(t *testing.T) {
	cfg := newRouterTestConfig(t)
	db := newRouterTestDB(t)
	file := createRepositoryFileForDownload(t, cfg, db, model.ResourceStatusActive, "notes.txt", []byte("hello"))
	file.MimeType = "text/plain"
	file.Description = "detail"
	if err := db.Save(file).Error; err != nil {
		t.Fatalf("save detail file failed: %v", err)
	}
	engine := New(db, cfg, newRouterSessionManager(db))

	request := httptest.NewRequest(http.MethodGet, "/api/public/files/"+file.ID, nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		ID          string `json:"id"`
		Extension   string `json:"extension"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode detail response: %v", err)
	}
	if response.ID != file.ID || response.Description != "detail" || response.Extension != file.Extension {
		t.Fatalf("unexpected detail response: %+v", response)
	}
}

func TestPublicBatchDownloadStreamsZip(t *testing.T) {
	cfg := newRouterTestConfig(t)
	db := newRouterTestDB(t)
	first := createRepositoryFileForDownload(t, cfg, db, model.ResourceStatusActive, "a.txt", []byte("alpha"))
	first.MimeType = "text/plain"
	if err := db.Save(first).Error; err != nil {
		t.Fatalf("save first batch file failed: %v", err)
	}
	second := createRepositoryFileForDownload(t, cfg, db, model.ResourceStatusActive, "b.txt", []byte("beta"))
	second.MimeType = "text/plain"
	if err := db.Save(second).Error; err != nil {
		t.Fatalf("save second batch file failed: %v", err)
	}

	engine := New(db, cfg, newRouterSessionManager(db))
	body := bytes.NewBufferString(`{"file_ids":["` + first.ID + `","` + second.ID + `"]}`)
	request := httptest.NewRequest(http.MethodPost, "/api/public/files/batch-download", body)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/zip" {
		t.Fatalf("expected zip content type, got %q", got)
	}

	reader, err := zip.NewReader(bytes.NewReader(recorder.Body.Bytes()), int64(recorder.Body.Len()))
	if err != nil {
		t.Fatalf("read zip response failed: %v", err)
	}
	if len(reader.File) != 2 {
		t.Fatalf("expected 2 files in zip, got %d", len(reader.File))
	}

	assertEventuallyDownloadCount(t, db, first.ID, 1)
	assertEventuallyDownloadCount(t, db, second.ID, 1)
}

func TestPublicFolderDownloadStreamsZip(t *testing.T) {
	cfg := newRouterTestConfig(t)
	db := newRouterTestDB(t)
	rootFolder := createPublicDownloadFolder(t, db, nil, "课程资料")
	nestedFolder := createPublicDownloadFolder(t, db, &rootFolder.ID, "讲义")

	rootFile := createRepositoryFileForDownload(t, cfg, db, model.ResourceStatusActive, "overview.txt", []byte("overview"))
	rootFile.MimeType = "text/plain"
	rootFile.FolderID = &rootFolder.ID
	if err := db.Save(rootFile).Error; err != nil {
		t.Fatalf("save root folder file failed: %v", err)
	}

	nestedFile := createRepositoryFileForDownload(t, cfg, db, model.ResourceStatusActive, "chapter1.pdf", []byte("chapter"))
	nestedFile.FolderID = &nestedFolder.ID
	if err := db.Save(nestedFile).Error; err != nil {
		t.Fatalf("save nested folder file failed: %v", err)
	}

	engine := New(db, cfg, newRouterSessionManager(db))
	request := httptest.NewRequest(http.MethodGet, "/api/public/folders/"+rootFolder.ID+"/download", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/zip" {
		t.Fatalf("expected zip content type, got %q", got)
	}

	reader, err := zip.NewReader(bytes.NewReader(recorder.Body.Bytes()), int64(recorder.Body.Len()))
	if err != nil {
		t.Fatalf("read zip response failed: %v", err)
	}
	if len(reader.File) != 2 {
		t.Fatalf("expected 2 files in zip, got %d", len(reader.File))
	}

	names := []string{reader.File[0].Name, reader.File[1].Name}
	expected := map[string]bool{
		"课程资料/overview.txt":    false,
		"课程资料/讲义/chapter1.pdf": false,
	}
	for _, name := range names {
		if _, ok := expected[name]; !ok {
			t.Fatalf("unexpected zip entry %q", name)
		}
		expected[name] = true
	}
	for name, seen := range expected {
		if !seen {
			t.Fatalf("missing zip entry %q", name)
		}
	}

	assertEventuallyDownloadCount(t, db, rootFile.ID, 1)
	assertEventuallyDownloadCount(t, db, nestedFile.ID, 1)
}

func createRepositoryFileForDownload(t *testing.T, cfg config.Config, db *gorm.DB, status model.ResourceStatus, originalName string, content []byte) *model.File {
	t.Helper()

	now := time.Date(2026, 3, 12, 15, 0, 0, 0, time.UTC)
	storedName := mustNewID(t) + filepath.Ext(originalName)
	diskPath := filepath.Join(cfg.Storage.Root, storedName)
	if err := os.WriteFile(diskPath, content, 0o644); err != nil {
		t.Fatalf("write file for download failed: %v", err)
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

func createPublicDownloadFolder(t *testing.T, db *gorm.DB, parentID *string, name string) *model.Folder {
	t.Helper()

	now := time.Date(2026, 3, 12, 15, 0, 0, 0, time.UTC)
	folder := &model.Folder{
		ID:        mustNewID(t),
		ParentID:  parentID,
		Name:      name,
		Status:    model.ResourceStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Create(folder).Error; err != nil {
		t.Fatalf("create public download folder failed: %v", err)
	}

	return folder
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
