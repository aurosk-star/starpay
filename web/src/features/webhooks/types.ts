export type WebhookDeliveryStatus = "pending" | "succeeded" | "failed";

export type WebhookEvent = {
  id: number;
  event_id: string;
  event_type: string;
  app_id: string;
  gateway_order_no: string;
  payment_order_id?: number;
  payload: Record<string, unknown>;
  created_at: string;
  updated_at: string;
};

export type WebhookDelivery = {
  id: number;
  delivery_no: string;
  event_id: number;
  app_id: string;
  event_type: string;
  gateway_order_no: string;
  target_url: string;
  status: WebhookDeliveryStatus;
  attempt_count: number;
  next_attempt_at?: string | null;
  last_attempt_at?: string | null;
  last_status_code?: number | null;
  last_response_body?: string;
  last_error?: string;
  succeeded_at?: string | null;
  created_at: string;
  updated_at: string;
};

export type ListWebhookDeliveriesResponse = {
  items: WebhookDelivery[];
  total: number;
  page: number;
  page_size: number;
};
