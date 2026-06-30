import { useEffect, useMemo, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { KeyRound, Pencil, Plus, RefreshCw, ToggleLeft } from "lucide-react";

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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
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

import {
  createApp,
  disableApp,
  enableApp,
  listApps,
  resetAppSecret,
  updateApp,
} from "./api";
import type { GatewayApp } from "./types";

const AppsDataTable = createDataTable<GatewayApp>();

const emptyForm = {
  appId: "",
  name: "",
  notifyUrl: "",
  allowedIps: "",
  status: "enabled",
};

type FormState = typeof emptyForm;

export function AppsPage() {
  const { t } = useTranslation();
  const accessToken = useAuthStore((state) => state.accessToken);
  const [apps, setApps] = useState<GatewayApp[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingApp, setEditingApp] = useState<GatewayApp | null>(null);
  const [form, setForm] = useState<FormState>(emptyForm);
  const [saving, setSaving] = useState(false);
  const [secret, setSecret] = useState<string | null>(null);
  const [resetTarget, setResetTarget] = useState<GatewayApp | null>(null);

  const columns = useMemo<DataTableColumn<GatewayApp>[]>(
    () => [
      {
        accessorKey: "app_id",
        header: t("apps.table.app"),
        cell: ({ row }) => (
          <div className="flex flex-col">
            <span className="font-medium">{row.original.name}</span>
            <span className="font-mono text-xs text-muted-foreground">
              {row.original.app_id}
            </span>
          </div>
        ),
      },
      {
        accessorKey: "notify_url",
        header: t("apps.table.notifyUrl"),
        cell: ({ row }) => (
          <span className="block max-w-[320px] truncate font-mono text-xs">
            {row.original.notify_url || "-"}
          </span>
        ),
      },
      {
        accessorKey: "allowed_ips",
        header: t("apps.table.allowedIps"),
        cell: ({ row }) => (
          <div className="flex flex-wrap gap-1">
            {row.original.allowed_ips.length > 0
              ? row.original.allowed_ips.map((ip) => (
                  <Badge key={ip} variant="outline">
                    {ip}
                  </Badge>
                ))
              : "-"}
          </div>
        ),
      },
      {
        accessorKey: "status",
        header: t("apps.table.status"),
        cell: ({ row }) => (
          <Badge
            variant={
              row.original.status === "enabled" ? "secondary" : "outline"
            }
          >
            {row.original.status === "enabled"
              ? t("apps.enabled")
              : t("apps.disabled")}
          </Badge>
        ),
      },
      {
        accessorKey: "updated_at",
        header: t("apps.table.updatedAt"),
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
                label: t("apps.edit"),
                icon: Pencil,
                onClick: () => openEdit(row.original),
              },
              {
                label:
                  row.original.status === "enabled"
                    ? t("apps.disable")
                    : t("apps.enable"),
                icon: ToggleLeft,
                onClick: () => toggleStatus(row.original),
              },
              {
                label: t("apps.resetSecret"),
                icon: KeyRound,
                onClick: () => setResetTarget(row.original),
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
      const result = await listApps(accessToken);
      setApps(result.items);
    } catch (err) {
      setError(err instanceof APIError ? err.message : t("apps.loadFailed"));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, [accessToken]);

  function openCreate() {
    setEditingApp(null);
    setForm(emptyForm);
    setDialogOpen(true);
  }

  function openEdit(app: GatewayApp) {
    setEditingApp(app);
    setForm({
      appId: app.app_id,
      name: app.name,
      notifyUrl: app.notify_url || "",
      allowedIps: app.allowed_ips.join(", "),
      status: app.status,
    });
    setDialogOpen(true);
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!accessToken) return;
    setSaving(true);
    setError(null);
    try {
      const payload = {
        app_id: editingApp ? undefined : form.appId,
        name: form.name,
        notify_url: form.notifyUrl || undefined,
        allowed_ips: form.allowedIps
          .split(/[\n,]/)
          .map((item) => item.trim())
          .filter(Boolean),
        status: form.status,
      };
      if (editingApp) {
        const result = await updateApp(accessToken, editingApp.id, payload);
        setApps((current) =>
          current.map((item) =>
            item.id === result.app.id ? result.app : item,
          ),
        );
      } else {
        const result = await createApp(accessToken, payload);
        setApps((current) => [result.app, ...current]);
        setSecret(result.app_secret);
      }
      setDialogOpen(false);
      setEditingApp(null);
      setForm(emptyForm);
    } catch (err) {
      setError(err instanceof APIError ? err.message : t("apps.saveFailed"));
    } finally {
      setSaving(false);
    }
  }

  async function toggleStatus(app: GatewayApp) {
    if (!accessToken) return;
    try {
      const result =
        app.status === "enabled"
          ? await disableApp(accessToken, app.id)
          : await enableApp(accessToken, app.id);
      setApps((current) =>
        current.map((item) => (item.id === result.app.id ? result.app : item)),
      );
    } catch (err) {
      setError(err instanceof APIError ? err.message : t("apps.statusFailed"));
    }
  }

  async function confirmResetSecret() {
    if (!accessToken || !resetTarget) return;
    try {
      const result = await resetAppSecret(accessToken, resetTarget.id);
      setApps((current) =>
        current.map((item) => (item.id === result.app.id ? result.app : item)),
      );
      setSecret(result.app_secret);
      setResetTarget(null);
    } catch (err) {
      setError(err instanceof APIError ? err.message : t("apps.resetFailed"));
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">
            {t("apps.title")}
          </h1>
          <p className="text-sm text-muted-foreground">
            {t("apps.description")}
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={load}>
            <RefreshCw />
            {t("common.refresh")}
          </Button>
          <Button onClick={openCreate}>
            <Plus />
            {t("apps.create")}
          </Button>
        </div>
      </div>

      {error ? (
        <Alert variant="destructive">
          <AlertTitle>{t("apps.loadFailed")}</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      ) : null}

      <Card>
        <CardHeader>
          <CardTitle>{t("apps.title")}</CardTitle>
          <CardDescription>{t("apps.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          {loading ? (
            <p className="text-sm text-muted-foreground">{t("apps.loading")}</p>
          ) : apps.length > 0 ? (
            <AppsDataTable columns={columns} data={apps} />
          ) : (
            <p className="text-sm text-muted-foreground">{t("apps.empty")}</p>
          )}
        </CardContent>
      </Card>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {editingApp ? t("apps.editTitle") : t("apps.createTitle")}
            </DialogTitle>
            <DialogDescription>{t("apps.formDescription")}</DialogDescription>
          </DialogHeader>
          <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="app_id">{t("apps.appId")}</FieldLabel>
                <Input
                  id="app_id"
                  value={form.appId}
                  disabled={Boolean(editingApp)}
                  required
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      appId: event.target.value,
                    }))
                  }
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="name">{t("apps.name")}</FieldLabel>
                <Input
                  id="name"
                  value={form.name}
                  required
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      name: event.target.value,
                    }))
                  }
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="notify_url">
                  {t("apps.notifyUrl")}
                </FieldLabel>
                <Input
                  id="notify_url"
                  value={form.notifyUrl}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      notifyUrl: event.target.value,
                    }))
                  }
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="allowed_ips">
                  {t("apps.allowedIps")}
                </FieldLabel>
                <Input
                  id="allowed_ips"
                  value={form.allowedIps}
                  placeholder={t("apps.allowedIpsHint")}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      allowedIps: event.target.value,
                    }))
                  }
                />
              </Field>
              <Field>
                <FieldLabel>{t("apps.status")}</FieldLabel>
                <Select
                  value={form.status}
                  onValueChange={(value) =>
                    setForm((current) => ({ ...current, status: value }))
                  }
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      <SelectItem value="enabled">
                        {t("apps.enabled")}
                      </SelectItem>
                      <SelectItem value="disabled">
                        {t("apps.disabled")}
                      </SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
            </FieldGroup>
            <div className="flex justify-end gap-2">
              <Button
                type="button"
                variant="outline"
                onClick={() => setDialogOpen(false)}
              >
                {t("apps.cancel")}
              </Button>
              <Button type="submit" disabled={saving}>
                {t("apps.save")}
              </Button>
            </div>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(secret)} onOpenChange={() => setSecret(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("apps.secretTitle")}</DialogTitle>
            <DialogDescription>{t("apps.secretDescription")}</DialogDescription>
          </DialogHeader>
          <div className="max-w-full overflow-x-auto break-all rounded-md border bg-muted p-3 font-mono text-sm leading-6">
            {secret}
          </div>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={Boolean(resetTarget)}
        onOpenChange={(open) => {
          if (!open) setResetTarget(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("apps.resetConfirmTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("apps.resetConfirmDescription")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("apps.cancel")}</AlertDialogCancel>
            <AlertDialogAction onClick={confirmResetSecret}>
              {t("apps.resetSecret")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
