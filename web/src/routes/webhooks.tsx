import { createRoute } from "@tanstack/react-router";

import { WebhooksPage } from "@/features/webhooks/webhooks-page";

import { rootRoute } from "./root";

export const webhooksRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/webhooks",
  component: WebhooksPage,
});
