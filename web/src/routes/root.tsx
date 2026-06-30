import { createRootRoute, Outlet } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import {
  Bell,
  Boxes,
  ClipboardList,
  CreditCard,
  Gauge,
  GitBranch,
  LifeBuoy,
  RadioTower,
  ReceiptText,
  RefreshCw,
  ShieldCheck,
  Webhook,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

export const rootRoute = createRootRoute({
  component: ShellLayout,
});

function ShellLayout() {
  const { t } = useTranslation();

  const navItems = [
    { label: t("nav.overview"), icon: Gauge, active: true },
    { label: t("nav.apps"), icon: Boxes },
    { label: t("nav.orders"), icon: ClipboardList },
    { label: t("nav.channels"), icon: CreditCard },
    { label: t("nav.routing"), icon: GitBranch },
    { label: t("nav.webhooks"), icon: Webhook },
    { label: t("nav.refunds"), icon: RefreshCw },
    { label: t("nav.subscriptions"), icon: ReceiptText },
  ];

  return (
    <div className="min-h-[100dvh] bg-background text-foreground">
      <div className="grid min-h-[100dvh] lg:grid-cols-[264px_1fr]">
        <aside className="hidden border-r bg-card/60 lg:block">
          <div className="flex h-full flex-col">
            <div className="border-b px-5 py-4">
              <div className="flex items-center gap-3">
                <div className="flex size-9 items-center justify-center rounded-md bg-primary text-primary-foreground">
                  <ShieldCheck className="size-4" />
                </div>
                <div>
                  <div className="text-sm font-semibold">
                    {t("common.productName")}
                  </div>
                  <div className="text-xs text-muted-foreground">
                    {t("common.console")}
                  </div>
                </div>
              </div>
            </div>
            <nav className="flex flex-1 flex-col gap-1 p-3">
              {navItems.map((item) => (
                <a
                  key={item.label}
                  href="/"
                  aria-current={item.active ? "page" : undefined}
                  className={`flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors ${
                    item.active
                      ? "bg-secondary text-secondary-foreground"
                      : "text-muted-foreground hover:bg-secondary/70 hover:text-foreground"
                  }`}
                >
                  <item.icon className="size-4" />
                  {item.label}
                </a>
              ))}
            </nav>
            <div className="border-t p-4">
              <div className="rounded-md border bg-background p-3">
                <div className="flex items-center gap-2 text-sm font-medium">
                  <LifeBuoy className="size-4" />
                  {t("shell.runbook")}
                </div>
                <p className="mt-2 text-xs leading-5 text-muted-foreground">
                  {t("shell.runbookDetail")}
                </p>
              </div>
            </div>
          </div>
        </aside>

        <div className="min-w-0">
          <header className="sticky top-0 z-20 border-b bg-background/95 backdrop-blur">
            <div className="flex items-center justify-between gap-4 px-4 py-3 md:px-6">
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <RadioTower className="size-4 text-muted-foreground" />
                  <span className="truncate text-sm font-medium">
                    {t("shell.environment")}
                  </span>
                  <Badge variant="secondary">{t("common.healthy")}</Badge>
                </div>
                <p className="mt-1 hidden text-xs text-muted-foreground md:block">
                  {t("shell.window")}
                </p>
              </div>
              <div className="flex items-center gap-2">
                <Button variant="outline" size="sm">
                  <RefreshCw className="size-4" />
                  {t("shell.sync")}
                </Button>
                <Button
                  variant="outline"
                  size="icon-sm"
                  aria-label={t("common.alerts")}
                >
                  <Bell className="size-4" />
                </Button>
              </div>
            </div>
          </header>
          <main className="px-4 py-5 md:px-6">
            <Outlet />
          </main>
        </div>
      </div>
    </div>
  );
}
