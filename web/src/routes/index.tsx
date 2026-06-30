import { createRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import {
  AlertTriangle,
  ArrowUpRight,
  CheckCircle2,
  Clock3,
  CreditCard,
  GitBranch,
  MoreHorizontal,
  TimerReset,
  Webhook,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { createDataTable, type DataTableColumn } from "@/components/data-table";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

import { rootRoute } from "./root";

export const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: HomePage,
});

const metrics = [
  {
    key: "home.metrics.paidVolume",
    value: "CNY 486,420",
    delta: "+12.4%",
    detailKey: "home.metrics.paidVolumeDetail",
  },
  {
    key: "home.metrics.authorizationRate",
    value: "97.3%",
    delta: "+1.8%",
    detailKey: "home.metrics.authorizationRateDetail",
  },
  {
    key: "home.metrics.pendingOrders",
    value: "142",
    delta: "-8.6%",
    detailKey: "home.metrics.pendingOrdersDetail",
  },
  {
    key: "home.metrics.webhookBacklog",
    value: "17",
    delta: "+4",
    detailKey: "home.metrics.webhookBacklogDetail",
  },
];

const channels = [
  {
    name: "Alipay",
    method: "CNY / wallet",
    status: "healthy",
    latency: "318 ms",
    success: "98.1%",
  },
  {
    name: "WeChat Pay",
    method: "CNY / wallet",
    status: "degraded",
    latency: "624 ms",
    success: "94.6%",
  },
  {
    name: "Stripe",
    method: "USD / card",
    status: "healthy",
    latency: "401 ms",
    success: "97.8%",
  },
];

const recentOrders = [
  {
    order: "pay_20260630_8HF2",
    app: "snsgo",
    channel: "alipay",
    amount: "CNY 99.00",
    status: "succeeded",
    time: "18:42:11",
  },
  {
    order: "pay_20260630_C4KP",
    app: "billing-lab",
    channel: "stripe",
    amount: "USD 12.99",
    status: "pending",
    time: "18:39:47",
  },
  {
    order: "pay_20260630_M9RA",
    app: "snsgo",
    channel: "wechat",
    amount: "CNY 199.00",
    status: "callback_waiting",
    time: "18:36:20",
  },
  {
    order: "pay_20260630_Q1VT",
    app: "ops-console",
    channel: "stripe",
    amount: "USD 49.00",
    status: "succeeded",
    time: "18:31:04",
  },
];

const RecentOrdersTable = createDataTable<(typeof recentOrders)[number]>();

const webhookDeliveries = [
  {
    event: "payment.succeeded",
    app: "snsgo",
    target: "/api/payments/webhook",
    state: "delivered",
  },
  {
    event: "refund.succeeded",
    app: "billing-lab",
    target: "/billing/events",
    state: "retrying",
  },
  {
    event: "payment.succeeded",
    app: "snsgo",
    target: "/api/payments/webhook",
    state: "scheduled",
  },
];

