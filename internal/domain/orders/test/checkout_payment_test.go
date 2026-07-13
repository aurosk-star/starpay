package orderstest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"

	"payment-gateway/ent"
	"payment-gateway/ent/enttest"
	channelrepo "payment-gateway/internal/domain/channels/repository"
	channelsvc "payment-gateway/internal/domain/channels/service"
	orderhandler "payment-gateway/internal/domain/orders/handler"
	ordersvc "payment-gateway/internal/domain/orders/service"
	paymentprovider "payment-gateway/internal/domain/payments/provider"
	paymentsvc "payment-gateway/internal/domain/payments/service"
	routingsvc "payment-gateway/internal/domain/routing/service"
	"payment-gateway/internal/platform/httpx"
)

func TestCheckoutHandlerListsOnlyEnabledPaymentMethods(t *testing.T) {
	router, created, orderService, channelService := newCheckoutPaymentTestRouter(t, "checkout_methods")
	if _, err := channelService.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
		Channel: "alipay",
		Name:    "支付宝沙箱",
		Enabled: true,
		Env:     "sandbox",
		Config: map[string]any{
			"app_id":      "app-1",
			"private_key": "private-key",
		},
	}); err != nil {
		t.Fatalf("CreateChannelAccount(alipay) error = %v", err)
	}
	if _, err := channelService.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
		Channel: "wechat",
		Name:    "微信禁用",
		Enabled: false,
		Env:     "sandbox",
		Config:  map[string]any{"app_id": "wx-1"},
	}); err != nil {
		t.Fatalf("CreateChannelAccount(wechat) error = %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, checkoutRequest(t, orderService, created, http.MethodGet, "/orders/"+created.GatewayOrderNo+"/methods", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data := response["data"].(map[string]any)
	methods := data["methods"].([]any)
	if len(methods) != 1 {
		t.Fatalf("methods len = %d, want 1", len(methods))
	}
	first := methods[0].(map[string]any)
	if first["pay_method"] != "alipay" || first["channel"] != "alipay" || first["enabled"] != true {
		t.Fatalf("first method = %#v, want enabled alipay", first)
	}
}

func TestCheckoutHandlerUsesMobileAlipayWapCapabilityFromUserAgent(t *testing.T) {
	router, created, orderService, channelService := newCheckoutPaymentTestRouter(t, "checkout_methods_mobile_wap")
	if _, err := channelService.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
		Channel: "alipay",
		Name:    "支付宝沙箱",
		Enabled: true,
		Env:     "sandbox",
		Config: map[string]any{
			"app_id":         "app-1",
			"private_key":    "private-key",
			"enable_wap_pay": "true",
			"enable_qr_pay":  "true",
		},
	}); err != nil {
		t.Fatalf("CreateChannelAccount(alipay) error = %v", err)
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/orders/"+created.GatewayOrderNo+"/methods", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) Mobile/15E148")
	req.Header.Set("X-Checkout-Token", checkoutTokenForOrder(t, orderService, created))
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	methods := decodeCheckoutMethods(t, recorder)
	if len(methods) != 1 {
		t.Fatalf("methods len = %d, want 1", len(methods))
	}
	first := methods[0].(map[string]any)
	if first["channel"] != "alipay" || first["pay_mode"] != "wap" {
		t.Fatalf("first method = %#v, want alipay wap for mobile UA", first)
	}
}

func TestCheckoutHandlerFallsBackToAlipayQRCodeOnMobileWhenWapDisabled(t *testing.T) {
	router, created, orderService, channelService := newCheckoutPaymentTestRouter(t, "checkout_methods_mobile_qr_fallback")
	if _, err := channelService.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
		Channel: "alipay",
		Name:    "支付宝沙箱",
		Enabled: true,
		Env:     "sandbox",
		Config: map[string]any{
			"app_id":          "app-1",
			"private_key":     "private-key",
			"enable_wap_pay":  "false",
			"enable_page_pay": "true",
			"enable_qr_pay":   "true",
		},
	}); err != nil {
		t.Fatalf("CreateChannelAccount(alipay) error = %v", err)
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/orders/"+created.GatewayOrderNo+"/methods", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 Mobile Safari/537.36")
	req.Header.Set("X-Checkout-Token", checkoutTokenForOrder(t, orderService, created))
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	methods := decodeCheckoutMethods(t, recorder)
	if len(methods) != 1 {
		t.Fatalf("methods len = %d, want 1", len(methods))
	}
	first := methods[0].(map[string]any)
	if first["channel"] != "alipay" || first["pay_mode"] != "qr" {
		t.Fatalf("first method = %#v, want alipay qr fallback for mobile UA", first)
	}
}

func TestCheckoutHandlerUsesWechatH5CapabilityFromMobileUserAgent(t *testing.T) {
	router, created, orderService, channelService := newCheckoutPaymentTestRouter(t, "checkout_methods_wechat_mobile_h5")
	created = createUnlockedCheckoutOrder(t, orderService, "checkout_methods_wechat_mobile_h5")
	if _, err := channelService.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
		Channel: "wechat",
		Name:    "微信支付",
		Enabled: true,
		Env:     "sandbox",
		Config: map[string]any{
			"app_id":            "wx-1",
			"enable_native_pay": "true",
			"enable_h5_pay":     "true",
		},
	}); err != nil {
		t.Fatalf("CreateChannelAccount(wechat) error = %v", err)
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/orders/"+created.GatewayOrderNo+"/methods", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) Mobile/15E148")
	req.Header.Set("X-Checkout-Token", checkoutTokenForOrder(t, orderService, created))
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	methods := decodeCheckoutMethods(t, recorder)
	if len(methods) != 1 {
		t.Fatalf("methods len = %d, want 1", len(methods))
	}
	first := methods[0].(map[string]any)
	if first["channel"] != "wechat" || first["pay_mode"] != "h5" {
		t.Fatalf("first method = %#v, want wechat h5 for mobile UA", first)
	}
}

