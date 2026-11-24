import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { AlertCircle, CheckCircle2, Clock3, ExternalLink } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { useDocumentTitle } from "@/hooks/use-document-title";
import { formatMinorAmount } from "@/lib/money";

import { getCheckoutOrder } from "./api";
import { CheckoutShell } from "./checkout-page";
import type { CheckoutOrderResponse } from "./types";
import { useCheckoutLanguage } from "./use-checkout-language";

type CheckoutResultPageProps = {
  gatewayOrderNo: string;
};

const REDIRECT_SECONDS = 5;

export function CheckoutResultPage({
  gatewayOrderNo,
}: CheckoutResultPageProps) {
  useCheckoutLanguage();
  const { t } = useTranslation();
  useDocumentTitle(t("checkout.paidTitle"));
  const checkoutToken = useMemo(() => {
    if (typeof window === "undefined") return "";
    return (
      new URLSearchParams(window.location.search).get("token")?.trim() ?? ""
    );
  }, []);
  const [orderData, setOrderData] = useState<CheckoutOrderResponse | null>(
    null,
  );
  const [seconds, setSeconds] = useState(REDIRECT_SECONDS);
  const [loadFailed, setLoadFailed] = useState(false);

  useEffect(() => {
    if (!checkoutToken) {
      setLoadFailed(true);
      toast.error(t("checkout.invalidLink"));
      return;
    }
    let alive = true;
    getCheckoutOrder(gatewayOrderNo, checkoutToken)
      .then((result) => {
        if (alive) setOrderData(result);
      })
      .catch((err: Error) => {
        if (!alive) return;
        setLoadFailed(true);
        toast.error(err.message || t("checkout.result.loadFailed"));
      });
    return () => {
      alive = false;
    };
  }, [checkoutToken, gatewayOrderNo, t]);

  const merchantReturnURL = orderData?.order.return_url?.trim() ?? "";

  useEffect(() => {
    if (!merchantReturnURL || loadFailed) return;
    if (seconds <= 0) {
      window.location.assign(
        withResultParams(
          merchantReturnURL,
          gatewayOrderNo,
          orderData?.order.status ?? "",
        ),
      );
      return;
    }
    const timer = window.setTimeout(
      () => setSeconds((current) => current - 1),
      1000,
    );
    return () => window.clearTimeout(timer);
  }, [
    loadFailed,
    gatewayOrderNo,
    merchantReturnURL,
    orderData?.order.status,
    seconds,
  ]);

  const order = orderData?.order;
  const isPaid = order?.status === "paid";
  const isClosed =
    loadFailed || order?.status === "closed" || order?.status === "failed";

  return (
    <CheckoutShell>
      <Card className="mx-auto w-full max-w-xl">
        <CardHeader>
          <div className="mb-2">
            {isPaid ? (
              <CheckCircle2 />
            ) : isClosed ? (
              <AlertCircle />
            ) : (
              <Clock3 />
            )}
          </div>
          <CardTitle>
            {isPaid
              ? t("checkout.result.paidTitle")
              : isClosed
                ? t("checkout.result.failedTitle")
                : t("checkout.result.processingTitle")}
          </CardTitle>
          <CardDescription>
            {merchantReturnURL
              ? t("checkout.result.redirectHint", { seconds })
              : t("checkout.result.noReturnUrl")}
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          {order ? (
            <div className="rounded-lg border p-4 text-sm">
              <div className="flex items-center justify-between gap-4">
                <span className="text-muted-foreground">
                  {t("checkout.gatewayOrderNo")}
                </span>
                <span className="font-mono text-xs">
                  {order.gateway_order_no}
                </span>
              </div>
              <div className="mt-3 flex items-center justify-between gap-4">
                <span className="text-muted-foreground">
                  {t("checkout.amount")}
                </span>
                <span className="font-mono">
                  {formatMinorAmount(order.amount, order.currency)}
                </span>
              </div>
            </div>
          ) : null}
        </CardContent>
        {merchantReturnURL ? (
          <CardFooter>
            <Button
              className="w-full"
              onClick={() =>
                window.location.assign(
                  withResultParams(
                    merchantReturnURL,
                    gatewayOrderNo,
                    order?.status ?? "",
                  ),
                )
              }
            >
              {t("checkout.result.returnNow")}
              <ExternalLink />
            </Button>
          </CardFooter>
        ) : null}
      </Card>
    </CheckoutShell>
  );
}

function withResultParams(
  target: string,
  gatewayOrderNo: string,
  status: string,
) {
  try {
    const parsed = new URL(target);
    parsed.searchParams.set("gateway_order_no", gatewayOrderNo);
    if (status) parsed.searchParams.set("status", status);
    return parsed.toString();
  } catch {
    return target;
  }
}
