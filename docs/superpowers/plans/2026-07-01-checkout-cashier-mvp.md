# Checkout 收银台 MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a minimal gateway cashier flow with a neutral payment initiation layer so any caller can start payment through a common contract.

**Architecture:** Keep the existing open order API as the merchant entry point. Add a neutral payment initiation package that accepts an order plus payment selection and returns a provider-independent payment result. Checkout is only one consumer of this layer: it exposes public order display and payer-facing start-payment endpoints, while future open APIs, admin tools, and retry jobs can call the same initiation service without depending on checkout UI code.

**Tech Stack:** Go, Gin, Ent, PostgreSQL, Redis, React 19, TanStack Router, Bun, shadcn/ui, i18n.

## Global Constraints

- Use the global response shape `{ code, message, data, error }`.
- Amounts remain integer minor units only.
- Do not implement automatic currency conversion or exchange rate logic.
- Keep `app_id` derived from auth context on open APIs; do not trust request JSON for app identity.
- Payment initiation must be channel-neutral and caller-neutral; do not put WeChat, Alipay, PayPal, or checkout-page-specific logic into order handlers.
- The first payment initiation implementation may use a mock provider, but the interface must support `pay_url`, `qr_code`, `form_html`, `provider_order_no`, `channel`, `pay_method`, and `status`.
- Frontend default language is Chinese.
- All frontend UI should use shadcn/ui components as the primary system.
- All frontend data displays must use the reusable Data Table factory in `web/src/components/data-table/`.
- After any backend code change, build the latest backend binary and restart the backend before handoff.
- Tests belong under each module’s `test/` directory.

---

### Task 1: Public Checkout Order Read API

**Files:**
- Create: `internal/domain/orders/handler/checkout_handler.go`
- Create: `internal/domain/orders/router/checkout_router.go`
- Modify: `internal/platform/http/router.go`
- Test: `internal/domain/orders/test/checkout_handler_test.go`

**Interfaces:**
- Consumes: `GET /v1/checkout/orders/:gateway_order_no`
- Produces: public checkout order payload for display only, with no secret fields.

- [ ] **Step 1: Write the failing handler test**

```go
func TestCheckoutHandlerReturnsPublicOrderView(t *testing.T) {
    // create an order via service
    // call GET /v1/checkout/orders/:gateway_order_no
    // assert code == "ok"
    // assert payload includes order, checkout title, amount, currency, status
    // assert payload excludes app secret or internal auth fields
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/domain/orders/test -run 'TestCheckoutHandlerReturnsPublicOrderView'`

Expected: compile failure because checkout handler and router do not exist yet.

- [ ] **Step 3: Implement the minimal handler and router**

```go
type CheckoutHandler struct {
    service ordersvc.Service
}

func NewCheckout(service ordersvc.Service) CheckoutHandler
func (h CheckoutHandler) GetOrder(ctx *gin.Context)
```

- [ ] **Step 4: Register the public checkout routes**

Register under `/v1/checkout` without app auth middleware:

```text
GET /v1/checkout/orders/:gateway_order_no
```

- [ ] **Step 5: Run the handler test and confirm it passes**

Run: `go test ./internal/domain/orders/test -run 'TestCheckoutHandlerReturnsPublicOrderView'`

Expected: PASS.

### Task 2: Neutral Payment Initiation Layer

**Files:**
- Create: `internal/domain/payments/service/service.go`
- Create: `internal/domain/payments/router/router.go`
- Create: `internal/domain/payments/handler/handler.go`
- Create: `internal/domain/payments/test/service_test.go`
- Modify: `internal/domain/orders/service/service.go` only if a read method is needed for order lookup.

**Interfaces:**
- Consumes: a payment order, selected `pay_method`, selected `channel`, `client_ip`, and `return_url`.
- Produces: `StartPayment(ctx context.Context, input StartPaymentInput) (*PaymentResult, error)`.

- [ ] **Step 1: Write failing service tests for payment initiation**

```go
func TestPaymentServiceStartsPaymentWithNeutralResult(t *testing.T) {
    // create a pending order
    // call payments.Service.StartPayment with pay_method=alipay and channel=alipay
    // assert result.status == "pending"
    // assert result.pay_method == "alipay"
    // assert result.channel == "alipay"
    // assert result.pay_url is non-empty for mock redirect-style payments
}

func TestPaymentServiceRejectsClosedOrder(t *testing.T) {
    // create or close an order
    // call StartPayment
    // assert ErrOrderNotPayable
}
```

- [ ] **Step 2: Run the tests and verify they fail**

Run: `go test ./internal/domain/payments/test -run 'TestPaymentService'`

Expected: compile or undefined-method failure before implementation.

- [ ] **Step 3: Implement service methods**

```go
type StartPaymentInput struct {
    Order          *ent.PaymentOrder
    PayMethod      string
    Channel        string
    ClientIP       string
    ReturnURL      string
}

type PaymentResult struct {
    Status          string `json:"status"`
    Channel         string `json:"channel"`
    PayMethod       string `json:"pay_method"`
    ProviderOrderNo string `json:"provider_order_no"`
    PayURL          string `json:"pay_url,omitempty"`
    QRCode          string `json:"qr_code,omitempty"`
    FormHTML        string `json:"form_html,omitempty"`
}
```

