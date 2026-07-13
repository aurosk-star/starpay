import { createRoute } from "@tanstack/react-router";
import { ReconciliationsPage } from "@/features/reconciliations/reconciliations-page";
import { rootRoute } from "./root";
export const reconciliationsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/reconciliations",
  component: ReconciliationsPage,
});
