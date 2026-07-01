import { useEffect, useMemo, useState } from "react";
import { Link, createRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import {
  AlertTriangle,
  ArrowUpRight,
  CheckCircle2,
  Clock3,
  CreditCard,
  RefreshCw,
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
import { listChannelAccounts } from "@/features/channels/api";
import type { ChannelAccount } from "@/features/channels/types";
import { useAuthStore } from "@/features/auth/store";
import { listOrders } from "@/features/orders/api";
import type { PaymentOrder } from "@/features/orders/types";
import { listWebhookDeliveries } from "@/features/webhooks/api";
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

const RecentOrdersTable = createDataTable<RecentOrderRow>();

function HomePage() {
  const { t } = useTranslation();
  const accessToken = useAuthStore((state) => state.accessToken);
  const [orders, setOrders] = useState<PaymentOrder[]>([]);
  const [orderTotal, setOrderTotal] = useState(0);
  const [channels, setChannels] = useState<ChannelAccount[]>([]);
  const [deliveries, setDeliveries] = useState<WebhookDelivery[]>([]);
  const [webhookTotal, setWebhookTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const recentOrderColumns = useMemo(
    () =>
      [
        {
          accessorKey: "order",
          header: t("home.labels.order"),
          cell: ({ row }) => (
            <Link
              to="/orders/$orderId"
              params={{ orderId: String(row.original.id) }}
              className="font-mono text-xs font-medium hover:underline"
            >
              {row.original.order}
            </Link>
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
          header: () => (
            <div className="text-right">{t("home.labels.time")}</div>
          ),
          cell: ({ row }) => (
            <div className="text-right font-mono text-xs text-muted-foreground">
              {row.original.time}
            </div>
          ),
        },
      ] satisfies DataTableColumn<RecentOrderRow>[],
    [t],
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

  const metrics = [
    {
      key: "home.metrics.paidVolume",
      value: formatPaidVolume(paidOrders),
      detail: t("home.metrics.paidVolumeDetail", { count: paidOrders.length }),
    },
    {
      key: "home.metrics.authorizationRate",
      value: formatRate(
        paidOrders.length,
        orders.filter(
          (order) => order.status === "paid" || order.status === "failed",
        ).length,
      ),
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

  const recentOrders = orders.slice(0, 8).map((order) => ({
    id: order.id,
    order: order.gateway_order_no,
    app: order.app_id,
    channel: order.channel || "-",
    amount: formatMinorAmount(order.amount, order.currency),
    status: order.status,
    time: formatDateTime(order.created_at),
  }));

  const webhookCards = deliveries.slice(0, 6);

  async function load() {
    if (!accessToken) return;
    setLoading(true);
    setError(null);
    try {
      const [ordersResult, channelsResult, deliveriesResult] =
        await Promise.all([
          listOrders(accessToken, { page: 1, page_size: 100 }),
          listChannelAccounts(accessToken),
          listWebhookDeliveries(accessToken, { page: 1, page_size: 100 }),
        ]);
      setOrders(ordersResult.items);
      setOrderTotal(ordersResult.total);
      setChannels(channelsResult.items);
      setDeliveries(deliveriesResult.items);
      setWebhookTotal(deliveriesResult.total);
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
    <div className="flex flex-col gap-5">
      <section className="grid gap-4 lg:grid-cols-[1fr_360px]">
        <Card className="overflow-hidden">
          <CardHeader className="border-b">
            <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
              <div>
                <CardDescription>{t("home.gatewayOverview")}</CardDescription>
                <CardTitle className="mt-2 text-2xl tracking-tight md:text-3xl">
                  {enabledChannels.length > 0
                    ? t("home.headline")
                    : t("home.headlineNoChannels")}
                </CardTitle>
              </div>
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={load}
                  disabled={loading}
                >
                  <RefreshCw />
                  {t("common.refresh")}
                </Button>
                <Button asChild size="sm">
                  <Link to="/test-pay">
                    {t("home.testPayment.button")}
                    <ArrowUpRight />
                  </Link>
                </Button>
              </div>
            </div>
          </CardHeader>
          <CardContent className="grid gap-0 p-0 md:grid-cols-4">
            {metrics.map((metric) => (
              <div
                key={metric.key}
                className="border-b p-5 md:border-b-0 md:border-r last:md:border-r-0"
              >
                <p className="text-sm text-muted-foreground">{t(metric.key)}</p>
                <p className="mt-3 font-mono text-2xl font-semibold">
                  {loading ? "-" : metric.value}
                </p>
                <p className="mt-2 text-xs leading-5 text-muted-foreground">
                  {loading ? t("common.loading") : metric.detail}
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
            {error ? (
              <Alert variant="destructive">
                <AlertTriangle />
                <AlertTitle>{t("home.loadFailed")}</AlertTitle>
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            ) : null}
            <QueueItem
              icon={CreditCard}
              title={t("home.queue.channelAccounts", {
                enabled: enabledChannels.length,
                total: channels.length,
              })}
              detail={t("home.queue.channelAccountsDetail")}
              tone={enabledChannels.length === 0 ? "warning" : undefined}
            />
            <QueueItem
              icon={TimerReset}
              title={t("home.queue.pendingOrders", {
                count: pendingOrders.length,
              })}
              detail={t("home.queue.pendingOrdersDetail")}
              tone={pendingOrders.length > 0 ? "warning" : undefined}
            />
            <QueueItem
              icon={Webhook}
              title={t("home.queue.webhookDeliveries", {
                failed: failedDeliveries.length,
                pending: pendingDeliveries.length,
              })}
              detail={t("home.queue.webhookDeliveriesDetail")}
              tone={failedDeliveries.length > 0 ? "warning" : undefined}
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
            {channels.length > 0 ? (
              channels.map((channel) => (
                <Card key={channel.id}>
                  <CardContent className="py-4">
                    <div className="flex items-start justify-between gap-3">
                      <div>
                        <div className="flex items-center gap-2">
                          <CreditCard className="size-4 text-muted-foreground" />
                          <p className="text-sm font-medium">{channel.name}</p>
                        </div>
                        <p className="mt-1 text-xs text-muted-foreground">
                          {t(`channels.${channel.channel}`)} /{" "}
                          {t(`channels.${channel.env}`)}
                        </p>
                      </div>
                      <StatusBadge
                        status={channel.enabled ? "enabled" : "disabled"}
                      />
                    </div>
                    <div className="mt-4 grid grid-cols-2 gap-3 text-sm">
                      <div>
                        <p className="text-xs text-muted-foreground">
                          {t("home.labels.channel")}
                        </p>
                        <p className="mt-1 font-mono">{channel.channel}</p>
                      </div>
                      <div>
                        <p className="text-xs text-muted-foreground">
                          {t("home.labels.updatedAt")}
                        </p>
                        <p className="mt-1 font-mono text-xs">
                          {formatDateTime(channel.updated_at)}
                        </p>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              ))
            ) : (
              <p className="text-sm text-muted-foreground">
                {loading ? t("common.loading") : t("home.empty.channels")}
              </p>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <div>
              <CardDescription>{t("home.liveOrders")}</CardDescription>
              <CardTitle>{t("home.recentPaymentOrders")}</CardTitle>
            </div>
            <Button variant="outline" size="sm" asChild>
              <Link to="/orders">{t("home.openOrders")}</Link>
            </Button>
          </CardHeader>
          <CardContent>
            <RecentOrdersTable
              columns={recentOrderColumns}
              data={recentOrders}
              loading={loading}
              loadingText={t("orders.loading")}
              emptyText={t("orders.empty")}
              pageSize={8}
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
            {webhookCards.length > 0 ? (
              webhookCards.map((delivery) => (
                <Card key={delivery.id} className="rounded-md shadow-none">
                  <CardContent className="py-4">
                    <div className="flex items-center justify-between gap-2">
                      <p className="text-sm font-medium">
                        {delivery.event_type}
                      </p>
                      <StatusBadge status={delivery.status} />
                    </div>
                    <p className="mt-2 text-xs text-muted-foreground">
                      {delivery.app_id}
                    </p>
                    <p className="mt-3 truncate font-mono text-xs">
                      {delivery.target_url}
                    </p>
                  </CardContent>
                </Card>
              ))
            ) : (
              <p className="text-sm text-muted-foreground">
                {loading ? t("common.loading") : t("home.empty.webhooks")}
              </p>
            )}
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
                    {t("home.jobs.orderSnapshot")}
                  </p>
                  <p className="text-xs text-muted-foreground">
                    {t("home.jobs.orderSnapshotDetail", { count: orderTotal })}
                  </p>
                </div>
              </div>
              <Badge variant="outline">{t("common.clean")}</Badge>
            </div>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <Webhook className="size-4 text-muted-foreground" />
                <div>
                  <p className="text-sm font-medium">
                    {t("home.jobs.webhookSnapshot")}
                  </p>
                  <p className="text-xs text-muted-foreground">
                    {t("home.jobs.webhookSnapshotDetail", {
                      count: webhookTotal,
                    })}
                  </p>
                </div>
              </div>
              <Badge variant="outline">{t("common.running")}</Badge>
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

function formatRate(success: number, total: number) {
  if (total === 0) return "-";
  return `${((success / total) * 100).toFixed(1)}%`;
}
