# Checkout Order Read Reconciliation Implementation Plan

> **For agentic workers:** Execute this plan inline with test-driven development. Steps use checkbox syntax for tracking.

**Goal:** Trigger asynchronous payment reconciliation through the existing checkout order GET request.

**Architecture:** After checkout-token authorization, `GetOrder` requests reconciliation for orders that have a provider order number and then returns the local order normally. The reconciliation repository atomically sets `active_query_requested_at` and changes the attempt to due-now, preventing repeated GET requests from resetting retry history, bypassing backoff, or duplicating queue messages.

**Tech Stack:** Go, Gin, Ent, Redis Streams, React, TypeScript.

## Global Constraints

- API responses retain the existing `{ code, message, data, error }` shape.
- Checkout order reads retain existing checkout-token authorization.
- Provider queries remain asynchronous in the reconciliation worker.
- Queue failures do not block order reads.

### Task 1: Add idempotent immediate reconciliation

**Files:**
- Modify: `internal/domain/reconciliations/repository/repository.go`
- Modify: `internal/domain/reconciliations/service/service.go`
- Test: `internal/domain/reconciliations/test/service_test.go`

- [x] Test that the first request moves a future attempt to now and enqueues it.
- [x] Test that repeated requests do not enqueue again or reset attempt counts.
- [x] Test that terminal orders are no-ops.
- [x] Test already-due records and enqueue failure recovery.
- [x] Implement the atomic active-query marker and conditional enqueue.

### Task 2: Trigger reconciliation from checkout GET

**Files:**
- Modify: `internal/domain/orders/handler/checkout_handler.go`
- Test: `internal/domain/orders/test/checkout_handler_test.go`

- [x] Test that authenticated GET requests reconciliation after payment starts.
- [x] Test that GET skips reconciliation before a provider order exists.
- [x] Invoke the scheduler best-effort and keep the existing response behavior.

### Task 3: Remove redundant frontend request

**Files:**
- Modify: `web/src/features/checkout/api.ts`
- Modify: `web/src/features/checkout/checkout-result-page.tsx`

- [x] Remove the dedicated reconciliation POST client.
- [x] Keep the existing result-page GET as the only request needed to trigger reconciliation.

### Task 4: Verify

- [x] Run `make test`.
- [x] Run `make web-typecheck`.
- [x] Run `make web-build`.
- [x] Build `bin/payment-gateway` from `./cmd/server`.
