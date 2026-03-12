package model

import (
	"errors"
	"slices"
	"strings"
)

type AdminRole string

const (
	AdminRoleSuperAdmin AdminRole = "super_admin"
	AdminRoleAdmin      AdminRole = "admin"
)

type AdminPermission string

const (
	AdminPermissionReviewSubmissions   AdminPermission = "review_submissions"
	AdminPermissionManageAnnouncements AdminPermission = "manage_announcements"
	AdminPermissionEditResources       AdminPermission = "edit_resources"
	AdminPermissionDeleteResources     AdminPermission = "delete_resources"
	AdminPermissionManageTags          AdminPermission = "manage_tags"
	AdminPermissionManageAdmins        AdminPermission = "manage_admins"
	AdminPermissionManageSystem        AdminPermission = "manage_system"
	AdminPermissionReviewReports       AdminPermission = "review_reports"
)

var validAdminPermissions = map[AdminPermission]struct{}{
	AdminPermissionReviewSubmissions:   {},
	AdminPermissionManageAnnouncements: {},
	AdminPermissionEditResources:       {},
	AdminPermissionDeleteResources:     {},
	AdminPermissionManageTags:          {},
	AdminPermissionManageAdmins:        {},
	AdminPermissionManageSystem:        {},
	AdminPermissionReviewReports:       {},
}

var validAdminRoles = map[AdminRole]struct{}{
	AdminRoleSuperAdmin: {},
	AdminRoleAdmin:      {},
}

var validAdminStatuses = map[AdminStatus]struct{}{
	AdminStatusActive:   {},
	AdminStatusDisabled: {},
}

func (a Admin) PermissionList() []AdminPermission {
	return ParseAdminPermissions(a.Permissions)
}

func (a Admin) HasPermission(permission AdminPermission) bool {
	if a.Role == string(AdminRoleSuperAdmin) {
		return true
	}

	return slices.Contains(a.PermissionList(), permission)
}

func (a Admin) RoleValue() AdminRole {
	return NormalizeAdminRole(a.Role)
}

func (a Admin) IsActive() bool {
	return a.Status == AdminStatusActive
}

func DefaultAdminPermissions(role AdminRole) []AdminPermission {
	switch NormalizeAdminRole(string(role)) {
	case AdminRoleAdmin:
		return []AdminPermission{AdminPermissionReviewSubmissions}
	case AdminRoleSuperAdmin:
		return nil
	default:
		return nil
	}
}

func NormalizeAdminRole(raw string) AdminRole {
	role := AdminRole(strings.ToLower(strings.TrimSpace(raw)))
	if _, ok := validAdminRoles[role]; !ok {
		return ""
	}

	return role
}

func NormalizeAdminStatus(raw string) AdminStatus {
	status := AdminStatus(strings.ToLower(strings.TrimSpace(raw)))
	if _, ok := validAdminStatuses[status]; !ok {
		return ""
	}

	return status
}

func ValidateAdminRole(raw string) error {
	if NormalizeAdminRole(raw) == "" {
		return errors.New("invalid admin role")
	}

	return nil
}

func ValidateAdminStatus(status AdminStatus) error {
	if _, ok := validAdminStatuses[status]; !ok {
		return errors.New("invalid admin status")
	}

	return nil
}

func NormalizeAdminPermissions(permissions []AdminPermission) string {
	seen := make(map[AdminPermission]struct{}, len(permissions))
	normalized := make([]string, 0, len(permissions))

	for _, permission := range permissions {
		permission = AdminPermission(strings.TrimSpace(string(permission)))
		if _, ok := validAdminPermissions[permission]; !ok {
			continue
		}
		if _, exists := seen[permission]; exists {
			continue
		}

		seen[permission] = struct{}{}
		normalized = append(normalized, string(permission))
	}

	slices.Sort(normalized)
	return strings.Join(normalized, ",")
}

func ParseAdminPermissions(raw string) []AdminPermission {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	permissions := make([]AdminPermission, 0, len(parts))
	seen := make(map[AdminPermission]struct{}, len(parts))

	for _, part := range parts {
		permission := AdminPermission(strings.TrimSpace(part))
		if _, ok := validAdminPermissions[permission]; !ok {
			continue
		}
		if _, exists := seen[permission]; exists {
			continue
		}

		seen[permission] = struct{}{}
		permissions = append(permissions, permission)
	}

	slices.Sort(permissions)
	return permissions
}
