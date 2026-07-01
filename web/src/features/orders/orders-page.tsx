import { useEffect, useMemo, useState, type FormEvent } from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Eye, RefreshCw, Search, XCircle } from "lucide-react";

import {
  createDataTable,
  DataTableRowActions,
  type DataTableColumn,
} from "@/components/data-table";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useAuthStore } from "@/features/auth/store";
import { APIError } from "@/lib/api";
import { formatDateTime } from "@/lib/date";
import { formatMinorAmount } from "@/lib/money";

import { closeOrder, listOrders } from "./api";
import type { ListOrdersParams, PaymentOrder } from "./types";
import { canCloseOrder, orderStatusVariant } from "./utils";

const OrdersDataTable = createDataTable<PaymentOrder>();

const defaultFilters = {
  appId: "",
  status: "all",
  channel: "all",
  currency: "all",
  merchantOrderNo: "",
};

type FilterState = typeof defaultFilters;

export function OrdersPage() {
  const { t } = useTranslation();
  const accessToken = useAuthStore((state) => state.accessToken);
  const [orders, setOrders] = useState<PaymentOrder[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filters, setFilters] = useState<FilterState>(defaultFilters);
  const [appliedFilters, setAppliedFilters] =
    useState<FilterState>(defaultFilters);
  const [closeTarget, setCloseTarget] = useState<PaymentOrder | null>(null);

  const columns = useMemo<DataTableColumn<PaymentOrder>[]>(
    () => [
      {
        accessorKey: "gateway_order_no",
        header: t("orders.table.order"),
        cell: ({ row }) => (
          <div className="flex flex-col">
            <span className="font-mono text-xs font-medium">
              {row.original.gateway_order_no}
            </span>
            <span className="text-xs text-muted-foreground">
              {row.original.subject}
            </span>
          </div>
        ),
      },
      {
        accessorKey: "app_id",
        header: t("orders.table.app"),
        cell: ({ row }) => (
          <span className="font-mono text-xs">{row.original.app_id}</span>
        ),
      },
      {
        accessorKey: "merchant_order_no",
        header: t("orders.table.merchantOrder"),
        cell: ({ row }) => (
          <span className="font-mono text-xs">
            {row.original.merchant_order_no}
          </span>
        ),
      },
      {
        accessorKey: "amount",
        header: t("orders.table.amount"),
        cell: ({ row }) => (
          <span className="font-mono text-xs">
            {formatMinorAmount(row.original.amount, row.original.currency)}
          </span>
        ),
      },
      {
        accessorKey: "status",
        header: t("orders.table.status"),
        cell: ({ row }) => (
          <Badge variant={orderStatusVariant(row.original.status)}>
            {t(`orders.status.${row.original.status}`)}
          </Badge>
        ),
      },
      {
        accessorKey: "channel",
        header: t("orders.table.channel"),
        cell: ({ row }) => row.original.channel || "-",
      },
      {
        accessorKey: "channel_trade_no",
        header: t("orders.table.channelTradeNo"),
        cell: ({ row }) => (
          <span className="font-mono text-xs">
            {row.original.channel_trade_no || "-"}
          </span>
        ),
      },
      {
        accessorKey: "created_at",
        header: t("orders.table.createdAt"),
        cell: ({ row }) => (
          <span className="font-mono text-xs text-muted-foreground">
            {formatDateTime(row.original.created_at)}
          </span>
        ),
      },
      {
        id: "actions",
        header: () => (
          <div className="text-right">{t("common.moreActions")}</div>
        ),
        cell: ({ row }) => (
          <DataTableRowActions
            actions={[
              {
                label: t("orders.view"),
                icon: Eye,
                asChild: true,
                child: (
                  <Link
                    to="/orders/$orderId"
                    params={{ orderId: String(row.original.id) }}
                  >
                    <Eye data-icon="inline-start" />
                    {t("orders.view")}
                  </Link>
                ),
              },
              {
                label: t("orders.close"),
                icon: XCircle,
                disabled: !canCloseOrder(row.original),
                onClick: () => setCloseTarget(row.original),
              },
            ]}
          />
        ),
      },
    ],
    [t],
  );

  async function load(nextFilters = appliedFilters) {
    if (!accessToken) return;
    setLoading(true);
    setError(null);
    try {
      const result = await listOrders(accessToken, toListParams(nextFilters));
      setOrders(result.items);
      setTotal(result.total);
    } catch (err) {
      setError(err instanceof APIError ? err.message : t("orders.loadFailed"));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, [accessToken, appliedFilters]);

  function applyFilters(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setAppliedFilters(filters);
  }

  function resetFilters() {
    setFilters(defaultFilters);
    setAppliedFilters(defaultFilters);
  }

  async function confirmClose() {
    if (!accessToken || !closeTarget) return;
    try {
      const result = await closeOrder(accessToken, closeTarget.id);
      setOrders((current) =>
        current.map((item) =>
          item.id === result.order.id ? result.order : item,
        ),
      );
      setCloseTarget(null);
    } catch (err) {
      setError(err instanceof APIError ? err.message : t("orders.closeFailed"));
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">
            {t("orders.title")}
          </h1>
          <p className="text-sm text-muted-foreground">
            {t("orders.description")}
          </p>
        </div>
        <Button variant="outline" onClick={() => load()}>
          <RefreshCw />
          {t("common.refresh")}
        </Button>
      </div>

      {error ? (
        <Alert variant="destructive">
          <AlertTitle>{t("orders.loadFailed")}</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      ) : null}

      <Card>
        <CardHeader>
          <CardTitle>{t("orders.filters")}</CardTitle>
          <CardDescription>{t("orders.filtersDescription")}</CardDescription>
        </CardHeader>
        <CardContent>
          <form
            className="flex flex-col gap-4 lg:flex-row lg:items-end"
            onSubmit={applyFilters}
          >
            <FieldGroup className="grid flex-1 gap-3 md:grid-cols-2 xl:grid-cols-5">
              <Field>
                <FieldLabel htmlFor="orders-app-id">
                  {t("orders.fields.appId")}
                </FieldLabel>
                <Input
                  id="orders-app-id"
                  value={filters.appId}
                  onChange={(event) =>
                    setFilters((current) => ({
                      ...current,
                      appId: event.target.value,
                    }))
                  }
                />
              </Field>
              <Field>
                <FieldLabel>{t("orders.fields.status")}</FieldLabel>
                <Select
                  value={filters.status}
                  onValueChange={(value) =>
                    setFilters((current) => ({ ...current, status: value }))
                  }
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      <SelectItem value="all">{t("orders.all")}</SelectItem>
                      <SelectItem value="pending">
                        {t("orders.status.pending")}
                      </SelectItem>
                      <SelectItem value="paid">
                        {t("orders.status.paid")}
                      </SelectItem>
                      <SelectItem value="failed">
                        {t("orders.status.failed")}
                      </SelectItem>
                      <SelectItem value="closed">
                        {t("orders.status.closed")}
                      </SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
              <Field>
                <FieldLabel>{t("orders.fields.channel")}</FieldLabel>
                <Select
                  value={filters.channel}
                  onValueChange={(value) =>
                    setFilters((current) => ({ ...current, channel: value }))
                  }
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      <SelectItem value="all">{t("orders.all")}</SelectItem>
                      <SelectItem value="wechat">
                        {t("channels.wechat")}
                      </SelectItem>
                      <SelectItem value="alipay">
                        {t("channels.alipay")}
                      </SelectItem>
                      <SelectItem value="paypal">
                        {t("channels.paypal")}
                      </SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
              <Field>
                <FieldLabel>{t("orders.fields.currency")}</FieldLabel>
                <Select
                  value={filters.currency}
                  onValueChange={(value) =>
                    setFilters((current) => ({ ...current, currency: value }))
                  }
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      <SelectItem value="all">{t("orders.all")}</SelectItem>
                      <SelectItem value="CNY">CNY</SelectItem>
                      <SelectItem value="USD">USD</SelectItem>
                      <SelectItem value="EUR">EUR</SelectItem>
                      <SelectItem value="HKD">HKD</SelectItem>
                      <SelectItem value="JPY">JPY</SelectItem>
                      <SelectItem value="GBP">GBP</SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
              <Field>
                <FieldLabel htmlFor="orders-merchant-order-no">
                  {t("orders.fields.merchantOrderNo")}
                </FieldLabel>
                <Input
                  id="orders-merchant-order-no"
                  value={filters.merchantOrderNo}
                  onChange={(event) =>
                    setFilters((current) => ({
                      ...current,
                      merchantOrderNo: event.target.value,
                    }))
                  }
                />
              </Field>
            </FieldGroup>
            <div className="flex gap-2">
              <Button type="submit">
                <Search />
                {t("orders.search")}
              </Button>
              <Button type="button" variant="outline" onClick={resetFilters}>
                {t("orders.reset")}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("orders.title")}</CardTitle>
          <CardDescription>
            {t("orders.total", { count: total })}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <OrdersDataTable
            columns={columns}
            data={orders}
            loading={loading}
            loadingText={t("orders.loading")}
            emptyText={t("orders.empty")}
            pageSize={20}
          />
        </CardContent>
      </Card>

      <AlertDialog
        open={Boolean(closeTarget)}
        onOpenChange={(open) => {
          if (!open) setCloseTarget(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("orders.closeConfirmTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("orders.closeConfirmDescription", {
                orderNo: closeTarget?.gateway_order_no ?? "",
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("orders.cancel")}</AlertDialogCancel>
            <AlertDialogAction onClick={confirmClose}>
              {t("orders.close")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

function toListParams(filters: FilterState): ListOrdersParams {
  return {
    app_id: filters.appId.trim() || undefined,
    status: filters.status === "all" ? undefined : filters.status,
    channel: filters.channel === "all" ? undefined : filters.channel,
    currency: filters.currency === "all" ? undefined : filters.currency,
    merchant_order_no: filters.merchantOrderNo.trim() || undefined,
    page: 1,
    page_size: 100,
  };
}
