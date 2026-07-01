import { createRoute } from "@tanstack/react-router";

import { CheckoutPage } from "@/features/checkout/checkout-page";
import { MockPayPage } from "@/features/checkout/mock-pay-page";

import { rootRoute } from "./root";

export const checkoutRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/checkout/$gatewayOrderNo",
  component: CheckoutRoute,
});

export const mockPayRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/checkout/mock-pay/$gatewayOrderNo",
  component: MockPayRoute,
});

function CheckoutRoute() {
  const { gatewayOrderNo } = checkoutRoute.useParams();
  return <CheckoutPage gatewayOrderNo={gatewayOrderNo} />;
}

function MockPayRoute() {
  const { gatewayOrderNo } = mockPayRoute.useParams();
  return <MockPayPage gatewayOrderNo={gatewayOrderNo} />;
}
