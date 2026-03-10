package handler

import (
	"github.com/openshare/backend/internal/config"
	"github.com/openshare/backend/internal/service"
	"github.com/openshare/backend/pkg/jwt"
	"github.com/openshare/backend/pkg/logger"
)

// Handlers 聚合所有 handler
type Handlers struct {
	Admin *AdminHandler
	// 后续扩展
	// File       *FileHandler
	// Submission *SubmissionHandler
	// ...
}

// Options handler 初始化配置
type Options struct {
	Services   *service.Services
	Config     *config.Config
	Logger     *logger.Logger
	JWTManager *jwt.Manager
}

// New 创建 handler 聚合实例
func New(opts *Options) *Handlers {
	return &Handlers{
		Admin: NewAdminHandler(opts),
	}
}
