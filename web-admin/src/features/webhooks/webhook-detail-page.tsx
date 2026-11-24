import { useEffect, useMemo, useState } from "react";
import { useParams } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { RotateCcw } from "lucide-react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ConfigDrawer } from "@/components/config-drawer";
import { Header } from "@/components/layout/header";
import { Main } from "@/components/layout/main";
import { PageHeader } from "@/components/page-header";
import { ProfileDropdown } from "@/components/profile-dropdown";
import { Search } from "@/components/search";
import { ThemeSwitch } from "@/components/theme-switch";
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
      toast.success(t("common.feedback.retried"));
    } catch (err) {
      toast.error(
        err instanceof APIError ? err.message : t("webhooks.retryFailed"),
      );
    } finally {
      setRetrying(false);
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
        <div className="flex min-w-0 max-w-full flex-col gap-5">
          <PageHeader
            title={t("webhooks.detailTitle")}
            badge={
              delivery ? (
                <Badge variant={webhookStatusVariant(delivery.status)}>
                  {t(`webhooks.status.${delivery.status}`)}
                </Badge>
              ) : null
            }
            description={t("webhooks.detail.description")}
            back={{ to: "/webhooks", label: t("common.back") }}
            className="sm:items-center"
            actions={
              <Button
                className="w-full sm:w-auto"
                variant="outline"
                onClick={retry}
                disabled={
                  !delivery || delivery.status === "succeeded" || retrying
                }
              >
                <RotateCcw data-icon="inline-start" />
                {t("webhooks.retry")}
              </Button>
            }
          />

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
                  <div className="max-h-[56vh] overflow-auto rounded-md border bg-muted">
                    <pre className="min-w-max px-3 py-2 text-xs leading-6">
                      {JSON.stringify(delivery, null, 2)}
                    </pre>
                  </div>
                </DetailCard>
              </div>
            </div>
          ) : null}
        </div>
      </Main>
    </>
  );
}
