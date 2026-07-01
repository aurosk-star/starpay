package handler

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	channelsvc "payment-gateway/internal/domain/channels/service"
	ordersvc "payment-gateway/internal/domain/orders/service"
	paymentsvc "payment-gateway/internal/domain/payments/service"
	"payment-gateway/internal/platform/httpx"
)

type CheckoutHandler struct {
	service         ordersvc.Service
	channels        channelsvc.Service
	payments        paymentsvc.Service
	notifyURL       func(ctx *gin.Context) string
	paypalReturnURL func(ctx *gin.Context, gatewayOrderNo string) string
}

type CheckoutOption func(*CheckoutHandler)

func WithPaymentService(paymentService paymentsvc.Service) CheckoutOption {
	return func(h *CheckoutHandler) {
		h.payments = paymentService
	}
}

func WithChannelService(channelService channelsvc.Service) CheckoutOption {
	return func(h *CheckoutHandler) {
		h.channels = channelService
	}
}

func WithNotifyURLResolver(resolver func(ctx *gin.Context) string) CheckoutOption {
	return func(h *CheckoutHandler) {
		h.notifyURL = resolver
	}
}

func WithPaypalReturnURLResolver(resolver func(ctx *gin.Context, gatewayOrderNo string) string) CheckoutOption {
	return func(h *CheckoutHandler) {
		h.paypalReturnURL = resolver
	}
}

func NewCheckout(service ordersvc.Service, options ...CheckoutOption) CheckoutHandler {
	paymentService := paymentsvc.New()
	handler := CheckoutHandler{service: service, payments: paymentService}
	for _, opt := range options {
		opt(&handler)
	}
	return handler
}

type checkoutPayRequest struct {
	PayMethod string `json:"pay_method"`
	Channel   string `json:"channel"`
	ClientIP  string `json:"client_ip"`
	ReturnURL string `json:"return_url"`
}

func (h CheckoutHandler) GetOrder(ctx *gin.Context) {
	order, err := h.service.FindOrderByGatewayOrderNo(ctx.Request.Context(), ctx.Param("gateway_order_no"))
	if err != nil {
		httpx.JSONError(ctx, http.StatusNotFound, "order_not_found", "payment order not found")
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{
		"title": order.Subject,
		"order": serializeCheckoutOrder(order),
	})
}

func (h CheckoutHandler) ListPaymentMethods(ctx *gin.Context) {
	order, err := h.service.FindOrderByGatewayOrderNo(ctx.Request.Context(), ctx.Param("gateway_order_no"))
	if err != nil {
		httpx.JSONError(ctx, http.StatusNotFound, "order_not_found", "payment order not found")
		return
	}
	methods := []gin.H{}
	lockedChannel := strings.ToLower(strings.TrimSpace(order.Channel))
	lockedPayMethod := strings.ToLower(strings.TrimSpace(order.PayMethod))
	locked := lockedChannel != "" || lockedPayMethod != ""
	if lockedChannel == "" {
		lockedChannel = lockedPayMethod
	}
	if lockedPayMethod == "" {
		lockedPayMethod = lockedChannel
	}
	var selectedMethod gin.H
	if !h.channels.IsZero() {
		accounts, err := h.channels.ListChannelAccounts(ctx.Request.Context())
		if err != nil {
			httpx.JSONError(ctx, http.StatusInternalServerError, "list_channels_failed", err.Error())
			return
		}
		for _, account := range accounts {
			if !account.Enabled {
				continue
			}
			if locked && !strings.EqualFold(account.Channel, lockedChannel) {
				continue
			}
			if !paymentsvc.ChannelSupportsCurrency(account.Channel, order.Currency) {
				continue
			}
			method := gin.H{
				"pay_method": account.Channel,
				"channel":    account.Channel,
				"label":      paymentMethodLabel(account.Channel),
				"enabled":    true,
			}
			if locked {
				method["pay_method"] = lockedPayMethod
				selectedMethod = method
			}
			methods = append(methods, method)
		}
	}
	if locked && selectedMethod == nil {
		selectedMethod = gin.H{
			"pay_method": lockedPayMethod,
			"channel":    lockedChannel,
			"label":      paymentMethodLabel(lockedChannel),
			"enabled":    false,
		}
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{
		"locked":          locked,
		"selected_method": selectedMethod,
		"methods":         methods,
	})
}

