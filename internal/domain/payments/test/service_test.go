package paymentstest

import (
	"errors"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	_ "github.com/mattn/go-sqlite3"

	"payment-gateway/ent/enttest"
	ordersvc "payment-gateway/internal/domain/orders/service"
	paymentsvc "payment-gateway/internal/domain/payments/service"
)

func TestPaymentServiceStartsPaymentWithNeutralResult(t *testing.T) {
	client := enttest.Open(t, dialect.SQLite, "file:payment_start?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createEnabledApp(t, client, "snsgo")

	orderService := ordersvc.New(client)
	order, err := orderService.CreateOrder(t.Context(), ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_pay_001",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}

	paymentService := paymentsvc.New()
	result, err := paymentService.StartPayment(t.Context(), paymentsvc.StartPaymentInput{
		Order:     order,
		PayMethod: "alipay",
		Channel:   "alipay",
		ClientIP:  "127.0.0.1",
		ReturnURL: "https://example.com/return",
	})
	if err != nil {
		t.Fatalf("StartPayment() error = %v", err)
	}
	if result.Status != "pending" || result.PayMethod != "alipay" || result.Channel != "alipay" {
		t.Fatalf("result = %#v, want pending alipay/alipay", result)
	}
	if result.ProviderOrderNo != "mock_"+order.GatewayOrderNo {
		t.Fatalf("ProviderOrderNo = %q, want mock gateway order", result.ProviderOrderNo)
	}
	if !strings.Contains(result.PayURL, order.GatewayOrderNo) {
		t.Fatalf("PayURL = %q, want gateway order number", result.PayURL)
	}
}

func TestPaymentServiceRejectsClosedOrder(t *testing.T) {
	client := enttest.Open(t, dialect.SQLite, "file:payment_closed?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createEnabledApp(t, client, "snsgo")

	orderService := ordersvc.New(client)
	order, err := orderService.CreateOrder(t.Context(), ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_pay_002",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	closed, err := orderService.CloseOrder(t.Context(), order.ID)
	if err != nil {
		t.Fatalf("CloseOrder() error = %v", err)
	}

	paymentService := paymentsvc.New()
	_, err = paymentService.StartPayment(t.Context(), paymentsvc.StartPaymentInput{
		Order:     closed,
		PayMethod: "alipay",
		Channel:   "alipay",
	})
	if !errors.Is(err, paymentsvc.ErrOrderNotPayable) {
		t.Fatalf("StartPayment() error = %v, want ErrOrderNotPayable", err)
	}
}
