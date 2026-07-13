import { createRoute } from "@tanstack/react-router";
import { RefundsPage } from "@/features/refunds/refunds-page";
import { rootRoute } from "./root";
export const refundsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/refunds",
  component: RefundsPage,
});
