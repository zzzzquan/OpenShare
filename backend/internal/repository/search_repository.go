package repository

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"openshare/backend/internal/model"
)

// SearchRepository handles FTS5 index operations and search queries.
type SearchRepository struct {
	db *gorm.DB
}

func NewSearchRepository(db *gorm.DB) *SearchRepository {
	return &SearchRepository{db: db}
}

// ---------------------------------------------------------------------------
// Index sync helpers
// ---------------------------------------------------------------------------

// UpsertFileIndex removes any existing entry for the file and inserts a fresh
// row into the FTS5 search_index. Call this whenever a file's title changes.
func (r *SearchRepository) UpsertFileIndex(ctx context.Context, fileID, title, description string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(
			"DELETE FROM search_index WHERE entity_type = 'file' AND entity_id = ?", fileID,
		).Error; err != nil {
			return fmt.Errorf("delete old file index: %w", err)
		}
		if err := tx.Exec(
			"INSERT INTO search_index(entity_type, entity_id, name) VALUES ('file', ?, ?)",
			fileID, strings.ToLower(strings.TrimSpace(title+" "+description)),
		).Error; err != nil {
			return fmt.Errorf("insert file index: %w", err)
		}
		return nil
	})
}

// UpsertFolderIndex removes any existing entry for the folder and inserts a
// fresh row into the FTS5 search_index.
func (r *SearchRepository) UpsertFolderIndex(ctx context.Context, folderID, name, description string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(
			"DELETE FROM search_index WHERE entity_type = 'folder' AND entity_id = ?", folderID,
		).Error; err != nil {
			return fmt.Errorf("delete old folder index: %w", err)
		}
		if err := tx.Exec(
			"INSERT INTO search_index(entity_type, entity_id, name) VALUES ('folder', ?, ?)",
			folderID, strings.ToLower(strings.TrimSpace(name+" "+description)),
		).Error; err != nil {
			return fmt.Errorf("insert folder index: %w", err)
		}
		return nil
	})
}

// RemoveIndex removes an entity from the FTS5 index.
func (r *SearchRepository) RemoveIndex(ctx context.Context, entityType, entityID string) error {
	return r.db.WithContext(ctx).Exec(
		"DELETE FROM search_index WHERE entity_type = ? AND entity_id = ?",
		entityType, entityID,
	).Error
}

// ---------------------------------------------------------------------------
// Full rebuild
// ---------------------------------------------------------------------------

// RebuildAllIndexes drops all entries and re-indexes every active file and
// folder. This is an admin/maintenance operation.
func (r *SearchRepository) RebuildAllIndexes(ctx context.Context) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Clear entire index
		if err := tx.Exec("DELETE FROM search_index").Error; err != nil {
			return fmt.Errorf("clear search_index: %w", err)
		}

		// Index all active files
		var files []struct {
			ID    string
			Title       string
			Description string
		}
		if err := tx.Model(&model.File{}).
			Select("id, title, description").
			Where("status = ? AND deleted_at IS NULL", model.ResourceStatusActive).
			Find(&files).Error; err != nil {
			return fmt.Errorf("load files for indexing: %w", err)
		}

		for _, f := range files {
			if err := tx.Exec(
				"INSERT INTO search_index(entity_type, entity_id, name) VALUES ('file', ?, ?)",
				f.ID, strings.ToLower(strings.TrimSpace(f.Title+" "+f.Description)),
			).Error; err != nil {
				return fmt.Errorf("index file %s: %w", f.ID, err)
			}
		}

		// Index all active folders
		var folders []struct {
			ID   string
			Name        string
			Description string
		}
		if err := tx.Model(&model.Folder{}).
			Select("id, name, description").
			Where("status = ? AND deleted_at IS NULL", model.ResourceStatusActive).
			Find(&folders).Error; err != nil {
			return fmt.Errorf("load folders for indexing: %w", err)
		}

		for _, f := range folders {
			if err := tx.Exec(
				"INSERT INTO search_index(entity_type, entity_id, name) VALUES ('folder', ?, ?)",
				f.ID, strings.ToLower(strings.TrimSpace(f.Name+" "+f.Description)),
			).Error; err != nil {
				return fmt.Errorf("index folder %s: %w", f.ID, err)
			}
		}

		return nil
	})
}

