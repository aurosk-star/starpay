import { createRoute } from "@tanstack/react-router";
import { RefundDetailPage } from "@/features/refunds/refund-detail-page";
import { rootRoute } from "./root";
export const refundDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/refunds/$refundId",
  component: RefundDetailPage,
});
