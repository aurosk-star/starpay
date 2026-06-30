import { createRoute } from "@tanstack/react-router";

import { UsersPage } from "@/features/users/users-page";

import { rootRoute } from "./root";

export const usersRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/users",
  component: UsersPage,
});
