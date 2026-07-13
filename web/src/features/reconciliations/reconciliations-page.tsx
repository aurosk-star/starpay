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
import { listReconciliations, retryReconciliation } from "./api";
import type { PaymentReconciliation } from "./types";
const Table = createDataTable<PaymentReconciliation>();
const defaults = { status: "all", channel: "all", gateway_order_no: "" };
export function ReconciliationsPage() {
  const { t } = useTranslation();
  const token = useAuthStore((s) => s.accessToken);
  const [items, setItems] = useState<PaymentReconciliation[]>([]);
  const [loading, setLoading] = useState(true);
  const [filters, setFilters] = useState(defaults);
  const [applied, setApplied] = useState(defaults);
  const columns = useMemo<DataTableColumn<PaymentReconciliation>[]>(
    () => [
      {
        accessorKey: "gateway_order_no",
        header: t("reconciliations.table.order"),
        cell: ({ row }) => (
          <span className="font-mono text-xs">
            {row.original.gateway_order_no}
          </span>
        ),
      },
      { accessorKey: "channel", header: t("reconciliations.table.channel") },
      {
        accessorKey: "status",
        header: t("reconciliations.table.status"),
        cell: ({ row }) => (
          <Badge
            variant={
              row.original.status === "manual_required"
                ? "destructive"
                : "outline"
            }
          >
            {t(`reconciliations.status.${row.original.status}`)}
          </Badge>
        ),
      },
      {
        accessorKey: "attempt_count",
        header: t("reconciliations.table.attempts"),
      },
      {
        accessorKey: "last_provider_status",
        header: t("reconciliations.table.providerStatus"),
      },
      {
        accessorKey: "next_attempt_at",
        header: t("reconciliations.table.nextAttempt"),
        cell: ({ row }) =>
          row.original.next_attempt_at
            ? formatDateTime(row.original.next_attempt_at)
            : "-",
      },
      {
        id: "actions",
        cell: ({ row }) => (
          <DataTableRowActions
            actions={[
              {
                label: t("common.view"),
                icon: Eye,
                asChild: true,
                child: (
                  <Link
                    to="/reconciliations/$reconciliationId"
                    params={{ reconciliationId: String(row.original.id) }}
                  >
                    <Eye data-icon="inline-start" />
                    {t("common.view")}
                  </Link>
                ),
              },
              {
                label: t("common.retry"),
                icon: RotateCcw,
                disabled: row.original.status === "resolved",
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
      const r = await listReconciliations(token, { ...next, page_size: 100 });
      setItems(r.items);
    } catch (err) {
      toast.error(
        err instanceof APIError ? err.message : t("reconciliations.loadFailed"),
      );
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => {
    void load();
  }, [token, applied]);
  async function retry(item: PaymentReconciliation) {
    if (!token) return;
    try {
      const r = await retryReconciliation(token, item.id);
      setItems((v) =>
        v.map((x) => (x.id === item.id ? r.payment_reconciliation : x)),
      );
    } catch (err) {
      toast.error(
        err instanceof APIError
          ? err.message
          : t("reconciliations.retryFailed"),
      );
    }
  }
  function submit(e: FormEvent) {
    e.preventDefault();
    setApplied(filters);
  }
  return (
    <div className="flex flex-col gap-5">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold">
            {t("reconciliations.title")}
          </h1>
          <p className="text-sm text-muted-foreground">
            {t("reconciliations.description")}
          </p>
        </div>
        <Button variant="outline" onClick={() => void load()}>
          <RefreshCw />
          {t("common.refresh")}
        </Button>
      </div>
      <Card>
        <CardHeader>
          <CardTitle>{t("reconciliations.filters")}</CardTitle>
        </CardHeader>
        <CardContent>
          <form className="flex flex-col gap-4" onSubmit={submit}>
            <FieldGroup className="grid md:grid-cols-3">
              <Field>
                <FieldLabel>{t("reconciliations.fields.status")}</FieldLabel>
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
                      {[
                        "all",
                        "pending",
                        "processing",
                        "resolved",
                        "manual_required",
                      ].map((x) => (
                        <SelectItem key={x} value={x}>
                          {x === "all"
                            ? t("common.all")
                            : t(`reconciliations.status.${x}`)}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
              <Field>
                <FieldLabel>{t("reconciliations.fields.channel")}</FieldLabel>
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
                <FieldLabel>{t("reconciliations.fields.order")}</FieldLabel>
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