func paymentMethodLabel(channel string) string {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "alipay":
		return "支付宝"
	case "wechat":
		return "微信支付"
	case "paypal":
		return "PayPal"
	default:
		return channel
	}
}

func (h CheckoutHandler) StartPayment(ctx *gin.Context) {
	order, err := h.service.FindOrderByGatewayOrderNo(ctx.Request.Context(), ctx.Param("gateway_order_no"))
	if err != nil {
		httpx.JSONError(ctx, http.StatusNotFound, "order_not_found", "payment order not found")
		return
	}
	var req checkoutPayRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	payMethod := strings.ToLower(strings.TrimSpace(req.PayMethod))
	if payMethod == "" {
		payMethod = strings.ToLower(strings.TrimSpace(order.PayMethod))
	}
	channel := strings.ToLower(strings.TrimSpace(req.Channel))
	if channel == "" {
		channel = strings.ToLower(strings.TrimSpace(order.Channel))
	}
	if channel == "" {
		channel = payMethod
	}
	lockedChannel := strings.ToLower(strings.TrimSpace(order.Channel))
	lockedPayMethod := strings.ToLower(strings.TrimSpace(order.PayMethod))
	if lockedChannel != "" || lockedPayMethod != "" {
		expectedChannel := lockedChannel
		if expectedChannel == "" {
			expectedChannel = lockedPayMethod
		}
		expectedPayMethod := lockedPayMethod
		if expectedPayMethod == "" {
			expectedPayMethod = expectedChannel
		}
		if (channel != "" && channel != expectedChannel) || (payMethod != "" && payMethod != expectedPayMethod) {
			httpx.JSONError(ctx, http.StatusBadRequest, "locked_payment_method_mismatch", "payment method is locked by the order")
			return
		}
		channel = expectedChannel
		payMethod = expectedPayMethod
	}
	if !paymentsvc.ChannelSupportsCurrency(channel, order.Currency) {
		httpx.JSONError(ctx, http.StatusBadRequest, "unsupported_currency_for_channel", "currency is not supported by channel")
		return
	}
	returnURL := req.ReturnURL
	if returnURL == "" {
		returnURL = order.ReturnURL
	}
	if channel == "paypal" || payMethod == "paypal" {
		finalReturnURL := strings.TrimSpace(returnURL)
		returnURL = h.resolvePaypalReturnURL(ctx, order.GatewayOrderNo)
		if finalReturnURL != "" {
			returnURL = appendReturnURL(returnURL, finalReturnURL)
		}
	}
	notifyURL := ""
	if h.notifyURL != nil {
		notifyURL = strings.TrimSpace(h.notifyURL(ctx))
	}
	payment, err := h.payments.StartPayment(ctx.Request.Context(), paymentsvc.StartPaymentInput{
		Order:     order,
		PayMethod: payMethod,
		Channel:   channel,
		ClientIP:  req.ClientIP,
		ReturnURL: returnURL,
		NotifyURL: notifyURL,
	})
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "start_payment_failed", err.Error())
		return
	}
	if order.Channel == "" || order.PayMethod == "" {
		if _, err := h.service.SetPaymentSelection(ctx.Request.Context(), order.ID, payment.Channel, payment.PayMethod); err != nil {
			httpx.JSONError(ctx, http.StatusBadRequest, "persist_payment_selection_failed", err.Error())
			return
		}
	}
	if payment.ProviderOrderNo != "" && order.ChannelTradeNo == "" {
		if _, err := h.service.SetChannelTradeNo(ctx.Request.Context(), order.ID, payment.ProviderOrderNo); err != nil {
			httpx.JSONError(ctx, http.StatusBadRequest, "persist_provider_order_failed", err.Error())
			return
		}
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"payment": payment})
}

