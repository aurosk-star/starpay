import { createRouter } from "@tanstack/react-router";

import { appsRoute } from "./routes/apps";
import {
  channelCreateRoute,
  channelEditRoute,
  channelsRoute,
} from "./routes/channels";
import { indexRoute } from "./routes/index";
import { ordersRoute } from "./routes/orders";
import { rootRoute } from "./routes/root";
import { usersRoute } from "./routes/users";

const routeTree = rootRoute.addChildren([
  indexRoute,
  usersRoute,
  appsRoute,
  ordersRoute,
  channelsRoute,
  channelCreateRoute,
  channelEditRoute,
]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
