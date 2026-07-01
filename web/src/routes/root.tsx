import {
  Link,
  createRootRoute,
  Outlet,
  useRouterState,
} from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import {
  Bell,
  Boxes,
  ChevronsUpDown,
  ClipboardList,
  CreditCard,
  Gauge,
  GitBranch,
  LogOut,
  MoonStar,
  ReceiptText,
  RefreshCw,
  ShieldCheck,
  SunMedium,
  Settings2,
  Webhook,
  Users,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { useTheme } from "@/components/theme-provider";
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbList,
  BreadcrumbPage,
} from "@/components/ui/breadcrumb";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Separator } from "@/components/ui/separator";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
  SidebarRail,
  SidebarTrigger,
  useSidebar,
} from "@/components/ui/sidebar";
import { logout } from "@/features/auth/api";
import { useAuthStore } from "@/features/auth/store";

export const rootRoute = createRootRoute({
  component: ShellLayout,
});

function ShellLayout() {
  const { t } = useTranslation();
  const accessToken = useAuthStore((state) => state.accessToken);
  const currentUser = useAuthStore((state) => state.user);
  const clearSession = useAuthStore((state) => state.clearSession);
  const { theme, setTheme } = useTheme();
  const pathname = useRouterState({
    select: (state) => state.location.pathname,
  });
  if (pathname.startsWith("/checkout/")) {
    return <Outlet />;
  }

  const navGroups: NavGroup[] = [
    {
      label: t("nav.groups.workspace"),
      items: [{ label: t("nav.overview"), icon: Gauge, to: "/" }],
    },
    {
      label: t("nav.groups.payments"),
      items: [
        { label: t("nav.orders"), icon: ClipboardList, to: "/orders" },
        { label: t("nav.webhooks"), icon: Webhook, to: "/webhooks" },
        { label: t("nav.refunds"), icon: RefreshCw, disabled: true },
        { label: t("nav.subscriptions"), icon: ReceiptText, disabled: true },
      ],
    },
    {
      label: t("nav.groups.platform"),
      items: [
        { label: t("nav.apps"), icon: Boxes, to: "/apps" },
        { label: t("nav.channels"), icon: CreditCard, to: "/channels" },
        { label: t("nav.routing"), icon: GitBranch, disabled: true },
        {
          label: t("nav.gatewayConfig"),
          icon: Settings2,
          to: "/config/gateway",
        },
        { label: t("nav.users"), icon: Users, to: "/users" },
      ],
    },
  ];

  function handleLogout() {
    if (accessToken) {
      void logout().finally(() => clearSession());
      return;
    }
    clearSession();
  }

  const currentPage =
    navGroups
      .flatMap((group) => group.items)
      .find((item) => isNavItemActive(item, pathname))?.label ??
    t("nav.overview");

  return (
    <SidebarProvider className="h-svh min-h-0 overflow-hidden">
      <AppSidebar
        navGroups={navGroups}
        pathname={pathname}
        user={currentUser}
        onLogout={handleLogout}
      />
      <SidebarInset className="min-h-0">
        <header className="sticky top-0 z-20 flex h-16 shrink-0 items-center justify-between gap-4 border-b bg-background/95 px-4 backdrop-blur transition-[width,height] ease-linear group-has-data-[collapsible=icon]/sidebar-wrapper:h-12 md:px-6">
          <div className="flex min-w-0 items-center gap-3">
            <SidebarTrigger className="-ml-1" />
            <Separator
              orientation="vertical"
              className="mr-1 data-[orientation=vertical]:h-4"
            />
            <Breadcrumb>
              <BreadcrumbList>
                <BreadcrumbItem>
                  <BreadcrumbPage>{currentPage}</BreadcrumbPage>
                </BreadcrumbItem>
              </BreadcrumbList>
            </Breadcrumb>
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="icon-sm"
              aria-label={t("shell.toggleTheme")}
              onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
            >
              {theme === "dark" ? <SunMedium /> : <MoonStar />}
            </Button>
            <Button
              variant="outline"
              size="sm"
              className="hidden sm:inline-flex"
            >
              <RefreshCw />
              {t("shell.sync")}
            </Button>
            <Button
              variant="outline"
              size="icon-sm"
              aria-label={t("common.alerts")}
            >
              <Bell />
            </Button>
          </div>
        </header>
        <ScrollArea className="min-h-0 flex-1">
          <div className="min-w-0 max-w-full px-3 py-4 sm:px-4 sm:py-5 md:px-6">
            <Outlet />
          </div>
        </ScrollArea>
      </SidebarInset>
    </SidebarProvider>
  );
}

