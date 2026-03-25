package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"openshare/backend/internal/model"
	"openshare/backend/internal/repository"
	"openshare/backend/internal/storage"
)

var (
	ErrDownloadFileNotFound    = errors.New("download file not found")
	ErrDownloadFolderNotFound  = errors.New("download folder not found")
	ErrDownloadFileUnavailable = errors.New("download file unavailable")
	ErrBatchDownloadInvalid    = errors.New("invalid batch download request")
)

type PublicDownloadService struct {
	repository *repository.PublicDownloadRepository
	storage    *storage.Service
}

type DownloadableFile struct {
	FileID       string
	OriginalName string
	MimeType     string
	Size         int64
	ModTime      time.Time
	Content      *os.File
}

type PublicFileDetail struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Extension     string    `json:"extension"`
	FolderID      string    `json:"folder_id"`
	Path          string    `json:"path"`
	Description   string    `json:"description"`
	OriginalName  string    `json:"original_name"`
	MimeType      string    `json:"mime_type"`
	Size          int64     `json:"size"`
	UploadedAt    time.Time `json:"uploaded_at"`
	DownloadCount int64     `json:"download_count"`
}

type BatchDownloadFile struct {
	FileID       string
	OriginalName string
	DiskPath     string
	ZipPath      string
}

type FolderDownload struct {
	FolderID   string
	FolderName string
	Items      []BatchDownloadFile
}

func NewPublicDownloadService(repository *repository.PublicDownloadRepository, storageService *storage.Service) *PublicDownloadService {
	return &PublicDownloadService{
		repository: repository,
		storage:    storageService,
	}
}

func (s *PublicDownloadService) PrepareDownload(ctx context.Context, fileID string) (*DownloadableFile, error) {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return nil, ErrDownloadFileNotFound
	}

	file, err := s.repository.FindActiveFileByID(ctx, fileID)
	if err != nil {
		return nil, fmt.Errorf("find downloadable file: %w", err)
	}
	if file == nil {
		return nil, ErrDownloadFileNotFound
	}

	opened, err := s.storage.OpenManagedFile(file.DiskPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrDownloadFileUnavailable
		}
		return nil, fmt.Errorf("open downloadable file: %w", err)
	}

	return &DownloadableFile{
		FileID:       file.ID,
		OriginalName: file.OriginalName,
		MimeType:     file.MimeType,
		Size:         opened.Info.Size(),
		ModTime:      opened.Info.ModTime(),
		Content:      opened.File,
	}, nil
}

func (s *PublicDownloadService) GetFileDetail(ctx context.Context, fileID string) (*PublicFileDetail, error) {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return nil, ErrDownloadFileNotFound
	}
	file, err := s.repository.FindActiveFileByID(ctx, fileID)
	if err != nil {
		return nil, fmt.Errorf("find public file detail: %w", err)
	}
	if file == nil {
		return nil, ErrDownloadFileNotFound
	}

	fullPath, err := s.buildFilePath(ctx, file)
	if err != nil {
		return nil, fmt.Errorf("build public file path: %w", err)
	}

	return &PublicFileDetail{
		ID:            file.ID,
		Title:         file.Title,
		Extension:     file.Extension,
		FolderID:      strings.TrimSpace(optionalString(file.FolderID)),
		Path:          fullPath,
		Description:   file.Description,
		OriginalName:  file.OriginalName,
		MimeType:      file.MimeType,
		Size:          file.Size,
		UploadedAt:    file.CreatedAt,
		DownloadCount: file.DownloadCount,
	}, nil
}

func (s *PublicDownloadService) buildFilePath(ctx context.Context, file *model.File) (string, error) {
	if file.FolderID == nil || strings.TrimSpace(*file.FolderID) == "" {
		return "主页根目录", nil
	}

	folderIDs := make([]string, 0, 8)
	seen := make(map[string]struct{}, 8)
	currentID := strings.TrimSpace(*file.FolderID)

	for currentID != "" {
		if _, ok := seen[currentID]; ok {
			break
		}
		seen[currentID] = struct{}{}
		folderIDs = append(folderIDs, currentID)

		folders, err := s.repository.ListActiveFoldersByIDs(ctx, []string{currentID})
		if err != nil {
			return "", err
		}
		if len(folders) == 0 || folders[0].ParentID == nil {
			break
		}
		currentID = strings.TrimSpace(*folders[0].ParentID)
	}

	folders, err := s.repository.ListActiveFoldersByIDs(ctx, folderIDs)
	if err != nil {
		return "", err
	}

	byID := make(map[string]repository.ActiveFolderNode, len(folders))
	for _, folder := range folders {
		byID[folder.ID] = folder
	}

	segments := make([]string, 0, len(folderIDs)+1)
	currentID = strings.TrimSpace(*file.FolderID)
	for currentID != "" {
		folder, ok := byID[currentID]
		if !ok {
			break
		}
		segments = append([]string{folder.Name}, segments...)
		if folder.ParentID == nil {
			break
		}
		currentID = strings.TrimSpace(*folder.ParentID)
	}

	if len(segments) == 0 {
		return "主页根目录", nil
	}
	return strings.Join(segments, " / "), nil
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (s *PublicDownloadService) PrepareBatchDownload(ctx context.Context, fileIDs []string) ([]BatchDownloadFile, error) {
	normalized := normalizeBatchFileIDs(fileIDs)
	if len(normalized) == 0 {
		return nil, ErrBatchDownloadInvalid
	}

	files, err := s.repository.ListActiveFilesByIDs(ctx, normalized)
	if err != nil {
		return nil, fmt.Errorf("list batch download files: %w", err)
	}
	if len(files) != len(normalized) {
		return nil, ErrDownloadFileNotFound
	}

	items := make([]BatchDownloadFile, 0, len(files))
	for _, file := range files {
		opened, err := s.storage.OpenManagedFile(file.DiskPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, ErrDownloadFileUnavailable
			}
			return nil, fmt.Errorf("validate batch download file: %w", err)
		}
		opened.File.Close()

		items = append(items, BatchDownloadFile{
			FileID:       file.ID,
			OriginalName: file.OriginalName,
			DiskPath:     file.DiskPath,
			ZipPath:      file.OriginalName,
		})
	}
	return items, nil
}

