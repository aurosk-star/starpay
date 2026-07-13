import { useEffect, useState } from "react";
import { useParams } from "@tanstack/react-router";
import { RotateCcw } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
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
    <div className="flex flex-col gap-5">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold">
            {t("reconciliations.detailTitle")}
          </h1>
          <Badge
            variant={
              item.status === "manual_required" ? "destructive" : "outline"
            }
          >
            {t(`reconciliations.status.${item.status}`)}
          </Badge>
        </div>
        <Button
          variant="outline"
          disabled={item.status === "resolved"}
          onClick={() => void retry()}
        >
          <RotateCcw />
          {t("common.retry")}
        </Button>
      </div>
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
    </div>
  );
}
