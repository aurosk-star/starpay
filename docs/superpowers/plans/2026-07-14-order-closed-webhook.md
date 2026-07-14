# Order Closed Webhook Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Emit an idempotent `order.closed` Webhook for administrator and merchant initiated order closure while preserving `order.expired` for automatic timeout closure.

**Architecture:** Extend the existing Webhook resource-event service with an `order.closed` payload carrying a close source. Route intentional order closure through the same Ent transaction/outbox pattern already used by expiration closure, while keeping provider-status synchronization on a no-event close path. Reuse the existing delivery worker, retry policy, signatures, admin APIs, and frontend event filter.

**Tech Stack:** Go, Ent, Gin, Redis Streams, React, TypeScript, i18next.

## Global Constraints

- Admin close emits `close_source=admin`.
- Authenticated open API close emits `close_source=merchant`.
- Timeout close emits only `order.expired`.
- Provider-reported `closed` does not emit an intentional-close event.
- Event persistence is atomic with the intentional order transition.

---

### Task 1: Add the Webhook event contract

**Files:**
- Modify: `internal/domain/webhooks/service/service.go`
- Test: `internal/domain/webhooks/test/service_test.go`

**Interfaces:**
- Produces: `EventOrderClosed = "order.closed"`
- Produces: `Service.RecordOrderClosed(context.Context, *ent.PaymentOrder, string) (*ent.WebhookEvent, error)`

- [ ] Add a failing test that records an admin close and asserts one `order.closed` event and one pending delivery.
- [ ] Add a failing idempotency assertion that a repeated record call reuses the same event.
- [ ] Run `go test ./internal/domain/webhooks/test -run RecordOrderClosed -count=1` and confirm the missing contract fails.
- [ ] Add the event constant, recorder, and payload fields from the approved design.
- [ ] Re-run the focused test and confirm it passes.

### Task 2: Emit events from intentional close paths

**Files:**
- Modify: `internal/domain/orders/service/service.go`
- Test: `internal/domain/orders/test/service_test.go`

**Interfaces:**
- Consumes: `RecordOrderClosed(ctx, order, closeSource)` from Task 1.
- Produces: existing `CloseOrder` with admin semantics and existing `CloseOrderForApp` with merchant semantics.
- Keeps: provider-result `closed` synchronization on an internal no-event close path.

- [ ] Add failing service tests for admin and merchant close sources, duplicate prevention, expiration separation, and rollback on event persistence failure.
- [ ] Run the focused order tests and confirm they fail because no intentional-close event exists.
- [ ] Implement a transactional intentional-close helper using `tx.Client()` repositories and the Webhook transactional clone.
- [ ] Make `CloseOrder` use source `admin` and `CloseOrderForApp` use source `merchant`.
- [ ] Make `ApplyPaymentResult(status=closed)` call a no-event conditional close helper.
- [ ] Run order, payment, reconciliation, and Webhook tests.

### Task 3: Expose the event in Webhook Center

**Files:**
- Modify: `web/src/features/webhooks/event-types.ts`
- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/zh-CN.json`
- Test: `web/test/webhook-event-types.test.mts`

**Interfaces:**
- Produces: `order.closed` in `supportedWebhookEventTypes` and localized formatting.

- [ ] Update the event-list test first to expect `order.closed` and run it to confirm failure.
- [ ] Add `order.closed` to the supported/known event list.
- [ ] Add English `Order closed` and Chinese `订单主动关闭` descriptions.
- [ ] Run frontend tests and TypeScript checks.

### Task 4: Verify and run the latest backend

**Files:**
- No source changes expected.

- [ ] Run `make verify`.
- [ ] Run focused Go race tests for orders and Webhooks.
- [ ] Build the embedded-web backend binary from the current source.
- [ ] Restart `payment-gateway-dev.service` from the new binary.
- [ ] Confirm local/public health, binary hash equality, and a real admin-close delivery record.
