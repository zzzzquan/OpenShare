package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Report 举报记录表
type Report struct {
	ID           uint           `gorm:"primarykey" json:"id"`
	TargetType   string         `gorm:"type:varchar(20);not null;index" json:"target_type"`              // file, folder
	TargetID     uuid.UUID      `gorm:"type:uuid;not null;index" json:"target_id"`                       // 被举报资源的 ID
	Reason       string         `gorm:"type:varchar(50);not null" json:"reason"`                         // 举报原因类型
	Description  string         `gorm:"type:text" json:"description,omitempty"`                          // 补充说明
	ReporterIP   string         `gorm:"type:varchar(45)" json:"-"`                                       // 举报者IP
	Status       string         `gorm:"type:varchar(20);not null;default:'pending';index" json:"status"` // pending, approved, rejected
	ReviewerID   *uint          `gorm:"index" json:"reviewer_id,omitempty"`
	ReviewReason string         `gorm:"type:text" json:"review_reason,omitempty"` // 处理备注
	ReviewedAt   *time.Time     `json:"reviewed_at,omitempty"`
	Action       string         `gorm:"type:varchar(20)" json:"action,omitempty"` // 处理动作: offline, deleted, none
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联
	Reviewer *Admin `gorm:"foreignKey:ReviewerID" json:"reviewer,omitempty"`
}

// TableName 指定表名
func (Report) TableName() string {
	return "reports"
}

// 举报目标类型常量
const (
	ReportTargetFile   = "file"
	ReportTargetFolder = "folder"
)

// 举报原因常量
const (
	ReportReasonCopyright  = "copyright"  // 侵权
	ReportReasonError      = "error"      // 内容错误
	ReportReasonCorrupt    = "corrupt"    // 文件损坏
	ReportReasonIrrelevant = "irrelevant" // 无关资料
	ReportReasonOther      = "other"      // 其他
)

// AllReportReasons 所有举报原因
var AllReportReasons = []string{
	ReportReasonCopyright,
	ReportReasonError,
	ReportReasonCorrupt,
	ReportReasonIrrelevant,
	ReportReasonOther,
}

// 举报处理动作常量
const (
	ReportActionOffline = "offline" // 下架
	ReportActionDeleted = "deleted" // 删除
	ReportActionNone    = "none"    // 无需处理
)

// IsPending 是否待处理
func (r *Report) IsPending() bool {
	return r.Status == StatusPending
}

// IsApproved 是否举报成立
func (r *Report) IsApproved() bool {
	return r.Status == StatusApproved
}

// IsRejected 是否驳回
func (r *Report) IsRejected() bool {
	return r.Status == StatusRejected
}
