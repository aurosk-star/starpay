# Checkout Order Read Reconciliation Design

## Goal

When the checkout frontend reads an order after a provider payment has been created, trigger an immediate asynchronous payment reconciliation without adding another frontend request or delaying the response.

## Design

The existing checkout-token-protected `GET /v1/checkout/orders/:gateway_order_no` endpoint requests reconciliation after authorization. It only does so when the order has a provider order number, so initial checkout reads do not create invalid work.

The reconciliation service ensures the record exists, atomically records `active_query_requested_at`, moves the attempt to the current time, and enqueues it. Repeated GET requests do not reset attempt counts, bypass worker backoff, or add duplicate queue messages because only the first null-to-timestamp transition succeeds. Terminal, processing, and resolved reconciliations are no-ops. Already-due reconciliations are still eligible for the first active enqueue.

If enqueueing fails, the service clears `active_query_requested_at` so the next GET can retry. Queue failures are logged and do not prevent the endpoint from returning the order. Existing workers perform the provider query and retain the current retry/backoff behavior.

## Testing

Backend tests cover GET-triggered reconciliation, pre-payment reads, immediate due-time transition, duplicate request behavior, retry-backoff preservation, already-due records, enqueue failure recovery, attempt-count preservation, and terminal-order no-op behavior.
