# Checkout Flow Consolidation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make checkout behavior match the business contract: merchant-specified payment methods are locked and paid directly; unspecified orders let checkout choose from compatible methods.

**Architecture:** Keep order creation responsible for locking payment method when provided, and checkout responsible for presenting either a locked payment action or a selectable method list. Backend remains authoritative: it validates locked methods, compatible currencies, enabled channels, and stale frontend requests. Frontend uses backend order/method response to choose its UI state instead of inferring business rules locally.

**Tech Stack:** Go, Gin, Ent, existing order/payment/channel services, React + TanStack Router, shadcn/ui.

## Global Constraints

- Backend responses must keep `{ code, message, data, error }` except provider callback bodies that require platform-specific text.
- `channel` and `pay_method` are equivalent for `alipay`, `wechat`, and `paypal` in the current phase.
- `alipay` and `wechat` only support `CNY`; `paypal` supports `USD/EUR/HKD/JPY/GBP`.
- Locked orders must not expose a channel picker in checkout.
- Unlocked orders must persist selected `channel/pay_method` when payment starts.
- After backend code changes, build the latest backend binary and restart the running backend service.
- Tests live under each module's `test/` directory.

---

### Task 1: Make checkout method API explicitly expose locked state

**Files:**
- Modify: `internal/domain/orders/handler/checkout_handler.go`
- Modify: `internal/domain/orders/test/checkout_payment_test.go`
- Modify: `web/src/features/checkout/types.ts`

**Interfaces:**
- Consumes: checkout order `channel`, `pay_method`, `currency`, enabled channel accounts
- Produces: `GET /v1/checkout/orders/:gateway_order_no/methods` response with `locked: boolean`, `selected_method?: { channel, pay_method, label, enabled }`, and compatible `methods`

- [ ] **Step 1: Write failing backend test**

Add a test in `internal/domain/orders/test/checkout_payment_test.go`:

```go
func TestCheckoutHandlerReturnsLockedMethodState(t *testing.T) {
    router, created, _, channelService := newCheckoutPaymentTestRouter(t, "checkout_methods_locked_state")
    _, _ = channelService.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
        Channel: "alipay", Name: "支付宝沙箱", Enabled: true, Env: "sandbox", Config: map[string]any{"app_id": "app-1"},
    })

    recorder := httptest.NewRecorder()
    router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/orders/"+created.GatewayOrderNo+"/methods", nil))

    if recorder.Code != http.StatusOK { t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String()) }
    var response map[string]any
    if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil { t.Fatalf("decode response: %v", err) }
    data := response["data"].(map[string]any)
    if data["locked"] != true { t.Fatalf("locked = %#v, want true", data["locked"]) }
    selected := data["selected_method"].(map[string]any)
    if selected["channel"] != "alipay" || selected["pay_method"] != "alipay" { t.Fatalf("selected = %#v, want alipay", selected) }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/orders/test -run TestCheckoutHandlerReturnsLockedMethodState -count=1`
Expected: FAIL because `locked` and `selected_method` are missing.

- [ ] **Step 3: Implement backend response shape**

In `ListPaymentMethods`, compute:

```go
lockedChannel := strings.TrimSpace(order.Channel)
lockedPayMethod := strings.TrimSpace(order.PayMethod)
locked := lockedChannel != "" || lockedPayMethod != ""
```

Return `locked`, `selected_method`, and `methods`. For locked orders, `methods` should contain only the locked method when enabled and currency-compatible; otherwise return an empty list and `selected_method.enabled=false`.

- [ ] **Step 4: Update frontend types**

In `web/src/features/checkout/types.ts`, add:

```ts
export type CheckoutMethodsResponse = {
  locked: boolean;
  selected_method?: CheckoutPaymentMethod;
  methods: CheckoutPaymentMethod[];
};
```

- [ ] **Step 5: Verify**

Run: `go test ./internal/domain/orders/test -run TestCheckoutHandlerReturnsLockedMethodState -count=1`
Expected: PASS

### Task 2: Enforce locked method on payment start

**Files:**
- Modify: `internal/domain/orders/handler/checkout_handler.go`
- Modify: `internal/domain/orders/test/checkout_payment_test.go`

**Interfaces:**
- Consumes: locked order `channel/pay_method`, request body `channel/pay_method`
- Produces: locked order ignores matching omissions and rejects mismatched selections

- [ ] **Step 1: Write failing tests**

Add tests:

