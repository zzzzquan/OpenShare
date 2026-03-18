package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"openshare/backend/internal/model"
)

type AnnouncementRepository struct {
	db *gorm.DB
}

func NewAnnouncementRepository(db *gorm.DB) *AnnouncementRepository {
	return &AnnouncementRepository{db: db}
}

func (r *AnnouncementRepository) ListPublic(ctx context.Context) ([]model.Announcement, error) {
	var items []model.Announcement
	err := r.db.WithContext(ctx).
		Preload("CreatedBy").
		Where("status = ? AND deleted_at IS NULL", model.AnnouncementStatusPublished).
		Order("is_pinned DESC, published_at DESC, created_at DESC").
		Find(&items).Error
	if err != nil {
		return nil, fmt.Errorf("list public announcements: %w", err)
	}
	return items, nil
}

func (r *AnnouncementRepository) ListAll(ctx context.Context) ([]model.Announcement, error) {
	var items []model.Announcement
	err := r.db.WithContext(ctx).
		Preload("CreatedBy").
		Where("deleted_at IS NULL").
		Order("is_pinned DESC, published_at DESC, created_at DESC").
		Find(&items).Error
	if err != nil {
		return nil, fmt.Errorf("list announcements: %w", err)
	}
	return items, nil
}

func (r *AnnouncementRepository) FindByID(ctx context.Context, id string) (*model.Announcement, error) {
	var item model.Announcement
	err := r.db.WithContext(ctx).Preload("CreatedBy").Where("id = ? AND deleted_at IS NULL", id).Take(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find announcement: %w", err)
	}
	return &item, nil
}

func (r *AnnouncementRepository) CreateWithLog(
	ctx context.Context,
	item *model.Announcement,
	operatorID string,
	operatorIP string,
	logID string,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(item).Error; err != nil {
			return fmt.Errorf("create announcement: %w", err)
		}
		return createOperationLogTx(tx, logID, operatorID, "announcement_created", "announcement", item.ID, item.Title, operatorIP, item.UpdatedAt)
	})
}

func (r *AnnouncementRepository) UpdateWithLog(
	ctx context.Context,
	id string,
	updates map[string]any,
	operatorID string,
	operatorIP string,
	detail string,
	logID string,
	now time.Time,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.Announcement{}).Where("id = ? AND deleted_at IS NULL", id).Updates(updates)
		if result.Error != nil {
			return fmt.Errorf("update announcement: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return createOperationLogTx(tx, logID, operatorID, "announcement_updated", "announcement", id, detail, operatorIP, now)
	})
}

func (r *AnnouncementRepository) SoftDeleteWithLog(
	ctx context.Context,
	id string,
	operatorID string,
	operatorIP string,
	detail string,
	logID string,
	now time.Time,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.Announcement{}).
			Where("id = ? AND deleted_at IS NULL", id).
			Updates(map[string]any{"deleted_at": now, "updated_at": now})
		if result.Error != nil {
			return fmt.Errorf("soft delete announcement: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return createOperationLogTx(tx, logID, operatorID, "announcement_deleted", "announcement", id, detail, operatorIP, now)
	})
}
