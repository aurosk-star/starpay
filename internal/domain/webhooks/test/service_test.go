package webhookstest

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	_ "github.com/mattn/go-sqlite3"

	"payment-gateway/ent"
	"payment-gateway/ent/enttest"
	appsvc "payment-gateway/internal/domain/apps/service"
	ordersvc "payment-gateway/internal/domain/orders/service"
	webhookrepo "payment-gateway/internal/domain/webhooks/repository"
	webhooksvc "payment-gateway/internal/domain/webhooks/service"
)

type recordingEnqueuer struct {
	ids []int
	err error
}

func (e *recordingEnqueuer) EnqueueWebhookDelivery(ctx context.Context, deliveryID int) error {
	if e.err != nil {
		return e.err
	}
	e.ids = append(e.ids, deliveryID)
	return nil
}

func TestRecordPaymentSucceededQueuesOneDeliveryForAppNotifyURL(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:webhook_payment_succeeded?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createApp(t, client, "snsgo", "https://merchant.example.com/webhooks/payment")
	order := createPaidOrder(t, client, "snsgo", "biz_001")

	service := webhooksvc.New(client)
	event, err := service.RecordPaymentSucceeded(ctx, order)
	if err != nil {
		t.Fatalf("RecordPaymentSucceeded() error = %v", err)
	}
	if event.EventType != "payment.succeeded" {
		t.Fatalf("EventType = %q, want payment.succeeded", event.EventType)
	}
	if event.AppID != "snsgo" || event.GatewayOrderNo != order.GatewayOrderNo {
		t.Fatalf("event = %#v, want app and order references", event)
	}

	events, totalEvents, err := webhookrepo.New(client).ListEvents(ctx, webhookrepo.ListEventsInput{})
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	if totalEvents != 1 || len(events) != 1 {
		t.Fatalf("events total=%d len=%d, want 1", totalEvents, len(events))
	}
	deliveries, totalDeliveries, err := webhookrepo.New(client).ListDeliveries(ctx, webhookrepo.ListDeliveriesInput{})
	if err != nil {
		t.Fatalf("ListDeliveries() error = %v", err)
	}
	if totalDeliveries != 1 || len(deliveries) != 1 {
		t.Fatalf("deliveries total=%d len=%d, want 1", totalDeliveries, len(deliveries))
	}
	delivery := deliveries[0]
	if delivery.EventID != event.ID || delivery.TargetURL != "https://merchant.example.com/webhooks/payment" || delivery.Status != "pending" {
		t.Fatalf("delivery = %#v, want pending delivery for app notify url", delivery)
	}
	if delivery.AttemptCount != 0 || delivery.NextAttemptAt == nil {
		t.Fatalf("delivery attempt state = count %d next %v, want queued initial state", delivery.AttemptCount, delivery.NextAttemptAt)
	}
}

func TestRecordPaymentSucceededEnqueuesDeliveryToStream(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:webhook_payment_succeeded_enqueue?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createApp(t, client, "snsgo", "https://merchant.example.com/webhooks/payment")
	order := createPaidOrder(t, client, "snsgo", "biz_enqueue")
	enqueuer := &recordingEnqueuer{}

	if _, err := webhooksvc.New(client, webhooksvc.WithEnqueuer(enqueuer)).RecordPaymentSucceeded(ctx, order); err != nil {
		t.Fatalf("RecordPaymentSucceeded() error = %v", err)
	}

	deliveries, _, err := webhookrepo.New(client).ListDeliveries(ctx, webhookrepo.ListDeliveriesInput{})
	if err != nil {
		t.Fatalf("ListDeliveries() error = %v", err)
	}
	if len(enqueuer.ids) != 1 || enqueuer.ids[0] != deliveries[0].ID {
		t.Fatalf("enqueued delivery ids = %#v, want [%d]", enqueuer.ids, deliveries[0].ID)
	}
}

