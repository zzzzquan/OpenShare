package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"openshare/backend/internal/service"
	"openshare/backend/internal/session"
)

type ImportHandler struct {
	service *service.ImportService
}

type importLocalRequest struct {
	RootPath string `json:"root_path"`
}

type bindFolderTagsRequest struct {
	Tags []string `json:"tags"`
}

func NewImportHandler(service *service.ImportService) *ImportHandler {
	return &ImportHandler{service: service}
}

func (h *ImportHandler) ImportLocalDirectory(ctx *gin.Context) {
	identity, ok := session.GetAdminIdentity(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	var req importLocalRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	result, err := h.service.ImportLocalDirectory(ctx.Request.Context(), service.LocalImportInput{
		RootPath:   req.RootPath,
		AdminID:    identity.AdminID,
		OperatorIP: ctx.ClientIP(),
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidImportPath):
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid import path"})
		default:
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to import local directory"})
		}
		return
	}

	ctx.JSON(http.StatusOK, result)
}

func (h *ImportHandler) GetFolderTree(ctx *gin.Context) {
	tree, err := h.service.GetFolderTree(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load folder tree"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"items": tree})
}

func (h *ImportHandler) BindFolderTags(ctx *gin.Context) {
	identity, ok := session.GetAdminIdentity(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	var req bindFolderTagsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	err := h.service.BindFolderTags(ctx.Request.Context(), ctx.Param("folderID"), req.Tags, identity.AdminID, ctx.ClientIP())
	if err != nil {
		switch {
		case errors.Is(err, service.ErrFolderTreeNotFound):
			ctx.JSON(http.StatusNotFound, gin.H{"error": "folder not found"})
		case errors.Is(err, service.ErrInvalidUploadInput):
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid tags"})
		default:
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to bind folder tags"})
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}
