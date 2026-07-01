# Webhook Delivery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build gateway-to-business webhook delivery for `payment.succeeded` events with DB outbox persistence, Redis Stream async dispatch, retry recovery, and admin visibility.

**Architecture:** Payment-platform callbacks update gateway orders first. When an order transitions to paid, the webhook domain creates a durable event and delivery record, enqueues the delivery id into Redis Stream, and an in-process worker delivers it to the app `notify_url`. The DB remains the source of truth; Redis only accelerates delivery and DB scanning restores missed jobs.

**Tech Stack:** Go, Gin, Ent, PostgreSQL, Redis Stream through `github.com/redis/go-redis/v9`, shadcn/ui, Bun.

## Global Constraints

- Do not confuse payment-platform notify URLs with business application webhook URLs.
- First phase supports `payment.succeeded` only.
- Webhook tests belong under `internal/domain/webhooks/test/`.
- All backend API responses use `internal/platform/httpx` response helpers, except provider-specific plain callback responses.
- After backend changes, build the latest backend binary and restart the running backend service.
- Frontend UI must use shadcn/ui components and Chinese i18n keys.

---

### Task 1: Durable Webhook Models

**Files:**
- Create: `ent/schema/webhook_event.go`
- Create: `ent/schema/webhook_delivery.go`
- Create: `internal/domain/webhooks/repository/repository.go`
- Test: `internal/domain/webhooks/test/service_test.go`

**Interfaces:**
- Produces: `webhooks.Service.RecordPaymentSucceeded(ctx, orderID int) (*RecordResult, error)`
- Produces: DB rows for `webhook_events` and `webhook_deliveries`

- [ ] **Step 1: Write failing tests**

Add tests proving:
- app with `notify_url` creates one event and one pending delivery;
- app without `notify_url` creates an event but no delivery;
- duplicate paid transition does not create duplicate event rows.

Run:

```bash
go test ./internal/domain/webhooks/test -run TestRecordPaymentSucceeded -count=1
```

Expected: fail because schema/service do not exist.

- [ ] **Step 2: Add Ent schemas**

Add `webhook_events` with `event_id`, `event_type`, `app_id`, `gateway_order_no`, `payload`, timestamps.
Add `webhook_deliveries` with `delivery_no`, `event_id`, `app_id`, `target_url`, `status`, `attempt_count`, `next_attempt_at`, `last_http_status`, `last_response_body`, `last_error`, timestamps.

- [ ] **Step 3: Generate Ent code**

Run:

```bash
make ent-up
```

- [ ] **Step 4: Implement repository and service**

Use existing domain layering. The service reads the order, app, and notify URL, builds PRD-compatible JSON payload, creates the event idempotently by `gateway_order_no + event_type`, and creates delivery only when `notify_url` is non-empty.

- [ ] **Step 5: Verify**

Run:

```bash
go test ./internal/domain/webhooks/test -run TestRecordPaymentSucceeded -count=1
go test ./...
```

### Task 2: Payment Success Integration

**Files:**
- Modify: `internal/domain/payments/service/service.go`
- Modify or create tests under: `internal/domain/payments/test/`

**Interfaces:**
- Consumes: `webhooks.Service.RecordPaymentSucceeded(ctx, orderID int)`
- Produces: payment notify success creates webhook event and delivery after order is paid.

- [ ] **Step 1: Write failing integration test**

Add a payment notify test that seeds an app with `notify_url`, sends a successful provider notify, and asserts a `payment.succeeded` webhook event and pending delivery exist.

Run:

```bash
go test ./internal/domain/payments/test -run TestNotifyCreatesWebhookDelivery -count=1
```

Expected: fail because payments does not call webhooks yet.

- [ ] **Step 2: Wire webhook service into payment service**

After successful idempotent paid update, call `RecordPaymentSucceeded`. Do not create events from raw provider payload before the order state is confirmed.

- [ ] **Step 3: Verify idempotency**

Run the notify test twice or add duplicate notify assertion. Expected: one event and one delivery.

### Task 3: Redis Stream Queue and Worker

**Files:**
- Create: `internal/domain/webhooks/queue/redis_stream.go`
- Create: `internal/domain/webhooks/worker/worker.go`
- Test: `internal/domain/webhooks/test/worker_test.go`

