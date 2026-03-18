package storage

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"openshare/backend/internal/config"
	"openshare/backend/pkg/identity"
)

var ErrFileTooLarge = fmt.Errorf("file too large")
var ErrManagedDirectoryConflict = fmt.Errorf("managed directory conflict")

const maxStoredNameAttempts = 5

type Service struct {
	stagingDir string
	trashDir   string
}

type StagedFile struct {
	StoredName string
	DiskPath   string
	Size       int64
}

type OpenedFile struct {
	File *os.File
	Info os.FileInfo
}

type ScannedEntry struct {
	AbsolutePath string
	RelativePath string
	Name         string
	IsDir        bool
	Size         int64
	Extension    string
	MimeType     string
}

func NewService(cfg config.StorageConfig) *Service {
	return &Service{
		stagingDir: filepath.Join(cfg.Root, cfg.Staging),
		trashDir:   filepath.Join(cfg.Root, cfg.Trash),
	}
}

func (s *Service) SaveToStaging(reader io.Reader, extension string, maxBytes int64) (*StagedFile, error) {
	tempFile, err := os.CreateTemp(s.stagingDir, ".openshare-upload-*")
	if err != nil {
		return nil, fmt.Errorf("create staging temp file: %w", err)
	}

	tempPath := tempFile.Name()
	cleanup := func() {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
	}

	written, err := io.Copy(tempFile, io.LimitReader(reader, maxBytes+1))
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("write staging file: %w", err)
	}
	if written == 0 {
		cleanup()
		return nil, fmt.Errorf("write staging file: empty file")
	}
	if written > maxBytes {
		cleanup()
		return nil, ErrFileTooLarge
	}
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return nil, fmt.Errorf("close staging file: %w", err)
	}

	finalPath, storedName, err := s.claimStoredPath(tempPath, extension)
	if err != nil {
		_ = os.Remove(tempPath)
		return nil, err
	}

	return &StagedFile{
		StoredName: storedName,
		DiskPath:   finalPath,
		Size:       written,
	}, nil
}

func (s *Service) DeleteStagedFile(diskPath string) error {
	if strings.TrimSpace(diskPath) == "" {
		return nil
	}
	if !strings.HasPrefix(diskPath, s.stagingDir+string(os.PathSeparator)) && diskPath != s.stagingDir {
		return fmt.Errorf("refuse to delete file outside staging directory")
	}
	if err := os.Remove(diskPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete staged file: %w", err)
	}
	return nil
}

func (s *Service) StagedFileExists(diskPath string) (bool, error) {
	if !s.isWithinDir(diskPath, s.stagingDir) {
		return false, fmt.Errorf("file is outside staging directory")
	}

	info, err := os.Stat(diskPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("stat staged file: %w", err)
	}
	if info.IsDir() {
		return false, fmt.Errorf("staged path is a directory")
	}

	return true, nil
}

// MoveStagedFileToFolder moves a file from staging into targetDir using
// originalName as the preferred filename. When a name conflict exists a
// numeric suffix (_1, _2, …) is appended before the extension.
// Returns the final absolute path and the chosen filename.
func (s *Service) MoveStagedFileToFolder(stagedPath, targetDir, originalName string) (finalPath, finalName string, err error) {
	if !s.isWithinDir(stagedPath, s.stagingDir) {
		return "", "", fmt.Errorf("file is outside staging directory")
	}
	targetDir = filepath.Clean(targetDir)
	if targetDir == "" {
		return "", "", fmt.Errorf("target directory is empty")
	}
	info, statErr := os.Stat(targetDir)
	if statErr != nil {
		return "", "", fmt.Errorf("stat target directory: %w", statErr)
	}
	if !info.IsDir() {
		return "", "", fmt.Errorf("target path is not a directory")
	}

	// Sanitize: use only the base component to prevent path traversal.
	originalName = filepath.Base(originalName)
	ext := filepath.Ext(originalName)
	base := strings.TrimSuffix(originalName, ext)

	const maxAttempts = 100
	for i := 0; i < maxAttempts; i++ {
		candidate := originalName
		if i > 0 {
			candidate = fmt.Sprintf("%s_%d%s", base, i, ext)
		}
		destPath := filepath.Join(targetDir, candidate)
		if !s.isWithinDir(destPath, targetDir) {
			return "", "", fmt.Errorf("target path traversal detected")
		}
		if _, err := os.Stat(destPath); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", "", fmt.Errorf("inspect target: %w", err)
		}
		if err := os.Rename(stagedPath, destPath); err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return "", "", fmt.Errorf("move staged file to folder: %w", err)
		}
		return destPath, candidate, nil
	}
	return "", "", fmt.Errorf("all candidate filenames are taken in target directory")
}

