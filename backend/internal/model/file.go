package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// File 文件表
type File struct {
	ID          uuid.UUID      `gorm:"type:uuid;primarykey" json:"id"`
	Name        string         `gorm:"type:varchar(255);not null" json:"name"`                         // 原始文件名
	StoragePath string         `gorm:"type:varchar(1000);not null" json:"-"`                           // 磁盘存储路径，不暴露给前端
	Size        int64          `gorm:"not null" json:"size"`                                           // 文件大小（字节）
	MimeType    string         `gorm:"type:varchar(100)" json:"mime_type"`                             // MIME 类型
	Extension   string         `gorm:"type:varchar(20)" json:"extension"`                              // 文件扩展名
	FolderID    *uuid.UUID     `gorm:"type:uuid;index" json:"folder_id,omitempty"`                     // 所属文件夹
	Status      string         `gorm:"type:varchar(20);not null;default:'active';index" json:"status"` // active, offline, deleted
	Downloads   int64          `gorm:"not null;default:0" json:"downloads"`                            // 下载次数
	Title       string         `gorm:"type:varchar(255)" json:"title"`                                 // 资料标题（可选，默认使用文件名）
	Description string         `gorm:"type:text" json:"description,omitempty"`                         // 资料描述
	Hash        string         `gorm:"type:varchar(64);index" json:"-"`                                // 文件哈希（用于去重）
	UploadIP    string         `gorm:"type:varchar(45)" json:"-"`                                      // 上传者IP（审计用，不展示）
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联
	Folder *Folder `gorm:"foreignKey:FolderID" json:"folder,omitempty"`
	Tags   []Tag   `gorm:"many2many:file_tags" json:"tags,omitempty"`
}

// BeforeCreate 创建前自动生成 UUID
func (f *File) BeforeCreate(tx *gorm.DB) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	return nil
}

// TableName 指定表名
func (File) TableName() string {
	return "files"
}

// GetDisplayTitle 获取展示标题，优先使用 Title，否则使用文件名
func (f *File) GetDisplayTitle() string {
	if f.Title != "" {
		return f.Title
	}
	return f.Name
}

// IsPreviewable 判断文件是否支持预览
func (f *File) IsPreviewable() bool {
	switch f.MimeType {
	case "application/pdf":
		return true
	case "text/plain":
		return true
	}
	// 检查是否是图片类型
	if len(f.MimeType) > 6 && f.MimeType[:6] == "image/" {
		return true
	}
	return false
}