- [ ] **Step 4: Implement a mock provider behind the service**

The mock provider must be deterministic enough for tests and must not depend on checkout HTTP state. For example, it can build:

```text
pay_url = /checkout/mock-pay/:gateway_order_no?method=:pay_method
provider_order_no = mock_:gateway_order_no
```

- [ ] **Step 5: Run payment service tests**

Run: `go test ./internal/domain/payments/test -run 'TestPaymentService'`

Expected: PASS.

### Task 3: Checkout Payment Methods and Pay API

**Files:**
- Modify: `internal/domain/orders/handler/checkout_handler.go`
- Modify: `internal/domain/orders/router/checkout_router.go`
- Create: `internal/domain/orders/test/checkout_payment_test.go`
- Modify: `internal/platform/http/router.go`

**Interfaces:**
- Consumes: `payments.Service.StartPayment` from Task 2.
- Produces:
  - `GET /v1/checkout/orders/:gateway_order_no/methods`
  - `POST /v1/checkout/orders/:gateway_order_no/pay`

- [ ] **Step 1: Write failing checkout payment handler tests**

```go
func TestCheckoutHandlerListsPaymentMethods(t *testing.T) {
    // create a pending order
    // call GET /v1/checkout/orders/:gateway_order_no/methods
    // assert methods include alipay, wechat, paypal
}

func TestCheckoutHandlerStartsPaymentThroughPaymentService(t *testing.T) {
    // create a pending order
    // call POST /v1/checkout/orders/:gateway_order_no/pay with pay_method=alipay
    // assert payment result has status, channel, pay_method, provider_order_no, and pay_url
}
```

- [ ] **Step 2: Run the tests and verify they fail**

Run: `go test ./internal/domain/orders/test -run 'TestCheckoutHandler'`

Expected: undefined checkout methods or missing payment service wiring.

- [ ] **Step 3: Implement method list endpoint**

Return a static MVP method list:

```json
[
  { "pay_method": "alipay", "channel": "alipay", "label": "支付宝", "enabled": true },
  { "pay_method": "wechat", "channel": "wechat", "label": "微信支付", "enabled": true },
  { "pay_method": "paypal", "channel": "paypal", "label": "PayPal", "enabled": true }
]
```

- [ ] **Step 4: Implement checkout pay endpoint**

```text
POST /v1/checkout/orders/:gateway_order_no/pay
```

The handler must look up the order, reject non-payable statuses, then call `payments.Service.StartPayment`. It must not contain channel-specific SDK logic.

- [ ] **Step 5: Run service and handler tests**

Run:
- `go test ./internal/domain/payments/test -run 'TestPaymentService'`
- `go test ./internal/domain/orders/test -run 'TestCheckoutHandler'`

Expected: PASS.

### Task 4: Frontend Cashier Page

**Files:**
- Create: `web/src/features/checkout/*`
- Create: `web/src/routes/checkout.tsx`
- Modify: `web/src/router.tsx`
- Modify: `web/src/i18n/resources.ts`
- Modify: `web/src/lib/api.ts` if a small helper is needed

**Interfaces:**
- Consumes: checkout read endpoint, method list endpoint, and payment start endpoint.
- Produces: `/checkout/:gatewayOrderNo` page with method selection, pay action, and status display.

- [ ] **Step 1: Write the failing page test or route-level smoke test**

```tsx
// render /checkout/:gatewayOrderNo
// assert it shows order title, amount, method buttons, and pay action
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `bun test` or the project’s existing frontend test command if one exists.

- [ ] **Step 3: Implement the cashier page**

Use shadcn/ui components for:
- order summary card
- payment method buttons
- primary pay button
- status badge / alert
- loading and empty states

- [ ] **Step 4: Add Chinese copy keys**

Add new i18n keys under `checkout` in `web/src/i18n/resources.ts`.

- [ ] **Step 5: Wire the route**

Register the page in TanStack Router as a public route that does not render the admin console sidebar, topbar, or auth guard. The cashier page uses an independent layout because it is shown to payers, not administrators.

### Task 5: Verification, Build, And Restart

**Files:**
- Modify only files touched above.

**Interfaces:**
- Produces: verified backend binary and working checkout page route.

- [ ] **Step 1: Run full backend tests**

Run: `go test ./...`

- [ ] **Step 2: Build the latest backend binary**

Run: `go build -o ./server ./cmd/server`

- [ ] **Step 3: Restart backend from the new binary**

Start it with `.env` loaded and confirm the new process is serving requests.

- [ ] **Step 4: Verify endpoints**

Run:
- `curl -i http://127.0.0.1:8080/healthz`
- `curl -i http://127.0.0.1:8080/v1/ping`
- `curl -i http://127.0.0.1:8080/v1/checkout/orders/:gateway_order_no`

- [ ] **Step 5: Verify frontend manually**

Open `/checkout/:gateway_order_no` and confirm the cashier page renders in Chinese, shows method selection, and can trigger the mock payment flow.
