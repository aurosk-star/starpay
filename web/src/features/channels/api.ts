import { apiRequest } from "@/lib/api";

import type { ChannelAccount, ManageChannelAccountPayload } from "./types";

export function listChannelAccounts(accessToken: string) {
  return apiRequest<{ items: ChannelAccount[] }>("/v1/admin/channels", {
    accessToken,
  });
}

export function getChannelAccount(accessToken: string, id: number) {
  return apiRequest<{ channel_account: ChannelAccount }>(
    `/v1/admin/channels/${id}`,
    {
      accessToken,
    },
  );
}

export function createChannelAccount(
  accessToken: string,
  payload: ManageChannelAccountPayload,
) {
  return apiRequest<{ channel_account: ChannelAccount }>("/v1/admin/channels", {
    method: "POST",
    accessToken,
    body: JSON.stringify(payload),
  });
}

export function updateChannelAccount(
  accessToken: string,
  id: number,
  payload: ManageChannelAccountPayload,
) {
  return apiRequest<{ channel_account: ChannelAccount }>(
    `/v1/admin/channels/${id}`,
    {
      method: "PUT",
      accessToken,
      body: JSON.stringify(payload),
    },
  );
}

export function enableChannelAccount(accessToken: string, id: number) {
  return apiRequest<{ channel_account: ChannelAccount }>(
    `/v1/admin/channels/${id}/enable`,
    {
      method: "POST",
      accessToken,
    },
  );
}

export function disableChannelAccount(accessToken: string, id: number) {
  return apiRequest<{ channel_account: ChannelAccount }>(
    `/v1/admin/channels/${id}/disable`,
    {
      method: "POST",
      accessToken,
    },
  );
}
