export type GatewayConfig = {
  id: number;
  site_name: string;
  gateway_base_url: string;
  payment_notify_path: string;
  default_currency: string;
  default_locale: string;
  request_id_enabled: boolean;
  maintenance_mode: boolean;
  extra: Record<string, unknown>;
  created_at: string;
  updated_at: string;
};

export type UpdateGatewayConfigPayload = {
  site_name: string;
  gateway_base_url: string;
  default_currency: string;
  default_locale: string;
  request_id_enabled: boolean;
  maintenance_mode: boolean;
  extra: Record<string, unknown>;
};

export type PublicSiteConfig = {
  site_name: string;
  default_locale: string;
};
