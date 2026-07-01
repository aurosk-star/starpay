# Payment Core End-to-End Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the production-shaped payment main flow from order creation to Alipay payment initiation, callback handling, event creation, webhook outbox records, and checkout success display.

**Architecture:** Keep checkout and open APIs as callers, but move real provider work into `internal/domain/payments`. Payment initiation selects an enabled channel account, calls a provider adapter, returns a provider-neutral `PaymentResult`, and never embeds SDK logic in order or checkout handlers. Alipay is the first real provider; WeChat and PayPal remain behind the same provider interface for later implementation.

**Tech Stack:** Go, Gin, Ent, PostgreSQL, Redis, gopay latest installed in `go.mod`, React 19, TanStack Router, Bun, shadcn/ui.

## Global Constraints

- Use the global response shape `{ code, message, data, error }`.
- Amounts remain integer minor units only.
- Do not implement automatic currency conversion or exchange rate logic.
- Payment initiation must be channel-neutral and caller-neutral.
- Order handlers and checkout handlers must not call gopay directly.
- Alipay is the first real provider to implement.
- When Alipay channel config is missing or disabled in local development, keep mock provider fallback available for checkout testing.
- All payment state changes must be idempotent.
- After backend changes, build the latest backend binary and restart the backend before handoff.
- Tests belong under each module’s `test/` directory.

---

### Task 1: Payment Provider Abstraction And Channel Account Selection

**Files:**
- Create: `internal/domain/payments/provider/provider.go`
- Create: `internal/domain/payments/provider/mock/mock.go`
- Create: `internal/domain/payments/test/provider_test.go`
- Modify: `internal/domain/channels/repository/repository.go`
- Modify: `internal/domain/payments/service/service.go`

**Interfaces:**
- Consumes: `ent.PaymentOrder`, requested `pay_method`, requested `channel`, channel account config.
- Produces:
  - `provider.Provider`
  - `provider.StartPaymentRequest`
  - `provider.StartPaymentResult`
  - `channels.Repository.FindEnabledByChannel(ctx, channel string)`

- [ ] **Step 1: Write failing provider service tests**

Create tests proving:
- `StartPayment` uses an enabled channel account when one exists.
- `StartPayment` falls back to mock in local/dev when no enabled account exists.
- `StartPayment` rejects unsupported channels without fallback when fallback is disabled.

Run: `go test ./internal/domain/payments/test -run 'TestProviderSelection'`

Expected: compile failure because provider abstraction and channel lookup do not exist.

- [ ] **Step 2: Implement provider interface**

Define:

```go
type StartPaymentRequest struct {
    Order     *ent.PaymentOrder
    PayMethod string
    Channel   string
    ClientIP  string
    ReturnURL string
    NotifyURL string
}

type StartPaymentResult struct {
    Status          string
    Channel         string
    PayMethod       string
    ProviderOrderNo string
    PayURL          string
    QRCode          string
    FormHTML        string
}

type Provider interface {
    Channel() string
    StartPayment(ctx context.Context, req StartPaymentRequest) (*StartPaymentResult, error)
}
```

- [ ] **Step 3: Implement enabled channel lookup**

Add repository method:

```go
func (r Repository) FindEnabledByChannel(ctx context.Context, channel string) (*ent.ChannelAccount, error)
```

It must filter by `channel` and `enabled=true`, newest first.

- [ ] **Step 4: Refactor payment service to use providers**

`payments.Service` should accept:
- `ent.Client` or a channel repository
- a provider registry
- `AllowMockFallback bool`

Do not remove mock; use it as fallback only when explicitly allowed.

- [ ] **Step 5: Run provider selection tests**

Run: `go test ./internal/domain/payments/test -run 'TestProviderSelection'`

Expected: PASS.

### Task 2: Alipay gopay Provider

**Files:**
- Create: `internal/domain/payments/provider/alipay/alipay.go`
- Create: `internal/domain/payments/provider/alipay/config.go`
- Create: `internal/domain/payments/provider/alipay/alipay_test.go`
- Modify: `internal/domain/channels/service/service.go`
- Modify: `web/src/features/channels/config.ts`

