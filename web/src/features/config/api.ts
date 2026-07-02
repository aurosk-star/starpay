import { apiRequest } from "@/lib/api";

import type {
  GatewayConfig,
  PublicSiteConfig,
  UpdateGatewayConfigPayload,
} from "./types";

export function getPublicSiteConfig() {
  return apiRequest<{ site_config: PublicSiteConfig }>("/v1/public/site-config");
}

export function getGatewayConfig(accessToken: string) {
  return apiRequest<{ gateway_config: GatewayConfig }>(
    "/v1/admin/config/gateway",
    {
      accessToken,
    },
  );
}

export function updateGatewayConfig(
  accessToken: string,
  payload: UpdateGatewayConfigPayload,
) {
  return apiRequest<{ gateway_config: GatewayConfig }>(
    "/v1/admin/config/gateway",
    {
      method: "PUT",
      accessToken,
      body: JSON.stringify(payload),
    },
  );
}