function AppSidebar({
  navGroups,
  pathname,
  user,
  onLogout,
}: {
  navGroups: NavGroup[];
  pathname: string;
  user: { username: string; email: string; display_name?: string } | null;
  onLogout: () => void;
}) {
  const { t } = useTranslation();
  const { open } = useSidebar();

  return (
    <Sidebar collapsible="icon" className="relative">
      <SidebarHeader>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <SidebarMenuButton
              size="lg"
              className="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
            >
              <div className="flex size-8 items-center justify-center rounded-lg bg-sidebar-primary text-sidebar-primary-foreground">
                <ShieldCheck className="size-4" />
              </div>
              {open ? (
                <>
                  <div className="grid flex-1 text-left text-sm leading-tight">
                    <span className="truncate font-medium">
                      {t("common.productName")}
                    </span>
                    <span className="truncate text-xs text-sidebar-foreground/70">
                      {t("common.console")}
                    </span>
                  </div>
                  <ChevronsUpDown className="ml-auto" />
                </>
              ) : null}
            </SidebarMenuButton>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" side="right" className="w-56">
            <DropdownMenuLabel>{t("common.productName")}</DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem>{t("shell.environment")}</DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </SidebarHeader>
      <SidebarContent>
        {navGroups.map((group) => (
          <SidebarGroup key={group.label}>
            <SidebarGroupLabel>{group.label}</SidebarGroupLabel>
            <SidebarMenu>
              {group.items.map((item) => {
                const active = isNavItemActive(item, pathname);
                const itemClassName = item.disabled
                  ? "text-sidebar-foreground/40"
                  : "text-sidebar-foreground/75";

                return (
                  <SidebarMenuItem key={item.label}>
                    <SidebarMenuButton
                      asChild={Boolean(item.to)}
                      isActive={active}
                      aria-current={active ? "page" : undefined}
                      aria-disabled={item.disabled ? true : undefined}
                      className={itemClassName}
                      title={item.label}
                      tooltip={item.label}
                    >
                      {item.to ? (
                        <Link to={item.to}>
                          <item.icon />
                          {open ? <span>{item.label}</span> : null}
                        </Link>
                      ) : (
                        <>
                          <item.icon />
                          {open ? <span>{item.label}</span> : null}
                        </>
                      )}
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                );
              })}
            </SidebarMenu>
          </SidebarGroup>
        ))}
      </SidebarContent>
      <SidebarFooter>
        <UserMenu user={user} onLogout={onLogout} />
      </SidebarFooter>
      <SidebarRail />
    </Sidebar>
  );
}

type NavItem = {
  label: string;
  icon: LucideIcon;
  to?: string;
  disabled?: boolean;
};

type NavGroup = {
  label: string;
  items: NavItem[];
};

function isNavItemActive(item: NavItem, pathname: string) {
  if (!item.to) return false;
  if (item.to === "/") return pathname === "/";
  return pathname === item.to || pathname.startsWith(`${item.to}/`);
}

function UserMenu({
  user,
  onLogout,
}: {
  user: { username: string; email: string; display_name?: string } | null;
  onLogout: () => void;
}) {
  const { t } = useTranslation();
  const { isMobile, open } = useSidebar();
  const name = user?.display_name || user?.username || "admin";
  const email = user?.email || "admin@example.com";

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <SidebarMenuButton
          size="lg"
          className="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
        >
          <Avatar className="size-8 rounded-lg">
            <AvatarFallback className="rounded-lg">
              {name.slice(0, 2).toUpperCase()}
            </AvatarFallback>
          </Avatar>
          {open ? (
            <>
              <div className="grid flex-1 text-left text-sm leading-tight">
                <span className="truncate font-medium">{name}</span>
                <span className="truncate text-xs text-sidebar-foreground/70">
                  {email}
                </span>
              </div>
              <ChevronsUpDown className="ml-auto" />
            </>
          ) : null}
        </SidebarMenuButton>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        side={isMobile ? "bottom" : "right"}
        align="end"
        sideOffset={4}
        className="w-(--radix-dropdown-menu-trigger-width) min-w-56 rounded-lg"
      >
        <DropdownMenuLabel className="p-0 font-normal">
          <div className="flex items-center gap-2 px-1 py-1.5 text-left text-sm">
            <Avatar className="size-8 rounded-lg">
              <AvatarFallback className="rounded-lg">
                {name.slice(0, 2).toUpperCase()}
              </AvatarFallback>
            </Avatar>
            <div className="grid flex-1 text-left text-sm leading-tight">
              <span className="truncate font-medium">{name}</span>
              <span className="truncate text-xs text-muted-foreground">
                {email}
              </span>
            </div>
          </div>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuGroup>
          <DropdownMenuItem onClick={onLogout}>
            <LogOut />
            {t("shell.logout")}
          </DropdownMenuItem>
        </DropdownMenuGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
