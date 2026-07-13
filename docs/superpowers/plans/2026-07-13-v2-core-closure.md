# V2 Core Payment Closure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver active payment reconciliation, resource-aware webhooks, three-channel refunds, administrator recovery workflows, SDK support, and one-command verification.

**Architecture:** Add durable reconciliation and refund Ent models, extend the existing `payments/provider` capability interfaces, and reuse Redis Streams for recovery workers. Keep payment order transitions in the order service, refund behavior in a new refunds domain, and HTTP serialization in handlers. Migrate webhook uniqueness from order identity to generic resource identity before emitting refund events.

**Tech Stack:** Go 1.24, Gin, Ent, PostgreSQL/MySQL/SQLite tests, Redis Streams, go-pay, React 19, TypeScript, shadcn/ui, Rsbuild, Go SDK.

## Global Constraints

- Store money only as integer minor units.
- Never apply provider results without channel, account, identity, amount, and currency validation.
- Use tests before each behavior change and observe the intended failure.
- Use the existing `{ code, message, data, error }` response envelope.
- Use the current `payments/provider` abstraction; do not wire the unused `channels/service.Adapter`.
- Use the reusable frontend Data Table factory and existing shadcn components.
- Preserve existing payment and webhook data during migration.
- After backend changes, regenerate Ent, run all verification, rebuild the binary, and restart the backend service.

---

### Task 1: Migrate Webhooks to Resource Identity

**Files:**
- Modify: `ent/schema/webhook_event.go`
- Modify: `ent/schema/webhook_delivery.go`
- Modify: `internal/platform/database/database.go`
- Create: `internal/platform/database/webhook_migration.go`
- Modify: `internal/domain/webhooks/repository/repository.go`
- Modify: `internal/domain/webhooks/service/service.go`
- Modify: `internal/domain/webhooks/handler/handler.go`
- Modify: `internal/domain/webhooks/test/service_test.go`
- Modify: `internal/domain/webhooks/test/handler_test.go`
- Regenerate: `ent/`

**Interfaces:**
- Produces: `ResourcePaymentOrder = "payment_order"` and generic event lookup by `(event_type, resource_type, resource_id)`.
- Produces: delivery filters `resource_type` and `refund_no`.
- Preserves: `RecordPaymentSucceeded`, `RecordPaymentFailed`, and `RecordOrderExpired` signatures.

- [x] **Step 1: Write failing resource-idempotency tests**

Add tests that assert payment events persist `resource_type=payment_order` and `resource_id=gateway_order_no`, duplicate payment events reuse the same event, and two generic refund resources under one gateway order can create separate events.

```go
func TestRecordResourceEventsAreUniquePerResource(t *testing.T) {
    first, err := service.RecordResourceEvent(ctx, webhooksvc.ResourceEventInput{
        EventType: "refund.succeeded", AppID: "snsgo",
        ResourceType: "refund", ResourceID: "rf_001",
        GatewayOrderNo: "gw_001", RefundNo: "rf_001", Payload: map[string]any{"refund_no": "rf_001"},
    })
    // Create rf_002 for the same gateway order and assert different IDs.
}
```

- [x] **Step 2: Run focused tests and verify missing resource APIs fail**

Run: `go test ./internal/domain/webhooks/test ./internal/platform/database`

Expected: FAIL because resource fields and `RecordResourceEvent` do not exist.

- [x] **Step 3: Add resource fields and targeted compatibility migration**

Schema fields:

```go
field.String("resource_type").Default("payment_order"),
field.String("resource_id"),
field.String("refund_no").Optional(),
```

Replace the event unique index with:

```go
index.Fields("event_type", "resource_type", "resource_id").Unique()
```

Before `client.Schema.Create`, `prepareWebhookResourceMigration` detects an existing `webhook_events` table, adds nullable columns, backfills `payment_order` and `gateway_order_no`, and drops only `webhookevent_event_type_gateway_order_no`. Implement separate PostgreSQL and MySQL statements; return errors instead of ignoring failed migrations.

- [x] **Step 4: Implement generic event persistence and compatible payment wrappers**

```go
type ResourceEventInput struct {
    EventType, AppID, ResourceType, ResourceID string
    GatewayOrderNo, RefundNo string
    PaymentOrderID int
    Payload map[string]any
}

func (s Service) RecordResourceEvent(ctx context.Context, input ResourceEventInput) (*ent.WebhookEvent, error)
```

