package orderstest

import (
	"context"
	"errors"
	"testing"

	"entgo.io/ent/dialect"
	_ "github.com/mattn/go-sqlite3"

	"payment-gateway/ent/enttest"
	appsvc "payment-gateway/internal/domain/apps/service"
	ordersvc "payment-gateway/internal/domain/orders/service"
)

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
