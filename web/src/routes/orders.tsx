import { createRoute } from "@tanstack/react-router";

import { OrdersPage } from "@/features/orders/orders-page";

import { rootRoute } from "./root";

export const ordersRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/orders",
  component: OrdersPage,
});
