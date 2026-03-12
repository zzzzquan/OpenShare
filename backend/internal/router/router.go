package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"openshare/backend/internal/config"
	"openshare/backend/internal/handler"
	"openshare/backend/internal/middleware"
	"openshare/backend/internal/model"
	"openshare/backend/internal/repository"
	"openshare/backend/internal/service"
	"openshare/backend/internal/session"
	"openshare/backend/internal/storage"
)

func New(db *gorm.DB, cfg config.Config, sessionManager *session.Manager) *gin.Engine {
	engine := gin.New()
	engine.Use(gin.Logger(), gin.Recovery())
	engine.Use(middleware.SessionLoader(sessionManager))

	storageService := storage.NewService(cfg.Storage)
	adminRepo := repository.NewAdminRepository(db)
	adminAuthService := service.NewAdminAuthService(db, adminRepo, sessionManager)
	adminAuthHandler := handler.NewAdminAuthHandler(adminAuthService, sessionManager)

	searchRepo := repository.NewSearchRepository(db)
	tagRepo := repository.NewTagRepository(db)
	searchService := service.NewSearchService(searchRepo, tagRepo)
	searchHandler := handler.NewSearchHandler(searchService)

	importHandler := handler.NewImportHandler(
		service.NewImportService(repository.NewImportRepository(db), storageService, searchService),
	)
	moderationHandler := handler.NewModerationHandler(
		service.NewModerationService(repository.NewModerationRepository(db), storageService, searchService),
	)
	tagService := service.NewTagService(tagRepo, searchService)
	tagHandler := handler.NewTagHandler(tagService)
	publicCatalogHandler := handler.NewPublicCatalogHandler(
		service.NewPublicCatalogService(repository.NewPublicCatalogRepository(db)),
	)
	publicDownloadHandler := handler.NewPublicDownloadHandler(
		service.NewPublicDownloadService(repository.NewPublicDownloadRepository(db), storageService),
	)
	publicSubmissionHandler := handler.NewPublicSubmissionHandler(
		service.NewPublicSubmissionService(repository.NewPublicSubmissionRepository(db)),
	)
	publicUploadHandler := handler.NewPublicUploadHandler(
		service.NewPublicUploadService(
			cfg.Upload,
			repository.NewUploadRepository(db),
			storageService,
		),
		cfg.Upload.MaxFileSizeBytes+(1<<20),
	)

	reportRepo := repository.NewReportRepository(db)
	reportService := service.NewReportService(reportRepo, searchService)
	reportHandler := handler.NewReportHandler(reportService)

	engine.GET("/healthz", func(ctx *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"status": "error",
				"error":  "database handle is unavailable",
			})
			return
		}

		if err := sqlDB.Ping(); err != nil {
			ctx.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "error",
				"error":  "database ping failed",
			})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := engine.Group("/api")
	public := api.Group("/public")
	public.GET("/files", publicCatalogHandler.ListPublicFiles)
	public.GET("/files/:fileID/download", publicDownloadHandler.DownloadFile)
	public.GET("/folders", publicCatalogHandler.ListPublicFolders)
	public.GET("/search", searchHandler.Search)
	public.POST("/submissions", publicUploadHandler.CreateSubmission)
	public.GET("/submissions/:receiptCode", publicSubmissionHandler.LookupByReceiptCode)
	public.POST("/tag-submissions", tagHandler.SubmitCandidateTag)
	public.POST("/reports", reportHandler.CreateReport)

	admin := api.Group("/admin")
	admin.POST("/session/login", adminAuthHandler.Login)
	admin.POST("/session/logout", adminAuthHandler.Logout)

	adminProtected := admin.Group("")
	adminProtected.Use(middleware.RequireAdminAuth())
	adminProtected.GET("/me", adminAuthHandler.Me)
	adminProtected.GET(
		"/submissions/pending",
		middleware.RequireAdminPermission(model.AdminPermissionReviewSubmissions),
		moderationHandler.ListPendingSubmissions,
	)
	adminProtected.POST(
		"/submissions/:submissionID/approve",
		middleware.RequireAdminPermission(model.AdminPermissionReviewSubmissions),
		moderationHandler.ApproveSubmission,
	)
	adminProtected.POST(
		"/submissions/:submissionID/reject",
		middleware.RequireAdminPermission(model.AdminPermissionReviewSubmissions),
		moderationHandler.RejectSubmission,
	)
	adminProtected.POST(
		"/imports/local",
		middleware.RequireAdminPermission(model.AdminPermissionManageSystem),
		importHandler.ImportLocalDirectory,
	)
	adminProtected.POST(
		"/search/rebuild-index",
		middleware.RequireAdminPermission(model.AdminPermissionManageSystem),
		searchHandler.RebuildIndex,
	)
	adminProtected.GET(
		"/folders/tree",
		importHandler.GetFolderTree,
	)
	adminProtected.PUT(
		"/folders/:folderID/tags",
		middleware.RequireAdminPermission(model.AdminPermissionManageTags),
		tagHandler.BindFolderTags,
	)

	// Tag management routes
	adminProtected.GET(
		"/tags",
		middleware.RequireAdminPermission(model.AdminPermissionManageTags),
		tagHandler.ListTags,
	)
	adminProtected.POST(
		"/tags",
		middleware.RequireAdminPermission(model.AdminPermissionManageTags),
		tagHandler.CreateTag,
	)
	adminProtected.PUT(
		"/tags/:tagID",
		middleware.RequireAdminPermission(model.AdminPermissionManageTags),
		tagHandler.UpdateTag,
	)
	adminProtected.DELETE(
		"/tags/:tagID",
		middleware.RequireAdminPermission(model.AdminPermissionManageTags),
		tagHandler.DeleteTag,
	)
	adminProtected.POST(
		"/tags/merge",
		middleware.RequireAdminPermission(model.AdminPermissionManageTags),
		tagHandler.MergeTags,
	)
	adminProtected.PUT(
		"/files/:fileID/tags",
		middleware.RequireAdminPermission(model.AdminPermissionManageTags),
		tagHandler.BindFileTags,
	)
	adminProtected.GET(
		"/files/:fileID/tags",
		tagHandler.GetFileTagsWithInheritance,
	)
	adminProtected.GET(
		"/folders/:folderID/tags",
		tagHandler.GetFolderTagsWithInheritance,
	)
	adminProtected.GET(
		"/tag-submissions/pending",
		middleware.RequireAdminPermission(model.AdminPermissionManageTags),
		tagHandler.ListPendingTagSubmissions,
	)
	adminProtected.POST(
		"/tag-submissions/:submissionID/approve",
		middleware.RequireAdminPermission(model.AdminPermissionManageTags),
		tagHandler.ApproveCandidateTag,
	)
	adminProtected.POST(
		"/tag-submissions/:submissionID/reject",
		middleware.RequireAdminPermission(model.AdminPermissionManageTags),
		tagHandler.RejectCandidateTag,
	)

	// Report management routes
	adminProtected.GET(
		"/reports/pending",
		middleware.RequireAdminPermission(model.AdminPermissionReviewReports),
		reportHandler.ListPendingReports,
	)
	adminProtected.POST(
		"/reports/:reportID/approve",
		middleware.RequireAdminPermission(model.AdminPermissionReviewReports),
		reportHandler.ApproveReport,
	)
	adminProtected.POST(
		"/reports/:reportID/reject",
		middleware.RequireAdminPermission(model.AdminPermissionReviewReports),
		reportHandler.RejectReport,
	)

	adminPermissionProbe := adminProtected.Group("/_internal")
	adminPermissionProbe.GET(
		"/review",
		middleware.RequireAdminPermission(model.AdminPermissionReviewSubmissions),
		adminAuthHandler.PermissionProbe(model.AdminPermissionReviewSubmissions),
	)
	adminPermissionProbe.GET(
		"/system",
		middleware.RequireAdminPermission(model.AdminPermissionManageSystem),
		adminAuthHandler.PermissionProbe(model.AdminPermissionManageSystem),
	)

	return engine
}
