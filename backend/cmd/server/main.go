package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/openshare/backend/internal/config"
	"github.com/openshare/backend/internal/handler"
	"github.com/openshare/backend/internal/router"
	"github.com/openshare/backend/internal/service"
	"github.com/openshare/backend/pkg/database"
	"github.com/openshare/backend/pkg/jwt"
	"github.com/openshare/backend/pkg/logger"
	"github.com/openshare/backend/pkg/storage"
)

func main() {
	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	log, err := logger.New(cfg.Log.Level, cfg.Log.Format)
	if err != nil {
		fmt.Printf("Failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()
	logger.SetDefault(log) // 设置全局默认日志实例

	// 初始化存储目录
	if err := storage.InitDirectories(cfg.Storage.BasePath); err != nil {
		log.Fatal("Failed to init storage directories", "error", err)
	}
	log.Info("Storage directories initialized", "path", cfg.Storage.BasePath)

	// 初始化数据库连接
	if err := database.Init(&cfg.Database); err != nil {
		log.Fatal("Failed to connect database", "error", err)
	}
	defer database.Close()

	// 执行数据库迁移
	if err := database.AutoMigrate(); err != nil {
		log.Fatal("Failed to migrate database", "error", err)
	}

	// 初始化 JWT 管理器
	jwtManager := jwt.NewManager(cfg.JWT.Secret, cfg.JWT.ExpireHour)

	// 初始化服务层
	services := service.New(&service.Options{
		DB:     database.GetDB(),
		Config: cfg,
		Logger: log,
	})

	// 初始化超级管理员
	initSuperAdmin(services, log)

	// 初始化 handler 层
	handlers := handler.New(&handler.Options{
		Services:   services,
		Config:     cfg,
		Logger:     log,
		JWTManager: jwtManager,
	})

	// 初始化路由
	r := router.Setup(&router.Options{
		Config:     cfg,
		Logger:     log,
		Handlers:   handlers,
		JWTManager: jwtManager,
	})

	// 启动服务
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Info("Server starting", "port", cfg.Server.Port)
	if err := r.Run(addr); err != nil {
		log.Fatal("Failed to start server", "error", err)
	}
}

// initSuperAdmin 初始化超级管理员账号
// 仅在首次运行时创建，密码以安全方式输出到日志
func initSuperAdmin(services *service.Services, log *logger.Logger) {
	created, password, err := services.Admin.InitSuperAdmin()
	if err != nil {
		log.Fatal("Failed to initialize super admin", "error", err)
	}

	if created {
		// 使用醒目的格式输出初始密码，便于首次部署时记录
		separator := strings.Repeat("=", 60)
		log.Info(separator)
		log.Info("SUPER ADMIN INITIALIZED")
		log.Info("Username: admin")
		log.Info("Password: " + password)
		log.Info("Please change this password immediately after first login!")
		log.Info(separator)
	}
}
