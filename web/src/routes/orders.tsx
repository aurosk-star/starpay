import { createRoute } from "@tanstack/react-router";

import { OrderDetailPage } from "@/features/orders/order-detail-page";
import { OrdersPage } from "@/features/orders/orders-page";

import { rootRoute } from "./root";

export const ordersRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/orders",
  component: OrdersPage,
});

export const orderDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/orders/$orderId",
  component: OrderDetailPage,
});
