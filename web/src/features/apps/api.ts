import { apiRequest } from "@/lib/api";

import type { GatewayApp, ManageAppPayload } from "./types";

export function listApps(accessToken: string) {
  return apiRequest<{ items: GatewayApp[] }>("/v1/admin/apps", {
    accessToken,
  });
}

export function createApp(accessToken: string, payload: ManageAppPayload) {
  return apiRequest<{ app: GatewayApp; app_secret: string }>("/v1/admin/apps", {
    method: "POST",
    accessToken,
    body: JSON.stringify(payload),
  });
}

export function updateApp(
  accessToken: string,
  id: number,
  payload: ManageAppPayload,
) {
  return apiRequest<{ app: GatewayApp }>(`/v1/admin/apps/${id}`, {
    method: "PUT",
    accessToken,
    body: JSON.stringify(payload),
  });
}

export function enableApp(accessToken: string, id: number) {
  return apiRequest<{ app: GatewayApp }>(`/v1/admin/apps/${id}/enable`, {
    method: "POST",
    accessToken,
  });
}

export function disableApp(accessToken: string, id: number) {
  return apiRequest<{ app: GatewayApp }>(`/v1/admin/apps/${id}/disable`, {
    method: "POST",
    accessToken,
  });
}

export function resetAppSecret(accessToken: string, id: number) {
  return apiRequest<{ app: GatewayApp; app_secret: string }>(
    `/v1/admin/apps/${id}/reset-secret`,
    {
      method: "POST",
      accessToken,
    },
  );
}
