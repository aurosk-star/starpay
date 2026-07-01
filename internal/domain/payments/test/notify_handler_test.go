package paymentstest

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"

	"payment-gateway/ent/enttest"
	channelrepo "payment-gateway/internal/domain/channels/repository"
	ordersvc "payment-gateway/internal/domain/orders/service"
	paymenthandler "payment-gateway/internal/domain/payments/handler"
	paymentprovider "payment-gateway/internal/domain/payments/provider"
	paymentrouter "payment-gateway/internal/domain/payments/router"
	paymentsvc "payment-gateway/internal/domain/payments/service"
	webhookrepo "payment-gateway/internal/domain/webhooks/repository"
	webhooksvc "payment-gateway/internal/domain/webhooks/service"
)

func TestNotifyHandlerMarksOrderPaidAndReturnsAlipaySuccess(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	ctx := t.Context()
	client := enttest.Open(t, dialect.SQLite, "file:notify_handler_paid?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createEnabledAppWithNotifyURL(t, client, "snsgo", "https://merchant.example.com/payment/webhook")

	orderService := ordersvc.New(client, ordersvc.WithWebhookService(webhooksvc.New(client)))
	order, err := orderService.CreateOrder(ctx, ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_notify_handler",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
		PayMethod:       "alipay",
		Channel:         "alipay",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}

	channels := channelrepo.New(client)
	if _, err := channels.Create(ctx, channelrepo.CreateChannelAccountInput{
		Channel: "alipay",
		Name:    "Alipay Sandbox",
		Enabled: true,
		Env:     "sandbox",
		Config:  map[string]any{"app_id": "app-1"},
	}); err != nil {
		t.Fatalf("Create channel account error = %v", err)
	}
	provider := &fakeNotifyProvider{
		channel: "alipay",
		result: &paymentprovider.NotifyResult{
			Channel:        "alipay",
			GatewayOrderNo: order.GatewayOrderNo,
			ChannelTradeNo: "2026070122000000001",
			Status:         "paid",
			Amount:         9900,
			Currency:       "CNY",
		},
	}
	paymentService := paymentsvc.New(
		paymentsvc.WithChannelRepository(channels),
		paymentsvc.WithProvider(provider),
	)

	router := gin.New()
	paymentrouter.RegisterNotify(router.Group("/v1/channel"), paymenthandler.NewNotify(paymentService, orderService))

	form := url.Values{}
	form.Set("out_trade_no", order.GatewayOrderNo)
	req := httptest.NewRequest(http.MethodPost, "/v1/channel/notify", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Body.String() != "success" {
		t.Fatalf("body = %q, want success", recorder.Body.String())
	}
	updated, err := orderService.FindOrder(ctx, order.ID)
	if err != nil {
		t.Fatalf("FindOrder() error = %v", err)
	}
	if updated.Status != "paid" || updated.ChannelTradeNo != "2026070122000000001" || updated.PaidAt == nil {
		t.Fatalf("updated order = %#v, want paid with channel trade no", updated)
	}
	_, totalEvents, err := webhookrepo.New(client).ListEvents(ctx, webhookrepo.ListEventsInput{})
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	deliveries, totalDeliveries, err := webhookrepo.New(client).ListDeliveries(ctx, webhookrepo.ListDeliveriesInput{})
	if err != nil {
		t.Fatalf("ListDeliveries() error = %v", err)
	}
	if totalEvents != 1 || totalDeliveries != 1 {
		t.Fatalf("webhook totals events=%d deliveries=%d, want one payment.succeeded delivery", totalEvents, totalDeliveries)
	}
	if deliveries[0].EventType != webhooksvc.EventPaymentSucceeded || deliveries[0].TargetURL != "https://merchant.example.com/payment/webhook" {
		t.Fatalf("delivery = %#v, want payment.succeeded to merchant notify url", deliveries[0])
	}
}
