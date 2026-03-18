package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"openshare/backend/internal/model"
)

type PublicCatalogRepository struct {
	db *gorm.DB
}

type PublicFileListQuery struct {
	FolderID       *string
	FilterByFolder bool // true when the caller explicitly passed a folder_id (including root)
	Offset         int
	Limit          int
	OrderBy        []string
}

type PublicFolderRow struct {
	ID          string
	ParentID    *string
	Name        string
	Description string
	UpdatedAt   time.Time
}

type PublicFolderStatsRow struct {
	FolderID       string
	FileCount      int64
	DownloadCount  int64
	TotalSizeBytes int64
}

func NewPublicCatalogRepository(db *gorm.DB) *PublicCatalogRepository {
	return &PublicCatalogRepository{db: db}
}

func (r *PublicCatalogRepository) ListPublicFiles(ctx context.Context, query PublicFileListQuery) ([]model.File, int64, error) {
	base := r.db.WithContext(ctx).
		Model(&model.File{}).
		Where("status = ?", model.ResourceStatusActive)

	if query.FilterByFolder {
		if query.FolderID == nil {
			base = base.Where("folder_id IS NULL")
		} else {
			base = base.Where("folder_id = ?", *query.FolderID)
		}
	}
	// When FilterByFolder is false, no folder filter → show ALL active files

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count public files: %w", err)
	}

	listQuery := base
	for _, orderBy := range query.OrderBy {
		listQuery = listQuery.Order(orderBy)
	}

	var files []model.File
	if err := listQuery.Offset(query.Offset).Limit(query.Limit).Find(&files).Error; err != nil {
		return nil, 0, fmt.Errorf("list public files: %w", err)
	}

	return files, total, nil
}

func (r *PublicCatalogRepository) FolderExists(ctx context.Context, folderID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.Folder{}).
		Where("id = ? AND status = ?", folderID, model.ResourceStatusActive).
		Count(&count).
		Error
	if err != nil {
		return false, fmt.Errorf("check folder existence: %w", err)
	}

	return count > 0, nil
}

func (r *PublicCatalogRepository) ListPublicFolders(ctx context.Context, parentID *string) ([]PublicFolderRow, error) {
	query := r.db.WithContext(ctx).
		Model(&model.Folder{}).
		Select("id, parent_id, name, description, updated_at").
		Where("status = ?", model.ResourceStatusActive)

	if parentID == nil {
		query = query.Where("parent_id IS NULL")
	} else {
		query = query.Where("parent_id = ?", *parentID)
	}

	var rows []PublicFolderRow
	if err := query.Order("name ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list public folders: %w", err)
	}

	return rows, nil
}

func (r *PublicCatalogRepository) FindPublicFolderByID(ctx context.Context, folderID string) (*model.Folder, error) {
	var folder model.Folder
	err := r.db.WithContext(ctx).
		Where("id = ? AND status = ?", folderID, model.ResourceStatusActive).
		Take(&folder).
		Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("find public folder: %w", err)
	}

	return &folder, nil
}

func (r *PublicCatalogRepository) SummarizePublicFolders(ctx context.Context, folderIDs []string) (map[string]PublicFolderStatsRow, error) {
	if len(folderIDs) == 0 {
		return map[string]PublicFolderStatsRow{}, nil
	}

	var rows []PublicFolderStatsRow
	query := `
		WITH RECURSIVE folder_tree(root_id, id) AS (
			SELECT id AS root_id, id
			FROM folders
			WHERE id IN ?
			UNION ALL
			SELECT folder_tree.root_id, folders.id
			FROM folders
			JOIN folder_tree ON folders.parent_id = folder_tree.id
			WHERE folders.status = ?
		)
		SELECT
			folder_tree.root_id AS folder_id,
			COUNT(files.id) AS file_count,
			COALESCE(SUM(files.download_count), 0) AS download_count,
			COALESCE(SUM(files.size), 0) AS total_size_bytes
		FROM folder_tree
		LEFT JOIN files
			ON files.folder_id = folder_tree.id
			AND files.status = ?
			AND files.deleted_at IS NULL
		GROUP BY folder_tree.root_id
	`
	if err := r.db.WithContext(ctx).Raw(query, folderIDs, model.ResourceStatusActive, model.ResourceStatusActive).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("summarize public folders: %w", err)
	}

	result := make(map[string]PublicFolderStatsRow, len(rows))
	for _, row := range rows {
		result[row.FolderID] = row
	}
	return result, nil
}
