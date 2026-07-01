import { createRouter } from "@tanstack/react-router";

import { appsRoute } from "./routes/apps";
import {
  channelCreateRoute,
  channelEditRoute,
  channelsRoute,
} from "./routes/channels";
import { gatewayConfigRoute } from "./routes/config";
import { checkoutRoute, mockPayRoute } from "./routes/checkout";
import { indexRoute } from "./routes/index";
import { ordersRoute } from "./routes/orders";
import { rootRoute } from "./routes/root";
import { testPayRoute } from "./routes/test-pay";
import { usersRoute } from "./routes/users";

const routeTree = rootRoute.addChildren([
  indexRoute,
  usersRoute,
  appsRoute,
  ordersRoute,
  testPayRoute,
  checkoutRoute,
  mockPayRoute,
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
