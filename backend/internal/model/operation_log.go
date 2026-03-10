package model

import (
	"time"
)

// OperationLog 操作日志表（不使用软删除，日志不应被删除）
type OperationLog struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	OperatorID   *uint     `gorm:"index" json:"operator_id,omitempty"`             // 操作人ID（管理员），NULL 表示系统或游客
	OperatorRole string    `gorm:"type:varchar(20);not null" json:"operator_role"` // 角色：guest, admin, super_admin, system
	Action       string    `gorm:"type:varchar(50);not null;index" json:"action"`  // 操作类型
	TargetType   string    `gorm:"type:varchar(20)" json:"target_type,omitempty"`  // 目标类型：file, folder, submission, tag, admin, announcement, report
	TargetID     string    `gorm:"type:varchar(50)" json:"target_id,omitempty"`    // 目标ID（使用字符串以兼容 UUID 和 uint）
	Detail       string    `gorm:"type:text" json:"detail,omitempty"`              // 详细信息（JSON 格式）
	IP           string    `gorm:"type:varchar(45);not null" json:"ip"`            // 操作者IP
	UserAgent    string    `gorm:"type:varchar(500)" json:"user_agent,omitempty"`  // 用户代理
	Result       string    `gorm:"type:varchar(20);not null" json:"result"`        // 结果：success, failure
	ErrorMsg     string    `gorm:"type:text" json:"error_msg,omitempty"`           // 错误信息
	CreatedAt    time.Time `gorm:"index" json:"created_at"`                        // 操作时间

	// 关联
	Operator *Admin `gorm:"foreignKey:OperatorID" json:"operator,omitempty"`
}

// TableName 指定表名
func (OperationLog) TableName() string {
	return "operation_logs"
}

// 操作类型常量
const (
	// 认证相关
	ActionLogin       = "login"
	ActionLogout      = "logout"
	ActionLoginFailed = "login_failed"

	// 资料相关
	ActionUpload       = "upload"
	ActionDownload     = "download"
	ActionFileCreate   = "file_create"
	ActionFileUpdate   = "file_update"
	ActionFileDelete   = "file_delete"
	ActionFileOffline  = "file_offline"
	ActionFolderCreate = "folder_create"
	ActionFolderUpdate = "folder_update"
	ActionFolderDelete = "folder_delete"

	// 审核相关
	ActionSubmissionApprove = "submission_approve"
	ActionSubmissionReject  = "submission_reject"

	// Tag 相关
	ActionTagCreate = "tag_create"
	ActionTagUpdate = "tag_update"
	ActionTagDelete = "tag_delete"
	ActionTagMerge  = "tag_merge"

	// 举报相关
	ActionReportCreate  = "report_create"
	ActionReportApprove = "report_approve"
	ActionReportReject  = "report_reject"

	// 公告相关
	ActionAnnouncementCreate = "announcement_create"
	ActionAnnouncementUpdate = "announcement_update"
	ActionAnnouncementDelete = "announcement_delete"

	// 管理员相关
	ActionAdminCreate     = "admin_create"
	ActionAdminUpdate     = "admin_update"
	ActionAdminDelete     = "admin_delete"
	ActionAdminPermUpdate = "admin_perm_update"
	ActionAdminReset      = "admin_reset" // 重置密码

	// 系统相关
	ActionSystemConfig  = "system_config"
	ActionSystemStartup = "system_startup"
	ActionFileImport    = "file_import"
)

// 目标类型常量
const (
	TargetTypeFile         = "file"
	TargetTypeFolder       = "folder"
	TargetTypeSubmission   = "submission"
	TargetTypeTag          = "tag"
	TargetTypeAdmin        = "admin"
	TargetTypeAnnouncement = "announcement"
	TargetTypeReport       = "report"
	TargetTypeSystem       = "system"
)

// 操作结果常量
const (
	ResultSuccess = "success"
	ResultFailure = "failure"
)

// 操作角色常量（包含系统）
const (
	OperatorRoleSystem = "system"
)
