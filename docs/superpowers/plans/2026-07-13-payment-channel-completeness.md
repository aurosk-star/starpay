# Payment Channel Completeness Repair Plan

**Goal:** Make Alipay, WeChat Pay, and PayPal payment initiation and callback handling safe for multi-account production use, with correct terminal states and merchant events.

**Architecture:** Keep the existing `payments/provider` abstraction for payment initiation and provider notifications. Persist the selected channel account on each payment order, carry that account through notification and PayPal capture handling, and validate every terminal provider result against the local order before changing state. Use the existing idempotent webhook event store for `payment.succeeded` and the new `payment.failed` event.

**Scope:** This plan fixes payment correctness and account binding now. Provider query, channel close, and refunds are specified as the next implementation phase because they require reconciliation and refund domain models that do not exist yet.

## Constraints

- Store all amounts as integer minor units.
- Do not mark an order paid without matching order number, channel, channel account, amount, and currency.
- Do not treat PayPal order approval as captured payment.
- Preserve provider-specific callback responses.
- Add tests before each behavior change.
- Regenerate Ent after schema changes.
- After backend changes, run all tests, build the backend binary, and restart the running backend from that binary.

## Phase 1: Persist Channel Account Binding

Files:

- Modify `ent/schema/payment_order.go`
- Regenerate `ent/`
- Modify `internal/domain/orders/repository/repository.go`
- Modify `internal/domain/orders/service/service.go`
- Modify order serializers and SDK order type
- Add order and payment tests

Steps:

- [x] Add optional `channel_account_id` to payment orders and index it.
- [x] Add optional `failed_at` and `failure_reason` fields.
- [x] Return the selected channel account ID from `payments.Service.StartPayment`.
- [x] Persist channel, payment method, and channel account ID together after payment initiation.
- [x] Serialize the binding in admin/Open API order responses.
- [x] Reject a requested channel account whose channel does not match the selected channel.

## Phase 2: Bind Notifications to the Correct Account

Files:

- Modify `internal/domain/channels/repository/repository.go`
- Modify `internal/domain/payments/service/service.go`
- Modify `internal/domain/payments/handler/notify_handler.go`
- Modify `internal/platform/http/router.go`
- Add payment notification tests

Steps:

- [x] Accept optional `channel_account_id` on the unified notify endpoint.
- [x] When one account exists, keep backward-compatible automatic selection.
- [x] When multiple accounts exist, require an explicit account ID instead of choosing the newest account.
- [x] Append the selected account ID to per-order Alipay and WeChat notify URLs.
- [x] Return the verified channel account ID with the normalized provider result.
- [x] Verify that the result account matches the order binding.
- [x] Document that each PayPal webhook URL must include its account ID.

## Phase 3: Validate Terminal Provider Results

Files:

- Modify `internal/domain/payments/handler/notify_handler.go`
- Modify `internal/domain/payments/provider/paypal/paypal.go`
- Modify `internal/domain/orders/handler/checkout_handler.go`
- Add provider and handler tests

Steps:

- [x] Before `paid`, require exact channel, account, amount, and currency matches.
- [x] Before `closed` or `failed`, require channel and account matches.
- [x] Reject callbacks for an order bound to another channel.
- [x] Parse PayPal capture/webhook amount and currency.
- [x] Map only captured PayPal payment events to `paid`.
- [x] Keep `CHECKOUT.ORDER.APPROVED` pending.
- [x] Reject a PayPal return token that differs from the stored provider order ID.
- [x] Validate PayPal capture custom ID, amount, and currency before marking paid.

## Phase 4: Complete Failed Payment State

Files:

- Modify `internal/domain/orders/repository/repository.go`
- Modify `internal/domain/orders/service/service.go`
- Modify `internal/domain/webhooks/service/service.go`
- Modify frontend event labels and order types
- Add order and webhook tests

Steps:

- [x] Add idempotent `MarkFailed` behavior.
- [x] Persist `failed_at` and a normalized failure reason.
- [x] Add `payment.failed` event recording and payload.
- [x] Handle normalized provider `failed` notifications.
- [x] Ensure duplicate terminal callbacks recreate any missing merchant event without changing terminal timestamps.
- [x] Add admin/frontend labels for the real event.

## Phase 5: Provider Configuration Hardening

Files:

- Modify provider config parsing and tests
- Modify channel admin config fields where needed
- Modify integration documentation

Steps:

- [x] Restrict PayPal intent to the implemented `CAPTURE` flow.
- [x] Expose PayPal locale in channel configuration.
- [x] Reject unsupported WeChat modes during config parsing.
- [x] Stop presenting WeChat sandbox as a functional provider environment, or explicitly document that API v3 uses production endpoints.
- [x] Remove or implement unused WeChat certificate configuration.
- [x] Parse Alipay callback amounts without floating point.
- [x] Add invalid-signature and disabled-capability provider tests.

## Phase 6: Verification and Runtime Handoff

- [x] Run `make ent-up`.
- [x] Run focused order/payment/webhook tests.
- [x] Run `make test`.
- [x] Run `make web-typecheck` and `make web-build` if frontend types or labels changed.
- [x] Build `.tmp/payment-gateway-server` from `cmd/server`.
- [x] Stop stale backend processes and restart from the new binary.
- [x] Verify health and callback routes against the restarted service.

## Phase 7: Follow-up Completeness Work

These are required for full payment-channel lifecycle completeness, but are separate implementation units:

- [ ] Add `QueryProvider` and implement Alipay, WeChat, and PayPal order query.
- [ ] Add pending-order reconciliation records, scanner, Redis worker, retry policy, and manual review status.
- [ ] Add provider close support for Alipay and WeChat; define PayPal cancellation semantics.
- [ ] Build the refunds domain and implement provider refunds for all three channels.
- [ ] Introduce payment attempts so repeated PayPal initiation cannot create untracked provider orders.
- [ ] Encrypt channel secrets at rest with migration support for existing plaintext configuration.
- [ ] Add real sandbox/staging end-to-end tests and operational runbooks.
