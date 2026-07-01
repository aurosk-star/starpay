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
)

func TestNotifyHandlerMarksOrderPaidAndReturnsAlipaySuccess(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	ctx := t.Context()
	client := enttest.Open(t, dialect.SQLite, "file:notify_handler_paid?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createEnabledApp(t, client, "snsgo")

	orderService := ordersvc.New(client)
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
		paymentsvc.WithMockFallback(false),
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
}
