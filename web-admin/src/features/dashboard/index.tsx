import { useCallback, useEffect, useMemo, useState } from "react";
import { Link } from "@tanstack/react-router";
import { Bar, BarChart, ResponsiveContainer, XAxis, YAxis } from "recharts";
import {
  ArrowUpRight,
  AlertTriangle,
  CheckCircle2,
  Clock3,
  CreditCard,
  Database,
  RefreshCw,
  Server,
  TimerReset,
  Webhook,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useTranslation } from "react-i18next";

import { ConfigDrawer } from "@/components/config-drawer";
import { Header } from "@/components/layout/header";
import { Main } from "@/components/layout/main";
import { PageHeader } from "@/components/page-header";
import { ProfileDropdown } from "@/components/profile-dropdown";
import { Search } from "@/components/search";
import { ThemeSwitch } from "@/components/theme-switch";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { TopNav } from "@/components/layout/top-nav";
import { listChannelAccounts } from "@/features/channels/api";
import type { ChannelAccount } from "@/features/channels/types";
import { useAuthStore } from "@/features/auth/store";
import { getMonitoringOverview } from "@/features/monitoring/api";
import type { MonitoringOverview } from "@/features/monitoring/api";
import { listOrders } from "@/features/orders/api";
import type { PaymentOrder } from "@/features/orders/types";
import { listWebhookDeliveries } from "@/features/webhooks/api";
import type { WebhookDelivery } from "@/features/webhooks/types";
import { APIError } from "@/lib/api";
import { formatDateTime } from "@/lib/date";
import { formatMinorAmount } from "@/lib/money";
import { toast } from "sonner";

type MonthPoint = { name: string; total: number };

