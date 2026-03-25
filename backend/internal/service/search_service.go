package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"openshare/backend/internal/config"
	"openshare/backend/internal/repository"
)

// ---------------------------------------------------------------------------
// Sentinel errors
// ---------------------------------------------------------------------------

var (
	ErrSearchQueryEmpty   = errors.New("search query is empty")
	ErrSearchQueryTooLong = errors.New("search query exceeds maximum length")
	ErrSearchInvalidInput = errors.New("invalid search parameters")
)

const (
	defaultSearchPage     = 1
	defaultSearchPageSize = 20
	maxSearchPageSize     = 100
	maxSearchQueryLength  = 200
	maxSearchTerms        = 8
	maxCandidateLimit     = 300
)

// ---------------------------------------------------------------------------
// Service
// ---------------------------------------------------------------------------

// SearchService implements public search over ordinary resource tables:
//   - parameterized LIKE recall over files/folders
//   - optional folder-scoped search
//   - application-side relevance ranking
type SearchService struct {
	searchRepo *repository.SearchRepository
	settings   *SystemSettingService
}

func NewSearchService(searchRepo *repository.SearchRepository, settings *SystemSettingService) *SearchService {
	return &SearchService{
		searchRepo: searchRepo,
		settings:   settings,
	}
}

// ---------------------------------------------------------------------------
// Input / Output
// ---------------------------------------------------------------------------

// SearchInput is the external request from the handler layer.
type SearchInput struct {
	Keyword  string // raw user input
	FolderID string // optional folder scope
	Page     int
	PageSize int
}

