package model

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Tag 标签表
type Tag struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	Name      string         `gorm:"type:varchar(50);not null" json:"name"`                    // 标签名称
	NameLower string         `gorm:"type:varchar(50);uniqueIndex;not null" json:"-"`           // 小写名称，用于唯一性校验
	Color     string         `gorm:"type:varchar(20)" json:"color,omitempty"`                  // 标签颜色（可选）
	Status    string         `gorm:"type:varchar(20);not null;default:'active'" json:"status"` // active, pending（待审核）, disabled
	CreatedBy *uint          `gorm:"index" json:"created_by,omitempty"`                        // 创建者（管理员ID），NULL 表示系统创建
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联
	Creator *Admin   `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
	Files   []File   `gorm:"many2many:file_tags" json:"files,omitempty"`
	Folders []Folder `gorm:"many2many:folder_tags" json:"folders,omitempty"`
}

// BeforeCreate 创建前处理
func (t *Tag) BeforeCreate(tx *gorm.DB) error {
	// 自动设置小写名称用于唯一性校验
	t.NameLower = strings.ToLower(t.Name)
	return nil
}

// BeforeUpdate 更新前处理
func (t *Tag) BeforeUpdate(tx *gorm.DB) error {
	// 同步更新小写名称
	t.NameLower = strings.ToLower(t.Name)
	return nil
}

// TableName 指定表名
func (Tag) TableName() string {
	return "tags"
}

// Tag 状态常量
const (
	TagStatusActive   = "active"
	TagStatusPending  = "pending"  // 用户提交的 Tag，待审核
	TagStatusDisabled = "disabled" // 已禁用
)

// FileTag 文件与标签的关联表
type FileTag struct {
	FileID    uuid.UUID `gorm:"type:uuid;primaryKey" json:"file_id"`
	TagID     uint      `gorm:"primaryKey" json:"tag_id"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName 指定表名
func (FileTag) TableName() string {
	return "file_tags"
}

// FolderTag 文件夹与标签的关联表
type FolderTag struct {
	FolderID  uuid.UUID `gorm:"type:uuid;primaryKey" json:"folder_id"`
	TagID     uint      `gorm:"primaryKey" json:"tag_id"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName 指定表名
func (FolderTag) TableName() string {
	return "folder_tags"
}

// TagSubmission 用户提交的 Tag 申请表（可选：如果需要独立的 Tag 审核流程）
type TagSubmission struct {
	ID           uint           `gorm:"primarykey" json:"id"`
	Name         string         `gorm:"type:varchar(50);not null" json:"name"`
	SubmitterIP  string         `gorm:"type:varchar(45)" json:"-"`
	Status       string         `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`
	ReviewerID   *uint          `gorm:"index" json:"reviewer_id,omitempty"`
	ReviewReason string         `gorm:"type:text" json:"review_reason,omitempty"`
	ReviewedAt   *time.Time     `json:"reviewed_at,omitempty"`
	TagID        *uint          `json:"tag_id,omitempty"` // 审核通过后创建的 Tag ID
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联
	Reviewer *Admin `gorm:"foreignKey:ReviewerID" json:"reviewer,omitempty"`
	Tag      *Tag   `gorm:"foreignKey:TagID" json:"tag,omitempty"`
}

// TableName 指定表名
func (TagSubmission) TableName() string {
	return "tag_submissions"
}
