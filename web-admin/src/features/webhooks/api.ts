import { apiRequest } from "@/lib/api";

import type { ListWebhookDeliveriesResponse, WebhookDelivery } from "./types";

export function listWebhookDeliveries(
  accessToken: string,
  params: Record<string, string | number | undefined> = {},
) {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === "") continue;
    search.set(key, String(value));
  }
  const query = search.toString();
  return apiRequest<ListWebhookDeliveriesResponse>(
    `/v1/admin/webhook-deliveries${query ? `?${query}` : ""}`,
    { accessToken },
  );
}

export function getWebhookDelivery(accessToken: string, id: number) {
  return apiRequest<{ webhook_delivery: WebhookDelivery }>(
    `/v1/admin/webhook-deliveries/${id}`,
    { accessToken },
  );
}

export function retryWebhookDelivery(accessToken: string, id: number) {
  return apiRequest<{ webhook_delivery: WebhookDelivery }>(
    `/v1/admin/webhook-deliveries/${id}/retry`,
    {
      method: "POST",
      accessToken,
    },
  );
}
