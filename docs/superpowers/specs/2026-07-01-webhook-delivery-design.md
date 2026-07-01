# Webhook Delivery Design

## Goal

Build the gateway-to-business webhook delivery flow for payment success events, with durable persistence, Redis-assisted async dispatch, retries, and admin visibility.

## Architecture

Webhook delivery is a separate concern from payment-platform callbacks. Payment providers notify the gateway through `/v1/channel/notify`; after the gateway marks an order paid, it creates a webhook event for the business application and enqueues a delivery job.

The source of truth is the database. Redis Stream is only an async transport hint for fast delivery. A background worker consumes Redis jobs, delivers HTTP POST requests to the app `notify_url`, and updates delivery state in the database. A DB scan job re-enqueues missed or delayed deliveries for recovery.

## Scope

- In scope: `payment.succeeded` delivery, delivery retries, admin listing, manual retry, response capture.
- Out of scope: refunds, subscription webhooks, external merchant onboarding, MQ migration.

## Data Model

- `webhook_events`
  - `event_id`, `event_type`, `app_id`, `gateway_order_no`, `payload`, `created_at`
- `webhook_deliveries`
  - `delivery_no`, `event_id`, `app_id`, `target_url`, `status`, `attempt_count`, `next_attempt_at`, `last_http_status`, `last_response_body`, `last_error`, timestamps

## Delivery Contract

- Trigger: order transitions to `paid`.
- Target: app-level `notify_url`.
- Request: `POST` JSON body with headers `X-Pay-Gateway-Event-Id`, `X-Pay-Gateway-Timestamp`, `X-Pay-Gateway-Signature`.
- Signature: `HMAC-SHA256(app_secret, timestamp + "." + raw_body)`.
- Success: any `2xx` response.
- Failure: network error, timeout, or non-2xx response.

## Retry Policy

- Retry schedule: `10s -> 30s -> 2m -> 10m -> 30m -> 2h`.
- Keep retry state in `webhook_deliveries`.
- Redis job loss must not lose delivery state; DB scan restores pending work.

## Admin Surface

- List webhook deliveries.
- Filter by app, event type, status, time.
- Inspect last HTTP status, response body, and error text.
- Trigger manual retry.

## Testing

- Cover event creation on paid orders.
- Cover delivery success, failure, and retry scheduling.
- Cover idempotent re-delivery for the same event.
- Cover worker recovery after Redis miss via DB scan.
- Keep tests in `internal/domain/webhooks/test/`.

## Implementation Order

1. Add webhook Ent schemas and repositories.
2. Add webhook service for event creation and delivery state transitions.
3. Add Redis enqueue and worker consumer.
4. Add DB recovery scanner.
5. Add admin handlers and routes.
6. Add frontend Webhook center pages.
