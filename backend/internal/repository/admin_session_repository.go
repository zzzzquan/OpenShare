package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"openshare/backend/internal/model"
)

type AdminSessionRepository struct{}

func NewAdminSessionRepository() *AdminSessionRepository {
	return &AdminSessionRepository{}
}

func (r *AdminSessionRepository) Create(tx *gorm.DB, session *model.AdminSession) error {
	return tx.Create(session).Error
}

func (r *AdminSessionRepository) FindByTokenHash(tx *gorm.DB, tokenHash string) (*model.AdminSession, error) {
	var session model.AdminSession
	err := tx.
		Preload("Admin").
		Where("token_hash = ?", tokenHash).
		Take(&session).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &session, nil
}

func (r *AdminSessionRepository) UpdateActivityAndExpiry(tx *gorm.DB, sessionID string, lastActivityAt, expiresAt time.Time) error {
	return tx.Model(&model.AdminSession{}).
		Where("id = ?", sessionID).
		Updates(map[string]any{
			"last_activity_at": lastActivityAt,
			"expires_at":       expiresAt,
		}).
		Error
}

func (r *AdminSessionRepository) DeleteByTokenHash(tx *gorm.DB, tokenHash string) error {
	return tx.Where("token_hash = ?", tokenHash).Delete(&model.AdminSession{}).Error
}

func (r *AdminSessionRepository) DeleteExpired(tx *gorm.DB, now time.Time) error {
	return tx.Where("expires_at <= ?", now).Delete(&model.AdminSession{}).Error
}
