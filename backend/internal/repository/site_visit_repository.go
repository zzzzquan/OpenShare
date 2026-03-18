package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"openshare/backend/internal/model"
	"openshare/backend/pkg/identity"
)

type SiteVisitRepository struct {
	db *gorm.DB
}

func NewSiteVisitRepository(db *gorm.DB) *SiteVisitRepository {
	return &SiteVisitRepository{db: db}
}

func (r *SiteVisitRepository) Create(ctx context.Context, scope string, path string, ip string) error {
	eventID, err := identity.NewID()
	if err != nil {
		return fmt.Errorf("generate site visit event id: %w", err)
	}

	return r.db.WithContext(ctx).Create(&model.SiteVisitEvent{
		ID:        eventID,
		Scope:     strings.TrimSpace(scope),
		Path:      strings.TrimSpace(path),
		IP:        strings.TrimSpace(ip),
		CreatedAt: time.Now().UTC(),
	}).Error
}
