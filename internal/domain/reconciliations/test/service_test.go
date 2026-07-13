package reconciliationstest

import (
	"context"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	_ "github.com/mattn/go-sqlite3"

	"payment-gateway/ent"
	"payment-gateway/ent/enttest"
	appsvc "payment-gateway/internal/domain/apps/service"
	channelrepo "payment-gateway/internal/domain/channels/repository"
	ordersvc "payment-gateway/internal/domain/orders/service"
	paymentsvc "payment-gateway/internal/domain/payments/service"
	reconciliationsvc "payment-gateway/internal/domain/reconciliations/service"
	webhookrepo "payment-gateway/internal/domain/webhooks/repository"
	webhooksvc "payment-gateway/internal/domain/webhooks/service"
)

type fakePaymentGateway struct {
	result     *paymentsvc.NotifyResult
	err        error
	closeCalls int
}

func (g *fakePaymentGateway) QueryPayment(ctx context.Context, input paymentsvc.QueryPaymentInput) (*paymentsvc.NotifyResult, error) {
	return g.result, g.err
}

func (g *fakePaymentGateway) ClosePayment(ctx context.Context, input paymentsvc.ClosePaymentInput) error {
	g.closeCalls++
	return g.err
}

func TestProcessMarksVerifiedPaidOrderAndResolvesReconciliation(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:reconciliation_paid?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	if _, err := appsvc.New(client).CreateApp(ctx, appsvc.ManageAppInput{AppID: "snsgo", Name: "SNSGO", Status: "enabled", NotifyURL: "https://merchant.example.com/webhooks"}); err != nil {
		t.Fatalf("CreateApp() error = %v", err)
	}
	account, err := channelrepo.New(client).Create(ctx, channelrepo.CreateChannelAccountInput{Channel: "alipay", Name: "Alipay", Enabled: true, Env: "sandbox", Config: map[string]any{"app_id": "app"}})
	if err != nil {
		t.Fatalf("Create channel account error = %v", err)
	}
	orderService := ordersvc.New(client, ordersvc.WithWebhookService(webhooksvc.New(client)))
	order, err := orderService.CreateOrder(ctx, ordersvc.ManageOrderInput{AppID: "snsgo", MerchantOrderNo: "biz_1", Subject: "Pro", Amount: 9900, Currency: "CNY", Channel: "alipay", PayMethod: "alipay"})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	order, err = client.PaymentOrder.UpdateOneID(order.ID).SetChannelAccountID(account.ID).SetProviderOrderNo(order.GatewayOrderNo).Save(ctx)
	if err != nil {
		t.Fatalf("bind order error = %v", err)
	}
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	gateway := &fakePaymentGateway{result: &paymentsvc.NotifyResult{Channel: "alipay", ChannelAccountID: account.ID, GatewayOrderNo: order.GatewayOrderNo, ChannelTradeNo: "ali_trade_1", Status: "paid", Amount: 9900, Currency: "CNY"}}
	service := reconciliationsvc.New(client, reconciliationsvc.WithPaymentGateway(gateway), reconciliationsvc.WithOrderService(orderService), reconciliationsvc.WithNow(func() time.Time { return now }))
	reconciliation, err := service.EnsureForOrder(ctx, order)
	if err != nil {
		t.Fatalf("EnsureForOrder() error = %v", err)
	}
	processed, err := service.Process(ctx, reconciliation.ID)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if processed.Status != "resolved" || processed.LastProviderStatus != "paid" || processed.ResolvedAt == nil {
		t.Fatalf("reconciliation = %#v, want resolved paid", processed)
	}
	persisted, err := client.PaymentOrder.Get(ctx, order.ID)
	if err != nil {
		t.Fatalf("Get order error = %v", err)
	}
	if persisted.Status != "paid" || persisted.ChannelTradeNo != "ali_trade_1" {
		t.Fatalf("order = %#v, want paid", persisted)
	}
	_, total, err := webhookrepo.New(client).ListEvents(ctx, webhookrepo.ListEventsInput{EventType: webhooksvc.EventPaymentSucceeded})
	if err != nil || total != 1 {
		t.Fatalf("payment succeeded events total=%d err=%v, want 1", total, err)
	}
}

