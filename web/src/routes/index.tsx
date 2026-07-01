import { useEffect, useMemo, useState } from "react";
import { Link, createRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import {
  Bar,
  BarChart,
  CartesianGrid,
  PolarAngleAxis,
  PolarGrid,
  Radar,
  RadarChart,
  XAxis,
} from "recharts";
import {
  AlertTriangle,
  ArrowUpRight,
  CheckCircle2,
  Clock3,
  Database,
  RefreshCw,
  Server,
  TimerReset,
  Webhook,
} from "lucide-react";

import { createDataTable, type DataTableColumn } from "@/components/data-table";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart";
import {
  Carousel,
  CarouselContent,
  CarouselItem,
  type CarouselApi,
} from "@/components/ui/carousel";
import { listChannelAccounts } from "@/features/channels/api";
import type { ChannelAccount } from "@/features/channels/types";
import { useAuthStore } from "@/features/auth/store";
import { getMonitoringOverview } from "@/features/monitoring/api";
import type { MonitoringOverview } from "@/features/monitoring/api";
import { listOrders } from "@/features/orders/api";
import type { PaymentOrder } from "@/features/orders/types";
import { listWebhookDeliveries } from "@/features/webhooks/api";
import { formatWebhookEventType } from "@/features/webhooks/event-types";
import type { WebhookDelivery } from "@/features/webhooks/types";
import { APIError } from "@/lib/api";
import { formatDateTime } from "@/lib/date";
import { formatMinorAmount } from "@/lib/money";

import { rootRoute } from "./root";

export const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: HomePage,
});

type RecentOrderRow = {
  id: number;
  order: string;
  app: string;
  channel: string;
  amount: string;
  status: PaymentOrder["status"];
  time: string;
};

type WebhookDeliveryRow = {
  id: number;
  deliveryNo: string;
  eventType: string;
  app: string;
  targetUrl: string;
  status: WebhookDelivery["status"];
  attempts: number;
  time: string;
};

const WebhookDeliveriesTable = createDataTable<WebhookDeliveryRow>();

