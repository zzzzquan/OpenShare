package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"openshare/backend/internal/model"
)

type PublicSubmissionRepository struct {
	db *gorm.DB
}

type SubmissionLookupRow struct {
	ReceiptCode   string
	TitleSnapshot string
	RelativePath  string
	Status        model.SubmissionStatus
	RejectReason  string
	CreatedAt     time.Time
	DownloadCount int64
}

func NewPublicSubmissionRepository(db *gorm.DB) *PublicSubmissionRepository {
	return &PublicSubmissionRepository{db: db}
}

func (r *PublicSubmissionRepository) FindAllByReceiptCode(ctx context.Context, receiptCode string) ([]SubmissionLookupRow, error) {
	var rows []SubmissionLookupRow
	err := r.db.WithContext(ctx).
		Table("submissions").
		Select(`
			submissions.receipt_code AS receipt_code,
			submissions.title_snapshot AS title_snapshot,
			submissions.relative_path_snapshot AS relative_path,
			submissions.status AS status,
			submissions.reject_reason AS reject_reason,
			submissions.created_at AS created_at,
			COALESCE((
				SELECT MAX(files.download_count)
				FROM files
				WHERE files.submission_id = submissions.id
			), 0) AS download_count
		`).
		Where("submissions.receipt_code = ?", receiptCode).
		Order("submissions.created_at DESC").
		Find(&rows).
		Error
	if err != nil {
		return nil, fmt.Errorf("find submissions by receipt code: %w", err)
	}

	return rows, nil
}
