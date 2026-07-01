package paymentstest

import (
	"errors"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	_ "github.com/mattn/go-sqlite3"

	"payment-gateway/ent/enttest"
	ordersvc "payment-gateway/internal/domain/orders/service"
	paymentsvc "payment-gateway/internal/domain/payments/service"
)

func TestPaymentServiceRejectsUnavailableProvider(t *testing.T) {
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
	_, err = paymentService.StartPayment(t.Context(), paymentsvc.StartPaymentInput{
		Order:     order,
		PayMethod: "alipay",
		Channel:   "alipay",
		ClientIP:  "127.0.0.1",
		ReturnURL: "https://example.com/return",
	})
	if !errors.Is(err, paymentsvc.ErrProviderUnavailable) {
		t.Fatalf("StartPayment() error = %v, want ErrProviderUnavailable", err)
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

func TestPaymentServiceRejectsExpiredOrder(t *testing.T) {
	client := enttest.Open(t, dialect.SQLite, "file:payment_expired?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createEnabledApp(t, client, "snsgo")

	expiresAt := time.Now().Add(-time.Minute)
	orderService := ordersvc.New(client)
	order, err := orderService.CreateOrder(t.Context(), ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_pay_expired",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
		ExpiresAt:       &expiresAt,
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}

	paymentService := paymentsvc.New()
	_, err = paymentService.StartPayment(t.Context(), paymentsvc.StartPaymentInput{
		Order:     order,
		PayMethod: "alipay",
		Channel:   "alipay",
	})
	if !errors.Is(err, paymentsvc.ErrOrderExpired) {
		t.Fatalf("StartPayment() error = %v, want ErrOrderExpired", err)
	}
}
