package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"openshare/backend/internal/model"
)

type OperationLogRepository struct {
	db *gorm.DB
}

type OperationLogRow struct {
	ID         string
	AdminID    *string
	AdminName  string
	Action     string
	TargetType string
	TargetID   string
	Detail     string
	IP         string
	CreatedAt  time.Time
}

func NewOperationLogRepository(db *gorm.DB) *OperationLogRepository {
	return &OperationLogRepository{db: db}
}

func (r *OperationLogRepository) List(ctx context.Context, action string, targetType string, page int, pageSize int) ([]OperationLogRow, int64, error) {
	dbq := r.db.WithContext(ctx).
		Table("operation_logs").
		Select(`
			operation_logs.id,
			operation_logs.admin_id,
			COALESCE(NULLIF(admins.display_name, ''), admins.username, '') AS admin_name,
			operation_logs.action,
			operation_logs.target_type,
			operation_logs.target_id,
			operation_logs.detail,
			operation_logs.ip,
			operation_logs.created_at
		`).
		Joins("LEFT JOIN admins ON admins.id = operation_logs.admin_id")

	if trimmed := strings.TrimSpace(action); trimmed != "" {
		dbq = dbq.Where("operation_logs.action = ?", trimmed)
	}
	if trimmed := strings.TrimSpace(targetType); trimmed != "" {
		dbq = dbq.Where("operation_logs.target_type = ?", trimmed)
	}

	var total int64
	if err := dbq.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count operation logs: %w", err)
	}

	var rows []OperationLogRow
	if err := dbq.
		Order("operation_logs.created_at DESC, operation_logs.id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("list operation logs: %w", err)
	}
	return rows, total, nil
}

func createOperationLogTx(
	tx *gorm.DB,
	logID string,
	adminID string,
	action string,
	targetType string,
	targetID string,
	detail string,
	operatorIP string,
	now time.Time,
) error {
	var adminRef *string
	if adminID != "" {
		adminRef = &adminID
	}

	entry := &model.OperationLog{
		ID:         logID,
		AdminID:    adminRef,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Detail:     detail,
		IP:         operatorIP,
		CreatedAt:  now,
	}
	if err := tx.Create(entry).Error; err != nil {
		return fmt.Errorf("create operation log: %w", err)
	}
	return nil
}
