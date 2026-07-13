package refundstest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"

	"payment-gateway/ent/enttest"
	appsvc "payment-gateway/internal/domain/apps/service"
	ordersvc "payment-gateway/internal/domain/orders/service"
	paymentsvc "payment-gateway/internal/domain/payments/service"
	refundhandler "payment-gateway/internal/domain/refunds/handler"
	refundsvc "payment-gateway/internal/domain/refunds/service"
	"payment-gateway/internal/platform/httpx"
)

func TestOpenRefundHandlerCreatesAndGetsAppScopedRefund(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	client := enttest.Open(t, dialect.SQLite, "file:refund_handler?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	if _, err := appsvc.New(client).CreateApp(t.Context(), appsvc.ManageAppInput{AppID: "snsgo", Name: "SNSGO", Status: "enabled"}); err != nil {
		t.Fatal(err)
	}
	order, err := ordersvc.New(client).CreateOrder(t.Context(), ordersvc.ManageOrderInput{AppID: "snsgo", MerchantOrderNo: "biz_1", Subject: "Pro", Amount: 9900, Currency: "CNY", Channel: "alipay", PayMethod: "alipay"})
	if err != nil {
		t.Fatal(err)
	}
	order, err = client.PaymentOrder.UpdateOneID(order.ID).SetStatus("paid").SetChannelAccountID(1).SetProviderOrderNo(order.GatewayOrderNo).SetChannelTradeNo("trade_1").Save(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	gateway := &fakeRefundGateway{result: &paymentsvc.RefundResult{Channel: "alipay", ChannelAccountID: 1, ChannelRefundNo: "ali_rf_1", Status: "succeeded", Amount: 1000, Currency: "CNY"}}
	handler := refundhandler.NewOpen(refundsvc.New(client, refundsvc.WithPaymentGateway(gateway)))
	router := gin.New()
	router.Use(func(ctx *gin.Context) { ctx.Set(httpx.ContextAppID, "snsgo") })
	router.POST("/refunds", handler.Create)
	router.GET("/refunds/:refund_no", handler.Get)
	payload, _ := json.Marshal(map[string]any{"gateway_order_no": order.GatewayOrderNo, "merchant_refund_no": "mrf_1", "amount": 1000, "currency": "CNY"})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/refunds", bytes.NewReader(payload)))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	refundNo := body["data"].(map[string]any)["refund"].(map[string]any)["refund_no"].(string)
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/refunds/"+refundNo, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