export function Dashboard() {
  const { t } = useTranslation();
  const accessToken = useAuthStore((state) => state.accessToken);
  const [orders, setOrders] = useState<PaymentOrder[]>([]);
  const [orderTotal, setOrderTotal] = useState(0);
  const [channels, setChannels] = useState<ChannelAccount[]>([]);
  const [deliveries, setDeliveries] = useState<WebhookDelivery[]>([]);
  const [webhookTotal, setWebhookTotal] = useState(0);
  const [monitoring, setMonitoring] = useState<MonitoringOverview | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    if (!accessToken) return;
    setLoading(true);
    try {
      const [ordersResult, channelsResult, deliveriesResult, monitoringResult] =
        await Promise.all([
          listOrders(accessToken, { page: 1, page_size: 100 }),
          listChannelAccounts(accessToken),
          listWebhookDeliveries(accessToken, { page: 1, page_size: 100 }),
          getMonitoringOverview(accessToken),
        ]);
      setOrders(ordersResult.items);
      setOrderTotal(ordersResult.total);
      setChannels(channelsResult.items);
      setDeliveries(deliveriesResult.items);
      setWebhookTotal(deliveriesResult.total);
      setMonitoring(monitoringResult.monitoring);
    } catch (error) {
      toast.error(
        error instanceof APIError ? error.message : t("home.loadFailed"),
      );
    } finally {
      setLoading(false);
    }
  }, [accessToken, t]);

  useEffect(() => {
    void load();
  }, [load]);

  const paidOrders = orders.filter((order) => order.status === "paid");
  const pendingOrders = orders.filter((order) => order.status === "pending");
  const failedDeliveries = deliveries.filter(
    (delivery) => delivery.status === "failed",
  );
  const pendingDeliveries = deliveries.filter(
    (delivery) => delivery.status === "pending",
  );
  const activeChannels = channels.filter((channel) => channel.enabled).length;
  const completedOrders = orders.filter(
    (order) => order.status === "paid" || order.status === "failed",
  );
  const authorizationRate = formatRate(
    paidOrders.length,
    completedOrders.length,
  );
  const failedWebhooks = failedDeliveries.length;
  const monthData = useMemo(() => buildMonthData(paidOrders), [paidOrders]);

  return (
    <>
      <Header fixed>
        <TopNav links={topNav(t)} className="me-auto" />
        <Search />
        <ThemeSwitch />
        <ConfigDrawer />
        <ProfileDropdown />
      </Header>

      <Main fixed className="flex min-w-0 flex-1 flex-col gap-4 sm:gap-6">
        <PageHeader
          title={t("home.gatewayOverview")}
          description={t("home.description")}
          actions={
            <>
              <Button
                variant="outline"
                onClick={() => void load()}
                disabled={loading}
              >
                <RefreshCw
                  data-icon="inline-start"
                  className={loading ? "animate-spin" : undefined}
                />
                {t("common.refresh")}
              </Button>
              <Button asChild>
                <Link to="/test-pay">
                  {t("home.testPayment.button")}
                  <ArrowUpRight data-icon="inline-end" />
                </Link>
              </Button>
            </>
          }
        />

        <Tabs defaultValue="overview" className="flex flex-col gap-4">
          <div className="w-full overflow-x-auto pb-2">
            <TabsList>
              <TabsTrigger value="overview">
                {t("home.tabs.overview")}
              </TabsTrigger>
              <TabsTrigger value="analytics">
                {t("home.tabs.analytics")}
              </TabsTrigger>
              <TabsTrigger value="reports" disabled>
                {t("home.tabs.reports")}
              </TabsTrigger>
              <TabsTrigger value="notifications" disabled>
                {t("home.tabs.notifications")}
              </TabsTrigger>
            </TabsList>
          </div>

          <TabsContent value="overview" className="flex flex-col gap-4">
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <MetricCard
                title={t("home.metrics.paidVolume")}
                value={loading ? "-" : formatPaidVolume(paidOrders)}
                detail={t("home.metrics.paidVolumeDetail", {
                  count: paidOrders.length,
                })}
                icon={<CreditCard className="size-4 text-muted-foreground" />}
              />
              <MetricCard
                title={t("home.metrics.authorizationRate")}
                value={loading ? "-" : authorizationRate}
                detail={t("home.metrics.authorizationRateDetail", {
                  count: orders.length,
                })}
                icon={<Server className="size-4 text-muted-foreground" />}
              />
              <MetricCard
                title={t("home.metrics.pendingOrders")}
                value={loading ? "-" : String(pendingOrders.length)}
                detail={t("home.metrics.pendingOrdersDetail", {
                  count: pendingOrders.length,
                })}
                icon={<CreditCard className="size-4 text-muted-foreground" />}
              />
              <MetricCard
                title={t("home.metrics.webhookBacklog")}
                value={
                  loading
                    ? "-"
                    : String(failedWebhooks + pendingDeliveries.length)
                }
                detail={t("home.metrics.webhookBacklogDetail", {
                  failed: failedWebhooks,
                  pending: pendingDeliveries.length,
                })}
                icon={<Webhook className="size-4 text-muted-foreground" />}
              />
            </div>

            <div className="grid grid-cols-1 gap-4 lg:grid-cols-7">
              <Card className="col-span-1 lg:col-span-4">
                <CardHeader>
                  <CardTitle>{t("home.overviewChart.title")}</CardTitle>
                  <CardDescription>
                    {t("home.overviewChart.description")}
                  </CardDescription>
                </CardHeader>
                <CardContent className="ps-2">
                  <OverviewChart data={monthData} />
                </CardContent>
              </Card>
              <Card className="col-span-1 lg:col-span-3">
                <CardHeader>
                  <CardTitle>{t("home.recentSales.title")}</CardTitle>
                  <CardDescription>
                    {t("home.recentSales.description", {
                      count: paidOrders.length,
                    })}
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <RecentSales orders={paidOrders.slice(0, 5)} />
                </CardContent>
              </Card>
            </div>

            <div className="grid min-w-0 gap-4 lg:grid-cols-[minmax(0,1fr)_360px]">
              <Card className="min-w-0">
                <CardHeader>
                  <CardDescription>
                    {t("home.monitor.description")}
                  </CardDescription>
                  <CardTitle>{t("home.operationalChecks")}</CardTitle>
                </CardHeader>
                <CardContent className="flex flex-col gap-3">
                  <QueueItem
                    icon={Database}
                    title={t("home.monitor.database")}
                    detail={
                      monitoring
                        ? formatComponentStatus(t, monitoring.database)
                        : t("common.loading")
                    }
                    tone={
                      monitoring?.database.status === "degraded"
                        ? "warning"
                        : undefined
                    }
                  />
                  <QueueItem
                    icon={Server}
                    title={t("home.monitor.redis")}
                    detail={
                      monitoring
                        ? formatComponentStatus(t, monitoring.redis)
                        : t("common.loading")
                    }
                    tone={
                      monitoring?.redis.status === "degraded"
                        ? "warning"
                        : undefined
                    }
                  />
                  <QueueItem
                    icon={TimerReset}
                    title={t("home.monitor.workers")}
                    detail={
                      monitoring
                        ? t("home.monitor.workersDetail", {
                            streams: monitoring.queues.length,
                            length: monitoring.queues.reduce(
                              (sum, queue) => sum + queue.length,
                              0,
                            ),
                            pending: monitoring.queues.reduce(
                              (sum, queue) => sum + queue.pending,
                              0,
                            ),
                          })
                        : t("common.loading")
                    }
                    tone={
                      monitoring?.queues.some(
                        (queue) => queue.status === "degraded",
                      )
                        ? "warning"
                        : undefined
                    }
                  />
                </CardContent>
              </Card>

              <Card className="min-w-0">
                <CardHeader>
                  <CardDescription>{t("home.routingStatus")}</CardDescription>
                  <CardTitle>{t("home.channelHealth")}</CardTitle>
                </CardHeader>
                <CardContent className="flex flex-col gap-4">
                  <HealthBar
                    label={t("home.overviewCharts.enabledChannels")}
                    value={activeChannels}
                    max={Math.max(channels.length, 1)}
                  />
                  <HealthBar
                    label={t("home.overviewCharts.disabledChannels")}
                    value={channels.length - activeChannels}
                    max={Math.max(channels.length, 1)}
                    tone="muted"
                  />
                  {channels.length === 0 && (
                    <p className="text-sm text-muted-foreground">
                      {t("home.empty.channels")}
                    </p>
                  )}
                </CardContent>
              </Card>
            </div>

            <div className="grid min-w-0 gap-4 lg:grid-cols-[minmax(0,1fr)_360px]">
              <Card className="min-w-0">
                <CardHeader className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                  <div className="min-w-0">
                    <CardDescription>{t("home.liveOrders")}</CardDescription>
                    <CardTitle>{t("home.recentPaymentOrders")}</CardTitle>
                  </div>
                  <Button
                    variant="outline"
                    size="sm"
                    className="w-full sm:w-auto"
                    asChild
                  >
                    <Link to="/orders">{t("home.openOrders")}</Link>
                  </Button>
                </CardHeader>
                <CardContent>
                  <RecentOrders
                    orders={orders.slice(0, 6)}
                    loading={loading}
                    emptyText={t("orders.empty")}
                  />
                </CardContent>
              </Card>

              <Card className="min-w-0">
                <CardHeader>
                  <CardDescription>{t("home.compensation")}</CardDescription>
                  <CardTitle>{t("home.queryJobs")}</CardTitle>
                </CardHeader>
                <CardContent className="flex flex-col gap-4">
                  <SnapshotItem
                    icon={Clock3}
                    title={t("home.jobs.orderSnapshot")}
                    detail={t("home.jobs.orderSnapshotDetail", {
                      count: orderTotal,
                    })}
                    status={t("common.clean")}
                  />
                  <SnapshotItem
                    icon={Webhook}
                    title={t("home.jobs.webhookSnapshot")}
                    detail={t("home.jobs.webhookSnapshotDetail", {
                      count: webhookTotal,
                    })}
                    status={t("common.running")}
                  />
                </CardContent>
              </Card>
            </div>

            <Card className="min-w-0">
              <CardHeader>
                <CardDescription>{t("home.webhookCenter")}</CardDescription>
                <CardTitle>{t("home.deliveryQueue")}</CardTitle>
              </CardHeader>
              <CardContent>
                <WebhookSummary
                  deliveries={deliveries.slice(0, 6)}
                  loading={loading}
                />
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="analytics" className="flex flex-col gap-4">
            <Analytics
              orders={orders}
              channels={channels}
              deliveries={deliveries}
            />
          </TabsContent>
        </Tabs>
      </Main>
    </>
  );
}