func TestRecordOrderExpiredQueuesOneDeliveryForAppNotifyURL(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:webhook_order_expired?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createApp(t, client, "snsgo", "https://merchant.example.com/webhooks/payment")
	order := createExpiredOrder(t, client, "snsgo", "biz_expired")
	enqueuer := &recordingEnqueuer{}

	event, err := webhooksvc.New(client, webhooksvc.WithEnqueuer(enqueuer)).RecordOrderExpired(ctx, order)
	if err != nil {
		t.Fatalf("RecordOrderExpired() error = %v", err)
	}
	if event.EventType != "order.expired" {
		t.Fatalf("EventType = %q, want order.expired", event.EventType)
	}
	if event.AppID != "snsgo" || event.GatewayOrderNo != order.GatewayOrderNo {
		t.Fatalf("event = %#v, want app and order references", event)
	}

	deliveries, totalDeliveries, err := webhookrepo.New(client).ListDeliveries(ctx, webhookrepo.ListDeliveriesInput{GatewayOrderNo: order.GatewayOrderNo})
	if err != nil {
		t.Fatalf("ListDeliveries() error = %v", err)
	}
	if totalDeliveries != 1 || len(deliveries) != 1 {
		t.Fatalf("deliveries total=%d len=%d, want 1", totalDeliveries, len(deliveries))
	}
	if deliveries[0].EventType != "order.expired" || deliveries[0].Status != "pending" {
		t.Fatalf("delivery = %#v, want pending order.expired delivery", deliveries[0])
	}
	if len(enqueuer.ids) != 1 || enqueuer.ids[0] != deliveries[0].ID {
		t.Fatalf("enqueued delivery ids = %#v, want [%d]", enqueuer.ids, deliveries[0].ID)
	}
}

