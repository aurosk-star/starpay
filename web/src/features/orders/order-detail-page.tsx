import { useEffect, useMemo, useState } from "react";
import { Link, useParams } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { ArrowLeft, XCircle } from "lucide-react";

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
import { ScrollArea } from "@/components/ui/scroll-area";
import { DetailCard, DetailSkeleton, DetailTable } from "@/components/detail";
import { useAuthStore } from "@/features/auth/store";
import { APIError } from "@/lib/api";
import { formatDateTime } from "@/lib/date";
import { formatMinorAmount } from "@/lib/money";

import { closeOrder, getOrder } from "./api";
import type { PaymentOrder } from "./types";
import { canCloseOrder, orderStatusVariant } from "./utils";

export function OrderDetailPage() {
  const { t } = useTranslation();
  const { orderId } = useParams({ from: "/orders/$orderId" });
  const accessToken = useAuthStore((state) => state.accessToken);
  const [order, setOrder] = useState<PaymentOrder | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [closeConfirmOpen, setCloseConfirmOpen] = useState(false);

  const baseRows = useMemo<Array<[string, string]>>(
    () =>
      order
        ? [
            [t("orders.fields.appId"), order.app_id],
            [t("orders.fields.merchantOrderNo"), order.merchant_order_no],
            [t("orders.fields.businessType"), order.business_type || "-"],
            [t("orders.fields.subject"), order.subject],
            [t("orders.fields.description"), order.description || "-"],
          ]
        : [],
    [order, t],
  );

  const paymentRows = useMemo<Array<[string, string]>>(
    () =>
      order
        ? [
            [
              t("orders.fields.amount"),
              formatMinorAmount(order.amount, order.currency),
            ],
            [
              t("orders.fields.settlementAmount"),
              order.settlement_amount
                ? formatMinorAmount(
                    order.settlement_amount,
                    order.settlement_currency || order.currency,
                  )
                : "-",
            ],
            [t("orders.fields.channel"), order.channel || "-"],
            [t("orders.fields.payMethod"), order.pay_method || "-"],
            [t("orders.fields.channelTradeNo"), order.channel_trade_no || "-"],
            [t("orders.fields.returnUrl"), order.return_url || "-"],
          ]
        : [],
    [order, t],
  );

  const timelineRows = useMemo<Array<[string, string]>>(
    () =>
      order
        ? [
            [t("orders.fields.createdAt"), formatDateTime(order.created_at)],
            [t("orders.fields.updatedAt"), formatDateTime(order.updated_at)],
            [t("orders.fields.expiresAt"), formatDateTime(order.expires_at)],
            [t("orders.fields.paidAt"), formatDateTime(order.paid_at)],
            [t("orders.fields.closedAt"), formatDateTime(order.closed_at)],
          ]
        : [],
    [order, t],
  );

  async function load() {
    if (!accessToken) return;
    setLoading(true);
    setError(null);
    try {
      const result = await getOrder(accessToken, Number(orderId));
      setOrder(result.order);
    } catch (err) {
      setError(err instanceof APIError ? err.message : t("orders.loadFailed"));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, [accessToken, orderId]);

  async function confirmClose() {
    if (!accessToken || !order) return;
    try {
      const result = await closeOrder(accessToken, order.id);
      setOrder(result.order);
      setCloseConfirmOpen(false);
    } catch (err) {
      setError(err instanceof APIError ? err.message : t("orders.closeFailed"));
    }
  }

  return (
    <div className="flex min-w-0 max-w-full flex-col gap-5">
      <div className="flex min-w-0 flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div className="flex min-w-0 items-start gap-3">
          <Button variant="outline" size="icon" asChild>
            <Link to="/orders" aria-label={t("common.back")}>
              <ArrowLeft />
            </Link>
          </Button>
          <div className="min-w-0 flex flex-col gap-1">
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-2xl font-semibold tracking-tight">
                {t("orders.detailTitle")}
              </h1>
              {order ? (
                <Badge variant={orderStatusVariant(order.status)}>
                  {t(`orders.status.${order.status}`)}
                </Badge>
              ) : null}
            </div>
            <p className="break-all text-sm text-muted-foreground">
              {order?.gateway_order_no ?? t("orders.detailDescription")}
            </p>
          </div>
        </div>
        <Button
          className="w-full md:w-auto"
          variant="outline"
          disabled={!order || !canCloseOrder(order)}
          onClick={() => setCloseConfirmOpen(true)}
        >
          <XCircle />
          {t("orders.close")}
        </Button>
      </div>

      {error ? (
        <Alert variant="destructive">
          <AlertTitle>{t("orders.loadFailed")}</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      ) : null}

      {loading ? (
        <DetailSkeleton />
      ) : order ? (
        <div className="grid min-w-0 max-w-full gap-4 xl:grid-cols-[minmax(0,1fr)_420px]">
          <div className="flex min-w-0 flex-col gap-4">
            <DetailCard
              title={t("orders.detail.basic")}
              description={order.gateway_order_no}
            >
              <DetailTable rows={baseRows} />
            </DetailCard>
            <DetailCard title={t("orders.detail.payment")}>
              <DetailTable rows={paymentRows} />
            </DetailCard>
            <DetailCard title={t("orders.detail.timeline")}>
              <DetailTable rows={timelineRows} />
            </DetailCard>
          </div>

          <div className="flex min-w-0 flex-col gap-4">
            <DetailCard title={t("orders.fields.metadata")}>
              <ScrollArea className="max-h-[56vh] rounded-md border bg-muted">
                <pre className="min-w-max px-3 py-2 text-xs leading-6">
                  {JSON.stringify(order.metadata ?? {}, null, 2)}
                </pre>
              </ScrollArea>
            </DetailCard>
            <DetailCard title={t("orders.detail.raw")}>
              <ScrollArea className="max-h-[56vh] rounded-md border bg-muted">
                <pre className="min-w-max px-3 py-2 text-xs leading-6">
                  {JSON.stringify(order, null, 2)}
                </pre>
              </ScrollArea>
            </DetailCard>
          </div>
        </div>
      ) : null}

      <AlertDialog open={closeConfirmOpen} onOpenChange={setCloseConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("orders.closeConfirmTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("orders.closeConfirmDescription", {
                orderNo: order?.gateway_order_no ?? "",
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("orders.cancel")}</AlertDialogCancel>
            <AlertDialogAction onClick={confirmClose}>
              {t("orders.close")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
