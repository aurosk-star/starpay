package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	reconciliationsvc "payment-gateway/internal/domain/reconciliations/service"
	"payment-gateway/internal/platform/httpx"
)

type Handler struct{ service reconciliationsvc.Service }

func New(service reconciliationsvc.Service) Handler { return Handler{service: service} }

func (h Handler) List(ctx *gin.Context) {
	result, err := h.service.List(ctx.Request.Context(), reconciliationsvc.ListInput{
		Status: ctx.Query("status"), Channel: ctx.Query("channel"), GatewayOrderNo: ctx.Query("gateway_order_no"),
		Page: httpx.IntQuery(ctx, "page", 1), PageSize: httpx.IntQuery(ctx, "page_size", 20),
	})
	if err != nil {
		httpx.JSONError(ctx, http.StatusInternalServerError, "list_payment_reconciliations_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"items": result.Items, "total": result.Total, "page": result.Page, "page_size": result.PageSize})
}

func (h Handler) Get(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_reconciliation_id", "invalid reconciliation id")
		return
	}
	item, err := h.service.Get(ctx.Request.Context(), id)
	if err != nil {
		httpx.JSONError(ctx, http.StatusNotFound, "payment_reconciliation_not_found", "payment reconciliation not found")
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"payment_reconciliation": item})
}

func (h Handler) Retry(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_reconciliation_id", "invalid reconciliation id")
		return
	}
	item, err := h.service.Retry(ctx.Request.Context(), id)
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "retry_payment_reconciliation_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"payment_reconciliation": item})
}