function MetricCard({
  title,
  value,
  detail,
  icon,
}: {
  title: string;
  value: string;
  detail: string;
  icon: React.ReactNode;
}) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between gap-0 pb-2">
        <CardTitle className="text-sm font-medium">{title}</CardTitle>
        {icon}
      </CardHeader>
      <CardContent>
        <div className="text-2xl font-bold">{value}</div>
        <p className="text-xs text-muted-foreground">{detail}</p>
      </CardContent>
    </Card>
  );
}

function QueueItem({
  icon: Icon,
  title,
  detail,
  tone,
}: {
  icon: LucideIcon;
  title: string;
  detail: string;
  tone?: "warning";
}) {
  return (
    <Card className="rounded-md shadow-none">
      <CardContent className="flex gap-3 py-3">
        <div
          className={
            tone === "warning"
              ? "mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-md bg-destructive/10 text-destructive"
              : "mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-md bg-secondary text-muted-foreground"
          }
        >
          <Icon className="size-4" />
        </div>
        <div className="min-w-0">
          <p className="text-sm font-medium">{title}</p>
          <p className="mt-1 break-words text-xs leading-5 text-muted-foreground">
            {detail}
          </p>
        </div>
      </CardContent>
    </Card>
  );
}

function HealthBar({
  label,
  value,
  max,
  tone = "primary",
}: {
  label: string;
  value: number;
  max: number;
  tone?: "primary" | "muted";
}) {
  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center justify-between gap-4 text-sm">
        <span>{label}</span>
        <span className="font-mono text-xs text-muted-foreground">{value}</span>
      </div>
      <div className="h-2.5 w-full rounded-full bg-muted">
        <div
          className={
            tone === "muted"
              ? "h-2.5 rounded-full bg-secondary"
              : "h-2.5 rounded-full bg-primary"
          }
          style={{ width: `${Math.round((value / max) * 100)}%` }}
        />
      </div>
    </div>
  );
}

