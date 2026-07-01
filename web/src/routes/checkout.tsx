import { createRoute } from "@tanstack/react-router";

import { CheckoutPage } from "@/features/checkout/checkout-page";

import { rootRoute } from "./root";

export const checkoutRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/checkout/$gatewayOrderNo",
  component: CheckoutRoute,
});

function CheckoutRoute() {
  const { gatewayOrderNo } = checkoutRoute.useParams();
  return <CheckoutPage gatewayOrderNo={gatewayOrderNo} />;
}