// ---------------------------------------------------------------------------
// Search queries
// ---------------------------------------------------------------------------

// SearchQuery encapsulates all parameters for a search request.
type SearchQuery struct {
	// FTS5Query is the sanitized MATCH expression (prefix-match tokens).
	FTS5Query string
	// ScopeFolderIDs, if non-nil, restricts files to those in one of these
	// folders AND restricts folder results to these IDs. Computed by the
	// service layer from a folder-scope request.
	ScopeFolderIDs []string
	// Offset and Limit for pagination.
	Offset int
	Limit  int
}

// SearchResultRow is a single row returned by the search query.
type SearchResultRow struct {
	EntityType string  `json:"entity_type"`
	EntityID   string  `json:"entity_id"`
	Rank       float64 `json:"-"`
}

// Search runs a full-text search with optional folder filtering.
// Results are ordered by FTS5 relevance rank (lower is better).
func (r *SearchRepository) Search(ctx context.Context, query SearchQuery) ([]SearchResultRow, int64, error) {
	if query.FTS5Query == "" {
		return nil, 0, nil
	}

	var conditions []string
	var args []any

	if query.FTS5Query != "" {
		conditions = append(conditions, "search_index MATCH ?")
		args = append(args, query.FTS5Query)
	}

	whereClause := strings.Join(conditions, " AND ")

	// Build scope filter on active entities, optionally limited to folder subtree.
	var scopeFilter string
	if query.ScopeFolderIDs != nil {
		placeholders := makePlaceholders(len(query.ScopeFolderIDs))
		for _, id := range query.ScopeFolderIDs {
			args = append(args, id)
		}
		scopeFilter = fmt.Sprintf(`
		  AND (
		    (search_index.entity_type = 'file' AND search_index.entity_id IN (
		      SELECT id FROM files WHERE folder_id IN (%s) AND status = 'active' AND deleted_at IS NULL
		    ))
		    OR
		    (search_index.entity_type = 'folder' AND search_index.entity_id IN (%s))
		  )`, placeholders, placeholders)
		// Need to pass folder IDs again for the second IN clause
		for _, id := range query.ScopeFolderIDs {
			args = append(args, id)
		}
	} else {
		scopeFilter = `
		  AND (
		    (search_index.entity_type = 'file' AND search_index.entity_id IN (
		      SELECT id FROM files WHERE status = 'active' AND deleted_at IS NULL
		    ))
		    OR
		    (search_index.entity_type = 'folder' AND search_index.entity_id IN (
		      SELECT id FROM folders WHERE status = 'active' AND deleted_at IS NULL
		    ))
		  )`
	}

	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM search_index WHERE %s %s", whereClause, scopeFilter)
	// For count we need same args minus limit/offset
	countArgs := make([]any, len(args))
	copy(countArgs, args)

	var total int64
	if err := r.db.WithContext(ctx).Raw(countSQL, countArgs...).Scan(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count search results: %w", err)
	}
	if total == 0 {
		return nil, 0, nil
	}

	fetchSQL := fmt.Sprintf(`
		SELECT search_index.entity_type, search_index.entity_id, rank
		FROM search_index
		WHERE %s %s
		ORDER BY rank
		LIMIT ? OFFSET ?
	`, whereClause, scopeFilter)
	args = append(args, query.Limit, query.Offset)

	var rows []SearchResultRow
	if err := r.db.WithContext(ctx).Raw(fetchSQL, args...).Scan(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("search query: %w", err)
	}

	return rows, total, nil
}

