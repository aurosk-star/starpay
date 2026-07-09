import { useEffect, useState } from "react";
import { Link, useParams } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { ArrowLeft, KeyRound } from "lucide-react";

import { DetailCard, DetailSkeleton, DetailTable } from "@/components/detail";
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
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useAuthStore } from "@/features/auth/store";
import { APIError } from "@/lib/api";

import { getApp, resetAppSecret } from "./api";
import type { GatewayApp } from "./types";

export function AppDetailPage() {
  const { t } = useTranslation();
  const accessToken = useAuthStore((state) => state.accessToken);
  const params = useParams({ strict: false });
  const appId = "appId" in params ? Number(params.appId) : 0;
  const [app, setApp] = useState<GatewayApp | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [resetOpen, setResetOpen] = useState(false);
  const [secret, setSecret] = useState<string | null>(null);

  useEffect(() => {
    if (!accessToken || !appId) return;
    setLoading(true);
    setError(null);
    void getApp(accessToken, appId)
      .then((result) => setApp(result.app))
      .catch((err) =>
        setError(err instanceof APIError ? err.message : t("apps.loadFailed")),
      )
      .finally(() => setLoading(false));
  }, [accessToken, appId, t]);

  async function confirmResetSecret() {
    if (!accessToken || !app) return;
    try {
      const result = await resetAppSecret(accessToken, app.id);
      setApp(result.app);
      setSecret(result.app_secret);
      setResetOpen(false);
    } catch (err) {
      setError(err instanceof APIError ? err.message : t("apps.resetFailed"));
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div className="flex min-w-0 flex-col gap-2">
          <Button variant="ghost" size="sm" className="w-fit" asChild>
            <Link to="/apps">
              <ArrowLeft data-icon="inline-start" />
              {t("apps.backToList")}
            </Link>
          </Button>
          <div>
            <h1 className="text-2xl font-semibold tracking-tight">
              {app?.name ?? t("apps.detailTitle")}
            </h1>
            <p className="break-all font-mono text-xs text-muted-foreground">
              {app?.app_id ?? "-"}
            </p>
          </div>
        </div>
        <Button
          type="button"
          variant="outline"
          disabled={!app}
          onClick={() => setResetOpen(true)}
        >
          <KeyRound data-icon="inline-start" />
          {t("apps.resetSecret")}
        </Button>
      </div>

      {error ? (
        <Alert variant="destructive">
          <AlertTitle>{t("apps.loadFailed")}</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      ) : null}

      {loading ? (
        <DetailSkeleton />
      ) : app ? (
        <div className="grid min-w-0 gap-4 xl:grid-cols-[minmax(0,1fr)_360px]">
          <DetailCard title={t("apps.detail.basic")}>
            <DetailTable
              rows={[
                [t("apps.appId"), app.app_id],
                [t("apps.name"), app.name],
                [t("apps.status"), app.status],
                [t("apps.detail.createdAt"), new Date(app.created_at).toLocaleString()],
                [t("apps.detail.updatedAt"), new Date(app.updated_at).toLocaleString()],
              ]}
            />
          </DetailCard>
          <DetailCard title={t("apps.detail.security")}>
            <div className="flex flex-col gap-3">
              <div className="flex items-center justify-between rounded-lg border px-3 py-2">
                <span className="text-sm text-muted-foreground">
                  {t("apps.status")}
                </span>
                <Badge variant={app.status === "enabled" ? "secondary" : "outline"}>
                  {app.status === "enabled" ? t("apps.enabled") : t("apps.disabled")}
                </Badge>
              </div>
              <div className="rounded-lg border px-3 py-2">
                <div className="mb-2 text-sm text-muted-foreground">
                  {t("apps.allowedIps")}
                </div>
                <div className="flex flex-wrap gap-1">
                  {app.allowed_ips.length > 0
                    ? app.allowed_ips.map((ip) => (
                        <Badge key={ip} variant="outline">
                          {ip}
                        </Badge>
                      ))
                    : "-"}
                </div>
              </div>
            </div>
          </DetailCard>
          <DetailCard title={t("apps.detail.callbacks")}>
            <DetailTable
              rows={[
                [t("apps.notifyUrl"), app.notify_url ?? ""],
                [t("apps.defaultReturnUrl"), app.default_return_url ?? ""],
              ]}
            />
          </DetailCard>
        </div>
      ) : null}

      <AlertDialog open={resetOpen} onOpenChange={setResetOpen}>
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
    </div>
  );
}
