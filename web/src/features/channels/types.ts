export type PaymentChannel = "wechat" | "alipay" | "paypal";

export type ChannelEnv = "sandbox" | "prod";

export type ChannelAccount = {
  id: number;
  channel: PaymentChannel;
  name: string;
  enabled: boolean;
  env: ChannelEnv;
  config: Record<string, unknown>;
  created_at: string;
  updated_at: string;
};

export type ManageChannelAccountPayload = {
  channel: PaymentChannel;
  name: string;
  enabled?: boolean;
  env: ChannelEnv;
  config: Record<string, unknown>;
};