func TestCheckoutHandlerUsesWechatNativeOnDesktopWhenEnabled(t *testing.T) {
	router, created, orderService, channelService := newCheckoutPaymentTestRouter(t, "checkout_methods_wechat_desktop_native")
	created = createUnlockedCheckoutOrder(t, orderService, "checkout_methods_wechat_desktop_native")
	if _, err := channelService.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
		Channel: "wechat",
		Name:    "微信支付",
		Enabled: true,
		Env:     "sandbox",
		Config: map[string]any{
			"app_id":            "wx-1",
			"enable_native_pay": "true",
			"enable_h5_pay":     "true",
		},
	}); err != nil {
		t.Fatalf("CreateChannelAccount(wechat) error = %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, checkoutRequest(t, orderService, created, http.MethodGet, "/orders/"+created.GatewayOrderNo+"/methods", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	methods := decodeCheckoutMethods(t, recorder)
	if len(methods) != 1 {
		t.Fatalf("methods len = %d, want 1", len(methods))
	}
	first := methods[0].(map[string]any)
	if first["channel"] != "wechat" || first["pay_mode"] != "native" {
		t.Fatalf("first method = %#v, want wechat native for desktop UA", first)
	}
}

func TestCheckoutHandlerPassesAlipayQRModeWhenMobileWapDisabled(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	client := enttest.Open(t, dialect.SQLite, "file:checkout_pay_mobile_qr_fallback?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { client.Close() })
	createEnabledApp(t, client, "snsgo")

	orderService := ordersvc.New(client)
	order, err := orderService.CreateOrder(t.Context(), ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_checkout_pay_mobile_qr_fallback",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
		PayMethod:       "alipay",
		Channel:         "alipay",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}

	channelService := channelsvc.New(client)
	if _, err := channelService.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
		Channel: "alipay",
		Name:    "支付宝沙箱",
		Enabled: true,
		Env:     "sandbox",
		Config: map[string]any{
			"app_id":         "app-1",
			"private_key":    "private-key",
			"enable_wap_pay": "false",
			"enable_qr_pay":  "true",
		},
	}); err != nil {
		t.Fatalf("CreateChannelAccount(alipay) error = %v", err)
	}
	provider := &checkoutFakeProvider{channel: "alipay"}
	paymentService := paymentsvc.New(
		paymentsvc.WithChannelRepository(channelrepo.New(client)),
		paymentsvc.WithProvider(provider),
	)

	router := gin.New()
	checkoutHandler := orderhandler.NewCheckout(
		orderService,
		orderhandler.WithChannelService(channelService),
		orderhandler.WithPaymentService(paymentService),
	)
	router.POST("/orders/:gateway_order_no/pay", checkoutHandler.StartPayment)

	recorder := httptest.NewRecorder()
	req := jsonRequest(http.MethodPost, "/orders/"+order.GatewayOrderNo+"/pay", map[string]any{
		"pay_method": "alipay",
		"channel":    "alipay",
	})
	req.Header.Set("X-Checkout-Token", checkoutTokenForOrder(t, orderService, order))
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) Mobile/15E148")
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if provider.req.ChannelAccount.Config["mode"] != "qr" {
		t.Fatalf("provider config mode = %#v, want qr fallback", provider.req.ChannelAccount.Config["mode"])
	}
}

func TestCheckoutHandlerPassesWechatH5ModeForMobileUserAgent(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	client := enttest.Open(t, dialect.SQLite, "file:checkout_pay_wechat_mobile_h5?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { client.Close() })
	createEnabledApp(t, client, "snsgo")

	orderService := ordersvc.New(client)
	order, err := orderService.CreateOrder(t.Context(), ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_checkout_pay_wechat_mobile_h5",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
		PayMethod:       "wechat",
		Channel:         "wechat",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}

	channelService := channelsvc.New(client)
	if _, err := channelService.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
		Channel: "wechat",
		Name:    "微信支付",
		Enabled: true,
		Env:     "sandbox",
		Config: map[string]any{
			"app_id":            "wx-1",
			"enable_native_pay": "true",
			"enable_h5_pay":     "true",
		},
	}); err != nil {
		t.Fatalf("CreateChannelAccount(wechat) error = %v", err)
	}
	provider := &checkoutFakeProvider{channel: "wechat"}
	paymentService := paymentsvc.New(
		paymentsvc.WithChannelRepository(channelrepo.New(client)),
		paymentsvc.WithProvider(provider),
	)

	router := gin.New()
	checkoutHandler := orderhandler.NewCheckout(
		orderService,
		orderhandler.WithChannelService(channelService),
		orderhandler.WithPaymentService(paymentService),
	)
	router.POST("/orders/:gateway_order_no/pay", checkoutHandler.StartPayment)

	recorder := httptest.NewRecorder()
	req := jsonRequest(http.MethodPost, "/orders/"+order.GatewayOrderNo+"/pay", map[string]any{
		"pay_method": "wechat",
		"channel":    "wechat",
	})
	req.Header.Set("X-Checkout-Token", checkoutTokenForOrder(t, orderService, order))
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) Mobile/15E148")
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if provider.req.ChannelAccount.Config["mode"] != "h5" {
		t.Fatalf("provider config mode = %#v, want h5 for mobile UA", provider.req.ChannelAccount.Config["mode"])
	}
}

