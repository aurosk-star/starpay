import { useEffect, useMemo, useState, type FormEvent } from "react";
import { Link } from "@tanstack/react-router";
import { Eye, RefreshCw, RotateCcw, Search } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import {
  createDataTable,
  DataTableRowActions,
  type DataTableColumn,
} from "@/components/data-table";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
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
import { listRefunds, retryRefund } from "./api";
import { RefundCreateDialog } from "./refund-create-dialog";
import type { Refund } from "./types";
const Table = createDataTable<Refund>();
const defaults = {
  app_id: "",
  status: "all",
  channel: "all",
  gateway_order_no: "",
  merchant_refund_no: "",
};
export function RefundsPage() {
  const { t } = useTranslation();
  const token = useAuthStore((s) => s.accessToken);
  const [items, setItems] = useState<Refund[]>([]);
  const [loading, setLoading] = useState(true);
  const [filters, setFilters] = useState(defaults);
  const [applied, setApplied] = useState(defaults);
  const columns = useMemo<DataTableColumn<Refund>[]>(
    () => [
      {
        accessorKey: "refund_no",
        header: t("refunds.table.refund"),
        cell: ({ row }) => (
          <span className="font-mono text-xs">{row.original.refund_no}</span>
        ),
      },
      {
        accessorKey: "gateway_order_no",
        header: t("refunds.table.order"),
        cell: ({ row }) => (
          <span className="font-mono text-xs">
            {row.original.gateway_order_no}
          </span>
        ),
      },
      {
        accessorKey: "merchant_refund_no",
        header: t("refunds.table.merchantRefund"),
      },
      {
        accessorKey: "amount",
        header: t("refunds.table.amount"),
        cell: ({ row }) =>
          formatMinorAmount(row.original.amount, row.original.currency),
      },
      { accessorKey: "channel", header: t("refunds.table.channel") },
      {
        accessorKey: "status",
        header: t("refunds.table.status"),
        cell: ({ row }) => (
          <Badge
            variant={
              row.original.status === "failed" ? "destructive" : "outline"
            }
          >
            {t(`refunds.status.${row.original.status}`)}
          </Badge>
        ),
      },
      {
        accessorKey: "created_at",
        header: t("refunds.table.createdAt"),
        cell: ({ row }) => formatDateTime(row.original.created_at),
      },
      {
        id: "actions",
        header: () => null,
        cell: ({ row }) => (
          <DataTableRowActions
            actions={[
              {
                label: t("common.view"),
                icon: Eye,
                asChild: true,
                child: (
                  <Link
                    to="/refunds/$refundId"
                    params={{ refundId: String(row.original.id) }}
                  >
                    <Eye data-icon="inline-start" />
                    {t("common.view")}
                  </Link>
                ),
              },
              {
                label: t("common.retry"),
                icon: RotateCcw,
                disabled: row.original.status === "succeeded",
                onClick: () => void retry(row.original),
              },
            ]}
          />
        ),
      },
    ],
    [t],
  );
  async function load(next = applied) {
    if (!token) return;
    setLoading(true);
    try {
      const result = await listRefunds(token, { ...next, page_size: 100 });
      setItems(result.items);
    } catch (err) {
      toast.error(
        err instanceof APIError ? err.message : t("refunds.loadFailed"),
      );
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => {
    void load();
  }, [token, applied]);
  async function retry(item: Refund) {
    if (!token) return;
    try {
      const result = await retryRefund(token, item.id);
      setItems((v) => v.map((x) => (x.id === item.id ? result.refund : x)));
    } catch (err) {
      toast.error(
        err instanceof APIError ? err.message : t("refunds.retryFailed"),
      );
    }
  }
  function submit(e: FormEvent) {
    e.preventDefault();
    setApplied(filters);
  }
  return (
    <div className="flex min-w-0 flex-col gap-5">
      <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div>
          <h1 className="text-2xl font-semibold">{t("refunds.title")}</h1>
          <p className="text-sm text-muted-foreground">
            {t("refunds.description")}
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => void load()}>
            <RefreshCw />
            {t("common.refresh")}
          </Button>
          <RefundCreateDialog
            onCreated={(refund) => setItems((v) => [refund, ...v])}
          />
        </div>
      </div>
      <Card>
        <CardHeader>
          <CardTitle>{t("refunds.filters")}</CardTitle>
        </CardHeader>
        <CardContent>
          <form className="flex flex-col gap-4" onSubmit={submit}>
            <FieldGroup className="grid md:grid-cols-2 xl:grid-cols-5">
              <Field>
                <FieldLabel>{t("refunds.fields.app")}</FieldLabel>
                <Input
                  value={filters.app_id}
                  onChange={(e) =>
                    setFilters((v) => ({ ...v, app_id: e.target.value }))
                  }
                />
              </Field>
              <Field>
                <FieldLabel>{t("refunds.fields.status")}</FieldLabel>
                <Select
                  value={filters.status}
                  onValueChange={(status) =>
                    setFilters((v) => ({ ...v, status }))
                  }
                >
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      {["all", "pending", "succeeded", "failed", "closed"].map(
                        (x) => (
                          <SelectItem key={x} value={x}>
                            {x === "all"
                              ? t("common.all")
                              : t(`refunds.status.${x}`)}
                          </SelectItem>
                        ),
                      )}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
              <Field>
                <FieldLabel>{t("refunds.fields.channel")}</FieldLabel>
                <Select
                  value={filters.channel}
                  onValueChange={(channel) =>
                    setFilters((v) => ({ ...v, channel }))
                  }
                >
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      {["all", "alipay", "wechat", "paypal"].map((x) => (
                        <SelectItem key={x} value={x}>
                          {x === "all" ? t("common.all") : x}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
              <Field>
                <FieldLabel>{t("refunds.fields.order")}</FieldLabel>
                <Input
                  value={filters.gateway_order_no}
                  onChange={(e) =>
                    setFilters((v) => ({
                      ...v,
                      gateway_order_no: e.target.value,
                    }))
                  }
                />
              </Field>
              <Field>
                <FieldLabel>{t("refunds.fields.merchantRefund")}</FieldLabel>
                <Input
                  value={filters.merchant_refund_no}
                  onChange={(e) =>
                    setFilters((v) => ({
                      ...v,
                      merchant_refund_no: e.target.value,
                    }))
                  }
                />
              </Field>
            </FieldGroup>
            <div>
              <Button type="submit">
                <Search />
                {t("common.search")}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
      <Card>
        <CardContent className="pt-6">
          <Table
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