**Interfaces:**
- Consumes channel account config fields:
  - `app_id`
  - `private_key`
  - `alipay_public_key`
  - `server_url`
  - `notify_url`
  - `return_url`
  - `product_code`
  - `mode` (`page` or `qr`)
- Produces an Alipay provider that calls gopay `TradePagePay` for page pay and `TradePrecreate` for QR pay.

- [ ] **Step 1: Write failing config validation tests**

Cover:
- missing `app_id` fails;
- missing `private_key` fails;
- default `product_code` is `FAST_INSTANT_TRADE_PAY`;
- `env=sandbox` creates a non-prod client.

Run: `go test ./internal/domain/payments/provider/alipay -run 'TestConfig'`

Expected: compile failure because package does not exist.

- [ ] **Step 2: Implement Alipay config parser**

Parse channel account config into:

```go
type Config struct {
    AppID           string
    PrivateKey      string
    AlipayPublicKey string
    NotifyURL       string
    ReturnURL       string
    ProductCode     string
    Mode            string
    IsProd          bool
}
```

- [ ] **Step 3: Write provider start-payment tests using a fake client boundary**

Do not hit Alipay network in unit tests. Wrap gopay calls behind a small interface so tests can assert request body fields:
- `out_trade_no`
- `subject`
- `total_amount`
- `product_code`
- `notify_url`
- `return_url`

- [ ] **Step 4: Implement gopay provider**

Use:
- `alipay.NewClient(appID, privateKey, isProd)`
- `client.SetNotifyUrl(...)`
- `client.SetReturnUrl(...)`
- `client.TradePagePay(...)` for redirect/page payments

For QR mode, use `TradePrecreate` and map QR code result into `PaymentResult.QRCode`.

- [ ] **Step 5: Run Alipay provider tests**

Run: `go test ./internal/domain/payments/provider/alipay`

Expected: PASS.

### Task 3: Checkout Payment Uses Real Alipay When Configured

**Files:**
- Modify: `internal/platform/http/router.go`
- Modify: `internal/domain/orders/handler/checkout_handler.go`
- Modify: `internal/domain/orders/test/checkout_payment_test.go`
- Modify: `internal/domain/payments/test/service_test.go`

**Interfaces:**
- Consumes: `payments.Service` with provider registry and channel repository.
- Produces: checkout pay API that returns Alipay `pay_url` or `qr_code` when Alipay channel config exists.

- [ ] **Step 1: Write failing checkout integration test**

Seed an enabled Alipay channel account with fake config and a fake provider registry. Call:

```text
POST /v1/checkout/orders/:gateway_order_no/pay
```

Assert:
- `payment.channel == "alipay"`
- `payment.pay_method == "alipay"`
- `payment.provider_order_no` is non-empty
- `payment.pay_url` is non-empty

- [ ] **Step 2: Wire payments service in router**

Construct `payments.Service` once in `internal/platform/http/router.go` with:
- Ent client
- channel repository
- mock fallback enabled for local env
- Alipay provider registered

- [ ] **Step 3: Keep mock fallback available**

If no enabled Alipay channel account exists in local env, checkout pay should still return mock provider output so frontend testing does not block on external credentials.

- [ ] **Step 4: Run checkout payment tests**

Run: `go test ./internal/domain/orders/test -run 'TestCheckoutHandler'`

Expected: PASS.

### Task 4: Alipay Callback And Idempotent Order Status Update

**Files:**
- Create: `internal/domain/payments/handler/callback_handler.go`
- Create: `internal/domain/payments/router/router.go`
- Create: `internal/domain/payments/test/callback_handler_test.go`
- Modify: `internal/platform/http/router.go`
- Modify: `internal/domain/orders/service/service.go`

**Interfaces:**
- Consumes: `POST /v1/callback/alipay`
- Produces: idempotent order update to paid/failed/closed and global response.

- [ ] **Step 1: Write failing callback tests**

Cover:
- successful trade status marks order paid;
- duplicate success callback remains 200 and does not corrupt state;
- callback for unknown order returns a controlled error;
- paid order with mismatched amount is rejected.

- [ ] **Step 2: Parse and verify Alipay notify**

