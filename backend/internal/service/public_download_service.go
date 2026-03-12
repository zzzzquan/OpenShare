package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"openshare/backend/internal/repository"
	"openshare/backend/internal/storage"
)

var (
	ErrDownloadFileNotFound    = errors.New("download file not found")
	ErrDownloadFileUnavailable = errors.New("download file unavailable")
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

func (s *PublicDownloadService) RecordDownloadAsync(fileID string) {
	go func() {
		if err := s.repository.IncrementDownloadCount(context.Background(), fileID); err != nil {
			log.Printf("increment download count for file %s: %v", fileID, err)
		}
	}()
}
