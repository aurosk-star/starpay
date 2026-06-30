package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	channelsvc "payment-gateway/internal/domain/channels/service"
	"payment-gateway/internal/platform/httpx"
)

type Handler struct {
	service channelsvc.Service
}

func New(service channelsvc.Service) Handler {
	return Handler{service: service}
}

type manageChannelAccountRequest struct {
	Channel string         `json:"channel" binding:"required"`
	Name    string         `json:"name" binding:"required"`
	Enabled *bool          `json:"enabled"`
	Env     string         `json:"env"`
	Config  map[string]any `json:"config"`
}

func (h Handler) ListChannelAccounts(ctx *gin.Context) {
	items, err := h.service.ListChannelAccounts(ctx.Request.Context())
	if err != nil {
		httpx.JSONError(ctx, http.StatusInternalServerError, "list_channels_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"items": items})
}

func (h Handler) GetChannelAccount(ctx *gin.Context) {
	id, err := parseID(ctx)
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_channel_id", "invalid channel id")
		return
	}
	item, err := h.service.FindChannelAccount(ctx.Request.Context(), id)
	if err != nil {
		httpx.JSONError(ctx, http.StatusNotFound, "channel_not_found", "channel account not found")
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"channel_account": item})
}

func (h Handler) CreateChannelAccount(ctx *gin.Context) {
	var req manageChannelAccountRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := h.service.CreateChannelAccount(ctx.Request.Context(), channelsvc.ManageChannelAccountInput{
		Channel: req.Channel,
		Name:    req.Name,
		Enabled: boolValue(req.Enabled, true),
		Env:     req.Env,
		Config:  req.Config,
	})
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "create_channel_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusCreated, gin.H{"channel_account": item})
}

func (h Handler) UpdateChannelAccount(ctx *gin.Context) {
	id, err := parseID(ctx)
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_channel_id", "invalid channel id")
		return
	}
	var req manageChannelAccountRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := h.service.UpdateChannelAccount(ctx.Request.Context(), id, channelsvc.ManageChannelAccountInput{
		Channel: req.Channel,
		Name:    req.Name,
		Enabled: boolValue(req.Enabled, true),
		Env:     req.Env,
		Config:  req.Config,
	})
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "update_channel_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"channel_account": item})
}

func (h Handler) EnableChannelAccount(ctx *gin.Context) {
	h.setEnabled(ctx, true)
}

func (h Handler) DisableChannelAccount(ctx *gin.Context) {
	h.setEnabled(ctx, false)
}

func (h Handler) setEnabled(ctx *gin.Context, enabled bool) {
	id, err := parseID(ctx)
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_channel_id", "invalid channel id")
		return
	}
	var item *channelsvc.ChannelAccountView
	if enabled {
		item, err = h.service.EnableChannelAccount(ctx.Request.Context(), id)
	} else {
		item, err = h.service.DisableChannelAccount(ctx.Request.Context(), id)
	}
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "update_channel_status_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"channel_account": item})
}

func parseID(ctx *gin.Context) (int, error) {
	return strconv.Atoi(ctx.Param("id"))
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}
