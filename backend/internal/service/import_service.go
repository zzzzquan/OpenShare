package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"openshare/backend/internal/model"
	"openshare/backend/internal/repository"
	"openshare/backend/internal/storage"
	"openshare/backend/pkg/identity"
)

var (
	ErrInvalidImportPath  = errors.New("invalid import path")
	ErrFolderTreeNotFound = errors.New("folder not found")
)

type ImportService struct {
	repository *repository.ImportRepository
	storage    *storage.Service
	nowFunc    func() time.Time
}

type LocalImportInput struct {
	RootPath   string
	AdminID    string
	OperatorIP string
}

type LocalImportResult struct {
	RootPath        string   `json:"root_path"`
	ImportedFolders int      `json:"imported_folders"`
	ImportedFiles   int      `json:"imported_files"`
	SkippedFolders  int      `json:"skipped_folders"`
	SkippedFiles    int      `json:"skipped_files"`
	Conflicts       []string `json:"conflicts"`
}

type FolderTreeNode struct {
	ID      string               `json:"id"`
	Name    string               `json:"name"`
	Status  model.ResourceStatus `json:"status"`
	Tags    []string             `json:"tags"`
	Folders []FolderTreeNode     `json:"folders"`
	Files   []FolderTreeFile     `json:"files"`
}

type FolderTreeFile struct {
	ID            string               `json:"id"`
	Title         string               `json:"title"`
	OriginalName  string               `json:"original_name"`
	Status        model.ResourceStatus `json:"status"`
	Size          int64                `json:"size"`
	DownloadCount int64                `json:"download_count"`
}

func NewImportService(repository *repository.ImportRepository, storageService *storage.Service) *ImportService {
	return &ImportService{
		repository: repository,
		storage:    storageService,
		nowFunc:    func() time.Time { return time.Now().UTC() },
	}
}

func (s *ImportService) ImportLocalDirectory(ctx context.Context, input LocalImportInput) (*LocalImportResult, error) {
	rootPath := filepath.Clean(strings.TrimSpace(input.RootPath))
	if rootPath == "" || !filepath.IsAbs(rootPath) {
		return nil, ErrInvalidImportPath
	}

	entries, err := s.storage.ScanDirectory(rootPath)
	if err != nil {
		return nil, fmt.Errorf("scan local directory: %w", err)
	}

	now := s.nowFunc()
	result := &LocalImportResult{
		RootPath:  rootPath,
		Conflicts: make([]string, 0),
	}

	rootFolder, created, conflict, err := s.ensureFolder(ctx, nil, filepath.Base(rootPath), rootPath, now)
	if err != nil {
		return nil, err
	}
	if conflict != "" {
		result.Conflicts = append(result.Conflicts, conflict)
		return result, nil
	}
	if created {
		result.ImportedFolders++
	} else {
		result.SkippedFolders++
	}

	folderMap := map[string]*model.Folder{
		".": rootFolder,
		"":  rootFolder,
	}
	skippedPrefixes := make(map[string]struct{})

	for _, entry := range entries {
		if shouldSkipEntry(entry.RelativePath, skippedPrefixes) {
			if entry.IsDir {
				result.SkippedFolders++
			} else {
				result.SkippedFiles++
			}
			continue
		}

		parentRelative := filepath.ToSlash(filepath.Dir(entry.RelativePath))
		parentFolder, ok := folderMap[parentRelative]
		if parentRelative == "." || parentRelative == "" {
			parentFolder = rootFolder
			ok = true
		}
		if !ok {
			if entry.IsDir {
				skippedPrefixes[entry.RelativePath] = struct{}{}
				result.SkippedFolders++
			} else {
				result.SkippedFiles++
			}
			continue
		}

		if entry.IsDir {
			folder, created, conflict, err := s.ensureFolder(ctx, &parentFolder.ID, entry.Name, entry.AbsolutePath, now)
			if err != nil {
				return nil, err
			}
			if conflict != "" {
				result.Conflicts = append(result.Conflicts, conflict)
				skippedPrefixes[entry.RelativePath] = struct{}{}
				result.SkippedFolders++
				continue
			}
			folderMap[entry.RelativePath] = folder
			if created {
				result.ImportedFolders++
			} else {
				result.SkippedFolders++
			}
			continue
		}

		created, conflict, err := s.ensureFile(ctx, &parentFolder.ID, entry, now)
		if err != nil {
			return nil, err
		}
		if conflict != "" {
			result.Conflicts = append(result.Conflicts, conflict)
			result.SkippedFiles++
			continue
		}
		if created {
			result.ImportedFiles++
		} else {
			result.SkippedFiles++
		}
	}

	detail, _ := json.Marshal(result)
	_ = s.repository.LogOperation(ctx, input.AdminID, "local_import", "folder", rootFolder.ID, string(detail), input.OperatorIP, now)

	return result, nil
}

