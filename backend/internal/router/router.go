package router

import (
	"github.com/gin-gonic/gin"
	"github.com/openshare/backend/internal/config"
	"github.com/openshare/backend/internal/handler"
	"github.com/openshare/backend/internal/middleware"
	"github.com/openshare/backend/pkg/jwt"
	"github.com/openshare/backend/pkg/logger"
)

// Options 路由初始化配置
type Options struct {
	Config     *config.Config
	Logger     *logger.Logger
	Handlers   *handler.Handlers
	JWTManager *jwt.Manager
}

// Setup 初始化路由
func Setup(opts *Options) *gin.Engine {
	// 设置运行模式
	gin.SetMode(opts.Config.Server.Mode)

	r := gin.New()

	// 全局中间件
	r.Use(middleware.Recovery(opts.Logger))
	r.Use(middleware.Logger(opts.Logger))
	r.Use(middleware.CORS())

	// 健康检查
	r.GET("/health", handler.Health)

	// API v1 路由组
	v1 := r.Group("/api/v1")
	{
		// 公开接口（无需认证）
		public := v1.Group("")
		{
			// 资料相关
			public.GET("/files", handler.NotImplemented)
			public.GET("/files/:id", handler.NotImplemented)
			public.GET("/files/:id/download", handler.NotImplemented)
			public.POST("/files/upload", handler.NotImplemented)

			// 搜索
			public.GET("/search", handler.NotImplemented)

			// 投稿查询
			public.GET("/submissions", handler.NotImplemented)

			// 公告
			public.GET("/announcements", handler.NotImplemented)

			// Tag
			public.GET("/tags", handler.NotImplemented)

			// 举报
			public.POST("/reports", handler.NotImplemented)
		}

		// 管理端接口
		admin := v1.Group("/admin")
		{
			// 认证接口（无需 token）
			admin.POST("/login", opts.Handlers.Admin.Login)

			// 需要认证的接口
			auth := admin.Group("")
			auth.Use(middleware.Auth(opts.JWTManager))
			{
				// 当前用户
				auth.GET("/me", opts.Handlers.Admin.GetCurrentAdmin)
				auth.POST("/password", opts.Handlers.Admin.ChangePassword)
				auth.POST("/refresh", opts.Handlers.Admin.RefreshToken)
				auth.POST("/logout", opts.Handlers.Admin.Logout)

				// 审核管理
				auth.GET("/submissions", handler.NotImplemented)
				auth.POST("/submissions/:id/approve", handler.NotImplemented)
				auth.POST("/submissions/:id/reject", handler.NotImplemented)

				// 资料管理
				auth.GET("/files", handler.NotImplemented)
				auth.PUT("/files/:id", handler.NotImplemented)
				auth.DELETE("/files/:id", handler.NotImplemented)
				auth.POST("/files/:id/offline", handler.NotImplemented)

				// Tag 管理
				auth.POST("/tags", handler.NotImplemented)
				auth.PUT("/tags/:id", handler.NotImplemented)
				auth.DELETE("/tags/:id", handler.NotImplemented)

				// 举报管理
				auth.GET("/reports", handler.NotImplemented)
				auth.POST("/reports/:id/approve", handler.NotImplemented)
				auth.POST("/reports/:id/reject", handler.NotImplemented)

				// 公告管理
				auth.POST("/announcements", handler.NotImplemented)
				auth.PUT("/announcements/:id", handler.NotImplemented)
				auth.DELETE("/announcements/:id", handler.NotImplemented)

				// 管理员管理
				auth.GET("/admins", handler.NotImplemented)
				auth.POST("/admins", handler.NotImplemented)
				auth.PUT("/admins/:id", handler.NotImplemented)
				auth.DELETE("/admins/:id", handler.NotImplemented)

				// 操作日志
				auth.GET("/logs", handler.NotImplemented)

				// 系统配置
				auth.GET("/settings", handler.NotImplemented)
				auth.PUT("/settings", handler.NotImplemented)
			}
		}
	}

	return r
}
