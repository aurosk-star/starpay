import { useEffect, useMemo, useState } from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Pencil, Plus, RefreshCw, ToggleLeft } from "lucide-react";
import { toast } from "sonner";

import {
  createDataTable,
  DataTableRowActions,
  type DataTableColumn,
} from "@/components/data-table";
import { ConfigDrawer } from "@/components/config-drawer";
import { Header } from "@/components/layout/header";
import { Main } from "@/components/layout/main";
import { PageHeader } from "@/components/layout/page-header";
import { ProfileDropdown } from "@/components/profile-dropdown";
import { Search } from "@/components/search";
import { ThemeSwitch } from "@/components/theme-switch";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { useAuthStore } from "@/features/auth/store";
import { APIError } from "@/lib/api";

import { disableRoutingRule, enableRoutingRule, listRoutingRules } from "./api";
import type { RoutingRule } from "./types";

const RoutingDataTable = createDataTable<RoutingRule>();

export function RoutingPage() {
  const { t } = useTranslation();
  const accessToken = useAuthStore((state) => state.accessToken);
  const [rules, setRules] = useState<RoutingRule[]>([]);
  const [loading, setLoading] = useState(true);

  const columns = useMemo<DataTableColumn<RoutingRule>[]>(
    () => [
      {
        accessorKey: "name",
        header: t("routing.table.name"),
        cell: ({ row }) => (
          <div className="flex flex-col">
            <span className="font-medium">{row.original.name}</span>
            <span className="text-xs text-muted-foreground">
              {t("routing.table.priorityValue", {
                priority: row.original.priority,
              })}
            </span>
          </div>
        ),
      },
      {
        accessorKey: "enabled",
        header: t("routing.table.status"),
        cell: ({ row }) => (
          <Badge variant={row.original.enabled ? "secondary" : "outline"}>
            {row.original.enabled
              ? t("routing.enabled")
              : t("routing.disabled")}
          </Badge>
        ),
      },
      {
        accessorKey: "scope",
        header: t("routing.table.scope"),
        cell: ({ row }) => (
          <div className="flex flex-wrap gap-1">
            <Badge variant="outline">
              {row.original.app_scope === "all"
                ? t("routing.appScopes.all")
                : row.original.app_ids.join(", ")}
            </Badge>
            <Badge variant="outline">{row.original.currency || "*"}</Badge>
            <Badge variant="outline">
              {t(`routing.terminals.${row.original.terminal}`)}
            </Badge>
          </div>
        ),
      },
      {
        accessorKey: "amount",
        header: t("routing.table.amount"),
        cell: ({ row }) => (
          <span className="font-mono text-xs">
            {formatAmountRange(
              row.original.min_amount,
              row.original.max_amount,
            )}
          </span>
        ),
      },
      {
        accessorKey: "payment_method",
        header: t("routing.table.target"),
        cell: ({ row }) => (
          <div className="flex flex-wrap gap-1">
            <Badge>{t(`channels.${row.original.payment_method}`)}</Badge>
            {row.original.pay_modes.map((mode) => (
              <Badge key={mode} variant="outline">
                {mode}
              </Badge>
            ))}
            <Badge variant="secondary">
              {t("routing.table.targetCount", {
                count: row.original.targets.length,
              })}
            </Badge>
          </div>
        ),
      },
      {
        accessorKey: "updated_at",
        header: t("routing.table.updatedAt"),
        cell: ({ row }) => (
          <span className="font-mono text-xs text-muted-foreground">
            {new Date(row.original.updated_at).toLocaleString()}
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
                label: t("routing.edit"),
                asChild: true,
                child: (
                  <Link
                    to="/routing/$ruleId/edit"
                    params={{ ruleId: String(row.original.id) }}
                  >
                    <Pencil data-icon="inline-start" />
                    {t("routing.edit")}
                  </Link>
                ),
              },
              {
                label: row.original.enabled
                  ? t("routing.disable")
                  : t("routing.enable"),
                icon: ToggleLeft,
                onClick: () => toggleStatus(row.original),
              },
            ]}
          />
        ),
      },
    ],
    [t],
  );

  async function load() {
    if (!accessToken) return;
    setLoading(true);
    try {
      const result = await listRoutingRules(accessToken);
      setRules(result.items);
    } catch (err) {
      toast.error(
        err instanceof APIError ? err.message : t("routing.loadFailed"),
      );
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, [accessToken]);

  async function toggleStatus(rule: RoutingRule) {
    if (!accessToken) return;
    try {
      const result = rule.enabled
        ? await disableRoutingRule(accessToken, rule.id)
        : await enableRoutingRule(accessToken, rule.id);
      setRules((current) =>
        current.map((item) =>
          item.id === result.routing_rule.id ? result.routing_rule : item,
        ),
      );
      toast.success(
        t(
          rule.enabled
            ? "common.feedback.disabled"
            : "common.feedback.enabled",
        ),
      );
    } catch (err) {
      toast.error(
        err instanceof APIError ? err.message : t("routing.statusFailed"),
      );
    }
  }

  return (
    <>
      <Header fixed>
        <Search className="me-auto" />
        <ThemeSwitch />
        <ConfigDrawer />
        <ProfileDropdown />
      </Header>

      <Main fixed className="flex min-w-0 flex-1 flex-col gap-4 sm:gap-6">
        <PageHeader
          title={t("routing.title")}
          description={t("routing.description")}
          actions={
            <>
              <Button variant="outline" onClick={() => void load()}>
                <RefreshCw data-icon="inline-start" />
                {t("common.refresh")}
              </Button>
              <Button asChild>
                <Link to="/routing/new">
                  <Plus data-icon="inline-start" />
                  {t("routing.create")}
                </Link>
              </Button>
            </>
          }
        />

        <Card>
          <CardHeader>
            <CardTitle>{t("routing.title")}</CardTitle>
            <CardDescription>{t("routing.description")}</CardDescription>
          </CardHeader>
          <CardContent>
            {loading ? (
              <p className="text-sm text-muted-foreground">
                {t("routing.loading")}
              </p>
            ) : rules.length > 0 ? (
              <RoutingDataTable columns={columns} data={rules} viewOptions />
            ) : (
              <p className="text-sm text-muted-foreground">
                {t("routing.empty")}
              </p>
            )}
          </CardContent>
        </Card>
      </Main>
    </>
  );
}

function formatAmountRange(min: number, max: number) {
  if (!min && !max) return "*";
  if (!min) return `<= ${max}`;
  if (!max) return `>= ${min}`;
  return `${min} - ${max}`;
}
