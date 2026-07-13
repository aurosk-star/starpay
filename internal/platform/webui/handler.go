package webui

import (
	"bytes"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	immutableCacheControl = "public, max-age=31536000, immutable"
	htmlCacheControl      = "no-cache"
)

func Register(router *gin.Engine, assets fs.FS) error {
	if router == nil {
		return fmt.Errorf("web UI router is required")
	}
	if assets == nil {
		return nil
	}
	info, err := fs.Stat(assets, "index.html")
	if err != nil {
		return fmt.Errorf("web UI index.html is required: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("web UI index.html must be a file")
	}
	indexContent, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		return fmt.Errorf("read web UI index.html: %w", err)
	}

	fileServer := http.FileServer(http.FS(assets))
	router.NoRoute(func(ctx *gin.Context) {
		if ctx.Request.Method != http.MethodGet && ctx.Request.Method != http.MethodHead {
			ctx.Status(http.StatusNotFound)
			return
		}

		name := cleanAssetName(ctx.Request.URL.Path)
		if isBackendPath(name) {
			ctx.Status(http.StatusNotFound)
			return
		}
		if name == "index.html" {
			serveIndex(ctx, indexContent, info.ModTime())
			return
		}
		if isRegularFile(assets, name) {
			serveAsset(ctx, fileServer, name)
			return
		}
		if strings.HasPrefix(name, "static/") || path.Ext(name) != "" {
			ctx.Status(http.StatusNotFound)
			return
		}

		serveIndex(ctx, indexContent, info.ModTime())
	})
	return nil
}

func cleanAssetName(requestPath string) string {
	name := strings.TrimPrefix(path.Clean("/"+requestPath), "/")
	if name == "" || name == "." {
		return "index.html"
	}
	return name
}

func isBackendPath(name string) bool {
	return name == "v1" || strings.HasPrefix(name, "v1/") ||
		name == "healthz" || strings.HasPrefix(name, "healthz/")
}

func isRegularFile(assets fs.FS, name string) bool {
	info, err := fs.Stat(assets, name)
	return err == nil && !info.IsDir()
}

func serveAsset(ctx *gin.Context, fileServer http.Handler, name string) {
	if strings.HasPrefix(name, "static/") {
		ctx.Header("Cache-Control", immutableCacheControl)
	} else if name == "index.html" {
		ctx.Header("Cache-Control", htmlCacheControl)
	}

	request := ctx.Request.Clone(ctx.Request.Context())
	requestURL := *request.URL
	requestURL.Path = "/" + name
	request.URL = &requestURL
	fileServer.ServeHTTP(ctx.Writer, request)
}

func serveIndex(ctx *gin.Context, content []byte, modifiedAt time.Time) {
	ctx.Header("Cache-Control", htmlCacheControl)
	http.ServeContent(ctx.Writer, ctx.Request, "index.html", modifiedAt, bytes.NewReader(content))
}
