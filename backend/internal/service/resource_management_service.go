package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"

	"openshare/backend/internal/model"
	"openshare/backend/internal/repository"
	"openshare/backend/internal/storage"
	"openshare/backend/pkg/identity"
)

var (
	ErrManagedFileNotFound   = errors.New("managed file not found")
	ErrManagedFolderNotFound = errors.New("managed folder not found")
	ErrManagedFolderConflict = errors.New("managed folder conflict")
	ErrInvalidResourceEdit   = errors.New("invalid resource edit")
)

type ResourceManagementService struct {
	repo     *repository.ResourceManagementRepository
	storage  *storage.Service
	settings *SystemSettingService
	search   *SearchService
	nowFunc  func() time.Time
}

type ManagedFileItem struct {
	ID            string               `json:"id"`
	Title         string               `json:"title"`
	Description   string               `json:"description"`
	OriginalName  string               `json:"original_name"`
	Status        model.ResourceStatus `json:"status"`
	Size          int64                `json:"size"`
	DownloadCount int64                `json:"download_count"`
	FolderName    string               `json:"folder_name"`
	CreatedAt     time.Time            `json:"created_at"`
	UpdatedAt     time.Time            `json:"updated_at"`
}

type ListManagedFilesInput struct {
	Query  string
	Status string
}

type UpdateManagedFileInput struct {
	Title       string
	Extension   string
	Description string
	OperatorID  string
	OperatorIP  string
}

type UpdateManagedFolderDescriptionInput struct {
	Name        string
	Description string
	OperatorID  string
	OperatorIP  string
}

func NewResourceManagementService(repo *repository.ResourceManagementRepository, storageService *storage.Service, searchService *SearchService) *ResourceManagementService {
	return &ResourceManagementService{
		repo:    repo,
		storage: storageService,
		search:  searchService,
		nowFunc: func() time.Time { return time.Now().UTC() },
	}
}

func NewResourceManagementServiceWithSettings(repo *repository.ResourceManagementRepository, storageService *storage.Service, settings *SystemSettingService, searchService *SearchService) *ResourceManagementService {
	service := NewResourceManagementService(repo, storageService, searchService)
	service.settings = settings
	return service
}

func (s *ResourceManagementService) ListFiles(ctx context.Context, input ListManagedFilesInput) ([]ManagedFileItem, error) {
	rows, err := s.repo.ListFiles(ctx, input.Query, input.Status)
	if err != nil {
		return nil, err
	}
	items := make([]ManagedFileItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, ManagedFileItem{
			ID:            row.ID,
			Title:         row.Title,
			Description:   row.Description,
			OriginalName:  row.OriginalName,
			Status:        row.Status,
			Size:          row.Size,
			DownloadCount: row.DownloadCount,
			FolderName:    row.FolderName,
			CreatedAt:     row.CreatedAt,
			UpdatedAt:     row.UpdatedAt,
		})
	}
	return items, nil
}

func (s *ResourceManagementService) UpdateFile(ctx context.Context, fileID string, input UpdateManagedFileInput) error {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return ErrManagedFileNotFound
	}

	current, err := s.repo.FindFileByID(ctx, fileID)
	if err != nil {
		return err
	}
	if current == nil {
		return ErrManagedFileNotFound
	}

	title := strings.TrimSpace(input.Title)
	if title == "" {
		return ErrInvalidResourceEdit
	}
	extension, ok := normalizeManagedFileExtension(input.Extension, current.Extension)
	if !ok {
		return ErrInvalidResourceEdit
	}
	description := strings.TrimSpace(input.Description)
	originalName := buildManagedOriginalName(title, extension)
	logID, err := identity.NewID()
	if err != nil {
		return fmt.Errorf("generate resource update log id: %w", err)
	}
	if err := s.repo.UpdateFileMetadata(ctx, fileID, title, extension, originalName, description, input.OperatorID, input.OperatorIP, logID, s.nowFunc()); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrManagedFileNotFound
		}
		return fmt.Errorf("update managed file: %w", err)
	}
	if s.search != nil {
		_ = s.search.IndexFile(ctx, fileID, title, description)
	}
	return nil
}

