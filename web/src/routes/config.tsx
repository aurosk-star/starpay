import { createRoute } from "@tanstack/react-router";

import { GatewayConfigPage } from "@/features/config/gateway-config-page";

import { rootRoute } from "./root";

export const gatewayConfigRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/config/gateway",
  component: GatewayConfigPage,
});
