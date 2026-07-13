# Webhook Event Type Filter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the Webhook Center event-type text input with a dropdown limited to event types currently emitted by the gateway.

**Architecture:** Keep event identifiers and query-value normalization in `event-types.ts` as the shared source of truth. The page renders the existing shadcn `Select`, keeps submit-based filtering, and converts the UI-only `all` value to an omitted API query parameter.

**Tech Stack:** React 19, TypeScript, shadcn/ui Select, i18next, Node test runner, Rsbuild.

## Global Constraints

- Use the existing shadcn `Select` implementation.
- Preserve the current backend `event_type` exact-match API contract.
- Show only `payment.succeeded`, `payment.failed`, and `order.expired`.
- Keep technical event identifiers visible alongside localized descriptions.
- Do not introduce a backend endpoint or new dependency.

---

### Task 1: Add the Webhook Event Type Dropdown

**Files:**
- Modify: `web/src/features/webhooks/event-types.ts`
- Modify: `web/src/features/webhooks/webhooks-page.tsx`
- Create: `web/test/webhook-event-types.test.mts`

**Interfaces:**
- Produces: `supportedWebhookEventTypes`, a readonly array of emitted event identifiers.
- Produces: `normalizeWebhookEventTypeFilter(value: string): string`, returning an empty string for `all` and the exact identifier otherwise.
- Consumes: the existing `formatWebhookEventType(eventType, t)` formatter and `/v1/admin/webhook-deliveries?event_type=...` API.

- [x] **Step 1: Write the failing event-type contract test**

```typescript
import assert from "node:assert/strict";
import test from "node:test";

import {
  normalizeWebhookEventTypeFilter,
  supportedWebhookEventTypes,
} from "../src/features/webhooks/event-types.ts";

test("lists only webhook event types currently emitted by the gateway", () => {
  assert.deepEqual(supportedWebhookEventTypes, [
    "payment.succeeded",
    "payment.failed",
    "order.expired",
  ]);
});

test("omits the all filter and preserves concrete event identifiers", () => {
  assert.equal(normalizeWebhookEventTypeFilter("all"), "");
  assert.equal(
    normalizeWebhookEventTypeFilter("payment.failed"),
    "payment.failed",
  );
});
```

- [x] **Step 2: Run the test and verify it fails because the exports do not exist**

Run: `node --test test/webhook-event-types.test.mts`

Expected: FAIL because `supportedWebhookEventTypes` and `normalizeWebhookEventTypeFilter` are not exported yet.

- [x] **Step 3: Add the shared supported list and normalization helper**

```typescript
export const supportedWebhookEventTypes = [
  "payment.succeeded",
  "payment.failed",
  "order.expired",
] as const;

const knownEventTypes = new Set<string>([
  ...supportedWebhookEventTypes,
  "refund.succeeded",
  "refund.failed",
]);

export function normalizeWebhookEventTypeFilter(value: string) {
  return value === "all" ? "" : value;
}
```

- [x] **Step 4: Run the contract test and verify it passes**

Run: `node --test test/webhook-event-types.test.mts`

Expected: PASS with 2 tests and 0 failures.

- [x] **Step 5: Replace the event-type input with the existing Select**

Set `defaultFilters.eventType` to `all`. Render an All option followed by `supportedWebhookEventTypes`, using `formatWebhookEventType` for each label. Pass `normalizeWebhookEventTypeFilter(nextFilters.eventType)` to `listWebhookDeliveries`.

```tsx
<Select
  value={filters.eventType}
  onValueChange={(value) =>
    setFilters((current) => ({ ...current, eventType: value }))
  }
>
  <SelectTrigger>
    <SelectValue />
  </SelectTrigger>
  <SelectContent>
    <SelectItem value="all">{t("common.all")}</SelectItem>
    {supportedWebhookEventTypes.map((eventType) => (
      <SelectItem key={eventType} value={eventType}>
        {formatWebhookEventType(eventType, t)}
      </SelectItem>
    ))}
  </SelectContent>
</Select>
```

- [x] **Step 6: Run focused and full frontend verification**

Run:

```bash
node --test test/webhook-event-types.test.mts
bun run lint
bun run typecheck
bun run build
```

Expected: all commands exit with status 0.
