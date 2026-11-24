import type { TFunction } from "i18next";
import {
  AppWindow,
  ClipboardCheck,
  Command,
  CreditCard,
  GitBranch,
  LayoutDashboard,
  PlayCircle,
  RotateCcw,
  Settings,
  ShoppingCart,
  Users,
  Webhook,
} from "lucide-react";

import { type SidebarData } from "../types";

/** Starpay navigation metadata shared by the sidebar and command menu. */
export function createSidebarData(t: TFunction): SidebarData {
  return {
    user: {
      name: t("common.admin"),
      email: t("common.adminEmail"),
      avatar: "/avatars/shadcn.jpg",
    },
    teams: [
      {
        name: t("common.productName"),
        logo: Command,
        plan: t("common.console"),
      },
    ],
    navGroups: [
      {
        title: t("nav.groups.workspace"),
        items: [
          {
            title: t("nav.overview"),
            url: "/",
            icon: LayoutDashboard,
          },
          {
            title: t("nav.users"),
            url: "/users",
            icon: Users,
          },
          {
            title: t("nav.apps"),
            url: "/apps",
            icon: AppWindow,
          },
        ],
      },
      {
        title: t("nav.groups.payments"),
        items: [
          {
            title: t("nav.orders"),
            url: "/orders",
            icon: ShoppingCart,
          },
          {
            title: t("nav.channels"),
            url: "/channels",
            icon: CreditCard,
          },
          {
            title: t("nav.routing"),
            url: "/routing",
            icon: GitBranch,
          },
          {
            title: t("nav.webhooks"),
            url: "/webhooks",
            icon: Webhook,
          },
          {
            title: t("nav.refunds"),
            url: "/refunds",
            icon: RotateCcw,
          },
          {
            title: t("nav.reconciliations"),
            url: "/reconciliations",
            icon: ClipboardCheck,
          },
          {
            title: t("nav.testPay"),
            url: "/test-pay",
            icon: PlayCircle,
          },
        ],
      },
      {
        title: t("nav.groups.platform"),
        items: [
          {
            title: t("nav.gatewayConfig"),
            url: "/config/gateway",
            icon: Settings,
          },
        ],
      },
    ],
  };
}
