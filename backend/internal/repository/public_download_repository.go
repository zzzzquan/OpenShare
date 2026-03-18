package repository

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"gorm.io/gorm"

	"openshare/backend/internal/model"
	"openshare/backend/pkg/identity"
)

type PublicDownloadRepository struct {
	db *gorm.DB
}

type ActiveFolderNode struct {
	ID       string
	ParentID *string
	Name     string
}

func NewPublicDownloadRepository(db *gorm.DB) *PublicDownloadRepository {
	return &PublicDownloadRepository{db: db}
}

func (r *PublicDownloadRepository) FindActiveFileByID(ctx context.Context, fileID string) (*model.File, error) {
	var file model.File
	err := r.db.WithContext(ctx).
		Where("id = ? AND status = ?", fileID, model.ResourceStatusActive).
		Take(&file).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find active file by id: %w", err)
	}

	return &file, nil
}

func (r *PublicDownloadRepository) FindActiveFolderByID(ctx context.Context, folderID string) (*model.Folder, error) {
	var folder model.Folder
	err := r.db.WithContext(ctx).
		Where("id = ? AND status = ?", folderID, model.ResourceStatusActive).
		Take(&folder).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find active folder by id: %w", err)
	}

	return &folder, nil
}

func (r *PublicDownloadRepository) IncrementDownloadCount(ctx context.Context, fileID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.File{}).
			Where("id = ?", fileID).
			UpdateColumn("download_count", gorm.Expr("download_count + 1")).
			Error; err != nil {
			return err
		}

		eventID, err := identity.NewID()
		if err != nil {
			return fmt.Errorf("generate download event id: %w", err)
		}

		return tx.Create(&model.DownloadEvent{
			ID:        eventID,
			FileID:    fileID,
			CreatedAt: time.Now().UTC(),
		}).Error
	})
}

func (r *PublicDownloadRepository) IncrementDownloadCounts(ctx context.Context, fileIDs []string) error {
	if len(fileIDs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.File{}).
			Where("id IN ?", fileIDs).
			UpdateColumn("download_count", gorm.Expr("download_count + 1")).
			Error; err != nil {
			return err
		}

		now := time.Now().UTC()
		events := make([]model.DownloadEvent, 0, len(fileIDs))
		for _, fileID := range fileIDs {
			eventID, err := identity.NewID()
			if err != nil {
				return fmt.Errorf("generate download event id: %w", err)
			}
			events = append(events, model.DownloadEvent{
				ID:        eventID,
				FileID:    fileID,
				CreatedAt: now,
			})
		}

		return tx.Create(&events).Error
	})
}

func (r *PublicDownloadRepository) ListActiveFilesByIDs(ctx context.Context, fileIDs []string) ([]model.File, error) {
	if len(fileIDs) == 0 {
		return nil, nil
	}

	var files []model.File
	err := r.db.WithContext(ctx).
		Where("id IN ? AND status = ?", fileIDs, model.ResourceStatusActive).
		Find(&files).Error
	if err != nil {
		return nil, fmt.Errorf("list active files by ids: %w", err)
	}

	byID := make(map[string]model.File, len(files))
	for _, file := range files {
		byID[file.ID] = file
	}

	ordered := make([]model.File, 0, len(fileIDs))
	for _, fileID := range fileIDs {
		if file, ok := byID[strings.TrimSpace(fileID)]; ok {
			ordered = append(ordered, file)
		}
	}
	return ordered, nil
}

func (r *PublicDownloadRepository) ListActiveFoldersByParentIDs(ctx context.Context, parentIDs []string) ([]ActiveFolderNode, error) {
	if len(parentIDs) == 0 {
		return nil, nil
	}

	var rows []ActiveFolderNode
	err := r.db.WithContext(ctx).
		Model(&model.Folder{}).
		Select("id, parent_id, name").
		Where("status = ?", model.ResourceStatusActive).
		Where("parent_id IN ?", parentIDs).
		Order("name ASC").
		Find(&rows).
		Error
	if err != nil {
		return nil, fmt.Errorf("list active folders by parent ids: %w", err)
	}

	return rows, nil
}

func (r *PublicDownloadRepository) ListActiveFilesByFolderIDs(ctx context.Context, folderIDs []string) ([]model.File, error) {
	if len(folderIDs) == 0 {
		return nil, nil
	}

	var files []model.File
	err := r.db.WithContext(ctx).
		Where("status = ?", model.ResourceStatusActive).
		Where("folder_id IN ?", folderIDs).
		Order("created_at ASC").
		Find(&files).
		Error
	if err != nil {
		return nil, fmt.Errorf("list active files by folder ids: %w", err)
	}

	slices.SortFunc(files, func(a, b model.File) int {
		if a.FolderID != nil && b.FolderID != nil && *a.FolderID != *b.FolderID {
			return strings.Compare(*a.FolderID, *b.FolderID)
		}
		return strings.Compare(a.OriginalName, b.OriginalName)
	})

	return files, nil
}

func (r *PublicDownloadRepository) ListTagsByFileIDs(ctx context.Context, fileIDs []string) (map[string][]string, error) {
	if len(fileIDs) == 0 {
		return map[string][]string{}, nil
	}

	type tagRow struct {
		FileID  string
		TagName string
	}

	var rows []tagRow
	err := r.db.WithContext(ctx).
		Table("file_tags").
		Select("file_tags.file_id AS file_id, tags.name AS tag_name").
		Joins("JOIN tags ON tags.id = file_tags.tag_id").
		Where("file_tags.file_id IN ?", fileIDs).
		Where("tags.deleted_at IS NULL").
		Order("tags.name ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list file tags by ids: %w", err)
	}

	result := make(map[string][]string, len(fileIDs))
	for _, row := range rows {
		result[row.FileID] = append(result[row.FileID], row.TagName)
	}
	return result, nil
}
