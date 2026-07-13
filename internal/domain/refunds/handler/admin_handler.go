package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	refundsvc "payment-gateway/internal/domain/refunds/service"
	"payment-gateway/internal/platform/httpx"
)

type AdminHandler struct{ service refundsvc.Service }

func NewAdmin(service refundsvc.Service) AdminHandler { return AdminHandler{service: service} }
func (h AdminHandler) List(ctx *gin.Context) {
	result, err := h.service.List(ctx.Request.Context(), refundsvc.ListInput{AppID: ctx.Query("app_id"), Status: ctx.Query("status"), Channel: ctx.Query("channel"), GatewayOrderNo: ctx.Query("gateway_order_no"), MerchantRefundNo: ctx.Query("merchant_refund_no"), Page: httpx.IntQuery(ctx, "page", 1), PageSize: httpx.IntQuery(ctx, "page_size", 20)})
	if err != nil {
		httpx.JSONError(ctx, http.StatusInternalServerError, "list_refunds_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"items": result.Items, "total": result.Total, "page": result.Page, "page_size": result.PageSize})
}
func (h AdminHandler) Get(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_refund_id", "invalid refund id")
		return
	}
	item, err := h.service.Get(ctx.Request.Context(), id)
	if err != nil {
		httpx.JSONError(ctx, http.StatusNotFound, httpx.CodeRefundNotFound, "refund not found")
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"refund": item})
}
func (h AdminHandler) Create(ctx *gin.Context) {
	var req struct {
		AppID string `json:"app_id"`
		createRequest
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, httpx.CodeInvalidRequest, err.Error())
		return
	}
	item, created, err := h.service.Create(ctx.Request.Context(), refundsvc.CreateInput{AppID: req.AppID, GatewayOrderNo: req.GatewayOrderNo, MerchantRefundNo: req.MerchantRefundNo, Amount: req.Amount, Currency: req.Currency, Reason: req.Reason, Metadata: req.Metadata})
	if err != nil {
		writeRefundError(ctx, err)
		return
	}
	httpx.JSONOK(ctx, http.StatusCreated, gin.H{"created": created, "refund": item})
}
func (h AdminHandler) Retry(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_refund_id", "invalid refund id")
		return
	}
	item, err := h.service.Retry(ctx.Request.Context(), id)
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "retry_refund_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"refund": item})
}
