package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"openshare/backend/internal/model"
	"openshare/backend/pkg/identity"
)

type ModerationRepository struct {
	db *gorm.DB
}

type PendingSubmissionRow struct {
	SubmissionID  string
	ReceiptCode   string
	Title         string
	Description   string
	RelativePath  string
	Status        model.SubmissionStatus
	CreatedAt     time.Time
	FileID        string
	FileName      string
	FileSize      int64
	FileMimeType  string
	FileDiskPath  string
	StoredName    string
	DownloadCount int64
}

type SubmissionWithFile struct {
	Submission model.Submission
	File       model.File
}

func NewModerationRepository(db *gorm.DB) *ModerationRepository {
	return &ModerationRepository{db: db}
}

func (r *ModerationRepository) ListPendingSubmissions(ctx context.Context) ([]PendingSubmissionRow, error) {
	var rows []PendingSubmissionRow
	err := r.db.WithContext(ctx).
		Table("submissions").
		Select(`
			submissions.id AS submission_id,
			submissions.receipt_code AS receipt_code,
			submissions.title_snapshot AS title,
			submissions.description_snapshot AS description,
			submissions.relative_path_snapshot AS relative_path,
			submissions.status AS status,
			submissions.created_at AS created_at,
			files.id AS file_id,
			files.original_name AS file_name,
			files.size AS file_size,
			files.mime_type AS file_mime_type,
			files.disk_path AS file_disk_path,
			files.stored_name AS stored_name,
			files.download_count AS download_count
		`).
		Joins("JOIN files ON files.submission_id = submissions.id").
		Where("submissions.status = ?", model.SubmissionStatusPending).
		Order("submissions.created_at DESC").
		Scan(&rows).
		Error
	if err != nil {
		return nil, fmt.Errorf("list pending submissions: %w", err)
	}

	return rows, nil
}

func (r *ModerationRepository) FindPendingSubmission(ctx context.Context, submissionID string) (*SubmissionWithFile, error) {
	var submission model.Submission
	err := r.db.WithContext(ctx).
		Where("id = ?", submissionID).
		Take(&submission).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find submission: %w", err)
	}

	var file model.File
	err = r.db.WithContext(ctx).
		Where("submission_id = ?", submission.ID).
		Take(&file).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find file by submission: %w", err)
	}

	return &SubmissionWithFile{Submission: submission, File: file}, nil
}

func (r *ModerationRepository) FindFolderByID(ctx context.Context, folderID string) (*model.Folder, error) {
	var folder model.Folder
	err := r.db.WithContext(ctx).Where("id = ?", folderID).Take(&folder).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find folder: %w", err)
	}
	return &folder, nil
}

func (r *ModerationRepository) DB() *gorm.DB {
	return r.db
}

func (r *ModerationRepository) ApproveSubmission(
	ctx context.Context,
	submissionID string,
	adminID string,
	operatorIP string,
	reviewedAt time.Time,
	targetFolderID string,
	finalDiskPath string,
	finalStoredName string,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var submission model.Submission
		if err := tx.Where("id = ?", submissionID).Take(&submission).Error; err != nil {
			return fmt.Errorf("reload submission: %w", err)
		}
		if submission.Status != model.SubmissionStatusPending {
			return fmt.Errorf("submission is not pending")
		}

		var file model.File
		if err := tx.Where("submission_id = ?", submissionID).Take(&file).Error; err != nil {
			return fmt.Errorf("reload file: %w", err)
		}

		reviewerID := adminID
		if err := tx.Model(&model.File{}).
			Where("id = ?", file.ID).
			Updates(map[string]any{
				"folder_id":   targetFolderID,
				"status":      model.ResourceStatusActive,
				"disk_path":   finalDiskPath,
				"stored_name": finalStoredName,
				"updated_at":  reviewedAt,
			}).Error; err != nil {
			return fmt.Errorf("approve file: %w", err)
		}

		if err := tx.Model(&model.Submission{}).
			Where("id = ?", submissionID).
			Updates(map[string]any{
				"status":        model.SubmissionStatusApproved,
				"reject_reason": "",
				"reviewer_id":   &reviewerID,
				"reviewed_at":   &reviewedAt,
				"updated_at":    reviewedAt,
			}).Error; err != nil {
			return fmt.Errorf("approve submission: %w", err)
		}

		logID, err := newOperationLogID()
		if err != nil {
			return fmt.Errorf("generate operation log id: %w", err)
		}
		entry := &model.OperationLog{
			ID:         logID,
			AdminID:    &reviewerID,
			Action:     "submission_approved",
			TargetType: "submission",
			TargetID:   submissionID,
			Detail:     file.OriginalName,
			IP:         operatorIP,
			CreatedAt:  reviewedAt,
		}
		if err := tx.Create(entry).Error; err != nil {
			return fmt.Errorf("create operation log: %w", err)
		}

		return nil
	})
}

func (r *ModerationRepository) RejectSubmission(
	ctx context.Context,
	submissionID string,
	adminID string,
	operatorIP string,
	reviewedAt time.Time,
	rejectReason string,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var submission model.Submission
		if err := tx.Where("id = ?", submissionID).Take(&submission).Error; err != nil {
			return fmt.Errorf("reload submission: %w", err)
		}
		if submission.Status != model.SubmissionStatusPending {
			return fmt.Errorf("submission is not pending")
		}

		var file model.File
		if err := tx.Where("submission_id = ?", submissionID).Take(&file).Error; err != nil {
			return fmt.Errorf("reload file: %w", err)
		}

		reviewerID := adminID
		if err := tx.Model(&model.Submission{}).
			Where("id = ?", submissionID).
			Updates(map[string]any{
				"status":        model.SubmissionStatusRejected,
				"reject_reason": rejectReason,
				"reviewer_id":   &reviewerID,
				"reviewed_at":   &reviewedAt,
				"updated_at":    reviewedAt,
			}).Error; err != nil {
			return fmt.Errorf("reject submission: %w", err)
		}

		if err := tx.Model(&model.File{}).
			Where("id = ?", file.ID).
			Updates(map[string]any{
				"disk_path":  "",
				"updated_at": reviewedAt,
			}).Error; err != nil {
			return fmt.Errorf("touch file after rejection: %w", err)
		}

		logID, err := newOperationLogID()
		if err != nil {
			return fmt.Errorf("generate operation log id: %w", err)
		}
		entry := &model.OperationLog{
			ID:         logID,
			AdminID:    &reviewerID,
			Action:     "submission_rejected",
			TargetType: "submission",
			TargetID:   submissionID,
			Detail:     rejectReason,
			IP:         operatorIP,
			CreatedAt:  reviewedAt,
		}
		if err := tx.Create(entry).Error; err != nil {
			return fmt.Errorf("create operation log: %w", err)
		}

		return nil
	})
}

func newOperationLogID() (string, error) {
	return identity.NewID()
}
