import { createRoute } from "@tanstack/react-router";

import { WebhookDetailPage } from "@/features/webhooks/webhook-detail-page";

import { rootRoute } from "./root";

export const webhookDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/webhooks/$deliveryId",
  component: WebhookDetailPage,
});
