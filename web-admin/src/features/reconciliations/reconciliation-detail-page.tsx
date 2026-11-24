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
import { getReconciliation, retryReconciliation } from "./api";
import type { PaymentReconciliation } from "./types";
export function ReconciliationDetailPage() {
  const { t } = useTranslation();
  const { reconciliationId } = useParams({
    from: "/reconciliations/$reconciliationId",
  });
  const token = useAuthStore((s) => s.accessToken);
  const [item, setItem] = useState<PaymentReconciliation | null>(null);
  useEffect(() => {
    if (!token) return;
    getReconciliation(token, Number(reconciliationId))
      .then((r) => setItem(r.payment_reconciliation))
      .catch((err) =>
        toast.error(
          err instanceof APIError
            ? err.message
            : t("reconciliations.loadFailed"),
        ),
      );
  }, [token, reconciliationId, t]);
  async function retry() {
    if (!token || !item) return;
    try {
      const r = await retryReconciliation(token, item.id);
      setItem(r.payment_reconciliation);
    } catch (err) {
      toast.error(
        err instanceof APIError
          ? err.message
          : t("reconciliations.retryFailed"),
      );
    }
  }
  if (!item)
    return (
      <p className="text-sm text-muted-foreground">{t("common.loading")}</p>
    );
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
          title={t("reconciliations.detailTitle")}
          badge={
            <Badge
              variant={
                item.status === "manual_required" ? "destructive" : "outline"
              }
            >
              {t(`reconciliations.status.${item.status}`)}
            </Badge>
          }
          back={{ to: "/reconciliations", label: t("common.back") }}
          className="sm:items-center"
          actions={
            <Button
              variant="outline"
              disabled={item.status === "resolved"}
              onClick={() => void retry()}
            >
              <RotateCcw data-icon="inline-start" />
              {t("common.retry")}
            </Button>
          }
        />
        <Card>
          <CardHeader>
            <CardTitle>{t("reconciliations.overview")}</CardTitle>
          </CardHeader>
          <CardContent>
            <dl className="grid gap-4 md:grid-cols-2">
              <div>
                <dt className="text-xs text-muted-foreground">
                  {t("reconciliations.fields.order")}
                </dt>
                <dd className="font-mono text-sm">{item.gateway_order_no}</dd>
              </div>
              <div>
                <dt className="text-xs text-muted-foreground">
                  {t("reconciliations.fields.channel")}
                </dt>
                <dd>{item.channel}</dd>
              </div>
              <div>
                <dt className="text-xs text-muted-foreground">
                  {t("reconciliations.fields.attempts")}
                </dt>
                <dd>{item.attempt_count}</dd>
              </div>
              <div>
                <dt className="text-xs text-muted-foreground">
                  {t("reconciliations.fields.nextAttempt")}
                </dt>
                <dd>
                  {item.next_attempt_at
                    ? formatDateTime(item.next_attempt_at)
                    : "-"}
                </dd>
              </div>
            </dl>
          </CardContent>
        </Card>
        {item.last_error ? (
          <Card>
            <CardHeader>
              <CardTitle>{t("reconciliations.failure")}</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-sm text-destructive">{item.last_error}</p>
            </CardContent>
          </Card>
        ) : null}
      </Main>
    </>
  );
}
