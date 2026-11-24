export type ReconciliationStatus =
  "pending" | "processing" | "resolved" | "manual_required";
export type PaymentReconciliation = {
  id: number;
  payment_order_id: number;
  gateway_order_no: string;
  channel: string;
  channel_account_id: number;
  status: ReconciliationStatus;
  attempt_count: number;
  next_attempt_at?: string | null;
  last_attempt_at?: string | null;
  last_provider_status?: string;
  last_error?: string;
  provider_snapshot: Record<string, unknown>;
  resolved_at?: string | null;
  created_at: string;
  updated_at: string;
};
export type ReconciliationListParams = {
  status?: string;
  channel?: string;
  gateway_order_no?: string;
  page?: number;
  page_size?: number;
};
export type ReconciliationListResponse = {
  items: PaymentReconciliation[];
  total: number;
  page: number;
  page_size: number;
};
