package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	routingsvc "payment-gateway/internal/domain/routing/service"
	"payment-gateway/internal/platform/httpx"
)

type Handler struct {
	service routingsvc.Service
}

func New(service routingsvc.Service) Handler {
	return Handler{service: service}
}

type manageRuleRequest struct {
	Name          string                `json:"name" binding:"required"`
	Enabled       *bool                 `json:"enabled"`
	Priority      int                   `json:"priority"`
	AppScope      string                `json:"app_scope"`
	AppIDs        []string              `json:"app_ids"`
	PaymentMethod string                `json:"payment_method" binding:"required"`
	PayModes      []string              `json:"pay_modes"`
	Currency      string                `json:"currency"`
	MinAmount     int64                 `json:"min_amount"`
	MaxAmount     int64                 `json:"max_amount"`
	Terminal      string                `json:"terminal"`
	Metadata      map[string]any        `json:"metadata"`
	Targets       []manageTargetRequest `json:"targets"`
}

type manageTargetRequest struct {
	ChannelAccountID int   `json:"channel_account_id"`
	Enabled          *bool `json:"enabled"`
	Priority         int   `json:"priority"`
	Weight           int   `json:"weight"`
}

type previewRequest struct {
	AppID         string `json:"app_id"`
	PaymentMethod string `json:"payment_method"`
	PayMode       string `json:"pay_mode"`
	Amount        int64  `json:"amount"`
	Currency      string `json:"currency"`
	Terminal      string `json:"terminal"`
}

func (h Handler) ListRules(ctx *gin.Context) {
	items, err := h.service.ListRules(ctx.Request.Context())
	if err != nil {
		httpx.JSONError(ctx, http.StatusInternalServerError, "list_routing_rules_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"items": items})
}

func (h Handler) GetRule(ctx *gin.Context) {
	id, err := parseID(ctx)
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_routing_rule_id", "invalid routing rule id")
		return
	}
	item, err := h.service.GetRule(ctx.Request.Context(), id)
	if err != nil {
		httpx.JSONError(ctx, http.StatusNotFound, "routing_rule_not_found", "routing rule not found")
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"routing_rule": item})
}

func (h Handler) CreateRule(ctx *gin.Context) {
	var req manageRuleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := h.service.CreateRule(ctx.Request.Context(), toInput(req))
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "create_routing_rule_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusCreated, gin.H{"routing_rule": item})
}

func (h Handler) UpdateRule(ctx *gin.Context) {
	id, err := parseID(ctx)
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_routing_rule_id", "invalid routing rule id")
		return
	}
	var req manageRuleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := h.service.UpdateRule(ctx.Request.Context(), id, toInput(req))
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "update_routing_rule_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"routing_rule": item})
}

func (h Handler) EnableRule(ctx *gin.Context) {
	h.setEnabled(ctx, true)
}

func (h Handler) DisableRule(ctx *gin.Context) {
	h.setEnabled(ctx, false)
}

func (h Handler) Preview(ctx *gin.Context) {
	var req previewRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	candidates, err := h.service.Resolve(ctx.Request.Context(), routingsvc.RouteInput{
		AppID:         req.AppID,
		PaymentMethod: req.PaymentMethod,
		PayMode:       req.PayMode,
		Amount:        req.Amount,
		Currency:      req.Currency,
		Terminal:      req.Terminal,
	})
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "preview_routing_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"candidates": candidates})
}

func (h Handler) setEnabled(ctx *gin.Context, enabled bool) {
	id, err := parseID(ctx)
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_routing_rule_id", "invalid routing rule id")
		return
	}
	item, err := h.service.SetRuleEnabled(ctx.Request.Context(), id, enabled)
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "update_routing_rule_status_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"routing_rule": item})
}

func toInput(req manageRuleRequest) routingsvc.ManageRuleInput {
	targets := make([]routingsvc.ManageTargetInput, 0, len(req.Targets))
	for _, target := range req.Targets {
		targets = append(targets, routingsvc.ManageTargetInput{
			ChannelAccountID: target.ChannelAccountID,
			Enabled:          boolValue(target.Enabled, true),
			Priority:         target.Priority,
			Weight:           target.Weight,
		})
	}
	return routingsvc.ManageRuleInput{
		Name:          req.Name,
		Enabled:       boolValue(req.Enabled, true),
		Priority:      req.Priority,
		AppScope:      req.AppScope,
		AppIDs:        req.AppIDs,
		PaymentMethod: req.PaymentMethod,
		PayModes:      req.PayModes,
		Currency:      req.Currency,
		MinAmount:     req.MinAmount,
		MaxAmount:     req.MaxAmount,
		Terminal:      req.Terminal,
		Metadata:      req.Metadata,
		Targets:       targets,
	}
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
