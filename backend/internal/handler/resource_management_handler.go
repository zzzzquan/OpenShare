package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"openshare/backend/internal/service"
	"openshare/backend/internal/session"
)

type ResourceManagementHandler struct {
	service     *service.ResourceManagementService
	authService *service.AdminAuthService
}

type updateManagedFileRequest struct {
	Title       string `json:"title"`
	Extension   string `json:"extension"`
	Description string `json:"description"`
}

type updateManagedFolderDescriptionRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type deleteManagedResourceRequest struct {
	Password string `json:"password"`
}

func NewResourceManagementHandler(service *service.ResourceManagementService, authService *service.AdminAuthService) *ResourceManagementHandler {
	return &ResourceManagementHandler{service: service, authService: authService}
}

func (h *ResourceManagementHandler) ListFiles(ctx *gin.Context) {
	items, err := h.service.ListFiles(ctx.Request.Context(), service.ListManagedFilesInput{
		Query:  ctx.Query("q"),
		Status: ctx.Query("status"),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list resources"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *ResourceManagementHandler) UpdateFile(ctx *gin.Context) {
	identity, ok := session.GetAdminIdentity(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	var req updateManagedFileRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	err := h.service.UpdateFile(ctx.Request.Context(), ctx.Param("fileID"), service.UpdateManagedFileInput{
		Title:       req.Title,
		Extension:   req.Extension,
		Description: req.Description,
		OperatorID:  identity.AdminID,
		OperatorIP:  ctx.ClientIP(),
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidResourceEdit):
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource input"})
		case errors.Is(err, service.ErrManagedFileNotFound):
			ctx.JSON(http.StatusNotFound, gin.H{"error": "resource not found"})
		default:
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update resource"})
		}
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (h *ResourceManagementHandler) PublicUpdateFile(ctx *gin.Context) {
	var req updateManagedFileRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	err := h.service.PublicUpdateFile(ctx.Request.Context(), ctx.Param("fileID"), service.UpdateManagedFileInput{
		Title:       req.Title,
		Extension:   req.Extension,
		Description: req.Description,
		OperatorIP:  ctx.ClientIP(),
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidResourceEdit):
			ctx.JSON(http.StatusForbidden, gin.H{"error": "guest resource edit is disabled"})
		case errors.Is(err, service.ErrManagedFileNotFound):
			ctx.JSON(http.StatusNotFound, gin.H{"error": "resource not found"})
		default:
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update resource"})
		}
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (h *ResourceManagementHandler) UpdateFolderDescription(ctx *gin.Context) {
	identity, ok := session.GetAdminIdentity(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	var req updateManagedFolderDescriptionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	err := h.service.UpdateFolderDescription(ctx.Request.Context(), ctx.Param("folderID"), service.UpdateManagedFolderDescriptionInput{
		Name:        req.Name,
		Description: req.Description,
		OperatorID:  identity.AdminID,
		OperatorIP:  ctx.ClientIP(),
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidResourceEdit):
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid folder input"})
		case errors.Is(err, service.ErrManagedFolderConflict):
			ctx.JSON(http.StatusConflict, gin.H{"error": "folder name already exists"})
		case errors.Is(err, service.ErrManagedFolderNotFound):
			ctx.JSON(http.StatusNotFound, gin.H{"error": "folder not found"})
		default:
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update folder"})
		}
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (h *ResourceManagementHandler) OfflineFile(ctx *gin.Context) {
	identity, ok := session.GetAdminIdentity(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	err := h.service.OfflineFile(ctx.Request.Context(), ctx.Param("fileID"), identity.AdminID, ctx.ClientIP())
	if err != nil {
		switch {
		case errors.Is(err, service.ErrManagedFileNotFound):
			ctx.JSON(http.StatusNotFound, gin.H{"error": "resource not found"})
		default:
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to offline resource"})
		}
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (h *ResourceManagementHandler) DeleteFile(ctx *gin.Context) {
	identity, ok := session.GetAdminIdentity(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	var req deleteManagedResourceRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if err := h.authService.VerifyPassword(ctx.Request.Context(), identity.AdminID, req.Password); err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidAdminCredentials):
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "invalid password"})
		default:
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify password"})
		}
		return
	}

	err := h.service.DeleteFile(ctx.Request.Context(), ctx.Param("fileID"), identity.AdminID, ctx.ClientIP())
	if err != nil {
		switch {
		case errors.Is(err, service.ErrManagedFileNotFound):
			ctx.JSON(http.StatusNotFound, gin.H{"error": "resource not found"})
		default:
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete resource"})
		}
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (h *ResourceManagementHandler) DeleteFolder(ctx *gin.Context) {
	identity, ok := session.GetAdminIdentity(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	var req deleteManagedResourceRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if err := h.authService.VerifyPassword(ctx.Request.Context(), identity.AdminID, req.Password); err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidAdminCredentials):
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "invalid password"})
		default:
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify password"})
		}
		return
	}

	err := h.service.DeleteFolder(ctx.Request.Context(), ctx.Param("folderID"), identity.AdminID, ctx.ClientIP())
	if err != nil {
		switch {
		case errors.Is(err, service.ErrManagedFolderNotFound):
			ctx.JSON(http.StatusNotFound, gin.H{"error": "folder not found"})
		default:
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete folder"})
		}
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (h *ResourceManagementHandler) PublicDeleteFile(ctx *gin.Context) {
	err := h.service.PublicDeleteFile(ctx.Request.Context(), ctx.Param("fileID"), ctx.ClientIP())
	if err != nil {
		switch {
		case errors.Is(err, service.ErrManagedFileNotFound):
			ctx.JSON(http.StatusNotFound, gin.H{"error": "resource not found"})
		case errors.Is(err, service.ErrInvalidResourceEdit):
			ctx.JSON(http.StatusForbidden, gin.H{"error": "guest resource delete is disabled"})
		default:
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete resource"})
		}
		return
	}
	ctx.Status(http.StatusNoContent)
}
