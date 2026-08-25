import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Bell, MoonStar, RefreshCw, SunMedium } from "lucide-react";

import { useTheme } from "@/components/theme-provider";
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbList,
  BreadcrumbPage,
} from "@/components/ui/breadcrumb";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { SidebarTrigger } from "@/components/ui/sidebar";

export type HeaderProps = {
  title?: ReactNode;
  children?: ReactNode;
};

export function Header({ title, children }: HeaderProps) {
  const { t } = useTranslation();
  const { theme, setTheme } = useTheme();

  return (
    <header className="sticky top-0 z-20 flex h-16 shrink-0 items-center justify-between gap-4 border-b bg-background/95 px-4 backdrop-blur transition-[width,height] ease-linear group-has-data-[collapsible=icon]/sidebar-wrapper:h-12 md:px-6">
      <div className="flex min-w-0 items-center gap-3">
        <SidebarTrigger className="-ml-1" />
        <Separator
          orientation="vertical"
          className="mr-1 data-[orientation=vertical]:h-4"
        />
        {title ? (
          <Breadcrumb>
            <BreadcrumbList>
              <BreadcrumbItem>
                <BreadcrumbPage>{title}</BreadcrumbPage>
              </BreadcrumbItem>
            </BreadcrumbList>
          </Breadcrumb>
        ) : null}
      </div>
      <div className="flex items-center gap-2">
        {children}
        <Button
          variant="outline"
          size="icon-sm"
          aria-label={t("shell.toggleTheme")}
          onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
        >
          {theme === "dark" ? <SunMedium /> : <MoonStar />}
        </Button>
        <Button variant="outline" size="sm" className="hidden sm:inline-flex">
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
  );
}
