package orderstest

import (
	"context"
	"errors"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	_ "github.com/mattn/go-sqlite3"

	"payment-gateway/ent/enttest"
	appsvc "payment-gateway/internal/domain/apps/service"
	ordersvc "payment-gateway/internal/domain/orders/service"
	webhookrepo "payment-gateway/internal/domain/webhooks/repository"
	webhooksvc "payment-gateway/internal/domain/webhooks/service"
)

type recordingExpirationEnqueuer struct {
	ids []int
	err error
}

func (e *recordingExpirationEnqueuer) EnqueueOrderExpiration(ctx context.Context, orderID int) error {
	if e.err != nil {
		return e.err
	}
	e.ids = append(e.ids, orderID)
	return nil
}

func TestCreateOrderRejectsMissingApp(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:create_order_missing_app?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	svc := ordersvc.New(client)
	if _, err := svc.CreateOrder(ctx, ordersvc.ManageOrderInput{
		AppID:           "missing-app",
		MerchantOrderNo: "biz_missing_app",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
		PayMethod:       "alipay",
		Channel:         "alipay",
	}); err == nil {
		t.Fatal("CreateOrder() missing app error = nil, want error")
	}
}

func TestCreateOrderStoresMinorUnitAmount(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:create_order?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createEnabledApp(t, client, "snsgo")

	svc := ordersvc.New(client)
	order, err := svc.CreateOrder(ctx, ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_001",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
		PayMethod:       "alipay",
		Channel:         "alipay",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	if order.GatewayOrderNo == "" {
		t.Fatal("GatewayOrderNo is empty")
	}
	if order.Amount != 9900 || order.Currency != "CNY" || order.Status != "pending" {
		t.Fatalf("order = %#v, want pending CNY minor-unit amount", order)
	}
}

func TestCreateOrderRejectsInvalidAmountAndCurrency(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:reject_invalid_order?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createEnabledApp(t, client, "snsgo")

	svc := ordersvc.New(client)
	if _, err := svc.CreateOrder(ctx, ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_001",
		Subject:         "Pro 会员",
		Amount:          0,
		Currency:        "CNY",
	}); err == nil {
		t.Fatal("CreateOrder() amount error = nil, want error")
	}
	if _, err := svc.CreateOrder(ctx, ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_002",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "DOGE",
	}); err == nil {
		t.Fatal("CreateOrder() currency error = nil, want error")
	}
}

func TestCreateOrderRejectsDuplicateMerchantOrderForSameApp(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:duplicate_merchant_order?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createEnabledApp(t, client, "snsgo")

	svc := ordersvc.New(client)
	input := ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_001",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
	}
	if _, err := svc.CreateOrder(ctx, input); err != nil {
		t.Fatalf("CreateOrder() first error = %v", err)
	}
	if _, err := svc.CreateOrder(ctx, input); err == nil {
		t.Fatal("CreateOrder() duplicate error = nil, want error")
	}
}

func TestCreateOrderRejectsUnsupportedCurrencyForChannel(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:create_order_unsupported_channel_currency?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createEnabledApp(t, client, "snsgo")

	svc := ordersvc.New(client)
	_, err := svc.CreateOrder(ctx, ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_unsupported_currency",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "USD",
		PayMethod:       "alipay",
		Channel:         "alipay",
	})
	if !errors.Is(err, ordersvc.ErrUnsupportedCurrencyForChannel) {
		t.Fatalf("CreateOrder() error = %v, want ErrUnsupportedCurrencyForChannel", err)
	}
}

