import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { AlertCircle, ArrowUpRight, CheckCircle2, CreditCard } from "lucide-react";
import { QRCodeSVG } from "qrcode.react";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { formatMinorAmount } from "@/lib/money";

import {
  getCheckoutOrder,
  listCheckoutMethods,
  startCheckoutPayment,
} from "./api";
import type {
  CheckoutOrderResponse,
  CheckoutPaymentMethod,
  CheckoutPaymentResult,
} from "./types";

type CheckoutPageProps = {
  gatewayOrderNo: string;
};

export function CheckoutPage({ gatewayOrderNo }: CheckoutPageProps) {
  const { t } = useTranslation();
  const [orderData, setOrderData] = useState<CheckoutOrderResponse | null>(null);
  const [methods, setMethods] = useState<CheckoutPaymentMethod[]>([]);
  const [methodsLocked, setMethodsLocked] = useState(false);
  const [lockedMethod, setLockedMethod] = useState<CheckoutPaymentMethod | null>(null);
  const [selected, setSelected] = useState<string>("");
  const [payment, setPayment] = useState<CheckoutPaymentResult | null>(null);
  const [loading, setLoading] = useState(true);
  const [paying, setPaying] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    setLoading(true);
    setError(null);
    Promise.all([
      getCheckoutOrder(gatewayOrderNo),
      listCheckoutMethods(gatewayOrderNo),
    ])
      .then(([orderResult, methodResult]) => {
        if (!alive) return;
        setOrderData(orderResult);
        setMethods(methodResult.methods);
        setMethodsLocked(methodResult.locked);
        setLockedMethod(methodResult.selected_method ?? null);
        setSelected(
          methodResult.selected_method?.pay_method ??
            methodResult.methods.find((method) => method.enabled)?.pay_method ??
            "",
        );
      })
      .catch((err: Error) => {
        if (!alive) return;
        setError(err.message || t("checkout.loadFailed"));
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, [gatewayOrderNo, t]);

  const selectedMethod = useMemo(
    () =>
      methodsLocked
        ? lockedMethod
        : methods.find((method) => method.pay_method === selected),
    [lockedMethod, methods, methodsLocked, selected],
  );

  async function handlePay() {
    if (!selectedMethod) return;
    setPaying(true);
    setError(null);
    try {
      const result = await startCheckoutPayment(gatewayOrderNo, {
        pay_method: methodsLocked ? undefined : selectedMethod.pay_method,
        channel: methodsLocked ? undefined : selectedMethod.channel,
      });
      setPayment(result.payment);
      if (result.payment.form_html) {
        submitPaymentForm(result.payment.form_html);
      } else if (result.payment.pay_url) {
        window.location.assign(result.payment.pay_url);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t("checkout.payFailed"));
    } finally {
      setPaying(false);
    }
  }

  if (loading) {
    return <CheckoutSkeleton />;
  }

  if (error && !orderData) {
    return (
      <CheckoutShell>
        <Alert variant="destructive">
          <AlertCircle />
          <AlertTitle>{t("checkout.loadFailed")}</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      </CheckoutShell>
    );
  }

  if (!orderData) {
    return null;
  }

  const order = orderData.order;

  return (
    <CheckoutShell>
      <div className="flex flex-col gap-5 lg:grid lg:grid-cols-[1fr_360px]">
        <Card>
          <CardHeader>
            <CardDescription>{t("checkout.orderSummary")}</CardDescription>
            <CardTitle className="text-2xl">{order.subject}</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-5">
            <div>
              <p className="text-sm text-muted-foreground">{t("checkout.amount")}</p>
              <p className="mt-2 font-mono text-4xl font-semibold">
                {formatMinorAmount(order.amount, order.currency)}
              </p>
            </div>
            <Separator />
            <div className="grid gap-3 text-sm md:grid-cols-2">
              <Info label={t("checkout.gatewayOrderNo")} value={order.gateway_order_no} />
              <Info label={t("checkout.merchantOrderNo")} value={order.merchant_order_no} />
              <Info label={t("checkout.currency")} value={order.currency} />
              <Info label={t("checkout.status")} value={t(`checkout.statuses.${order.status}`)} />
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-lg">
              <CreditCard />
              {t("checkout.selectMethod")}
            </CardTitle>
            <CardDescription>{t("checkout.selectMethodHint")}</CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-3">
            {methodsLocked && selectedMethod ? (
              <div className="flex items-center justify-between rounded-lg border p-3">
                <div className="space-y-1">
                  <p className="text-sm font-medium">{selectedMethod.label}</p>
                  <p className="text-xs text-muted-foreground">
                    {t("checkout.lockedMethodHint")}
                  </p>
                </div>
                <Badge variant="secondary">{selectedMethod.channel}</Badge>
              </div>
            ) : methods.length > 0 ? (
              methods.map((method) => (
                <Button
                  key={`${method.channel}:${method.pay_method}`}
                  type="button"
                  variant={selected === method.pay_method ? "default" : "outline"}
                  className="justify-between"
                  disabled={!method.enabled || paying}
                  onClick={() => setSelected(method.pay_method)}
                >
                  <span>{method.label}</span>
                  <Badge variant="secondary">{method.channel}</Badge>
                </Button>
              ))
            ) : (
              <Alert>
                <AlertCircle />
                <AlertTitle>{t("checkout.noMethodsTitle")}</AlertTitle>
                <AlertDescription>{t("checkout.noMethodsDescription")}</AlertDescription>
              </Alert>
            )}
            {methodsLocked && lockedMethod && !lockedMethod.enabled ? (
              <Alert variant="destructive">
                <AlertCircle />
                <AlertTitle>{t("checkout.lockedMethodUnavailable")}</AlertTitle>
                <AlertDescription>
                  {t("checkout.lockedMethodUnavailableHint")}
                </AlertDescription>
              </Alert>
            ) : null}
            {error ? (
              <Alert variant="destructive">
                <AlertCircle />
                <AlertTitle>{t("checkout.payFailed")}</AlertTitle>
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            ) : null}
            {payment ? (
              <Alert>
                <CheckCircle2 />
                <AlertTitle>{t("checkout.paymentStarted")}</AlertTitle>
                <AlertDescription className="flex flex-col gap-4">
                  <span>
                    {payment.qr_code
                      ? t("checkout.paymentQrHint")
                      : t("checkout.paymentRedirecting")}
                  </span>
                  {payment.qr_code ? <QrPreview value={payment.qr_code} /> : null}
                </AlertDescription>
              </Alert>
            ) : null}
          </CardContent>
          <CardFooter>
            <Button
              className="w-full"
              disabled={!selectedMethod || paying}
              onClick={handlePay}
            >
              {paying ? t("checkout.paying") : t("checkout.payNow")}
              <ArrowUpRight />
            </Button>
          </CardFooter>
        </Card>
      </div>
    </CheckoutShell>
  );
}

function QrPreview({ value }: { value: string }) {
  return (
    <div className="flex justify-center rounded-lg border bg-background p-4">
      <QRCodeSVG
        aria-label="payment qr code"
        className="size-48 rounded-md bg-white p-3"
        value={value}
      />
    </div>
  );
}

function CheckoutShell({ children }: { children: React.ReactNode }) {
  const { t } = useTranslation();
  return (
    <main className="min-h-[100dvh] bg-background px-4 py-6 text-foreground md:px-6">
      <div className="mx-auto flex w-full max-w-5xl flex-col gap-6">
        <header className="flex items-center justify-between gap-4">
          <div>
            <p className="text-sm text-muted-foreground">{t("checkout.gateway")}</p>
            <h1 className="text-xl font-semibold">{t("checkout.title")}</h1>
          </div>
          <Badge variant="secondary">{t("checkout.secure")}</Badge>
        </header>
        {children}
      </div>
    </main>
  );
}

function CheckoutSkeleton() {
  return (
    <CheckoutShell>
      <div className="grid gap-5 lg:grid-cols-[1fr_360px]">
        <Card>
          <CardHeader>
            <Skeleton className="h-4 w-24" />
            <Skeleton className="h-8 w-48" />
          </CardHeader>
          <CardContent className="flex flex-col gap-5">
            <Skeleton className="h-12 w-56" />
            <Skeleton className="h-20 w-full" />
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <Skeleton className="h-6 w-32" />
            <Skeleton className="h-4 w-48" />
          </CardHeader>
          <CardContent className="flex flex-col gap-3">
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
          </CardContent>
        </Card>
      </div>
    </CheckoutShell>
  );
}

function Info({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0">
      <p className="text-muted-foreground">{label}</p>
      <p className={cn("mt-1 truncate font-medium", value.startsWith("pay_") ? "font-mono text-xs" : "")}>
        {value}
      </p>
    </div>
  );
}

function submitPaymentForm(formHTML: string) {
  const container = document.createElement("div");
  container.innerHTML = formHTML;
  container.style.position = "fixed";
  container.style.inset = "0";
  container.style.zIndex = "9999";
  container.style.background = "white";
  document.body.appendChild(container);
  const form = container.querySelector("form");
  if (form) {
    form.submit();
  }
}