**Interfaces:**
- Produces: `Enqueuer.EnqueueDelivery(ctx, deliveryNo string) error`
- Produces: `Worker.ProcessOne(ctx, deliveryNo string) error`
- Consumes: pending `webhook_deliveries`

- [ ] **Step 1: Write failing worker tests**

Use `httptest.Server` for successful and failed business endpoints. Assert 2xx marks `succeeded`; non-2xx records status/error and schedules retry.

- [ ] **Step 2: Implement delivery client**

POST raw JSON payload with:

```text
X-Pay-Gateway-Event-Id
X-Pay-Gateway-Timestamp
X-Pay-Gateway-Signature
```

Signature is `HMAC-SHA256(app_secret, timestamp + "." + raw_body)`.

- [ ] **Step 3: Implement Redis Stream enqueue**

Use stream `webhook:deliveries`, group `payment-gateway-workers`, message field `delivery_no`.

- [ ] **Step 4: Implement recovery scan**

Scan DB for `pending` deliveries with `next_attempt_at <= now` and re-enqueue them.

- [ ] **Step 5: Verify**

Run:

```bash
go test ./internal/domain/webhooks/test -run 'TestWorker|TestRecovery' -count=1
go test ./...
```

### Task 4: Admin Webhook APIs

**Files:**
- Create: `internal/domain/webhooks/handler/handler.go`
- Create: `internal/domain/webhooks/router/router.go`
- Modify: `internal/platform/http/router.go`
- Test: `internal/domain/webhooks/test/handler_test.go`

**Interfaces:**
- Produces: `GET /v1/admin/webhook-deliveries`
- Produces: `GET /v1/admin/webhook-deliveries/:delivery_no`
- Produces: `POST /v1/admin/webhook-deliveries/:delivery_no/retry`

- [ ] **Step 1: Write failing handler tests**

Assert list returns `delivery_no`, event fields, status, target URL, attempt count, last response, and timestamps. Assert manual retry resets status to pending and enqueues the delivery.

- [ ] **Step 2: Implement handlers with `httpx`**

Use global response format. Do not expose app secrets or signing material.

- [ ] **Step 3: Register routes**

Register under authenticated admin route group.

- [ ] **Step 4: Verify**

Run:

```bash
go test ./internal/domain/webhooks/test -run TestWebhookAdmin -count=1
go test ./...
```

### Task 5: Frontend Webhook Center

**Files:**
- Create: `web/src/features/webhooks/api.ts`
- Create: `web/src/features/webhooks/types.ts`
- Create: `web/src/features/webhooks/webhooks-page.tsx`
- Create: `web/src/routes/webhooks.tsx`
- Modify: `web/src/router.tsx`
- Modify: `web/src/routes/root.tsx`
- Modify: `web/src/i18n/resources.ts`

**Interfaces:**
- Consumes: admin webhook delivery APIs.
- Produces: shadcn Data Table page for delivery records and manual retry.

- [ ] **Step 1: Add API/types**

Type delivery list/detail responses matching backend fields.

- [ ] **Step 2: Add page using Data Table factory**

Columns: event type, app id, gateway order no, target URL, status, attempts, last status, next attempt time, updated time, actions.

- [ ] **Step 3: Add manual retry action**

Use shadcn Button/Dialog/Toast patterns already present in the app.

- [ ] **Step 4: Add route and i18n**

Default Chinese labels and English fallback.

- [ ] **Step 5: Verify**

Run:

```bash
cd web && bun run typecheck && bun run lint
```

### Task 6: Final Verification and Runtime Restart

**Files:**
- Modify as needed only for integration wiring.

- [ ] **Step 1: Full backend verification**

Run:

```bash
go test ./...
go build -o .tmp/payment-gateway-server ./cmd/server
```

- [ ] **Step 2: Restart backend from latest binary**

Stop stale backend processes and start `.tmp/payment-gateway-server` with `.env`.

- [ ] **Step 3: Frontend verification**

Run:

```bash
cd web && bun run typecheck && bun run lint
```

- [ ] **Step 4: Smoke test**

Check `/healthz` and manually trigger a test payment path that creates a pending webhook delivery.