function HomePage() {
  const { t } = useTranslation();
  const recentOrderColumns = [
    {
      accessorKey: "order",
      header: t("home.labels.order"),
      cell: ({ row }) => (
        <span className="font-mono text-xs">{row.original.order}</span>
      ),
    },
    { accessorKey: "app", header: t("home.labels.app") },
    { accessorKey: "channel", header: t("home.labels.channel") },
    {
      accessorKey: "amount",
      header: t("home.labels.amount"),
      cell: ({ row }) => (
        <span className="font-mono">{row.original.amount}</span>
      ),
    },
    {
      accessorKey: "status",
      header: t("home.labels.status"),
      cell: ({ row }) => <StatusBadge status={row.original.status} />,
    },
    {
      accessorKey: "time",
      header: () => <div className="text-right">{t("home.labels.time")}</div>,
      cell: ({ row }) => (
        <div className="text-right font-mono text-xs text-muted-foreground">
          {row.original.time}
        </div>
      ),
    },
  ] satisfies DataTableColumn<(typeof recentOrders)[number]>[];

  return (
    <div className="flex flex-col gap-5">
      <section className="grid gap-4 lg:grid-cols-[1fr_360px]">
        <Card className="overflow-hidden">
          <CardHeader className="border-b">
            <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
              <div>
                <CardDescription>{t("home.gatewayOverview")}</CardDescription>
                <CardTitle className="mt-2 text-2xl tracking-tight md:text-3xl">
                  {t("home.headline")}
                </CardTitle>
              </div>
              <Button size="sm">
                {t("home.openOrders")}
                <ArrowUpRight className="size-4" />
              </Button>
            </div>
          </CardHeader>
          <CardContent className="grid gap-0 p-0 md:grid-cols-4">
            {metrics.map((metric) => (
              <div
                key={metric.key}
                className="border-b p-5 md:border-b-0 md:border-r last:md:border-r-0"
              >
                <div className="flex items-center justify-between gap-2">
                  <p className="text-sm text-muted-foreground">
                    {t(metric.key)}
                  </p>
                  <Badge variant="secondary">{metric.delta}</Badge>
                </div>
                <p className="mt-3 font-mono text-2xl font-semibold">
                  {metric.value}
                </p>
                <p className="mt-2 text-xs leading-5 text-muted-foreground">
                  {t(metric.detailKey)}
                </p>
              </div>
            ))}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardDescription>{t("home.actionQueue")}</CardDescription>
            <CardTitle>{t("home.operationalChecks")}</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-3">
            <QueueItem
              icon={AlertTriangle}
              title={t("home.queue.wechatLatency")}
              detail={t("home.queue.wechatLatencyDetail")}
              tone="warning"
            />
            <QueueItem
              icon={TimerReset}
              title={t("home.queue.expiringOrders")}
              detail={t("home.queue.expiringOrdersDetail")}
            />
            <QueueItem
              icon={Webhook}
              title={t("home.queue.webhookRetries")}
              detail={t("home.queue.webhookRetriesDetail")}
            />
          </CardContent>
        </Card>
      </section>

      <section className="grid gap-4 xl:grid-cols-[380px_1fr]">
        <Card>
          <CardHeader>
            <CardDescription>{t("home.routingStatus")}</CardDescription>
            <CardTitle>{t("home.channelHealth")}</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-3">
            {channels.map((channel) => (
              <Card key={channel.name}>
                <CardContent className="py-4">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <div className="flex items-center gap-2">
                        <CreditCard className="size-4 text-muted-foreground" />
                        <p className="text-sm font-medium">{channel.name}</p>
                      </div>
                      <p className="mt-1 text-xs text-muted-foreground">
                        {channel.method}
                      </p>
                    </div>
                    <StatusBadge status={channel.status} />
                  </div>
                  <div className="mt-4 grid grid-cols-2 gap-3 text-sm">
                    <div>
                      <p className="text-xs text-muted-foreground">
                        {t("home.labels.latency")}
                      </p>
                      <p className="mt-1 font-mono">{channel.latency}</p>
                    </div>
                    <div>
                      <p className="text-xs text-muted-foreground">
                        {t("home.labels.success")}
                      </p>
                      <p className="mt-1 font-mono">{channel.success}</p>
                    </div>
                  </div>
                </CardContent>
              </Card>
            ))}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <div>
              <CardDescription>{t("home.liveOrders")}</CardDescription>
              <CardTitle>{t("home.recentPaymentOrders")}</CardTitle>
            </div>
            <Button
              variant="outline"
              size="icon-sm"
              aria-label={t("common.moreActions")}
            >
              <MoreHorizontal className="size-4" />
            </Button>
          </CardHeader>
          <CardContent>
            <RecentOrdersTable
              columns={recentOrderColumns}
              data={recentOrders}
            />
          </CardContent>
        </Card>
      </section>

      <section className="grid gap-4 lg:grid-cols-[1fr_360px]">
        <Card>
          <CardHeader>
            <CardDescription>{t("home.webhookCenter")}</CardDescription>
            <CardTitle>{t("home.deliveryQueue")}</CardTitle>
          </CardHeader>
          <CardContent className="grid gap-3 md:grid-cols-3">
            {webhookDeliveries.map((delivery) => (
              <Card
                key={`${delivery.event}-${delivery.target}`}
                className="rounded-md shadow-none"
              >
                <CardContent className="py-4">
                  <div className="flex items-center justify-between gap-2">
                    <p className="text-sm font-medium">{delivery.event}</p>
                    <StatusBadge status={delivery.state} />
                  </div>
                  <p className="mt-2 text-xs text-muted-foreground">
                    {delivery.app}
                  </p>
                  <p className="mt-3 truncate font-mono text-xs">
                    {delivery.target}
                  </p>
                </CardContent>
              </Card>
            ))}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardDescription>{t("home.compensation")}</CardDescription>
            <CardTitle>{t("home.queryJobs")}</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-4">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <Clock3 className="size-4 text-muted-foreground" />
                <div>
                  <p className="text-sm font-medium">
                    {t("home.jobs.pendingScan")}
                  </p>
                  <p className="text-xs text-muted-foreground">
                    {t("home.jobs.pendingScanDetail")}
                  </p>
                </div>
              </div>
              <Badge variant="outline">{t("common.running")}</Badge>
            </div>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <GitBranch className="size-4 text-muted-foreground" />
                <div>
                  <p className="text-sm font-medium">
                    {t("home.jobs.routeAudit")}
                  </p>
                  <p className="text-xs text-muted-foreground">
                    {t("home.jobs.routeAuditDetail")}
                  </p>
                </div>
              </div>
              <Badge variant="outline">{t("common.clean")}</Badge>
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
          className={`mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-md ${
            tone === "warning"
              ? "bg-destructive/10 text-destructive"
              : "bg-secondary text-muted-foreground"
          }`}
        >
          <Icon className="size-4" />
        </div>
        <div>
          <p className="text-sm font-medium">{title}</p>
          <p className="mt-1 text-xs leading-5 text-muted-foreground">
            {detail}
          </p>
        </div>
      </CardContent>
    </Card>
  );
}

function StatusBadge({ status }: { status: string }) {
  const { t } = useTranslation();

  if (
    status === "healthy" ||
    status === "succeeded" ||
    status === "delivered"
  ) {
    return (
      <Badge variant="secondary" className="gap-1">
        <CheckCircle2 className="size-3" />
        {t(`common.${status}`)}
      </Badge>
    );
  }

  if (status === "degraded" || status === "retrying") {
    return (
      <Badge variant="destructive" className="gap-1">
        <AlertTriangle className="size-3" />
        {t(`common.${status}`)}
      </Badge>
    );
  }

  return <Badge variant="outline">{t(`common.${status}`)}</Badge>;
}
