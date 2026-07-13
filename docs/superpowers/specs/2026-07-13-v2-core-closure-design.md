# V2 Core Payment Closure Design

## Goal

Complete the V2 payment lifecycle requirements that remain after the existing payment callback hardening:

- actively reconcile provider payment state when callbacks are delayed or lost;
- support idempotent partial and full refunds across Alipay, WeChat Pay, and PayPal;
- make webhook event idempotency resource-aware;
- expose reconciliation and refund failures to administrators;
- make root verification cover backend, SDK, and frontend checks.

Stripe, subscriptions, ledger accounting, settlement files, advanced routing, and channel-secret encryption are outside this change.

## Architecture Decision

Use durable Ent records for reconciliation and refunds, with Redis Streams for asynchronous execution. Reuse the current order-expiration and webhook-worker patterns.

The active payment implementation remains `internal/domain/payments/provider`. The unused `internal/domain/channels/service.Adapter` abstraction is not extended and is not wired into the new flow.

This design was selected over storing reconciliation state directly on payment orders because separate records provide an audit trail and can later grow into provider reconciliation and settlement workflows.

## Payment Reconciliation Model

Add `ent/schema/payment_reconciliation.go` with one reconciliation record per payment order.

Fields:

- `payment_order_id`: unique local order ID.
- `gateway_order_no`: denormalized gateway order number.
- `channel`: normalized channel name.
- `channel_account_id`: the account bound to the order.
- `status`: `pending`, `processing`, `resolved`, or `manual_required`.
- `attempt_count`: completed provider query attempts.
- `next_attempt_at`: next eligible scan time.
- `last_attempt_at`: most recent claimed execution time.
- `last_provider_status`: most recent normalized provider status.
- `last_error`: most recent query or state-application error.
- `provider_snapshot`: normalized raw provider response for diagnosis.
- `resolved_at`: terminal resolution time.
- standard `created_at` and `updated_at` timestamps.

Indexes cover `payment_order_id`, `gateway_order_no`, `(status, next_attempt_at)`, `channel`, and `updated_at`.

## Payment Provider Contracts

Extend `internal/domain/payments/provider` with capability interfaces instead of adding methods to the base `Provider` interface:

```go
type QueryProvider interface {
    Provider
    QueryPayment(context.Context, QueryPaymentRequest) (*QueryPaymentResult, error)
}

type CloseProvider interface {
    Provider
    ClosePayment(context.Context, ClosePaymentRequest) error
}

type RefundProvider interface {
    Provider
    CreateRefund(context.Context, CreateRefundRequest) (*RefundResult, error)
}

type RefundQueryProvider interface {
    Provider
    QueryRefund(context.Context, QueryRefundRequest) (*RefundResult, error)
}
```

Every request carries the bound channel account and the local order or refund identity. Query results contain provider order IDs, provider trade IDs, normalized status, amount, currency, failure reason, and raw response.

Before applying a successful payment query result, the service validates channel, channel account, gateway order number, amount, and currency using the same rules as provider callbacks.

Provider mappings:

- Alipay uses `alipay.trade.query`, `alipay.trade.close`, `alipay.trade.refund`, and `alipay.trade.fastpay.refund.query`.
- WeChat Pay API v3 uses transaction query, close order, refund, and refund query APIs.
- PayPal uses order detail, capture refund, and refund detail APIs. PayPal has no equivalent order-cancel operation in the implemented Orders flow, so an expired uncaptured order is closed locally and capture remains rejected by the local terminal state.

## Reconciliation Lifecycle

A reconciliation record is created after payment initiation has persisted the selected channel account and provider order ID. The first query is scheduled two minutes later or at order expiry, whichever comes first.

The scanner also creates missing reconciliation records for pre-upgrade pending orders that already have a channel account and provider order binding. This makes deployment compatible with in-flight payments.

