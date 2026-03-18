package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"openshare/backend/internal/model"
)

type AdminDashboardRepository struct {
	db *gorm.DB
}

type AdminDashboardStatsRow struct {
	TotalVisitorIPs    int64
	TotalFiles         int64
	TotalDownloads     int64
	RecentVisitorIPs   int64
	RecentFiles        int64
	RecentDownloads    int64
	PendingSubmissions int64
	PendingReports     int64
}

func NewAdminDashboardRepository(db *gorm.DB) *AdminDashboardRepository {
	return &AdminDashboardRepository{db: db}
}

func (r *AdminDashboardRepository) GetStats(ctx context.Context, since time.Time) (*AdminDashboardStatsRow, error) {
	row := &AdminDashboardStatsRow{}

	if err := r.db.WithContext(ctx).
		Model(&model.SiteVisitEvent{}).
		Where("ip <> ''").
		Distinct("ip").
		Count(&row.TotalVisitorIPs).
		Error; err != nil {
		return nil, fmt.Errorf("count total visitor ips: %w", err)
	}

	if err := r.db.WithContext(ctx).
		Model(&model.File{}).
		Where("status = ?", model.ResourceStatusActive).
		Count(&row.TotalFiles).
		Error; err != nil {
		return nil, fmt.Errorf("count active files: %w", err)
	}

	if err := r.db.WithContext(ctx).
		Model(&model.File{}).
		Where("status = ?", model.ResourceStatusActive).
		Select("COALESCE(SUM(download_count), 0)").
		Scan(&row.TotalDownloads).
		Error; err != nil {
		return nil, fmt.Errorf("sum active file downloads: %w", err)
	}

	if err := r.db.WithContext(ctx).
		Model(&model.SiteVisitEvent{}).
		Where("ip <> '' AND created_at >= ?", since).
		Distinct("ip").
		Count(&row.RecentVisitorIPs).
		Error; err != nil {
		return nil, fmt.Errorf("count recent visitor ips: %w", err)
	}

	if err := r.db.WithContext(ctx).
		Model(&model.File{}).
		Where("status = ? AND created_at >= ?", model.ResourceStatusActive, since).
		Count(&row.RecentFiles).
		Error; err != nil {
		return nil, fmt.Errorf("count recent active files: %w", err)
	}

	if err := r.db.WithContext(ctx).
		Model(&model.DownloadEvent{}).
		Where("created_at >= ?", since).
		Count(&row.RecentDownloads).
		Error; err != nil {
		return nil, fmt.Errorf("count recent downloads: %w", err)
	}

	if err := r.db.WithContext(ctx).
		Model(&model.Submission{}).
		Where("status = ?", model.SubmissionStatusPending).
		Count(&row.PendingSubmissions).
		Error; err != nil {
		return nil, fmt.Errorf("count pending submissions: %w", err)
	}

	if err := r.db.WithContext(ctx).
		Model(&model.Report{}).
		Where("status = ?", model.ReportStatusPending).
		Count(&row.PendingReports).
		Error; err != nil {
		return nil, fmt.Errorf("count pending reports: %w", err)
	}

	return row, nil
}
