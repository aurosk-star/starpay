import { createRouter } from "@tanstack/react-router";

import { appsRoute } from "./routes/apps";
import {
  channelCreateRoute,
  channelEditRoute,
  channelsRoute,
} from "./routes/channels";
import { gatewayConfigRoute } from "./routes/config";
import { checkoutResultRoute, checkoutRoute } from "./routes/checkout";
import { indexRoute } from "./routes/index";
import { orderDetailRoute, ordersRoute } from "./routes/orders";
import { rootRoute } from "./routes/root";
import { webhooksRoute } from "./routes/webhooks";
import { webhookDetailRoute } from "./routes/webhook-detail";
import { testPayRoute } from "./routes/test-pay";
import { usersRoute } from "./routes/users";

const routeTree = rootRoute.addChildren([
  indexRoute,
  usersRoute,
  appsRoute,
  ordersRoute,
  orderDetailRoute,
  webhooksRoute,
  webhookDetailRoute,
  testPayRoute,
  checkoutRoute,
  checkoutResultRoute,
  channelsRoute,
  channelCreateRoute,
  channelEditRoute,
  gatewayConfigRoute,
]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