func TestCheckoutHandlerHidesMethodsWhenNoChannelEnabled(t *testing.T) {
	router, created, orderService, channelService := newCheckoutPaymentTestRouter(t, "checkout_methods_empty")
	if _, err := channelService.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
		Channel: "paypal",
		Name:    "PayPal 禁用",
		Enabled: false,
		Env:     "sandbox",
		Config:  map[string]any{"client_id": "client-1"},
	}); err != nil {
		t.Fatalf("CreateChannelAccount(paypal) error = %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, checkoutRequest(t, orderService, created, http.MethodGet, "/orders/"+created.GatewayOrderNo+"/methods", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	methods := response["data"].(map[string]any)["methods"].([]any)
	if len(methods) != 0 {
		t.Fatalf("methods len = %d, want 0", len(methods))
	}
}

func TestCheckoutHandlerHidesUnsupportedCurrencyMethods(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	client := enttest.Open(t, dialect.SQLite, "file:checkout_methods_currency?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { client.Close() })
	createEnabledApp(t, client, "snsgo")

	orderService := ordersvc.New(client)
	created, err := orderService.CreateOrder(t.Context(), ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_checkout_methods_currency",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "USD",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}

	channelService := channelsvc.New(client)
	if _, err := channelService.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
		Channel: "alipay",
		Name:    "支付宝沙箱",
		Enabled: true,
		Env:     "sandbox",
		Config:  map[string]any{"app_id": "app-1"},
	}); err != nil {
		t.Fatalf("CreateChannelAccount(alipay) error = %v", err)
	}
	if _, err := channelService.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
		Channel: "paypal",
		Name:    "PayPal Sandbox",
		Enabled: true,
		Env:     "sandbox",
		Config:  map[string]any{"client_id": "client-1"},
	}); err != nil {
		t.Fatalf("CreateChannelAccount(paypal) error = %v", err)
	}

	router := gin.New()
	checkoutHandler := orderhandler.NewCheckout(orderService, orderhandler.WithChannelService(channelService))
	router.GET("/orders/:gateway_order_no/methods", checkoutHandler.ListPaymentMethods)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, checkoutRequest(t, orderService, created, http.MethodGet, "/orders/"+created.GatewayOrderNo+"/methods", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	methods := response["data"].(map[string]any)["methods"].([]any)
	if len(methods) != 1 {
		t.Fatalf("methods len = %d, want 1", len(methods))
	}
	first := methods[0].(map[string]any)
	if first["channel"] != "paypal" {
		t.Fatalf("first method = %#v, want paypal only", first)
	}
}

func TestCheckoutHandlerReturnsLockedMethodState(t *testing.T) {
	router, created, orderService, channelService := newCheckoutPaymentTestRouter(t, "checkout_methods_locked_state")
	if _, err := channelService.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
		Channel: "alipay",
		Name:    "支付宝沙箱",
		Enabled: true,
		Env:     "sandbox",
		Config:  map[string]any{"app_id": "app-1"},
	}); err != nil {
		t.Fatalf("CreateChannelAccount(alipay) error = %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, checkoutRequest(t, orderService, created, http.MethodGet, "/orders/"+created.GatewayOrderNo+"/methods", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data := response["data"].(map[string]any)
	if data["locked"] != true {
		t.Fatalf("locked = %#v, want true", data["locked"])
	}
	selected := data["selected_method"].(map[string]any)
	if selected["channel"] != "alipay" || selected["pay_method"] != "alipay" {
		t.Fatalf("selected = %#v, want alipay", selected)
	}
}

func TestCheckoutHandlerLockedOrderShowsOnlyPersistedMethod(t *testing.T) {
	router, created, orderService, channelService := newCheckoutPaymentTestRouter(t, "checkout_methods_locked")
	if _, err := channelService.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
		Channel: "alipay",
		Name:    "支付宝沙箱",
		Enabled: true,
		Env:     "sandbox",
		Config:  map[string]any{"app_id": "app-1"},
	}); err != nil {
		t.Fatalf("CreateChannelAccount(alipay) error = %v", err)
	}
	if _, err := channelService.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
		Channel: "paypal",
		Name:    "PayPal Sandbox",
		Enabled: true,
		Env:     "sandbox",
		Config:  map[string]any{"client_id": "client-1"},
	}); err != nil {
		t.Fatalf("CreateChannelAccount(paypal) error = %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, checkoutRequest(t, orderService, created, http.MethodGet, "/orders/"+created.GatewayOrderNo+"/methods", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	methods := response["data"].(map[string]any)["methods"].([]any)
	if len(methods) != 1 {
		t.Fatalf("methods len = %d, want locked single method", len(methods))
	}
	first := methods[0].(map[string]any)
	if first["channel"] != "alipay" {
		t.Fatalf("first method = %#v, want locked alipay", first)
	}
}

func TestCheckoutHandlerStartsPaymentThroughPaymentService(t *testing.T) {
	router, created, orderService, channelService := newCheckoutPaymentTestRouter(t, "checkout_pay")
	if _, err := channelService.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
		Channel: "alipay",
		Name:    "支付宝沙箱",
		Enabled: true,
		Env:     "sandbox",
		Config:  map[string]any{"app_id": "app-1"},
	}); err != nil {
		t.Fatalf("CreateChannelAccount(alipay) error = %v", err)
	}
	body := map[string]any{
		"pay_method": "alipay",
		"channel":    "alipay",
		"return_url": "https://example.com/return",
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, checkoutRequest(t, orderService, created, http.MethodPost, "/orders/"+created.GatewayOrderNo+"/pay", body))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data := response["data"].(map[string]any)
	payment := data["payment"].(map[string]any)
	if payment["status"] != "pending" || payment["pay_method"] != "alipay" || payment["channel"] != "alipay" {
		t.Fatalf("payment = %#v, want pending alipay/alipay", payment)
	}
	if payment["provider_order_no"] != "provider_"+created.GatewayOrderNo {
		t.Fatalf("provider_order_no = %#v, want provider gateway order", payment["provider_order_no"])
	}
	if payment["pay_url"] == "" {
		t.Fatalf("payment = %#v, want pay_url", payment)
	}
}