func TestRecordOrderExpiredIsIdempotentPerOrder(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:webhook_order_expired_idempotent?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createApp(t, client, "snsgo", "https://merchant.example.com/webhooks/payment")
	order := createExpiredOrder(t, client, "snsgo", "biz_expired_idempotent")

	service := webhooksvc.New(client)
	first, err := service.RecordOrderExpired(ctx, order)
	if err != nil {
		t.Fatalf("first RecordOrderExpired() error = %v", err)
	}
	second, err := service.RecordOrderExpired(ctx, order)
	if err != nil {
		t.Fatalf("second RecordOrderExpired() error = %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("second event ID = %d, want existing event ID %d", second.ID, first.ID)
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

func TestRecordOrderClosedQueuesIdempotentDeliveryWithCloseSource(t *testing.T) {
	ctx := t.Context()
	client := enttest.Open(t, dialect.SQLite, "file:webhook_order_closed?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createApp(t, client, "snsgo_closed", "https://merchant.example.com/webhooks/payment")
	orderService := ordersvc.New(client)
	order, err := orderService.CreateOrder(ctx, ordersvc.ManageOrderInput{
		AppID: "snsgo_closed", MerchantOrderNo: "biz_closed", Subject: "Closed",
		Amount: 9900, Currency: "CNY", Channel: "alipay", PayMethod: "alipay",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	closedAt := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	order, err = client.PaymentOrder.UpdateOneID(order.ID).SetStatus("closed").SetClosedAt(closedAt).Save(ctx)
	if err != nil {
		t.Fatalf("seed closed order error = %v", err)
	}

	service := webhooksvc.New(client)
	first, err := service.RecordOrderClosed(ctx, order, "admin")
	if err != nil {
		t.Fatalf("RecordOrderClosed() first error = %v", err)
	}
	second, err := service.RecordOrderClosed(ctx, order, "admin")
	if err != nil {
		t.Fatalf("RecordOrderClosed() second error = %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("event IDs first=%d second=%d, want idempotent event", first.ID, second.ID)
	}
	if first.EventType != webhooksvc.EventOrderClosed || first.Payload["close_source"] != "admin" || first.Payload["status"] != "closed" {
		t.Fatalf("event = %#v, want admin order.closed payload", first)
	}
	if first.Payload["closed_at"] != closedAt.Format(time.RFC3339) {
		t.Fatalf("closed_at = %#v, want %q", first.Payload["closed_at"], closedAt.Format(time.RFC3339))
	}

	_, eventTotal, err := webhookrepo.New(client).ListEvents(ctx, webhookrepo.ListEventsInput{EventType: webhooksvc.EventOrderClosed})
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	deliveries, deliveryTotal, err := webhookrepo.New(client).ListDeliveries(ctx, webhookrepo.ListDeliveriesInput{EventType: webhooksvc.EventOrderClosed})
	if err != nil {
		t.Fatalf("ListDeliveries() error = %v", err)
	}
	if eventTotal != 1 || deliveryTotal != 1 || len(deliveries) != 1 || deliveries[0].Status != "pending" {
		t.Fatalf("eventTotal=%d deliveryTotal=%d deliveries=%#v, want one pending delivery", eventTotal, deliveryTotal, deliveries)
	}
}

func TestRecordPaymentFailedQueuesOneDeliveryForAppNotifyURL(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:webhook_payment_failed?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createApp(t, client, "snsgo", "https://merchant.example.com/webhooks/payment")
	orderService := ordersvc.New(client)
	order, err := orderService.CreateOrder(ctx, ordersvc.ManageOrderInput{
		AppID: "snsgo", MerchantOrderNo: "pay_failed_001", Subject: "Pro", Amount: 9900, Currency: "CNY", Channel: "wechat", PayMethod: "wechat",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	failedAt := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	if _, err := client.PaymentOrder.UpdateOneID(order.ID).SetStatus("failed").SetFailedAt(failedAt).SetFailureReason("PAYERROR").Save(ctx); err != nil {
		t.Fatalf("seed failed order error = %v", err)
	}
	order, _ = client.PaymentOrder.Get(ctx, order.ID)

	event, err := webhooksvc.New(client).RecordPaymentFailed(ctx, order)
	if err != nil {
		t.Fatalf("RecordPaymentFailed() error = %v", err)
	}
	if event.EventType != webhooksvc.EventPaymentFailed || event.Payload["failure_reason"] != "PAYERROR" {
		t.Fatalf("event = %#v, want payment.failed payload", event)
	}
	deliveries, total, err := webhookrepo.New(client).ListDeliveries(ctx, webhookrepo.ListDeliveriesInput{EventType: webhooksvc.EventPaymentFailed})
	if err != nil {
		t.Fatalf("ListDeliveries() error = %v", err)
	}
	if total != 1 || deliveries[0].Status != "pending" {
		t.Fatalf("deliveries = %#v total=%d, want one pending", deliveries, total)
	}
}

func TestRecordPaymentSucceededReturnsErrorWhenEnqueueFails(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:webhook_payment_succeeded_enqueue_fail?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createApp(t, client, "snsgo", "https://merchant.example.com/webhooks/payment")
	order := createPaidOrder(t, client, "snsgo", "biz_enqueue_fail")
	enqueuer := &recordingEnqueuer{err: errors.New("redis unavailable")}

	if _, err := webhooksvc.New(client, webhooksvc.WithEnqueuer(enqueuer)).RecordPaymentSucceeded(ctx, order); err == nil {
		t.Fatal("RecordPaymentSucceeded() error = nil, want enqueue error")
	}
}

func TestRecordPaymentSucceededIsIdempotentPerOrder(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:webhook_payment_succeeded_idempotent?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createApp(t, client, "snsgo", "https://merchant.example.com/webhooks/payment")
	order := createPaidOrder(t, client, "snsgo", "biz_002")

	service := webhooksvc.New(client)
	first, err := service.RecordPaymentSucceeded(ctx, order)
	if err != nil {
		t.Fatalf("first RecordPaymentSucceeded() error = %v", err)
	}
	second, err := service.RecordPaymentSucceeded(ctx, order)
	if err != nil {
		t.Fatalf("second RecordPaymentSucceeded() error = %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("second event ID = %d, want existing event ID %d", second.ID, first.ID)
	}

	_, totalEvents, err := webhookrepo.New(client).ListEvents(ctx, webhookrepo.ListEventsInput{})
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	_, totalDeliveries, err := webhookrepo.New(client).ListDeliveries(ctx, webhookrepo.ListDeliveriesInput{})
	if err != nil {
		t.Fatalf("ListDeliveries() error = %v", err)
	}
	if totalEvents != 1 || totalDeliveries != 1 {
		t.Fatalf("totals events=%d deliveries=%d, want one of each", totalEvents, totalDeliveries)
	}
}

func TestRecordPaymentSucceededUsesPaymentOrderResourceIdentity(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:webhook_payment_resource_identity?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createApp(t, client, "snsgo_resource", "")
	order := createPaidOrder(t, client, "snsgo_resource", "biz_resource")

	event, err := webhooksvc.New(client).RecordPaymentSucceeded(ctx, order)
	if err != nil {
		t.Fatalf("RecordPaymentSucceeded() error = %v", err)
	}
	if event.ResourceType != webhooksvc.ResourcePaymentOrder || event.ResourceID != order.GatewayOrderNo {
		t.Fatalf("resource = %q/%q, want %q/%q", event.ResourceType, event.ResourceID, webhooksvc.ResourcePaymentOrder, order.GatewayOrderNo)
	}
}

func TestRecordResourceEventsAreUniquePerResource(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:webhook_refund_resource_identity?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createApp(t, client, "snsgo_refund_resource", "")
	service := webhooksvc.New(client)
	first, err := service.RecordResourceEvent(ctx, webhooksvc.ResourceEventInput{
		EventType:      "refund.succeeded",
		AppID:          "snsgo_refund_resource",
		ResourceType:   webhooksvc.ResourceRefund,
		ResourceID:     "rf_001",
		GatewayOrderNo: "gw_001",
		RefundNo:       "rf_001",
		Payload:        map[string]any{"refund_no": "rf_001"},
	})
	if err != nil {
		t.Fatalf("first RecordResourceEvent() error = %v", err)
	}
	duplicate, err := service.RecordResourceEvent(ctx, webhooksvc.ResourceEventInput{
		EventType:      "refund.succeeded",
		AppID:          "snsgo_refund_resource",
		ResourceType:   webhooksvc.ResourceRefund,
		ResourceID:     "rf_001",
		GatewayOrderNo: "gw_001",
		RefundNo:       "rf_001",
		Payload:        map[string]any{"refund_no": "rf_001"},
	})
	if err != nil {
		t.Fatalf("duplicate RecordResourceEvent() error = %v", err)
	}
	second, err := service.RecordResourceEvent(ctx, webhooksvc.ResourceEventInput{
		EventType:      "refund.succeeded",
		AppID:          "snsgo_refund_resource",
		ResourceType:   webhooksvc.ResourceRefund,
		ResourceID:     "rf_002",
		GatewayOrderNo: "gw_001",
		RefundNo:       "rf_002",
		Payload:        map[string]any{"refund_no": "rf_002"},
	})
	if err != nil {
		t.Fatalf("second RecordResourceEvent() error = %v", err)
	}
	if duplicate.ID != first.ID {
		t.Fatalf("duplicate event ID = %d, want %d", duplicate.ID, first.ID)
	}
	if second.ID == first.ID {
		t.Fatalf("second resource reused event ID %d", first.ID)
	}

	_, total, err := webhookrepo.New(client).ListEvents(ctx, webhookrepo.ListEventsInput{
		EventType:    "refund.succeeded",
		ResourceType: webhooksvc.ResourceRefund,
	})
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	if total != 2 {
		t.Fatalf("resource event total = %d, want 2", total)
	}
}

func TestRecordPaymentSucceededSkipsDeliveryWhenNotifyURLMissing(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:webhook_payment_succeeded_no_url?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createApp(t, client, "snsgo", "")
	order := createPaidOrder(t, client, "snsgo", "biz_003")

	if _, err := webhooksvc.New(client).RecordPaymentSucceeded(ctx, order); err != nil {
		t.Fatalf("RecordPaymentSucceeded() error = %v", err)
	}
	_, totalEvents, err := webhookrepo.New(client).ListEvents(ctx, webhookrepo.ListEventsInput{})
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	_, totalDeliveries, err := webhookrepo.New(client).ListDeliveries(ctx, webhookrepo.ListDeliveriesInput{})
	if err != nil {
		t.Fatalf("ListDeliveries() error = %v", err)
	}
	if totalEvents != 1 || totalDeliveries != 0 {
		t.Fatalf("totals events=%d deliveries=%d, want event without delivery", totalEvents, totalDeliveries)
	}
}

func TestRetryDeliveryResetsAndEnqueuesDelivery(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:webhook_retry_enqueue?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createApp(t, client, "snsgo", "https://merchant.example.com/webhooks/payment")
	order := createPaidOrder(t, client, "snsgo", "biz_retry")
	service := webhooksvc.New(client)
	if _, err := service.RecordPaymentSucceeded(ctx, order); err != nil {
		t.Fatalf("RecordPaymentSucceeded() error = %v", err)
	}
	deliveries, _, err := webhookrepo.New(client).ListDeliveries(ctx, webhookrepo.ListDeliveriesInput{})
	if err != nil {
		t.Fatalf("ListDeliveries() error = %v", err)
	}
	failedAt := time.Now().Add(-time.Minute)
	if _, err := client.WebhookDelivery.UpdateOneID(deliveries[0].ID).
		SetStatus("failed").
		SetAttemptCount(3).
		SetLastAttemptAt(failedAt).
		SetLastStatusCode(http.StatusInternalServerError).
		SetLastResponseBody("oops").
		SetLastError("server error").
		Save(ctx); err != nil {
		t.Fatalf("seed failed delivery error = %v", err)
	}
	enqueuer := &recordingEnqueuer{}

	delivery, err := webhooksvc.New(client, webhooksvc.WithEnqueuer(enqueuer)).RetryDelivery(ctx, deliveries[0].ID)
	if err != nil {
		t.Fatalf("RetryDelivery() error = %v", err)
	}

	if delivery.Status != "pending" || delivery.AttemptCount != 0 || delivery.LastAttemptAt != nil || delivery.LastStatusCode != 0 || delivery.LastResponseBody != "" || delivery.LastError != "" {
		t.Fatalf("delivery after retry = %#v, want reset pending delivery", delivery)
	}
	if len(enqueuer.ids) != 1 || enqueuer.ids[0] != delivery.ID {
		t.Fatalf("enqueued delivery ids = %#v, want [%d]", enqueuer.ids, delivery.ID)
	}
}

func TestDeliverWebhookMarksSucceededOnTwoHundredResponse(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:webhook_deliver_success?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	var gotEventType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEventType = r.Header.Get("X-Pay-Gateway-Event-Type")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	deliveryID := seedPendingDelivery(t, client, server.URL, time.Now().Add(-time.Minute))

	delivery, err := webhooksvc.New(client).DeliverWebhook(ctx, deliveryID)
	if err != nil {
		t.Fatalf("DeliverWebhook() error = %v", err)
	}

	if delivery.Status != "succeeded" || delivery.AttemptCount != 1 || delivery.SucceededAt == nil || delivery.LastStatusCode != http.StatusNoContent {
		t.Fatalf("delivery = %#v, want succeeded first attempt", delivery)
	}
	if gotEventType != webhooksvc.EventPaymentSucceeded {
		t.Fatalf("X-Pay-Gateway-Event-Type = %q, want %q", gotEventType, webhooksvc.EventPaymentSucceeded)
	}
}

func TestDeliverWebhookSchedulesRetryThenFailsAfterMaxAttempts(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:webhook_deliver_retry_fail?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer server.Close()

	deliveryID := seedPendingDelivery(t, client, server.URL, time.Now().Add(-time.Minute))
	service := webhooksvc.New(client)

	first, err := service.DeliverWebhook(ctx, deliveryID)
	if err != nil {
		t.Fatalf("first DeliverWebhook() error = %v", err)
	}
	if first.Status != "pending" || first.AttemptCount != 1 || first.NextAttemptAt == nil || first.LastStatusCode != http.StatusInternalServerError {
		t.Fatalf("first delivery = %#v, want pending retry state", first)
	}

	if _, err := client.WebhookDelivery.UpdateOneID(deliveryID).SetAttemptCount(2).Save(ctx); err != nil {
		t.Fatalf("seed attempt count error = %v", err)
	}
	final, err := service.DeliverWebhook(ctx, deliveryID)
	if err != nil {
		t.Fatalf("final DeliverWebhook() error = %v", err)
	}
	if final.Status != "failed" || final.AttemptCount != 3 || final.NextAttemptAt != nil || final.LastError == "" {
		t.Fatalf("final delivery = %#v, want failed max-attempt state", final)
	}
}

func TestScanDueDeliveriesEnqueuesPendingDueDeliveries(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:webhook_scan_due?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	dueID := seedPendingDelivery(t, client, "https://merchant.example.com/due", time.Now().Add(-time.Minute))
	_ = seedPendingDelivery(t, client, "https://merchant.example.com/future", time.Now().Add(time.Hour))
	enqueuer := &recordingEnqueuer{}

	count, err := webhooksvc.New(client, webhooksvc.WithEnqueuer(enqueuer)).ScanDueDeliveries(ctx, 10)
	if err != nil {
		t.Fatalf("ScanDueDeliveries() error = %v", err)
	}

	if count != 1 || len(enqueuer.ids) != 1 || enqueuer.ids[0] != dueID {
		t.Fatalf("scan count=%d ids=%#v, want only due id %d", count, enqueuer.ids, dueID)
	}
}

func TestMarkPaidEmitsPaymentSucceededOnlyOnFirstPaidConfirmation(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:webhook_order_mark_paid?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createApp(t, client, "snsgo_mark_paid", "https://merchant.example.com/webhooks/payment")
	orderService := ordersvc.New(client, ordersvc.WithWebhookService(webhooksvc.New(client)))
	order, err := orderService.CreateOrder(ctx, ordersvc.ManageOrderInput{
		AppID:           "snsgo_mark_paid",
		MerchantOrderNo: "biz_004",
		Subject:         "Pro",
		Amount:          9900,
		Currency:        "CNY",
		Channel:         "alipay",
		PayMethod:       "alipay",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}

	if _, err := orderService.UpdateOrder(ctx, order.ID, ordersvc.UpdateOrderInput{
		Subject:  "Pro Plus",
		Metadata: map[string]any{"plan": "plus"},
	}); err != nil {
		t.Fatalf("UpdateOrder() error = %v", err)
	}
	if _, totalEvents, err := webhookrepo.New(client).ListEvents(ctx, webhookrepo.ListEventsInput{}); err != nil || totalEvents != 0 {
		t.Fatalf("events after update total=%d err=%v, want 0", totalEvents, err)
	}

	if _, err := orderService.MarkPaid(ctx, order.ID, "trade_001"); err != nil {
		t.Fatalf("first MarkPaid() error = %v", err)
	}
	if _, err := orderService.MarkPaid(ctx, order.ID, "trade_001"); err != nil {
		t.Fatalf("second MarkPaid() error = %v", err)
	}
	_, totalEvents, err := webhookrepo.New(client).ListEvents(ctx, webhookrepo.ListEventsInput{})
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	_, totalDeliveries, err := webhookrepo.New(client).ListDeliveries(ctx, webhookrepo.ListDeliveriesInput{})
	if err != nil {
		t.Fatalf("ListDeliveries() error = %v", err)
	}
	if totalEvents != 1 || totalDeliveries != 1 {
		t.Fatalf("totals events=%d deliveries=%d, want one payment.succeeded event and delivery", totalEvents, totalDeliveries)
	}
}

func createApp(t *testing.T, client *ent.Client, appID string, notifyURL string) {
	t.Helper()
	if _, err := appsvc.New(client).CreateApp(context.Background(), appsvc.ManageAppInput{
		AppID:     appID,
		Name:      appID,
		NotifyURL: notifyURL,
		Status:    "enabled",
	}); err != nil {
		t.Fatalf("CreateApp(%q) error = %v", appID, err)
	}
}

func createPaidOrder(t *testing.T, client *ent.Client, appID string, merchantOrderNo string) *ent.PaymentOrder {
	t.Helper()
	orderService := ordersvc.New(client)
	order, err := orderService.CreateOrder(context.Background(), ordersvc.ManageOrderInput{
		AppID:           appID,
		MerchantOrderNo: merchantOrderNo,
		Subject:         "Pro",
		Amount:          9900,
		Currency:        "CNY",
		Channel:         "alipay",
		PayMethod:       "alipay",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	paid, err := orderService.MarkPaid(context.Background(), order.ID, "trade_"+merchantOrderNo)
	if err != nil {
		t.Fatalf("MarkPaid() error = %v", err)
	}
	return paid
}

func createExpiredOrder(t *testing.T, client *ent.Client, appID string, merchantOrderNo string) *ent.PaymentOrder {
	t.Helper()
	now := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	orderService := ordersvc.New(client, ordersvc.WithNow(func() time.Time { return now }), ordersvc.WithDefaultOrderTTL(15*time.Minute))
	order, err := orderService.CreateOrder(context.Background(), ordersvc.ManageOrderInput{
		AppID:           appID,
		MerchantOrderNo: merchantOrderNo,
		Subject:         "Pro",
		Amount:          9900,
		Currency:        "CNY",
		Channel:         "alipay",
		PayMethod:       "alipay",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	expiredService := ordersvc.New(client, ordersvc.WithNow(func() time.Time { return now.Add(16 * time.Minute) }))
	closed, err := expiredService.CloseExpiredPendingOrder(context.Background(), order.ID)
	if err != nil {
		t.Fatalf("CloseExpiredPendingOrder() error = %v", err)
	}
	if !closed {
		t.Fatal("CloseExpiredPendingOrder() closed = false, want true")
	}
	expired, err := expiredService.FindOrder(context.Background(), order.ID)
	if err != nil {
		t.Fatalf("FindOrder() error = %v", err)
	}
	return expired
}

func seedPendingDelivery(t *testing.T, client *ent.Client, targetURL string, nextAttemptAt time.Time) int {
	t.Helper()
	suffix := strings.NewReplacer(":", "", ".", "", "-", "", "/", "", "_", "").Replace(nextAttemptAt.Format("150405.000000000"))
	appID := fmt.Sprintf("snsgo_%s_%d", suffix, len(targetURL))
	createApp(t, client, appID, targetURL)
	order := createPaidOrder(t, client, appID, "biz_"+suffix)
	if _, err := webhooksvc.New(client).RecordPaymentSucceeded(context.Background(), order); err != nil {
		t.Fatalf("RecordPaymentSucceeded() error = %v", err)
	}
	deliveries, _, err := webhookrepo.New(client).ListDeliveries(context.Background(), webhookrepo.ListDeliveriesInput{GatewayOrderNo: order.GatewayOrderNo})
	if err != nil {
		t.Fatalf("ListDeliveries() error = %v", err)
	}
	if _, err := client.WebhookDelivery.UpdateOneID(deliveries[0].ID).SetNextAttemptAt(nextAttemptAt).Save(context.Background()); err != nil {
		t.Fatalf("set next attempt error = %v", err)
	}
	return deliveries[0].ID
}
