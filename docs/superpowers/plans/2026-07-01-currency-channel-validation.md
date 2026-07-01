# Currency-Channel Validation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enforce currency support by channel and support two order flows: merchant-specified payment method locks the order to direct payment, while unspecified orders let checkout select from compatible methods.

**Architecture:** Keep currency rules close to the payment domain. Add a small channel-capability layer that answers “which currencies does this channel support?” and use it in order creation, checkout method listing, and payment start. Orders with `channel/pay_method` are validated and locked at creation; orders without them defer selection to checkout. Default to built-in rules for now: `alipay`/`wechat` -> `CNY`, `paypal` -> `USD/EUR/HKD/JPY/GBP`.

**Tech Stack:** Go, Gin, Ent, existing payment/order/channel services, shadcn/ui frontend.

## Global Constraints

- Backend responses must keep the global `{ code, message, data, error }` shape.
- Money stays in integer minor units only.
- `alipay` and `wechat` stay `CNY` only unless a future cross-border mode is added.
- `paypal` remains multi-currency.
- If an order specifies `channel/pay_method`, payment is locked to that choice and must be validated.
- If an order specifies `channel/pay_method`, checkout must not show a channel picker; it should show the locked method and start that payment directly.
- If an order does not specify a channel, checkout may choose among enabled compatible methods only, and the selected method is written to the order when payment starts.
- For now `channel` and `pay_method` are treated as the same value for `alipay`, `wechat`, and `paypal`; future WeChat modes such as `native/h5/jsapi` should be modeled separately later.
- Tests live under each module’s `test/` directory.

---

### Task 1: Add channel currency capability helpers in backend service layer

**Files:**
- Modify: `internal/domain/payments/service/service.go`
- Create: `internal/domain/payments/test/currency_support_test.go`

**Interfaces:**
- Consumes: `StartPaymentInput.Order.Currency`, `StartPaymentInput.Channel`, `StartPaymentInput.PayMethod`
- Produces: helper used by order creation and checkout to validate currency compatibility

- [ ] **Step 1: Write the failing test**

```go
func TestChannelSupportsCurrency(t *testing.T) {
    cases := []struct {
        channel  string
        currency string
        want     bool
    }{
        {channel: "alipay", currency: "CNY", want: true},
        {channel: "alipay", currency: "USD", want: false},
        {channel: "wechat", currency: "CNY", want: true},
        {channel: "wechat", currency: "USD", want: false},
        {channel: "paypal", currency: "USD", want: true},
        {channel: "paypal", currency: "JPY", want: true},
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/payments/test -run TestChannelSupportsCurrency -count=1`
Expected: FAIL because helper does not exist yet.

- [ ] **Step 3: Write minimal implementation**

