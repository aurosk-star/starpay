import type { TFunction } from "i18next";

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

export function formatWebhookEventType(eventType: string, t: TFunction) {
  if (!knownEventTypes.has(eventType)) {
    return eventType || "-";
  }
  return t("webhooks.eventTypes.withDescription", {
    eventType,
    description: t(`webhooks.eventTypes.${eventType}`),
  });
}
