import { useEffect, useMemo, useState, type FormEvent } from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Eye, RefreshCw, RotateCcw, Search } from "lucide-react";

import {
  createDataTable,
  DataTableRowActions,
  type DataTableColumn,
} from "@/components/data-table";
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
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useAuthStore } from "@/features/auth/store";
import { APIError } from "@/lib/api";

import { listWebhookDeliveries, retryWebhookDelivery } from "./api";
import { formatWebhookEventType } from "./event-types";
import type { WebhookDelivery } from "./types";
import { webhookStatusVariant } from "./utils";

const WebhooksDataTable = createDataTable<WebhookDelivery>();

const defaultFilters = {
  appId: "",
  eventType: "",
  status: "all",
  gatewayOrderNo: "",
};

export function WebhooksPage() {
  const { t } = useTranslation();
  const accessToken = useAuthStore((state) => state.accessToken);
  const [items, setItems] = useState<WebhookDelivery[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filters, setFilters] = useState(defaultFilters);
  const [appliedFilters, setAppliedFilters] = useState(defaultFilters);

  const columns = useMemo<DataTableColumn<WebhookDelivery>[]>(
    () => [
      {
        accessorKey: "delivery_no",
        header: t("webhooks.table.deliveryNo"),
        cell: ({ row }) => (
          <span className="block max-w-48 truncate font-mono text-xs">
            {row.original.delivery_no}
          </span>
        ),
      },
      {
        accessorKey: "event_type",
        header: t("webhooks.table.eventType"),
        cell: ({ row }) => formatWebhookEventType(row.original.event_type, t),
      },
      {
        accessorKey: "app_id",
        header: t("webhooks.table.app"),
        cell: ({ row }) => (
          <span className="block max-w-40 truncate font-mono text-xs">
            {row.original.app_id}
          </span>
        ),
      },
      {
        accessorKey: "gateway_order_no",
        header: t("webhooks.table.orderNo"),
        cell: ({ row }) => (
          <span className="block max-w-48 truncate font-mono text-xs">
            {row.original.gateway_order_no}
          </span>
        ),
      },
      {
        accessorKey: "status",
        header: t("webhooks.table.status"),
        cell: ({ row }) => (
          <Badge variant={webhookStatusVariant(row.original.status)}>
            {t(`webhooks.status.${row.original.status}`)}
          </Badge>
        ),
      },
      { accessorKey: "attempt_count", header: t("webhooks.table.attempts") },
      {
        accessorKey: "last_status_code",
        header: t("webhooks.table.lastStatus"),
        cell: ({ row }) => row.original.last_status_code ?? "-",
      },
      {
        accessorKey: "next_attempt_at",
        header: t("webhooks.table.nextAttempt"),
        cell: ({ row }) =>
          row.original.next_attempt_at
            ? new Date(row.original.next_attempt_at).toLocaleString()
            : "-",
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
                label: t("webhooks.view"),
                icon: Eye,
                asChild: true,
                child: (
                  <Link
                    to="/webhooks/$deliveryId"
                    params={{ deliveryId: String(row.original.id) }}
                  >
                    <Eye data-icon="inline-start" />
                    {t("webhooks.view")}
                  </Link>
                ),
              },
              {
                label: t("webhooks.retry"),
                icon: RotateCcw,
                disabled: row.original.status === "succeeded",
                onClick: () => retry(row.original),
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
      const result = await listWebhookDeliveries(accessToken, {
        app_id: nextFilters.appId,
        event_type: nextFilters.eventType,
        status: nextFilters.status === "all" ? "" : nextFilters.status,
        gateway_order_no: nextFilters.gatewayOrderNo,
        page: 1,
        page_size: 100,
      });
      setItems(result.items);
    } catch (err) {
      setError(
        err instanceof APIError ? err.message : t("webhooks.loadFailed"),
      );
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, [accessToken, appliedFilters]);

  async function retry(delivery: WebhookDelivery) {
    if (!accessToken) return;
    try {
      const result = await retryWebhookDelivery(accessToken, delivery.id);
      setItems((current) =>
        current.map((item) =>
          item.id === result.webhook_delivery.id
            ? result.webhook_delivery
            : item,
        ),
      );
    } catch (err) {
      setError(
        err instanceof APIError ? err.message : t("webhooks.retryFailed"),
      );
    }
  }

  function applyFilters(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setAppliedFilters(filters);
  }

  function resetFilters() {
    setFilters(defaultFilters);
    setAppliedFilters(defaultFilters);
  }

  return (
    <div className="flex min-w-0 max-w-full flex-col gap-5">
      <div className="flex min-w-0 flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div className="min-w-0">
          <h1 className="text-2xl font-semibold tracking-tight">
            {t("webhooks.title")}
          </h1>
          <p className="text-sm text-muted-foreground">
            {t("webhooks.description")}
          </p>
        </div>
        <Button
          className="w-full md:w-auto"
          variant="outline"
          onClick={() => load()}
        >
          <RefreshCw />
          {t("common.refresh")}
        </Button>
      </div>
      {error ? (
        <Alert variant="destructive">
          <AlertTitle>{t("webhooks.loadFailed")}</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      ) : null}
      <Card className="min-w-0 max-w-full">
        <CardHeader>
          <CardTitle>{t("webhooks.filters")}</CardTitle>
          <CardDescription>{t("webhooks.description")}</CardDescription>
        </CardHeader>
        <CardContent className="min-w-0">
          <form
            className="grid min-w-0 gap-4 md:grid-cols-[repeat(4,minmax(0,1fr))]"
            onSubmit={applyFilters}
          >
            <FieldGroup>
              <Field>
                <FieldLabel>{t("webhooks.filter.app")}</FieldLabel>
                <Input
                  value={filters.appId}
                  onChange={(e) =>
                    setFilters((curr) => ({ ...curr, appId: e.target.value }))
                  }
                />
              </Field>
            </FieldGroup>
            <FieldGroup>
              <Field>
                <FieldLabel>{t("webhooks.filter.eventType")}</FieldLabel>
                <Input
                  value={filters.eventType}
                  onChange={(e) =>
                    setFilters((curr) => ({
                      ...curr,
                      eventType: e.target.value,
                    }))
                  }
                />
              </Field>
            </FieldGroup>
            <FieldGroup>
              <Field>
                <FieldLabel>{t("webhooks.filter.status")}</FieldLabel>
                <Select
                  value={filters.status}
                  onValueChange={(value) =>
                    setFilters((curr) => ({ ...curr, status: value }))
                  }
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">{t("common.all")}</SelectItem>
                    <SelectItem value="pending">
                      {t("webhooks.status.pending")}
                    </SelectItem>
                    <SelectItem value="succeeded">
                      {t("webhooks.status.succeeded")}
                    </SelectItem>
                    <SelectItem value="failed">
                      {t("webhooks.status.failed")}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </Field>
            </FieldGroup>
            <FieldGroup>
              <Field>
                <FieldLabel>{t("webhooks.filter.orderNo")}</FieldLabel>
                <Input
                  value={filters.gatewayOrderNo}
                  onChange={(e) =>
                    setFilters((curr) => ({
                      ...curr,
                      gatewayOrderNo: e.target.value,
                    }))
                  }
                />
              </Field>
            </FieldGroup>
            <div className="flex flex-col gap-2 md:col-span-4 sm:flex-row">
              <Button type="submit" className="w-full sm:w-auto">
                <Search />
                {t("common.search")}
              </Button>
              <Button
                type="button"
                variant="outline"
                className="w-full sm:w-auto"
                onClick={resetFilters}
              >
                {t("common.reset")}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
      <Card className="min-w-0 max-w-full">
        <CardContent className="min-w-0 pt-6">
          <WebhooksDataTable
            columns={columns}
            data={items}
            loading={loading}
            pageSize={20}
          />
        </CardContent>
      </Card>
    </div>
  );
}
