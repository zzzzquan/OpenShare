package middleware

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/openshare/backend/pkg/jwt"
	"github.com/openshare/backend/pkg/response"
)

// Auth JWT 认证中间件
// 使用 JWT Manager 进行 token 验证
func Auth(jwtManager *jwt.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Unauthorized(c, "missing authorization header")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.Unauthorized(c, "invalid authorization format")
			c.Abort()
			return
		}

		tokenString := parts[1]
		if tokenString == "" {
			response.Unauthorized(c, "empty token")
			c.Abort()
			return
		}

		// 验证 JWT token
		claims, err := jwtManager.ParseToken(tokenString)
		if err != nil {
			if errors.Is(err, jwt.ErrExpiredToken) {
				response.Unauthorized(c, "token has expired")
			} else {
				response.Unauthorized(c, "invalid token")
			}
			c.Abort()
			return
		}

		// 将用户信息存入上下文
		c.Set("admin_id", claims.AdminID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Set("token", tokenString)

		c.Next()
	}
}

// RequireRole 角色检查中间件
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			response.Forbidden(c, "role not found")
			c.Abort()
			return
		}

		roleStr, ok := role.(string)
		if !ok {
			response.Forbidden(c, "invalid role type")
			c.Abort()
			return
		}

		allowed := false
		for _, r := range roles {
			if r == roleStr {
				allowed = true
				break
			}
		}

		if !allowed {
			response.Forbidden(c, "permission denied")
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetAdminID 从上下文获取管理员ID
func GetAdminID(c *gin.Context) (uint, bool) {
	id, exists := c.Get("admin_id")
	if !exists {
		return 0, false
	}
	adminID, ok := id.(uint)
	return adminID, ok
}

// GetRole 从上下文获取角色
func GetRole(c *gin.Context) (string, bool) {
	role, exists := c.Get("role")
	if !exists {
		return "", false
	}
	roleStr, ok := role.(string)
	return roleStr, ok
}

// IsSuperAdmin 检查是否为超级管理员
func IsSuperAdmin(c *gin.Context) bool {
	role, ok := GetRole(c)
	return ok && role == "super_admin"
}
