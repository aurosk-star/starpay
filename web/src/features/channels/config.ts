import type { PaymentChannel } from "./types";

export type ChannelConfig = {
  app_id?: string;
  mch_id?: string;
  api_v3_key?: string;
  serial_no?: string;
  private_key?: string;
  alipay_public_key?: string;
  server_url?: string;
  client_id?: string;
  client_secret?: string;
};

export const emptyChannelConfig: Record<PaymentChannel, ChannelConfig> = {
  wechat: {
    app_id: "",
    mch_id: "",
    api_v3_key: "",
    serial_no: "",
    private_key: "",
  },
  alipay: {
    app_id: "",
    private_key: "",
    alipay_public_key: "",
    server_url: "",
  },
  paypal: {
    client_id: "",
    client_secret: "",
  },
};

export const sensitiveChannelConfigKeys = new Set([
  "api_key",
  "api_v3_key",
  "secret",
  "client_secret",
  "private_key",
  "alipay_public_key",
  "wechat_pay_public_key",
  "cert",
  "cert_key",
]);

export function normalizeConfig(
  channel: PaymentChannel,
  config: Record<string, unknown>,
): ChannelConfig {
  return {
    ...emptyChannelConfig[channel],
    ...Object.fromEntries(
      Object.entries(config).map(([key, value]) => [key, String(value ?? "")]),
    ),
  };
}

export function buildConfigPayload(config: ChannelConfig) {
  return Object.fromEntries(
    Object.entries(config).filter(([, value]) => String(value ?? "").trim()),
  );
}
