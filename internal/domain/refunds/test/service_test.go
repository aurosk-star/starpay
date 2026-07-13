package refundstest

import (
	"context"
	"errors"
	"testing"

	"entgo.io/ent/dialect"
	_ "github.com/mattn/go-sqlite3"

	"payment-gateway/ent/enttest"
	appsvc "payment-gateway/internal/domain/apps/service"
	ordersvc "payment-gateway/internal/domain/orders/service"
	paymentsvc "payment-gateway/internal/domain/payments/service"
	refundsvc "payment-gateway/internal/domain/refunds/service"
	webhookrepo "payment-gateway/internal/domain/webhooks/repository"
	webhooksvc "payment-gateway/internal/domain/webhooks/service"
)

type fakeRefundGateway struct{ result *paymentsvc.RefundResult }

func (g *fakeRefundGateway) CreateRefund(ctx context.Context, input paymentsvc.CreateRefundInput) (*paymentsvc.RefundResult, error) {
	return g.result, nil
}

func (g *fakeRefundGateway) QueryRefund(ctx context.Context, input paymentsvc.QueryRefundInput) (*paymentsvc.RefundResult, error) {
	return g.result, nil
}

func TestCreateRefundIsIdempotentAndPreventsOverRefund(t *testing.T) {
	client := enttest.Open(t, dialect.SQLite, "file:refund_service?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	if _, err := appsvc.New(client).CreateApp(t.Context(), appsvc.ManageAppInput{AppID: "snsgo", Name: "SNSGO", Status: "enabled"}); err != nil {
		t.Fatalf("CreateApp() error = %v", err)
	}
	orderService := ordersvc.New(client)
	order, err := orderService.CreateOrder(t.Context(), ordersvc.ManageOrderInput{AppID: "snsgo", MerchantOrderNo: "biz_1", Subject: "Pro", Amount: 9900, Currency: "CNY", Channel: "alipay", PayMethod: "alipay"})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	order, err = client.PaymentOrder.UpdateOneID(order.ID).SetStatus("paid").SetChannelAccountID(1).SetProviderOrderNo(order.GatewayOrderNo).SetChannelTradeNo("ali_trade_1").Save(t.Context())
	if err != nil {
		t.Fatalf("seed paid order error = %v", err)
	}
	gateway := &fakeRefundGateway{result: &paymentsvc.RefundResult{Channel: "alipay", ChannelAccountID: 1, RefundNo: "rf_provider", ChannelRefundNo: "rf_provider", Status: "succeeded", Amount: 6000, Currency: "CNY"}}
	service := refundsvc.New(client, refundsvc.WithPaymentGateway(gateway), refundsvc.WithWebhookService(webhooksvc.New(client)))
	input := refundsvc.CreateInput{AppID: "snsgo", GatewayOrderNo: order.GatewayOrderNo, MerchantRefundNo: "merchant_rf_1", Amount: 6000, Currency: "CNY", Reason: "duplicate", Metadata: map[string]any{"source": "test"}}
	first, created, err := service.Create(t.Context(), input)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if !created || first.Status != "succeeded" || first.Amount != 6000 {
		t.Fatalf("first=%#v created=%v, want succeeded refund", first, created)
	}
	events, total, err := webhookrepo.New(client).ListEvents(t.Context(), webhookrepo.ListEventsInput{EventType: webhooksvc.EventRefundSucceeded, ResourceType: webhooksvc.ResourceRefund})
	if err != nil || total != 1 || events[0].ResourceID != first.RefundNo {
		t.Fatalf("refund events=%#v total=%d err=%v, want one event for %s", events, total, err, first.RefundNo)
	}
	second, created, err := service.Create(t.Context(), input)
	if err != nil || created || second.ID != first.ID {
		t.Fatalf("idempotent refund=%#v created=%v err=%v, want existing %d", second, created, err, first.ID)
	}
	_, _, err = service.Create(t.Context(), refundsvc.CreateInput{AppID: "snsgo", GatewayOrderNo: order.GatewayOrderNo, MerchantRefundNo: "merchant_rf_2", Amount: 4000, Currency: "CNY"})
	if !errors.Is(err, refundsvc.ErrRefundAmountExceedsPaid) {
		t.Fatalf("over-refund error = %v, want ErrRefundAmountExceedsPaid", err)
	}
}
