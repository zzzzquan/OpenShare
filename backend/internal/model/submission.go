package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Submission 投稿审核记录表
type Submission struct {
	ID           uuid.UUID      `gorm:"type:uuid;primarykey" json:"id"`
	ReceiptCode  string         `gorm:"type:varchar(50);uniqueIndex;not null" json:"receipt_code"`       // 回执码
	Title        string         `gorm:"type:varchar(255);not null" json:"title"`                         // 资料标题
	Description  string         `gorm:"type:text" json:"description,omitempty"`                          // 资料描述
	FileName     string         `gorm:"type:varchar(255);not null" json:"file_name"`                     // 原始文件名
	FileSize     int64          `gorm:"not null" json:"file_size"`                                       // 文件大小
	MimeType     string         `gorm:"type:varchar(100)" json:"mime_type"`                              // MIME 类型
	StagingPath  string         `gorm:"type:varchar(1000);not null" json:"-"`                            // 暂存路径
	FolderID     *uuid.UUID     `gorm:"type:uuid" json:"folder_id,omitempty"`                            // 目标文件夹
	Status       string         `gorm:"type:varchar(20);not null;default:'pending';index" json:"status"` // pending, approved, rejected
	ReviewerID   *uint          `gorm:"index" json:"reviewer_id,omitempty"`                              // 审核人
	ReviewReason string         `gorm:"type:text" json:"review_reason,omitempty"`                        // 审核原因/备注
	ReviewedAt   *time.Time     `json:"reviewed_at,omitempty"`                                           // 审核时间
	FileID       *uuid.UUID     `gorm:"type:uuid" json:"file_id,omitempty"`                              // 审核通过后关联的文件ID
	UploadIP     string         `gorm:"type:varchar(45)" json:"-"`                                       // 上传者IP
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联
	Reviewer *Admin  `gorm:"foreignKey:ReviewerID" json:"reviewer,omitempty"`
	File     *File   `gorm:"foreignKey:FileID" json:"file,omitempty"`
	Folder   *Folder `gorm:"foreignKey:FolderID" json:"folder,omitempty"`
	Tags     []Tag   `gorm:"many2many:submission_tags" json:"tags,omitempty"` // 提交时选择的 Tag
}

// BeforeCreate 创建前自动生成 UUID
func (s *Submission) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

// TableName 指定表名
func (Submission) TableName() string {
	return "submissions"
}

// IsPending 是否待审核
func (s *Submission) IsPending() bool {
	return s.Status == StatusPending
}

// IsApproved 是否已通过
func (s *Submission) IsApproved() bool {
	return s.Status == StatusApproved
}

// IsRejected 是否已驳回
func (s *Submission) IsRejected() bool {
	return s.Status == StatusRejected
}

// SubmissionTag 投稿与标签的关联表
type SubmissionTag struct {
	SubmissionID uuid.UUID `gorm:"type:uuid;primaryKey" json:"submission_id"`
	TagID        uint      `gorm:"primaryKey" json:"tag_id"`
	CreatedAt    time.Time `json:"created_at"`
}

// TableName 指定表名
func (SubmissionTag) TableName() string {
	return "submission_tags"
}
