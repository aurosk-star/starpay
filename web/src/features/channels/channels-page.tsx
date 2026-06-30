import { useEffect, useMemo, useState } from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Pencil, Plus, RefreshCw, ToggleLeft } from "lucide-react";

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
import { useAuthStore } from "@/features/auth/store";
import { APIError } from "@/lib/api";

import {
  disableChannelAccount,
  enableChannelAccount,
  listChannelAccounts,
} from "./api";
import type { ChannelAccount } from "./types";

const ChannelsDataTable = createDataTable<ChannelAccount>();

export function ChannelsPage() {
  const { t } = useTranslation();
  const accessToken = useAuthStore((state) => state.accessToken);
  const [channels, setChannels] = useState<ChannelAccount[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const columns = useMemo<DataTableColumn<ChannelAccount>[]>(
    () => [
      {
        accessorKey: "channel",
        header: t("channels.table.channel"),
        cell: ({ row }) => (
          <div className="flex flex-col">
            <span className="font-medium">
              {t(`channels.${row.original.channel}`)}
            </span>
            <span className="text-xs text-muted-foreground">
              {row.original.name}
            </span>
          </div>
        ),
      },
      {
        accessorKey: "env",
        header: t("channels.table.env"),
        cell: ({ row }) => (
          <Badge variant="outline">{t(`channels.${row.original.env}`)}</Badge>
        ),
      },
      {
        accessorKey: "enabled",
        header: t("channels.table.status"),
        cell: ({ row }) => (
          <Badge variant={row.original.enabled ? "secondary" : "outline"}>
            {row.original.enabled
              ? t("channels.enabled")
              : t("channels.disabled")}
          </Badge>
        ),
      },
      {
        accessorKey: "config",
        header: t("channels.table.config"),
        cell: ({ row }) => (
          <div className="flex max-w-[420px] flex-wrap gap-1">
            {Object.entries(row.original.config).map(([key, value]) => (
              <Badge key={key} variant="outline" className="max-w-full">
                <span className="truncate font-mono text-xs">
                  {key}: {String(value)}
                </span>
              </Badge>
            ))}
          </div>
        ),
      },
      {
        accessorKey: "updated_at",
        header: t("channels.table.updatedAt"),
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
                label: t("channels.edit"),
                asChild: true,
                child: (
                  <Link
                    to="/channels/$channelId/edit"
                    params={{ channelId: String(row.original.id) }}
                  >
                    <Pencil data-icon="inline-start" />
                    {t("channels.edit")}
                  </Link>
                ),
              },
              {
                label: row.original.enabled
                  ? t("channels.disable")
                  : t("channels.enable"),
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
    setError(null);
    try {
      const result = await listChannelAccounts(accessToken);
      setChannels(result.items);
    } catch (err) {
      setError(
        err instanceof APIError ? err.message : t("channels.loadFailed"),
      );
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, [accessToken]);

  async function toggleStatus(channel: ChannelAccount) {
    if (!accessToken) return;
    try {
      const result = channel.enabled
        ? await disableChannelAccount(accessToken, channel.id)
        : await enableChannelAccount(accessToken, channel.id);
      setChannels((current) =>
        current.map((item) =>
          item.id === result.channel_account.id ? result.channel_account : item,
        ),
      );
    } catch (err) {
      setError(
        err instanceof APIError ? err.message : t("channels.statusFailed"),
      );
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">
            {t("channels.title")}
          </h1>
          <p className="text-sm text-muted-foreground">
            {t("channels.description")}
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={load}>
            <RefreshCw />
            {t("common.refresh")}
          </Button>
          <Button asChild>
            <Link to="/channels/new">
              <Plus />
              {t("channels.create")}
            </Link>
          </Button>
        </div>
      </div>

      {error ? (
        <Alert variant="destructive">
          <AlertTitle>{t("channels.loadFailed")}</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      ) : null}

      <Card>
        <CardHeader>
          <CardTitle>{t("channels.title")}</CardTitle>
          <CardDescription>{t("channels.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          {loading ? (
            <p className="text-sm text-muted-foreground">
              {t("channels.loading")}
            </p>
          ) : channels.length > 0 ? (
            <ChannelsDataTable columns={columns} data={channels} />
          ) : (
            <p className="text-sm text-muted-foreground">
              {t("channels.empty")}
            </p>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
