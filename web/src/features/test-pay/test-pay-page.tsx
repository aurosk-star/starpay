import { useEffect, useMemo, useState } from "react";
import type { FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { ArrowRight, RefreshCw } from "lucide-react";
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
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { listApps } from "@/features/apps/api";
import type { GatewayApp } from "@/features/apps/types";
import { useAuthStore } from "@/features/auth/store";
import { createTestCheckoutOrder } from "@/features/orders/api";

type Channel = "alipay" | "wechat" | "paypal";
type Currency = "CNY" | "USD" | "EUR" | "HKD" | "JPY" | "GBP";

type TestPayForm = {
  appId: string;
  merchantOrderNo: string;
  businessType: string;
  subject: string;
  description: string;
  amountMajor: string;
  currency: Currency;
  specifyMethod: boolean;
  channel: Channel;
  payMethod: string;
  returnUrl: string;
  metadataJson: string;
};

const currencies: Currency[] = ["CNY", "USD", "EUR", "HKD", "JPY", "GBP"];
const channels: Channel[] = ["alipay", "wechat", "paypal"];

function newMerchantOrderNo() {
  return `test_checkout_${Date.now()}`;
}

function defaultForm(t: (key: string) => string): TestPayForm {
  return {
    appId: "",
    merchantOrderNo: newMerchantOrderNo(),
    businessType: "checkout_test",
    subject: t("testPay.defaults.subject"),
    description: t("testPay.defaults.description"),
    amountMajor: "0.99",
    currency: "USD",
    specifyMethod: false,
    channel: "paypal",
    payMethod: "paypal",
    returnUrl: "",
    metadataJson: JSON.stringify(
      {
        source: "admin_test_pay_page",
      },
      null,
      2,
    ),
  };
}

export function TestPayPage() {
  const { t } = useTranslation();
  const accessToken = useAuthStore((state) => state.accessToken);
  const [form, setForm] = useState<TestPayForm>(() => defaultForm(t));
  const [apps, setApps] = useState<GatewayApp[]>([]);
  const [loadingApps, setLoadingApps] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!accessToken) return;
    let alive = true;
    setLoadingApps(true);
    listApps(accessToken)
      .then((result) => {
        if (!alive) return;
        const enabledApps = result.items.filter((app) => app.status === "enabled");
        setApps(enabledApps);
        setForm((current) => {
          if (current.appId || enabledApps.length === 0) return current;
          return { ...current, appId: enabledApps[0].app_id };
        });
      })
      .catch((err: Error) => {
        if (alive) {
          toast.error(err.message || t("testPay.errors.loadAppsFailed"));
        }
      })
      .finally(() => {
        if (alive) setLoadingApps(false);
      });
    return () => {
      alive = false;
    };
  }, [accessToken, t]);

  const minorAmountPreview = useMemo(() => {
    const amount = parseMajorAmount(form.amountMajor, form.currency);
    return Number.isFinite(amount) && amount > 0 ? amount : 0;
  }, [form.amountMajor, form.currency]);

  const selectedApp = useMemo(
    () => apps.find((app) => app.app_id === form.appId),
    [apps, form.appId],
  );
  const channelCurrencySupported = channelSupportsCurrency(
    form.channel,
    form.currency,
  );

  function update<K extends keyof TestPayForm>(key: K, value: TestPayForm[K]) {
    setForm((current) => ({ ...current, [key]: value }));
  }

  function handleChannelChange(channel: Channel) {
    setForm((current) => ({
      ...current,
      channel,
      payMethod: channel,
      currency: channel === "paypal" ? "USD" : "CNY",
    }));
  }

  function resetMerchantOrderNo() {
    update("merchantOrderNo", newMerchantOrderNo());
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!accessToken) {
      toast.error(t("testPay.errors.authRequired"));
      return;
    }
    setSubmitting(true);
    try {
      const amount = parseMajorAmount(form.amountMajor, form.currency);
      if (amount <= 0) {
        throw new Error(t("testPay.errors.invalidAmount"));
      }
      if (!form.appId.trim()) {
        throw new Error(t("testPay.errors.appRequired"));
      }
      if (!form.returnUrl.trim() && !selectedApp?.default_return_url) {
        throw new Error(t("testPay.errors.returnUrlRequired"));
      }
      if (form.specifyMethod && !channelCurrencySupported) {
        throw new Error(t("testPay.errors.unsupportedCurrency"));
      }
      const metadata = parseMetadata(
        form.metadataJson,
        t("testPay.errors.invalidMetadata"),
      );
      const result = await createTestCheckoutOrder(accessToken, {
        app_id: form.appId.trim(),
        merchant_order_no: form.merchantOrderNo.trim(),
        business_type: form.businessType.trim() || undefined,
        subject: form.subject.trim(),
        description: form.description.trim() || undefined,
        amount,
        currency: form.currency,
        channel: form.specifyMethod ? form.channel : undefined,
        pay_method: form.specifyMethod
          ? form.payMethod.trim() || form.channel
          : undefined,
        return_url: form.returnUrl.trim() || undefined,
        metadata,
      });
      toast.success(t("testPay.created"));
      if (result.payment?.pay_url) {
        window.open(result.payment.pay_url, "_blank", "noopener,noreferrer");
      } else {
        throw new Error(t("testPay.errors.missingPayUrl"));
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("testPay.errors.failed"));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-col gap-5">
      <div className="flex flex-col gap-1">
        <h1 className="text-2xl font-semibold tracking-tight">
          {t("testPay.title")}
        </h1>
        <p className="text-sm text-muted-foreground">
          {t("testPay.description")}
        </p>
      </div>

      <form onSubmit={handleSubmit}>
        <Card>
          <CardHeader>
            <CardTitle>{t("testPay.formTitle")}</CardTitle>
            <CardDescription>{t("testPay.formDescription")}</CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-6">
            <FieldGroup className="grid gap-5 md:grid-cols-2">
              <Field>
                <FieldLabel htmlFor="test_pay_app_id">
                  {t("testPay.fields.appId")}
                </FieldLabel>
                <Select
                  value={form.appId}
                  onValueChange={(value) => update("appId", value)}
                  disabled={loadingApps || apps.length === 0}
                >
                  <SelectTrigger id="test_pay_app_id" className="w-full">
                    <SelectValue placeholder={t("testPay.selectApp")} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      {apps.map((app) => (
                        <SelectItem key={app.id} value={app.app_id}>
                          {app.name} ({app.app_id})
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <FieldDescription>
                  {selectedApp?.default_return_url
                    ? t("testPay.appDefaultReturnUrl", {
                        url: selectedApp.default_return_url,
                      })
                    : t("testPay.noAppDefaultReturnUrl")}
                </FieldDescription>
              </Field>
              <Field>
                <FieldLabel htmlFor="test_pay_merchant_order_no">
                  {t("testPay.fields.merchantOrderNo")}
                </FieldLabel>
                <div className="flex gap-2">
                  <Input
                    id="test_pay_merchant_order_no"
                    value={form.merchantOrderNo}
                    onChange={(event) =>
                      update("merchantOrderNo", event.target.value)
                    }
                    required
                  />
                  <Button
                    type="button"
                    variant="outline"
                    size="icon"
                    aria-label={t("testPay.regenerateOrderNo")}
                    onClick={resetMerchantOrderNo}
                  >
                    <RefreshCw />
                  </Button>
                </div>
              </Field>
              <Field>
                <FieldLabel htmlFor="test_pay_business_type">
                  {t("testPay.fields.businessType")}
                </FieldLabel>
                <Input
                  id="test_pay_business_type"
                  value={form.businessType}
                  onChange={(event) =>
                    update("businessType", event.target.value)
                  }
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="test_pay_subject">
                  {t("testPay.fields.subject")}
                </FieldLabel>
                <Input
                  id="test_pay_subject"
                  value={form.subject}
                  onChange={(event) => update("subject", event.target.value)}
                  required
                />
              </Field>
            </FieldGroup>

            <Separator />

            <Field orientation="horizontal" className="justify-between rounded-lg border p-4">
              <div className="space-y-1">
                <FieldLabel htmlFor="test_pay_specify_method">
                  {t("testPay.fields.specifyMethod")}
                </FieldLabel>
                <FieldDescription>
                  {t("testPay.specifyMethodHint")}
                </FieldDescription>
              </div>
              <Switch
                id="test_pay_specify_method"
                checked={form.specifyMethod}
                onCheckedChange={(checked) => update("specifyMethod", checked)}
              />
            </Field>

            <FieldGroup className="grid gap-5 md:grid-cols-4">
              <Field>
                <FieldLabel htmlFor="test_pay_channel">
                  {t("testPay.fields.channel")}
                </FieldLabel>
                <Select
                  value={form.channel}
                  onValueChange={(value) =>
                    handleChannelChange(value as Channel)
                  }
                >
                  <SelectTrigger id="test_pay_channel" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      {channels.map((channel) => (
                        <SelectItem key={channel} value={channel}>
                          {t(`testPay.channels.${channel}`)}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
                {form.specifyMethod && !channelCurrencySupported ? (
                  <FieldError>
                    {t("testPay.errors.unsupportedCurrency")}
                  </FieldError>
                ) : null}
              </Field>
              <Field>
                <FieldLabel htmlFor="test_pay_pay_method">
                  {t("testPay.fields.payMethod")}
                </FieldLabel>
                <Input
                  id="test_pay_pay_method"
                  value={form.payMethod}
                  onChange={(event) => update("payMethod", event.target.value)}
                  disabled={!form.specifyMethod}
                  required={form.specifyMethod}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="test_pay_currency">
                  {t("testPay.fields.currency")}
                </FieldLabel>
                <Select
                  value={form.currency}
                  onValueChange={(value) =>
                    update("currency", value as Currency)
                  }
                >
                  <SelectTrigger id="test_pay_currency" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      {currencies.map((currency) => (
                        <SelectItem key={currency} value={currency}>
                          {currency}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
              <Field>
                <FieldLabel htmlFor="test_pay_amount">
                  {t("testPay.fields.amount")}
                </FieldLabel>
                <Input
                  id="test_pay_amount"
                  inputMode="decimal"
                  value={form.amountMajor}
                  onChange={(event) =>
                    update("amountMajor", event.target.value)
                  }
                  required
                />
                <FieldDescription>
                  {t("testPay.minorAmount", { amount: minorAmountPreview })}
                </FieldDescription>
              </Field>
            </FieldGroup>

            <Field>
              <FieldLabel htmlFor="test_pay_description">
                {t("testPay.fields.description")}
              </FieldLabel>
              <Textarea
                id="test_pay_description"
                value={form.description}
                onChange={(event) => update("description", event.target.value)}
              />
            </Field>

            <Field>
              <FieldLabel htmlFor="test_pay_return_url">
                {t("testPay.fields.returnUrl")}
              </FieldLabel>
              <Input
                id="test_pay_return_url"
                value={form.returnUrl}
                onChange={(event) => update("returnUrl", event.target.value)}
                placeholder="https://merchant.example.com/payment/result"
              />
              <FieldDescription>{t("testPay.returnUrlHint")}</FieldDescription>
            </Field>

            <Field>
              <FieldLabel htmlFor="test_pay_metadata">
                {t("testPay.fields.metadata")}
              </FieldLabel>
              <Textarea
                id="test_pay_metadata"
                className="font-mono"
                value={form.metadataJson}
                onChange={(event) => update("metadataJson", event.target.value)}
              />
              <FieldDescription>{t("testPay.metadataHint")}</FieldDescription>
              <FieldError />
            </Field>
          </CardContent>
          <CardFooter className="justify-end gap-2">
            <Button type="submit" disabled={submitting}>
              {submitting ? t("testPay.submitting") : t("testPay.submit")}
              <ArrowRight />
            </Button>
          </CardFooter>
        </Card>
      </form>
    </div>
  );
}

function parseMajorAmount(value: string, currency: Currency) {
  const normalized = value.trim();
  if (normalized === "") return 0;
  const numberValue = Number(normalized);
  if (!Number.isFinite(numberValue)) return 0;
  if (currency === "JPY") {
    return Math.round(numberValue);
  }
  return Math.round(numberValue * 100);
}

function parseMetadata(
  value: string,
  invalidMessage: string,
): Record<string, unknown> {
  const trimmed = value.trim();
  if (trimmed === "") {
    return {};
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(trimmed) as unknown;
  } catch {
    throw new Error(invalidMessage);
  }
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new Error(invalidMessage);
  }
  return parsed as Record<string, unknown>;
}

function channelSupportsCurrency(channel: Channel, currency: Currency) {
  if (channel === "alipay" || channel === "wechat") {
    return currency === "CNY";
  }
  if (channel === "paypal") {
    return ["USD", "EUR", "HKD", "JPY", "GBP"].includes(currency);
  }
  return false;
}