func TestCheckoutHandlerUsesRoutingRulesForUnlockedOrder(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	client := enttest.Open(t, dialect.SQLite, "file:checkout_routing_unlocked?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { client.Close() })
	createEnabledApp(t, client, "snsgo")

	orderService := ordersvc.New(client)
	order, err := orderService.CreateOrder(t.Context(), ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_checkout_routing_unlocked",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	channelService := channelsvc.New(client)
	if _, err := channelService.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
		Channel: "alipay",
		Name:    "支付宝",
		Enabled: true,
		Env:     "sandbox",
		Config:  map[string]any{"app_id": "app-1"},
	}); err != nil {
		t.Fatalf("CreateChannelAccount(alipay) error = %v", err)
	}
	wechatAccount, err := channelService.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
		Channel: "wechat",
		Name:    "微信",
		Enabled: true,
		Env:     "prod",
		Config:  map[string]any{"app_id": "wx-1", "mode": "native"},
	})
	if err != nil {
		t.Fatalf("CreateChannelAccount(wechat) error = %v", err)
	}
	routingService := routingsvc.New(client)
	if _, err := routingService.CreateRule(t.Context(), routingsvc.ManageRuleInput{
		Name:          "CNY 微信优先",
		Enabled:       true,
		Priority:      100,
		Currency:      "CNY",
		Terminal:      "any",
		PaymentMethod: "wechat",
		PayModes:      []string{"native"},
		Targets: []routingsvc.ManageTargetInput{{
			ChannelAccountID: wechatAccount.ID,
			Enabled:          true,
		}},
	}); err != nil {
		t.Fatalf("CreateRule() error = %v", err)
	}

	router := gin.New()
	checkoutHandler := orderhandler.NewCheckout(
		orderService,
		orderhandler.WithChannelService(channelService),
		orderhandler.WithRoutingService(routingService),
	)
	router.GET("/orders/:gateway_order_no/methods", checkoutHandler.ListPaymentMethods)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, checkoutRequest(t, orderService, order, http.MethodGet, "/orders/"+order.GatewayOrderNo+"/methods", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	methods := decodeCheckoutMethods(t, recorder)
	if len(methods) != 1 {
		t.Fatalf("methods len = %d, want routed single method", len(methods))
	}
	first := methods[0].(map[string]any)
	if first["channel"] != "wechat" || first["pay_method"] != "wechat" || first["pay_mode"] != "native" {
		t.Fatalf("first method = %#v, want routed wechat native", first)
	}
	if first["channel_account_id"].(float64) != float64(wechatAccount.ID) {
		t.Fatalf("first method = %#v, want routed account %d", first, wechatAccount.ID)
	}
}

func TestCheckoutHandlerFallsBackToEnabledChannelsWhenNoRoutingRulesExist(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	client := enttest.Open(t, dialect.SQLite, "file:checkout_routing_fallback?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { client.Close() })
	createEnabledApp(t, client, "snsgo")

	orderService := ordersvc.New(client)
	order, err := orderService.CreateOrder(t.Context(), ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_checkout_routing_fallback",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	channelService := channelsvc.New(client)
	if _, err := channelService.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
		Channel: "alipay",
		Name:    "支付宝",
		Enabled: true,
		Env:     "sandbox",
		Config:  map[string]any{"app_id": "app-1"},
	}); err != nil {
		t.Fatalf("CreateChannelAccount(alipay) error = %v", err)
	}
	router := gin.New()
	checkoutHandler := orderhandler.NewCheckout(
		orderService,
		orderhandler.WithChannelService(channelService),
		orderhandler.WithRoutingService(routingsvc.New(client)),
	)
	router.GET("/orders/:gateway_order_no/methods", checkoutHandler.ListPaymentMethods)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, checkoutRequest(t, orderService, order, http.MethodGet, "/orders/"+order.GatewayOrderNo+"/methods", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	methods := decodeCheckoutMethods(t, recorder)
	if len(methods) != 1 {
		t.Fatalf("methods len = %d, want fallback enabled channels", len(methods))
	}
}

func TestCheckoutHandlerKeepsLockedOrderIndependentFromRoutingRules(t *testing.T) {
	router, created, orderService, channelService := newCheckoutPaymentTestRouter(t, "checkout_routing_locked")
	if _, err := channelService.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
		Channel: "alipay",
		Name:    "支付宝",
		Enabled: true,
		Env:     "sandbox",
		Config:  map[string]any{"app_id": "app-1"},
	}); err != nil {
		t.Fatalf("CreateChannelAccount(alipay) error = %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, checkoutRequest(t, orderService, created, http.MethodGet, "/orders/"+created.GatewayOrderNo+"/methods", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	methods := decodeCheckoutMethods(t, recorder)
	if len(methods) != 1 {
		t.Fatalf("methods len = %d, want locked single method", len(methods))
	}
	first := methods[0].(map[string]any)
	if first["channel"] != "alipay" {
		t.Fatalf("first method = %#v, want locked alipay", first)
	}
}

func TestCheckoutHandlerSkipsRoutedChannelWhenAccountDisabled(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	client := enttest.Open(t, dialect.SQLite, "file:checkout_routing_disabled_account?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { client.Close() })
	createEnabledApp(t, client, "snsgo")

	orderService := ordersvc.New(client)
	order, err := orderService.CreateOrder(t.Context(), ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_checkout_routing_disabled_account",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	channelService := channelsvc.New(client)
	disabledAccount, err := channelService.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
		Channel: "wechat",
		Name:    "微信",
		Enabled: false,
		Env:     "prod",
		Config:  map[string]any{"app_id": "wx-1"},
	})
	if err != nil {
		t.Fatalf("CreateChannelAccount(wechat) error = %v", err)
	}
	routingService := routingsvc.New(client)
	if _, err := routingService.CreateRule(t.Context(), routingsvc.ManageRuleInput{
		Name:          "CNY 微信",
		Enabled:       true,
		Priority:      100,
		Currency:      "CNY",
		Terminal:      "any",
		PaymentMethod: "wechat",
		PayModes:      []string{"native"},
		Targets: []routingsvc.ManageTargetInput{{
			ChannelAccountID: disabledAccount.ID,
			Enabled:          true,
		}},
	}); err != nil {
		t.Fatalf("CreateRule() error = %v", err)
	}

	router := gin.New()
	checkoutHandler := orderhandler.NewCheckout(
		orderService,
		orderhandler.WithChannelService(channelService),
		orderhandler.WithRoutingService(routingService),
	)
	router.GET("/orders/:gateway_order_no/methods", checkoutHandler.ListPaymentMethods)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, checkoutRequest(t, orderService, order, http.MethodGet, "/orders/"+order.GatewayOrderNo+"/methods", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	methods := decodeCheckoutMethods(t, recorder)
	if len(methods) != 0 {
		t.Fatalf("methods len = %d, want routed disabled channel skipped", len(methods))
	}
}

func TestCheckoutHandlerPersistsSelectedMethodWhenOrderHasNoMethod(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	client := enttest.Open(t, dialect.SQLite, "file:checkout_persist_selected_method?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { client.Close() })
	createEnabledApp(t, client, "snsgo")

	orderService := ordersvc.New(client)
	order, err := orderService.CreateOrder(t.Context(), ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_checkout_persist_method",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "USD",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}

	channelService := channelsvc.New(client)
	account, err := channelService.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
		Channel: "paypal",
		Name:    "PayPal Sandbox",
		Enabled: true,
		Env:     "sandbox",
		Config:  map[string]any{"client_id": "client-1"},
	})
	if err != nil {
		t.Fatalf("CreateChannelAccount(paypal) error = %v", err)
	}
	provider := &checkoutFakeProvider{channel: "paypal"}
	paymentService := paymentsvc.New(
		paymentsvc.WithChannelRepository(channelrepo.New(client)),
		paymentsvc.WithProvider(provider),
	)

	router := gin.New()
	checkoutHandler := orderhandler.NewCheckout(
		orderService,
		orderhandler.WithChannelService(channelService),
		orderhandler.WithPaymentService(paymentService),
	)
	router.POST("/orders/:gateway_order_no/pay", checkoutHandler.StartPayment)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, checkoutRequest(t, orderService, order, http.MethodPost, "/orders/"+order.GatewayOrderNo+"/pay", map[string]any{
		"pay_method": "paypal",
		"channel":    "paypal",
	}))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	updated, err := orderService.FindOrderByGatewayOrderNo(t.Context(), order.GatewayOrderNo)
	if err != nil {
		t.Fatalf("FindOrderByGatewayOrderNo() error = %v", err)
	}
	if updated.Channel != "paypal" || updated.PayMethod != "paypal" || updated.ChannelAccountID != account.ID {
		t.Fatalf("updated order = %#v, want paypal account %d locked after payment start", updated, account.ID)
	}
}

func TestCheckoutHandlerResolvesPayModeFromSelectedAccount(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	client := enttest.Open(t, dialect.SQLite, "file:checkout_selected_account_mode?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { client.Close() })
	createEnabledApp(t, client, "snsgo")

	orderService := ordersvc.New(client)
	order := createUnlockedCheckoutOrder(t, orderService, "selected_account_mode")
	channelService := channelsvc.New(client)
	selected, err := channelService.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
		Channel: "alipay", Name: "Selected", Enabled: true, Env: "prod", Config: map[string]any{"app_id": "selected", "private_key": "private", "enable_page_pay": true, "enable_qr_pay": false},
	})
	if err != nil {
		t.Fatalf("Create selected account error = %v", err)
	}
	if _, err := channelService.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
		Channel: "alipay", Name: "Other", Enabled: true, Env: "prod", Config: map[string]any{"app_id": "other", "private_key": "private", "enable_page_pay": false, "enable_qr_pay": true},
	}); err != nil {
		t.Fatalf("Create other account error = %v", err)
	}
	provider := &checkoutFakeProvider{channel: "alipay"}
	paymentService := paymentsvc.New(paymentsvc.WithChannelRepository(channelrepo.New(client)), paymentsvc.WithProvider(provider))
	router := gin.New()
	router.POST("/orders/:gateway_order_no/pay", orderhandler.NewCheckout(orderService, orderhandler.WithChannelService(channelService), orderhandler.WithPaymentService(paymentService)).StartPayment)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, checkoutRequest(t, orderService, order, http.MethodPost, "/orders/"+order.GatewayOrderNo+"/pay", map[string]any{
		"channel": "alipay", "pay_method": "alipay", "channel_account_id": selected.ID,
	}))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if provider.req.ChannelAccount.ID != selected.ID || provider.req.ChannelAccount.Config["mode"] != "page" {
		t.Fatalf("provider account/mode = %d/%v, want %d/page", provider.req.ChannelAccount.ID, provider.req.ChannelAccount.Config["mode"], selected.ID)
	}
}