Payment wrappers populate `payment_order` and the gateway order number. Delivery creation copies resource fields and refund number. List filters reach repository predicates.

- [x] **Step 5: Regenerate Ent and verify webhook behavior**

Run:

```bash
make ent-up
go test ./internal/domain/webhooks/test ./internal/platform/database
```

Expected: PASS.

- [x] **Step 6: Commit the webhook migration unit**

```bash
git add ent internal/domain/webhooks internal/platform/database
git commit -m "Migrate webhooks to resource identity"
```

---

### Task 2: Add Provider Query, Close, and Refund Capabilities

**Files:**
- Modify: `internal/domain/payments/provider/provider.go`
- Modify: `internal/domain/payments/service/service.go`
- Modify: `internal/domain/payments/provider/alipay/alipay.go`
- Modify: `internal/domain/payments/provider/alipay/alipay_test.go`
- Modify: `internal/domain/payments/provider/wechat/wechat.go`
- Modify: `internal/domain/payments/provider/wechat/wechat_test.go`
- Modify: `internal/domain/payments/provider/paypal/paypal.go`
- Modify: `internal/domain/payments/provider/paypal/paypal_test.go`
- Modify: `internal/domain/payments/test/service_test.go`

**Interfaces:**
- Produces: `QueryProvider`, `CloseProvider`, `RefundProvider`, and `RefundQueryProvider`.
- Produces service methods: `QueryPayment`, `ClosePayment`, `CreateRefund`, and `QueryRefund`.
- All service methods resolve the exact bound channel account ID before calling a provider.

- [x] **Step 1: Write failing provider mapping tests**

Cover these normalized mappings:

- Alipay `TRADE_SUCCESS/TRADE_FINISHED -> paid`, `TRADE_CLOSED -> closed`, wait states -> pending.
- WeChat `SUCCESS -> paid`, `CLOSED/REVOKED -> closed`, `PAYERROR -> failed`, other states -> pending.
- PayPal completed capture -> paid, voided -> closed, denied -> failed, approved/created -> pending.
- Refund results map provider terminal and pending states without converting ambiguous errors to failed.

- [x] **Step 2: Run provider tests and verify capability methods are absent**

Run: `go test ./internal/domain/payments/provider/... ./internal/domain/payments/test`

Expected: FAIL on missing query/refund client methods and capability interfaces.

- [x] **Step 3: Add normalized provider request and result types**

```go
type QueryPaymentRequest struct { ChannelAccount *ent.ChannelAccount; Order *ent.PaymentOrder }
type QueryPaymentResult struct {
    Channel, GatewayOrderNo, ProviderOrderNo, ChannelTradeNo, Status, Currency, FailureReason string
    Amount int64
    Raw map[string]any
}
type CreateRefundRequest struct {
    ChannelAccount *ent.ChannelAccount
    GatewayOrderNo, ProviderOrderNo, ChannelTradeNo string
    RefundNo, Currency, Reason string
    Amount int64
}
type QueryRefundRequest struct {
    ChannelAccount *ent.ChannelAccount
    GatewayOrderNo, ChannelTradeNo, RefundNo, ChannelRefundNo string
}
type RefundResult struct { Channel, RefundNo, ChannelRefundNo, Status, Currency, FailureReason string; Amount int64; Raw map[string]any }
```

- [x] **Step 4: Implement Alipay capabilities using go-pay v3**

Extend the test client interface with `TradeQuery`, `TradeClose`, `TradeRefund`, and `TradeFastPayRefundQuery`. Build requests with `out_trade_no`, `trade_no`, `refund_amount`, and `out_request_no`. Reject non-200 responses with provider details and parse amounts without floats.

- [x] **Step 5: Implement WeChat API v3 capabilities**

Extend the client interface with transaction query, close, refund, and refund query. Use `OutTradeNo`, `out_refund_no=refund_no`, original/ refund integer amounts, and CNY. Validate response bodies and normalize WeChat refund states.

- [x] **Step 6: Implement PayPal capabilities**

Extend the client interface with order detail, capture refund, and refund detail. Use the stored capture ID as `channel_trade_no`; set `PayPal-Request-Id` to `refund_no`; set `invoice_id`, amount, and currency. Parse PayPal amounts with the existing zero-decimal currency rules.