func (s *ResourceManagementService) UpdateFolderDescription(ctx context.Context, folderID string, input UpdateManagedFolderDescriptionInput) error {
	folderID = strings.TrimSpace(folderID)
	if folderID == "" {
		return ErrManagedFolderNotFound
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return ErrInvalidResourceEdit
	}

	current, err := s.repo.FindFolderByID(ctx, folderID)
	if err != nil {
		return err
	}
	if current == nil {
		return ErrManagedFolderNotFound
	}

	conflict, err := s.repo.FolderNameExists(ctx, current.ParentID, name, current.ID)
	if err != nil {
		return err
	}
	if conflict {
		return ErrManagedFolderConflict
	}

	logID, err := identity.NewID()
	if err != nil {
		return fmt.Errorf("generate folder update log id: %w", err)
	}

	description := strings.TrimSpace(input.Description)
	if current.SourcePath == nil || strings.TrimSpace(*current.SourcePath) == "" || current.Name == name {
		if err := s.repo.UpdateFolderMetadata(ctx, folderID, name, description, input.OperatorID, input.OperatorIP, logID, s.nowFunc()); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrManagedFolderNotFound
			}
			return fmt.Errorf("update folder metadata: %w", err)
		}
		if s.search != nil {
			_ = s.search.IndexFolder(ctx, folderID, name, description)
		}
		return nil
	}

	folders, err := s.repo.ListFolderPaths(ctx)
	if err != nil {
		return fmt.Errorf("list folder paths: %w", err)
	}
	files, err := s.repo.ListFilePaths(ctx)
	if err != nil {
		return fmt.Errorf("list file paths: %w", err)
	}

	oldRootPath := strings.TrimSpace(*current.SourcePath)
	newRootPath, err := s.storage.RenameManagedDirectory(oldRootPath, name)
	if err != nil {
		if errors.Is(err, storage.ErrManagedDirectoryConflict) {
			return ErrManagedFolderConflict
		}
		return fmt.Errorf("rename managed directory: %w", err)
	}

	folderSourcePaths := map[string]string{
		current.ID: newRootPath,
	}
	for _, folder := range folders {
		if folder.SourcePath == nil {
			continue
		}
		sourcePath := strings.TrimSpace(*folder.SourcePath)
		if sourcePath == "" || sourcePath == oldRootPath || !isPathWithinRoot(sourcePath, oldRootPath) {
			continue
		}
		relative, relErr := filepath.Rel(oldRootPath, sourcePath)
		if relErr != nil {
			return fmt.Errorf("resolve folder relative path: %w", relErr)
		}
		folderSourcePaths[folder.ID] = filepath.Join(newRootPath, relative)
	}

	filePathUpdates := make(map[string]repository.ManagedFilePathRow)
	for _, file := range files {
		updated := file
		changed := false
		if sourcePath := normalizePathPointer(file.SourcePath); sourcePath != "" && (sourcePath == oldRootPath || isPathWithinRoot(sourcePath, oldRootPath)) {
			relative, relErr := filepath.Rel(oldRootPath, sourcePath)
			if relErr != nil {
				return fmt.Errorf("resolve file source relative path: %w", relErr)
			}
			next := filepath.Join(newRootPath, relative)
			updated.SourcePath = &next
			changed = true
		}
		if diskPath := strings.TrimSpace(file.DiskPath); diskPath != "" && (diskPath == oldRootPath || isPathWithinRoot(diskPath, oldRootPath)) {
			relative, relErr := filepath.Rel(oldRootPath, diskPath)
			if relErr != nil {
				return fmt.Errorf("resolve file disk relative path: %w", relErr)
			}
			updated.DiskPath = filepath.Join(newRootPath, relative)
			changed = true
		}
		if changed {
			filePathUpdates[file.ID] = updated
		}
	}

	if err := s.repo.UpdateFolderTreePaths(ctx, folderID, name, description, folderSourcePaths, filePathUpdates, input.OperatorID, input.OperatorIP, logID, s.nowFunc()); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrManagedFolderNotFound
		}
		return fmt.Errorf("update folder tree paths: %w", err)
	}
	if s.search != nil {
		_ = s.search.RebuildAllIndexes(ctx)
	}
	return nil
}

func normalizePathPointer(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func isPathWithinRoot(path, root string) bool {
	path = filepath.Clean(strings.TrimSpace(path))
	root = filepath.Clean(strings.TrimSpace(root))
	if path == "" || root == "" {
		return false
	}
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}

func (s *ResourceManagementService) OfflineFile(ctx context.Context, fileID string, operatorID string, operatorIP string) error {
	current, err := s.repo.FindFileByID(ctx, strings.TrimSpace(fileID))
	if err != nil {
		return err
	}
	if current == nil {
		return ErrManagedFileNotFound
	}
	logID, err := identity.NewID()
	if err != nil {
		return fmt.Errorf("generate resource offline log id: %w", err)
	}
	if err := s.repo.UpdateFileStatusWithLog(ctx, current.ID, model.ResourceStatusOffline, nil, current.DiskPath, operatorID, operatorIP, "resource_offlined", current.Title, logID, s.nowFunc()); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrManagedFileNotFound
		}
		return fmt.Errorf("offline managed file: %w", err)
	}
	return nil
}

