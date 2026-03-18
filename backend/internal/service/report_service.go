package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"openshare/backend/internal/model"
	"openshare/backend/internal/repository"
	"openshare/backend/internal/storage"
	"openshare/backend/pkg/identity"
)

// ---------------------------------------------------------------------------
// Sentinel errors
// ---------------------------------------------------------------------------

var (
	ErrReportNotFound       = errors.New("report not found")
	ErrReportNotPending     = errors.New("report is not pending")
	ErrReportReasonRequired = errors.New("report reason is required")
	ErrReportDescriptionRequired = errors.New("report description is required")
	ErrReportReasonInvalid  = errors.New("invalid report reason")
	ErrReportTargetRequired = errors.New("exactly one of file_id or folder_id is required")
	ErrReportTargetNotFound = errors.New("reported resource not found or already offline")
)

// validReportReasons maps the allowed reason codes to human-readable labels.
var validReportReasons = map[string]string{
	"copyright":     "侵权",
	"content_error": "内容错误",
	"irrelevant":    "无关资料",
}

// ---------------------------------------------------------------------------
// Service
// ---------------------------------------------------------------------------

// ReportService encapsulates the business logic for user reports.
type ReportService struct {
	repo         *repository.ReportRepository
	receiptCodes *ReceiptCodeService
	nowFunc      func() time.Time
}

func NewReportService(repo *repository.ReportRepository, receiptCodes *ReceiptCodeService, _ *SearchService, _ *storage.Service) *ReportService {
	return &ReportService{
		repo:         repo,
		receiptCodes: receiptCodes,
		nowFunc:      func() time.Time { return time.Now().UTC() },
	}
}

// ---------------------------------------------------------------------------
// DTOs
// ---------------------------------------------------------------------------

// CreateReportInput carries the validated request data for filing a report.
type CreateReportInput struct {
	FileID      string
	FolderID    string
	ReceiptCode string
	Reason      string
	Description string
	ReporterIP  string
}

// CreateReportResult is the response returned to the public caller.
type CreateReportResult struct {
	ReportID    string    `json:"report_id"`
	ReceiptCode string    `json:"receipt_code"`
	CreatedAt   time.Time `json:"created_at"`
}

type PublicReportLookupResult struct {
	ReceiptCode string                   `json:"receipt_code"`
	Items       []PublicReportLookupItem `json:"items"`
}

type PublicReportLookupItem struct {
	TargetName   string             `json:"target_name"`
	TargetType   string             `json:"target_type"`
	Reason       string             `json:"reason"`
	ReasonLabel  string             `json:"reason_label"`
	Description  string             `json:"description"`
	Status       model.ReportStatus `json:"status"`
	ReviewReason string             `json:"review_reason"`
	CreatedAt    time.Time          `json:"created_at"`
	ReviewedAt   *time.Time         `json:"reviewed_at"`
}

// PendingReportItem is the admin-facing projection of an unreviewed report.
type PendingReportItem struct {
	ID          string             `json:"id"`
	FileID      *string            `json:"file_id"`
	FolderID    *string            `json:"folder_id"`
	TargetName  string             `json:"target_name"`
	TargetType  string             `json:"target_type"`
	Reason      string             `json:"reason"`
	ReasonLabel string             `json:"reason_label"`
	Description string             `json:"description"`
	ReporterIP  string             `json:"reporter_ip"`
	Status      model.ReportStatus `json:"status"`
	CreatedAt   time.Time          `json:"created_at"`
}

// ReviewReportResult is returned after an admin approves or rejects a report.
type ReviewReportResult struct {
	ReportID   string             `json:"report_id"`
	Status     model.ReportStatus `json:"status"`
	ReviewedAt time.Time          `json:"reviewed_at"`
}

// ---------------------------------------------------------------------------
// Public: create a report
// ---------------------------------------------------------------------------

func (s *ReportService) CreateReport(ctx context.Context, input CreateReportInput) (*CreateReportResult, error) {
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		return nil, ErrReportReasonRequired
	}
	description := strings.TrimSpace(input.Description)
	if description == "" {
		return nil, ErrReportDescriptionRequired
	}
	if _, ok := validReportReasons[reason]; !ok {
		return nil, ErrReportReasonInvalid
	}

	hasFile := strings.TrimSpace(input.FileID) != ""
	hasFolder := strings.TrimSpace(input.FolderID) != ""
	if hasFile == hasFolder { // both set or neither set
		return nil, ErrReportTargetRequired
	}

	// Verify the target resource exists and is active.
	targetName := ""
	targetType := ""
	if hasFile {
		exists, err := s.repo.FileExists(ctx, input.FileID)
		if err != nil {
			return nil, fmt.Errorf("check file existence: %w", err)
		}
		if !exists {
			return nil, ErrReportTargetNotFound
		}
		targetType = "file"
	} else {
		exists, err := s.repo.FolderExists(ctx, input.FolderID)
		if err != nil {
			return nil, fmt.Errorf("check folder existence: %w", err)
		}
		if !exists {
			return nil, ErrReportTargetNotFound
		}
		targetType = "folder"
	}

	receiptCode, err := s.receiptCodes.ResolveForSession(ctx, input.ReceiptCode)
	if err != nil {
		return nil, err
	}

	if hasFile {
		targetName, err = s.repo.FindFileTitleByID(ctx, strings.TrimSpace(input.FileID))
	} else {
		targetName, err = s.repo.FindFolderNameByID(ctx, strings.TrimSpace(input.FolderID))
	}
	if err != nil {
		return nil, fmt.Errorf("load report target snapshot: %w", err)
	}

	reportID, err := identity.NewID()
	if err != nil {
		return nil, fmt.Errorf("generate report id: %w", err)
	}

	now := s.nowFunc()
	report := &model.Report{
		ID:          reportID,
		ReceiptCode: receiptCode,
		TargetName:  targetName,
		TargetType:  targetType,
		Reason:      reason,
		Description: description,
		ReporterIP:  input.ReporterIP,
		Status:      model.ReportStatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if hasFile {
		fid := strings.TrimSpace(input.FileID)
		report.FileID = &fid
	} else {
		fid := strings.TrimSpace(input.FolderID)
		report.FolderID = &fid
	}

	if err := s.repo.CreateReport(ctx, report); err != nil {
		return nil, fmt.Errorf("create report: %w", err)
	}

	return &CreateReportResult{
		ReportID:    reportID,
		ReceiptCode: receiptCode,
		CreatedAt:   now,
	}, nil
}

