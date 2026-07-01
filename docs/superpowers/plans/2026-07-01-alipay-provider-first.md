# Alipay Provider First Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace mock payment initiation with a real Alipay gopay provider path while keeping the same checkout contract and a local fallback for development.

**Architecture:** Keep checkout and admin pages unchanged at the edges: checkout still calls `payments.Service.StartPayment`, and payment results still return the same neutral `PaymentResult`. Inside `internal/domain/payments`, add a provider registry, an Alipay provider backed by `gopay`, and a channel-account selector that pulls live credentials from the enabled Alipay channel account. Local development keeps mock fallback only when no enabled Alipay account exists.

**Tech Stack:** Go, Gin, Ent, PostgreSQL, Redis, `github.com/go-pay/gopay` v1.5.121, React 19, TanStack Router, Bun, shadcn/ui.

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

### Task 1: Provider Registry And Enabled Channel Lookup

**Files:**
- Create: `internal/domain/payments/provider/provider.go`
- Create: `internal/domain/payments/provider/mock/mock.go`
- Modify: `internal/domain/channels/repository/repository.go`
- Modify: `internal/domain/payments/service/service.go`
- Test: `internal/domain/payments/test/provider_selection_test.go`

**Interfaces:**
- Consumes: `ent.PaymentOrder`, `pay_method`, `channel`, `client_ip`, runtime `return_url`, and runtime `notify_url`.
- Produces:
  - `provider.Provider`
  - `provider.StartPaymentRequest`
  - `provider.StartPaymentResult`
  - `channels.Repository.FindEnabledByChannel(ctx, channel string)`

- [ ] **Step 1: Write failing tests for provider selection**

Write tests proving:

```go
func TestStartPaymentUsesEnabledAlipayProvider(t *testing.T)
func TestStartPaymentFallsBackToMockWithoutAlipayAccount(t *testing.T)
```

Run: `go test ./internal/domain/payments/test -run 'TestStartPayment'`

Expected: compile failure because provider registry and enabled-channel lookup do not exist yet.

- [ ] **Step 2: Implement provider interface**

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

- [ ] **Step 3: Add enabled channel lookup**

Add repository method:

```go
func (r Repository) FindEnabledByChannel(ctx context.Context, channel string) (*ent.ChannelAccount, error)
```

Filter by `channel` and `enabled=true`, newest first.

- [ ] **Step 4: Refactor payment service constructor**

`payments.Service` should accept:
- channel repository
- provider registry
- `AllowMockFallback bool`

Keep existing checkout-facing `StartPayment` signature stable.

- [ ] **Step 5: Run provider selection tests**

Run: `go test ./internal/domain/payments/test -run 'TestStartPayment'`

Expected: PASS.

### Task 2: Alipay Config Parsing And Provider

**Files:**
- Create: `internal/domain/payments/provider/alipay/alipay.go`
- Create: `internal/domain/payments/provider/alipay/config.go`
- Create: `internal/domain/payments/provider/alipay/alipay_test.go`
- Modify: `web/src/features/channels/config.ts`
- Modify: `web/src/features/channels/channel-form-page.tsx`
- Modify: `internal/domain/channels/service/service.go`

**Interfaces:**
- Consumes channel account config fields:
  - `app_id`
  - `private_key`
  - `alipay_public_key`
  - `server_url`
  - `product_code`
  - `mode`
- Produces an Alipay provider that calls gopay `TradePagePay` for page pay and `TradePrecreate` for QR pay.

- [ ] **Step 1: Write config parser tests**

Cover:
- missing `app_id` fails;
- missing `private_key` fails;
- default `product_code` is `FAST_INSTANT_TRADE_PAY`;
- `mode=qr` selects QR behavior.

Run: `go test ./internal/domain/payments/provider/alipay -run 'TestConfig'`

Expected: compile failure because package does not exist yet.

- [ ] **Step 2: Implement config parser**

```go
type Config struct {
    AppID           string
    PrivateKey      string
    AlipayPublicKey string
    ServerURL       string
    ProductCode     string
    Mode            string
    IsProd          bool
}
```

- [ ] **Step 3: Implement provider tests with fake gopay boundary**

Wrap gopay behind a thin interface so tests can assert request values:
- `out_trade_no`
- `subject`
- `total_amount`
- `product_code`
- `notify_url`
- `return_url`

`notify_url` and `return_url` must come from `StartPaymentRequest`; they are not stored in the channel account config.

- [ ] **Step 4: Implement Alipay provider**

Use:
- `alipay.NewClient(appID, privateKey, isProd)`
- `client.SetNotifyUrl(...)`
- `client.SetReturnUrl(...)`
- `client.SetSignType(...)`
- `client.TradePagePay(...)` for page mode
- `client.TradePrecreate(...)` for QR mode

- [ ] **Step 5: Update channel form fields**

Ensure the admin channel form shows and persists:
- `product_code`
- `mode`

Keep `server_url` as the Alipay gateway URL.

- [ ] **Step 6: Run Alipay provider tests**

Run:
- `go test ./internal/domain/payments/provider/alipay`
- `go test ./internal/domain/channels/test`

Expected: PASS.

### Task 3: Route Wiring And Checkout Real Alipay Usage

**Files:**
- Modify: `internal/platform/http/router.go`
- Modify: `internal/domain/orders/handler/checkout_handler.go`
- Modify: `internal/domain/orders/test/checkout_payment_test.go`
- Modify: `internal/domain/payments/service/service.go`

**Interfaces:**
- Consumes: enabled Alipay channel account from repository.
- Produces: checkout pay API returns a real Alipay `pay_url` or `qr_code` when config exists, with runtime callback URLs passed through the payment provider request.

- [ ] **Step 1: Write failing checkout integration tests**

Seed an enabled Alipay channel account with fake config and assert:

```text
POST /v1/checkout/orders/:gateway_order_no/pay
```

returns:
- `payment.channel == "alipay"`
- `payment.pay_method == "alipay"`
- non-empty `payment.provider_order_no`
- non-empty `payment.pay_url`

- [ ] **Step 2: Wire the real provider into router startup**

Construct `payments.Service` in `internal/platform/http/router.go` with:
- Ent client
- channel repository
- mock fallback enabled only in local development
- Alipay provider registered

- [ ] **Step 3: Keep mock fallback**

If no enabled Alipay account exists locally, continue returning mock provider output so frontend testing remains possible.

- [ ] **Step 4: Run checkout tests**

Run: `go test ./internal/domain/orders/test -run 'TestCheckoutHandler'`

Expected: PASS.

### Task 4: Verification And Restart

**Files:**
- Modify only files touched above.

**Interfaces:**
- Produces: a working Alipay-first checkout payment path.

- [ ] **Step 1: Run full backend tests**

Run: `go test ./...`

- [ ] **Step 2: Build latest backend binary**

Run: `go build -o ./server ./cmd/server`

- [ ] **Step 3: Restart backend**

Load `.env`, stop the old server, and start the new binary.

- [ ] **Step 4: Verify checkout flow**

Run:
- `curl -i http://127.0.0.1:8080/healthz`
- `curl -i http://127.0.0.1:8080/v1/checkout/orders/:gateway_order_no`
- `curl -i -X POST http://127.0.0.1:8080/v1/checkout/orders/:gateway_order_no/pay`

Expected: checkout pay returns a real provider result when Alipay config exists, mock fallback otherwise.
