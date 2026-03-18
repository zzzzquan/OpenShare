package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"openshare/backend/internal/model"
)

type AdminRepository struct {
	db *gorm.DB
}

func NewAdminRepository(db *gorm.DB) *AdminRepository {
	return &AdminRepository{db: db}
}

func (r *AdminRepository) HasSuperAdmin(tx *gorm.DB) (bool, error) {
	var count int64
	err := tx.Model(&model.Admin{}).
		Where("role = ?", model.AdminRoleSuperAdmin).
		Count(&count).
		Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *AdminRepository) Create(tx *gorm.DB, admin *model.Admin) error {
	return tx.Create(admin).Error
}

func (r *AdminRepository) FindByUsername(tx *gorm.DB, username string) (*model.Admin, error) {
	var admin model.Admin
	err := tx.Where("username = ?", username).Take(&admin).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &admin, nil
}

func (r *AdminRepository) FindByID(ctx context.Context, adminID string) (*model.Admin, error) {
	var admin model.Admin
	err := r.db.WithContext(ctx).Where("id = ?", adminID).Take(&admin).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find admin by id: %w", err)
	}
	return &admin, nil
}

func (r *AdminRepository) ListAdmins(ctx context.Context) ([]model.Admin, error) {
	var admins []model.Admin
	if err := r.db.WithContext(ctx).Order("created_at ASC").Find(&admins).Error; err != nil {
		return nil, fmt.Errorf("list admins: %w", err)
	}
	return admins, nil
}

func (r *AdminRepository) UsernameExists(ctx context.Context, username string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Admin{}).Where("username = ?", username).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check username existence: %w", err)
	}
	return count > 0, nil
}

func (r *AdminRepository) DisplayNameExists(ctx context.Context, displayName string, excludeAdminID string) (bool, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(&model.Admin{}).Where("display_name = ?", displayName)
	if excludeAdminID != "" {
		query = query.Where("id <> ?", excludeAdminID)
	}
	err := query.Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check display name existence: %w", err)
	}
	return count > 0, nil
}

func (r *AdminRepository) CreateWithLog(
	ctx context.Context,
	admin *model.Admin,
	operatorID string,
	operatorIP string,
	action string,
	detail string,
	logID string,
	now time.Time,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(admin).Error; err != nil {
			return fmt.Errorf("create admin: %w", err)
		}
		return createOperationLogTx(tx, logID, operatorID, action, "admin", admin.ID, detail, operatorIP, now)
	})
}

func (r *AdminRepository) UpdateAdminWithLog(
	ctx context.Context,
	adminID string,
	updates map[string]any,
	operatorID string,
	operatorIP string,
	action string,
	detail string,
	logID string,
	now time.Time,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.Admin{}).Where("id = ?", adminID).Updates(updates)
		if result.Error != nil {
			return fmt.Errorf("update admin: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return createOperationLogTx(tx, logID, operatorID, action, "admin", adminID, detail, operatorIP, now)
	})
}

func (r *AdminRepository) DeleteAdminWithLog(
	ctx context.Context,
	adminID string,
	operatorID string,
	operatorIP string,
	action string,
	detail string,
	logID string,
	now time.Time,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("admin_id = ?", adminID).Delete(&model.AdminSession{}).Error; err != nil {
			return fmt.Errorf("delete admin sessions: %w", err)
		}
		result := tx.Where("id = ?", adminID).Delete(&model.Admin{})
		if result.Error != nil {
			return fmt.Errorf("delete admin: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return createOperationLogTx(tx, logID, operatorID, action, "admin", adminID, detail, operatorIP, now)
	})
}
