package model

import (
	"time"

	"gorm.io/gorm"
)

// Admin 管理员表
type Admin struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	Username  string         `gorm:"type:varchar(50);uniqueIndex;not null" json:"username"`
	Password  string         `gorm:"type:varchar(255);not null" json:"-"`                      // bcrypt 哈希，不返回给前端
	Role      string         `gorm:"type:varchar(20);not null;default:'admin'" json:"role"`    // admin, super_admin
	Status    string         `gorm:"type:varchar(20);not null;default:'active'" json:"status"` // active, disabled
	LastLogin *time.Time     `json:"last_login,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联
	Permissions []AdminPermission `gorm:"foreignKey:AdminID" json:"permissions,omitempty"`
}

// AdminPermission 管理员权限表
// 使用独立表存储权限，便于扩展和细粒度控制
type AdminPermission struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	AdminID    uint      `gorm:"not null;index" json:"admin_id"`
	Permission string    `gorm:"type:varchar(50);not null" json:"permission"`
	CreatedAt  time.Time `json:"created_at"`

	// 确保同一管理员不会有重复权限
	// 通过组合唯一索引实现
}

// TableName 指定表名
func (Admin) TableName() string {
	return "admins"
}

// TableName 指定表名
func (AdminPermission) TableName() string {
	return "admin_permissions"
}

// 管理员状态常量
const (
	AdminStatusActive   = "active"
	AdminStatusDisabled = "disabled"
)

// 权限常量 - 可配置的权限项
const (
	PermissionReviewSubmission = "review_submission" // 审核资料
	PermissionPublishAnnounce  = "publish_announce"  // 发布公告
	PermissionEditFile         = "edit_file"         // 修改资料
	PermissionDeleteFile       = "delete_file"       // 删除资料
	PermissionManageTag        = "manage_tag"        // 管理 Tag
	PermissionManageReport     = "manage_report"     // 处理举报
	PermissionViewLog          = "view_log"          // 查看日志
)

// AllPermissions 所有可配置的权限列表
var AllPermissions = []string{
	PermissionReviewSubmission,
	PermissionPublishAnnounce,
	PermissionEditFile,
	PermissionDeleteFile,
	PermissionManageTag,
	PermissionManageReport,
	PermissionViewLog,
}

// HasPermission 检查管理员是否拥有指定权限
func (a *Admin) HasPermission(perm string) bool {
	// 超级管理员拥有所有权限
	if a.Role == RoleSuperAdmin {
		return true
	}
	for _, p := range a.Permissions {
		if p.Permission == perm {
			return true
		}
	}
	return false
}
