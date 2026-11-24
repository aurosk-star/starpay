import {
  Outlet,
  createRouter,
  createRootRouteWithContext,
  createRoute,
  useRouterState,
} from "@tanstack/react-router";
import type { QueryClient } from "@tanstack/react-query";

import { AuthenticatedLayout } from "@/components/layout/authenticated-layout";
import { Toaster } from "@/components/ui/sonner";
import { AuthScreen } from "@/features/auth/auth-screen";
import { useAuthStore } from "@/features/auth/store";
import { AppsPage } from "@/features/apps/apps-page";
import { AppDetailPage } from "@/features/apps/app-detail-page";
import { ChannelsPage } from "@/features/channels/channels-page";
import { ChannelFormPage } from "@/features/channels/channel-form-page";
import { CheckoutPage } from "@/features/checkout/checkout-page";
import { CheckoutResultPage } from "@/features/checkout/checkout-result-page";
import { GatewayConfigPage } from "@/features/config/gateway-config-page";
import { Dashboard } from "@/features/dashboard";
import { GeneralError } from "@/features/errors/general-error";
import { OrderDetailPage } from "@/features/orders/order-detail-page";
import { OrdersPage } from "@/features/orders/orders-page";
import { ReconciliationDetailPage } from "@/features/reconciliations/reconciliation-detail-page";
import { ReconciliationsPage } from "@/features/reconciliations/reconciliations-page";
import { RefundDetailPage } from "@/features/refunds/refund-detail-page";
import { RefundsPage } from "@/features/refunds/refunds-page";
import { RoutingFormPage } from "@/features/routing/routing-form-page";
import { RoutingPage } from "@/features/routing/routing-page";
import { TestPayPage } from "@/features/test-pay/test-pay-page";
import { UsersPage } from "@/features/users/users-page";
import { WebhookDetailPage } from "@/features/webhooks/webhook-detail-page";
import { WebhooksPage } from "@/features/webhooks/webhooks-page";

export const rootRoute = createRootRouteWithContext<{ queryClient: QueryClient }>()({
  component: ShellLayout,
  errorComponent: GeneralError,
  notFoundComponent: () => <GeneralError />,
});

function ShellLayout() {
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const accessToken = useAuthStore((state) => state.accessToken);

  return (
    <>
      {pathname.startsWith("/checkout/") ? (
        <Outlet />
      ) : !accessToken ? (
        <AuthScreen />
      ) : (
        <AuthenticatedLayout>
          <Outlet />
        </AuthenticatedLayout>
      )}
      <Toaster duration={5000} />
    </>
  );
}

const route = <TPath extends string>(path: TPath, component: React.ComponentType) =>
  createRoute({
    getParentRoute: () => rootRoute,
    path: path as TPath,
    component: component as never,
  });

export const indexRoute = route("/", Dashboard);
export const usersRoute = route("/users", UsersPage);
export const appsRoute = route("/apps", AppsPage);
export const appDetailRoute = route("/apps/$appId", AppDetailPage);
export const ordersRoute = route("/orders", OrdersPage);
export const orderDetailRoute = route("/orders/$orderId", OrderDetailPage);
export const webhooksRoute = route("/webhooks", WebhooksPage);
export const webhookDetailRoute = route("/webhooks/$deliveryId", WebhookDetailPage);
export const refundsRoute = route("/refunds", RefundsPage);
export const refundDetailRoute = route("/refunds/$refundId", RefundDetailPage);
export const reconciliationsRoute = route("/reconciliations", ReconciliationsPage);
export const reconciliationDetailRoute = route(
  "/reconciliations/$reconciliationId",
  ReconciliationDetailPage,
);
export const testPayRoute = route("/test-pay", TestPayPage);
export const channelsRoute = route("/channels", ChannelsPage);
export const channelCreateRoute = route(
  "/channels/new",
  () => <ChannelFormPage mode="create" />,
);
export const channelEditRoute = route(
  "/channels/$channelId/edit",
  () => <ChannelFormPage mode="edit" />,
);
export const routingRoute = route("/routing", RoutingPage);
export const routingCreateRoute = route(
  "/routing/new",
  () => <RoutingFormPage mode="create" />,
);
export const routingEditRoute = route(
  "/routing/$ruleId/edit",
  () => <RoutingFormPage mode="edit" />,
);
export const gatewayConfigRoute = route("/config/gateway", GatewayConfigPage);
export const checkoutRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/checkout/$gatewayOrderNo",
  component: function CheckoutRoute() {
    const { gatewayOrderNo } = checkoutRoute.useParams();
    return <CheckoutPage gatewayOrderNo={gatewayOrderNo} />;
  },
});
export const checkoutResultRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/checkout/$gatewayOrderNo/result",
  component: function CheckoutResultRoute() {
    const { gatewayOrderNo } = checkoutResultRoute.useParams();
    return <CheckoutResultPage gatewayOrderNo={gatewayOrderNo} />;
  },
});

export const routeTree = rootRoute.addChildren([
  indexRoute,
  usersRoute,
  appsRoute,
  appDetailRoute,
  ordersRoute,
  orderDetailRoute,
  webhooksRoute,
  webhookDetailRoute,
  refundsRoute,
  refundDetailRoute,
  reconciliationsRoute,
  reconciliationDetailRoute,
  testPayRoute,
  channelsRoute,
  channelCreateRoute,
  channelEditRoute,
  routingRoute,
  routingCreateRoute,
  routingEditRoute,
  gatewayConfigRoute,
  checkoutRoute,
  checkoutResultRoute,
]);

export const router = createRouter({
  routeTree,
  context: { queryClient: undefined as never },
  defaultPreload: "intent",
  defaultPreloadStaleTime: 0,
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
