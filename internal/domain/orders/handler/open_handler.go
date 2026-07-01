package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	ordersvc "payment-gateway/internal/domain/orders/service"
	"payment-gateway/internal/platform/httpx"
)

type OpenHandler struct {
	service ordersvc.Service
}

func NewOpen(service ordersvc.Service) OpenHandler {
	return OpenHandler{service: service}
}

type openOrderRequest struct {
	MerchantOrderNo  string         `json:"merchant_order_no"`
	BusinessType     string         `json:"business_type"`
	Subject          string         `json:"subject"`
	Description      string         `json:"description"`
	Amount           int64          `json:"amount"`
	Currency         string         `json:"currency"`
	Channel          string         `json:"channel"`
	PayMethod        string         `json:"pay_method"`
	PreferredChannel string         `json:"preferred_channel"`
	ClientIP         string         `json:"client_ip"`
	ReturnURL        string         `json:"return_url"`
	Metadata         map[string]any `json:"metadata"`
}

func (h OpenHandler) CreateOrder(ctx *gin.Context) {
	appID := ctx.GetString(httpx.ContextAppID)
	if appID == "" {
		httpx.JSONError(ctx, http.StatusUnauthorized, "unauthorized", "missing app context")
		return
	}
	var req openOrderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	order, created, err := h.service.CreateOpenOrder(ctx.Request.Context(), appID, ordersvc.OpenOrderInput{
		MerchantOrderNo:  req.MerchantOrderNo,
		BusinessType:     req.BusinessType,
		Subject:          req.Subject,
		Description:      req.Description,
		Amount:           req.Amount,
		Currency:         req.Currency,
		Channel:          req.Channel,
		PayMethod:        req.PayMethod,
		PreferredChannel: req.PreferredChannel,
		ClientIP:         req.ClientIP,
		ReturnURL:        req.ReturnURL,
		Metadata:         req.Metadata,
	})
	if err != nil {
		status := http.StatusBadRequest
		if err == ordersvc.ErrIdempotencyConflict {
			status = http.StatusConflict
		}
		httpx.JSONError(ctx, status, "create_order_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusCreated, gin.H{
		"created": created,
		"order":   serializeOrder(order),
		"payment": gin.H{"status": "pending"},
	})
}

func (h OpenHandler) GetOrder(ctx *gin.Context) {
	appID := ctx.GetString(httpx.ContextAppID)
	if appID == "" {
		httpx.JSONError(ctx, http.StatusUnauthorized, "unauthorized", "missing app context")
		return
	}
	order, err := h.service.FindOrderByGatewayOrderNoForApp(ctx.Request.Context(), appID, ctx.Param("gateway_order_no"))
	if err != nil {
		httpx.JSONError(ctx, http.StatusNotFound, "order_not_found", "payment order not found")
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"order": serializeOrder(order)})
}

func (h OpenHandler) GetOrderByMerchant(ctx *gin.Context) {
	appID := ctx.GetString(httpx.ContextAppID)
	if appID == "" {
		httpx.JSONError(ctx, http.StatusUnauthorized, "unauthorized", "missing app context")
		return
	}
	order, err := h.service.FindOrderByMerchantOrderNoForApp(ctx.Request.Context(), appID, ctx.Param("merchant_order_no"))
	if err != nil {
		httpx.JSONError(ctx, http.StatusNotFound, "order_not_found", "payment order not found")
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"order": serializeOrder(order)})
}

func (h OpenHandler) CloseOrder(ctx *gin.Context) {
	appID := ctx.GetString(httpx.ContextAppID)
	if appID == "" {
		httpx.JSONError(ctx, http.StatusUnauthorized, "unauthorized", "missing app context")
		return
	}
	order, err := h.service.CloseOrderForApp(ctx.Request.Context(), appID, ctx.Param("gateway_order_no"))
	if err != nil {
		status := http.StatusNotFound
		if errors.Is(err, ordersvc.ErrOrderCannotBeClosed) {
			status = http.StatusConflict
		}
		httpx.JSONError(ctx, status, "close_order_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"order": serializeOrder(order)})
}