func TestCheckoutHandlerUsesLockedMethodWhenRequestOmitsMethod(t *testing.T) {
	router, created, orderService, channelService := newCheckoutPaymentTestRouter(t, "checkout_locked_pay_empty_request")
	if _, err := channelService.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
		Channel: "alipay",
		Name:    "支付宝沙箱",
		Enabled: true,
		Env:     "sandbox",
		Config:  map[string]any{"app_id": "app-1"},
	}); err != nil {
		t.Fatalf("CreateChannelAccount(alipay) error = %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, checkoutRequest(t, orderService, created, http.MethodPost, "/orders/"+created.GatewayOrderNo+"/pay", map[string]any{}))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestCheckoutHandlerRejectsMismatchedMethodForLockedOrder(t *testing.T) {
	router, created, orderService, channelService := newCheckoutPaymentTestRouter(t, "checkout_locked_pay_mismatch")
	if _, err := channelService.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
		Channel: "alipay",
		Name:    "支付宝沙箱",
		Enabled: true,
		Env:     "sandbox",
		Config:  map[string]any{"app_id": "app-1"},
	}); err != nil {
		t.Fatalf("CreateChannelAccount(alipay) error = %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, checkoutRequest(t, orderService, created, http.MethodPost, "/orders/"+created.GatewayOrderNo+"/pay", map[string]any{
		"pay_method": "wechat",
		"channel":    "wechat",
	}))

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	assertResponseCode(t, recorder, httpx.CodeOrderStatusNotAllowed)
}

func TestCheckoutHandlerReturnsStableChannelUnavailableCode(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	client := enttest.Open(t, dialect.SQLite, "file:checkout_channel_unavailable?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { client.Close() })
	createEnabledApp(t, client, "snsgo")

	orderService := ordersvc.New(client)
	created, err := orderService.CreateOrder(t.Context(), ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_checkout_channel_unavailable",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
		PayMethod:       "alipay",
		Channel:         "alipay",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}

	channelService := channelsvc.New(client)
	if _, err := channelService.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
		Channel: "alipay",
		Name:    "支付宝沙箱",
		Enabled: true,
		Env:     "sandbox",
		Config:  map[string]any{"app_id": "app-1"},
	}); err != nil {
		t.Fatalf("CreateChannelAccount(alipay) error = %v", err)
	}

	router := gin.New()
	checkoutHandler := orderhandler.NewCheckout(
		orderService,
		orderhandler.WithChannelService(channelService),
		orderhandler.WithPaymentService(paymentsvc.New(paymentsvc.WithChannelRepository(channelrepo.New(client)))),
	)
	router.POST("/orders/:gateway_order_no/pay", checkoutHandler.StartPayment)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, checkoutRequest(t, orderService, created, http.MethodPost, "/orders/"+created.GatewayOrderNo+"/pay", map[string]any{
		"pay_method": "alipay",
		"channel":    "alipay",
	}))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	assertResponseCode(t, recorder, httpx.CodeChannelUnavailable)
}