Add a small function in `payments/service` that returns allowed currencies per channel and defaults unknown channels to false.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/domain/payments/test -run TestChannelSupportsCurrency -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/domain/payments/service/service.go internal/domain/payments/test/currency_support_test.go
git commit -m "Add payment channel currency rules"
```

### Task 2: Reject unsupported currency/channel combinations when creating orders

**Files:**
- Modify: `internal/domain/orders/service/service.go`
- Modify: `internal/domain/orders/test/service_test.go`

**Interfaces:**
- Consumes: `ManageOrderInput.Channel`, `ManageOrderInput.PayMethod`, `ManageOrderInput.Currency`
- Produces: order creation error before persistence

- [ ] **Step 1: Write the failing test**

```go
func TestCreateOrderRejectsUnsupportedCurrencyForChannel(t *testing.T) {
    // arrange enabled app
    // create order with channel=alipay and currency=USD
    // expect error
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/orders/test -run TestCreateOrderRejectsUnsupportedCurrencyForChannel -count=1`
Expected: FAIL because the order currently persists.

- [ ] **Step 3: Write minimal implementation**

Validate the channel/currency pair in `CreateOrder` and `CreateOpenOrder` before repository writes. Return a dedicated error such as `ErrUnsupportedCurrencyForChannel`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/domain/orders/test -run TestCreateOrderRejectsUnsupportedCurrencyForChannel -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/domain/orders/service/service.go internal/domain/orders/test/service_test.go
git commit -m "Validate order currency against channel support"
```

### Task 3: Hide incompatible payment methods in checkout and block invalid starts

**Files:**
- Modify: `internal/domain/orders/handler/checkout_handler.go`
- Modify: `internal/domain/orders/service/service.go`
- Modify: `internal/domain/orders/repository/repository.go`
- Modify: `internal/domain/orders/test/checkout_payment_test.go`
- Modify: `internal/domain/orders/test/open_handler_test.go` if route coverage needs it

**Interfaces:**
- Consumes: persisted order currency, persisted order channel/pay method, enabled channel accounts, channel support helper
- Produces: checkout methods filtered by currency; locked orders expose only their locked method; `StartPayment` persists selected channel/pay method and rejects mismatches even if UI is stale

- [ ] **Step 1: Write the failing test**

```go
func TestCheckoutHandlerHidesUnsupportedCurrencyMethods(t *testing.T) {
    // order currency = USD
    // enabled alipay + paypal accounts exist
    // expect only paypal in methods response
}

func TestCheckoutHandlerLockedOrderShowsOnlyPersistedMethod(t *testing.T) {
    // order currency = CNY, channel/pay_method = alipay
    // enabled alipay + paypal accounts exist
    // expect only alipay in methods response
}

func TestCheckoutHandlerPersistsSelectedMethodWhenOrderHasNoMethod(t *testing.T) {
    // order has empty channel/pay_method
    // POST pay with channel/pay_method=paypal
    // expect persisted order channel/pay_method to become paypal
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/orders/test -run TestCheckoutHandlerHidesUnsupportedCurrencyMethods -count=1`
Expected: FAIL because methods are not filtered by currency yet.

- [ ] **Step 3: Write minimal implementation**

Filter `ListPaymentMethods` by supported currency. If `order.Channel` or `order.PayMethod` is already set, return only that locked method after validating it is enabled and currency-compatible. In `StartPayment`, use the locked method when present and reject mismatched requests. When the order is not locked, validate the selected method and persist `channel/pay_method` before calling the payment provider.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/domain/orders/test -run TestCheckoutHandlerHidesUnsupportedCurrencyMethods -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/domain/orders/handler/checkout_handler.go internal/domain/orders/test/checkout_payment_test.go
git commit -m "Filter checkout methods by currency"
```

### Task 4: Update frontend test-payment flow to stop implying every channel can take every currency

**Files:**
- Modify: `web/src/features/test-pay/test-pay-page.tsx`
- Modify: `web/src/i18n/resources.ts`

**Interfaces:**
- Consumes: selected app, selected channel, selected currency
- Produces: UI that prevents invalid combinations and explains why

- [ ] **Step 1: Write the failing test or manual verification note**

Add a minimal UI assertion or smoke check that selecting `alipay` while currency is `USD` shows a validation error / disables submit.

- [ ] **Step 2: Run verification to show it fails first**

Run: `cd web && bun run typecheck`
Expected: still passes before behavior change; the real failure should be captured by the new UI check if added.

- [ ] **Step 3: Write minimal implementation**

Auto-switch currency when channel changes only for preview convenience, but block submit when the chosen channel and currency are incompatible.

- [ ] **Step 4: Run verification to show it passes**

Run: `cd web && bun run typecheck && bun run lint`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/features/test-pay/test-pay-page.tsx web/src/i18n/resources.ts
git commit -m "Align test payment UI with channel currency support"
```

### Task 5: Rebuild and restart backend after backend changes

**Files:**
- No code changes; operational step only

**Interfaces:**
- Consumes: latest Go backend binary
- Produces: running server that serves the new validation rules

- [ ] **Step 1: Build latest backend**

Run: `go build -o .tmp/payment-gateway-server ./cmd/server`

- [ ] **Step 2: Restart server**

Stop the existing `payment-gateway` process on port `8080`, then start `.tmp/payment-gateway-server`.

- [ ] **Step 3: Verify health**

Run: `curl -sS http://127.0.0.1:8080/healthz`
Expected: `{"code":"ok",...}`
