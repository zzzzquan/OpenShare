package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"openshare/backend/internal/model"
)

type SystemSettingRepository struct {
	db *gorm.DB
}

func NewSystemSettingRepository(db *gorm.DB) *SystemSettingRepository {
	return &SystemSettingRepository{db: db}
}

func (r *SystemSettingRepository) FindByKey(ctx context.Context, key string) (*model.SystemSetting, error) {
	var item model.SystemSetting
	err := r.db.WithContext(ctx).Where("key = ?", key).Take(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find system setting: %w", err)
	}
	return &item, nil
}

func (r *SystemSettingRepository) UpsertWithLog(
	ctx context.Context,
	key string,
	value string,
	updatedBy string,
	operatorIP string,
	logID string,
	now time.Time,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.SystemSetting
		err := tx.Where("key = ?", key).Take(&existing).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			ref := updatedBy
			existing = model.SystemSetting{
				Key:         key,
				Value:       value,
				UpdatedByID: &ref,
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			if err := tx.Create(&existing).Error; err != nil {
				return fmt.Errorf("create system setting: %w", err)
			}
		case err != nil:
			return fmt.Errorf("find system setting for upsert: %w", err)
		default:
			ref := updatedBy
			if err := tx.Model(&model.SystemSetting{}).
				Where("key = ?", key).
				Updates(map[string]any{
					"value":         value,
					"updated_by_id": &ref,
					"updated_at":    now,
				}).Error; err != nil {
				return fmt.Errorf("update system setting: %w", err)
			}
		}

		return createOperationLogTx(tx, logID, updatedBy, "system_settings_updated", "system_setting", key, key, operatorIP, now)
	})
}