func TestCheckoutHandlerRejectsUnsupportedCurrencyAtPaymentStart(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	client := enttest.Open(t, dialect.SQLite, "file:checkout_reject_currency_start?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { client.Close() })
	createEnabledApp(t, client, "snsgo")

	orderService := ordersvc.New(client)
	order, err := orderService.CreateOrder(t.Context(), ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_checkout_reject_currency",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "USD",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}

	channelService := channelsvc.New(client)
	if _, err := channelService.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
		Channel: "alipay",
		Name:    "支付宝沙箱",
		Enabled: true,
		Env:     "sandbox",
		Config:  map[string]any{"app_id": "app-1"},
	}); err != nil {
		t.Fatalf("CreateChannelAccount(alipay) error = %v", err)
	}

	router := gin.New()
	checkoutHandler := orderhandler.NewCheckout(
		orderService,
		orderhandler.WithChannelService(channelService),
		orderhandler.WithPaymentService(paymentsvc.New()),
	)
	router.POST("/orders/:gateway_order_no/pay", checkoutHandler.StartPayment)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, checkoutRequest(t, orderService, order, http.MethodPost, "/orders/"+order.GatewayOrderNo+"/pay", map[string]any{
		"pay_method": "alipay",
		"channel":    "alipay",
	}))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestCheckoutHandlerUsesGatewayPaypalReturnURL(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	client := enttest.Open(t, dialect.SQLite, "file:checkout_paypal_return_url?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { client.Close() })

	createEnabledApp(t, client, "snsgo")

	orderService := ordersvc.New(client)
	order, err := orderService.CreateOrder(t.Context(), ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_paypal_return",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "USD",
		PayMethod:       "paypal",
		Channel:         "paypal",
		ReturnURL:       "https://merchant.example.com/pay/result",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}

	channels := channelsvc.New(client)
	if _, err := channels.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
		Channel: "paypal",
		Name:    "PayPal Sandbox",
		Enabled: true,
		Env:     "sandbox",
		Config: map[string]any{
			"client_id":     "client",
			"client_secret": "secret",
		},
	}); err != nil {
		t.Fatalf("CreateChannelAccount() error = %v", err)
	}
	provider := &checkoutFakeProvider{channel: "paypal"}
	paymentService := paymentsvc.New(
		paymentsvc.WithChannelRepository(channelrepo.New(client)),
		paymentsvc.WithProvider(provider),
	)

	router := gin.New()
	checkoutHandler := orderhandler.NewCheckout(
		orderService,
		orderhandler.WithChannelService(channels),
		orderhandler.WithPaymentService(paymentService),
		orderhandler.WithNotifyURLResolver(func(ctx *gin.Context) string {
			return "https://sns.itlight.cn/v1/channel/notify"
		}),
		orderhandler.WithPaypalReturnURLResolver(func(ctx *gin.Context, gatewayOrderNo string) string {
			return "https://sns.itlight.cn/v1/checkout/paypal/return?gateway_order_no=" + gatewayOrderNo
		}),
	)
	router.POST("/orders/:gateway_order_no/pay", checkoutHandler.StartPayment)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, checkoutRequest(t, orderService, order, http.MethodPost, "/orders/"+order.GatewayOrderNo+"/pay", map[string]any{
		"pay_method": "paypal",
		"channel":    "paypal",
		"return_url": "https://merchant.example.com/ignored-by-paypal-provider",
	}))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.HasPrefix(provider.req.ReturnURL, "https://sns.itlight.cn/v1/checkout/paypal/return?") ||
		!strings.Contains(provider.req.ReturnURL, "gateway_order_no="+order.GatewayOrderNo) ||
		!strings.Contains(provider.req.ReturnURL, "%2Fcheckout%2F"+order.GatewayOrderNo+"%2Fresult") {
		t.Fatalf("provider ReturnURL = %q, want gateway paypal return URL with checkout result return_url", provider.req.ReturnURL)
	}
	notifyURL, err := url.Parse(provider.req.NotifyURL)
	if err != nil {
		t.Fatalf("parse provider NotifyURL error = %v", err)
	}
	if notifyURL.Scheme+"://"+notifyURL.Host+notifyURL.Path != "https://sns.itlight.cn/v1/channel/notify" ||
		notifyURL.Query().Get("channel") != "paypal" || notifyURL.Query().Get("channel_account_id") == "" {
		t.Fatalf("provider NotifyURL = %q, want bound unified notify URL", provider.req.NotifyURL)
	}
}

func TestCheckoutHandlerAppendsOrderReturnURLToPaypalReturnURLWhenRequestOmitsIt(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	client := enttest.Open(t, dialect.SQLite, "file:checkout_paypal_order_return_url?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { client.Close() })

	createEnabledApp(t, client, "snsgo")

	orderService := ordersvc.New(client)
	order, err := orderService.CreateOrder(t.Context(), ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_paypal_order_return",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "USD",
		PayMethod:       "paypal",
		Channel:         "paypal",
		ReturnURL:       "http://127.0.0.1:3000/apps",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}

	channels := channelsvc.New(client)
	if _, err := channels.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
		Channel: "paypal",
		Name:    "PayPal Sandbox",
		Enabled: true,
		Env:     "sandbox",
		Config: map[string]any{
			"client_id":     "client",
			"client_secret": "secret",
		},
	}); err != nil {
		t.Fatalf("CreateChannelAccount() error = %v", err)
	}
	provider := &checkoutFakeProvider{channel: "paypal"}
	paymentService := paymentsvc.New(
		paymentsvc.WithChannelRepository(channelrepo.New(client)),
		paymentsvc.WithProvider(provider),
	)

	router := gin.New()
	checkoutHandler := orderhandler.NewCheckout(
		orderService,
		orderhandler.WithChannelService(channels),
		orderhandler.WithPaymentService(paymentService),
		orderhandler.WithPaypalReturnURLResolver(func(ctx *gin.Context, gatewayOrderNo string) string {
			return "https://sns.itlight.cn/v1/checkout/paypal/return?gateway_order_no=" + gatewayOrderNo
		}),
	)
	router.POST("/orders/:gateway_order_no/pay", checkoutHandler.StartPayment)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, checkoutRequest(t, orderService, order, http.MethodPost, "/orders/"+order.GatewayOrderNo+"/pay", map[string]any{
		"pay_method": "paypal",
		"channel":    "paypal",
	}))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.HasPrefix(provider.req.ReturnURL, "https://sns.itlight.cn/v1/checkout/paypal/return?") ||
		!strings.Contains(provider.req.ReturnURL, "gateway_order_no="+order.GatewayOrderNo) ||
		!strings.Contains(provider.req.ReturnURL, "%2Fcheckout%2F"+order.GatewayOrderNo+"%2Fresult") {
		t.Fatalf("provider ReturnURL = %q, want gateway paypal return URL with checkout result return_url", provider.req.ReturnURL)
	}
}

