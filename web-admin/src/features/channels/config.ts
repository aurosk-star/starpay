import type { PaymentChannel } from "./types";

export type ChannelConfig = {
  app_id?: string;
  mch_id?: string;
  api_v3_key?: string;
  serial_no?: string;
  private_key?: string;
  cert?: string;
  wechat_pay_public_key?: string;
  wechat_pay_public_key_id?: string;
  alipay_public_key?: string;
  server_url?: string;
  product_code?: string;
  mode?: string;
  enable_native_pay?: string;
  enable_h5_pay?: string;
  enable_page_pay?: string;
  enable_wap_pay?: string;
  enable_qr_pay?: string;
  client_id?: string;
  client_secret?: string;
  webhook_id?: string;
  brand_name?: string;
  intent?: string;
  locale?: string;
};

export const emptyChannelConfig: Record<PaymentChannel, ChannelConfig> = {
  wechat: {
    app_id: "",
    mch_id: "",
    api_v3_key: "",
    serial_no: "",
    private_key: "",
    cert: "",
    wechat_pay_public_key: "",
    wechat_pay_public_key_id: "",
    mode: "native",
    enable_native_pay: "true",
    enable_h5_pay: "false",
  },
  alipay: {
    app_id: "",
    private_key: "",
    alipay_public_key: "",
    server_url: "",
    product_code: "FAST_INSTANT_TRADE_PAY",
    enable_page_pay: "true",
    enable_wap_pay: "true",
    enable_qr_pay: "true",
  },
  paypal: {
    client_id: "",
    client_secret: "",
    webhook_id: "",
    brand_name: "",
    intent: "CAPTURE",
    locale: "zh-CN",
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
  "webhook_id",
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

export function buildChangedConfigPayload(
  current: ChannelConfig,
  initial: ChannelConfig,
) {
  return Object.fromEntries(
    Object.entries(current).filter(([key, value]) => {
      const normalizedValue = String(value ?? "").trim();
      if (!normalizedValue || normalizedValue === "********") {
        return false;
      }
      const initialValue = String(
        initial[key as keyof ChannelConfig] ?? "",
      ).trim();
      return normalizedValue !== initialValue;
    }),
  );
}
