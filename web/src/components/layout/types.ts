import type { AdminUser } from "@/features/auth/types";
import type { SidebarData } from "@/config/navigation";

export type LayoutUser = Pick<AdminUser, "username" | "email" | "display_name">;

export type AppSidebarProps = {
  data?: SidebarData;
  pathname?: string;
};

export type NavUserProps = {
  user?: LayoutUser | null;
  onLogout?: () => void;
};
