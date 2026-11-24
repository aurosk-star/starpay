import type { PaymentChannel } from "@/features/channels/types";

export type RoutingPaymentMethod = PaymentChannel;
export type RoutingTerminal = "any" | "desktop" | "mobile" | "wechat_browser";
export type RoutingAppScope = "all" | "include";

export type RoutingTarget = {
  id: number;
  routing_rule_id: number;
  channel_account_id: number;
  enabled: boolean;
  priority: number;
  weight: number;
  created_at: string;
  updated_at: string;
};

export type RoutingRule = {
  id: number;
  name: string;
  enabled: boolean;
  priority: number;
  app_scope: RoutingAppScope;
  app_ids: string[];
  payment_method: RoutingPaymentMethod;
  pay_modes: string[];
  currency: string;
  min_amount: number;
  max_amount: number;
  terminal: RoutingTerminal;
  metadata: Record<string, unknown>;
  targets: RoutingTarget[];
  created_at: string;
  updated_at: string;
};

export type ManageRoutingTargetPayload = {
  channel_account_id: number;
  enabled?: boolean;
  priority: number;
  weight: number;
};

export type ManageRoutingRulePayload = {
  name: string;
  enabled?: boolean;
  priority: number;
  app_scope: RoutingAppScope;
  app_ids: string[];
  payment_method: RoutingPaymentMethod;
  pay_modes: string[];
  currency?: string;
  min_amount: number;
  max_amount: number;
  terminal: RoutingTerminal;
  metadata?: Record<string, unknown>;
  targets: ManageRoutingTargetPayload[];
};

export type RouteCandidate = {
  rule_id: number;
  target_id: number;
  channel_account_id: number;
  channel: string;
  payment_method: string;
  pay_mode: string;
  priority: number;
  target_priority: number;
};

export type RoutePreviewPayload = {
  app_id?: string;
  payment_method?: string;
  pay_mode?: string;
  amount: number;
  currency?: string;
  terminal?: string;
};
