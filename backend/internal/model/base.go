package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BaseModel 基础模型（使用自增 ID）
type BaseModel struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// UUIDModel 使用 UUID 作为主键的基础模型
type UUIDModel struct {
	ID        uuid.UUID      `gorm:"type:uuid;primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// BeforeCreate 创建前自动生成 UUID
func (m *UUIDModel) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// ============ 通用状态常量 ============

// 审核状态常量（投稿、举报、Tag申请等通用）
const (
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusRejected = "rejected"
)

// 资源状态常量（文件、文件夹通用）
const (
	ResourceActive  = "active"
	ResourceOffline = "offline"
	ResourceDeleted = "deleted"
)

// ============ 角色常量 ============

const (
	RoleGuest      = "guest"
	RoleAdmin      = "admin"
	RoleSuperAdmin = "super_admin"
)

// ============ 分页默认值 ============

const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// ============ 模型注册（用于自动迁移） ============

// AllModels 返回所有需要迁移的模型
func AllModels() []interface{} {
	return []interface{}{
		&Admin{},
		&AdminPermission{},
		&File{},
		&Folder{},
		&Submission{},
		&SubmissionTag{},
		&Tag{},
		&FileTag{},
		&FolderTag{},
		&TagSubmission{},
		&Report{},
		&Announcement{},
		&OperationLog{},
	}
}