func (s *ImportService) GetFolderTree(ctx context.Context) ([]FolderTreeNode, error) {
	folders, err := s.repository.ListFolders(ctx)
	if err != nil {
		return nil, fmt.Errorf("list folders: %w", err)
	}
	files, err := s.repository.ListFiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}
	tagRows, err := s.repository.ListFolderTagRows(ctx)
	if err != nil {
		return nil, fmt.Errorf("list folder tags: %w", err)
	}

	nodes := make(map[string]*FolderTreeNode, len(folders))
	childrenByParent := make(map[string][]string)
	rootIDs := make([]string, 0)
	for _, folder := range folders {
		nodes[folder.ID] = &FolderTreeNode{
			ID:      folder.ID,
			Name:    folder.Name,
			Status:  folder.Status,
			Tags:    []string{},
			Folders: []FolderTreeNode{},
			Files:   []FolderTreeFile{},
		}
	}
	for _, folder := range folders {
		if folder.ParentID == nil {
			rootIDs = append(rootIDs, folder.ID)
			continue
		}
		childrenByParent[*folder.ParentID] = append(childrenByParent[*folder.ParentID], folder.ID)
	}
	for _, row := range tagRows {
		if node, ok := nodes[row.FolderID]; ok {
			node.Tags = append(node.Tags, row.TagName)
		}
	}
	for _, file := range files {
		if file.FolderID == nil {
			continue
		}
		parent := nodes[*file.FolderID]
		if parent == nil {
			continue
		}
		parent.Files = append(parent.Files, FolderTreeFile{
			ID:            file.ID,
			Title:         file.Title,
			OriginalName:  file.OriginalName,
			Status:        file.Status,
			Size:          file.Size,
			DownloadCount: file.DownloadCount,
		})
	}

	var build func(string) FolderTreeNode
	build = func(folderID string) FolderTreeNode {
		node := nodes[folderID]
		result := *node
		result.Folders = make([]FolderTreeNode, 0, len(childrenByParent[folderID]))
		for _, childID := range childrenByParent[folderID] {
			result.Folders = append(result.Folders, build(childID))
		}
		return result
	}

	result := make([]FolderTreeNode, 0, len(rootIDs))
	for _, rootID := range rootIDs {
		result = append(result, build(rootID))
	}
	return result, nil
}

