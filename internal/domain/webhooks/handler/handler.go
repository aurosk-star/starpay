package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	webhooksvc "payment-gateway/internal/domain/webhooks/service"
	"payment-gateway/internal/platform/httpx"
)

type Handler struct {
	service webhooksvc.Service
}

func New(service webhooksvc.Service) Handler {
	return Handler{service: service}
}

func (h Handler) ListDeliveries(ctx *gin.Context) {
	result, err := h.service.ListDeliveries(ctx.Request.Context(), webhooksvc.ListDeliveriesInput{
		AppID:          ctx.Query("app_id"),
		EventType:      ctx.Query("event_type"),
		Status:         ctx.Query("status"),
		GatewayOrderNo: ctx.Query("gateway_order_no"),
		Page:           httpx.IntQuery(ctx, "page", 1),
		PageSize:       httpx.IntQuery(ctx, "page_size", 20),
	})
	if err != nil {
		httpx.JSONError(ctx, http.StatusInternalServerError, "list_webhook_deliveries_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{
		"items":     result.Items,
		"total":     result.Total,
		"page":      result.Page,
		"page_size": result.PageSize,
	})
}

func (h Handler) GetDelivery(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_delivery_id", "invalid delivery id")
		return
	}
	delivery, err := h.service.GetDelivery(ctx.Request.Context(), id)
	if err != nil {
		httpx.JSONError(ctx, http.StatusNotFound, "webhook_delivery_not_found", "webhook delivery not found")
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"webhook_delivery": delivery})
}

func (h Handler) RetryDelivery(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_delivery_id", "invalid delivery id")
		return
	}
	delivery, err := h.service.RetryDelivery(ctx.Request.Context(), id)
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "retry_webhook_delivery_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"webhook_delivery": delivery})
}
