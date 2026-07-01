import { apiRequest } from "@/lib/api";

import type {
  CheckoutMethodsResponse,
  CheckoutOrderResponse,
  StartCheckoutPaymentPayload,
  StartCheckoutPaymentResponse,
} from "./types";

export function getCheckoutOrder(gatewayOrderNo: string) {
  return apiRequest<CheckoutOrderResponse>(
    `/v1/checkout/orders/${encodeURIComponent(gatewayOrderNo)}`,
  );
}

export function listCheckoutMethods(gatewayOrderNo: string) {
  return apiRequest<CheckoutMethodsResponse>(
    `/v1/checkout/orders/${encodeURIComponent(gatewayOrderNo)}/methods`,
  );
}

export function startCheckoutPayment(
  gatewayOrderNo: string,
  payload: StartCheckoutPaymentPayload,
) {
  return apiRequest<StartCheckoutPaymentResponse>(
    `/v1/checkout/orders/${encodeURIComponent(gatewayOrderNo)}/pay`,
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
  );
}

export function completeMockPayment(gatewayOrderNo: string) {
  return apiRequest<{ order: { status: string; gateway_order_no: string } }>(
    `/v1/checkout/mock-pay/${encodeURIComponent(gatewayOrderNo)}/complete`,
    { method: "POST" },
  );
}
