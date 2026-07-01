package orderstest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"

	"payment-gateway/ent/enttest"
	appsvc "payment-gateway/internal/domain/apps/service"
	orderhandler "payment-gateway/internal/domain/orders/handler"
	ordersvc "payment-gateway/internal/domain/orders/service"
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

	router := gin.New()
	checkoutHandler := orderhandler.NewCheckout(svc)
	router.POST("/orders/:gateway_order_no/pay", checkoutHandler.StartPayment)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, jsonRequest(http.MethodPost, "/orders/"+order.GatewayOrderNo+"/pay", map[string]any{
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
	payment := response["data"].(map[string]any)["payment"].(map[string]any)
	payURL := payment["pay_url"].(string)
	if !strings.Contains(payURL, "https%3A%2F%2Fsnsgo.example.com%2Forder-return") {
		t.Fatalf("pay_url = %q, want persisted return_url query", payURL)
	}
}