// SearchResult is the response delivered to the handler layer.
type SearchResult struct {
	Items    []SearchResultItem `json:"items"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
	Total    int64              `json:"total"`
}

// SearchResultItem represents a single file or folder in the search results.
type SearchResultItem struct {
	EntityType    string     `json:"entity_type"` // "file" | "folder"
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	OriginalName  string     `json:"original_name,omitempty"`
	Extension     string     `json:"extension,omitempty"`
	Size          int64      `json:"size,omitempty"`
	DownloadCount int64      `json:"download_count,omitempty"`
	UploadedAt    *time.Time `json:"uploaded_at,omitempty"`
}

// ---------------------------------------------------------------------------
// Core search
// ---------------------------------------------------------------------------

func (s *SearchService) Search(ctx context.Context, input SearchInput) (*SearchResult, error) {
	policy := s.searchPolicy(ctx)

	// --- 1. Validate & normalise -----------------------------------------
	page, pageSize, err := normalizeSearchPagination(input.Page, input.PageSize, policy.ResultWindow)
	if err != nil {
		return nil, err
	}

	normalizedQuery, err := normalizeSearchKeyword(input.Keyword, policy.EnableFuzzyMatch)
	if err != nil {
		return nil, err
	}

	scopeFolderID := strings.TrimSpace(input.FolderID)

	// --- 2. Resolve folder scope -----------------------------------------
	var scopeFolderIDs []string
	if scopeFolderID != "" {
		if !policy.EnableFolderScope {
			return nil, ErrSearchInvalidInput
		}
		ids, err := s.searchRepo.GetDescendantFolderIDs(ctx, scopeFolderID)
		if err != nil {
			return nil, fmt.Errorf("resolve folder scope: %w", err)
		}
		scopeFolderIDs = ids
	}

	// --- 3. Recall candidates from ordinary tables -----------------------
	candidates, total, err := s.searchRepo.SearchCandidates(ctx, repository.SearchCandidateQuery{
		FullQuery:      normalizedQuery.Full,
		Terms:          normalizedQuery.Terms,
		ScopeFolderIDs: scopeFolderIDs,
		Limit:          searchCandidateLimit(policy.ResultWindow, page, pageSize),
	})
	if err != nil {
		return nil, fmt.Errorf("search candidates: %w", err)
	}

	if total == 0 {
		return &SearchResult{
			Items:    []SearchResultItem{},
			Page:     page,
			PageSize: pageSize,
			Total:    0,
		}, nil
	}

	// --- 4. Rank, paginate, and shape response ---------------------------
	ranked := rankSearchCandidates(candidates, normalizedQuery, scopeFolderID)
	offset := (page - 1) * pageSize
	if offset >= len(ranked) {
		return &SearchResult{
			Items:    []SearchResultItem{},
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		}, nil
	}

	end := offset + pageSize
	if end > len(ranked) {
		end = len(ranked)
	}

	items := make([]SearchResultItem, 0, end-offset)
	for _, candidate := range ranked[offset:end] {
		items = append(items, candidateToResultItem(candidate.Candidate))
	}

	return &SearchResult{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

func (s *SearchService) searchPolicy(ctx context.Context) SearchPolicy {
	if s.settings == nil {
		return defaultSystemPolicy(config.UploadConfig{}).Search
	}

	policy, err := s.settings.GetPolicy(ctx)
	if err != nil || policy == nil {
		return defaultSystemPolicy(config.UploadConfig{}).Search
	}
	return policy.Search
}

// ---------------------------------------------------------------------------
// Ranking helpers
// ---------------------------------------------------------------------------

type normalizedSearchQuery struct {
	Full  string
	Terms []string
}

type scoredSearchCandidate struct {
	Candidate repository.SearchCandidate
	Score     int
}

func normalizeSearchKeyword(raw string, enableFuzzy bool) (normalizedSearchQuery, error) {
	trimmed := strings.TrimSpace(raw)
	if len([]rune(trimmed)) > maxSearchQueryLength {
		return normalizedSearchQuery{}, ErrSearchQueryTooLong
	}

	full := collapseSearchWhitespace(strings.ToLower(trimmed))
	if full == "" {
		return normalizedSearchQuery{}, ErrSearchQueryEmpty
	}

	terms := []string{full}
	if enableFuzzy {
		terms = splitSearchTerms(full)
		if len(terms) == 0 {
			terms = []string{full}
		}
	}

	return normalizedSearchQuery{
		Full:  full,
		Terms: terms,
	}, nil
}

func splitSearchTerms(full string) []string {
	fields := strings.Fields(full)
	terms := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		term := strings.TrimSpace(field)
		if term == "" {
			continue
		}
		if _, exists := seen[term]; exists {
			continue
		}
		seen[term] = struct{}{}
		terms = append(terms, term)
		if len(terms) >= maxSearchTerms {
			break
		}
	}
	return terms
}

func collapseSearchWhitespace(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))

	lastWasSpace := true
	for _, r := range value {
		if unicode.IsSpace(r) {
			if !lastWasSpace {
				builder.WriteByte(' ')
			}
			lastWasSpace = true
			continue
		}
		builder.WriteRune(r)
		lastWasSpace = false
	}

	return strings.TrimSpace(builder.String())
}

func searchCandidateLimit(resultWindow, page, pageSize int) int {
	candidateLimit := page * pageSize * 4
	if candidateLimit < 120 {
		candidateLimit = 120
	}
	if resultWindow > 0 && resultWindow*3 > candidateLimit {
		candidateLimit = resultWindow * 3
	}
	if candidateLimit > maxCandidateLimit {
		candidateLimit = maxCandidateLimit
	}
	return candidateLimit
}

func rankSearchCandidates(candidates []repository.SearchCandidate, query normalizedSearchQuery, scopeFolderID string) []scoredSearchCandidate {
	ranked := make([]scoredSearchCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		ranked = append(ranked, scoredSearchCandidate{
			Candidate: candidate,
			Score:     scoreSearchCandidate(candidate, query, scopeFolderID),
		})
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		left := ranked[i]
		right := ranked[j]

		if left.Score != right.Score {
			return left.Score > right.Score
		}
		if left.Candidate.DownloadCount != right.Candidate.DownloadCount {
			return left.Candidate.DownloadCount > right.Candidate.DownloadCount
		}
		if !left.Candidate.UpdatedAt.Equal(right.Candidate.UpdatedAt) {
			return left.Candidate.UpdatedAt.After(right.Candidate.UpdatedAt)
		}
		if left.Candidate.EntityType != right.Candidate.EntityType {
			return left.Candidate.EntityType == "folder"
		}

		leftName := searchDisplayName(left.Candidate)
		rightName := searchDisplayName(right.Candidate)
		if leftName != rightName {
			return leftName < rightName
		}
		return left.Candidate.ID < right.Candidate.ID
	})

	return ranked
}

func scoreSearchCandidate(candidate repository.SearchCandidate, query normalizedSearchQuery, scopeFolderID string) int {
	primaryFields := []string{normalizeSearchField(candidate.Name)}
	if original := normalizeSearchField(candidate.OriginalName); original != "" {
		primaryFields = append(primaryFields, original)
	}
	description := normalizeSearchField(candidate.Description)

	score := bestFieldMatchScore(query.Full, primaryFields, 1200, 920, 720)
	if description != "" && strings.Contains(description, query.Full) {
		score += 120
	}

	if len(query.Terms) > 1 {
		for _, term := range query.Terms {
			score += bestFieldMatchScore(term, primaryFields, 200, 150, 90)
			if description != "" && strings.Contains(description, term) {
				score += 25
			}
		}
	}

	score += scopeBias(candidate, scopeFolderID)
	score += downloadCountBias(candidate.DownloadCount)
	return score
}

func bestFieldMatchScore(term string, fields []string, exactScore, prefixScore, containsScore int) int {
	if term == "" {
		return 0
	}

	best := 0
	for _, field := range fields {
		switch {
		case field == term:
			if exactScore > best {
				best = exactScore
			}
		case strings.HasPrefix(field, term):
			if prefixScore > best {
				best = prefixScore
			}
		case strings.Contains(field, term):
			if containsScore > best {
				best = containsScore
			}
		}
	}
	return best
}

func normalizeSearchField(value string) string {
	return collapseSearchWhitespace(strings.ToLower(strings.TrimSpace(value)))
}

func scopeBias(candidate repository.SearchCandidate, scopeFolderID string) int {
	if scopeFolderID == "" {
		return 0
	}

	switch candidate.EntityType {
	case "file":
		if candidate.FolderID != nil && *candidate.FolderID == scopeFolderID {
			return 120
		}
	case "folder":
		if candidate.ID == scopeFolderID {
			return 100
		}
		if candidate.ParentID != nil && *candidate.ParentID == scopeFolderID {
			return 80
		}
	}

	return 0
}

func downloadCountBias(downloadCount int64) int {
	switch {
	case downloadCount >= 100:
		return 20
	case downloadCount >= 50:
		return 16
	case downloadCount >= 20:
		return 12
	case downloadCount >= 10:
		return 8
	case downloadCount > 0:
		return int(downloadCount)
	default:
		return 0
	}
}

func searchDisplayName(candidate repository.SearchCandidate) string {
	if candidate.EntityType == "file" && strings.TrimSpace(candidate.OriginalName) != "" {
		return strings.ToLower(candidate.OriginalName)
	}
	return strings.ToLower(candidate.Name)
}

func candidateToResultItem(candidate repository.SearchCandidate) SearchResultItem {
	switch candidate.EntityType {
	case "file":
		uploadedAt := candidate.CreatedAt
		return SearchResultItem{
			EntityType:    "file",
			ID:            candidate.ID,
			Name:          candidate.Name,
			OriginalName:  candidate.OriginalName,
			Extension:     candidate.Extension,
			Size:          candidate.Size,
			DownloadCount: candidate.DownloadCount,
			UploadedAt:    &uploadedAt,
		}
	default:
		return SearchResultItem{
			EntityType: "folder",
			ID:         candidate.ID,
			Name:       candidate.Name,
		}
	}
}

// ---------------------------------------------------------------------------
// Index sync public API (called by other services after mutations)
// ---------------------------------------------------------------------------

// IndexFile updates the FTS5 index for a single file.
func (s *SearchService) IndexFile(ctx context.Context, fileID, title, description string) error {
	return s.searchRepo.UpsertFileIndex(ctx, fileID, title, description)
}

// IndexFolder updates the FTS5 index for a single folder.
func (s *SearchService) IndexFolder(ctx context.Context, folderID, name, description string) error {
	return s.searchRepo.UpsertFolderIndex(ctx, folderID, name, description)
}

// RemoveFromIndex removes an entity from the FTS5 index.
func (s *SearchService) RemoveFromIndex(ctx context.Context, entityType, entityID string) error {
	return s.searchRepo.RemoveIndex(ctx, entityType, entityID)
}

// RebuildAllIndexes rebuilds the full FTS5 index from scratch.
func (s *SearchService) RebuildAllIndexes(ctx context.Context) error {
	return s.searchRepo.RebuildAllIndexes(ctx)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func normalizeSearchPagination(page, pageSize, resultWindow int) (int, int, error) {
	if page == 0 {
		page = defaultSearchPage
	}
	if page < 1 {
		return 0, 0, ErrSearchInvalidInput
	}
	if pageSize == 0 {
		pageSize = defaultSearchPageSize
	}
	if pageSize < 1 || pageSize > maxSearchPageSize {
		return 0, 0, ErrSearchInvalidInput
	}
	if resultWindow > 0 && page*pageSize > resultWindow {
		return 0, 0, ErrSearchInvalidInput
	}
	return page, pageSize, nil
}
