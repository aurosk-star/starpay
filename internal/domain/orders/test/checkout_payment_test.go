package orderstest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
)

func TestCheckoutHandlerListsOnlyEnabledPaymentMethods(t *testing.T) {
	router, created, _, channelService := newCheckoutPaymentTestRouter(t, "checkout_methods")
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
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/orders/"+created.GatewayOrderNo+"/methods", nil))

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

func TestCheckoutHandlerHidesMethodsWhenNoChannelEnabled(t *testing.T) {
	router, created, _, channelService := newCheckoutPaymentTestRouter(t, "checkout_methods_empty")
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
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/orders/"+created.GatewayOrderNo+"/methods", nil))

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
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/orders/"+created.GatewayOrderNo+"/methods", nil))

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
	router, created, _, channelService := newCheckoutPaymentTestRouter(t, "checkout_methods_locked_state")
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
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/orders/"+created.GatewayOrderNo+"/methods", nil))

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
	router, created, _, channelService := newCheckoutPaymentTestRouter(t, "checkout_methods_locked")
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
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/orders/"+created.GatewayOrderNo+"/methods", nil))

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
	router, created, _, _ := newCheckoutPaymentTestRouter(t, "checkout_pay")
	body := map[string]any{
		"pay_method": "alipay",
		"channel":    "alipay",
		"return_url": "https://example.com/return",
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, jsonRequest(http.MethodPost, "/orders/"+created.GatewayOrderNo+"/pay", body))

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
	if payment["provider_order_no"] != "mock_"+created.GatewayOrderNo {
		t.Fatalf("provider_order_no = %#v, want mock gateway order", payment["provider_order_no"])
	}
	if payment["pay_url"] == "" {
		t.Fatalf("payment = %#v, want pay_url", payment)
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
	if _, err := channelService.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
		Channel: "paypal",
		Name:    "PayPal Sandbox",
		Enabled: true,
		Env:     "sandbox",
		Config:  map[string]any{"client_id": "client-1"},
	}); err != nil {
		t.Fatalf("CreateChannelAccount(paypal) error = %v", err)
	}
	provider := &checkoutFakeProvider{channel: "paypal"}
	paymentService := paymentsvc.New(
		paymentsvc.WithChannelRepository(channelrepo.New(client)),
		paymentsvc.WithProvider(provider),
		paymentsvc.WithMockFallback(false),
	)

	router := gin.New()
	checkoutHandler := orderhandler.NewCheckout(
		orderService,
		orderhandler.WithChannelService(channelService),
		orderhandler.WithPaymentService(paymentService),
	)
	router.POST("/orders/:gateway_order_no/pay", checkoutHandler.StartPayment)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, jsonRequest(http.MethodPost, "/orders/"+order.GatewayOrderNo+"/pay", map[string]any{
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
	if updated.Channel != "paypal" || updated.PayMethod != "paypal" {
		t.Fatalf("updated order = %#v, want paypal locked after payment start", updated)
	}
}

func TestCheckoutHandlerUsesLockedMethodWhenRequestOmitsMethod(t *testing.T) {
	router, created, _, channelService := newCheckoutPaymentTestRouter(t, "checkout_locked_pay_empty_request")
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
	router.ServeHTTP(recorder, jsonRequest(http.MethodPost, "/orders/"+created.GatewayOrderNo+"/pay", map[string]any{}))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestCheckoutHandlerRejectsMismatchedMethodForLockedOrder(t *testing.T) {
	router, created, _, channelService := newCheckoutPaymentTestRouter(t, "checkout_locked_pay_mismatch")
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
	router.ServeHTTP(recorder, jsonRequest(http.MethodPost, "/orders/"+created.GatewayOrderNo+"/pay", map[string]any{
		"pay_method": "wechat",
		"channel":    "wechat",
	}))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
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
		orderhandler.WithPaymentService(paymentsvc.New(paymentsvc.WithMockFallback(true))),
	)
	router.POST("/orders/:gateway_order_no/pay", checkoutHandler.StartPayment)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, jsonRequest(http.MethodPost, "/orders/"+order.GatewayOrderNo+"/pay", map[string]any{
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
		paymentsvc.WithMockFallback(false),
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
	router.ServeHTTP(recorder, jsonRequest(http.MethodPost, "/orders/"+order.GatewayOrderNo+"/pay", map[string]any{
		"pay_method": "paypal",
		"channel":    "paypal",
		"return_url": "https://merchant.example.com/ignored-by-paypal-provider",
	}))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	wantPaypalReturnURL := "https://sns.itlight.cn/v1/checkout/paypal/return?gateway_order_no=" + order.GatewayOrderNo + "&return_url=https%3A%2F%2Fmerchant.example.com%2Fignored-by-paypal-provider"
	if provider.req.ReturnURL != wantPaypalReturnURL {
		t.Fatalf("provider ReturnURL = %q, want gateway paypal return URL", provider.req.ReturnURL)
	}
	if provider.req.NotifyURL != "https://sns.itlight.cn/v1/channel/notify" {
		t.Fatalf("provider NotifyURL = %q, want unified notify URL", provider.req.NotifyURL)
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
		paymentsvc.WithMockFallback(false),
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
	router.ServeHTTP(recorder, jsonRequest(http.MethodPost, "/orders/"+order.GatewayOrderNo+"/pay", map[string]any{
		"pay_method": "paypal",
		"channel":    "paypal",
	}))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	wantPaypalReturnURL := "https://sns.itlight.cn/v1/checkout/paypal/return?gateway_order_no=" + order.GatewayOrderNo + "&return_url=http%3A%2F%2F127.0.0.1%3A3000%2Fapps"
	if provider.req.ReturnURL != wantPaypalReturnURL {
		t.Fatalf("provider ReturnURL = %q, want gateway paypal return URL with order return URL", provider.req.ReturnURL)
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

func TestCheckoutHandlerCompletesMockPayment(t *testing.T) {
	router, created, _, _ := newCheckoutPaymentTestRouter(t, "checkout_complete")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/mock-pay/"+created.GatewayOrderNo+"/complete", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data := response["data"].(map[string]any)
	order := data["order"].(map[string]any)
	if order["status"] != "paid" {
		t.Fatalf("order.status = %#v, want paid", order["status"])
	}
	if order["channel_trade_no"] != "mock_"+created.GatewayOrderNo {
		t.Fatalf("channel_trade_no = %#v, want mock trade no", order["channel_trade_no"])
	}
}

func TestCheckoutHandlerRejectsMockCompleteForClosedOrder(t *testing.T) {
	router, created, svc, _ := newCheckoutPaymentTestRouter(t, "checkout_complete_closed")

	if _, err := svc.CloseOrder(t.Context(), created.ID); err != nil {
		t.Fatalf("CloseOrder() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/mock-pay/"+created.GatewayOrderNo+"/complete", nil))

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
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
	checkoutHandler := orderhandler.NewCheckout(svc, orderhandler.WithChannelService(channelService))
	router.GET("/orders/:gateway_order_no/methods", checkoutHandler.ListPaymentMethods)
	router.POST("/orders/:gateway_order_no/pay", checkoutHandler.StartPayment)
	router.POST("/mock-pay/:gateway_order_no/complete", checkoutHandler.CompleteMockPayment)
	return router, created, svc, channelService
}

type checkoutFakeProvider struct {
	channel string
	req     paymentprovider.StartPaymentRequest
}

func (p *checkoutFakeProvider) Channel() string {
	return p.channel
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
