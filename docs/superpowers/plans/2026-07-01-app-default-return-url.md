# App Default Return URL Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every payment channel redirect to `order.return_url`, falling back to the App default return URL and then a checkout fallback page.

**Architecture:** Resolve the final browser return URL when orders are created, store it on `payment_orders.return_url`, and make channel-specific flows use that stored value. PayPal still returns to the gateway for capture first, then the gateway redirects to the stored final return URL.

**Tech Stack:** Go, Gin, Ent, existing domain modules, React checkout page.

## Global Constraints

- All backend responses continue using `internal/platform/httpx`.
- Tests for order behavior live under `internal/domain/orders/test/`.
- `notify_url` is only for asynchronous payment notifications and must not be used as the browser return URL.
- After backend changes, build `./server` and restart the running backend.

---

### Task 1: Preserve App Default Return URL as Final Order Return URL

**Files:**
- Modify: `internal/domain/orders/service/service.go`
- Test: `internal/domain/orders/test/return_url_test.go`

**Interfaces:**
- Consumes: `ordersvc.Service.CreateOpenOrder(ctx, appID, OpenOrderInput)`
- Produces: orders with `ReturnURL` set to request `return_url` or `app.default_return_url`

- [ ] **Step 1: Confirm existing tests cover fallback**

Run: `go test ./internal/domain/orders/test -run 'TestOpenOrderStoresRequestReturnURL|TestOpenOrderUsesAppDefaultReturnURLWhenRequestOmitsIt'`

Expected: PASS. If it fails, fix `CreateOpenOrder` so request `ReturnURL` wins and App `DefaultReturnURL` is the fallback.

### Task 2: PayPal Final Redirect Uses Stored Return URL with Checkout Fallback

**Files:**
- Modify: `internal/domain/orders/handler/checkout_handler.go`
- Test: `internal/domain/orders/test/checkout_payment_test.go`

**Interfaces:**
- Consumes: `CheckoutHandler.CompletePaypalPayment`
- Produces: browser redirect target chosen by `order.ReturnURL`, else PayPal callback query `return_url`, else `/checkout/{gateway_order_no}`

- [ ] **Step 1: Write/update tests**

Ensure tests assert:
- PayPal provider gets a gateway return URL, not the notify URL.
- PayPal final redirect uses stored `order.ReturnURL` when present.
- PayPal final redirect uses callback `return_url` only when stored order return URL is empty.

- [ ] **Step 2: Implement minimal redirect selection**

Use `paypalFinalReturnURL(ctx, order.ReturnURL)` inside `CompletePaypalPayment` for cancel and success branches.

- [ ] **Step 3: Run targeted tests**

Run: `go test ./internal/domain/orders/test -run 'ReturnURL|PaypalReturn'`

Expected: PASS.

### Task 3: Frontend Checkout Sends Fallback Page Only

**Files:**
- Modify: `web/src/features/checkout/checkout-page.tsx`

**Interfaces:**
- Consumes: `startCheckoutPayment(gatewayOrderNo, payload)`
- Produces: `payload.return_url = window.location.href` as a fallback for orders without App return URL

- [ ] **Step 1: Keep checkout fallback payload**

Confirm `handlePay` sends:

```tsx
return_url: window.location.href,
```

- [ ] **Step 2: Run frontend typecheck**

Run: `cd web && bun run typecheck`

Expected: PASS.

### Task 4: Verify and Restart

**Files:**
- Modify: none

**Interfaces:**
- Consumes: completed backend/frontend changes
- Produces: latest backend binary running on `:8080`

- [ ] **Step 1: Run full backend tests**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 2: Build backend**

Run: `go build -o ./server ./cmd/server`

Expected: exit 0.

- [ ] **Step 3: Build frontend**

Run: `cd web && bun run build`

Expected: PASS.

- [ ] **Step 4: Restart backend**

Find PID: `lsof -iTCP:8080 -sTCP:LISTEN -n -P`

Kill old `server` PID, then run:

```bash
set -a && source .env && set +a && ./server
```

Confirm `server` listens on `:8080`.