// MoveFileBackToStaging moves an approved file back to staging for rollback.
func (s *Service) MoveFileBackToStaging(diskPath, storedName string) (string, error) {
	if strings.TrimSpace(storedName) == "" {
		return "", fmt.Errorf("stored name must not be empty")
	}
	finalPath := filepath.Join(s.stagingDir, storedName)
	if err := os.Rename(diskPath, finalPath); err != nil {
		return "", fmt.Errorf("move file back to staging: %w", err)
	}
	return finalPath, nil
}

// OpenManagedFile opens any file tracked by the system (imported or uploaded).
func (s *Service) OpenManagedFile(diskPath string) (*OpenedFile, error) {
	diskPath = filepath.Clean(strings.TrimSpace(diskPath))
	if diskPath == "" {
		return nil, fmt.Errorf("disk path must not be empty")
	}

	file, err := os.Open(diskPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("open managed file: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("stat managed file: %w", err)
	}
	if info.IsDir() {
		_ = file.Close()
		return nil, fmt.Errorf("managed file path is a directory")
	}

	return &OpenedFile{File: file, Info: info}, nil
}

func (s *Service) MoveManagedFileToTrash(diskPath string) (string, error) {
	diskPath = filepath.Clean(strings.TrimSpace(diskPath))
	if diskPath == "" {
		return "", nil
	}

	info, err := os.Stat(diskPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("stat managed file before trash move: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("managed file path is a directory")
	}

	base := filepath.Base(diskPath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	for attempt := 0; attempt < 100; attempt++ {
		candidate := base
		if attempt > 0 {
			candidate = fmt.Sprintf("%s_%d%s", name, attempt, ext)
		}
		target := filepath.Join(s.trashDir, candidate)
		if _, err := os.Stat(target); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("inspect trash target: %w", err)
		}

		if err := os.Rename(diskPath, target); err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return "", fmt.Errorf("move managed file to trash: %w", err)
		}
		return target, nil
	}

	return "", fmt.Errorf("move managed file to trash: unable to allocate target path")
}

func (s *Service) MoveManagedDirectoryToTrash(dirPath string) (string, error) {
	dirPath = filepath.Clean(strings.TrimSpace(dirPath))
	if dirPath == "" {
		return "", nil
	}

	info, err := os.Stat(dirPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("stat managed directory before trash move: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("managed directory path is not a directory")
	}

	base := filepath.Base(dirPath)
	for attempt := 0; attempt < 100; attempt++ {
		candidate := base
		if attempt > 0 {
			candidate = fmt.Sprintf("%s_%d", base, attempt)
		}
		target := filepath.Join(s.trashDir, candidate)
		if _, err := os.Stat(target); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("inspect trash target: %w", err)
		}

		if err := os.Rename(dirPath, target); err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return "", fmt.Errorf("move managed directory to trash: %w", err)
		}
		return target, nil
	}

	return "", fmt.Errorf("move managed directory to trash: unable to allocate target path")
}

