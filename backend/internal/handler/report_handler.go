package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"openshare/backend/internal/service"
	"openshare/backend/internal/session"
)

// ReportHandler exposes HTTP endpoints for the report lifecycle.
type ReportHandler struct {
	service *service.ReportService
}

func NewReportHandler(service *service.ReportService) *ReportHandler {
	return &ReportHandler{service: service}
}

// ---------------------------------------------------------------------------
// Public: create a report
// ---------------------------------------------------------------------------

type createReportRequest struct {
	FileID      string `json:"file_id"`
	FolderID    string `json:"folder_id"`
	Reason      string `json:"reason"`
	Description string `json:"description"`
}

func (h *ReportHandler) CreateReport(ctx *gin.Context) {
	var req createReportRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	result, err := h.service.CreateReport(ctx.Request.Context(), service.CreateReportInput{
		FileID:      req.FileID,
		FolderID:    req.FolderID,
		Reason:      req.Reason,
		Description: req.Description,
		ReporterIP:  ctx.ClientIP(),
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrReportReasonRequired):
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "reason is required"})
		case errors.Is(err, service.ErrReportReasonInvalid):
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid report reason"})
		case errors.Is(err, service.ErrReportTargetRequired):
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "exactly one of file_id or folder_id is required"})
		case errors.Is(err, service.ErrReportTargetNotFound):
			ctx.JSON(http.StatusNotFound, gin.H{"error": "reported resource not found or already offline"})
		default:
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create report"})
		}
		return
	}

	ctx.JSON(http.StatusCreated, result)
}

// ---------------------------------------------------------------------------
// Admin: list pending reports
// ---------------------------------------------------------------------------

func (h *ReportHandler) ListPendingReports(ctx *gin.Context) {
	items, err := h.service.ListPendingReports(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list pending reports"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"items": items})
}

// ---------------------------------------------------------------------------
// Admin: approve report (upholds the report, takes resource offline)
// ---------------------------------------------------------------------------

type reviewReportRequest struct {
	ReviewReason string `json:"review_reason"`
}

func (h *ReportHandler) ApproveReport(ctx *gin.Context) {
	identity, ok := session.GetAdminIdentity(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	var req reviewReportRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		// Body is optional for approve; treat parse error gracefully.
		req = reviewReportRequest{}
	}

	result, err := h.service.ApproveReport(
		ctx.Request.Context(),
		ctx.Param("reportID"),
		identity.AdminID,
		ctx.ClientIP(),
		req.ReviewReason,
	)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrReportNotFound):
			ctx.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
		case errors.Is(err, service.ErrReportNotPending):
			ctx.JSON(http.StatusConflict, gin.H{"error": "report is not pending"})
		default:
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to approve report"})
		}
		return
	}

	ctx.JSON(http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// Admin: reject report (dismisses the report, resource stays visible)
// ---------------------------------------------------------------------------

func (h *ReportHandler) RejectReport(ctx *gin.Context) {
	identity, ok := session.GetAdminIdentity(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	var req reviewReportRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		req = reviewReportRequest{}
	}

	result, err := h.service.RejectReport(
		ctx.Request.Context(),
		ctx.Param("reportID"),
		identity.AdminID,
		ctx.ClientIP(),
		req.ReviewReason,
	)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrReportNotFound):
			ctx.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
		case errors.Is(err, service.ErrReportNotPending):
			ctx.JSON(http.StatusConflict, gin.H{"error": "report is not pending"})
		default:
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reject report"})
		}
		return
	}

	ctx.JSON(http.StatusOK, result)
}
