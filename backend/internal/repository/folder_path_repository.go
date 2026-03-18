package repository

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"

	"openshare/backend/internal/model"
	"openshare/backend/pkg/identity"
)

func EnsureActiveFolderPathTx(tx *gorm.DB, rootFolder *model.Folder, relativePath string, now time.Time) (*model.Folder, error) {
	current := rootFolder
	normalized := NormalizeRelativePathForStorage(relativePath)
	if normalized == "" {
		return current, nil
	}

	for _, segment := range strings.Split(normalized, "/") {
		var child model.Folder
		err := tx.
			Where("parent_id = ? AND status = ? AND LOWER(name) = LOWER(?)", current.ID, model.ResourceStatusActive, segment).
			Take(&child).
			Error
		if err == nil {
			current = &child
			continue
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("find child folder: %w", err)
		}

		id, idErr := identity.NewID()
		if idErr != nil {
			return nil, fmt.Errorf("generate folder id: %w", idErr)
		}
		sourcePath := filepath.Join(strings.TrimSpace(derefString(current.SourcePath)), segment)
		child = model.Folder{
			ID:          id,
			ParentID:    &current.ID,
			SourcePath:  stringPtr(sourcePath),
			Name:        segment,
			Description: "",
			Status:      model.ResourceStatusActive,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := tx.Create(&child).Error; err != nil {
			return nil, fmt.Errorf("create child folder: %w", err)
		}
		current = &child
	}

	return current, nil
}

func NormalizeRelativePathForStorage(value string) string {
	value = filepath.ToSlash(strings.TrimSpace(value))
	value = strings.Trim(value, "/")
	if value == "" || value == "." {
		return ""
	}
	parts := strings.Split(value, "/")
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." || part == ".." {
			continue
		}
		cleaned = append(cleaned, part)
	}
	return strings.Join(cleaned, "/")
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func stringPtr(value string) *string {
	copied := value
	return &copied
}
