package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

type ReceiptCodeRepository struct {
	db *gorm.DB
}

func NewReceiptCodeRepository(db *gorm.DB) *ReceiptCodeRepository {
	return &ReceiptCodeRepository{db: db}
}

func (r *ReceiptCodeRepository) Exists(ctx context.Context, receiptCode string) (bool, error) {
	var submissionCount int64
	if err := r.db.WithContext(ctx).
		Table("submissions").
		Where("receipt_code = ?", receiptCode).
		Count(&submissionCount).
		Error; err != nil {
		return false, fmt.Errorf("count submissions by receipt code: %w", err)
	}
	if submissionCount > 0 {
		return true, nil
	}

	var reportCount int64
	if err := r.db.WithContext(ctx).
		Table("reports").
		Where("receipt_code = ?", receiptCode).
		Count(&reportCount).
		Error; err != nil {
		return false, fmt.Errorf("count reports by receipt code: %w", err)
	}

	return reportCount > 0, nil
}