// SearchWithLike provides a LIKE-based fallback when FTS5 returns no results
// or for very short queries.
func (r *SearchRepository) SearchWithLike(ctx context.Context, keyword string, scopeFolderIDs []string, offset, limit int) ([]SearchResultRow, int64, error) {
	if keyword == "" && scopeFolderIDs == nil {
		return nil, 0, nil
	}

	var fileCondParts []string
	var fileArgs []any
	fileCondParts = append(fileCondParts, "f.status = 'active'", "f.deleted_at IS NULL")

	if keyword != "" {
		likePattern := "%" + escapeLike(strings.ToLower(keyword)) + "%"
		fileCondParts = append(fileCondParts, "(LOWER(f.title) LIKE ? OR LOWER(f.original_name) LIKE ?)")
		fileArgs = append(fileArgs, likePattern, likePattern)
	}
	if scopeFolderIDs != nil {
		fileCondParts = append(fileCondParts, "f.folder_id IN ("+makePlaceholders(len(scopeFolderIDs))+")")
		for _, id := range scopeFolderIDs {
			fileArgs = append(fileArgs, id)
		}
	}
	fileConds := strings.Join(fileCondParts, " AND ")

	var folderCondParts []string
	var folderArgs []any
	folderCondParts = append(folderCondParts, "d.status = 'active'", "d.deleted_at IS NULL")

	if keyword != "" {
		likePattern := "%" + escapeLike(strings.ToLower(keyword)) + "%"
		folderCondParts = append(folderCondParts, "LOWER(d.name) LIKE ?")
		folderArgs = append(folderArgs, likePattern)
	}
	if scopeFolderIDs != nil {
		folderCondParts = append(folderCondParts, "d.id IN ("+makePlaceholders(len(scopeFolderIDs))+")")
		for _, id := range scopeFolderIDs {
			folderArgs = append(folderArgs, id)
		}
	}
	folderConds := strings.Join(folderCondParts, " AND ")

	// Count
	countSQL := fmt.Sprintf(`
		SELECT (SELECT COUNT(*) FROM files f WHERE %s)
		     + (SELECT COUNT(*) FROM folders d WHERE %s)
	`, fileConds, folderConds)
	countArgs := append(append([]any{}, fileArgs...), folderArgs...)
	var total int64
	if err := r.db.WithContext(ctx).Raw(countSQL, countArgs...).Scan(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("like count: %w", err)
	}
	if total == 0 {
		return nil, 0, nil
	}

	// Results — files sorted by download_count desc, then folders
	fetchSQL := fmt.Sprintf(`
		SELECT * FROM (
			SELECT 'file' AS entity_type, f.id AS entity_id, -f.download_count AS rank
			FROM files f WHERE %s
			UNION ALL
			SELECT 'folder' AS entity_type, d.id AS entity_id, 0 AS rank
			FROM folders d WHERE %s
		) ORDER BY rank
		LIMIT ? OFFSET ?
	`, fileConds, folderConds)
	fetchArgs := append(append([]any{}, fileArgs...), folderArgs...)
	fetchArgs = append(fetchArgs, limit, offset)

	var rows []SearchResultRow
	if err := r.db.WithContext(ctx).Raw(fetchSQL, fetchArgs...).Scan(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("like search: %w", err)
	}

	return rows, total, nil
}

// GetFilesByIDs loads file metadata for a list of IDs, preserving order.
func (r *SearchRepository) GetFilesByIDs(ctx context.Context, ids []string) ([]model.File, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var files []model.File
	if err := r.db.WithContext(ctx).
		Where("id IN ?", ids).
		Find(&files).Error; err != nil {
		return nil, fmt.Errorf("get files by ids: %w", err)
	}
	return files, nil
}

// GetFoldersByIDs loads folder metadata for a list of IDs.
func (r *SearchRepository) GetFoldersByIDs(ctx context.Context, ids []string) ([]model.Folder, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var folders []model.Folder
	if err := r.db.WithContext(ctx).
		Where("id IN ?", ids).
		Find(&folders).Error; err != nil {
		return nil, fmt.Errorf("get folders by ids: %w", err)
	}
	return folders, nil
}

// GetDescendantFolderIDs returns the given folderID plus all its descendants.
func (r *SearchRepository) GetDescendantFolderIDs(ctx context.Context, folderID string) ([]string, error) {
	result := []string{folderID}
	queue := []string{folderID}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		var childIDs []string
		if err := r.db.WithContext(ctx).
			Model(&model.Folder{}).
			Where("parent_id = ? AND status = ? AND deleted_at IS NULL", current, model.ResourceStatusActive).
			Pluck("id", &childIDs).Error; err != nil {
			return nil, fmt.Errorf("get child folders: %w", err)
		}
		result = append(result, childIDs...)
		queue = append(queue, childIDs...)
	}
	return result, nil
}

// escapeLike escapes LIKE special characters.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// makePlaceholders returns "?,?,?" with n question marks.
func makePlaceholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat("?,", n-1) + "?"
}
