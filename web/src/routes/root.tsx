import {
  createRootRoute,
  Outlet,
  useRouterState,
} from "@tanstack/react-router";
import { useTranslation } from "react-i18next";

import { CommandMenu } from "@/components/command-menu";
import { AuthenticatedLayout } from "@/components/layout/authenticated-layout";
import { AuthScreen } from "@/features/auth/auth-screen";
import { useAuthStore } from "@/features/auth/store";
import { useDocumentTitle } from "@/hooks/use-document-title";

export const rootRoute = createRootRoute({
  component: ShellLayout,
  notFoundComponent: NotFoundPage,
});

function ShellLayout() {
  const { t } = useTranslation();
  const accessToken = useAuthStore((state) => state.accessToken);
  const pathname = useRouterState({
    select: (state) => state.location.pathname,
  });
  const pageTitle = getPageTitle(pathname, t);
  useDocumentTitle(pageTitle);

  if (pathname.startsWith("/checkout/")) {
    return <Outlet />;
  }

  if (!accessToken) {
    return <AuthScreen />;
  }

  return (
    <AuthenticatedLayout
      title={pageTitle}
      headerActions={<CommandMenu />}
    />
  );
}

function NotFoundPage() {
  const { t } = useTranslation();
  return (
    <AuthenticatedLayout title={t("errors.notFound.title")}>
      <div className="flex min-h-64 flex-col items-center justify-center gap-3 text-center">
        <p className="text-5xl font-semibold tracking-tight">404</p>
        <p className="text-muted-foreground">{t("errors.notFound.description")}</p>
      </div>
    </AuthenticatedLayout>
  );
}

function getPageTitle(
  pathname: string,
  t: ReturnType<typeof useTranslation>["t"],
) {
  if (pathname === "/") return t("nav.overview");
  if (pathname === "/users") return t("users.title");
  if (pathname === "/apps") return t("apps.title");
  if (pathname === "/orders") return t("orders.title");
  if (pathname.startsWith("/orders/")) return t("orders.detailTitle");
  if (pathname === "/webhooks") return t("webhooks.title");
  if (pathname.startsWith("/webhooks/")) return t("webhooks.detailTitle");
  if (pathname === "/refunds") return t("refunds.title");
  if (pathname.startsWith("/refunds/")) return t("refunds.detailTitle");
  if (pathname === "/reconciliations") return t("reconciliations.title");
  if (pathname.startsWith("/reconciliations/")) {
    return t("reconciliations.detailTitle");
  }
  if (pathname === "/test-pay") return t("testPay.title");
  if (pathname.startsWith("/channels")) return t("channels.title");
  if (pathname.startsWith("/routing")) return t("routing.title");
  if (pathname === "/config/gateway") return t("config.title");
  if (pathname.startsWith("/checkout/") && pathname.endsWith("/result")) {
    return t("checkout.paidTitle");
  }
  if (pathname.startsWith("/checkout/")) return t("checkout.title");
  return t("nav.overview");
}
