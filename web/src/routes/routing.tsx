import { createRoute } from "@tanstack/react-router";

import { RoutingFormPage } from "@/features/routing/routing-form-page";
import { RoutingPage } from "@/features/routing/routing-page";

import { rootRoute } from "./root";

export const routingRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/routing",
  component: RoutingPage,
});

export const routingCreateRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/routing/new",
  component: () => <RoutingFormPage mode="create" />,
});

export const routingEditRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/routing/$ruleId/edit",
  component: () => <RoutingFormPage mode="edit" />,
});
