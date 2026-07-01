export type GatewayApp = {
  id: number;
  app_id: string;
  name: string;
  notify_url?: string;
  default_return_url?: string;
  allowed_ips: string[];
  status: string;
  created_at: string;
  updated_at: string;
};

export type ManageAppPayload = {
  app_id?: string;
  name: string;
  notify_url?: string;
  default_return_url?: string;
  allowed_ips: string[];
  status: string;
};