func TestCreateOrderUsesAppDefaultReturnURLWhenRequestOmitsIt(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:create_order_app_default_return_url?mode=memory&cache=shared&_fk=1")
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

	svc := ordersvc.New(client)
	order, err := svc.CreateOrder(ctx, ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_default_return",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
		PayMethod:       "alipay",
		Channel:         "alipay",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	if order.ReturnURL != "https://snsgo.example.com/default-result" {
		t.Fatalf("ReturnURL = %q, want app default return URL", order.ReturnURL)
	}
}

func TestCreateOrderKeepsRequestReturnURLOverAppDefault(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:create_order_request_return_url?mode=memory&cache=shared&_fk=1")
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

	svc := ordersvc.New(client)
	order, err := svc.CreateOrder(ctx, ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_request_return",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
		PayMethod:       "alipay",
		Channel:         "alipay",
		ReturnURL:       "https://merchant.example.com/order-return",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	if order.ReturnURL != "https://merchant.example.com/order-return" {
		t.Fatalf("ReturnURL = %q, want request return URL", order.ReturnURL)
	}
}

func TestListOrdersFiltersByStatusAndApp(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:list_orders?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createEnabledApp(t, client, "snsgo")
	createEnabledApp(t, client, "billing")

	svc := ordersvc.New(client)
	_, _ = svc.CreateOrder(ctx, ordersvc.ManageOrderInput{AppID: "snsgo", MerchantOrderNo: "biz_001", Subject: "A", Amount: 100, Currency: "CNY"})
	paid, err := svc.CreateOrder(ctx, ordersvc.ManageOrderInput{AppID: "billing", MerchantOrderNo: "biz_002", Subject: "B", Amount: 200, Currency: "USD"})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	if _, err := svc.MarkPaid(ctx, paid.ID, "trade_001"); err != nil {
		t.Fatalf("MarkPaid() error = %v", err)
	}

	result, err := svc.ListOrders(ctx, ordersvc.ListOrdersInput{AppID: "billing", Status: "paid", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("ListOrders() error = %v", err)
	}
	if result.Total != 1 || len(result.Items) != 1 || result.Items[0].AppID != "billing" {
		t.Fatalf("result = %#v, want one paid billing order", result)
	}
}

func TestCloseOrderOnlyAllowsMutableStatuses(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:close_order?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createEnabledApp(t, client, "snsgo")

	svc := ordersvc.New(client)
	pending, err := svc.CreateOrder(ctx, ordersvc.ManageOrderInput{AppID: "snsgo", MerchantOrderNo: "biz_001", Subject: "A", Amount: 100, Currency: "CNY"})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	closed, err := svc.CloseOrder(ctx, pending.ID)
	if err != nil {
		t.Fatalf("CloseOrder() error = %v", err)
	}
	if closed.Status != "closed" || closed.ClosedAt == nil {
		t.Fatalf("closed = %#v, want closed status and timestamp", closed)
	}

	paid, _ := svc.CreateOrder(ctx, ordersvc.ManageOrderInput{AppID: "snsgo", MerchantOrderNo: "biz_002", Subject: "B", Amount: 100, Currency: "CNY"})
	paid, _ = svc.MarkPaid(ctx, paid.ID, "trade_001")
	if _, err := svc.CloseOrder(ctx, paid.ID); err == nil {
		t.Fatal("CloseOrder() paid error = nil, want error")
	}
}

func TestUpdateOrderRejectsImmutableFinancialFields(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:update_order?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createEnabledApp(t, client, "snsgo")

	svc := ordersvc.New(client)
	created, err := svc.CreateOrder(ctx, ordersvc.ManageOrderInput{AppID: "snsgo", MerchantOrderNo: "biz_001", Subject: "A", Amount: 100, Currency: "CNY"})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	updated, err := svc.UpdateOrder(ctx, created.ID, ordersvc.UpdateOrderInput{
		Subject:      "A updated",
		Description:  "manual adjustment note",
		BusinessType: "membership",
		Metadata:     map[string]any{"user_id": "123"},
	})
	if err != nil {
		t.Fatalf("UpdateOrder() error = %v", err)
	}
	if updated.Subject != "A updated" || updated.Amount != 100 || updated.MerchantOrderNo != "biz_001" {
		t.Fatalf("updated = %#v, want metadata-only update", updated)
	}
}

func TestOpenOrderCreateReturnsExistingOrderForSameRequest(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:open_order_idempotent?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createEnabledApp(t, client, "snsgo")

	svc := ordersvc.New(client)
	input := ordersvc.OpenOrderInput{
		MerchantOrderNo:  "biz_open_001",
		Subject:          "Pro 会员",
		Amount:           9900,
		Currency:         "CNY",
		PayMethod:        "alipay",
		PreferredChannel: "alipay",
		BusinessType:     "membership",
		Metadata:         map[string]any{"user_id": "123"},
	}
	created, createdFlag, err := svc.CreateOpenOrder(ctx, "snsgo", input)
	if err != nil {
		t.Fatalf("CreateOpenOrder() first error = %v", err)
	}
	if !createdFlag {
		t.Fatal("createdFlag = false, want true for first request")
	}
	existing, createdFlag, err := svc.CreateOpenOrder(ctx, "snsgo", input)
	if err != nil {
		t.Fatalf("CreateOpenOrder() second error = %v", err)
	}
	if createdFlag {
		t.Fatal("createdFlag = true, want false for idempotent existing order")
	}
	if existing.ID != created.ID || existing.GatewayOrderNo != created.GatewayOrderNo {
		t.Fatalf("existing = %#v, want original order %#v", existing, created)
	}
}

func TestOpenOrderCreateUsesGatewayDefaultExpiration(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:open_order_default_expiration?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createEnabledApp(t, client, "snsgo")

	now := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	svc := ordersvc.New(client, ordersvc.WithNow(func() time.Time { return now }), ordersvc.WithDefaultOrderTTL(15*time.Minute))
	order, _, err := svc.CreateOpenOrder(ctx, "snsgo", ordersvc.OpenOrderInput{
		MerchantOrderNo: "biz_open_expire",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
	})
	if err != nil {
		t.Fatalf("CreateOpenOrder() error = %v", err)
	}
	if order.ExpiresAt == nil {
		t.Fatal("ExpiresAt = nil, want gateway default expiration")
	}
	if want := now.Add(15 * time.Minute); !order.ExpiresAt.Equal(want) {
		t.Fatalf("ExpiresAt = %v, want %v", order.ExpiresAt, want)
	}
}

func TestOpenOrderCreateUsesDynamicGatewayDefaultExpiration(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:open_order_dynamic_default_expiration?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createEnabledApp(t, client, "snsgo")
	now := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	svc := ordersvc.New(client,
		ordersvc.WithNow(func() time.Time { return now }),
		ordersvc.WithDefaultOrderTTL(15*time.Minute),
		ordersvc.WithDefaultOrderTTLResolver(func(ctx context.Context) (time.Duration, error) {
			return 20 * time.Minute, nil
		}),
	)

	order, _, err := svc.CreateOpenOrder(ctx, "snsgo", ordersvc.OpenOrderInput{
		MerchantOrderNo: "biz_open_dynamic_expire",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
	})
	if err != nil {
		t.Fatalf("CreateOpenOrder() error = %v", err)
	}
	if order.ExpiresAt == nil {
		t.Fatal("ExpiresAt = nil, want gateway default expiration")
	}
	if want := now.Add(20 * time.Minute); !order.ExpiresAt.Equal(want) {
		t.Fatalf("ExpiresAt = %v, want %v", order.ExpiresAt, want)
	}
}

func TestOpenOrderRejectsIdempotencyConflict(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:open_order_conflict?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createEnabledApp(t, client, "snsgo")

	svc := ordersvc.New(client)
	input := ordersvc.OpenOrderInput{
		MerchantOrderNo: "biz_open_001",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
		PayMethod:       "alipay",
	}
	if _, _, err := svc.CreateOpenOrder(ctx, "snsgo", input); err != nil {
		t.Fatalf("CreateOpenOrder() first error = %v", err)
	}
	input.Amount = 19900
	if _, _, err := svc.CreateOpenOrder(ctx, "snsgo", input); !errors.Is(err, ordersvc.ErrIdempotencyConflict) {
		t.Fatalf("CreateOpenOrder() conflict error = %v, want ErrIdempotencyConflict", err)
	}
}

func TestOpenOrderLookupIsScopedToApp(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:open_order_lookup_scope?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createEnabledApp(t, client, "snsgo")

	svc := ordersvc.New(client)
	created, _, err := svc.CreateOpenOrder(ctx, "snsgo", ordersvc.OpenOrderInput{
		MerchantOrderNo: "biz_open_001",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
	})
	if err != nil {
		t.Fatalf("CreateOpenOrder() error = %v", err)
	}
	found, err := svc.FindOrderByGatewayOrderNoForApp(ctx, "snsgo", created.GatewayOrderNo)
	if err != nil {
		t.Fatalf("FindOrderByGatewayOrderNoForApp() owner error = %v", err)
	}
	if found.ID != created.ID {
		t.Fatalf("found.ID = %d, want %d", found.ID, created.ID)
	}
	if _, err := svc.FindOrderByGatewayOrderNoForApp(ctx, "billing", created.GatewayOrderNo); err == nil {
		t.Fatal("FindOrderByGatewayOrderNoForApp() other app error = nil, want error")
	}
	if _, err := svc.FindOrderByMerchantOrderNoForApp(ctx, "billing", created.MerchantOrderNo); err == nil {
		t.Fatal("FindOrderByMerchantOrderNoForApp() other app error = nil, want error")
	}
}

func TestOpenOrderCloseIsScopedToApp(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:open_order_close_scope?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createEnabledApp(t, client, "snsgo")

	svc := ordersvc.New(client)
	created, _, err := svc.CreateOpenOrder(ctx, "snsgo", ordersvc.OpenOrderInput{
		MerchantOrderNo: "biz_open_001",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
	})
	if err != nil {
		t.Fatalf("CreateOpenOrder() error = %v", err)
	}
	if _, err := svc.CloseOrderForApp(ctx, "billing", created.GatewayOrderNo); err == nil {
		t.Fatal("CloseOrderForApp() other app error = nil, want error")
	}
	closed, err := svc.CloseOrderForApp(ctx, "snsgo", created.GatewayOrderNo)
	if err != nil {
		t.Fatalf("CloseOrderForApp() owner error = %v", err)
	}
	if closed.Status != "closed" {
		t.Fatalf("closed.Status = %q, want closed", closed.Status)
	}

	paid, _, err := svc.CreateOpenOrder(ctx, "snsgo", ordersvc.OpenOrderInput{
		MerchantOrderNo: "biz_open_002",
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
	})
	if err != nil {
		t.Fatalf("CreateOpenOrder() paid setup error = %v", err)
	}
	if _, err := svc.MarkPaid(ctx, paid.ID, "trade_001"); err != nil {
		t.Fatalf("MarkPaid() error = %v", err)
	}
	if _, err := svc.CloseOrderForApp(ctx, "snsgo", paid.GatewayOrderNo); !errors.Is(err, ordersvc.ErrOrderCannotBeClosed) {
		t.Fatalf("CloseOrderForApp() paid error = %v, want ErrOrderCannotBeClosed", err)
	}
}

func TestScanExpiredPendingOrdersEnqueuesOnlyExpiredPendingOrders(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:scan_expired_pending_orders?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createEnabledApp(t, client, "snsgo")

	now := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	currentNow := now
	enqueuer := &recordingExpirationEnqueuer{}
	svc := ordersvc.New(client,
		ordersvc.WithNow(func() time.Time { return currentNow }),
		ordersvc.WithDefaultOrderTTL(15*time.Minute),
		ordersvc.WithExpirationEnqueuer(enqueuer),
	)
	expired, _, err := svc.CreateOpenOrder(ctx, "snsgo", ordersvc.OpenOrderInput{
		MerchantOrderNo: "biz_expired",
		Subject:         "Expired",
		Amount:          100,
		Currency:        "CNY",
	})
	if err != nil {
		t.Fatalf("CreateOpenOrder() expired error = %v", err)
	}
	currentNow = now.Add(10 * time.Minute)
	fresh, _, err := svc.CreateOpenOrder(ctx, "snsgo", ordersvc.OpenOrderInput{
		MerchantOrderNo: "biz_fresh",
		Subject:         "Fresh",
		Amount:          100,
		Currency:        "CNY",
	})
	if err != nil {
		t.Fatalf("CreateOpenOrder() fresh error = %v", err)
	}
	paid, _, err := svc.CreateOpenOrder(ctx, "snsgo", ordersvc.OpenOrderInput{
		MerchantOrderNo: "biz_paid",
		Subject:         "Paid",
		Amount:          100,
		Currency:        "CNY",
	})
	if err != nil {
		t.Fatalf("CreateOpenOrder() paid error = %v", err)
	}
	if _, err := svc.MarkPaid(ctx, paid.ID, "trade_001"); err != nil {
		t.Fatalf("MarkPaid() error = %v", err)
	}

	later := now.Add(16 * time.Minute)
	currentNow = later
	enqueuedCount, err := svc.ScanExpiredPendingOrders(ctx, 100)
	if err != nil {
		t.Fatalf("ScanExpiredPendingOrders() error = %v", err)
	}
	if enqueuedCount != 1 {
		t.Fatalf("enqueuedCount = %d, want 1", enqueuedCount)
	}
	if len(enqueuer.ids) != 1 || enqueuer.ids[0] != expired.ID {
		t.Fatalf("enqueued ids = %#v, want [%d]", enqueuer.ids, expired.ID)
	}
	expired, _ = svc.FindOrder(ctx, expired.ID)
	fresh, _ = svc.FindOrder(ctx, fresh.ID)
	paid, _ = svc.FindOrder(ctx, paid.ID)
	if expired.Status != "pending" {
		t.Fatalf("expired.Status = %q, want pending before worker consumes task", expired.Status)
	}
	if fresh.Status != "pending" {
		t.Fatalf("fresh.Status = %q, want pending", fresh.Status)
	}
	if paid.Status != "paid" {
		t.Fatalf("paid.Status = %q, want paid", paid.Status)
	}
}

func TestCloseExpiredPendingOrderIsAtomicAndIdempotent(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:close_expired_pending_order_atomic?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createEnabledApp(t, client, "snsgo")

	now := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	currentNow := now
	svc := ordersvc.New(client, ordersvc.WithNow(func() time.Time { return currentNow }), ordersvc.WithDefaultOrderTTL(15*time.Minute))
	expired, _, err := svc.CreateOpenOrder(ctx, "snsgo", ordersvc.OpenOrderInput{
		MerchantOrderNo: "biz_expired_atomic",
		Subject:         "Expired",
		Amount:          100,
		Currency:        "CNY",
	})
	if err != nil {
		t.Fatalf("CreateOpenOrder() error = %v", err)
	}

	currentNow = now.Add(16 * time.Minute)
	closed, err := svc.CloseExpiredPendingOrder(ctx, expired.ID)
	if err != nil {
		t.Fatalf("CloseExpiredPendingOrder() first error = %v", err)
	}
	if !closed {
		t.Fatal("CloseExpiredPendingOrder() first closed = false, want true")
	}
	closed, err = svc.CloseExpiredPendingOrder(ctx, expired.ID)
	if err != nil {
		t.Fatalf("CloseExpiredPendingOrder() duplicate error = %v", err)
	}
	if closed {
		t.Fatal("CloseExpiredPendingOrder() duplicate closed = true, want false")
	}
	expired, _ = svc.FindOrder(ctx, expired.ID)
	if expired.Status != "closed" || expired.ClosedAt == nil {
		t.Fatalf("expired order = %#v, want closed with closed_at", expired)
	}
}

func TestCloseExpiredPendingOrderRecordsWebhookOnlyWhenClosed(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:close_expired_pending_order_webhook?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	if _, err := appsvc.New(client).CreateApp(ctx, appsvc.ManageAppInput{
		AppID:     "snsgo",
		Name:      "Snsgo",
		NotifyURL: "https://merchant.example.com/webhooks/payment",
		Status:    "enabled",
	}); err != nil {
		t.Fatalf("CreateApp() error = %v", err)
	}

	now := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	currentNow := now
	webhooks := webhooksvc.New(client)
	svc := ordersvc.New(client,
		ordersvc.WithNow(func() time.Time { return currentNow }),
		ordersvc.WithDefaultOrderTTL(15*time.Minute),
		ordersvc.WithWebhookService(webhooks),
	)
	order, _, err := svc.CreateOpenOrder(ctx, "snsgo", ordersvc.OpenOrderInput{
		MerchantOrderNo: "biz_expired_webhook",
		Subject:         "Expired",
		Amount:          100,
		Currency:        "CNY",
	})
	if err != nil {
		t.Fatalf("CreateOpenOrder() error = %v", err)
	}

	currentNow = now.Add(16 * time.Minute)
	closed, err := svc.CloseExpiredPendingOrder(ctx, order.ID)
	if err != nil {
		t.Fatalf("CloseExpiredPendingOrder() first error = %v", err)
	}
	if !closed {
		t.Fatal("CloseExpiredPendingOrder() first closed = false, want true")
	}
	closed, err = svc.CloseExpiredPendingOrder(ctx, order.ID)
	if err != nil {
		t.Fatalf("CloseExpiredPendingOrder() duplicate error = %v", err)
	}
	if closed {
		t.Fatal("CloseExpiredPendingOrder() duplicate closed = true, want false")
	}

	_, totalEvents, err := webhookrepo.New(client).ListEvents(ctx, webhookrepo.ListEventsInput{EventType: "order.expired"})
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	_, totalDeliveries, err := webhookrepo.New(client).ListDeliveries(ctx, webhookrepo.ListDeliveriesInput{EventType: "order.expired"})
	if err != nil {
		t.Fatalf("ListDeliveries() error = %v", err)
	}
	if totalEvents != 1 || totalDeliveries != 1 {
		t.Fatalf("totals events=%d deliveries=%d, want one order.expired event and delivery", totalEvents, totalDeliveries)
	}
}

func TestApplyPaymentResultRejectsAmountMismatch(t *testing.T) {
	ctx := t.Context()
	client := enttest.Open(t, dialect.SQLite, "file:apply_payment_result_amount?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createEnabledApp(t, client, "snsgo")
	svc := ordersvc.New(client)
	order, err := svc.CreateOrder(ctx, ordersvc.ManageOrderInput{
		AppID: "snsgo", MerchantOrderNo: "biz_amount_mismatch", Subject: "Pro", Amount: 9900, Currency: "USD", Channel: "paypal", PayMethod: "paypal",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	if _, err := svc.SetPaymentSelection(ctx, order.ID, "paypal", "paypal", 17); err != nil {
		t.Fatalf("SetPaymentSelection() error = %v", err)
	}

	_, err = svc.ApplyPaymentResult(ctx, order.ID, ordersvc.PaymentResultInput{
		Channel: "paypal", ChannelAccountID: 17, ChannelTradeNo: "CAPTURE_001", Status: "paid", Amount: 9800, Currency: "USD",
	})
	if !errors.Is(err, ordersvc.ErrPaymentAmountMismatch) {
		t.Fatalf("ApplyPaymentResult() error = %v, want ErrPaymentAmountMismatch", err)
	}
	stored, err := svc.FindOrder(ctx, order.ID)
	if err != nil {
		t.Fatalf("FindOrder() error = %v", err)
	}
	if stored.Status != "pending" {
		t.Fatalf("Status = %q, want pending", stored.Status)
	}
}

func TestApplyPaymentResultMarksFailedWithReason(t *testing.T) {
	ctx := t.Context()
	client := enttest.Open(t, dialect.SQLite, "file:apply_payment_result_failed?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createEnabledApp(t, client, "snsgo")
	svc := ordersvc.New(client)
	order, err := svc.CreateOrder(ctx, ordersvc.ManageOrderInput{
		AppID: "snsgo", MerchantOrderNo: "biz_payment_failed", Subject: "Pro", Amount: 9900, Currency: "CNY", Channel: "wechat", PayMethod: "wechat",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	if _, err := svc.SetPaymentSelection(ctx, order.ID, "wechat", "wechat", 23); err != nil {
		t.Fatalf("SetPaymentSelection() error = %v", err)
	}

	failed, err := svc.ApplyPaymentResult(ctx, order.ID, ordersvc.PaymentResultInput{
		Channel: "wechat", ChannelAccountID: 23, Status: "failed", FailureReason: "PAYERROR",
	})
	if err != nil {
		t.Fatalf("ApplyPaymentResult() error = %v", err)
	}
	if failed.Status != "failed" || failed.FailedAt == nil || failed.FailureReason != "PAYERROR" {
		t.Fatalf("failed order = %#v", failed)
	}
}
