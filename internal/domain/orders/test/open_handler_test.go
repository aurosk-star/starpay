package orderstest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"

	"payment-gateway/ent/enttest"
	orderhandler "payment-gateway/internal/domain/orders/handler"
	ordersvc "payment-gateway/internal/domain/orders/service"
	"payment-gateway/internal/platform/httpx"
)

func TestOpenOrderHandlerCreatesOrderFromContextApp(t *testing.T) {
	router := newOpenOrderTestRouter(t, "snsgo")
	body := map[string]any{
		"app_id":            "malicious",
		"merchant_order_no": "biz_open_001",
		"subject":           "Pro 会员",
		"amount":            9900,
		"currency":          "CNY",
		"pay_method":        "alipay",
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, jsonRequest(http.MethodPost, "/orders", body))

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data := response["data"].(map[string]any)
	order := data["order"].(map[string]any)
	if order["app_id"] != "snsgo" {
		t.Fatalf("order.app_id = %#v, want context app", order["app_id"])
	}
	if data["created"] != true {
		t.Fatalf("created = %#v, want true", data["created"])
	}
	payment := data["payment"].(map[string]any)
	payURL, _ := payment["pay_url"].(string)
	if payment["status"] != "pending" || payURL == "" {
		t.Fatalf("payment = %#v, want pending with pay_url", payment)
	}
	if !strings.HasPrefix(payURL, "http://localhost:8080/checkout/"+order["gateway_order_no"].(string)+"?token=") {
		t.Fatalf("payment.pay_url = %q, want checkout url with token", payURL)
	}
}

func TestOpenOrderHandlerScopesLookupToContextApp(t *testing.T) {
	client := enttest.Open(t, dialect.SQLite, "file:open_handler_lookup?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	createEnabledApp(t, client, "snsgo")
	svc := ordersvc.New(client)
	created, _, err := svc.CreateOpenOrder(t.Context(), "snsgo", ordersvc.OpenOrderInput{
		MerchantOrderNo: "biz_open_001",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
	})
	if err != nil {
		t.Fatalf("CreateOpenOrder() error = %v", err)
	}

	router := gin.New()
	openHandler := orderhandler.NewOpen(svc)
	router.Use(func(ctx *gin.Context) {
		ctx.Set(httpx.ContextAppID, "billing")
		ctx.Next()
	})
	router.GET("/orders/:gateway_order_no", openHandler.GetOrder)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/orders/"+created.GatewayOrderNo, nil))

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["code"] != httpx.CodeOrderNotFound {
		t.Fatalf("code = %#v, want %s", response["code"], httpx.CodeOrderNotFound)
	}
}

func TestOpenOrderHandlerClosesOrderWithGlobalResponse(t *testing.T) {
	client := enttest.Open(t, dialect.SQLite, "file:open_handler_close?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	createEnabledApp(t, client, "snsgo")
	svc := ordersvc.New(client)
	created, _, err := svc.CreateOpenOrder(t.Context(), "snsgo", ordersvc.OpenOrderInput{
		MerchantOrderNo: "biz_open_001",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
	})
	if err != nil {
		t.Fatalf("CreateOpenOrder() error = %v", err)
	}

	router := gin.New()
	openHandler := orderhandler.NewOpen(svc)
	router.Use(func(ctx *gin.Context) {
		ctx.Set(httpx.ContextAppID, "snsgo")
		ctx.Next()
	})
	router.POST("/orders/:gateway_order_no/close", openHandler.CloseOrder)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/orders/"+created.GatewayOrderNo+"/close", nil))

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
}

func TestOpenCreateOrderReturnsStableInvalidAmountCode(t *testing.T) {
	router := newOpenOrderTestRouter(t, "snsgo")
	body := map[string]any{
		"merchant_order_no": "biz_invalid_amount",
		"subject":           "Pro 会员",
		"amount":            int64(0),
		"currency":          "CNY",
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, jsonRequest(http.MethodPost, "/orders", body))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["code"] != httpx.CodeInvalidAmount {
		t.Fatalf("code = %#v, want %s", response["code"], httpx.CodeInvalidAmount)
	}
}

func TestOpenCreateOrderReturnsStableCurrencyNotSupportedCode(t *testing.T) {
	router := newOpenOrderTestRouter(t, "snsgo")
	body := map[string]any{
		"merchant_order_no": "biz_currency",
		"subject":           "Pro 会员",
		"amount":            int64(9900),
		"currency":          "USD",
		"channel":           "wechat",
		"pay_method":        "wechat",
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, jsonRequest(http.MethodPost, "/orders", body))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["code"] != httpx.CodeCurrencyNotSupported {
		t.Fatalf("code = %#v, want %s", response["code"], httpx.CodeCurrencyNotSupported)
	}
}

func newOpenOrderTestRouter(t *testing.T, appID string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.ReleaseMode)
	client := enttest.Open(t, dialect.SQLite, "file:open_handler_create?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { client.Close() })

	createEnabledApp(t, client, "snsgo")

	router := gin.New()
	openHandler := orderhandler.NewOpen(ordersvc.New(client))
	router.Use(func(ctx *gin.Context) {
		ctx.Set(httpx.ContextAppID, appID)
		ctx.Next()
	})
	router.POST("/orders", openHandler.CreateOrder)
	return router
}

func jsonRequest(method string, path string, body map[string]any) *http.Request {
	payload, _ := json.Marshal(body)
	request := httptest.NewRequest(method, path, bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	return request
}
