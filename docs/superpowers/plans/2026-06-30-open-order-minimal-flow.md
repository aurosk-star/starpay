# Open Order Minimal Flow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the smallest runnable external payment order flow: signed app request creates an order, queries it, and closes it.

**Architecture:** Reuse the existing `orders` domain service for validation and persistence. Add an open-facing HTTP handler/router under `/v1/open/orders` behind `AppAuthMiddleware`; the handler must derive `app_id` from auth context, not from request JSON, so one application can only operate on its own orders. No currency module, real channel invocation, routing engine, webhook delivery, or provider payment URL is included in this phase.

**Tech Stack:** Go, Gin, Ent, PostgreSQL, Redis replay protection, existing HMAC-SHA256 app auth.

## Global Constraints

- Use global response shape `{ code, message, data, error }`.
- Signed open API requests must include `app_id`, `request_id`, `timestamp`, `nonce`, and `sign`.
- Use `request_id` for replay protection and context tracing.
- Amounts remain integer minor units.
- Do not implement automatic currency conversion or exchange rate logic.
- Do not call WeChat, Alipay, PayPal, or `go-pay` in this minimal flow.
- After backend changes, run `go test ./...`, build `./server`, and restart the backend.

---

### Task 1: Order Service Open-API Interfaces

**Files:**
- Modify: `internal/domain/orders/service/service.go`
- Modify: `internal/domain/orders/repository/repository.go`
- Test: `internal/domain/orders/test/service_test.go`

**Interfaces:**
- Consumes: authenticated `app_id`, `merchant_order_no`, `amount`, `currency`, `subject`, `description`, `business_type`, `pay_method`, `preferred_channel`, `client_ip`, `return_url`, and `metadata`.
- Produces: `CreateOpenOrder`, `FindOrderByGatewayOrderNoForApp`, `FindOrderByMerchantOrderNoForApp`, and `CloseOrderForApp`.

- [ ] **Step 1: Write failing tests for app-scoped lookup and close**

Add tests proving:
- querying by gateway order number returns only orders for the authenticated app;
- querying by merchant order number returns only orders for the authenticated app;
- closing an order from another app is rejected;
- closing a `paid` order is rejected.

Run: `go test ./internal/domain/orders/test -run 'TestOpenOrder'`

Expected: compile failure because open-order service methods do not exist.

- [ ] **Step 2: Implement repository lookups**

Add repository methods:
- `FindByGatewayOrderNoForApp(ctx context.Context, appID string, gatewayOrderNo string) (*ent.PaymentOrder, error)`
- `FindByMerchantOrderNo(ctx context.Context, appID string, merchantOrderNo string) (*ent.PaymentOrder, error)` already exists and should be reused.

- [ ] **Step 3: Implement service methods**

Add service methods:
- `CreateOpenOrder(ctx context.Context, appID string, input OpenOrderInput) (*ent.PaymentOrder, bool, error)`
- `FindOrderByGatewayOrderNoForApp(ctx context.Context, appID string, gatewayOrderNo string) (*ent.PaymentOrder, error)`
- `FindOrderByMerchantOrderNoForApp(ctx context.Context, appID string, merchantOrderNo string) (*ent.PaymentOrder, error)`
- `CloseOrderForApp(ctx context.Context, appID string, gatewayOrderNo string) (*ent.PaymentOrder, error)`

The `bool` return from `CreateOpenOrder` means `created`: `true` for new order, `false` for idempotent existing order.

- [ ] **Step 4: Preserve idempotency rules**

If `app_id + merchant_order_no` already exists:
- same `amount`, `currency`, `subject`, `business_type`, `pay_method`, `channel` returns existing order with `created=false`;
- different values returns an idempotency conflict error.

- [ ] **Step 5: Run service tests**

Run: `go test ./internal/domain/orders/test`

Expected: all order service tests pass.

### Task 2: Open Orders Handler And Router

**Files:**
- Create: `internal/domain/orders/handler/open_handler.go`
- Create: `internal/domain/orders/router/open_router.go`
- Modify: `internal/platform/http/router.go`
- Test: `internal/domain/orders/test/open_handler_test.go`

