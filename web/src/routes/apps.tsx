import { createRoute } from "@tanstack/react-router";

import { AppDetailPage } from "@/features/apps/app-detail-page";
import { AppsPage } from "@/features/apps/apps-page";

import { rootRoute } from "./root";

export const appsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/apps",
  component: AppsPage,
});

export const appDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/apps/$appId",
  component: AppDetailPage,
});
