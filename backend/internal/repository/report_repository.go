package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"openshare/backend/internal/model"
	"openshare/backend/pkg/identity"
)

// ReportRepository handles persistence for the report lifecycle.
type ReportRepository struct {
	db *gorm.DB
}

func NewReportRepository(db *gorm.DB) *ReportRepository {
	return &ReportRepository{db: db}
}

// ---------------------------------------------------------------------------
// Public (guest) operations
// ---------------------------------------------------------------------------

// CreateReport inserts a new report. Caller must ensure exactly one of
// report.FileID or report.FolderID is set.
func (r *ReportRepository) CreateReport(ctx context.Context, report *model.Report) error {
	return r.db.WithContext(ctx).Create(report).Error
}

// FileExists returns true if an active file with the given ID exists.
func (r *ReportRepository) FileExists(ctx context.Context, fileID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.File{}).
		Where("id = ? AND status = ? AND deleted_at IS NULL", fileID, model.ResourceStatusActive).
		Count(&count).Error
	return count > 0, err
}

// FolderExists returns true if an active folder with the given ID exists.
func (r *ReportRepository) FolderExists(ctx context.Context, folderID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.Folder{}).
		Where("id = ? AND status = ? AND deleted_at IS NULL", folderID, model.ResourceStatusActive).
		Count(&count).Error
	return count > 0, err
}

func (r *ReportRepository) FindFileTitleByID(ctx context.Context, fileID string) (string, error) {
	var row struct {
		Title string
	}
	if err := r.db.WithContext(ctx).
		Model(&model.File{}).
		Select("title").
		Where("id = ?", fileID).
		Take(&row).
		Error; err != nil {
		return "", err
	}
	return row.Title, nil
}

func (r *ReportRepository) FindFolderNameByID(ctx context.Context, folderID string) (string, error) {
	var row struct {
		Name string
	}
	if err := r.db.WithContext(ctx).
		Model(&model.Folder{}).
		Select("name").
		Where("id = ?", folderID).
		Take(&row).
		Error; err != nil {
		return "", err
	}
	return row.Name, nil
}

// ---------------------------------------------------------------------------
// Admin query helpers
// ---------------------------------------------------------------------------

// PendingReportRow is the denormalized projection for the pending-reports list.
type PendingReportRow struct {
	ID          string
	FileID      *string
	FolderID    *string
	TargetName  string
	TargetType  string // "file" | "folder"
	Reason      string
	Description string
	ReporterIP  string
	Status      model.ReportStatus
	CreatedAt   time.Time
}

type PublicReportLookupRow struct {
	ReceiptCode  string
	TargetName   string
	TargetType   string
	Reason       string
	Description  string
	Status       model.ReportStatus
	ReviewReason string
	CreatedAt    time.Time
	ReviewedAt   *time.Time
}

// ListPendingReports returns reports that have not yet been reviewed,
// ordered newest-first.
func (r *ReportRepository) ListPendingReports(ctx context.Context) ([]PendingReportRow, error) {
	var rows []PendingReportRow
	if err := r.db.WithContext(ctx).
		Table("reports").
		Select(`
			reports.id,
			reports.file_id,
			reports.folder_id,
			reports.target_name AS target_name,
			reports.target_type AS target_type,
			reports.reason,
			reports.description,
			reports.reporter_ip,
			reports.status,
			reports.created_at
		`).
		Where("reports.status = ?", model.ReportStatusPending).
		Order("reports.created_at DESC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list pending reports: %w", err)
	}
	return rows, nil
}

// FindReportByID loads a single report with its target entity name.
func (r *ReportRepository) FindReportByID(ctx context.Context, reportID string) (*model.Report, error) {
	var report model.Report
	err := r.db.WithContext(ctx).Where("id = ?", reportID).Take(&report).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find report: %w", err)
	}
	return &report, nil
}

