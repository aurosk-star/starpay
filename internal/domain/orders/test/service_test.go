package orderstest

import (
	"context"
	"testing"

	"entgo.io/ent/dialect"
	_ "github.com/mattn/go-sqlite3"

	"payment-gateway/ent/enttest"
	ordersvc "payment-gateway/internal/domain/orders/service"
)

func TestCreateOrderStoresMinorUnitAmount(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:create_order?mode=memory&cache=shared&_fk=1")
	defer client.Close()

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

func TestListOrdersFiltersByStatusAndApp(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:list_orders?mode=memory&cache=shared&_fk=1")
	defer client.Close()

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
