package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"openshare/backend/internal/model"
)

type ResourceManagementRepository struct {
	db *gorm.DB
}

type ManagedFileRow struct {
	ID            string
	Title         string
	Description   string
	OriginalName  string
	Status        model.ResourceStatus
	Size          int64
	DownloadCount int64
	FolderName    string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DiskPath      string
}

type ManagedFolderPathRow struct {
	ID         string
	ParentID   *string
	Name       string
	SourcePath *string
}

type ManagedFilePathRow struct {
	ID         string
	FolderID   *string
	DiskPath   string
	SourcePath *string
}

type FolderTreeNode struct {
	ID string
}

func NewResourceManagementRepository(db *gorm.DB) *ResourceManagementRepository {
	return &ResourceManagementRepository{db: db}
}

func (r *ResourceManagementRepository) ListFiles(ctx context.Context, query string, status string) ([]ManagedFileRow, error) {
	dbq := r.db.WithContext(ctx).
		Table("files").
		Select(`
			files.id,
			files.title,
			files.description,
			files.original_name,
			files.status,
			files.size,
			files.download_count,
			files.created_at,
			files.updated_at,
			files.disk_path,
			COALESCE(folders.name, '') AS folder_name
		`).
		Joins("LEFT JOIN folders ON folders.id = files.folder_id")

	if trimmed := strings.TrimSpace(status); trimmed != "" {
		dbq = dbq.Where("files.status = ?", trimmed)
	}
	if trimmed := strings.TrimSpace(query); trimmed != "" {
		like := "%" + trimmed + "%"
		dbq = dbq.Where("files.title LIKE ? OR files.original_name LIKE ? OR files.description LIKE ?", like, like, like)
	}

	var rows []ManagedFileRow
	if err := dbq.Order("files.updated_at DESC, files.id DESC").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list managed files: %w", err)
	}
	return rows, nil
}

func (r *ResourceManagementRepository) FindFileByID(ctx context.Context, fileID string) (*model.File, error) {
	var file model.File
	err := r.db.WithContext(ctx).Where("id = ?", fileID).Take(&file).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find file: %w", err)
	}
	return &file, nil
}

func (r *ResourceManagementRepository) FindFolderByID(ctx context.Context, folderID string) (*model.Folder, error) {
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

func (r *ResourceManagementRepository) ListFolderTreeIDs(ctx context.Context, folderID string) ([]string, error) {
	var rows []FolderTreeNode
	query := `
		WITH RECURSIVE folder_tree(id) AS (
			SELECT id FROM folders WHERE id = ?
			UNION ALL
			SELECT folders.id
			FROM folders
			JOIN folder_tree ON folders.parent_id = folder_tree.id
		)
		SELECT id FROM folder_tree
	`
	if err := r.db.WithContext(ctx).Raw(query, folderID).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list folder tree ids: %w", err)
	}
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.ID)
	}
	return result, nil
}

func (r *ResourceManagementRepository) FolderNameExists(ctx context.Context, parentID *string, name, excludeFolderID string) (bool, error) {
	query := r.db.WithContext(ctx).Model(&model.Folder{}).
		Where("LOWER(name) = LOWER(?)", name).
		Where("id <> ?", excludeFolderID)
	if parentID == nil {
		query = query.Where("parent_id IS NULL")
	} else {
		query = query.Where("parent_id = ?", *parentID)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, fmt.Errorf("check folder name conflict: %w", err)
	}
	return count > 0, nil
}

func (r *ResourceManagementRepository) ListFolderPaths(ctx context.Context) ([]ManagedFolderPathRow, error) {
	var rows []ManagedFolderPathRow
	if err := r.db.WithContext(ctx).
		Model(&model.Folder{}).
		Select("id, parent_id, name, source_path").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list folder paths: %w", err)
	}
	return rows, nil
}

func (r *ResourceManagementRepository) ListFilePaths(ctx context.Context) ([]ManagedFilePathRow, error) {
	var rows []ManagedFilePathRow
	if err := r.db.WithContext(ctx).
		Model(&model.File{}).
		Select("id, folder_id, disk_path, source_path").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list file paths: %w", err)
	}
	return rows, nil
}

func (r *ResourceManagementRepository) UpdateFileMetadata(
	ctx context.Context,
	fileID string,
	title string,
	description string,
	operatorID string,
	operatorIP string,
	logID string,
	now time.Time,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.File{}).Where("id = ?", fileID).Updates(map[string]any{
			"title":       title,
			"description": description,
			"updated_at":  now,
		})
		if result.Error != nil {
			return fmt.Errorf("update file metadata: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		return createOperationLogTx(tx, logID, operatorID, "resource_updated", "file", fileID, title, operatorIP, now)
	})
}

