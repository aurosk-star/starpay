# Order Closed Webhook Design

## Goal

Notify merchant systems when an administrator or authenticated merchant explicitly closes a payment order, without conflating an intentional close with automatic expiration.

## Event Semantics

Add `order.closed` as a payment-order Webhook event.

- `POST /v1/admin/orders/:id/close` emits `order.closed` with `close_source=admin`.
- `POST /v1/open/orders/:gateway_order_no/close` emits `order.closed` with `close_source=merchant`.
- Automatic timeout closure continues to emit `order.expired` and never emits `order.closed`.
- A repeated close request does not create another event or delivery.
- Paid orders and other states that cannot be closed continue to reject the close request and emit no event.

## Data Flow

The order service accepts an explicit close source when performing an intentional close. It atomically transitions only `pending` or `failed` orders to `closed`, then records one `order.closed` event through the existing Webhook service. The event creates a delivery only when the owning app has a non-empty `notify_url`; delivery continues to use the existing Redis Stream worker, signature headers, retry schedule, and admin visibility.

The existing unique Webhook identity `(event_type, resource_type, resource_id)` makes `order.closed` idempotent per payment order. `order.expired` remains a separate identity for automatic timeout handling.

## Payload

`order.closed` uses `resource_type=payment_order` and `resource_id=gateway_order_no`. Its payload contains:

- `event_type`: `order.closed`
- `resource_type`: `payment_order`
- `resource_id`: gateway order number
- `app_id`
- `gateway_order_no`
- `merchant_order_no`
- `status`: `closed`
- `close_source`: `admin` or `merchant`
- `amount`, `currency`, `channel`, `pay_method`, `metadata`
- `closed_at`

## Failure Handling

The order transition and event persistence must not leave a silently closed order when event creation fails. Intentional close should use the same transaction/outbox guarantees as expiration closure: database event and pending delivery are durable before Redis enqueue, and enqueue failure is recovered by the delivery scanner.

Concurrent payment and close operations retain the existing conditional status transitions. A payment that wins first prevents close; a close that wins first prevents a later payment callback from overwriting `closed`.

## Tests

- Admin close emits exactly one `order.closed` event and delivery with `close_source=admin`.
- Merchant close emits exactly one `order.closed` event and delivery with `close_source=merchant`.
- Automatic expiration emits only `order.expired`.
- Duplicate and rejected close attempts emit no duplicate `order.closed` event.
- Webhook persistence failure rolls back the intentional close.
- The frontend Webhook event filter includes `order.closed` in both languages.