func (s *ImportService) BindFolderTags(ctx context.Context, folderID string, tagNames []string, adminID, operatorIP string) error {
	folder, err := s.repository.FindFolderByID(ctx, strings.TrimSpace(folderID))
	if err != nil {
		return fmt.Errorf("find folder: %w", err)
	}
	if folder == nil {
		return ErrFolderTreeNotFound
	}

	names, err := normalizeTags(tagNames, 20, 32)
	if err != nil {
		return ErrInvalidUploadInput
	}

	existing, err := s.repository.FindTagsByNames(ctx, names)
	if err != nil {
		return fmt.Errorf("find tags: %w", err)
	}
	tagByNormalized := make(map[string]model.Tag, len(existing))
	for _, tag := range existing {
		tagByNormalized[tag.NameNormalized] = tag
	}

	tagIDs := make([]string, 0, len(names))
	for _, name := range names {
		key := strings.ToLower(strings.TrimSpace(name))
		tag, ok := tagByNormalized[key]
		if !ok {
			tagID, err := identity.NewID()
			if err != nil {
				return fmt.Errorf("generate tag id: %w", err)
			}
			tag = model.Tag{
				ID:             tagID,
				Name:           name,
				NameNormalized: key,
				CreatedAt:      s.nowFunc(),
				UpdatedAt:      s.nowFunc(),
			}
			if err := s.repository.CreateTag(ctx, &tag); err != nil {
				return fmt.Errorf("create tag: %w", err)
			}
			tagByNormalized[key] = tag
		}
		tagIDs = append(tagIDs, tag.ID)
	}

	if err := s.repository.ReplaceFolderTags(ctx, folder.ID, tagIDs); err != nil {
		return fmt.Errorf("replace folder tags: %w", err)
	}

	_ = s.repository.LogOperation(ctx, adminID, "folder_tags_updated", "folder", folder.ID, strings.Join(names, ","), operatorIP, s.nowFunc())
	return nil
}

func (s *ImportService) ensureFolder(ctx context.Context, parentID *string, name string, sourcePath string, now time.Time) (*model.Folder, bool, string, error) {
	existing, err := s.repository.FindFolderBySourcePath(ctx, sourcePath)
	if err != nil {
		return nil, false, "", fmt.Errorf("find imported folder: %w", err)
	}
	if existing != nil {
		return existing, false, "", nil
	}

	conflict, err := s.repository.FolderNameExists(ctx, parentID, name)
	if err != nil {
		return nil, false, "", err
	}
	if conflict {
		return nil, false, fmt.Sprintf("folder name conflict: %s", sourcePath), nil
	}

	id, err := identity.NewID()
	if err != nil {
		return nil, false, "", fmt.Errorf("generate folder id: %w", err)
	}
	sourcePathCopy := sourcePath
	folder := &model.Folder{
		ID:         id,
		ParentID:   parentID,
		SourcePath: &sourcePathCopy,
		Name:       name,
		Status:     model.ResourceStatusActive,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.repository.CreateFolder(ctx, folder); err != nil {
		return nil, false, "", fmt.Errorf("create folder: %w", err)
	}
	return folder, true, "", nil
}

func (s *ImportService) ensureFile(ctx context.Context, folderID *string, entry storage.ScannedEntry, now time.Time) (bool, string, error) {
	existing, err := s.repository.FindFileBySourcePath(ctx, entry.AbsolutePath)
	if err != nil {
		return false, "", fmt.Errorf("find imported file: %w", err)
	}
	if existing != nil {
		return false, "", nil
	}

	conflict, err := s.repository.FileNameExists(ctx, folderID, entry.Name)
	if err != nil {
		return false, "", err
	}
	if conflict {
		return false, fmt.Sprintf("file name conflict: %s", entry.AbsolutePath), nil
	}

	id, err := identity.NewID()
	if err != nil {
		return false, "", fmt.Errorf("generate file id: %w", err)
	}
	sourcePathCopy := entry.AbsolutePath
	file := &model.File{
		ID:            id,
		FolderID:      folderID,
		SubmissionID:  nil,
		SourcePath:    &sourcePathCopy,
		Title:         strings.TrimSuffix(entry.Name, filepath.Ext(entry.Name)),
		OriginalName:  entry.Name,
		StoredName:    entry.Name,
		Extension:     entry.Extension,
		MimeType:      entry.MimeType,
		Size:          entry.Size,
		DiskPath:      entry.AbsolutePath,
		Status:        model.ResourceStatusActive,
		DownloadCount: 0,
		UploaderIP:    "",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.repository.CreateFile(ctx, file); err != nil {
		return false, "", fmt.Errorf("create imported file: %w", err)
	}
	return true, "", nil
}

func shouldSkipEntry(relativePath string, skippedPrefixes map[string]struct{}) bool {
	for prefix := range skippedPrefixes {
		if relativePath == prefix || strings.HasPrefix(relativePath, prefix+"/") {
			return true
		}
	}
	return false
}