func (s *Service) RenameManagedDirectory(dirPath, newName string) (string, error) {
	dirPath = filepath.Clean(strings.TrimSpace(dirPath))
	newName = strings.TrimSpace(newName)
	if dirPath == "" || newName == "" {
		return "", fmt.Errorf("directory path and new name must not be empty")
	}

	info, err := os.Stat(dirPath)
	if err != nil {
		return "", fmt.Errorf("stat managed directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("managed path is not a directory")
	}

	parentDir := filepath.Dir(dirPath)
	targetPath := filepath.Join(parentDir, filepath.Base(newName))
	if targetPath == dirPath {
		return dirPath, nil
	}
	if _, err := os.Stat(targetPath); err == nil {
		return "", ErrManagedDirectoryConflict
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("inspect managed directory target: %w", err)
	}

	if err := os.Rename(dirPath, targetPath); err != nil {
		if errors.Is(err, os.ErrExist) {
			return "", ErrManagedDirectoryConflict
		}
		return "", fmt.Errorf("rename managed directory: %w", err)
	}
	return targetPath, nil
}

func (s *Service) EnsureManagedDirectory(dirPath string) error {
	dirPath = filepath.Clean(strings.TrimSpace(dirPath))
	if dirPath == "" {
		return fmt.Errorf("directory path must not be empty")
	}
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return fmt.Errorf("ensure managed directory: %w", err)
	}
	return nil
}

func (s *Service) ScanDirectory(rootPath string) ([]ScannedEntry, error) {
	rootPath = filepath.Clean(strings.TrimSpace(rootPath))
	if rootPath == "" {
		return nil, fmt.Errorf("root path must not be empty")
	}

	info, err := os.Stat(rootPath)
	if err != nil {
		return nil, fmt.Errorf("stat import root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("import root is not a directory")
	}

	entries := make([]ScannedEntry, 0, 32)
	err = filepath.WalkDir(rootPath, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == rootPath {
			return nil
		}
		if shouldIgnoreImportEntry(d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		relativePath, err := filepath.Rel(rootPath, path)
		if err != nil {
			return fmt.Errorf("resolve relative path: %w", err)
		}

		entry := ScannedEntry{
			AbsolutePath: path,
			RelativePath: filepath.ToSlash(relativePath),
			Name:         d.Name(),
			IsDir:        d.IsDir(),
		}

		if !d.IsDir() {
			fileInfo, err := d.Info()
			if err != nil {
				return fmt.Errorf("read file info: %w", err)
			}
			entry.Size = fileInfo.Size()
			entry.Extension = strings.ToLower(filepath.Ext(d.Name()))
			entry.MimeType = mime.TypeByExtension(entry.Extension)
			if entry.MimeType == "" {
				entry.MimeType = "application/octet-stream"
			}
		}

		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan import root: %w", err)
	}

	return entries, nil
}

func shouldIgnoreImportEntry(name string) bool {
	return name == ".DS_Store"
}

func (s *Service) claimStoredPath(tempPath, extension string) (string, string, error) {
	for i := 0; i < maxStoredNameAttempts; i++ {
		storedName, err := generateStoredName(extension)
		if err != nil {
			return "", "", fmt.Errorf("generate stored name: %w", err)
		}

		finalPath := filepath.Join(s.stagingDir, storedName)
		if _, err := os.Stat(finalPath); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", "", fmt.Errorf("inspect staging target: %w", err)
		}

		if err := os.Rename(tempPath, finalPath); err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return "", "", fmt.Errorf("finalize staging file: %w", err)
		}

		return finalPath, storedName, nil
	}

	return "", "", fmt.Errorf("finalize staging file: stored name conflict")
}

func generateStoredName(extension string) (string, error) {
	fileID, err := identity.NewID()
	if err != nil {
		return "", err
	}

	extension = strings.ToLower(strings.TrimSpace(extension))
	if extension == "" {
		return fileID, nil
	}

	return fileID + extension, nil
}

func (s *Service) isWithinDir(path, dir string) bool {
	path = filepath.Clean(strings.TrimSpace(path))
	dir = filepath.Clean(strings.TrimSpace(dir))
	if path == "" || dir == "" {
		return false
	}

	if path == dir {
		return true
	}

	return strings.HasPrefix(path, dir+string(os.PathSeparator))
}