func (s *PublicDownloadService) PrepareResourceBatchDownload(ctx context.Context, fileIDs []string, folderIDs []string) ([]BatchDownloadFile, error) {
	normalizedFiles := normalizeBatchFileIDs(fileIDs)
	normalizedFolders := normalizeBatchFileIDs(folderIDs)
	if len(normalizedFiles) == 0 && len(normalizedFolders) == 0 {
		return nil, ErrBatchDownloadInvalid
	}

	items := make([]BatchDownloadFile, 0, len(normalizedFiles))
	if len(normalizedFiles) > 0 {
		files, err := s.PrepareBatchDownload(ctx, normalizedFiles)
		if err != nil {
			return nil, err
		}
		items = append(items, files...)
	}

	for _, folderID := range normalizedFolders {
		folderDownload, err := s.PrepareFolderDownload(ctx, folderID)
		if err != nil {
			return nil, err
		}
		items = append(items, folderDownload.Items...)
	}

	if len(items) == 0 {
		return nil, ErrBatchDownloadInvalid
	}

	return items, nil
}

func (s *PublicDownloadService) PrepareFolderDownload(ctx context.Context, folderID string) (*FolderDownload, error) {
	folderID = strings.TrimSpace(folderID)
	if folderID == "" {
		return nil, ErrDownloadFolderNotFound
	}

	root, err := s.repository.FindActiveFolderByID(ctx, folderID)
	if err != nil {
		return nil, fmt.Errorf("find downloadable folder: %w", err)
	}
	if root == nil {
		return nil, ErrDownloadFolderNotFound
	}

	parentByFolder := map[string]string{root.ID: ""}
	nameByFolder := map[string]string{root.ID: root.Name}
	allFolderIDs := []string{root.ID}
	currentLevel := []string{root.ID}

	for len(currentLevel) > 0 {
		children, err := s.repository.ListActiveFoldersByParentIDs(ctx, currentLevel)
		if err != nil {
			return nil, fmt.Errorf("list descendant folders: %w", err)
		}

		nextLevel := make([]string, 0, len(children))
		for _, child := range children {
			nameByFolder[child.ID] = child.Name
			if child.ParentID != nil {
				parentByFolder[child.ID] = *child.ParentID
			}
			allFolderIDs = append(allFolderIDs, child.ID)
			nextLevel = append(nextLevel, child.ID)
		}
		currentLevel = nextLevel
	}

	files, err := s.repository.ListActiveFilesByFolderIDs(ctx, allFolderIDs)
	if err != nil {
		return nil, fmt.Errorf("list folder download files: %w", err)
	}

	items := make([]BatchDownloadFile, 0, len(files))
	for _, file := range files {
		opened, err := s.storage.OpenManagedFile(file.DiskPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, ErrDownloadFileUnavailable
			}
			return nil, fmt.Errorf("validate folder download file: %w", err)
		}
		opened.File.Close()

		items = append(items, BatchDownloadFile{
			FileID:       file.ID,
			OriginalName: file.OriginalName,
			DiskPath:     file.DiskPath,
			ZipPath:      buildFolderZipPath(file.OriginalName, file.FolderID, parentByFolder, nameByFolder),
		})
	}

	return &FolderDownload{
		FolderID:   root.ID,
		FolderName: root.Name,
		Items:      items,
	}, nil
}

func (s *PublicDownloadService) RecordDownload(ctx context.Context, fileID string) error {
	return s.repository.IncrementDownloadCount(ctx, fileID)
}

func (s *PublicDownloadService) RecordBatchDownload(ctx context.Context, fileIDs []string) error {
	normalized := normalizeBatchFileIDs(fileIDs)
	if len(normalized) == 0 {
		return nil
	}
	return s.repository.IncrementDownloadCounts(ctx, normalized)
}

func normalizeBatchFileIDs(fileIDs []string) []string {
	normalized := make([]string, 0, len(fileIDs))
	for _, fileID := range fileIDs {
		fileID = strings.TrimSpace(fileID)
		if fileID == "" || slices.Contains(normalized, fileID) {
			continue
		}
		normalized = append(normalized, fileID)
	}
	return normalized
}

func buildFolderZipPath(fileName string, folderID *string, parentByFolder map[string]string, nameByFolder map[string]string) string {
	parts := []string{fileName}
	if folderID == nil {
		return fileName
	}

	currentID := *folderID
	for currentID != "" {
		name := nameByFolder[currentID]
		if name != "" {
			parts = append([]string{name}, parts...)
		}
		currentID = parentByFolder[currentID]
	}

	return strings.Join(parts, "/")
}
