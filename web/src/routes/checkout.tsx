import { createRoute } from "@tanstack/react-router";

import { CheckoutPage } from "@/features/checkout/checkout-page";
import { CheckoutResultPage } from "@/features/checkout/checkout-result-page";

import { rootRoute } from "./root";

export const checkoutRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/checkout/$gatewayOrderNo",
  component: CheckoutRoute,
});

export const checkoutResultRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/checkout/$gatewayOrderNo/result",
  component: CheckoutResultRoute,
});

function CheckoutRoute() {
  const { gatewayOrderNo } = checkoutRoute.useParams();
  return <CheckoutPage gatewayOrderNo={gatewayOrderNo} />;
}

function CheckoutResultRoute() {
  const { gatewayOrderNo } = checkoutResultRoute.useParams();
  return <CheckoutResultPage gatewayOrderNo={gatewayOrderNo} />;
}