The scanner runs every 30 seconds and enqueues up to 100 due records to Redis stream `payment:reconciliations`. Duplicate messages are allowed. A worker claims work with a conditional `pending -> processing` update; messages that cannot claim the record are acknowledged without provider calls.

Processing records older than five minutes are considered abandoned and are returned to `pending` by the scanner.

Query outcomes:

- `paid`: apply the verified payment result, create or restore `payment.succeeded`, then mark reconciliation `resolved`.
- `failed`: apply the verified failure result, create or restore `payment.failed`, then mark reconciliation `resolved`.
- `closed`: close the pending order and mark reconciliation `resolved`.
- `pending` before expiry: schedule another query.
- `pending` after expiry: call provider close when supported, close locally after successful close, and resolve.
- provider or validation error: record the error and retry without changing the payment order.

Retry delays are 2, 5, 10, 30, 60, 120, 360, and 1,440 minutes. After eight unsuccessful attempts, the record enters `manual_required`. An administrator can reset it to `pending` for immediate retry.

Provider-bound pending orders are not blindly closed by the existing expiration worker. Their terminal decision moves to reconciliation. Orders without a provider binding keep the current local expiration behavior.

## Refund Model

Add `ent/schema/refund.go` and a complete `internal/domain/refunds` domain with repository, service, handler, router, worker, and external tests.

Fields:

- `refund_no`: unique gateway refund number.
- `app_id` and `merchant_refund_no`: unique together for merchant idempotency.
- `payment_order_id`, `gateway_order_no`, and `merchant_order_no`.
- `channel`, `channel_account_id`, `provider_order_no`, and `channel_trade_no` copied from the paid order.
- `channel_refund_no`: provider refund identifier when known.
- `amount` and `currency` in integer minor units.
- `reason` and `metadata`.
- `status`: `pending`, `succeeded`, `failed`, or `closed`.
- `failure_reason` and `provider_snapshot`.
- `attempt_count`, `next_attempt_at`, `last_attempt_at`, and `last_error` for recovery.
- `succeeded_at`, `failed_at`, `closed_at`, `created_at`, and `updated_at`.

Indexes cover refund lookup, app ownership, payment order, gateway order, channel, status, and due retry scanning.

## Refund Creation and Idempotency

Refund creation follows this sequence:

1. Authenticate the app or administrator and load the payment order.
2. Require order status `paid`, a positive amount, matching currency, a bound channel account, and a channel trade number.
3. In a database transaction, lock the payment order, calculate reserved refund amount from `pending` and `succeeded` refunds, reject an over-refund, and create the new `pending` refund.
4. If the same `app_id + merchant_refund_no` already exists with identical order, amount, currency, reason, and metadata, return it as an idempotent replay.
5. If the idempotency key exists with different material fields, return an idempotency conflict.
6. Call the provider after the transaction commits, using `refund_no` as the provider idempotency identifier where supported.
7. Persist the normalized result. A network or ambiguous provider error leaves the refund pending for query recovery rather than creating another refund request.

Failed and closed refunds no longer reserve refundable amount. Pending and succeeded refunds do reserve it.

Provider-specific idempotency:

- Alipay uses `refund_no` as `out_request_no`.
- WeChat uses `refund_no` as `out_refund_no`.
- PayPal sends `PayPal-Request-Id: refund_no` and `invoice_id: refund_no`.

## Refund Recovery

Pending refunds are scanned every 30 seconds. If a channel refund ID is known, the worker queries the provider. If no channel refund ID is known, the worker repeats creation with the same gateway refund number and provider idempotency key. This safely recovers a crash or network timeout between provider acceptance and local persistence. Administrative retry uses the same identity and resets the schedule immediately.

Normalized outcomes:

- `succeeded`: set the terminal timestamp and emit `refund.succeeded`.
- `failed`: store a failure reason and emit `refund.failed`.
- `pending`: schedule another query.
- ambiguous error: keep pending, record the error, and schedule another attempt.

