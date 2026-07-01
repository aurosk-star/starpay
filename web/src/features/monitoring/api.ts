import { apiRequest } from "@/lib/api";

export type MonitorStatus = "ok" | "degraded";

export type ComponentStatus = {
  status: MonitorStatus;
  latency_ms: number;
  latency_us?: number;
  error?: string;
  checked_at: string;
  metadata?: Record<string, unknown>;
};

export type QueueStatus = {
  name: string;
  stream: string;
  group: string;
  status: MonitorStatus;
  length: number;
  pending: number;
  consumers: number;
  error?: string;
  checked_at: string;
};

export type RuntimeStatus = {
  status: MonitorStatus;
  go_version: string;
  goos: string;
  goarch: string;
  goroutines: number;
  alloc_bytes: number;
  checked_at: string;
};

export type MonitoringOverview = {
  database: ComponentStatus;
  redis: ComponentStatus;
  queues: QueueStatus[];
  runtime: RuntimeStatus;
};

export function getMonitoringOverview(accessToken: string) {
  return apiRequest<{ monitoring: MonitoringOverview }>(
    "/v1/admin/monitoring/overview",
    {
      accessToken,
    },
  );
}