function HomePage() {
  const { t } = useTranslation();
  const accessToken = useAuthStore((state) => state.accessToken);
  const [orders, setOrders] = useState<PaymentOrder[]>([]);
  const [orderTotal, setOrderTotal] = useState(0);
  const [channels, setChannels] = useState<ChannelAccount[]>([]);
  const [deliveries, setDeliveries] = useState<WebhookDelivery[]>([]);
  const [monitoring, setMonitoring] = useState<MonitoringOverview | null>(null);
  const [webhookTotal, setWebhookTotal] = useState(0);
  const [recentOrderApi, setRecentOrderApi] = useState<CarouselApi>();
  const [recentOrderPaused, setRecentOrderPaused] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const webhookDeliveryColumns = useMemo(
    () =>
      [
        {
          accessorKey: "deliveryNo",
          header: t("webhooks.table.deliveryNo"),
          cell: ({ row }) => (
            <Link
              to="/webhooks/$deliveryId"
              params={{ deliveryId: String(row.original.id) }}
              className="font-mono text-xs font-medium hover:underline"
            >
              {row.original.deliveryNo}
            </Link>
          ),
        },
        { accessorKey: "eventType", header: t("webhooks.table.eventType") },
        { accessorKey: "app", header: t("webhooks.table.app") },
        {
          accessorKey: "targetUrl",
          header: t("webhooks.table.targetUrl"),
          cell: ({ row }) => (
            <span className="block max-w-72 truncate font-mono text-xs">
              {row.original.targetUrl}
            </span>
          ),
        },
        {
          accessorKey: "status",
          header: t("webhooks.table.status"),
          cell: ({ row }) => <StatusBadge status={row.original.status} />,
        },
        { accessorKey: "attempts", header: t("webhooks.table.attemptCount") },
        {
          accessorKey: "time",
          header: () => (
            <div className="text-right">{t("home.labels.time")}</div>
          ),
          cell: ({ row }) => (
            <div className="text-right font-mono text-xs text-muted-foreground">
              {row.original.time}
            </div>
          ),
        },
      ] satisfies DataTableColumn<WebhookDeliveryRow>[],
    [t],
  );

  useEffect(() => {
    if (!recentOrderApi || recentOrderPaused || orders.length <= 1) return;
    const timer = window.setInterval(() => {
      if (recentOrderApi.canScrollNext()) {
        recentOrderApi.scrollNext();
      } else {
        recentOrderApi.scrollTo(0);
      }
    }, 2800);
    return () => window.clearInterval(timer);
  }, [orders.length, recentOrderApi, recentOrderPaused]);

  const recentOrders = orders.slice(0, 8).map((order) => ({
    id: order.id,
    order: order.gateway_order_no,
    app: order.app_id,
    channel: order.channel || "-",
    amount: formatMinorAmount(order.amount, order.currency),
    status: order.status,
    time: formatDateTime(order.created_at),
  }));

  const webhookDeliveries = deliveries.slice(0, 8).map((delivery) => ({
    id: delivery.id,
    deliveryNo: delivery.delivery_no,
    eventType: formatWebhookEventType(delivery.event_type, t),
    app: delivery.app_id,
    targetUrl: delivery.target_url,
    status: delivery.status,
    attempts: delivery.attempt_count,
    time: formatDateTime(delivery.created_at),
  }));

  const channelChartConfig = {
    enabled: {
      label: t("channels.enabled"),
      color: "var(--chart-1)",
    },
    disabled: {
      label: t("channels.disabled"),
      color: "var(--chart-2)",
    },
  } satisfies ChartConfig;
  const channelChartData = buildChannelChartData(channels, (key) =>
    t(`channels.${key}`),
  );

  const paidOrders = orders.filter((order) => order.status === "paid");
  const pendingOrders = orders.filter((order) => order.status === "pending");
  const failedDeliveries = deliveries.filter(
    (delivery) => delivery.status === "failed",
  );
  const pendingDeliveries = deliveries.filter(
    (delivery) => delivery.status === "pending",
  );
  const enabledChannels = channels.filter((channel) => channel.enabled);
  const completedOrders = orders.filter(
    (order) => order.status === "paid" || order.status === "failed",
  );
  const authorizationRate = formatRate(
    paidOrders.length,
    completedOrders.length,
  );
  const amountRadarConfig = {
    amount: {
      label: t("home.overviewCharts.paymentAmount"),
      color: "var(--chart-1)",
    },
  } satisfies ChartConfig;
  const paymentStateConfig = {
    paid: {
      label: t("orders.status.paid"),
      color: "var(--chart-1)",
    },
    unpaid: {
      label: t("home.overviewCharts.unpaid"),
      color: "var(--chart-2)",
    },
  } satisfies ChartConfig;
  const operationsConfig = {
    count: {
      label: t("home.overviewCharts.count"),
      color: "var(--chart-3)",
    },
  } satisfies ChartConfig;

  const overviewMetrics = [
    {
      key: "home.metrics.paidVolume",
      value: formatPaidVolume(paidOrders),
      detail: t("home.metrics.paidVolumeDetail", { count: paidOrders.length }),
    },
    {
      key: "home.metrics.authorizationRate",
      value: authorizationRate,
      detail: t("home.metrics.authorizationRateDetail", {
        count: orders.length,
      }),
    },
    {
      key: "home.metrics.pendingOrders",
      value: String(pendingOrders.length),
      detail: t("home.metrics.pendingOrdersDetail", {
        count: pendingOrders.length,
      }),
    },
    {
      key: "home.metrics.webhookBacklog",
      value: String(failedDeliveries.length + pendingDeliveries.length),
      detail: t("home.metrics.webhookBacklogDetail", {
        failed: failedDeliveries.length,
        pending: pendingDeliveries.length,
      }),
    },
  ];
  const amountRadarData = buildPaymentAmountRadarData(paidOrders);
  const paymentStateData = buildPaymentStateData(
    orders,
    t("home.overviewCharts.unassigned"),
  );
  const operationsData = [
    {
      name: t("home.overviewCharts.enabledChannels"),
      count: enabledChannels.length,
      fill: "var(--chart-1)",
    },
    {
      name: t("home.overviewCharts.disabledChannels"),
      count: channels.length - enabledChannels.length,
      fill: "var(--chart-2)",
    },
    {
      name: t("webhooks.status.succeeded"),
      count: deliveries.filter((delivery) => delivery.status === "succeeded")
        .length,
      fill: "var(--chart-3)",
    },
    {
      name: t("webhooks.status.pending"),
      count: pendingDeliveries.length,
      fill: "var(--chart-4)",
    },
    {
      name: t("webhooks.status.failed"),
      count: failedDeliveries.length,
      fill: "var(--chart-5)",
    },
  ];
  const monitorItems = [
    {
      icon: Database,
      title: t("home.monitor.database"),
      detail: monitoring
        ? formatComponentStatus(t, monitoring.database)
        : t("common.loading"),
      tone: monitoring?.database.status === "degraded" ? "warning" : undefined,
    },
    {
      icon: Server,
      title: t("home.monitor.redis"),
      detail: monitoring
        ? formatComponentStatus(t, monitoring.redis)
        : t("common.loading"),
      tone: monitoring?.redis.status === "degraded" ? "warning" : undefined,
    },
    {
      icon: TimerReset,
      title: t("home.monitor.workers"),
      detail: monitoring
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
        : t("common.loading"),
      tone: monitoring?.queues.some((queue) => queue.status === "degraded")
        ? "warning"
        : undefined,
    },
    {
      icon: Clock3,
      title: t("home.monitor.system"),
      detail: monitoring
        ? t("home.monitor.systemDetail", {
            goroutines: monitoring.runtime.goroutines,
            memory: formatBytes(monitoring.runtime.alloc_bytes),
          })
        : t("common.loading"),
    },
  ] satisfies Array<{
    icon: typeof AlertTriangle;
    title: string;
    detail: string;
    tone?: "warning";
  }>;

  async function load() {
    if (!accessToken) return;
    setLoading(true);
    setError(null);
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
    } catch (err) {
      setError(err instanceof APIError ? err.message : t("home.loadFailed"));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, [accessToken]);

  return (
    <div className="flex min-w-0 flex-col gap-4 sm:gap-5">
      <section className="grid min-w-0 gap-4 lg:grid-cols-[minmax(0,1fr)_360px]">
        <Card className="min-w-0 overflow-hidden">
          <CardHeader className="border-b">
            <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
              <div className="min-w-0">
                <CardTitle className="text-xl tracking-tight sm:text-2xl md:text-3xl">
                  {t("home.gatewayOverview")}
                </CardTitle>
              </div>
              <div className="flex flex-col gap-2 sm:flex-row">
                <Button
                  variant="outline"
                  size="sm"
                  className="w-full sm:w-auto"
                  onClick={load}
                  disabled={loading}
                >
                  <RefreshCw />
                  {t("common.refresh")}
                </Button>
                <Button asChild size="sm" className="w-full sm:w-auto">
                  <Link to="/test-pay">
                    {t("home.testPayment.button")}
                    <ArrowUpRight />
                  </Link>
                </Button>
              </div>
            </div>
          </CardHeader>
          <CardContent className="grid gap-0 p-0">
            <div className="grid gap-0 border-b sm:grid-cols-2 xl:grid-cols-4">
              {overviewMetrics.map((metric) => (
                <div
                  key={metric.key}
                  className="min-w-0 border-b p-4 sm:p-5 xl:border-b-0 xl:border-r last:xl:border-r-0"
                >
                  <p className="text-sm text-muted-foreground">
                    {t(metric.key)}
                  </p>
                  <p className="mt-3 break-words font-mono text-xl font-semibold sm:text-2xl">
                    {loading ? "-" : metric.value}
                  </p>
                  <p className="mt-2 text-xs leading-5 text-muted-foreground">
                    {loading ? t("common.loading") : metric.detail}
                  </p>
                </div>
              ))}
            </div>
            <div className="grid gap-0 xl:grid-cols-3">
              <OverviewChartPanel
                title={t("home.overviewCharts.amountRadar")}
                description={t("home.overviewCharts.amountRadarDescription")}
              >
                <ChartContainer
                  config={amountRadarConfig}
                  className="h-56 w-full sm:h-64"
                  initialDimension={{ width: 300, height: 224 }}
                >
                  <RadarChart data={amountRadarData}>
                    <ChartTooltip
                      cursor={false}
                      content={<ChartTooltipContent hideLabel />}
                    />
                    <PolarAngleAxis dataKey="currency" />
                    <PolarGrid />
                    <Radar
                      dataKey="amount"
                      fill="var(--color-amount)"
                      fillOpacity={0.45}
                      stroke="var(--color-amount)"
                    />
                  </RadarChart>
                </ChartContainer>
              </OverviewChartPanel>

              <OverviewChartPanel
                title={t("home.overviewCharts.paymentState")}
                description={t("home.overviewCharts.paymentStateDescription")}
              >
                <ChartContainer
                  config={paymentStateConfig}
                  className="h-56 w-full sm:h-64"
                  initialDimension={{ width: 300, height: 224 }}
                >
                  <BarChart accessibilityLayer data={paymentStateData}>
                    <CartesianGrid vertical={false} />
                    <XAxis
                      dataKey="name"
                      tickLine={false}
                      tickMargin={8}
                      axisLine={false}
                    />
                    <ChartTooltip
                      cursor={false}
                      content={<ChartTooltipContent indicator="dashed" />}
                    />
                    <ChartLegend content={<ChartLegendContent />} />
                    <Bar
                      dataKey="paid"
                      fill="var(--color-paid)"
                      radius={[6, 6, 0, 0]}
                    />
                    <Bar
                      dataKey="unpaid"
                      fill="var(--color-unpaid)"
                      radius={[6, 6, 0, 0]}
                    />
                  </BarChart>
                </ChartContainer>
              </OverviewChartPanel>

              <OverviewChartPanel
                title={t("home.overviewCharts.operations")}
                description={t("home.overviewCharts.operationsDescription")}
              >
                <ChartContainer
                  config={operationsConfig}
                  className="h-56 w-full sm:h-64"
                  initialDimension={{ width: 300, height: 224 }}
                >
                  <BarChart accessibilityLayer data={operationsData}>
                    <CartesianGrid vertical={false} />
                    <XAxis
                      dataKey="name"
                      tickLine={false}
                      tickMargin={8}
                      axisLine={false}
                    />
                    <ChartTooltip
                      cursor={false}
                      content={<ChartTooltipContent hideLabel />}
                    />
                    <Bar dataKey="count" radius={6} />
                  </BarChart>
                </ChartContainer>
              </OverviewChartPanel>
            </div>
          </CardContent>
        </Card>

        <Card className="min-w-0">
          <CardHeader>
            <CardDescription>{t("home.monitor.description")}</CardDescription>
            <CardTitle>{t("home.operationalChecks")}</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-3">
            {error ? (
              <Alert variant="destructive">
                <AlertTriangle />
                <AlertTitle>{t("home.loadFailed")}</AlertTitle>
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            ) : null}
            {monitorItems.map((item) => (
              <QueueItem
                key={item.title}
                icon={item.icon}
                title={item.title}
                detail={loading ? t("common.loading") : item.detail}
                tone={item.tone}
              />
            ))}
          </CardContent>
        </Card>
      </section>

      <section className="grid min-w-0 gap-4 xl:grid-cols-[420px_minmax(0,1fr)]">
        <Card className="min-w-0">
          <CardHeader>
            <CardDescription>{t("home.routingStatus")}</CardDescription>
            <CardTitle>{t("home.channelHealth")}</CardTitle>
          </CardHeader>
          <CardContent>
            {channels.length > 0 ? (
              <ChartContainer
                config={channelChartConfig}
                className="h-56 w-full sm:h-72"
                initialDimension={{ width: 300, height: 224 }}
              >
                <BarChart accessibilityLayer data={channelChartData}>
                  <CartesianGrid vertical={false} />
                  <XAxis
                    dataKey="name"
                    tickLine={false}
                    tickMargin={8}
                    axisLine={false}
                  />
                  <ChartTooltip
                    cursor={false}
                    content={<ChartTooltipContent indicator="dashed" />}
                  />
                  <ChartLegend content={<ChartLegendContent />} />
                  <Bar
                    dataKey="enabled"
                    fill="var(--color-enabled)"
                    radius={[6, 6, 0, 0]}
                  />
                  <Bar
                    dataKey="disabled"
                    fill="var(--color-disabled)"
                    radius={[6, 6, 0, 0]}
                  />
                </BarChart>
              </ChartContainer>
            ) : (
              <p className="text-sm text-muted-foreground">
                {loading ? t("common.loading") : t("home.empty.channels")}
              </p>
            )}
          </CardContent>
        </Card>

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
            <RecentOrdersCarousel
              orders={recentOrders}
              loading={loading}
              emptyText={t("orders.empty")}
              setApi={setRecentOrderApi}
              onPauseChange={setRecentOrderPaused}
            />
          </CardContent>
        </Card>
      </section>

      <section className="grid min-w-0 gap-4 lg:grid-cols-[minmax(0,1fr)_360px]">
        <Card className="min-w-0">
          <CardHeader>
            <CardDescription>{t("home.webhookCenter")}</CardDescription>
            <CardTitle>{t("home.deliveryQueue")}</CardTitle>
          </CardHeader>
          <CardContent>
            <WebhookDeliveriesTable
              columns={webhookDeliveryColumns}
              data={webhookDeliveries}
              loading={loading}
              loadingText={t("webhooks.loading")}
              emptyText={t("home.empty.webhooks")}
              pageSize={20}
            />
          </CardContent>
        </Card>

        <Card className="min-w-0">
          <CardHeader>
            <CardDescription>{t("home.compensation")}</CardDescription>
            <CardTitle>{t("home.queryJobs")}</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-4">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div className="flex min-w-0 items-center gap-3">
                <Clock3 className="size-4 text-muted-foreground" />
                <div className="min-w-0">
                  <p className="text-sm font-medium">
                    {t("home.jobs.orderSnapshot")}
                  </p>
                  <p className="text-xs leading-5 text-muted-foreground">
                    {t("home.jobs.orderSnapshotDetail", { count: orderTotal })}
                  </p>
                </div>
              </div>
              <Badge variant="outline" className="w-fit">
                {t("common.clean")}
              </Badge>
            </div>
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div className="flex min-w-0 items-center gap-3">
                <Webhook className="size-4 text-muted-foreground" />
                <div className="min-w-0">
                  <p className="text-sm font-medium">
                    {t("home.jobs.webhookSnapshot")}
                  </p>
                  <p className="text-xs leading-5 text-muted-foreground">
                    {t("home.jobs.webhookSnapshotDetail", {
                      count: webhookTotal,
                    })}
                  </p>
                </div>
              </div>
              <Badge variant="outline" className="w-fit">
                {t("common.running")}
              </Badge>
            </div>
          </CardContent>
        </Card>
      </section>
    </div>
  );
}

