import { createRoute } from "@tanstack/react-router";

import { ChannelFormPage } from "@/features/channels/channel-form-page";
import { ChannelsPage } from "@/features/channels/channels-page";

import { rootRoute } from "./root";

export const channelsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/channels",
  component: ChannelsPage,
});

export const channelCreateRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/channels/new",
  component: () => <ChannelFormPage mode="create" />,
});

export const channelEditRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/channels/$channelId/edit",
  component: () => <ChannelFormPage mode="edit" />,
});
