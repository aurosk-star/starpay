package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"payment-gateway/ent"
	appsvc "payment-gateway/internal/domain/apps/service"
	"payment-gateway/internal/platform/httpx"
)

type Handler struct {
	service appsvc.Service
}

func New(service appsvc.Service) Handler {
	return Handler{service: service}
}

type manageAppRequest struct {
	Name             string   `json:"name" binding:"required"`
	NotifyURL        string   `json:"notify_url"`
	DefaultReturnURL string   `json:"default_return_url"`
	AllowedIPs       []string `json:"allowed_ips"`
	Status           string   `json:"status"`
}

func (h Handler) ListApps(ctx *gin.Context) {
	apps, err := h.service.ListApps(ctx.Request.Context())
	if err != nil {
		httpx.JSONError(ctx, http.StatusInternalServerError, "list_apps_failed", err.Error())
		return
	}
	items := make([]gin.H, 0, len(apps))
	for _, item := range apps {
		items = append(items, serializeApp(item))
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"items": items})
}

func (h Handler) GetApp(ctx *gin.Context) {
	id, err := parseID(ctx)
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_app_id", "invalid app id")
		return
	}
	app, err := h.service.GetApp(ctx.Request.Context(), id)
	if err != nil {
		httpx.JSONError(ctx, http.StatusNotFound, "app_not_found", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"app": serializeApp(app)})
}

func (h Handler) CreateApp(ctx *gin.Context) {
	var req manageAppRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	result, err := h.service.CreateApp(ctx.Request.Context(), appsvc.ManageAppInput{
		Name:             req.Name,
		NotifyURL:        req.NotifyURL,
		DefaultReturnURL: req.DefaultReturnURL,
		AllowedIPs:       req.AllowedIPs,
		Status:           req.Status,
	})
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "create_app_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusCreated, gin.H{
		"app":        serializeApp(result.App),
		"app_secret": result.AppSecret,
	})
}

func (h Handler) UpdateApp(ctx *gin.Context) {
	id, err := parseID(ctx)
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_app_id", "invalid app id")
		return
	}
	var req manageAppRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	app, err := h.service.UpdateApp(ctx.Request.Context(), id, appsvc.ManageAppInput{
		Name:             req.Name,
		NotifyURL:        req.NotifyURL,
		DefaultReturnURL: req.DefaultReturnURL,
		AllowedIPs:       req.AllowedIPs,
		Status:           req.Status,
	})
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "update_app_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"app": serializeApp(app)})
}

func (h Handler) EnableApp(ctx *gin.Context) {
	h.setStatus(ctx, true)
}

func (h Handler) DisableApp(ctx *gin.Context) {
	h.setStatus(ctx, false)
}

func (h Handler) ResetSecret(ctx *gin.Context) {
	id, err := parseID(ctx)
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_app_id", "invalid app id")
		return
	}
	result, err := h.service.ResetSecret(ctx.Request.Context(), id)
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "reset_app_secret_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{
		"app":        serializeApp(result.App),
		"app_secret": result.AppSecret,
	})
}

func (h Handler) setStatus(ctx *gin.Context, enabled bool) {
	id, err := parseID(ctx)
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_app_id", "invalid app id")
		return
	}
	var app *ent.App
	if enabled {
		app, err = h.service.EnableApp(ctx.Request.Context(), id)
	} else {
		app, err = h.service.DisableApp(ctx.Request.Context(), id)
	}
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "update_app_status_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"app": serializeApp(app)})
}

func parseID(ctx *gin.Context) (int, error) {
	return strconv.Atoi(ctx.Param("id"))
}

func serializeApp(app *ent.App) gin.H {
	return gin.H{
		"id":                 app.ID,
		"app_id":             app.AppID,
		"name":               app.Name,
		"notify_url":         app.NotifyURL,
		"default_return_url": app.DefaultReturnURL,
		"allowed_ips":        app.AllowedIps,
		"status":             app.Status,
		"created_at":         app.CreatedAt,
		"updated_at":         app.UpdatedAt,
	}
}
