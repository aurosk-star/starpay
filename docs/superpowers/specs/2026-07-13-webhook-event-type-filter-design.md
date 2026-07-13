# Webhook Event Type Filter Design

## Goal

Replace the Webhook Center event-type free-text filter with a constrained dropdown containing only event types currently produced by the gateway.

## Scope

- Change only the Webhook Center frontend filter.
- Keep the existing backend `event_type` query parameter and exact-match behavior.
- Do not add a metadata endpoint or derive options from the currently loaded page.
- Do not expose refund events until the refunds domain produces them.

## Supported Options

The dropdown contains:

- All
- `payment.succeeded`
- `payment.failed`
- `order.expired`

Each concrete event option uses the existing localized event description. The technical event name remains visible so operators can correlate the filter with webhook payloads and headers.

## Component Design

`web/src/features/webhooks/event-types.ts` exports the supported event-type list so formatting and filter options share one source of truth.

`web/src/features/webhooks/webhooks-page.tsx` replaces the event-type `Input` with the existing shadcn `Select`. The filter state uses `all` as the UI-only value. Search converts `all` to an omitted `event_type` query parameter, and reset restores `all`.

The existing submit-based filtering remains unchanged: selecting an option does not request data until the operator presses Search.

## Error Handling

No new error path is introduced. Existing API error toasts remain responsible for failed list requests.

## Verification

- Run frontend lint.
- Run TypeScript type checking.
- Run the production frontend build.
- Verify that All omits `event_type` and each concrete option sends its exact event identifier.