func (s *ResourceManagementService) DeleteFile(ctx context.Context, fileID string, operatorID string, operatorIP string) error {
	current, err := s.repo.FindFileByID(ctx, strings.TrimSpace(fileID))
	if err != nil {
		return err
	}
	if current == nil {
		return ErrManagedFileNotFound
	}

	newPath, err := s.storage.MoveManagedFileToTrash(current.DiskPath)
	if err != nil {
		return fmt.Errorf("move managed file to trash: %w", err)
	}

	now := s.nowFunc()
	logID, err := identity.NewID()
	if err != nil {
		return fmt.Errorf("generate resource delete log id: %w", err)
	}
	if err := s.repo.UpdateFileStatusWithLog(ctx, current.ID, model.ResourceStatusDeleted, &now, newPath, operatorID, operatorIP, "resource_deleted", current.Title, logID, now); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrManagedFileNotFound
		}
		return fmt.Errorf("delete managed file: %w", err)
	}
	if s.search != nil {
		_ = s.search.RemoveFromIndex(ctx, "file", current.ID)
	}
	return nil
}

func (s *ResourceManagementService) DeleteFolder(ctx context.Context, folderID string, operatorID string, operatorIP string) error {
	current, err := s.repo.FindFolderByID(ctx, strings.TrimSpace(folderID))
	if err != nil {
		return err
	}
	if current == nil {
		return ErrManagedFolderNotFound
	}

	folderIDs, err := s.repo.ListFolderTreeIDs(ctx, current.ID)
	if err != nil {
		return err
	}
	if len(folderIDs) == 0 {
		return ErrManagedFolderNotFound
	}

	newPath := ""
	if current.SourcePath != nil && strings.TrimSpace(*current.SourcePath) != "" {
		newPath, err = s.storage.MoveManagedDirectoryToTrash(*current.SourcePath)
		if err != nil {
			return fmt.Errorf("move managed folder to trash: %w", err)
		}
	}

	now := s.nowFunc()
	logID, err := identity.NewID()
	if err != nil {
		return fmt.Errorf("generate folder delete log id: %w", err)
	}
	if err := s.repo.DeleteFolderTreeWithLog(ctx, current.ID, folderIDs, newPath, operatorID, operatorIP, logID, current.Name, now); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrManagedFolderNotFound
		}
		return fmt.Errorf("delete managed folder: %w", err)
	}
	if s.search != nil {
		_ = s.search.RebuildAllIndexes(ctx)
	}
	return nil
}

func (s *ResourceManagementService) PublicUpdateFile(ctx context.Context, fileID string, input UpdateManagedFileInput) error {
	policy, err := s.guestPolicy(ctx)
	if err != nil {
		return err
	}
	if !policy.AllowGuestEditTitle && !policy.AllowGuestEditDescription {
		return ErrInvalidResourceEdit
	}

	current, err := s.repo.FindFileByID(ctx, strings.TrimSpace(fileID))
	if err != nil {
		return err
	}
	if current == nil || current.Status != model.ResourceStatusActive {
		return ErrManagedFileNotFound
	}

	merged := input
	if !policy.AllowGuestEditTitle {
		merged.Title = current.Title
	}
	if !policy.AllowGuestEditDescription {
		merged.Description = current.Description
	}
	merged.Extension = current.Extension
	merged.OperatorID = ""
	return s.UpdateFile(ctx, fileID, merged)
}

func normalizeManagedFileExtension(raw string, fallback string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		trimmed = strings.TrimSpace(fallback)
	}
	if trimmed == "" {
		return "", true
	}
	trimmed = strings.TrimPrefix(trimmed, ".")
	trimmed = strings.ToLower(strings.TrimSpace(trimmed))
	if trimmed == "" {
		return "", true
	}
	if strings.ContainsAny(trimmed, `/\ `) {
		return "", false
	}
	return "." + trimmed, true
}

func buildManagedOriginalName(title string, extension string) string {
	if extension == "" {
		return title
	}
	return title + extension
}

func (s *ResourceManagementService) PublicDeleteFile(ctx context.Context, fileID string, operatorIP string) error {
	policy, err := s.guestPolicy(ctx)
	if err != nil {
		return err
	}
	if !policy.AllowGuestResourceDelete {
		return ErrInvalidResourceEdit
	}

	current, err := s.repo.FindFileByID(ctx, strings.TrimSpace(fileID))
	if err != nil {
		return err
	}
	if current == nil || current.Status != model.ResourceStatusActive {
		return ErrManagedFileNotFound
	}

	return s.DeleteFile(ctx, fileID, "", operatorIP)
}

func (s *ResourceManagementService) guestPolicy(ctx context.Context) (GuestPolicy, error) {
	if s.settings == nil {
		return GuestPolicy{}, nil
	}

	policy, err := s.settings.GetPolicy(ctx)
	if err != nil {
		return GuestPolicy{}, fmt.Errorf("load system policy: %w", err)
	}
	if policy == nil {
		return GuestPolicy{}, nil
	}
	return policy.Guest, nil
}
