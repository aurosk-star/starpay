import { apiRequest } from "@/lib/api";

import type {
  ListOrdersParams,
  ListOrdersResponse,
  ManageOrderPayload,
  PaymentOrder,
  UpdateOrderPayload,
} from "./types";

export function listOrders(accessToken: string, params: ListOrdersParams = {}) {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === "") continue;
    search.set(key, String(value));
  }
  const query = search.toString();
  return apiRequest<ListOrdersResponse>(
    `/v1/admin/orders${query ? `?${query}` : ""}`,
    { accessToken },
  );
}

export function getOrder(accessToken: string, id: number) {
  return apiRequest<{ order: PaymentOrder }>(`/v1/admin/orders/${id}`, {
    accessToken,
  });
}

export function createOrder(accessToken: string, payload: ManageOrderPayload) {
  return apiRequest<{ order: PaymentOrder }>("/v1/admin/orders", {
    method: "POST",
    accessToken,
    body: JSON.stringify(payload),
  });
}

export function updateOrder(
  accessToken: string,
  id: number,
  payload: UpdateOrderPayload,
) {
  return apiRequest<{ order: PaymentOrder }>(`/v1/admin/orders/${id}`, {
    method: "PUT",
    accessToken,
    body: JSON.stringify(payload),
  });
}

export function closeOrder(accessToken: string, id: number) {
  return apiRequest<{ order: PaymentOrder }>(`/v1/admin/orders/${id}/close`, {
    method: "POST",
    accessToken,
  });
}