func (s *ReportService) LookupPublicReport(ctx context.Context, receiptCode string) (*PublicReportLookupResult, error) {
	normalized, err := normalizeReceiptCode(receiptCode)
	if err != nil {
		return nil, ErrInvalidUploadInput
	}

	reports, err := s.repo.FindPublicReportsByReceiptCode(ctx, normalized)
	if err != nil {
		return nil, fmt.Errorf("lookup public report: %w", err)
	}
	if len(reports) == 0 {
		return nil, ErrReportNotFound
	}

	items := make([]PublicReportLookupItem, 0, len(reports))
	for _, report := range reports {
		label, _ := validReportReasons[report.Reason]
		items = append(items, PublicReportLookupItem{
			TargetName:   report.TargetName,
			TargetType:   report.TargetType,
			Reason:       report.Reason,
			ReasonLabel:  label,
			Description:  report.Description,
			Status:       report.Status,
			ReviewReason: report.ReviewReason,
			CreatedAt:    report.CreatedAt,
			ReviewedAt:   report.ReviewedAt,
		})
	}

	return &PublicReportLookupResult{
		ReceiptCode: normalized,
		Items:       items,
	}, nil
}

// ---------------------------------------------------------------------------
// Admin: list pending reports
// ---------------------------------------------------------------------------

func (s *ReportService) ListPendingReports(ctx context.Context) ([]PendingReportItem, error) {
	rows, err := s.repo.ListPendingReports(ctx)
	if err != nil {
		return nil, fmt.Errorf("list pending reports: %w", err)
	}

	items := make([]PendingReportItem, 0, len(rows))
	for _, r := range rows {
		label, _ := validReportReasons[r.Reason]
		items = append(items, PendingReportItem{
			ID:          r.ID,
			FileID:      r.FileID,
			FolderID:    r.FolderID,
			TargetName:  r.TargetName,
			TargetType:  r.TargetType,
			Reason:      r.Reason,
			ReasonLabel: label,
			Description: r.Description,
			ReporterIP:  r.ReporterIP,
			Status:      r.Status,
			CreatedAt:   r.CreatedAt,
		})
	}

	return items, nil
}

// ---------------------------------------------------------------------------
// Admin: approve report (feedback accepted and marked as handled)
// ---------------------------------------------------------------------------

func (s *ReportService) ApproveReport(ctx context.Context, reportID, adminID, operatorIP, reviewReason string) (*ReviewReportResult, error) {
	report, err := s.repo.FindReportByID(ctx, strings.TrimSpace(reportID))
	if err != nil {
		return nil, fmt.Errorf("find report: %w", err)
	}
	if report == nil {
		return nil, ErrReportNotFound
	}
	if report.Status != model.ReportStatusPending {
		return nil, ErrReportNotPending
	}

	reviewedAt := s.nowFunc()
	logDetail := fmt.Sprintf("反馈原因=%s，处理说明=%s", report.Reason, strings.TrimSpace(reviewReason))
	if err := s.repo.ApproveReport(ctx, report.ID, adminID, operatorIP, reviewedAt, logDetail); err != nil {
		return nil, fmt.Errorf("approve report: %w", err)
	}

	return &ReviewReportResult{
		ReportID:   report.ID,
		Status:     model.ReportStatusApproved,
		ReviewedAt: reviewedAt,
	}, nil
}

// ---------------------------------------------------------------------------
// Admin: reject report (dismiss feedback)
// ---------------------------------------------------------------------------

func (s *ReportService) RejectReport(ctx context.Context, reportID, adminID, operatorIP, reviewReason string) (*ReviewReportResult, error) {
	report, err := s.repo.FindReportByID(ctx, strings.TrimSpace(reportID))
	if err != nil {
		return nil, fmt.Errorf("find report: %w", err)
	}
	if report == nil {
		return nil, ErrReportNotFound
	}
	if report.Status != model.ReportStatusPending {
		return nil, ErrReportNotPending
	}

	reviewedAt := s.nowFunc()
	logDetail := fmt.Sprintf("反馈原因=%s，驳回说明=%s", report.Reason, strings.TrimSpace(reviewReason))
	if err := s.repo.RejectReport(ctx, report.ID, adminID, operatorIP, reviewedAt, logDetail); err != nil {
		return nil, fmt.Errorf("reject report: %w", err)
	}

	return &ReviewReportResult{
		ReportID:   report.ID,
		Status:     model.ReportStatusRejected,
		ReviewedAt: reviewedAt,
	}, nil
}
