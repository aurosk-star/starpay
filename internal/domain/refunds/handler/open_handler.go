package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	refundsvc "payment-gateway/internal/domain/refunds/service"
	"payment-gateway/internal/platform/httpx"
)

type OpenHandler struct{ service refundsvc.Service }

func NewOpen(service refundsvc.Service) OpenHandler { return OpenHandler{service: service} }

type createRequest struct {
	GatewayOrderNo   string         `json:"gateway_order_no"`
	MerchantRefundNo string         `json:"merchant_refund_no"`
	Amount           int64          `json:"amount"`
	Currency         string         `json:"currency"`
	Reason           string         `json:"reason"`
	Metadata         map[string]any `json:"metadata"`
}

func (h OpenHandler) Create(ctx *gin.Context) {
	appID := ctx.GetString(httpx.ContextAppID)
	if appID == "" {
		httpx.JSONError(ctx, http.StatusUnauthorized, httpx.CodeInvalidSignature, "missing app context")
		return
	}
	var req createRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, httpx.CodeInvalidRequest, err.Error())
		return
	}
	item, created, err := h.service.Create(ctx.Request.Context(), refundsvc.CreateInput{AppID: appID, GatewayOrderNo: req.GatewayOrderNo, MerchantRefundNo: req.MerchantRefundNo, Amount: req.Amount, Currency: req.Currency, Reason: req.Reason, Metadata: req.Metadata})
	if err != nil {
		writeRefundError(ctx, err)
		return
	}
	httpx.JSONOK(ctx, http.StatusCreated, gin.H{"created": created, "refund": item})
}
func (h OpenHandler) Get(ctx *gin.Context) {
	item, err := h.service.GetForApp(ctx.Request.Context(), ctx.GetString(httpx.ContextAppID), ctx.Param("refund_no"))
	if err != nil {
		httpx.JSONError(ctx, http.StatusNotFound, httpx.CodeRefundNotFound, "refund not found")
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"refund": item})
}
func (h OpenHandler) GetByMerchant(ctx *gin.Context) {
	item, err := h.service.GetByMerchantForApp(ctx.Request.Context(), ctx.GetString(httpx.ContextAppID), ctx.Param("merchant_refund_no"))
	if err != nil {
		httpx.JSONError(ctx, http.StatusNotFound, httpx.CodeRefundNotFound, "refund not found")
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"refund": item})
}
func writeRefundError(ctx *gin.Context, err error) {
	code := httpx.CodeInvalidRequest
	status := http.StatusBadRequest
	switch err {
	case refundsvc.ErrRefundAmountExceedsPaid:
		code = httpx.CodeRefundAmountExceedsPaid
		status = http.StatusConflict
	case refundsvc.ErrRefundStatusNotAllowed:
		code = httpx.CodeRefundStatusNotAllowed
		status = http.StatusConflict
	case refundsvc.ErrIdempotencyConflict:
		code = httpx.CodeIdempotencyConflict
		status = http.StatusConflict
	}
	httpx.JSONError(ctx, status, code, err.Error())
}