func TestProcessClosesExpiredProviderPendingOrder(t *testing.T) {
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	client, orderService, order, account := seedPendingBoundOrder(t, "reconciliation_expired", now.Add(-time.Minute))
	defer client.Close()
	gateway := &fakePaymentGateway{result: &paymentsvc.NotifyResult{Channel: "alipay", ChannelAccountID: account.ID, GatewayOrderNo: order.GatewayOrderNo, Status: "pending", Amount: order.Amount, Currency: order.Currency}}
	service := reconciliationsvc.New(client, reconciliationsvc.WithPaymentGateway(gateway), reconciliationsvc.WithOrderService(orderService), reconciliationsvc.WithNow(func() time.Time { return now }))
	item, err := service.EnsureForOrder(t.Context(), order)
	if err != nil {
		t.Fatalf("EnsureForOrder() error = %v", err)
	}
	processed, err := service.Process(t.Context(), item.ID)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if processed.Status != "resolved" || processed.LastProviderStatus != "closed" || gateway.closeCalls != 1 {
		t.Fatalf("reconciliation=%#v closeCalls=%d, want resolved close", processed, gateway.closeCalls)
	}
	persisted, _ := client.PaymentOrder.Get(t.Context(), order.ID)
	if persisted.Status != "closed" {
		t.Fatalf("order status = %q, want closed", persisted.Status)
	}
}

func TestProcessMovesToManualRequiredAfterEighthFailure(t *testing.T) {
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	client, orderService, order, _ := seedPendingBoundOrder(t, "reconciliation_manual", now.Add(time.Hour))
	defer client.Close()
	gateway := &fakePaymentGateway{err: context.DeadlineExceeded}
	service := reconciliationsvc.New(client, reconciliationsvc.WithPaymentGateway(gateway), reconciliationsvc.WithOrderService(orderService), reconciliationsvc.WithNow(func() time.Time { return now }))
	item, err := service.EnsureForOrder(t.Context(), order)
	if err != nil {
		t.Fatalf("EnsureForOrder() error = %v", err)
	}
	if _, err := client.PaymentReconciliation.UpdateOneID(item.ID).SetAttemptCount(7).SetStatus("pending").Save(t.Context()); err != nil {
		t.Fatalf("seed attempt count error = %v", err)
	}
	processed, err := service.Process(t.Context(), item.ID)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if processed.Status != "manual_required" || processed.AttemptCount != 8 || processed.NextAttemptAt != nil || processed.LastError == "" {
		t.Fatalf("reconciliation = %#v, want manual_required after 8 attempts", processed)
	}
}

func seedPendingBoundOrder(t *testing.T, databaseName string, expiresAt time.Time) (*ent.Client, ordersvc.Service, *ent.PaymentOrder, *ent.ChannelAccount) {
	t.Helper()
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:"+databaseName+"?mode=memory&cache=shared&_fk=1")
	if _, err := appsvc.New(client).CreateApp(ctx, appsvc.ManageAppInput{AppID: databaseName, Name: databaseName, Status: "enabled", NotifyURL: "https://merchant.example.com/webhooks"}); err != nil {
		t.Fatalf("CreateApp() error = %v", err)
	}
	account, err := channelrepo.New(client).Create(ctx, channelrepo.CreateChannelAccountInput{Channel: "alipay", Name: "Alipay", Enabled: true, Env: "sandbox", Config: map[string]any{"app_id": "app"}})
	if err != nil {
		t.Fatalf("Create channel account error = %v", err)
	}
	orderService := ordersvc.New(client, ordersvc.WithWebhookService(webhooksvc.New(client)))
	order, err := orderService.CreateOrder(ctx, ordersvc.ManageOrderInput{AppID: databaseName, MerchantOrderNo: "biz_" + databaseName, Subject: "Pro", Amount: 9900, Currency: "CNY", Channel: "alipay", PayMethod: "alipay", ExpiresAt: &expiresAt})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	order, err = client.PaymentOrder.UpdateOneID(order.ID).SetChannelAccountID(account.ID).SetProviderOrderNo(order.GatewayOrderNo).Save(ctx)
	if err != nil {
		t.Fatalf("bind order error = %v", err)
	}
	return client, orderService, order, account
}