Automatic retry uses the same delay sequence as payment reconciliation. After eight unresolved attempts, automatic processing stops and the record remains visible as pending with its last error. Administrative retry resets its schedule immediately.

## Webhook Event Migration

Extend `webhook_events` with:

- `resource_type`;
- `resource_id`;
- optional `refund_no`.

The unique key becomes `(event_type, resource_type, resource_id)`. Existing payment events are backfilled as `resource_type = payment_order` and `resource_id = gateway_order_no`.

The legacy unique index `(event_type, gateway_order_no)` must be removed with a targeted compatibility migration. The migration must not enable global destructive index dropping.

For an existing database, startup migration performs the following before Ent auto-migration: detect the `webhook_events` table, add nullable resource columns when absent, backfill payment resources, and remove the legacy index with driver-specific PostgreSQL or MySQL SQL. Ent auto-migration then creates the desired resource-aware index. On a fresh database the compatibility step is a no-op and Ent creates the final schema directly.

`webhook_deliveries` keeps the existing denormalized fields and adds optional `resource_type`, `resource_id`, and `refund_no` so admin filtering does not require joins.

Payment event APIs continue to behave idempotently. Refund events use `resource_type = refund` and `resource_id = refund_no`, allowing multiple refunds for one payment order.

## API Surface

Open API:

```text
POST /v1/open/refunds
GET  /v1/open/refunds/:refund_no
GET  /v1/open/refunds/by-merchant/:merchant_refund_no
```

Admin API:

```text
GET  /v1/admin/payment-reconciliations
GET  /v1/admin/payment-reconciliations/:id
POST /v1/admin/payment-reconciliations/:id/retry
GET  /v1/admin/refunds
GET  /v1/admin/refunds/:id
POST /v1/admin/refunds
POST /v1/admin/refunds/:id/retry
```

Webhook delivery listing adds optional `resource_type` and `refund_no` filters.

All handlers use the global `{ code, message, data, error }` response shape and existing `httpx` error codes. Open refund reads and writes are always scoped to the authenticated app.

## Admin Console

Enable the existing disabled Refund navigation item and add:

- refund list with app, order, merchant refund number, amount, channel, status, and failure filters;
- refund detail with provider identifiers, timestamps, failure diagnostics, and retry action;
- refund creation from a paid order and from the refund page;
- reconciliation list with status, channel, order, attempt count, next attempt, and last error;
- reconciliation detail and manual retry;
- order detail summaries for reconciliation state and refunded amount;
- Webhook Center resource-type and refund-number filters.

All tables use the existing Data Table factory, all text is added to English and Simplified Chinese resources, and existing shadcn components are reused.

## SDK

The Go SDK adds:

- `Refund`, `CreateRefundRequest`, and refund result types;
- `CreateRefund`;
- `GetRefund`;
- `GetRefundByMerchant`;
- webhook payload fields for `resource_type`, `resource_id`, and refund identifiers.

The changelog records the new release surface. SDK request and response tests cover signing, paths, idempotent responses, and API errors.

## Verification Commands

Update the root Makefile so:

- `make test` runs backend tests and SDK tests;
- `make web-test` runs frontend contract tests;
- `make verify` runs backend tests, SDK tests, frontend tests, lint, type checking, and production build.

Behavior tests cover provider status mapping, callback/query validation, reconciliation retries and stale claims, concurrent refund amount reservation, refund idempotency conflicts, webhook resource uniqueness, API app scoping, SDK requests, and admin filter helpers.

## Deployment and Compatibility

Deployment order:

1. Generate Ent code.
2. Apply additive schema changes.
3. Backfill webhook resource fields.
4. Create the new resource-aware unique index.
5. Remove the legacy webhook unique index.
6. Start reconciliation and refund workers.
7. Verify health, queue groups, admin APIs, and callback routes.

The backend binary is rebuilt and the running backend service is restarted after all backend changes. Existing payment and webhook records remain queryable throughout the migration.
