import { useEffect, useMemo, useState } from "react";
import { useParams } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Pencil, XCircle } from "lucide-react";
import { toast } from "sonner";

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
import { ConfigDrawer } from "@/components/config-drawer";
import { Header } from "@/components/layout/header";
import { Main } from "@/components/layout/main";
import { PageHeader } from "@/components/page-header";
import { ProfileDropdown } from "@/components/profile-dropdown";
import { Search } from "@/components/search";
import { ThemeSwitch } from "@/components/theme-switch";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { DetailCard, DetailSkeleton, DetailTable } from "@/components/detail";
import { useAuthStore } from "@/features/auth/store";
import { listReconciliations } from "@/features/reconciliations/api";
import type { PaymentReconciliation } from "@/features/reconciliations/types";
import { listRefunds } from "@/features/refunds/api";
import type { Refund } from "@/features/refunds/types";
import { APIError } from "@/lib/api";
import { OrderEditDialog } from "./order-edit-dialog";
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
  const [refunds, setRefunds] = useState<Refund[]>([]);
  const [reconciliations, setReconciliations] = useState<
    PaymentReconciliation[]
  >([]);
  const [loading, setLoading] = useState(true);
  const [closeConfirmOpen, setCloseConfirmOpen] = useState(false);
  const [editOpen, setEditOpen] = useState(false);

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
            [
              t("orders.fields.providerOrderNo"),
              order.provider_order_no || "-",
            ],
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
            [t("orders.fields.failedAt"), formatDateTime(order.failed_at)],
            [t("orders.fields.failureReason"), order.failure_reason || "-"],
            [t("orders.fields.closedAt"), formatDateTime(order.closed_at)],
          ]
        : [],
    [order, t],
  );

  async function load() {
    if (!accessToken) return;
    setLoading(true);
    try {
      const result = await getOrder(accessToken, Number(orderId));
      setOrder(result.order);
      const [refundResult, reconciliationResult] = await Promise.all([
        listRefunds(accessToken, {
          gateway_order_no: result.order.gateway_order_no,
          page_size: 100,
        }),
        listReconciliations(accessToken, {
          gateway_order_no: result.order.gateway_order_no,
          page_size: 100,
        }),
      ]);
      setRefunds(refundResult.items);
      setReconciliations(reconciliationResult.items);
    } catch (err) {
      toast.error(
        err instanceof APIError ? err.message : t("orders.loadFailed"),
      );
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
      toast.error(
        err instanceof APIError ? err.message : t("orders.closeFailed"),
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
          title={t("orders.detailTitle")}
          badge={
            order ? (
              <Badge variant={orderStatusVariant(order.status)}>
                {t(`orders.status.${order.status}`)}
              </Badge>
            ) : null
          }
          description={order?.gateway_order_no ?? t("orders.detailDescription")}
          descriptionClassName="break-all"
          back={{ to: "/orders", label: t("common.back") }}
          className="sm:items-center"
          actions={
            <>
              <Button
                className="w-full sm:w-auto"
                variant="outline"
                disabled={!order}
                onClick={() => setEditOpen(true)}
              >
                <Pencil data-icon="inline-start" />
                {t("orders.edit.action")}
              </Button>
              <Button
                className="w-full sm:w-auto"
                variant="outline"
                disabled={!order || !canCloseOrder(order)}
                onClick={() => setCloseConfirmOpen(true)}
              >
                <XCircle data-icon="inline-start" />
                {t("orders.close")}
              </Button>
            </>
          }
        />

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
              <DetailCard title={t("orders.detail.refunds")}>
                <DetailTable
                  rows={
                    refunds.length
                      ? refunds.map((refund) => [
                          refund.refund_no,
                          `${formatMinorAmount(refund.amount, refund.currency)} · ${t(`refunds.status.${refund.status}`)}`,
                        ])
                      : [[t("orders.detail.none"), "-"]]
                  }
                />
              </DetailCard>
              <DetailCard title={t("orders.detail.reconciliation")}>
                <DetailTable
                  rows={
                    reconciliations.length
                      ? reconciliations.map((item) => [
                          t(`reconciliations.status.${item.status}`),
                          `${item.attempt_count}`,
                        ])
                      : [[t("orders.detail.none"), "-"]]
                  }
                />
              </DetailCard>
              <DetailCard title={t("orders.fields.metadata")}>
                <div className="max-h-[56vh] overflow-auto rounded-md border bg-muted">
                  <pre className="min-w-max px-3 py-2 text-xs leading-6">
                    {JSON.stringify(order.metadata ?? {}, null, 2)}
                  </pre>
                </div>
              </DetailCard>
              <DetailCard title={t("orders.detail.raw")}>
                <div className="max-h-[56vh] overflow-auto rounded-md border bg-muted">
                  <pre className="min-w-max px-3 py-2 text-xs leading-6">
                    {JSON.stringify(order, null, 2)}
                  </pre>
                </div>
              </DetailCard>
            </div>
          </div>
        ) : null}

        <AlertDialog open={closeConfirmOpen} onOpenChange={setCloseConfirmOpen}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>
                {t("orders.closeConfirmTitle")}
              </AlertDialogTitle>
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

        <OrderEditDialog
          open={editOpen}
          onOpenChange={setEditOpen}
          order={order}
          accessToken={accessToken ?? ""}
          onUpdated={(updated) => setOrder(updated)}
        />
      </Main>
    </>
  );
}
