import { useEffect, useState } from "react";
import { useParams } from "@tanstack/react-router";
import { RotateCcw } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ConfigDrawer } from "@/components/config-drawer";
import { Header } from "@/components/layout/header";
import { Main } from "@/components/layout/main";
import { PageHeader } from "@/components/page-header";
import { ProfileDropdown } from "@/components/profile-dropdown";
import { Search } from "@/components/search";
import { ThemeSwitch } from "@/components/theme-switch";
import { useAuthStore } from "@/features/auth/store";
import { APIError } from "@/lib/api";
import { formatDateTime } from "@/lib/date";
import { formatMinorAmount } from "@/lib/money";
import { getRefund, retryRefund } from "./api";
import type { Refund } from "./types";
export function RefundDetailPage() {
  const { t } = useTranslation();
  const { refundId } = useParams({ from: "/refunds/$refundId" });
  const token = useAuthStore((s) => s.accessToken);
  const [item, setItem] = useState<Refund | null>(null);
  useEffect(() => {
    if (!token) return;
    getRefund(token, Number(refundId))
      .then((r) => setItem(r.refund))
      .catch((err) =>
        toast.error(
          err instanceof APIError ? err.message : t("refunds.loadFailed"),
        ),
      );
  }, [token, refundId, t]);
  async function retry() {
    if (!token || !item) return;
    try {
      const r = await retryRefund(token, item.id);
      setItem(r.refund);
    } catch (err) {
      toast.error(
        err instanceof APIError ? err.message : t("refunds.retryFailed"),
      );
    }
  }
  if (!item)
    return (
      <p className="text-sm text-muted-foreground">{t("common.loading")}</p>
    );
  const rows = [
    [t("refunds.fields.refundNo"), item.refund_no],
    [t("refunds.fields.merchantRefund"), item.merchant_refund_no],
    [t("refunds.fields.order"), item.gateway_order_no],
    [t("refunds.fields.amount"), formatMinorAmount(item.amount, item.currency)],
    [t("refunds.fields.channel"), item.channel],
    [t("refunds.fields.channelRefund"), item.channel_refund_no || "-"],
    [t("refunds.fields.attempts"), String(item.attempt_count)],
    [t("refunds.fields.createdAt"), formatDateTime(item.created_at)],
  ];
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
          title={t("refunds.detailTitle")}
          badge={
            <Badge
              variant={item.status === "failed" ? "destructive" : "outline"}
            >
              {t(`refunds.status.${item.status}`)}
            </Badge>
          }
          back={{ to: "/refunds", label: t("common.back") }}
          className="sm:items-center"
          actions={
            <Button
              variant="outline"
              disabled={item.status === "succeeded"}
              onClick={() => void retry()}
            >
              <RotateCcw data-icon="inline-start" />
              {t("common.retry")}
            </Button>
          }
        />
        <Card>
          <CardHeader>
            <CardTitle>{t("refunds.overview")}</CardTitle>
          </CardHeader>
          <CardContent>
            <dl className="grid gap-4 md:grid-cols-2">
              {rows.map(([label, value]) => (
                <div key={label} className="min-w-0">
                  <dt className="text-xs text-muted-foreground">{label}</dt>
                  <dd className="break-words font-mono text-sm">{value}</dd>
                </div>
              ))}
            </dl>
          </CardContent>
        </Card>
        {item.failure_reason || item.last_error ? (
          <Card>
            <CardHeader>
              <CardTitle>{t("refunds.failure")}</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-sm text-destructive">
                {item.failure_reason || item.last_error}
              </p>
            </CardContent>
          </Card>
        ) : null}
      </Main>
    </>
  );
}