function RecentOrders({
  orders,
  loading,
  emptyText,
}: {
  orders: PaymentOrder[];
  loading: boolean;
  emptyText: string;
}) {
  const { t } = useTranslation();
  if (loading)
    return (
      <p className="text-sm text-muted-foreground">{t("common.loading")}</p>
    );
  if (orders.length === 0)
    return <p className="text-sm text-muted-foreground">{emptyText}</p>;
  return (
    <div className="flex flex-col divide-y">
      {orders.map((order) => (
        <Link
          key={order.id}
          to="/orders/$orderId"
          params={{ orderId: String(order.id) }}
          className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0 transition-colors hover:bg-muted/40 sm:flex-row sm:items-center sm:justify-between sm:gap-4"
        >
          <div className="min-w-0">
            <p className="truncate font-mono text-xs font-medium">
              {order.gateway_order_no}
            </p>
            <p className="mt-1 truncate text-sm">{order.subject}</p>
            <p className="mt-1 text-xs text-muted-foreground">
              {order.app_id} · {formatDateTime(order.created_at)}
            </p>
          </div>
          <div className="flex items-center justify-between gap-3 sm:justify-end">
            <StatusBadge status={order.status} />
            <span className="font-mono text-sm font-medium">
              {formatMinorAmount(order.amount, order.currency)}
            </span>
          </div>
        </Link>
      ))}
    </div>
  );
}

function SnapshotItem({
  icon: Icon,
  title,
  detail,
  status,
}: {
  icon: LucideIcon;
  title: string;
  detail: string;
  status: string;
}) {
  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div className="flex min-w-0 items-center gap-3">
        <Icon className="size-4 text-muted-foreground" />
        <div className="min-w-0">
          <p className="text-sm font-medium">{title}</p>
          <p className="text-xs leading-5 text-muted-foreground">{detail}</p>
        </div>
      </div>
      <Badge variant="outline" className="w-fit">
        {status}
      </Badge>
    </div>
  );
}

