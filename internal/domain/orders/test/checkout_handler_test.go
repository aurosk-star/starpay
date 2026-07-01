package orderstest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"

	"payment-gateway/ent/enttest"
	orderhandler "payment-gateway/internal/domain/orders/handler"
	ordersvc "payment-gateway/internal/domain/orders/service"
)

func TestCheckoutHandlerReturnsPublicOrderView(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	client := enttest.Open(t, dialect.SQLite, "file:checkout_order_view?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createEnabledApp(t, client, "snsgo")

	svc := ordersvc.New(client)
	createdResult, err := svc.CreateOrderWithCheckoutToken(t.Context(), ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_checkout_001",
		Subject:         "Pro 会员",
		Description:     "年度会员",
		Amount:          9900,
		Currency:        "CNY",
		PayMethod:       "alipay",
		Channel:         "alipay",
		BusinessType:    "membership",
		Metadata:        map[string]any{"internal_user_id": "u_001"},
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	created := createdResult.Order

	router := gin.New()
	checkoutHandler := orderhandler.NewCheckout(svc)
	router.GET("/orders/:gateway_order_no", checkoutHandler.GetOrder)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/orders/"+created.GatewayOrderNo, nil)
	req.Header.Set("X-Checkout-Token", createdResult.CheckoutToken)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["code"] != "ok" || response["error"] != nil {
		t.Fatalf("response = %#v, want global ok shape", response)
	}
	data := response["data"].(map[string]any)
	order := data["order"].(map[string]any)
	if order["gateway_order_no"] != created.GatewayOrderNo {
		t.Fatalf("gateway_order_no = %#v, want %q", order["gateway_order_no"], created.GatewayOrderNo)
	}
	if order["subject"] != "Pro 会员" || order["amount"].(float64) != 9900 || order["currency"] != "CNY" {
		t.Fatalf("order = %#v, want checkout display fields", order)
	}
	if _, ok := order["app_id"]; ok {
		t.Fatalf("order contains app_id: %#v", order)
	}
	if _, ok := order["metadata"]; ok {
		t.Fatalf("order contains metadata: %#v", order)
	}
	if data["title"] != "Pro 会员" {
		t.Fatalf("title = %#v, want subject", data["title"])
	}
}

func TestCheckoutHandlerRejectsMissingCheckoutToken(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	client := enttest.Open(t, dialect.SQLite, "file:checkout_order_token_required?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createEnabledApp(t, client, "snsgo")

	svc := ordersvc.New(client)
	created, err := svc.CreateOrder(t.Context(), ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_checkout_token_required",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}

	router := gin.New()
	checkoutHandler := orderhandler.NewCheckout(svc)
	router.GET("/orders/:gateway_order_no", checkoutHandler.GetOrder)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/orders/"+created.GatewayOrderNo, nil))

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}
