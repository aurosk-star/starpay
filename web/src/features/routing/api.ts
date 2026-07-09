import { apiRequest } from "@/lib/api";

import type { ManageRoutingRulePayload, RoutingRule } from "./types";

export function listRoutingRules(accessToken: string) {
  return apiRequest<{ items: RoutingRule[] }>("/v1/admin/routing-rules", {
    accessToken,
  });
}

export function getRoutingRule(accessToken: string, id: number) {
  return apiRequest<{ routing_rule: RoutingRule }>(
    `/v1/admin/routing-rules/${id}`,
    { accessToken },
  );
}

export function createRoutingRule(
  accessToken: string,
  payload: ManageRoutingRulePayload,
) {
  return apiRequest<{ routing_rule: RoutingRule }>("/v1/admin/routing-rules", {
    method: "POST",
    accessToken,
    body: JSON.stringify(payload),
  });
}

export function updateRoutingRule(
  accessToken: string,
  id: number,
  payload: ManageRoutingRulePayload,
) {
  return apiRequest<{ routing_rule: RoutingRule }>(
    `/v1/admin/routing-rules/${id}`,
    {
      method: "PUT",
      accessToken,
      body: JSON.stringify(payload),
    },
  );
}

export function enableRoutingRule(accessToken: string, id: number) {
  return apiRequest<{ routing_rule: RoutingRule }>(
    `/v1/admin/routing-rules/${id}/enable`,
    { method: "POST", accessToken },
  );
}

export function disableRoutingRule(accessToken: string, id: number) {
  return apiRequest<{ routing_rule: RoutingRule }>(
    `/v1/admin/routing-rules/${id}/disable`,
    { method: "POST", accessToken },
  );
}