func TestCheckoutHandlerRedirectsPaypalReturnToFallbackReturnURL(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	client := enttest.Open(t, dialect.SQLite, "file:checkout_paypal_return_fallback?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { client.Close() })

	createEnabledApp(t, client, "snsgo")

	orderService := ordersvc.New(client)
	order, err := orderService.CreateOrder(t.Context(), ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_paypal_return_fallback",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "USD",
		PayMethod:       "paypal",
		Channel:         "paypal",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}

	router := gin.New()
	checkoutHandler := orderhandler.NewCheckout(orderService, orderhandler.WithPaymentService(paymentsvc.New()))
	router.GET("/paypal/return", checkoutHandler.CompletePaypalPayment)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet,
		"/paypal/return?gateway_order_no="+order.GatewayOrderNo+"&cancel=1&return_url=https%3A%2F%2Fadmin.example.com%2Fcheckout%2F"+order.GatewayOrderNo,
		nil,
	)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusFound {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	want := "https://admin.example.com/checkout/" + order.GatewayOrderNo + "?gateway_order_no=" + order.GatewayOrderNo + "&status=cancelled"
	if recorder.Header().Get("Location") != want {
		t.Fatalf("Location = %q, want %q", recorder.Header().Get("Location"), want)
	}
}

func TestCheckoutHandlerPaypalReturnPrefersGatewayResultURL(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	client := enttest.Open(t, dialect.SQLite, "file:checkout_paypal_return_prefers_query?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { client.Close() })

	createEnabledApp(t, client, "snsgo")

	orderService := ordersvc.New(client)
	order, err := orderService.CreateOrder(t.Context(), ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_paypal_return_prefers_query",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "USD",
		PayMethod:       "paypal",
		Channel:         "paypal",
		ReturnURL:       "https://merchant.example.com/app-return",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}

	router := gin.New()
	checkoutHandler := orderhandler.NewCheckout(orderService, orderhandler.WithPaymentService(paymentsvc.New()))
	router.GET("/paypal/return", checkoutHandler.CompletePaypalPayment)

	resultURL := "https://pay.example.com/checkout/" + order.GatewayOrderNo + "/result?token=checkout-token"
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet,
		"/paypal/return?gateway_order_no="+order.GatewayOrderNo+"&cancel=1&return_url="+url.QueryEscape(resultURL),
		nil,
	)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusFound {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	location, err := url.Parse(recorder.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	if location.Scheme+"://"+location.Host+location.Path != "https://pay.example.com/checkout/"+order.GatewayOrderNo+"/result" ||
		location.Query().Get("token") != "checkout-token" ||
		location.Query().Get("gateway_order_no") != order.GatewayOrderNo ||
		location.Query().Get("status") != "cancelled" {
		t.Fatalf("Location = %q, want gateway result URL with token and status", recorder.Header().Get("Location"))
	}
}

func TestCheckoutHandlerRejectsMismatchedPaypalReturnToken(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	client := enttest.Open(t, dialect.SQLite, "file:checkout_paypal_token_mismatch?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { client.Close() })
	createEnabledApp(t, client, "snsgo")

	orderService := ordersvc.New(client)
	order, err := orderService.CreateOrder(t.Context(), ordersvc.ManageOrderInput{
		AppID: "snsgo", MerchantOrderNo: "biz_paypal_token_mismatch", Subject: "Pro", Amount: 9900, Currency: "USD", Channel: "paypal", PayMethod: "paypal",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	channels := channelrepo.New(client)
	account, err := channels.Create(t.Context(), channelrepo.CreateChannelAccountInput{Channel: "paypal", Name: "PayPal", Enabled: true, Env: "sandbox", Config: map[string]any{}})
	if err != nil {
		t.Fatalf("Create channel account error = %v", err)
	}
	if _, err := orderService.SetPaymentSelection(t.Context(), order.ID, "paypal", "paypal", account.ID); err != nil {
		t.Fatalf("SetPaymentSelection() error = %v", err)
	}
	if _, err := orderService.SetProviderOrderNo(t.Context(), order.ID, "PAYPAL_ORDER_EXPECTED"); err != nil {
		t.Fatalf("SetProviderOrderNo() error = %v", err)
	}
	provider := &checkoutFakeProvider{channel: "paypal"}
	paymentService := paymentsvc.New(paymentsvc.WithChannelRepository(channels), paymentsvc.WithProvider(provider))
	router := gin.New()
	router.GET("/paypal/return", orderhandler.NewCheckout(orderService, orderhandler.WithPaymentService(paymentService)).CompletePaypalPayment)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/paypal/return?gateway_order_no="+order.GatewayOrderNo+"&token=PAYPAL_ORDER_OTHER", nil))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if provider.captureCalled {
		t.Fatal("CapturePayment() was called for mismatched token")
	}
}

func TestCheckoutHandlerValidatesPaypalCaptureBeforeMarkingPaid(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	client := enttest.Open(t, dialect.SQLite, "file:checkout_paypal_capture_validation?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { client.Close() })
	createEnabledApp(t, client, "snsgo")

	orderService := ordersvc.New(client)
	order, err := orderService.CreateOrder(t.Context(), ordersvc.ManageOrderInput{
		AppID: "snsgo", MerchantOrderNo: "biz_paypal_capture_validation", Subject: "Pro", Amount: 9900, Currency: "USD", Channel: "paypal", PayMethod: "paypal", ReturnURL: "https://merchant.example.com/result",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	channels := channelrepo.New(client)
	account, err := channels.Create(t.Context(), channelrepo.CreateChannelAccountInput{Channel: "paypal", Name: "PayPal", Enabled: true, Env: "sandbox", Config: map[string]any{}})
	if err != nil {
		t.Fatalf("Create channel account error = %v", err)
	}
	if _, err := orderService.SetPaymentSelection(t.Context(), order.ID, "paypal", "paypal", account.ID); err != nil {
		t.Fatalf("SetPaymentSelection() error = %v", err)
	}
	if _, err := orderService.SetProviderOrderNo(t.Context(), order.ID, "PAYPAL_ORDER_001"); err != nil {
		t.Fatalf("SetProviderOrderNo() error = %v", err)
	}
	provider := &checkoutFakeProvider{channel: "paypal", captureResult: &paymentprovider.CapturePaymentResult{
		Channel: "paypal", ProviderOrderNo: "PAYPAL_ORDER_001", GatewayOrderNo: order.GatewayOrderNo, ChannelTradeNo: "CAPTURE_001", Status: "paid", Amount: 9900, Currency: "USD",
	}}
	paymentService := paymentsvc.New(paymentsvc.WithChannelRepository(channels), paymentsvc.WithProvider(provider))
	router := gin.New()
	router.GET("/paypal/return", orderhandler.NewCheckout(orderService, orderhandler.WithPaymentService(paymentService)).CompletePaypalPayment)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/paypal/return?gateway_order_no="+order.GatewayOrderNo+"&token=PAYPAL_ORDER_001", nil))

	if recorder.Code != http.StatusFound {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	updated, err := orderService.FindOrder(t.Context(), order.ID)
	if err != nil {
		t.Fatalf("FindOrder() error = %v", err)
	}
	if updated.Status != "paid" || updated.ChannelTradeNo != "CAPTURE_001" {
		t.Fatalf("updated order = %#v, want paid capture", updated)
	}
	if provider.captureReq.ChannelAccountID != account.ID {
		t.Fatalf("capture account = %d, want %d", provider.captureReq.ChannelAccountID, account.ID)
	}
}

func newCheckoutPaymentTestRouter(t *testing.T, dbName string) (*gin.Engine, *ent.PaymentOrder, ordersvc.Service, channelsvc.Service) {
	t.Helper()
	gin.SetMode(gin.ReleaseMode)
	client := enttest.Open(t, dialect.SQLite, "file:"+dbName+"?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { client.Close() })

	createEnabledApp(t, client, "snsgo")

	svc := ordersvc.New(client)
	channelService := channelsvc.New(client)
	created, err := svc.CreateOrder(t.Context(), ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_" + dbName,
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
		PayMethod:       "alipay",
		Channel:         "alipay",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}

	router := gin.New()
	provider := &checkoutFakeProvider{channel: "alipay"}
	paymentService := paymentsvc.New(
		paymentsvc.WithChannelRepository(channelrepo.New(client)),
		paymentsvc.WithProvider(provider),
	)
	checkoutHandler := orderhandler.NewCheckout(
		svc,
		orderhandler.WithChannelService(channelService),
		orderhandler.WithPaymentService(paymentService),
	)
	router.GET("/orders/:gateway_order_no/methods", checkoutHandler.ListPaymentMethods)
	router.POST("/orders/:gateway_order_no/pay", checkoutHandler.StartPayment)
	return router, created, svc, channelService
}

func decodeCheckoutMethods(t *testing.T, recorder *httptest.ResponseRecorder) []any {
	t.Helper()
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data := response["data"].(map[string]any)
	return data["methods"].([]any)
}

func assertResponseCode(t *testing.T, recorder *httptest.ResponseRecorder, want string) {
	t.Helper()
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["code"] != want {
		t.Fatalf("code = %#v, want %s; body = %s", response["code"], want, recorder.Body.String())
	}
}

func checkoutRequest(t *testing.T, service ordersvc.Service, order *ent.PaymentOrder, method string, path string, body map[string]any) *http.Request {
	t.Helper()
	req := jsonRequest(method, path, body)
	req.Header.Set("X-Checkout-Token", checkoutTokenForOrder(t, service, order))
	return req
}

func checkoutTokenForOrder(t *testing.T, service ordersvc.Service, order *ent.PaymentOrder) string {
	t.Helper()
	token, err := ordersvc.NewCheckoutToken()
	if err != nil {
		t.Fatalf("NewCheckoutToken() error = %v", err)
	}
	if _, err := service.SetCheckoutTokenHash(t.Context(), order.ID, ordersvc.HashCheckoutToken(token)); err != nil {
		t.Fatalf("SetCheckoutTokenHash() error = %v", err)
	}
	return token
}

func createUnlockedCheckoutOrder(t *testing.T, service ordersvc.Service, suffix string) *ent.PaymentOrder {
	t.Helper()
	order, err := service.CreateOrder(t.Context(), ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_unlocked_" + suffix,
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
	})
	if err != nil {
		t.Fatalf("CreateOrder(unlocked) error = %v", err)
	}
	return order
}

type checkoutFakeProvider struct {
	channel       string
	req           paymentprovider.StartPaymentRequest
	captureCalled bool
	captureReq    paymentprovider.CapturePaymentRequest
	captureResult *paymentprovider.CapturePaymentResult
}

func (p *checkoutFakeProvider) Channel() string {
	return p.channel
}

func (p *checkoutFakeProvider) CapturePayment(ctx context.Context, req paymentprovider.CapturePaymentRequest) (*paymentprovider.CapturePaymentResult, error) {
	_ = ctx
	p.captureCalled = true
	p.captureReq = req
	return p.captureResult, nil
}

func (p *checkoutFakeProvider) StartPayment(ctx context.Context, req paymentprovider.StartPaymentRequest) (*paymentprovider.StartPaymentResult, error) {
	_ = ctx
	p.req = req
	return &paymentprovider.StartPaymentResult{
		Status:          "pending",
		Channel:         req.Channel,
		PayMethod:       req.PayMethod,
		ProviderOrderNo: "provider_" + req.Order.GatewayOrderNo,
		PayURL:          "https://paypal.example.com/checkout?token=" + req.Order.GatewayOrderNo,
	}, nil
}
