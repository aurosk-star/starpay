import { apiRequest } from "@/lib/api";
import { buildReconciliationSearch } from "./filters";
import type {
  PaymentReconciliation,
  ReconciliationListParams,
  ReconciliationListResponse,
} from "./types";
export function listReconciliations(
  token: string,
  params: ReconciliationListParams = {},
) {
  const query = buildReconciliationSearch(params);
  return apiRequest<ReconciliationListResponse>(
    `/v1/admin/payment-reconciliations${query ? `?${query}` : ""}`,
    { accessToken: token },
  );
}
export function getReconciliation(token: string, id: number) {
  return apiRequest<{ payment_reconciliation: PaymentReconciliation }>(
    `/v1/admin/payment-reconciliations/${id}`,
    { accessToken: token },
  );
}
export function retryReconciliation(token: string, id: number) {
  return apiRequest<{ payment_reconciliation: PaymentReconciliation }>(
    `/v1/admin/payment-reconciliations/${id}/retry`,
    { method: "POST", accessToken: token },
  );
}
