package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"openshare/backend/internal/model"
)

type UploadRepository struct {
	db *gorm.DB
}

func NewUploadRepository(db *gorm.DB) *UploadRepository {
	return &UploadRepository{db: db}
}

func (r *UploadRepository) DB() *gorm.DB {
	return r.db
}

func (r *UploadRepository) FolderExists(ctx context.Context, folderID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.Folder{}).
		Where("id = ? AND status = ?", folderID, model.ResourceStatusActive).
		Count(&count).
		Error
	if err != nil {
		return false, fmt.Errorf("check folder existence: %w", err)
	}

	return count > 0, nil
}

func (r *UploadRepository) FindActiveFolderByID(ctx context.Context, folderID string) (*model.Folder, error) {
	var folder model.Folder
	err := r.db.WithContext(ctx).
		Where("id = ? AND status = ?", folderID, model.ResourceStatusActive).
		Take(&folder).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find active folder: %w", err)
	}
	return &folder, nil
}

func (r *UploadRepository) CreateUpload(ctx context.Context, submission *model.Submission, file *model.File) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(submission).Error; err != nil {
			return fmt.Errorf("create submission: %w", err)
		}
		if err := tx.Create(file).Error; err != nil {
			return fmt.Errorf("create file metadata: %w", err)
		}
		return nil
	})
}

func (r *UploadRepository) CreateUploadBatch(ctx context.Context, submissions []model.Submission, files []model.File) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i := range submissions {
			if err := tx.Create(&submissions[i]).Error; err != nil {
				return fmt.Errorf("create submission: %w", err)
			}
		}
		for i := range files {
			if err := tx.Create(&files[i]).Error; err != nil {
				return fmt.Errorf("create file metadata: %w", err)
			}
		}
		return nil
	})
}
