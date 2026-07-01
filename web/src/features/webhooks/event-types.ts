import type { TFunction } from "i18next";

const knownEventTypes = new Set([
  "payment.succeeded",
  "payment.failed",
  "order.expired",
  "refund.succeeded",
  "refund.failed",
]);

export function formatWebhookEventType(eventType: string, t: TFunction) {
  if (!knownEventTypes.has(eventType)) {
    return eventType || "-";
  }
  return t("webhooks.eventTypes.withDescription", {
    eventType,
    description: t(`webhooks.eventTypes.${eventType}`),
  });
}