func (r *ReportRepository) FindPublicReportsByReceiptCode(ctx context.Context, receiptCode string) ([]PublicReportLookupRow, error) {
	var rows []PublicReportLookupRow
	if err := r.db.WithContext(ctx).
		Table("reports").
		Select(`
			reports.receipt_code AS receipt_code,
			reports.target_name AS target_name,
			reports.target_type AS target_type,
			reports.reason,
			reports.description,
			reports.status,
			reports.review_reason,
			reports.created_at,
			reports.reviewed_at
		`).
		Where("receipt_code = ?", strings.TrimSpace(receiptCode)).
		Order("created_at DESC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("find public reports by receipt code: %w", err)
	}
	return rows, nil
}

// ---------------------------------------------------------------------------
// Admin review operations
// ---------------------------------------------------------------------------

// ApproveReport marks the report as handled and records an operation log.
func (r *ReportRepository) ApproveReport(
	ctx context.Context,
	reportID string,
	adminID string,
	operatorIP string,
	reviewedAt time.Time,
	reviewReason string,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var report model.Report
		if err := tx.Where("id = ?", reportID).Take(&report).Error; err != nil {
			return fmt.Errorf("reload report: %w", err)
		}
		if report.Status != model.ReportStatusPending {
			return fmt.Errorf("report is not pending")
		}

		reviewerID := adminID
		if err := tx.Model(&model.Report{}).Where("id = ?", reportID).Updates(map[string]any{
			"status":        model.ReportStatusApproved,
			"review_reason": reviewReason,
			"reviewer_id":   &reviewerID,
			"reviewed_at":   &reviewedAt,
			"updated_at":    reviewedAt,
		}).Error; err != nil {
			return fmt.Errorf("approve report: %w", err)
		}

		logID, err := identity.NewID()
		if err != nil {
			return fmt.Errorf("generate log id: %w", err)
		}

		targetType, targetID := reportTarget(&report)
		entry := &model.OperationLog{
			ID:         logID,
			AdminID:    &reviewerID,
			Action:     "report_approved",
			TargetType: targetType,
			TargetID:   targetID,
			Detail:     strings.TrimSpace(reviewReason),
			IP:         operatorIP,
			CreatedAt:  reviewedAt,
		}
		return tx.Create(entry).Error
	})
}

// RejectReport marks the report as rejected without affecting the target
// resource, and records an operation log.
func (r *ReportRepository) RejectReport(
	ctx context.Context,
	reportID string,
	adminID string,
	operatorIP string,
	reviewedAt time.Time,
	reviewReason string,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var report model.Report
		if err := tx.Where("id = ?", reportID).Take(&report).Error; err != nil {
			return fmt.Errorf("reload report: %w", err)
		}
		if report.Status != model.ReportStatusPending {
			return fmt.Errorf("report is not pending")
		}

		reviewerID := adminID
		if err := tx.Model(&model.Report{}).Where("id = ?", reportID).Updates(map[string]any{
			"status":        model.ReportStatusRejected,
			"review_reason": reviewReason,
			"reviewer_id":   &reviewerID,
			"reviewed_at":   &reviewedAt,
			"updated_at":    reviewedAt,
		}).Error; err != nil {
			return fmt.Errorf("reject report: %w", err)
		}

		logID, err := identity.NewID()
		if err != nil {
			return fmt.Errorf("generate log id: %w", err)
		}

		targetType, targetID := reportTarget(&report)
		entry := &model.OperationLog{
			ID:         logID,
			AdminID:    &reviewerID,
			Action:     "report_rejected",
			TargetType: targetType,
			TargetID:   targetID,
			Detail:     reviewReason,
			IP:         operatorIP,
			CreatedAt:  reviewedAt,
		}
		return tx.Create(entry).Error
	})
}

// reportTarget extracts (target_type, target_id) from a report.
func reportTarget(r *model.Report) (string, string) {
	if r.FileID != nil {
		return "file", *r.FileID
	}
	if r.FolderID != nil {
		return "folder", *r.FolderID
	}
	return "unknown", ""
}
