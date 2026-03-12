package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"openshare/backend/internal/model"
)

func TestImportLocalDirectoryCreatesMetadata(t *testing.T) {
	cfg := newRouterTestConfig(t)
	db := newRouterTestDB(t)
	admin := createRouterTestAdminWithAccess(t, db, adminAccess{
		username: "sysadmin",
		password: "s3cret-pass",
		role:     string(model.AdminRoleAdmin),
		permissions: []model.AdminPermission{
			model.AdminPermissionManageSystem,
		},
	})
	importRoot := createImportFixture(t)
	manager := newRouterSessionManager(db)
	engine := New(db, cfg, manager)

	cookieValue, _, err := manager.Create(t.Context(), admin)
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	body := bytes.NewBufferString(`{"root_path":"` + importRoot + `"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/admin/imports/local", body)
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: manager.CookieName(), Value: cookieValue, Path: "/"})
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", recorder.Code, recorder.Body.String())
	}

	var folderCount int64
	if err := db.Model(&model.Folder{}).Count(&folderCount).Error; err != nil {
		t.Fatalf("count folders failed: %v", err)
	}
	if folderCount != 2 {
		t.Fatalf("expected 2 folders, got %d", folderCount)
	}

	var fileCount int64
	if err := db.Model(&model.File{}).Count(&fileCount).Error; err != nil {
		t.Fatalf("count files failed: %v", err)
	}
	if fileCount != 2 {
		t.Fatalf("expected 2 files, got %d", fileCount)
	}

	var file model.File
	targetPath := filepath.Join(importRoot, "nested", "chapter1.txt")
	if err := db.Where("disk_path = ?", targetPath).Take(&file).Error; err != nil {
		t.Fatalf("find imported file failed: %v", err)
	}
	if file.Status != model.ResourceStatusActive {
		t.Fatalf("expected imported file active, got %q", file.Status)
	}
	if file.SubmissionID != nil {
		t.Fatal("expected imported file to have nil submission_id")
	}
}

func TestImportLocalDirectoryIsIncremental(t *testing.T) {
	cfg := newRouterTestConfig(t)
	db := newRouterTestDB(t)
	admin := createRouterTestAdminWithAccess(t, db, adminAccess{
		username: "sysadmin",
		password: "s3cret-pass",
		role:     string(model.AdminRoleAdmin),
		permissions: []model.AdminPermission{
			model.AdminPermissionManageSystem,
		},
	})
	importRoot := createImportFixture(t)
	manager := newRouterSessionManager(db)
	engine := New(db, cfg, manager)

	cookieValue, _, err := manager.Create(t.Context(), admin)
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	importBody := bytes.NewBufferString(`{"root_path":"` + importRoot + `"}`)
	for i := 0; i < 2; i++ {
		request := httptest.NewRequest(http.MethodPost, "/api/admin/imports/local", bytes.NewBuffer(importBody.Bytes()))
		request.Header.Set("Content-Type", "application/json")
		request.AddCookie(&http.Cookie{Name: manager.CookieName(), Value: cookieValue, Path: "/"})
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d, body=%s", recorder.Code, recorder.Body.String())
		}
	}

	var folderCount int64
	if err := db.Model(&model.Folder{}).Count(&folderCount).Error; err != nil {
		t.Fatalf("count folders failed: %v", err)
	}
	if folderCount != 2 {
		t.Fatalf("expected no duplicate folders, got %d", folderCount)
	}

	var fileCount int64
	if err := db.Model(&model.File{}).Count(&fileCount).Error; err != nil {
		t.Fatalf("count files failed: %v", err)
	}
	if fileCount != 2 {
		t.Fatalf("expected no duplicate files, got %d", fileCount)
	}
}

func TestFolderTreeAndBindTags(t *testing.T) {
	cfg := newRouterTestConfig(t)
	db := newRouterTestDB(t)
	admin := createRouterTestAdminWithAccess(t, db, adminAccess{
		username: "editor",
		password: "s3cret-pass",
		role:     string(model.AdminRoleAdmin),
		permissions: []model.AdminPermission{
			model.AdminPermissionManageSystem,
			model.AdminPermissionManageTags,
		},
	})
	importRoot := createImportFixture(t)
	manager := newRouterSessionManager(db)
	engine := New(db, cfg, manager)

	cookieValue, _, err := manager.Create(t.Context(), admin)
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	importRequest := httptest.NewRequest(http.MethodPost, "/api/admin/imports/local", bytes.NewBufferString(`{"root_path":"`+importRoot+`"}`))
	importRequest.Header.Set("Content-Type", "application/json")
	importRequest.AddCookie(&http.Cookie{Name: manager.CookieName(), Value: cookieValue, Path: "/"})
	importRecorder := httptest.NewRecorder()
	engine.ServeHTTP(importRecorder, importRequest)
	if importRecorder.Code != http.StatusOK {
		t.Fatalf("expected import status 200, got %d", importRecorder.Code)
	}

	var rootFolder model.Folder
	if err := db.Where("source_path = ?", importRoot).Take(&rootFolder).Error; err != nil {
		t.Fatalf("find root folder failed: %v", err)
	}

	tagRequest := httptest.NewRequest(http.MethodPut, "/api/admin/folders/"+rootFolder.ID+"/tags", bytes.NewBufferString(`{"tags":["课程","重点"]}`))
	tagRequest.Header.Set("Content-Type", "application/json")
	tagRequest.AddCookie(&http.Cookie{Name: manager.CookieName(), Value: cookieValue, Path: "/"})
	tagRecorder := httptest.NewRecorder()
	engine.ServeHTTP(tagRecorder, tagRequest)
	if tagRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected bind tags status 204, got %d body=%s", tagRecorder.Code, tagRecorder.Body.String())
	}

	treeRequest := httptest.NewRequest(http.MethodGet, "/api/admin/folders/tree", nil)
	treeRequest.AddCookie(&http.Cookie{Name: manager.CookieName(), Value: cookieValue, Path: "/"})
	treeRecorder := httptest.NewRecorder()
	engine.ServeHTTP(treeRecorder, treeRequest)
	if treeRecorder.Code != http.StatusOK {
		t.Fatalf("expected tree status 200, got %d body=%s", treeRecorder.Code, treeRecorder.Body.String())
	}

	var response struct {
		Items []struct {
			ID      string   `json:"id"`
			Tags    []string `json:"tags"`
			Folders []struct {
				Name string `json:"name"`
			} `json:"folders"`
			Files []struct {
				OriginalName string `json:"original_name"`
			} `json:"files"`
		} `json:"items"`
	}
	if err := json.Unmarshal(treeRecorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode tree response failed: %v", err)
	}
	if len(response.Items) != 1 {
		t.Fatalf("expected 1 root folder, got %d", len(response.Items))
	}
	if len(response.Items[0].Tags) != 2 {
		t.Fatalf("expected 2 tags, got %v", response.Items[0].Tags)
	}
	if len(response.Items[0].Folders) != 1 {
		t.Fatalf("expected 1 child folder, got %d", len(response.Items[0].Folders))
	}
	if len(response.Items[0].Files) != 1 {
		t.Fatalf("expected 1 root file, got %d", len(response.Items[0].Files))
	}
}

func createImportFixture(t *testing.T) string {
	t.Helper()

	root := filepath.Join(t.TempDir(), "import-root")
	if err := os.MkdirAll(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatalf("create import fixture dirs failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "root.pdf"), []byte("root file"), 0o644); err != nil {
		t.Fatalf("write root fixture file failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "chapter1.txt"), []byte("chapter one"), 0o644); err != nil {
		t.Fatalf("write nested fixture file failed: %v", err)
	}

	return root
}