- [x] **Step 7: Add service-level account resolution and capability errors**

Return explicit `ErrQueryUnsupported`, `ErrCloseUnsupported`, `ErrRefundUnsupported`, and `ErrRefundQueryUnsupported`. Never fall back to another account when an order has `channel_account_id`.

- [ ] **Step 8: Run payment tests and commit**

```bash
go test ./internal/domain/payments/provider/... ./internal/domain/payments/test
git add internal/domain/payments
git commit -m "Add provider query and refund capabilities"
```

---

### Task 3: Build Payment Reconciliation and Manual Recovery

**Files:**
- Create: `ent/schema/payment_reconciliation.go`
- Create: `internal/domain/reconciliations/repository/repository.go`
- Create: `internal/domain/reconciliations/service/enqueuer.go`
- Create: `internal/domain/reconciliations/service/service.go`
- Create: `internal/domain/reconciliations/service/worker.go`
- Create: `internal/domain/reconciliations/handler/handler.go`
- Create: `internal/domain/reconciliations/router/router.go`
- Create: `internal/domain/reconciliations/test/service_test.go`
- Create: `internal/domain/reconciliations/test/worker_test.go`
- Create: `internal/domain/reconciliations/test/handler_test.go`
- Modify: `internal/domain/orders/repository/repository.go`
- Modify: `internal/domain/orders/service/service.go`
- Modify: `internal/domain/orders/handler/checkout_handler.go`
- Modify: `internal/domain/orders/test/checkout_payment_test.go`
- Modify: `internal/platform/http/router.go`
- Modify: `cmd/server/main.go`
- Regenerate: `ent/`

**Interfaces:**
- Produces: `EnsureForOrder`, `ScanDue`, `Process`, `Retry`, `List`, and `Get`.
- Consumes: `payments.Service.QueryPayment/ClosePayment` and `orders.Service.ApplyPaymentResult`.
- Redis stream: `payment:reconciliations`; group: `payment-reconciliation-workers`.

- [ ] **Step 1: Write failing reconciliation state-machine tests**

Test paid, failed, closed, pending retry, expired-provider-close, validation error, stale processing recovery, maximum-attempt manual state, duplicate stream messages, and pre-upgrade order backfill.

- [ ] **Step 2: Run tests and verify missing schema/domain failure**

Run: `go test ./internal/domain/reconciliations/test ./internal/domain/orders/test`

Expected: FAIL because reconciliation packages and schema are absent.

- [ ] **Step 3: Add schema, repository claims, and retry schedule**

Implement conditional claim:

```go
UPDATE payment_reconciliations
SET status='processing', last_attempt_at=now
WHERE id=? AND status='pending'
```

Retry delays are exactly `2m, 5m, 10m, 30m, 1h, 2h, 6h, 24h`; attempt eight moves to `manual_required`. Scanner restores processing claims older than five minutes.

- [ ] **Step 4: Implement reconciliation service and worker**

`Process` reloads the order, skips terminal orders as resolved, queries the bound provider, validates/applies terminal results, and reschedules pending/errors. Expired pending provider orders are closed through `ClosePayment`; PayPal unsupported close is treated as local close only after a successful pending query.

- [ ] **Step 5: Integrate payment initiation and expiration behavior**

After checkout persists payment selection and provider order number, call `EnsureForOrder`. Change expiration scanning so provider-bound orders are left for reconciliation while unbound pending orders retain existing expiration closure.

- [ ] **Step 6: Add admin list/detail/retry endpoints and worker wiring**

Register `/v1/admin/payment-reconciliations` routes with RBAC. Start scanner and consumers in `cmd/server/main.go`, and add the stream to monitoring queue targets.

- [ ] **Step 7: Regenerate Ent, run tests, and commit**

```bash
make ent-up
go test ./internal/domain/reconciliations/test ./internal/domain/orders/test ./internal/domain/monitoring/...
git add ent internal/domain/reconciliations internal/domain/orders internal/platform/http cmd/server/main.go
git commit -m "Add payment reconciliation workers"
```

---

### Task 4: Build the Refund Domain and Webhook Events