**Interfaces:**
- Consumes: `httpx.ContextAppID` from `AppAuthMiddleware`.
- Produces:
  - `POST /v1/open/orders`
  - `GET /v1/open/orders/:gateway_order_no`
  - `GET /v1/open/orders/by-merchant/:merchant_order_no`
  - `POST /v1/open/orders/:gateway_order_no/close`

- [ ] **Step 1: Write failing handler tests**

Use a Gin router with `ctx.Set(httpx.ContextAppID, "snsgo")` test middleware to avoid cryptographic setup in handler tests.

Cover:
- create order ignores any JSON `app_id` and uses context app ID;
- get order returns 404 for another app’s order;
- close order returns global response shape;
- create response includes `created` and `order`.

Run: `go test ./internal/domain/orders/test -run 'TestOpenOrderHandler'`

Expected: compile failure because open handler/router do not exist.

- [ ] **Step 2: Implement open request/response structs**

Create request type:
- `merchant_order_no`
- `amount`
- `currency`
- `pay_method`
- `preferred_channel`
- `subject`
- `description`
- `business_type`
- `client_ip`
- `return_url`
- `metadata`

Response should include:
- `order`
- `created`
- `payment`: `{ "status": "pending" }`

Do not return fake `pay_url` or `qr_code` yet.

- [ ] **Step 3: Implement handlers**

Handlers must:
- read app ID from `ctx.GetString(httpx.ContextAppID)`;
- reject missing app context with `401`;
- use `httpx.JSONOK` and `httpx.JSONError`;
- map idempotency conflicts to `409`.

- [ ] **Step 4: Register open router**

In `internal/platform/http/router.go`, replace the temporary `/v1/open/ping`-only group with a group that keeps `/ping` and registers open order endpoints behind the same `AppAuthMiddleware`.

- [ ] **Step 5: Run handler tests**

Run: `go test ./internal/domain/orders/test -run 'TestOpenOrderHandler'`

Expected: pass.

### Task 3: Signed Integration Test Path

**Files:**
- Modify: `internal/platform/http/router_test.go`
- Test: `internal/platform/http/router_test.go`

**Interfaces:**
- Consumes: app auth middleware, app secret ciphertext, Redis replay store abstraction.
- Produces: one request-level test proving signed open order creation reaches the order handler.

- [ ] **Step 1: Add a router-level signed request test**

Seed an app with a known encrypted secret and issue a signed `POST /v1/open/orders` request with:
- query auth params;
- JSON body order payload;
- unique `request_id` and `nonce`.

Expected response:
- HTTP `201` for new order or `200` if the implementation standardizes on OK;
- body has `code=ok`;
- `data.order.app_id` matches signed `app_id`;
- `data.created=true`.

- [ ] **Step 2: Verify replay protection still works**

Send the same signed request again with the same `request_id` and `nonce`.

Expected: `401` with `replayed_request`.

- [ ] **Step 3: Run router tests**

Run: `go test ./internal/platform/http -run 'TestOpenOrder'`

Expected: pass.

### Task 4: Verification, Rebuild, And Restart

**Files:**
- Modify only files touched by Tasks 1-3.

**Interfaces:**
- Produces: fresh backend binary and running local backend.

- [ ] **Step 1: Run full backend tests**

Run: `go test ./...`

Expected: all packages pass.

- [ ] **Step 2: Build latest backend binary**

Run: `go build -o ./server ./cmd/server`

Expected: exit code 0.

- [ ] **Step 3: Restart backend**

Stop existing `./server` process and start the freshly built binary with `.env`.

Expected: new process is visible via `pgrep -af '^./server$|/payment-gateway/server$'`.

- [ ] **Step 4: Verify basic endpoints**

Run:
- `curl -i http://127.0.0.1:8080/healthz`
- `curl -i http://127.0.0.1:8080/v1/ping`

Expected: both return HTTP 200 with global response shape.

- [ ] **Step 5: Provide manual signed curl example**

Document a shell snippet that signs and calls:
- `POST /v1/open/orders`
- `GET /v1/open/orders/:gateway_order_no`
- `POST /v1/open/orders/:gateway_order_no/close`

Use the existing canonical signing format from `httpx.CanonicalAppSignString`.