function QueueItem({
  icon: Icon,
  title,
  detail,
  tone,
}: {
  icon: typeof AlertTriangle;
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

function OverviewChartPanel({
  title,
  description,
  children,
}: {
  title: string;
  description: string;
  children: React.ReactNode;
}) {
  return (
    <div className="min-w-0 border-b p-4 last:border-b-0 sm:p-5 xl:border-b-0 xl:border-r xl:last:border-r-0">
      <div className="mb-4">
        <p className="text-sm font-medium">{title}</p>
        <p className="mt-1 text-xs leading-5 text-muted-foreground">
          {description}
        </p>
      </div>
      {children}
    </div>
  );
}

function RecentOrdersCarousel({
  orders,
  loading,
  emptyText,
  setApi,
  onPauseChange,
}: {
  orders: RecentOrderRow[];
  loading: boolean;
  emptyText: string;
  setApi: (api: CarouselApi) => void;
  onPauseChange: (paused: boolean) => void;
}) {
  const { t } = useTranslation();

  if (loading) {
    return (
      <p className="text-sm text-muted-foreground">{t("common.loading")}</p>
    );
  }

  if (orders.length === 0) {
    return <p className="text-sm text-muted-foreground">{emptyText}</p>;
  }

  return (
    <Carousel
      orientation="vertical"
      opts={{ align: "start", loop: orders.length > 1 }}
      setApi={setApi}
      className="h-80 sm:h-72"
      onMouseEnter={() => onPauseChange(true)}
      onMouseLeave={() => onPauseChange(false)}
      onFocus={() => onPauseChange(true)}
      onBlur={() => onPauseChange(false)}
    >
      <CarouselContent className="h-80 sm:h-72">
        {orders.map((order) => (
          <CarouselItem key={order.id} className="basis-full sm:basis-1/2">
            <Link
              to="/orders/$orderId"
              params={{ orderId: String(order.id) }}
              className="group block h-full rounded-md border p-4 transition-colors hover:bg-muted/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <p className="truncate font-mono text-xs font-medium">
                    {order.order}
                  </p>
                  <p className="mt-2 text-sm font-semibold">{order.amount}</p>
                </div>
                <StatusBadge status={order.status} />
              </div>
              <div className="mt-4 grid grid-cols-1 gap-3 text-xs sm:grid-cols-3">
                <div className="min-w-0">
                  <p className="text-muted-foreground">
                    {t("home.labels.app")}
                  </p>
                  <p className="mt-1 truncate font-mono">{order.app}</p>
                </div>
                <div className="min-w-0">
                  <p className="text-muted-foreground">
                    {t("home.labels.channel")}
                  </p>
                  <p className="mt-1 truncate font-mono">{order.channel}</p>
                </div>
                <div className="min-w-0">
                  <p className="text-muted-foreground">
                    {t("home.labels.time")}
                  </p>
                  <p className="mt-1 truncate font-mono">{order.time}</p>
                </div>
              </div>
              <div className="mt-3 flex items-center justify-end text-xs text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100 group-focus-visible:opacity-100">
                {t("home.openOrderDetail")}
                <ArrowUpRight className="ml-1" />
              </div>
            </Link>
          </CarouselItem>
        ))}
      </CarouselContent>
    </Carousel>
  );
}

function StatusBadge({ status }: { status: string }) {
  const { t } = useTranslation();

  if (status === "paid" || status === "succeeded" || status === "enabled") {
    return (
      <Badge variant="secondary" className="gap-1">
        <CheckCircle2 className="size-3" />
        {t(statusLabelKey(status))}
      </Badge>
    );
  }

  if (status === "failed" || status === "disabled") {
    return (
      <Badge variant="destructive" className="gap-1">
        <AlertTriangle className="size-3" />
        {t(statusLabelKey(status))}
      </Badge>
    );
  }

  return <Badge variant="outline">{t(statusLabelKey(status))}</Badge>;
}

function statusLabelKey(status: string) {
  if (status === "enabled" || status === "disabled")
    return `channels.${status}`;
  if (
    status === "pending" ||
    status === "paid" ||
    status === "failed" ||
    status === "closed"
  ) {
    return `orders.status.${status}`;
  }
  if (status === "succeeded") return "webhooks.status.succeeded";
  return `webhooks.status.${status}`;
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

function formatBytes(bytes: number) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KiB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MiB`;
}

function buildPaymentAmountRadarData(orders: PaymentOrder[]) {
  const byCurrency = orders.reduce<Record<string, number>>((acc, order) => {
    acc[order.currency] = (acc[order.currency] ?? 0) + order.amount;
    return acc;
  }, {});

  const rows = Object.entries(byCurrency).map(([currency, amount]) => ({
    currency,
    amount: amount / minorUnitDivisor(currency),
  }));

  return rows.length > 0 ? rows : [{ currency: "-", amount: 0 }];
}

function buildPaymentStateData(orders: PaymentOrder[], fallbackName: string) {
  const byChannel = orders.reduce<
    Record<string, { name: string; paid: number; unpaid: number }>
  >((acc, order) => {
    const name = order.channel || fallbackName;
    acc[name] ??= { name, paid: 0, unpaid: 0 };
    if (order.status === "paid") {
      acc[name].paid += 1;
    } else {
      acc[name].unpaid += 1;
    }
    return acc;
  }, {});

  const rows = Object.values(byChannel);
  return rows.length > 0 ? rows : [{ name: fallbackName, paid: 0, unpaid: 0 }];
}

function buildChannelChartData(
  channels: ChannelAccount[],
  label: (channel: ChannelAccount["channel"]) => string,
) {
  const rows = channels.reduce<
    Record<string, { name: string; enabled: number; disabled: number }>
  >((acc, channel) => {
    const name = label(channel.channel);
    acc[name] ??= { name, enabled: 0, disabled: 0 };
    if (channel.enabled) {
      acc[name].enabled += 1;
    } else {
      acc[name].disabled += 1;
    }
    return acc;
  }, {});

  return Object.values(rows);
}

function minorUnitDivisor(currency: string) {
  return [
    "BIF",
    "CLP",
    "DJF",
    "GNF",
    "JPY",
    "KMF",
    "KRW",
    "PYG",
    "RWF",
    "UGX",
    "VND",
    "VUV",
    "XAF",
    "XOF",
    "XPF",
  ].includes(currency.toUpperCase())
    ? 1
    : 100;
}

function formatRate(success: number, total: number) {
  if (total === 0) return "-";
  return `${((success / total) * 100).toFixed(1)}%`;
}
