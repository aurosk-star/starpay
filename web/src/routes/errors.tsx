import { createRoute } from "@tanstack/react-router";

import { ErrorPage, type ErrorStatus } from "@/components/error-page";

import { rootRoute } from "./root";

export const errorsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/errors/$status",
  component: ErrorRoute,
});

function ErrorRoute() {
  const { status } = errorsRoute.useParams();
  const normalized = isErrorStatus(status) ? status : "404";
  return <ErrorPage status={normalized} />;
}

function isErrorStatus(status: string): status is ErrorStatus {
  return (
    status === "401" || status === "403" || status === "404" || status === "500"
  );
}
