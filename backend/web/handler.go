package webui

import (
	"bytes"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func Register(engine *gin.Engine) {
	dist, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("webui: load embedded dist: " + err.Error())
	}

	engine.NoRoute(func(ctx *gin.Context) {
		requestPath := ctx.Request.URL.Path
		if strings.HasPrefix(requestPath, "/api/") || requestPath == "/api" {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		target := strings.TrimPrefix(path.Clean("/"+requestPath), "/")
		if target == "" || target == "." {
			serveEmbeddedFile(ctx, dist, "index.html")
			return
		}

		if hasFile(dist, target) {
			serveEmbeddedFile(ctx, dist, target)
			return
		}

		serveEmbeddedFile(ctx, dist, "index.html")
	})
}

func hasFile(fsys fs.FS, name string) bool {
	file, err := fsys.Open(name)
	if err != nil {
		return false
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return false
	}

	return !info.IsDir()
}

func serveFile(ctx *gin.Context, handler http.Handler, name string) {
	originalPath := ctx.Request.URL.Path
	ctx.Request.URL.Path = name
	handler.ServeHTTP(ctx.Writer, ctx.Request)
	ctx.Request.URL.Path = originalPath
}

func serveEmbeddedFile(ctx *gin.Context, dist fs.FS, name string) {
	if name != "index.html" {
		// Static assets can still use FileServer semantics safely.
		fileServer := http.FileServer(http.FS(dist))
		serveFile(ctx, fileServer, "/"+name)
		return
	}

	data, err := fs.ReadFile(dist, name)
	if err != nil {
		ctx.Status(http.StatusNotFound)
		return
	}

	ctx.Header("Content-Type", contentType(name))
	http.ServeContent(ctx.Writer, ctx.Request, name, time.Time{}, bytes.NewReader(data))
}

func contentType(name string) string {
	if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
		return contentType
	}
	return "application/octet-stream"
}
