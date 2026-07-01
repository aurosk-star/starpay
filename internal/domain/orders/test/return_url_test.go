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

	"payment-gateway/ent/enttest"
	appsvc "payment-gateway/internal/domain/apps/service"
	channelrepo "payment-gateway/internal/domain/channels/repository"
	channelsvc "payment-gateway/internal/domain/channels/service"
	orderhandler "payment-gateway/internal/domain/orders/handler"
	ordersvc "payment-gateway/internal/domain/orders/service"
	paymentsvc "payment-gateway/internal/domain/payments/service"
)

func TestOpenOrderStoresRequestReturnURL(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:open_order_return_url?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	createEnabledApp(t, client, "snsgo")

	svc := ordersvc.New(client)
	order, _, err := svc.CreateOpenOrder(ctx, "snsgo", ordersvc.OpenOrderInput{
		MerchantOrderNo: "biz_return_001",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
		PayMethod:       "alipay",
		ReturnURL:       "https://snsgo.example.com/payment/result",
	})
	if err != nil {
		t.Fatalf("CreateOpenOrder() error = %v", err)
	}
	if order.ReturnURL != "https://snsgo.example.com/payment/result" {
		t.Fatalf("ReturnURL = %q, want request return_url", order.ReturnURL)
	}
}

func TestOpenOrderUsesAppDefaultReturnURLWhenRequestOmitsIt(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:open_order_app_default_return_url?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	appService := appsvc.New(client)
	if _, err := appService.CreateApp(ctx, appsvc.ManageAppInput{
		AppID:            "snsgo",
		Name:             "Snsgo",
		DefaultReturnURL: "https://snsgo.example.com/default-result",
		Status:           "enabled",
	}); err != nil {
		t.Fatalf("CreateApp() error = %v", err)
	}

	orderService := ordersvc.New(client)
	order, _, err := orderService.CreateOpenOrder(ctx, "snsgo", ordersvc.OpenOrderInput{
		MerchantOrderNo: "biz_return_002",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
		PayMethod:       "alipay",
	})
	if err != nil {
		t.Fatalf("CreateOpenOrder() error = %v", err)
	}
	if order.ReturnURL != "https://snsgo.example.com/default-result" {
		t.Fatalf("ReturnURL = %q, want app default return URL", order.ReturnURL)
	}
}

func TestCheckoutPaymentUsesPersistedOrderReturnURL(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	client := enttest.Open(t, dialect.SQLite, "file:checkout_uses_order_return_url?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	createEnabledApp(t, client, "snsgo")

	svc := ordersvc.New(client)
	order, err := svc.CreateOrder(t.Context(), ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_return_checkout",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
		PayMethod:       "alipay",
		Channel:         "alipay",
		ReturnURL:       "https://snsgo.example.com/order-return",
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
	provider := &checkoutFakeProvider{channel: "alipay"}
	paymentService := paymentsvc.New(
		paymentsvc.WithChannelRepository(channelrepo.New(client)),
		paymentsvc.WithProvider(provider),
	)

	router := gin.New()
	checkoutHandler := orderhandler.NewCheckout(
		svc,
		orderhandler.WithChannelService(channelService),
		orderhandler.WithPaymentService(paymentService),
	)
	router.POST("/orders/:gateway_order_no/pay", checkoutHandler.StartPayment)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, checkoutRequest(t, svc, order, http.MethodPost, "/orders/"+order.GatewayOrderNo+"/pay", map[string]any{
		"pay_method": "alipay",
		"channel":    "alipay",
	}))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if provider.req.ReturnURL != "https://snsgo.example.com/order-return" {
		t.Fatalf("provider ReturnURL = %q, want persisted order return URL", provider.req.ReturnURL)
	}
}