function WebhookSummary({
  deliveries,
  loading,
}: {
  deliveries: WebhookDelivery[];
  loading: boolean;
}) {
  const { t } = useTranslation();
  if (loading)
    return (
      <p className="text-sm text-muted-foreground">{t("common.loading")}</p>
    );
  if (deliveries.length === 0)
    return (
      <p className="text-sm text-muted-foreground">
        {t("home.empty.webhooks")}
      </p>
    );
  return (
    <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
      {deliveries.map((delivery) => (
        <Link
          key={delivery.id}
          to="/webhooks/$deliveryId"
          params={{ deliveryId: String(delivery.id) }}
          className="flex min-w-0 flex-col gap-2 rounded-md border p-3 transition-colors hover:bg-muted/40"
        >
          <div className="flex items-center justify-between gap-3">
            <span className="truncate font-mono text-xs font-medium">
              {delivery.delivery_no}
            </span>
            <StatusBadge status={delivery.status} />
          </div>
          <p className="truncate text-sm">{delivery.event_type}</p>
          <p className="truncate text-xs text-muted-foreground">
            {delivery.app_id} · {formatDateTime(delivery.created_at)}
          </p>
        </Link>
      ))}
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const { t } = useTranslation();
  const label =
    status === "paid"
      ? t("orders.status.paid")
      : status === "failed"
        ? t("orders.status.failed")
        : status === "pending"
          ? t("orders.status.pending")
          : status === "succeeded"
            ? t("webhooks.status.succeeded")
            : status;
  if (status === "paid" || status === "succeeded") {
    return (
      <Badge variant="secondary" className="gap-1">
        <CheckCircle2 className="size-3" />
        {label}
      </Badge>
    );
  }
  if (status === "failed") {
    return (
      <Badge variant="destructive" className="gap-1">
        <AlertTriangle className="size-3" />
        {label}
      </Badge>
    );
  }
  return <Badge variant="outline">{label}</Badge>;
}

function OverviewChart({ data }: { data: MonthPoint[] }) {
  return (
    <ResponsiveContainer width="100%" height={350}>
      <BarChart data={data}>
        <XAxis
          dataKey="name"
          stroke="#888888"
          fontSize={12}
          tickLine={false}
          axisLine={false}
        />
        <YAxis
          stroke="#888888"
          fontSize={12}
          tickLine={false}
          axisLine={false}
          tickFormatter={(value) => `¥${value}`}
        />
        <Bar
          dataKey="total"
          fill="currentColor"
          radius={[4, 4, 0, 0]}
          className="fill-primary"
        />
      </BarChart>
    </ResponsiveContainer>
  );
}

function RecentSales({ orders }: { orders: PaymentOrder[] }) {
  const { t } = useTranslation();
  if (orders.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">
        {t("home.recentSales.empty")}
      </p>
    );
  }
  return (
    <div className="flex flex-col gap-8">
      {orders.map((order) => (
        <div key={order.id} className="flex items-center gap-4">
          <Avatar className="size-9">
            <AvatarFallback>{order.currency.slice(0, 2)}</AvatarFallback>
          </Avatar>
          <div className="flex flex-1 flex-wrap items-center justify-between gap-2">
            <div className="flex flex-col gap-1">
              <p className="text-sm leading-none font-medium">
                {order.subject}
              </p>
              <p className="text-sm text-muted-foreground">{order.app_id}</p>
            </div>
            <div className="font-medium">
              +{formatMinorAmount(order.amount, order.currency)}
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

function Analytics({
  orders,
  channels,
  deliveries,
}: {
  orders: PaymentOrder[];
  channels: ChannelAccount[];
  deliveries: WebhookDelivery[];
}) {
  const { t } = useTranslation();
  const paid = orders.filter((order) => order.status === "paid").length;
  const failed = orders.filter((order) => order.status === "failed").length;
  const pending = orders.filter((order) => order.status === "pending").length;
  const max = Math.max(paid, failed, pending, 1);
  const items = [
    { name: t("home.analytics.paidOrders"), value: paid },
    { name: t("home.analytics.pendingOrders"), value: pending },
    { name: t("home.analytics.failedOrders"), value: failed },
    {
      name: t("home.analytics.enabledChannels"),
      value: channels.filter((channel) => channel.enabled).length,
    },
    {
      name: t("home.analytics.failedWebhooks"),
      value: deliveries.filter((delivery) => delivery.status === "failed")
        .length,
    },
  ];
  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader>
          <CardTitle>{t("home.analytics.title")}</CardTitle>
          <CardDescription>{t("home.analytics.description")}</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          {items.map((item) => (
            <div
              key={item.name}
              className="flex items-center justify-between gap-4"
            >
              <div className="min-w-0 flex-1">
                <div className="mb-1 truncate text-xs text-muted-foreground">
                  {item.name}
                </div>
                <div className="h-2.5 w-full rounded-full bg-muted">
                  <div
                    className="h-2.5 rounded-full bg-primary"
                    style={{
                      width: `${Math.round((item.value / max) * 100)}%`,
                    }}
                  />
                </div>
              </div>
              <div className="ps-2 text-xs font-medium tabular-nums">
                {item.value}
              </div>
            </div>
          ))}
        </CardContent>
      </Card>
    </div>
  );
}

function buildMonthData(orders: PaymentOrder[]): MonthPoint[] {
  const now = new Date();
  const points = Array.from({ length: 12 }, (_, index) => {
    const date = new Date(now.getFullYear(), now.getMonth() - (11 - index), 1);
    return {
      key: `${date.getFullYear()}-${date.getMonth()}`,
      name: date.toLocaleDateString(undefined, { month: "short" }),
      total: 0,
    };
  });
  const values = new Map(points.map((point) => [point.key, point]));
  for (const order of orders) {
    if (order.currency.toUpperCase() !== "CNY") continue;
    const date = new Date(order.paid_at ?? order.created_at);
    const point = values.get(`${date.getFullYear()}-${date.getMonth()}`);
    if (point) point.total += order.amount / 100;
  }
  return points.map(({ name, total }) => ({ name, total }));
}

function formatPaidVolume(orders: PaymentOrder[]) {
  const byCurrency = orders.reduce<Record<string, number>>((acc, order) => {
    acc[order.currency] = (acc[order.currency] ?? 0) + order.amount;
    return acc;
  }, {});
  const parts = Object.entries(byCurrency).map(([currency, amount]) =>
    formatMinorAmount(amount, currency),
  );
  return parts.length > 0 ? parts.join(" / ") : "-";
}

function formatComponentStatus(
  t: (key: string, options?: Record<string, unknown>) => string,
  status: {
    status: string;
    latency_ms: number;
    latency_us?: number;
    error?: string;
  },
) {
  if (status.status === "degraded") {
    return status.error || t("home.monitor.degraded");
  }
  return t("home.monitor.okWithLatency", {
    latency: formatLatency(status.latency_us, status.latency_ms),
  });
}

function formatLatency(latencyUS: number | undefined, latencyMS: number) {
  if (typeof latencyUS === "number" && latencyUS > 0) {
    if (latencyUS < 1000) return `${latencyUS}us`;
    return `${(latencyUS / 1000).toFixed(2)}ms`;
  }
  return latencyMS > 0 ? `${latencyMS}ms` : "<1ms";
}

function formatRate(success: number, total: number) {
  if (total === 0) return "-";
  return `${((success / total) * 100).toFixed(1)}%`;
}

const topNav = (t: (key: string) => string) => [
  { title: t("nav.overview"), href: "/", isActive: true, disabled: false },
  { title: t("nav.users"), href: "/users", isActive: false, disabled: false },
  { title: t("nav.apps"), href: "/apps", isActive: false, disabled: false },
  {
    title: t("nav.gatewayConfig"),
    href: "/config/gateway",
    isActive: false,
    disabled: false,
  },
];
