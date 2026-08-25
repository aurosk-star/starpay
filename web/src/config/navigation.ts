import type { TFunction } from "i18next";
import {
  Boxes,
  ClipboardList,
  CreditCard,
  Gauge,
  GitBranch,
  ReceiptText,
  RefreshCw,
  Settings2,
  Users,
  Webhook,
  type LucideIcon,
} from "lucide-react";

export type NavItem = {
  titleKey: string;
  url: string;
  icon: LucideIcon;
  disabled?: boolean;
  /** Optional custom matcher for detail routes or aliases. */
  match?: (pathname: string) => boolean;
};

export type NavGroup = {
  titleKey: string;
  items: NavItem[];
};

export type SidebarData = {
  groups: NavGroup[];
};

/** Untranslated metadata consumed by shell components and command menus. */
export const sidebarData: SidebarData = {
  groups: [
    {
      titleKey: "nav.groups.workspace",
      items: [{ titleKey: "nav.overview", icon: Gauge, url: "/" }],
    },
    {
      titleKey: "nav.groups.payments",
      items: [
        { titleKey: "nav.orders", icon: ClipboardList, url: "/orders" },
        { titleKey: "nav.webhooks", icon: Webhook, url: "/webhooks" },
        { titleKey: "nav.refunds", icon: RefreshCw, url: "/refunds" },
        { titleKey: "testPay.title", icon: CreditCard, url: "/test-pay" },
        {
          titleKey: "nav.reconciliations",
          icon: RefreshCw,
          url: "/reconciliations",
        },
        {
          titleKey: "nav.subscriptions",
          icon: ReceiptText,
          url: "/subscriptions",
          disabled: true,
        },
      ],
    },
    {
      titleKey: "nav.groups.platform",
      items: [
        { titleKey: "nav.apps", icon: Boxes, url: "/apps" },
        { titleKey: "nav.channels", icon: CreditCard, url: "/channels" },
        { titleKey: "nav.routing", icon: GitBranch, url: "/routing" },
        {
          titleKey: "nav.gatewayConfig",
          icon: Settings2,
          url: "/config/gateway",
        },
        { titleKey: "nav.users", icon: Users, url: "/users" },
      ],
    },
  ],
};

export function getNavigationItems(t: TFunction): SidebarData {
  return {
    groups: sidebarData.groups.map((group) => ({
      ...group,
      titleKey: t(group.titleKey),
      items: group.items.map((item) => ({
        ...item,
        titleKey: t(item.titleKey),
      })),
    })),
  };
}

export const navigationUrls = sidebarData.groups.flatMap((group) =>
  group.items.map((item) => item.url),
);

export function isNavItemActive(item: NavItem, pathname: string) {
  if (item.match) return item.match(pathname);
  if (item.url === "/") return pathname === "/";
  return pathname === item.url || pathname.startsWith(`${item.url}/`);
}
