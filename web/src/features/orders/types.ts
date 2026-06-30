export type PaymentOrderStatus = "pending" | "paid" | "failed" | "closed";

export type PaymentOrder = {
  id: number;
  gateway_order_no: string;
  app_id: string;
  merchant_order_no: string;
  business_type?: string;
  subject: string;
  description?: string;
  amount: number;
  currency: string;
  settlement_amount?: number;
  settlement_currency?: string;
  channel?: string;
  pay_method?: string;
  channel_trade_no?: string;
  status: PaymentOrderStatus;
  expires_at?: string | null;
  paid_at?: string | null;
  closed_at?: string | null;
  metadata: Record<string, unknown>;
  created_at: string;
  updated_at: string;
};

export type ListOrdersParams = {
  app_id?: string;
  status?: string;
  channel?: string;
  currency?: string;
  merchant_order_no?: string;
  page?: number;
  page_size?: number;
};

export type ListOrdersResponse = {
  items: PaymentOrder[];
  total: number;
  page: number;
  page_size: number;
};

export type ManageOrderPayload = {
  app_id: string;
  merchant_order_no: string;
  business_type?: string;
  subject: string;
  description?: string;
  amount: number;
  currency: string;
  settlement_amount?: number;
  settlement_currency?: string;
  channel?: string;
  pay_method?: string;
  expires_at?: string;
  metadata?: Record<string, unknown>;
};

export type UpdateOrderPayload = {
  business_type?: string;
  subject: string;
  description?: string;
  channel?: string;
  pay_method?: string;
  metadata?: Record<string, unknown>;
};
