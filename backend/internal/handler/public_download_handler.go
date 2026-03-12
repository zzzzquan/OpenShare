package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"openshare/backend/internal/service"
)

type PublicDownloadHandler struct {
	service *service.PublicDownloadService
}

func NewPublicDownloadHandler(service *service.PublicDownloadService) *PublicDownloadHandler {
	return &PublicDownloadHandler{service: service}
}

func (h *PublicDownloadHandler) DownloadFile(ctx *gin.Context) {
	download, err := h.service.PrepareDownload(ctx.Request.Context(), ctx.Param("fileID"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrDownloadFileNotFound):
			ctx.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		case errors.Is(err, service.ErrDownloadFileUnavailable):
			ctx.JSON(http.StatusGone, gin.H{"error": "file is unavailable"})
		default:
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to download file"})
		}
		return
	}
	defer download.Content.Close()

	if download.MimeType != "" {
		ctx.Header("Content-Type", download.MimeType)
	}
	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", download.OriginalName))
	ctx.Header("Content-Length", strconv.FormatInt(download.Size, 10))

	http.ServeContent(ctx.Writer, ctx.Request, download.OriginalName, download.ModTime, download.Content)
	h.service.RecordDownloadAsync(download.FileID)
}
