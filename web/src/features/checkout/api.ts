import { apiRequest } from "@/lib/api";

import type {
  CheckoutMethodsResponse,
  CheckoutOrderResponse,
  StartCheckoutPaymentPayload,
  StartCheckoutPaymentResponse,
} from "./types";

function checkoutHeaders(token: string) {
  return {
    "X-Checkout-Token": token,
  };
}

export function getCheckoutOrder(gatewayOrderNo: string, token: string) {
  return apiRequest<CheckoutOrderResponse>(
    `/v1/checkout/orders/${encodeURIComponent(gatewayOrderNo)}`,
    {
      headers: checkoutHeaders(token),
    },
  );
}

export function listCheckoutMethods(gatewayOrderNo: string, token: string) {
  return apiRequest<CheckoutMethodsResponse>(
    `/v1/checkout/orders/${encodeURIComponent(gatewayOrderNo)}/methods`,
    {
      headers: checkoutHeaders(token),
    },
  );
}

export function startCheckoutPayment(
  gatewayOrderNo: string,
  token: string,
  payload: StartCheckoutPaymentPayload,
) {
  return apiRequest<StartCheckoutPaymentResponse>(
    `/v1/checkout/orders/${encodeURIComponent(gatewayOrderNo)}/pay`,
    {
      method: "POST",
      headers: checkoutHeaders(token),
      body: JSON.stringify(payload),
    },
  );
}
