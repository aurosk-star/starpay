import { useRouterState, Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { ChevronsUpDown, ShieldCheck } from "lucide-react";

import {
  getNavigationItems,
  isNavItemActive,
  sidebarData,
} from "@/config/navigation";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
  useSidebar,
} from "@/components/ui/sidebar";
import { useAuthStore } from "@/features/auth/store";

import { NavUser } from "./nav-user";
import type { AppSidebarProps } from "./types";

export function AppSidebar({ data, pathname: pathnameProp }: AppSidebarProps) {
  const { t } = useTranslation();
  const { open, setOpenMobile } = useSidebar();
  const pathname = useRouterState({
    select: (state) => state.location.pathname,
  });
  const navigation = data ?? getNavigationItems(t);
  const currentPath = pathnameProp ?? pathname;
  const user = useAuthStore((state) => state.user);

  return (
    <Sidebar collapsible="icon" className="relative">
      <SidebarHeader>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <SidebarMenuButton
              size="lg"
              aria-label={t("common.productName")}
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
        {navigation.groups.map((group) => (
          <SidebarGroup key={group.titleKey}>
            <SidebarGroupLabel>{group.titleKey}</SidebarGroupLabel>
            <SidebarMenu>
              {group.items.map((item) => {
                const active = isNavItemActive(item, currentPath);
                const itemClassName = item.disabled
                  ? "text-sidebar-foreground/40"
                  : "text-sidebar-foreground/75";

                return (
                  <SidebarMenuItem key={item.url}>
                    <SidebarMenuButton
                      asChild={Boolean(item.url) && !item.disabled}
                      isActive={active}
                      aria-current={active ? "page" : undefined}
                      aria-disabled={item.disabled ? true : undefined}
                      className={itemClassName}
                      title={item.titleKey}
                      tooltip={item.titleKey}
                    >
                      {item.disabled ? (
                        <>
                          <item.icon />
                          {open ? <span>{item.titleKey}</span> : null}
                        </>
                      ) : (
                        <Link to={item.url} onClick={() => setOpenMobile(false)}>
                          <item.icon />
                          {open ? <span>{item.titleKey}</span> : null}
                        </Link>
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
        <NavUser user={user} />
      </SidebarFooter>
      <SidebarRail />
    </Sidebar>
  );
}

export { sidebarData };
