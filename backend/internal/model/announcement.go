package model

import (
	"time"

	"gorm.io/gorm"
)

// Announcement 公告表
type Announcement struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	Title       string         `gorm:"type:varchar(255);not null" json:"title"`
	Content     string         `gorm:"type:text;not null" json:"content"`
	AuthorID    uint           `gorm:"not null;index" json:"author_id"`
	IsVisible   bool           `gorm:"not null;default:true" json:"is_visible"` // 是否可见
	IsPinned    bool           `gorm:"not null;default:false" json:"is_pinned"` // 是否置顶
	SortOrder   int            `gorm:"not null;default:0" json:"sort_order"`    // 排序权重，数字越大越靠前
	PublishedAt *time.Time     `json:"published_at,omitempty"`                  // 发布时间
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联
	Author *Admin `gorm:"foreignKey:AuthorID" json:"author,omitempty"`
}

// TableName 指定表名
func (Announcement) TableName() string {
	return "announcements"
}

// IsPublished 是否已发布（可见且有发布时间）
func (a *Announcement) IsPublished() bool {
	return a.IsVisible && a.PublishedAt != nil
}
