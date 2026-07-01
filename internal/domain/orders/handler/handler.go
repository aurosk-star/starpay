package handler

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"payment-gateway/ent"
	ordersvc "payment-gateway/internal/domain/orders/service"
	"payment-gateway/internal/platform/httpx"
)

type Handler struct {
	service     ordersvc.Service
	checkoutURL func(ctx *gin.Context, gatewayOrderNo string, token string) string
}

type Option func(*Handler)

func WithAdminCheckoutURLResolver(resolver func(ctx *gin.Context, gatewayOrderNo string, token string) string) Option {
	return func(h *Handler) {
		h.checkoutURL = resolver
	}
}

func New(service ordersvc.Service, options ...Option) Handler {
	handler := Handler{
		service: service,
		checkoutURL: func(ctx *gin.Context, gatewayOrderNo string, token string) string {
			return "http://localhost:8080/checkout/" + url.PathEscape(gatewayOrderNo) + "?token=" + url.QueryEscape(token)
		},
	}
	for _, opt := range options {
		opt(&handler)
	}
	return handler
}

type manageOrderRequest struct {
	AppID              string         `json:"app_id"`
	MerchantOrderNo    string         `json:"merchant_order_no"`
	BusinessType       string         `json:"business_type"`
	Subject            string         `json:"subject"`
	Description        string         `json:"description"`
	Amount             int64          `json:"amount"`
	Currency           string         `json:"currency"`
	SettlementAmount   int64          `json:"settlement_amount"`
	SettlementCurrency string         `json:"settlement_currency"`
	Channel            string         `json:"channel"`
	PayMethod          string         `json:"pay_method"`
	ReturnURL          string         `json:"return_url"`
	ExpiresAt          *time.Time     `json:"expires_at"`
	Metadata           map[string]any `json:"metadata"`
}

type updateOrderRequest struct {
	BusinessType string         `json:"business_type"`
	Subject      string         `json:"subject"`
	Description  string         `json:"description"`
	Channel      string         `json:"channel"`
	PayMethod    string         `json:"pay_method"`
	Metadata     map[string]any `json:"metadata"`
}

func (h Handler) ListOrders(ctx *gin.Context) {
	result, err := h.service.ListOrders(ctx.Request.Context(), ordersvc.ListOrdersInput{
		AppID:           ctx.Query("app_id"),
		Status:          ctx.Query("status"),
		Channel:         ctx.Query("channel"),
		Currency:        ctx.Query("currency"),
		MerchantOrderNo: ctx.Query("merchant_order_no"),
		Page:            httpx.IntQuery(ctx, "page", 1),
		PageSize:        httpx.IntQuery(ctx, "page_size", 20),
	})
	if err != nil {
		httpx.JSONError(ctx, http.StatusInternalServerError, "list_orders_failed", err.Error())
		return
	}
	items := make([]gin.H, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, serializeOrder(item))
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{
		"items":     items,
		"total":     result.Total,
		"page":      result.Page,
		"page_size": result.PageSize,
	})
}

func (h Handler) GetOrder(ctx *gin.Context) {
	id, err := parseID(ctx)
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_order_id", "invalid order id")
		return
	}
	order, err := h.service.FindOrder(ctx.Request.Context(), id)
	if err != nil {
		httpx.JSONError(ctx, http.StatusNotFound, "order_not_found", "payment order not found")
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"order": serializeOrder(order)})
}

func (h Handler) CreateOrder(ctx *gin.Context) {
	var req manageOrderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	result, err := h.service.CreateOrderWithCheckoutToken(ctx.Request.Context(), ordersvc.ManageOrderInput{
		AppID:              req.AppID,
		MerchantOrderNo:    req.MerchantOrderNo,
		BusinessType:       req.BusinessType,
		Subject:            req.Subject,
		Description:        req.Description,
		Amount:             req.Amount,
		Currency:           req.Currency,
		SettlementAmount:   req.SettlementAmount,
		SettlementCurrency: req.SettlementCurrency,
		Channel:            req.Channel,
		PayMethod:          req.PayMethod,
		ReturnURL:          req.ReturnURL,
		ExpiresAt:          req.ExpiresAt,
		Metadata:           req.Metadata,
	})
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "create_order_failed", err.Error())
		return
	}
	order := result.Order
	httpx.JSONOK(ctx, http.StatusCreated, gin.H{
		"order": serializeOrder(order),
		"payment": gin.H{
			"status":  "pending",
			"pay_url": strings.TrimSpace(h.checkoutURL(ctx, order.GatewayOrderNo, result.CheckoutToken)),
		},
	})
}

func (h Handler) UpdateOrder(ctx *gin.Context) {
	id, err := parseID(ctx)
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_order_id", "invalid order id")
		return
	}
	var req updateOrderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	order, err := h.service.UpdateOrder(ctx.Request.Context(), id, ordersvc.UpdateOrderInput{
		BusinessType: req.BusinessType,
		Subject:      req.Subject,
		Description:  req.Description,
		Channel:      req.Channel,
		PayMethod:    req.PayMethod,
		Metadata:     req.Metadata,
	})
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "update_order_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"order": serializeOrder(order)})
}

func (h Handler) CloseOrder(ctx *gin.Context) {
	id, err := parseID(ctx)
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_order_id", "invalid order id")
		return
	}
	order, err := h.service.CloseOrder(ctx.Request.Context(), id)
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "close_order_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"order": serializeOrder(order)})
}

func parseID(ctx *gin.Context) (int, error) {
	return strconv.Atoi(ctx.Param("id"))
}

func serializeOrder(order *ent.PaymentOrder) gin.H {
	return gin.H{
		"id":                  order.ID,
		"gateway_order_no":    order.GatewayOrderNo,
		"app_id":              order.AppID,
		"merchant_order_no":   order.MerchantOrderNo,
		"business_type":       order.BusinessType,
		"subject":             order.Subject,
		"description":         order.Description,
		"amount":              order.Amount,
		"currency":            order.Currency,
		"settlement_amount":   order.SettlementAmount,
		"settlement_currency": order.SettlementCurrency,
		"channel":             order.Channel,
		"pay_method":          order.PayMethod,
		"channel_trade_no":    order.ChannelTradeNo,
		"return_url":          order.ReturnURL,
		"status":              order.Status,
		"expires_at":          order.ExpiresAt,
		"paid_at":             order.PaidAt,
		"closed_at":           order.ClosedAt,
		"metadata":            order.Metadata,
		"created_at":          order.CreatedAt,
		"updated_at":          order.UpdatedAt,
	}
}

func serializeCheckoutOrder(order *ent.PaymentOrder) gin.H {
	return gin.H{
		"gateway_order_no":  order.GatewayOrderNo,
		"merchant_order_no": order.MerchantOrderNo,
		"business_type":     order.BusinessType,
		"subject":           order.Subject,
		"description":       order.Description,
		"amount":            order.Amount,
		"currency":          order.Currency,
		"channel":           order.Channel,
		"pay_method":        order.PayMethod,
		"return_url":        order.ReturnURL,
		"status":            order.Status,
		"expires_at":        order.ExpiresAt,
		"created_at":        order.CreatedAt,
	}
}
