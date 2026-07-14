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
    "order.closed",
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
