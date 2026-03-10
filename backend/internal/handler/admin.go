package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/openshare/backend/internal/middleware"
	"github.com/openshare/backend/internal/service"
	"github.com/openshare/backend/pkg/jwt"
	"github.com/openshare/backend/pkg/logger"
	"github.com/openshare/backend/pkg/response"
)

// AdminHandler 管理员相关接口
type AdminHandler struct {
	adminService *service.AdminService
	jwtManager   *jwt.Manager
	logger       *logger.Logger
}

// NewAdminHandler 创建管理员 handler
func NewAdminHandler(opts *Options) *AdminHandler {
	return &AdminHandler{
		adminService: opts.Services.Admin,
		jwtManager:   opts.JWTManager,
		logger:       opts.Logger,
	}
}

// LoginRequest 登录请求参数
type LoginRequest struct {
	Username string `json:"username" binding:"required,min=1,max=50"`
	Password string `json:"password" binding:"required,min=1,max=100"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt int64     `json:"expires_at"` // Unix 时间戳
	Admin     AdminInfo `json:"admin"`
}

// AdminInfo 管理员基本信息
type AdminInfo struct {
	ID          uint     `json:"id"`
	Username    string   `json:"username"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions,omitempty"`
}

// Login 管理员登录
// POST /api/v1/admin/login
func (h *AdminHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request parameters")
		return
	}

	// 验证凭证
	admin, err := h.adminService.ValidateCredentials(req.Username, req.Password)
	if err != nil {
		h.logger.Error("Login validation error", "error", err, "username", req.Username)
		response.InternalError(c, "internal server error")
		return
	}

	if admin == nil {
		// 登录失败：用户不存在、密码错误或账号已禁用
		// 使用统一的错误信息避免泄露账号存在性
		h.logger.Info("Login failed", "username", req.Username, "ip", c.ClientIP())
		response.Unauthorized(c, "invalid username or password")
		return
	}

	// 生成 JWT Token
	token, expiresAt, err := h.jwtManager.GenerateToken(admin.ID, admin.Username, admin.Role)
	if err != nil {
		h.logger.Error("Failed to generate token", "error", err, "admin_id", admin.ID)
		response.InternalError(c, "failed to generate token")
		return
	}

	// 更新最后登录时间
	if err := h.adminService.UpdateLastLogin(admin.ID); err != nil {
		// 登录时间更新失败不影响主流程
		h.logger.Warn("Failed to update last login time", "error", err, "admin_id", admin.ID)
	}

	// 获取权限列表
	var permissions []string
	for _, p := range admin.Permissions {
		permissions = append(permissions, p.Permission)
	}

	h.logger.Info("Admin logged in",
		"admin_id", admin.ID,
		"username", admin.Username,
		"role", admin.Role,
		"ip", c.ClientIP(),
	)

	response.Success(c, LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt.Unix(),
		Admin: AdminInfo{
			ID:          admin.ID,
			Username:    admin.Username,
			Role:        admin.Role,
			Permissions: permissions,
		},
	})
}

// GetCurrentAdmin 获取当前登录管理员信息
// GET /api/v1/admin/me
func (h *AdminHandler) GetCurrentAdmin(c *gin.Context) {
	adminID, exists := middleware.GetAdminID(c)
	if !exists {
		response.Unauthorized(c, "not logged in")
		return
	}

	admin, err := h.adminService.GetByID(adminID)
	if err != nil {
		h.logger.Error("Failed to get admin", "error", err, "admin_id", adminID)
		response.InternalError(c, "internal server error")
		return
	}

	if admin == nil {
		response.NotFound(c, "admin not found")
		return
	}

	var permissions []string
	for _, p := range admin.Permissions {
		permissions = append(permissions, p.Permission)
	}

	response.Success(c, AdminInfo{
		ID:          admin.ID,
		Username:    admin.Username,
		Role:        admin.Role,
		Permissions: permissions,
	})
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required,min=1"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=100"`
}

// ChangePassword 修改当前管理员密码
// POST /api/v1/admin/password
func (h *AdminHandler) ChangePassword(c *gin.Context) {
	adminID, exists := middleware.GetAdminID(c)
	if !exists {
		response.Unauthorized(c, "not logged in")
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request parameters")
		return
	}

	if err := h.adminService.ChangePassword(adminID, req.OldPassword, req.NewPassword); err != nil {
		if err.Error() == "invalid old password" {
			response.BadRequest(c, "invalid old password")
			return
		}
		h.logger.Error("Failed to change password", "error", err, "admin_id", adminID)
		response.InternalError(c, "failed to change password")
		return
	}

	h.logger.Info("Password changed", "admin_id", adminID, "ip", c.ClientIP())
	response.Success(c, nil)
}

// RefreshToken 刷新 Token
// POST /api/v1/admin/refresh
func (h *AdminHandler) RefreshToken(c *gin.Context) {
	// 从 header 获取当前 token
	tokenString := c.GetString("token")
	if tokenString == "" {
		response.Unauthorized(c, "no token provided")
		return
	}

	// 刷新 token
	newToken, expiresAt, err := h.jwtManager.RefreshToken(tokenString)
	if err != nil {
		h.logger.Warn("Failed to refresh token", "error", err)
		response.Unauthorized(c, "invalid or expired token")
		return
	}

	response.Success(c, gin.H{
		"token":      newToken,
		"expires_at": expiresAt.Unix(),
	})
}

// Logout 退出登录（前端清除 token 即可，后端记录日志）
// POST /api/v1/admin/logout
func (h *AdminHandler) Logout(c *gin.Context) {
	adminID, _ := middleware.GetAdminID(c)
	h.logger.Info("Admin logged out", "admin_id", adminID, "ip", c.ClientIP())
	response.Success(c, nil)
}
