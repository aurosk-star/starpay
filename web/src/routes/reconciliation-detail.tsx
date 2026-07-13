import { createRoute } from "@tanstack/react-router";
import { ReconciliationDetailPage } from "@/features/reconciliations/reconciliation-detail-page";
import { rootRoute } from "./root";
export const reconciliationDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/reconciliations/$reconciliationId",
  component: ReconciliationDetailPage,
});
