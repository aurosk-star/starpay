import { apiRequest } from "@/lib/api";
import { buildRefundSearch } from "./filters";
import type {
  CreateRefundPayload,
  Refund,
  RefundListParams,
  RefundListResponse,
} from "./types";
export function listRefunds(token: string, params: RefundListParams = {}) {
  const query = buildRefundSearch(params);
  return apiRequest<RefundListResponse>(
    `/v1/admin/refunds${query ? `?${query}` : ""}`,
    { accessToken: token },
  );
}
export function getRefund(token: string, id: number) {
  return apiRequest<{ refund: Refund }>(`/v1/admin/refunds/${id}`, {
    accessToken: token,
  });
}
export function createRefund(token: string, payload: CreateRefundPayload) {
  return apiRequest<{ created: boolean; refund: Refund }>("/v1/admin/refunds", {
    method: "POST",
    accessToken: token,
    body: JSON.stringify(payload),
  });
}
export function retryRefund(token: string, id: number) {
  return apiRequest<{ refund: Refund }>(`/v1/admin/refunds/${id}/retry`, {
    method: "POST",
    accessToken: token,
  });
}
