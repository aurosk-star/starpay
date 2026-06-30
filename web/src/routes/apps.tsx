import { createRoute } from "@tanstack/react-router";

import { AppsPage } from "@/features/apps/apps-page";

import { rootRoute } from "./root";

export const appsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/apps",
  component: AppsPage,
});