```go
func TestCheckoutHandlerUsesLockedMethodWhenRequestOmitsMethod(t *testing.T) {
    router, created, _, channelService := newCheckoutPaymentTestRouter(t, "checkout_locked_pay_empty_request")
    _, _ = channelService.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
        Channel: "alipay", Name: "支付宝沙箱", Enabled: true, Env: "sandbox", Config: map[string]any{"app_id": "app-1"},
    })

    recorder := httptest.NewRecorder()
    router.ServeHTTP(recorder, jsonRequest(http.MethodPost, "/orders/"+created.GatewayOrderNo+"/pay", map[string]any{}))

    if recorder.Code != http.StatusOK { t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String()) }
}

func TestCheckoutHandlerRejectsMismatchedMethodForLockedOrder(t *testing.T) {
    router, created, _, channelService := newCheckoutPaymentTestRouter(t, "checkout_locked_pay_mismatch")
    _, _ = channelService.CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
        Channel: "alipay", Name: "支付宝沙箱", Enabled: true, Env: "sandbox", Config: map[string]any{"app_id": "app-1"},
    })

    recorder := httptest.NewRecorder()
    router.ServeHTTP(recorder, jsonRequest(http.MethodPost, "/orders/"+created.GatewayOrderNo+"/pay", map[string]any{
        "pay_method": "paypal",
        "channel": "paypal",
    }))

    if recorder.Code != http.StatusBadRequest { t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String()) }
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/domain/orders/test -run 'TestCheckoutHandler(UsesLockedMethodWhenRequestOmitsMethod|RejectsMismatchedMethodForLockedOrder)' -count=1`
Expected: at least one FAIL because current selection logic accepts request values first.

- [ ] **Step 3: Implement locked enforcement**

In `StartPayment`, if order has `Channel` or `PayMethod`, set final `channel/payMethod` from the order. If request provides a different non-empty value, return `400 locked_payment_method_mismatch`.

- [ ] **Step 4: Verify**

Run the same targeted test command.
Expected: PASS

### Task 3: Update checkout UI for locked and unlocked flows

**Files:**
- Modify: `web/src/features/checkout/checkout-page.tsx`
- Modify: `web/src/features/checkout/types.ts`
- Modify: `web/src/features/checkout/api.ts`
- Modify: `web/src/i18n/resources.ts`

**Interfaces:**
- Consumes: `locked`, `selected_method`, `methods`, order status
- Produces: locked UI with direct pay button; unlocked UI with method selector

- [ ] **Step 1: Update UI state handling**

Use `methodResult.locked` and `methodResult.selected_method` from API response. For locked orders, set selected method from `selected_method` and do not render multiple method buttons.

- [ ] **Step 2: Locked UI rendering**

Show a small locked method summary using existing shadcn components:

```tsx
{methodsLocked && selectedMethod ? (
  <div className="rounded-md border p-3 text-sm">
    <p className="font-medium">{selectedMethod.label}</p>
    <p className="text-muted-foreground">{t("checkout.lockedMethodHint")}</p>
  </div>
) : methodButtons}
```

- [ ] **Step 3: Submit payload**

For locked orders, allow `startCheckoutPayment` to be called without `channel/pay_method`; backend will use the locked order method. For unlocked orders, send selected method.

Update `StartCheckoutPaymentPayload` so `pay_method` and `channel` are optional:

```ts
export type StartCheckoutPaymentPayload = {
  pay_method?: string;
  channel?: string;
  return_url?: string;
};
```

- [ ] **Step 4: Add i18n keys**

Add Chinese and English keys:

```ts
lockedMethodHint: "该订单已由业务方指定支付方式。"
lockedMethodUnavailable: "指定支付方式当前不可用。"
```

- [ ] **Step 5: Verify frontend**

Run: `cd web && bun run typecheck && bun run lint`
Expected: PASS

### Task 4: Manual payment-link verification

**Files:**
- No code files; use running backend/frontend

**Interfaces:**
- Consumes: admin test payment page and checkout page
- Produces: verified behavior for both order modes

- [ ] **Step 1: Verify unlocked order**

On `/test-pay`, keep “指定支付方式” off, create a `USD` order. Checkout should show PayPal only if enabled.

- [ ] **Step 2: Verify locked order**

On `/test-pay`, enable “指定支付方式”, choose PayPal + USD, create order. Checkout should show PayPal locked summary and direct pay button.

- [ ] **Step 3: Verify invalid locked order**

On `/test-pay`, enable “指定支付方式”, choose Alipay + USD. The page should block submit or backend should reject with unsupported currency.

### Task 5: Build and restart backend

**Files:**
- No source changes in this task

**Interfaces:**
- Consumes: latest backend code
- Produces: running backend on `:8080`

- [ ] **Step 1: Run full verification**

Run:

```bash
go test ./...
cd web && bun run typecheck && bun run lint
```

- [ ] **Step 2: Build backend**

Run: `go build -o .tmp/payment-gateway-server ./cmd/server`

- [ ] **Step 3: Restart backend**

Find process on port 8080 with `ss -ltnp 'sport = :8080'`, stop it, then start `.tmp/payment-gateway-server`.

- [ ] **Step 4: Health check**

Run: `curl -sS http://127.0.0.1:8080/healthz`
Expected: response contains `"code":"ok"`.