**Files:**
- Create: `ent/schema/refund.go`
- Create: `internal/domain/refunds/repository/repository.go`
- Create: `internal/domain/refunds/service/enqueuer.go`
- Create: `internal/domain/refunds/service/service.go`
- Create: `internal/domain/refunds/service/worker.go`
- Create: `internal/domain/refunds/handler/open_handler.go`
- Create: `internal/domain/refunds/handler/admin_handler.go`
- Create: `internal/domain/refunds/router/open_router.go`
- Create: `internal/domain/refunds/router/router.go`
- Create: `internal/domain/refunds/test/service_test.go`
- Create: `internal/domain/refunds/test/handler_test.go`
- Create: `internal/domain/refunds/test/worker_test.go`
- Modify: `internal/domain/webhooks/service/service.go`
- Modify: `internal/domain/webhooks/test/service_test.go`
- Modify: `internal/platform/http/router.go`
- Modify: `cmd/server/main.go`
- Regenerate: `ent/`

**Interfaces:**
- Produces: open create/get/by-merchant APIs and admin list/get/create/retry APIs.
- Produces: `refund.succeeded` and `refund.failed` resource events.
- Redis stream: `refund:processing`; group: `refund-processing-workers`.

- [ ] **Step 1: Write failing refund validation and idempotency tests**

Cover paid-only validation, positive amount, currency match, app ownership, exact idempotent replay, conflicting replay, partial refunds, aggregate over-refund, pending reservation, failed reservation release, and two concurrent requests that cannot exceed the paid amount.

- [ ] **Step 2: Run refund tests and verify missing domain failure**

Run: `go test ./internal/domain/refunds/test`

Expected: FAIL because refund schema and service do not exist.

- [ ] **Step 3: Add refund schema and transactional reservation repository**

Use a transaction and lock the payment order for PostgreSQL/MySQL before summing `pending` and `succeeded` refunds. SQLite tests use the transaction without `FOR UPDATE`. Create the pending refund before any provider request.

- [ ] **Step 4: Implement refund service state transitions**

`Create` performs validation/reservation then invokes provider creation with the persisted refund. Network ambiguity remains pending. `Process` repeats creation with the same refund number when no channel refund ID exists, otherwise queries. Terminal updates are idempotent and emit exactly one resource-aware webhook event.

- [ ] **Step 5: Add workers and API handlers**

Use signed Open API app context for app-scoped create and reads. Admin create accepts `gateway_order_no`, `merchant_refund_no`, amount, reason, and metadata. Retry resets failed or unresolved records to pending and enqueues immediately.

- [ ] **Step 6: Wire routes, workers, monitoring, and webhook filters**

Register refund routes after the Open API middleware and under the protected admin group. Start scanner/consumers and expose the refund stream in monitoring.

- [ ] **Step 7: Regenerate Ent and verify refund/webhook tests**

```bash
make ent-up
go test ./internal/domain/refunds/test ./internal/domain/webhooks/test
```

- [ ] **Step 8: Commit the refund backend unit**

```bash
git add ent internal/domain/refunds internal/domain/webhooks internal/platform/http cmd/server/main.go
git commit -m "Add idempotent refund lifecycle"
```

---

### Task 5: Update the Go SDK

**Files:**
- Modify: `sdk/go/types.go`
- Modify: `sdk/go/client.go`
- Modify: `sdk/go/client_test.go`
- Modify: `sdk/go/webhook.go`
- Modify: `sdk/go/webhook_test.go`
- Modify: `sdk/go/README.md`
- Modify: `sdk/go/CHANGELOG.md`

**Interfaces:**
- Produces: `CreateRefund`, `GetRefund`, and `GetRefundByMerchant`.
- Produces webhook resource and refund fields.

- [ ] **Step 1: Write failing SDK request-path and decoding tests**

Assert signed requests use the three Open API paths, request bodies retain integer amount and metadata, API errors decode normally, and refund webhook fields verify without changing signature behavior.

- [ ] **Step 2: Run SDK tests and verify missing methods fail**

Run: `cd sdk/go && go test -count=1 ./...`

- [ ] **Step 3: Add refund types and client methods**

```go
func (c *Client) CreateRefund(ctx context.Context, input CreateRefundRequest) (*CreateRefundResult, error)
func (c *Client) GetRefund(ctx context.Context, refundNo string) (*Refund, error)
func (c *Client) GetRefundByMerchant(ctx context.Context, merchantRefundNo string) (*Refund, error)
```

- [ ] **Step 4: Update docs/changelog and run SDK verification**

