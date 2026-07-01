import { createRoute } from "@tanstack/react-router";

import { TestPayPage } from "@/features/test-pay/test-pay-page";

import { rootRoute } from "./root";

export const testPayRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/test-pay",
  component: TestPayPage,
});
