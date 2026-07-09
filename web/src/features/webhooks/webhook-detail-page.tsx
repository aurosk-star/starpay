import { useEffect, useMemo, useState } from "react";
import { Link, useParams } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { ArrowLeft, RotateCcw } from "lucide-react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  DetailCard,
  DetailSkeleton,
  DetailTable,
  ReadOnlyBlock,
} from "@/components/detail";
import { useAuthStore } from "@/features/auth/store";
import { APIError } from "@/lib/api";
import { formatDateTime } from "@/lib/date";

import { getWebhookDelivery, retryWebhookDelivery } from "./api";
import { formatWebhookEventType } from "./event-types";
import type { WebhookDelivery } from "./types";
import { webhookStatusVariant } from "./utils";

export function WebhookDetailPage() {
  const { t } = useTranslation();
  const { deliveryId } = useParams({ from: "/webhooks/$deliveryId" });
  const accessToken = useAuthStore((state) => state.accessToken);
  const [delivery, setDelivery] = useState<WebhookDelivery | null>(null);
  const [loading, setLoading] = useState(true);
  const [retrying, setRetrying] = useState(false);

  const overviewRows = useMemo<Array<[string, string]>>(
    () =>
      delivery
        ? [
            [t("webhooks.detail.deliveryNo"), delivery.delivery_no],
            [
              t("webhooks.detail.eventType"),
              formatWebhookEventType(delivery.event_type, t),
            ],
            [t("webhooks.detail.app"), delivery.app_id],
            [t("webhooks.detail.orderNo"), delivery.gateway_order_no],
            [t("webhooks.detail.targetUrl"), delivery.target_url],
          ]
        : [],
    [delivery, t],
  );

  const deliveryRows = useMemo<Array<[string, string]>>(
    () =>
      delivery
        ? [
            [t("webhooks.detail.attemptCount"), String(delivery.attempt_count)],
            [
              t("webhooks.detail.lastStatus"),
              delivery.last_status_code
                ? String(delivery.last_status_code)
                : "-",
            ],
            [
              t("webhooks.detail.nextAttempt"),
              formatDateTime(delivery.next_attempt_at),
            ],
            [
              t("webhooks.detail.lastAttempt"),
              formatDateTime(delivery.last_attempt_at),
            ],
            [
              t("webhooks.detail.succeededAt"),
              formatDateTime(delivery.succeeded_at),
            ],
            [
              t("webhooks.detail.createdAt"),
              formatDateTime(delivery.created_at),
            ],
          ]
        : [],
    [delivery, t],
  );

  async function load() {
    if (!accessToken) return;
    setLoading(true);
    try {
      const result = await getWebhookDelivery(accessToken, Number(deliveryId));
      setDelivery(result.webhook_delivery);
    } catch (err) {
      toast.error(
        err instanceof APIError ? err.message : t("webhooks.detail.loadFailed"),
      );
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, [accessToken, deliveryId]);

  async function retry() {
    if (!accessToken || !delivery) return;
    setRetrying(true);
    try {
      const result = await retryWebhookDelivery(accessToken, delivery.id);
      setDelivery(result.webhook_delivery);
    } catch (err) {
      toast.error(
        err instanceof APIError ? err.message : t("webhooks.retryFailed"),
      );
    } finally {
      setRetrying(false);
    }
  }

  return (
    <div className="flex min-w-0 max-w-full flex-col gap-5">
      <div className="flex min-w-0 flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div className="flex min-w-0 items-start gap-3">
          <Button variant="outline" size="icon" asChild>
            <Link to="/webhooks" aria-label={t("common.back")}>
              <ArrowLeft />
            </Link>
          </Button>
          <div className="min-w-0 flex flex-col gap-1">
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-2xl font-semibold tracking-tight">
                {t("webhooks.detailTitle")}
              </h1>
              {delivery ? (
                <Badge variant={webhookStatusVariant(delivery.status)}>
                  {t(`webhooks.status.${delivery.status}`)}
                </Badge>
              ) : null}
            </div>
            <p className="text-sm text-muted-foreground">
              {t("webhooks.detail.description")}
            </p>
          </div>
        </div>
        <Button
          className="w-full md:w-auto"
          variant="outline"
          onClick={retry}
          disabled={!delivery || delivery.status === "succeeded" || retrying}
        >
          <RotateCcw />
          {t("webhooks.retry")}
        </Button>
      </div>

      {loading ? (
        <DetailSkeleton />
      ) : delivery ? (
        <div className="grid min-w-0 max-w-full gap-4 xl:grid-cols-[minmax(0,1fr)_420px]">
          <div className="flex min-w-0 flex-col gap-4">
            <DetailCard
              title={t("webhooks.detail.overview")}
              description={delivery.delivery_no}
            >
              <DetailTable rows={overviewRows} />
            </DetailCard>

            <DetailCard
              title={t("webhooks.detail.attempts")}
              description={t("webhooks.detail.requestNote")}
            >
              <DetailTable rows={deliveryRows} />
            </DetailCard>

            <DetailCard title={t("webhooks.detail.failure")}>
              <div className="flex min-w-0 flex-col gap-3">
                <ReadOnlyBlock
                  label={t("webhooks.detail.lastError")}
                  value={delivery.last_error || "-"}
                />
                <ReadOnlyBlock
                  label={t("webhooks.detail.lastResponseBody")}
                  value={delivery.last_response_body || "-"}
                  monospace
                />
              </div>
            </DetailCard>
          </div>

          <div className="flex min-w-0 flex-col gap-4">
            <DetailCard title={t("webhooks.detail.requestTitle")}>
              <div className="flex min-w-0 flex-col gap-3">
                <ReadOnlyBlock
                  label={t("webhooks.detail.deliveryNo")}
                  value={delivery.delivery_no}
                  monospace
                />
                <ReadOnlyBlock
                  label={t("webhooks.detail.eventType")}
                  value={formatWebhookEventType(delivery.event_type, t)}
                />
                <ReadOnlyBlock
                  label={t("webhooks.detail.targetUrl")}
                  value={delivery.target_url}
                  monospace
                />
              </div>
            </DetailCard>

            <DetailCard title={t("webhooks.detail.raw")}>
              <ScrollArea className="max-h-[56vh] rounded-md border bg-muted">
                <pre className="min-w-max px-3 py-2 text-xs leading-6">
                  {JSON.stringify(delivery, null, 2)}
                </pre>
              </ScrollArea>
            </DetailCard>
          </div>
        </div>
      ) : null}
    </div>
  );
}
