export type CheckoutOrderStatus = "pending" | "paid" | "failed" | "closed";

export type CheckoutOrder = {
  gateway_order_no: string;
  merchant_order_no: string;
  business_type?: string;
  subject: string;
  description?: string;
  amount: number;
  currency: string;
  channel?: string;
  pay_method?: string;
  return_url?: string;
  status: CheckoutOrderStatus;
  expires_at?: string | null;
  created_at: string;
};

export type CheckoutOrderResponse = {
  title: string;
  order: CheckoutOrder;
};

export type CheckoutPaymentMethod = {
  pay_method: string;
  channel: string;
  channel_account_id?: number;
  pay_mode?: string;
  rule_id?: number;
  target_id?: number;
  label: string;
  enabled: boolean;
};

export type CheckoutMethodsResponse = {
  locked: boolean;
  selected_method?: CheckoutPaymentMethod;
  methods: CheckoutPaymentMethod[];
};

export type StartCheckoutPaymentPayload = {
  pay_method?: string;
  channel?: string;
  channel_account_id?: number;
  return_url?: string;
};

export type CheckoutPaymentResult = {
  status: string;
  channel: string;
  pay_method: string;
  provider_order_no: string;
  pay_url?: string;
  qr_code?: string;
  form_html?: string;
};

export type StartCheckoutPaymentResponse = {
  payment: CheckoutPaymentResult;
};
