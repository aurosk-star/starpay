import { Outlet } from "@tanstack/react-router";
import type { ReactNode } from "react";

import { ScrollArea } from "@/components/ui/scroll-area";
import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar";

import { AppSidebar } from "./app-sidebar";
import { Header } from "./header";
import { Main } from "./main";
import { SkipToMain } from "@/components/skip-to-main";

export type AuthenticatedLayoutProps = {
  title?: ReactNode;
  headerActions?: ReactNode;
  children?: ReactNode;
};

export function AuthenticatedLayout({
  title,
  headerActions,
  children,
}: AuthenticatedLayoutProps) {
  return (
    <SidebarProvider className="h-svh min-h-0 overflow-hidden">
      <SkipToMain />
      <AppSidebar />
      <SidebarInset className="min-h-0">
        <Header title={title}>{headerActions}</Header>
        <ScrollArea className="min-h-0 flex-1">
          <Main>{children ?? <Outlet />}</Main>
        </ScrollArea>
      </SidebarInset>
    </SidebarProvider>
  );
}
