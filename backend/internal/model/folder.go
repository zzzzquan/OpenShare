package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Folder 文件夹表
type Folder struct {
	ID        uuid.UUID      `gorm:"type:uuid;primarykey" json:"id"`
	Name      string         `gorm:"type:varchar(255);not null" json:"name"`
	ParentID  *uuid.UUID     `gorm:"type:uuid;index" json:"parent_id,omitempty"`    // 父文件夹ID，根目录为 NULL
	Path      string         `gorm:"type:varchar(1000);not null;index" json:"path"` // 完整路径，如 /课程资料/数学
	Status    string         `gorm:"type:varchar(20);not null;default:'active'" json:"status"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联
	Parent   *Folder  `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
	Children []Folder `gorm:"foreignKey:ParentID" json:"children,omitempty"`
	Files    []File   `gorm:"foreignKey:FolderID" json:"files,omitempty"`
	Tags     []Tag    `gorm:"many2many:folder_tags" json:"tags,omitempty"`
}

// BeforeCreate 创建前自动生成 UUID
func (f *Folder) BeforeCreate(tx *gorm.DB) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	return nil
}

// TableName 指定表名
func (Folder) TableName() string {
	return "folders"
}
