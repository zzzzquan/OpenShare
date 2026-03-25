package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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
			ID          string
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
			ID          string
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

// SearchCandidateQuery encapsulates parameters for LIKE-based candidate recall.
type SearchCandidateQuery struct {
	FullQuery      string
	Terms          []string
	ScopeFolderIDs []string
	Limit          int
}

// SearchCandidate is a hydrated search row used for application-side ranking.
type SearchCandidate struct {
	EntityType    string
	ID            string
	Name          string
	OriginalName  string
	Description   string
	Extension     string
	Size          int64
	DownloadCount int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
	FolderID      *string
	ParentID      *string
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

// SearchCandidates recalls active file and folder candidates using parameterized
// LIKE matching. It returns raw candidates for service-layer ranking.
func (r *SearchRepository) SearchCandidates(ctx context.Context, query SearchCandidateQuery) ([]SearchCandidate, int64, error) {
	files, fileTotal, err := r.searchFilesForCandidates(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	folders, folderTotal, err := r.searchFoldersForCandidates(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	candidates := make([]SearchCandidate, 0, len(files)+len(folders))
	for i := range files {
		file := files[i]
		candidates = append(candidates, SearchCandidate{
			EntityType:    "file",
			ID:            file.ID,
			Name:          file.Title,
			OriginalName:  file.OriginalName,
			Description:   file.Description,
			Extension:     file.Extension,
			Size:          file.Size,
			DownloadCount: file.DownloadCount,
			CreatedAt:     file.CreatedAt,
			UpdatedAt:     file.UpdatedAt,
			FolderID:      file.FolderID,
		})
	}

	for i := range folders {
		folder := folders[i]
		candidates = append(candidates, SearchCandidate{
			EntityType:    "folder",
			ID:            folder.ID,
			Name:          folder.Name,
			Description:   folder.Description,
			DownloadCount: folder.DownloadCount,
			CreatedAt:     folder.CreatedAt,
			UpdatedAt:     folder.UpdatedAt,
			ParentID:      folder.ParentID,
		})
	}

	return candidates, fileTotal + folderTotal, nil
}

func (r *SearchRepository) searchFilesForCandidates(ctx context.Context, query SearchCandidateQuery) ([]model.File, int64, error) {
	db := r.db.WithContext(ctx).
		Model(&model.File{}).
		Where("status = ? AND deleted_at IS NULL", model.ResourceStatusActive)

	if query.ScopeFolderIDs != nil {
		db = db.Where("folder_id IN ?", query.ScopeFolderIDs)
	}

	db = applySearchTermFilters(db, []string{"title", "original_name", "description"}, query.Terms)

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count file search candidates: %w", err)
	}
	if total == 0 {
		return nil, 0, nil
	}

	var files []model.File
	findDB := applyCandidateOrder(db, []string{"title", "original_name"}, "description", "download_count", "updated_at", query.FullQuery)
	if query.Limit > 0 {
		findDB = findDB.Limit(query.Limit)
	}
	if err := findDB.Find(&files).Error; err != nil {
		return nil, 0, fmt.Errorf("load file search candidates: %w", err)
	}

	return files, total, nil
}

func (r *SearchRepository) searchFoldersForCandidates(ctx context.Context, query SearchCandidateQuery) ([]model.Folder, int64, error) {
	db := r.db.WithContext(ctx).
		Model(&model.Folder{}).
		Where("status = ? AND deleted_at IS NULL", model.ResourceStatusActive)

	if query.ScopeFolderIDs != nil {
		db = db.Where("id IN ?", query.ScopeFolderIDs)
	}

	db = applySearchTermFilters(db, []string{"name", "description"}, query.Terms)

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count folder search candidates: %w", err)
	}
	if total == 0 {
		return nil, 0, nil
	}

	var folders []model.Folder
	findDB := applyCandidateOrder(db, []string{"name"}, "description", "download_count", "updated_at", query.FullQuery)
	if query.Limit > 0 {
		findDB = findDB.Limit(query.Limit)
	}
	if err := findDB.Find(&folders).Error; err != nil {
		return nil, 0, fmt.Errorf("load folder search candidates: %w", err)
	}

	return folders, total, nil
}

func applySearchTermFilters(db *gorm.DB, fields []string, terms []string) *gorm.DB {
	for _, term := range terms {
		if strings.TrimSpace(term) == "" {
			continue
		}

		pattern := containsLikePattern(term)
		conditions := make([]string, 0, len(fields))
		args := make([]any, 0, len(fields))
		for _, field := range fields {
			conditions = append(conditions, fmt.Sprintf("LOWER(%s) LIKE ? ESCAPE '\\'", field))
			args = append(args, pattern)
		}
		db = db.Where("("+strings.Join(conditions, " OR ")+")", args...)
	}

	return db
}

func applyCandidateOrder(db *gorm.DB, primaryFields []string, descriptionField, downloadField, updatedField, fullQuery string) *gorm.DB {
	if strings.TrimSpace(fullQuery) == "" {
		return db.Order(downloadField + " DESC").Order(updatedField + " DESC")
	}

	equalConditions := make([]string, 0, len(primaryFields))
	prefixConditions := make([]string, 0, len(primaryFields))
	containsConditions := make([]string, 0, len(primaryFields))
	args := make([]any, 0, len(primaryFields)*3+1)

	for range primaryFields {
		args = append(args, fullQuery)
	}
	for _, field := range primaryFields {
		equalConditions = append(equalConditions, fmt.Sprintf("LOWER(%s) = ?", field))
	}

	prefixPattern := prefixLikePattern(fullQuery)
	for _, field := range primaryFields {
		prefixConditions = append(prefixConditions, fmt.Sprintf("LOWER(%s) LIKE ? ESCAPE '\\'", field))
		args = append(args, prefixPattern)
	}

	containsPattern := containsLikePattern(fullQuery)
	for _, field := range primaryFields {
		containsConditions = append(containsConditions, fmt.Sprintf("LOWER(%s) LIKE ? ESCAPE '\\'", field))
		args = append(args, containsPattern)
	}

	descriptionPattern := containsLikePattern(fullQuery)
	args = append(args, descriptionPattern)

	sql := fmt.Sprintf(`
CASE
	WHEN %s THEN 0
	WHEN %s THEN 1
	WHEN %s THEN 2
	WHEN LOWER(%s) LIKE ? ESCAPE '\' THEN 3
	ELSE 4
END
`, strings.Join(equalConditions, " OR "), strings.Join(prefixConditions, " OR "), strings.Join(containsConditions, " OR "), descriptionField)

	return db.
		Order(clause.Expr{SQL: sql, Vars: args}).
		Order(downloadField + " DESC").
		Order(updatedField + " DESC")
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

func containsLikePattern(s string) string {
	return "%" + escapeLike(strings.ToLower(strings.TrimSpace(s))) + "%"
}

func prefixLikePattern(s string) string {
	return escapeLike(strings.ToLower(strings.TrimSpace(s))) + "%"
}

// makePlaceholders returns "?,?,?" with n question marks.
func makePlaceholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat("?,", n-1) + "?"
}