```bash
cd sdk/go
go test -count=1 ./...
go vet ./...
```

- [ ] **Step 5: Commit the SDK unit**

```bash
git add sdk/go
git commit -m "Add refund support to Go SDK"
```

---

### Task 6: Add Reconciliation and Refund Admin Pages

**Files:**
- Create: `web/src/features/reconciliations/api.ts`
- Create: `web/src/features/reconciliations/types.ts`
- Create: `web/src/features/reconciliations/reconciliations-page.tsx`
- Create: `web/src/features/reconciliations/reconciliation-detail-page.tsx`
- Create: `web/src/features/refunds/api.ts`
- Create: `web/src/features/refunds/types.ts`
- Create: `web/src/features/refunds/refunds-page.tsx`
- Create: `web/src/features/refunds/refund-detail-page.tsx`
- Create: `web/src/routes/reconciliations.tsx`
- Create: `web/src/routes/reconciliation-detail.tsx`
- Create: `web/src/routes/refunds.tsx`
- Create: `web/src/routes/refund-detail.tsx`
- Modify: `web/src/router.tsx`
- Modify: `web/src/routes/root.tsx`
- Modify: `web/src/features/orders/order-detail-page.tsx`
- Modify: `web/src/features/orders/types.ts`
- Modify: `web/src/features/webhooks/webhooks-page.tsx`
- Modify: `web/src/features/webhooks/types.ts`
- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/zh-CN.json`
- Create: `web/test/refund-and-reconciliation-filters.test.mts`

**Interfaces:**
- Consumes the new admin APIs.
- Enables existing Refund navigation and adds Compensation navigation.

- [ ] **Step 1: Write failing frontend contract tests**

Test query parameter builders, supported status options, event resource filters, and amount formatting without rendering internals.

- [ ] **Step 2: Run Node tests and verify missing modules fail**

Run: `cd web && node --test test/*.test.mts`

- [ ] **Step 3: Add typed APIs and Data Table pages**

Use full-width filter bands, existing Cards only for repeated/detail items, `Select` controls for finite status/channel/resource values, and Data Table factory for lists. Retry buttons use `RotateCcw`; detail links use `Eye`.

- [ ] **Step 4: Add routes, navigation, order summaries, and translations**

Ensure all English and Chinese text exists, controls fit mobile widths, and no route remains disabled.

- [ ] **Step 5: Run frontend verification and commit**

```bash
cd web
node --test test/*.test.mts
bun run lint
bun run typecheck
bun run build
git add web
git commit -m "Add refund and reconciliation admin workflows"
```

---

### Task 7: Unify Verification, Documentation, and Runtime Handoff

**Files:**
- Modify: `Makefile`
- Modify: `docs/PAYMENT_GATEWAY_INTEGRATION.md`
- Modify: `docs/v2prd.md`
- Modify: `docs/PRODUCTION_DEPLOYMENT.md`
- Modify: `docs/superpowers/plans/2026-07-13-v2-core-closure.md`

**Interfaces:**
- Produces: `make web-test` and `make verify`.
- Changes `make test` to include root Go tests and SDK Go tests.

- [ ] **Step 1: Update Make targets**

```make
test:
	go test ./...
	cd sdk/go && go test -count=1 ./...

web-test:
	cd web && node --test test/*.test.mts

verify: test web-test web-typecheck web-build
	cd sdk/go && go vet ./...
	cd web && bun run lint
```

- [ ] **Step 2: Document reconciliation, refunds, events, retries, and operations**

Add exact Open API examples, webhook payloads, retry/manual recovery behavior, required PayPal/WeChat configuration, and deployment migration order. Mark only implemented V2 acceptance items complete.

- [ ] **Step 3: Run fresh full verification**

```bash
make verify
git diff --check
```

Expected: every command exits 0.

- [ ] **Step 4: Build and restart the backend from the latest binary**

```bash
go build -o .tmp/payment-gateway-server ./cmd/server
systemctl --user restart payment-gateway-dev.service
```

Verify the running `/proc/<pid>/exe` hash matches `.tmp/payment-gateway-server`, `/healthz` returns 200, and refund/reconciliation routes are registered.

- [ ] **Step 5: Commit final integration documentation**

```bash
git add Makefile docs
git commit -m "Document V2 payment lifecycle operations"
```
