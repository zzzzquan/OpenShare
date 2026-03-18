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

type ImportRepository struct {
	db *gorm.DB
}

var ErrManagedRootRequired = errors.New("managed root folder required")

type FolderTreeFolderRow struct {
	ID         string
	ParentID   *string
	Name       string
	SourcePath *string
	Status     model.ResourceStatus
}

type FolderTreeFileRow struct {
	ID            string
	FolderID      *string
	Title         string
	OriginalName  string
	Status        model.ResourceStatus
	Size          int64
	DownloadCount int64
}

func NewImportRepository(db *gorm.DB) *ImportRepository {
	return &ImportRepository{db: db}
}

func (r *ImportRepository) FindFolderBySourcePath(ctx context.Context, sourcePath string) (*model.Folder, error) {
	var folder model.Folder
	err := r.db.WithContext(ctx).Where("source_path = ?", sourcePath).Take(&folder).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find folder by source path: %w", err)
	}
	return &folder, nil
}

func (r *ImportRepository) FindFileBySourcePath(ctx context.Context, sourcePath string) (*model.File, error) {
	var file model.File
	err := r.db.WithContext(ctx).Where("source_path = ?", sourcePath).Take(&file).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find file by source path: %w", err)
	}
	return &file, nil
}

func (r *ImportRepository) FolderNameExists(ctx context.Context, parentID *string, name string) (bool, error) {
	query := r.db.WithContext(ctx).Model(&model.Folder{}).Where("LOWER(name) = LOWER(?)", name)
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

func (r *ImportRepository) FileNameExists(ctx context.Context, folderID *string, name string) (bool, error) {
	query := r.db.WithContext(ctx).Model(&model.File{}).Where("LOWER(original_name) = LOWER(?)", name)
	if folderID == nil {
		query = query.Where("folder_id IS NULL")
	} else {
		query = query.Where("folder_id = ?", *folderID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, fmt.Errorf("check file name conflict: %w", err)
	}
	return count > 0, nil
}

func (r *ImportRepository) CreateFolder(ctx context.Context, folder *model.Folder) error {
	return r.db.WithContext(ctx).Create(folder).Error
}

func (r *ImportRepository) CreateFile(ctx context.Context, file *model.File) error {
	return r.db.WithContext(ctx).Create(file).Error
}

func (r *ImportRepository) LogOperation(ctx context.Context, adminID, action, targetType, targetID, detail, ip string, createdAt time.Time) error {
	logID, err := identity.NewID()
	if err != nil {
		return fmt.Errorf("generate operation log id: %w", err)
	}
	var adminRef *string
	if strings.TrimSpace(adminID) != "" {
		adminRef = &adminID
	}
	entry := &model.OperationLog{
		ID:         logID,
		AdminID:    adminRef,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Detail:     detail,
		IP:         ip,
		CreatedAt:  createdAt,
	}
	return r.db.WithContext(ctx).Create(entry).Error
}

func (r *ImportRepository) ListFolders(ctx context.Context) ([]FolderTreeFolderRow, error) {
	var rows []FolderTreeFolderRow
	err := r.db.WithContext(ctx).
		Model(&model.Folder{}).
		Select("id, parent_id, name, source_path, status").
		Order("name ASC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list folders: %w", err)
	}
	return rows, nil
}

func (r *ImportRepository) ListFiles(ctx context.Context) ([]FolderTreeFileRow, error) {
	var rows []FolderTreeFileRow
	err := r.db.WithContext(ctx).
		Model(&model.File{}).
		Select("id, folder_id, title, original_name, status, size, download_count").
		Order("title ASC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}
	return rows, nil
}

func (r *ImportRepository) FindFolderByID(ctx context.Context, folderID string) (*model.Folder, error) {
	var folder model.Folder
	err := r.db.WithContext(ctx).Where("id = ?", folderID).Take(&folder).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find folder by id: %w", err)
	}
	return &folder, nil
}

func (r *ImportRepository) DeleteManagedRootWithLog(ctx context.Context, rootFolderID, operatorID, operatorIP, detail, logID string, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var folders []FolderTreeFolderRow
		if err := tx.Model(&model.Folder{}).
			Select("id, parent_id, name, source_path, status").
			Find(&folders).Error; err != nil {
			return fmt.Errorf("list folders for deletion: %w", err)
		}

		childrenByParent := make(map[string][]string)
		folderByID := make(map[string]FolderTreeFolderRow, len(folders))
		for _, folder := range folders {
			folderByID[folder.ID] = folder
			if folder.ParentID != nil {
				childrenByParent[*folder.ParentID] = append(childrenByParent[*folder.ParentID], folder.ID)
			}
		}

		root, ok := folderByID[rootFolderID]
		if !ok {
			return gorm.ErrRecordNotFound
		}

		if root.ParentID != nil {
			return ErrManagedRootRequired
		}

		folderIDs := []string{rootFolderID}
		queue := []string{rootFolderID}
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			children := childrenByParent[current]
			folderIDs = append(folderIDs, children...)
			queue = append(queue, children...)
		}

		var fileIDs []string
		if err := tx.Model(&model.File{}).Where("folder_id IN ?", folderIDs).Pluck("id", &fileIDs).Error; err != nil {
			return fmt.Errorf("list files for deletion: %w", err)
		}

		if len(fileIDs) > 0 {
			if err := tx.Where("file_id IN ?", fileIDs).Delete(&model.DownloadEvent{}).Error; err != nil {
				return fmt.Errorf("delete download events: %w", err)
			}
			if err := tx.Where("file_id IN ?", fileIDs).Delete(&model.Report{}).Error; err != nil {
				return fmt.Errorf("delete file reports: %w", err)
			}
			if err := tx.Where("folder_id IN ?", folderIDs).Delete(&model.File{}).Error; err != nil {
				return fmt.Errorf("delete files: %w", err)
			}
		}

		if err := tx.Where("folder_id IN ?", folderIDs).Delete(&model.Report{}).Error; err != nil {
			return fmt.Errorf("delete folder reports: %w", err)
		}
		if err := tx.Where("id IN ?", folderIDs).Delete(&model.Folder{}).Error; err != nil {
			return fmt.Errorf("delete folders: %w", err)
		}

		return createOperationLogTx(tx, logID, operatorID, "managed_directory_deleted", "folder", rootFolderID, detail, operatorIP, now)
	})
}