func (r *ResourceManagementRepository) UpdateFileStatusWithLog(
	ctx context.Context,
	fileID string,
	status model.ResourceStatus,
	deletedAt *time.Time,
	diskPath string,
	operatorID string,
	operatorIP string,
	action string,
	detail string,
	logID string,
	now time.Time,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			"status":     status,
			"updated_at": now,
			"disk_path":  diskPath,
		}
		if deletedAt != nil {
			updates["deleted_at"] = deletedAt
		}

		result := tx.Model(&model.File{}).Where("id = ?", fileID).Updates(updates)
		if result.Error != nil {
			return fmt.Errorf("update file status: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		return createOperationLogTx(tx, logID, operatorID, action, "file", fileID, detail, operatorIP, now)
	})
}

func (r *ResourceManagementRepository) UpdateFolderMetadata(
	ctx context.Context,
	folderID string,
	name string,
	description string,
	operatorID string,
	operatorIP string,
	logID string,
	now time.Time,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.Folder{}).Where("id = ?", folderID).Updates(map[string]any{
			"name":        name,
			"description": description,
			"updated_at":  now,
		})
		if result.Error != nil {
			return fmt.Errorf("update folder metadata: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		return createOperationLogTx(tx, logID, operatorID, "folder_updated", "folder", folderID, name, operatorIP, now)
	})
}

func (r *ResourceManagementRepository) UpdateFolderTreePaths(
	ctx context.Context,
	folderID string,
	name string,
	description string,
	folderSourcePaths map[string]string,
	filePaths map[string]ManagedFilePathRow,
	operatorID string,
	operatorIP string,
	logID string,
	now time.Time,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rootUpdates := map[string]any{
			"name":        name,
			"description": description,
			"updated_at":  now,
		}
		if sourcePath, ok := folderSourcePaths[folderID]; ok {
			rootUpdates["source_path"] = sourcePath
		}

		result := tx.Model(&model.Folder{}).Where("id = ?", folderID).Updates(rootUpdates)
		if result.Error != nil {
			return fmt.Errorf("update root folder metadata: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		for id, sourcePath := range folderSourcePaths {
			if id == folderID {
				continue
			}
			if err := tx.Model(&model.Folder{}).Where("id = ?", id).Updates(map[string]any{
				"source_path": sourcePath,
				"updated_at":  now,
			}).Error; err != nil {
				return fmt.Errorf("update child folder path: %w", err)
			}
		}

		for id, row := range filePaths {
			updates := map[string]any{
				"disk_path":  row.DiskPath,
				"updated_at": now,
			}
			if row.SourcePath != nil {
				updates["source_path"] = *row.SourcePath
			}
			if err := tx.Model(&model.File{}).Where("id = ?", id).Updates(updates).Error; err != nil {
				return fmt.Errorf("update file path: %w", err)
			}
		}

		return createOperationLogTx(tx, logID, operatorID, "folder_updated", "folder", folderID, name, operatorIP, now)
	})
}

func (r *ResourceManagementRepository) DeleteFolderTreeWithLog(
	ctx context.Context,
	rootFolderID string,
	folderIDs []string,
	rootSourcePath string,
	operatorID string,
	operatorIP string,
	logID string,
	detail string,
	now time.Time,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(folderIDs) == 0 {
			return gorm.ErrRecordNotFound
		}

		if err := tx.Model(&model.File{}).
			Where("folder_id IN ?", folderIDs).
			Updates(map[string]any{
				"status":     model.ResourceStatusDeleted,
				"deleted_at": now,
				"updated_at": now,
			}).Error; err != nil {
			return fmt.Errorf("delete folder tree files: %w", err)
		}

		rootUpdates := map[string]any{
			"status":     model.ResourceStatusDeleted,
			"deleted_at": now,
			"updated_at": now,
		}
		if strings.TrimSpace(rootSourcePath) != "" {
			rootUpdates["source_path"] = rootSourcePath
		}

		result := tx.Model(&model.Folder{}).Where("id = ?", rootFolderID).Updates(rootUpdates)
		if result.Error != nil {
			return fmt.Errorf("delete root folder: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		if len(folderIDs) > 1 {
			if err := tx.Model(&model.Folder{}).
				Where("id IN ?", folderIDs[1:]).
				Updates(map[string]any{
					"status":     model.ResourceStatusDeleted,
					"deleted_at": now,
					"updated_at": now,
				}).Error; err != nil {
				return fmt.Errorf("delete child folders: %w", err)
			}
		}

		return createOperationLogTx(tx, logID, operatorID, "resource_deleted", "folder", rootFolderID, detail, operatorIP, now)
	})
}
