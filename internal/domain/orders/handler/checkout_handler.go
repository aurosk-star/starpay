package handler

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"payment-gateway/ent"
	channelsvc "payment-gateway/internal/domain/channels/service"
	ordersvc "payment-gateway/internal/domain/orders/service"
	paymentsvc "payment-gateway/internal/domain/payments/service"
	routingsvc "payment-gateway/internal/domain/routing/service"
	"payment-gateway/internal/platform/configvalue"
	"payment-gateway/internal/platform/httpx"
)

type CheckoutHandler struct {
	service         ordersvc.Service
	channels        channelsvc.Service
	payments        paymentsvc.Service
	routing         routingsvc.Service
	notifyURL       func(ctx *gin.Context) string
	paypalReturnURL func(ctx *gin.Context, gatewayOrderNo string) string
	resultURL       func(ctx *gin.Context, gatewayOrderNo string, token string) string
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

func WithRoutingService(routingService routingsvc.Service) CheckoutOption {
	return func(h *CheckoutHandler) {
		h.routing = routingService
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

func WithResultURLResolver(resolver func(ctx *gin.Context, gatewayOrderNo string, token string) string) CheckoutOption {
	return func(h *CheckoutHandler) {
		h.resultURL = resolver
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
	PayMethod        string `json:"pay_method"`
	Channel          string `json:"channel"`
	ChannelAccountID int    `json:"channel_account_id"`
	ClientIP         string `json:"client_ip"`
	ReturnURL        string `json:"return_url"`
}

const checkoutTokenHeader = "X-Checkout-Token"

func (h CheckoutHandler) GetOrder(ctx *gin.Context) {
	order, ok := h.authorizeCheckout(ctx)
	if !ok {
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{
		"title": order.Subject,
		"order": serializeCheckoutOrder(order),
	})
}

func (h CheckoutHandler) ListPaymentMethods(ctx *gin.Context) {
	order, ok := h.authorizeCheckout(ctx)
	if !ok {
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
		if !locked && !h.routing.IsZero() {
			routedMethods, err := h.routedPaymentMethods(ctx, order, accounts)
			if err != nil {
				httpx.JSONError(ctx, http.StatusInternalServerError, "resolve_routing_failed", err.Error())
				return
			}
			if routedMethods != nil {
				httpx.JSONOK(ctx, http.StatusOK, gin.H{
					"locked":          false,
					"selected_method": nil,
					"methods":         routedMethods,
				})
				return
			}
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
			payMode, ok := resolveAccountPayMode(account, ctx.Request.UserAgent())
			if !ok {
				continue
			}
			method := gin.H{
				"pay_method":         account.Channel,
				"channel":            account.Channel,
				"channel_account_id": account.ID,
				"label":              paymentMethodLabel(account.Channel),
				"enabled":            true,
			}
			if payMode != "" {
				method["pay_mode"] = payMode
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

func (h CheckoutHandler) routedPaymentMethods(ctx *gin.Context, order *ent.PaymentOrder, accounts []channelsvc.ChannelAccountView) ([]gin.H, error) {
	rules, err := h.routing.ListRules(ctx.Request.Context())
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, nil
	}
	terminal := "desktop"
	if isMobileUserAgent(ctx.Request.UserAgent()) {
		terminal = "mobile"
	}
	methods := make([]gin.H, 0, len(accounts))
	seenTargets := map[int]struct{}{}
	for _, account := range accounts {
		if !account.Enabled || !paymentsvc.ChannelSupportsCurrency(account.Channel, order.Currency) {
			continue
		}
		payMode, ok := resolveAccountPayMode(account, ctx.Request.UserAgent())
		if !ok {
			continue
		}
		candidates, err := h.routing.Resolve(ctx.Request.Context(), routingsvc.RouteInput{
			AppID:         order.AppID,
			PaymentMethod: account.Channel,
			PayMode:       payMode,
			Amount:        order.Amount,
			Currency:      order.Currency,
			Terminal:      terminal,
		})
		if err != nil {
			return nil, err
		}
		for _, candidate := range candidates {
			if candidate.ChannelAccountID != account.ID {
				continue
			}
			if _, exists := seenTargets[candidate.TargetID]; exists {
				continue
			}
			seenTargets[candidate.TargetID] = struct{}{}
			method := gin.H{
				"pay_method":         candidate.PaymentMethod,
				"channel":            candidate.Channel,
				"channel_account_id": candidate.ChannelAccountID,
				"label":              paymentMethodLabel(candidate.Channel),
				"enabled":            true,
				"rule_id":            candidate.RuleID,
				"target_id":          candidate.TargetID,
			}
			if payMode != "" {
				method["pay_mode"] = payMode
			}
			methods = append(methods, method)
		}
	}
	return methods, nil
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

func resolveAccountPayMode(account channelsvc.ChannelAccountView, userAgent string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(account.Channel)) {
	case "alipay":
		return selectAlipayPayMode(account.Config, userAgent)
	case "wechat":
		return selectWechatPayMode(account.Config, userAgent)
	case "paypal":
		return "checkout", true
	default:
		return "", false
	}
}

func configString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

func (h CheckoutHandler) StartPayment(ctx *gin.Context) {
	order, ok := h.authorizeCheckout(ctx)
	if !ok {
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
	payMode := ""
	if channel == "alipay" || channel == "wechat" {
		resolvedMode, ok, err := h.resolvePayMode(ctx, channel, ctx.Request.UserAgent())
		if err != nil {
			httpx.JSONError(ctx, http.StatusInternalServerError, "resolve_payment_mode_failed", err.Error())
			return
		}
		if !ok {
			httpx.JSONError(ctx, http.StatusBadRequest, "payment_mode_unavailable", "payment method is not available for current terminal")
			return
		}
		payMode = resolvedMode
	}
	checkoutToken := checkoutTokenFromRequest(ctx)
	returnURL := h.resolveResultURL(ctx, order.GatewayOrderNo, checkoutToken)
	if channel == "paypal" || payMethod == "paypal" {
		returnURL = h.resolvePaypalReturnURL(ctx, order.GatewayOrderNo)
		returnURL = appendReturnURL(returnURL, h.resolveResultURL(ctx, order.GatewayOrderNo, checkoutToken))
	}
	notifyURL := ""
	if h.notifyURL != nil {
		notifyURL = strings.TrimSpace(h.notifyURL(ctx))
	}
	payment, err := h.payments.StartPayment(ctx.Request.Context(), paymentsvc.StartPaymentInput{
		Order:            order,
		PayMethod:        payMethod,
		Channel:          channel,
		ChannelAccountID: req.ChannelAccountID,
		PayMode:          payMode,
		ClientIP:         req.ClientIP,
		ReturnURL:        returnURL,
		NotifyURL:        notifyURL,
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

func (h CheckoutHandler) authorizeCheckout(ctx *gin.Context) (*ent.PaymentOrder, bool) {
	token := checkoutTokenFromRequest(ctx)
	order, valid, err := h.service.VerifyCheckoutToken(ctx.Request.Context(), ctx.Param("gateway_order_no"), token)
	if err != nil {
		httpx.JSONError(ctx, http.StatusNotFound, "order_not_found", "payment order not found")
		return nil, false
	}
	if !valid {
		httpx.JSONError(ctx, http.StatusUnauthorized, "checkout_token_invalid", "invalid checkout token")
		return nil, false
	}
	return order, true
}

func (h CheckoutHandler) resolvePayMode(ctx *gin.Context, channel string, userAgent string) (string, bool, error) {
	switch channel {
	case "alipay":
		return h.resolveAlipayPayMode(ctx, userAgent)
	case "wechat":
		return h.resolveWechatPayMode(ctx, userAgent)
	default:
		return "", false, nil
	}
}

func checkoutTokenFromRequest(ctx *gin.Context) string {
	token := strings.TrimSpace(ctx.GetHeader(checkoutTokenHeader))
	if token == "" {
		token = strings.TrimSpace(ctx.Query("token"))
	}
	return token
}

func (h CheckoutHandler) resolveAlipayPayMode(ctx *gin.Context, userAgent string) (string, bool, error) {
	if h.channels.IsZero() {
		if isMobileUserAgent(userAgent) {
			return "wap", true, nil
		}
		return "page", true, nil
	}
	accounts, err := h.channels.ListChannelAccounts(ctx.Request.Context())
	if err != nil {
		return "", false, err
	}
	for _, account := range accounts {
		if !account.Enabled || !strings.EqualFold(account.Channel, "alipay") {
			continue
		}
		payMode, ok := selectAlipayPayMode(account.Config, userAgent)
		return payMode, ok, nil
	}
	return "", false, nil
}

func (h CheckoutHandler) resolveWechatPayMode(ctx *gin.Context, userAgent string) (string, bool, error) {
	if h.channels.IsZero() {
		if isMobileUserAgent(userAgent) {
			return "h5", true, nil
		}
		return "native", true, nil
	}
	accounts, err := h.channels.ListChannelAccounts(ctx.Request.Context())
	if err != nil {
		return "", false, err
	}
	for _, account := range accounts {
		if !account.Enabled || !strings.EqualFold(account.Channel, "wechat") {
			continue
		}
		payMode, ok := selectWechatPayMode(account.Config, userAgent)
		return payMode, ok, nil
	}
	return "", false, nil
}

func selectAlipayPayMode(config map[string]any, userAgent string) (string, bool) {
	pageEnabled := configvalue.BoolDefault(config["enable_page_pay"], true)
	wapEnabled := configvalue.BoolDefault(config["enable_wap_pay"], true)
	qrEnabled := configvalue.BoolDefault(config["enable_qr_pay"], true)
	if isMobileUserAgent(userAgent) {
		if wapEnabled {
			return "wap", true
		}
		if qrEnabled {
			return "qr", true
		}
		return "", false
	}
	if pageEnabled {
		return "page", true
	}
	if qrEnabled {
		return "qr", true
	}
	return "", false
}

func selectWechatPayMode(config map[string]any, userAgent string) (string, bool) {
	mode := strings.ToLower(strings.TrimSpace(configString(config["mode"])))
	nativeEnabled := configvalue.BoolDefault(config["enable_native_pay"], mode == "" || mode == "native" || mode == "qr")
	h5Enabled := configvalue.BoolDefault(config["enable_h5_pay"], mode == "h5")
	if isMobileUserAgent(userAgent) {
		if h5Enabled {
			return "h5", true
		}
		if nativeEnabled {
			return "native", true
		}
		return "", false
	}
	if nativeEnabled {
		return "native", true
	}
	if h5Enabled {
		return "h5", true
	}
	return "", false
}

func isMobileUserAgent(userAgent string) bool {
	ua := strings.ToLower(userAgent)
	return strings.Contains(ua, "mobile") ||
		strings.Contains(ua, "iphone") ||
		strings.Contains(ua, "android") ||
		strings.Contains(ua, "ipad") ||
		strings.Contains(ua, "ipod")
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

func (h CheckoutHandler) resolvePaypalReturnURL(ctx *gin.Context, gatewayOrderNo string) string {
	if h.paypalReturnURL != nil {
		if value := strings.TrimSpace(h.paypalReturnURL(ctx, gatewayOrderNo)); value != "" {
			return value
		}
	}
	return paypalReturnURL(ctx, gatewayOrderNo)
}

func (h CheckoutHandler) resolveResultURL(ctx *gin.Context, gatewayOrderNo string, token string) string {
	if h.resultURL != nil {
		if value := strings.TrimSpace(h.resultURL(ctx, gatewayOrderNo, token)); value != "" {
			return value
		}
	}
	return checkoutResultURL(ctx, gatewayOrderNo, token)
}

func paypalReturnURL(ctx *gin.Context, gatewayOrderNo string) string {
	return requestBaseURL(ctx) + "/v1/checkout/paypal/return?gateway_order_no=" + url.QueryEscape(gatewayOrderNo)
}

func checkoutResultURL(ctx *gin.Context, gatewayOrderNo string, token string) string {
	target := requestBaseURL(ctx) + "/checkout/" + url.PathEscape(gatewayOrderNo) + "/result"
	if strings.TrimSpace(token) == "" {
		return target
	}
	return target + "?token=" + url.QueryEscape(token)
}

func requestBaseURL(ctx *gin.Context) string {
	scheme := "http"
	if ctx.Request.TLS != nil {
		scheme = "https"
	}
	if forwardedProto := strings.TrimSpace(ctx.GetHeader("X-Forwarded-Proto")); forwardedProto != "" {
		scheme = forwardedProto
	}
	host := ctx.Request.Host
	return scheme + "://" + host
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
	if target := strings.TrimSpace(ctx.Query("return_url")); target != "" {
		return target
	}
	return strings.TrimSpace(orderReturnURL)
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