Use gopay:
- `alipay.ParseNotifyToBodyMap`
- `alipay.VerifySign` or certificate verification when configured.

For tests, inject a verifier interface so unit tests do not need real Alipay signatures.

- [ ] **Step 3: Implement idempotent status transition**

Add service method:

```go
func (s Service) MarkPaidByGatewayOrderNo(ctx context.Context, gatewayOrderNo string, channelTradeNo string) (*ent.PaymentOrder, error)
```

It must:
- allow `pending -> paid`;
- allow `paid -> paid` idempotently;
- reject `closed -> paid`.

- [ ] **Step 4: Register callback route**

Register:

```text
POST /v1/callback/alipay
```

- [ ] **Step 5: Run callback tests**

Run: `go test ./internal/domain/payments/test -run 'TestAlipayCallback'`

Expected: PASS.

### Task 5: Payment Events And Webhook Outbox

**Files:**
- Create: `ent/schema/payment_event.go`
- Create: `ent/schema/webhook_delivery.go`
- Create: `internal/domain/webhooks/repository/repository.go`
- Create: `internal/domain/webhooks/service/service.go`
- Create: `internal/domain/webhooks/test/service_test.go`
- Modify: `internal/domain/orders/service/service.go`

**Interfaces:**
- Consumes: paid/failed/closed payment status changes.
- Produces:
  - `payment_events`
  - `webhook_deliveries`
  - `webhooks.Service.RecordPaymentEvent`

- [ ] **Step 1: Write failing webhook outbox tests**

Cover:
- paid order creates one `payment.succeeded` event;
- duplicate paid transition does not create duplicate events;
- app `notify_url` creates one delivery record;
- missing `notify_url` creates event but no delivery.

- [ ] **Step 2: Add Ent schemas**

Create schemas with fields:
- event id, type, app id, gateway order no, payload, status, created_at;
- delivery id, event id, target_url, status, attempts, last_error, next_retry_at, created_at, updated_at.

- [ ] **Step 3: Regenerate Ent**

Run: `make ent-up`

- [ ] **Step 4: Implement outbox service**

Events must be idempotent by event type and gateway order number.

- [ ] **Step 5: Call outbox from payment status transitions**

When order becomes paid through callback or mock complete, create event and delivery records.

### Task 6: Admin Observability

**Files:**
- Create: `internal/domain/webhooks/handler/handler.go`
- Create: `internal/domain/webhooks/router/router.go`
- Create: `web/src/features/webhooks/*`
- Create: `web/src/routes/webhooks.tsx`
- Modify: `web/src/router.tsx`
- Modify: `web/src/routes/root.tsx`
- Modify: `web/src/i18n/resources.ts`

**Interfaces:**
- Consumes: webhook event and delivery records.
- Produces: admin list pages using shadcn Data Table.

- [ ] **Step 1: Backend list endpoints**

Add:
- `GET /v1/admin/payment-events`
- `GET /v1/admin/webhook-deliveries`

- [ ] **Step 2: Frontend pages**

Add admin Webhook page with tabs:
- events
- deliveries

All lists must use `web/src/components/data-table/`.

- [ ] **Step 3: Verification**

Run:
- `go test ./...`
- `bun run typecheck`
- `bun run build`

### Task 7: End-to-End Verification

**Files:**
- Modify only files touched above.

**Interfaces:**
- Produces: a runnable Alipay-first payment flow.

- [ ] **Step 1: Configure Alipay sandbox channel**

Use admin channel page to create enabled Alipay channel account with:
- app id
- private key
- Alipay public key or cert config
- notify URL
- return URL
- env=sandbox

- [ ] **Step 2: Run local flow**

Run:
- create test payment from admin home;
- checkout starts Alipay payment;
- assert `payment.pay_url` points to Alipay sandbox when config exists;
- trigger sandbox callback or callback fixture;
- assert order becomes paid;
- assert payment event exists;
- assert webhook delivery record exists.

- [ ] **Step 3: Build and restart**

Run:
- `go test ./...`
- `go build -o ./server ./cmd/server`
- restart backend with `.env`
- `bun run typecheck`
- `bun run build`