func (h CheckoutHandler) CompletePaypalPayment(ctx *gin.Context) {
	gatewayOrderNo := strings.TrimSpace(ctx.Query("gateway_order_no"))
	if gatewayOrderNo == "" {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_request", "gateway_order_no is required")
		return
	}
	order, err := h.service.FindOrderByGatewayOrderNo(ctx.Request.Context(), gatewayOrderNo)
	if err != nil {
		httpx.JSONError(ctx, http.StatusNotFound, "order_not_found", "payment order not found")
		return
	}
	finalReturnURL := paypalFinalReturnURL(ctx, order.ReturnURL)
	if ctx.Query("cancel") == "1" {
		redirectAfterPaypal(ctx, finalReturnURL, gatewayOrderNo, "cancelled")
		return
	}
	paypalOrderID := strings.TrimSpace(ctx.Query("token"))
	if paypalOrderID == "" {
		paypalOrderID = strings.TrimSpace(order.ChannelTradeNo)
	}
	if paypalOrderID == "" {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_request", "paypal order token is required")
		return
	}
	result, err := h.payments.CapturePayment(ctx.Request.Context(), paymentsvc.CapturePaymentInput{
		Channel:         "paypal",
		ProviderOrderNo: paypalOrderID,
		GatewayOrderNo:  gatewayOrderNo,
	})
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "paypal_capture_failed", err.Error())
		return
	}
	if result.Status == "paid" && order.Status != "paid" {
		if _, err := h.service.MarkPaid(ctx.Request.Context(), order.ID, result.ChannelTradeNo); err != nil {
			httpx.JSONError(ctx, http.StatusBadRequest, "mark_paid_failed", err.Error())
			return
		}
	}
	redirectAfterPaypal(ctx, finalReturnURL, gatewayOrderNo, result.Status)
}

func (h CheckoutHandler) CompleteMockPayment(ctx *gin.Context) {
	order, err := h.service.FindOrderByGatewayOrderNo(ctx.Request.Context(), ctx.Param("gateway_order_no"))
	if err != nil {
		httpx.JSONError(ctx, http.StatusNotFound, "order_not_found", "payment order not found")
		return
	}
	if order.Status != "pending" {
		httpx.JSONError(ctx, http.StatusConflict, "order_not_payable", "order is not payable")
		return
	}
	paid, err := h.service.MarkPaid(ctx.Request.Context(), order.ID, "mock_"+order.GatewayOrderNo)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, ordersvc.ErrOrderCannotBeClosed) {
			status = http.StatusConflict
		}
		httpx.JSONError(ctx, status, "complete_mock_payment_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"order": serializeOrder(paid)})
}

func (h CheckoutHandler) resolvePaypalReturnURL(ctx *gin.Context, gatewayOrderNo string) string {
	if h.paypalReturnURL != nil {
		if value := strings.TrimSpace(h.paypalReturnURL(ctx, gatewayOrderNo)); value != "" {
			return value
		}
	}
	return paypalReturnURL(ctx, gatewayOrderNo)
}

func paypalReturnURL(ctx *gin.Context, gatewayOrderNo string) string {
	scheme := "http"
	if ctx.Request.TLS != nil {
		scheme = "https"
	}
	if forwardedProto := strings.TrimSpace(ctx.GetHeader("X-Forwarded-Proto")); forwardedProto != "" {
		scheme = forwardedProto
	}
	host := ctx.Request.Host
	return scheme + "://" + host + "/v1/checkout/paypal/return?gateway_order_no=" + url.QueryEscape(gatewayOrderNo)
}

func appendReturnURL(paypalReturnURL string, finalReturnURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(paypalReturnURL))
	if err != nil {
		return paypalReturnURL
	}
	query := parsed.Query()
	query.Set("return_url", strings.TrimSpace(finalReturnURL))
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func paypalFinalReturnURL(ctx *gin.Context, orderReturnURL string) string {
	if target := strings.TrimSpace(orderReturnURL); target != "" {
		return target
	}
	return strings.TrimSpace(ctx.Query("return_url"))
}

func redirectAfterPaypal(ctx *gin.Context, returnURL string, gatewayOrderNo string, status string) {
	target := strings.TrimSpace(returnURL)
	if target == "" {
		target = "/checkout/" + url.PathEscape(gatewayOrderNo)
	}
	parsed, err := url.Parse(target)
	if err == nil {
		query := parsed.Query()
		query.Set("gateway_order_no", gatewayOrderNo)
		query.Set("status", status)
		parsed.RawQuery = query.Encode()
		target = parsed.String()
	}
	ctx.Redirect(http.StatusFound, target)
}
