import { useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Copy, Save, Settings2 } from "lucide-react";
import { toast } from "sonner";

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
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { useAuthStore } from "@/features/auth/store";
import { APIError } from "@/lib/api";

import { getGatewayConfig, updateGatewayConfig } from "./api";
import { setCachedSiteName } from "./site-config";
import type { GatewayConfig, UpdateGatewayConfigPayload } from "./types";

type FormState = {
  siteName: string;
  gatewayBaseUrl: string;
  defaultCurrency: string;
  defaultLocale: string;
  requestIdEnabled: boolean;
  maintenanceMode: boolean;
  extra: string;
};

const emptyForm: FormState = {
  siteName: "",
  gatewayBaseUrl: "",
  defaultCurrency: "",
  defaultLocale: "",
  requestIdEnabled: true,
  maintenanceMode: false,
  extra: "{}",
};

export function GatewayConfigPage() {
  const { t } = useTranslation();
  const accessToken = useAuthStore((state) => state.accessToken);
  const [form, setForm] = useState<FormState>(emptyForm);
  const [config, setConfig] = useState<GatewayConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    if (!accessToken) return;
    setLoading(true);
    setError(null);
    void getGatewayConfig(accessToken)
      .then((result) => {
        setConfig(result.gateway_config);
        setForm(toForm(result.gateway_config));
      })
      .catch((err) =>
        setError(err instanceof APIError ? err.message : t("config.loadFailed")),
      )
      .finally(() => setLoading(false));
  }, [accessToken, t]);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!accessToken) return;
    setSaving(true);
    setSaved(false);
    setError(null);
    try {
      const extra = parseExtra(form.extra);
      const payload: UpdateGatewayConfigPayload = {
        site_name: form.siteName,
        gateway_base_url: form.gatewayBaseUrl,
        default_currency: form.defaultCurrency,
        default_locale: form.defaultLocale,
        request_id_enabled: form.requestIdEnabled,
        maintenance_mode: form.maintenanceMode,
        extra,
      };
      const result = await updateGatewayConfig(accessToken, payload);
      setConfig(result.gateway_config);
      setForm(toForm(result.gateway_config));
      setCachedSiteName(result.gateway_config.site_name);
      setSaved(true);
    } catch (err) {
      setError(err instanceof APIError ? err.message : t("config.saveFailed"));
    } finally {
      setSaving(false);
    }
  }

  const notifyURL =
    config && form.gatewayBaseUrl
      ? `${form.gatewayBaseUrl.replace(/\/+$/, "")}${config.payment_notify_path}`
      : "";

  async function copyNotifyURL() {
    if (!notifyURL) return;
    await navigator.clipboard.writeText(notifyURL);
    toast.success(t("config.notifyCopied"));
  }

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col gap-2">
        <div className="flex items-center gap-2">
          <Settings2 />
          <h1 className="text-2xl font-semibold">{t("config.title")}</h1>
        </div>
        <p className="text-sm text-muted-foreground">
          {t("config.description")}
        </p>
      </div>

      {error ? (
        <Alert variant="destructive">
          <AlertTitle>{t("config.errorTitle")}</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      ) : null}

      {saved ? (
        <Alert>
          <AlertTitle>{t("config.savedTitle")}</AlertTitle>
          <AlertDescription>{t("config.savedDescription")}</AlertDescription>
        </Alert>
      ) : null}

      <div className="grid items-start gap-5 xl:grid-cols-[minmax(0,1fr)_360px]">
        <Card>
          <CardHeader>
            <CardTitle>{t("config.gateway")}</CardTitle>
            <CardDescription>{t("config.gatewayDescription")}</CardDescription>
          </CardHeader>
          {loading ? (
            <GatewayConfigSkeleton />
          ) : (
            <form onSubmit={handleSubmit}>
              <CardContent className="pb-5">
                <FieldGroup>
                  <div className="grid gap-5 md:grid-cols-2">
                    <Field>
                      <FieldLabel htmlFor="site_name">
                        {t("config.fields.siteName")}
                      </FieldLabel>
                      <Input
                        id="site_name"
                        value={form.siteName}
                        onChange={(event) =>
                          setForm((current) => ({
                            ...current,
                            siteName: event.target.value,
                          }))
                        }
                        required
                      />
                    </Field>
                    <Field>
                      <FieldLabel htmlFor="gateway_base_url">
                        {t("config.fields.gatewayBaseUrl")}
                      </FieldLabel>
                      <Input
                        id="gateway_base_url"
                        value={form.gatewayBaseUrl}
                        onChange={(event) =>
                          setForm((current) => ({
                            ...current,
                            gatewayBaseUrl: event.target.value,
                          }))
                        }
                        required
                      />
                    </Field>
                    <Field>
                      <FieldLabel htmlFor="default_currency">
                        {t("config.fields.defaultCurrency")}
                      </FieldLabel>
                      <Input
                        id="default_currency"
                        value={form.defaultCurrency}
                        onChange={(event) =>
                          setForm((current) => ({
                            ...current,
                            defaultCurrency: event.target.value,
                          }))
                        }
                      />
                    </Field>
                    <Field>
                      <FieldLabel htmlFor="default_locale">
                        {t("config.fields.defaultLocale")}
                      </FieldLabel>
                      <Input
                        id="default_locale"
                        value={form.defaultLocale}
                        onChange={(event) =>
                          setForm((current) => ({
                            ...current,
                            defaultLocale: event.target.value,
                          }))
                        }
                      />
                    </Field>
                  </div>
                  <div className="grid gap-3 md:grid-cols-2">
                    <Field
                      orientation="horizontal"
                      className="rounded-lg border bg-muted/20 p-3"
                    >
                      <FieldLabel htmlFor="request_id_enabled">
                        {t("config.fields.requestIdEnabled")}
                      </FieldLabel>
                      <Switch
                        id="request_id_enabled"
                        checked={form.requestIdEnabled}
                        onCheckedChange={(checked) =>
                          setForm((current) => ({
                            ...current,
                            requestIdEnabled: checked,
                          }))
                        }
                      />
                    </Field>
                    <Field
                      orientation="horizontal"
                      className="rounded-lg border bg-muted/20 p-3"
                    >
                      <FieldLabel htmlFor="maintenance_mode">
                        {t("config.fields.maintenanceMode")}
                      </FieldLabel>
                      <Switch
                        id="maintenance_mode"
                        checked={form.maintenanceMode}
                        onCheckedChange={(checked) =>
                          setForm((current) => ({
                            ...current,
                            maintenanceMode: checked,
                          }))
                        }
                      />
                    </Field>
                  </div>
                  <Field>
                    <FieldLabel htmlFor="extra">
                      {t("config.fields.extra")}
                    </FieldLabel>
                    <Textarea
                      id="extra"
                      value={form.extra}
                      rows={8}
                      className="font-mono text-xs"
                      onChange={(event) =>
                        setForm((current) => ({
                          ...current,
                          extra: event.target.value,
                        }))
                      }
                    />
                  </Field>
                </FieldGroup>
              </CardContent>
              <CardFooter className="justify-between gap-3">
                <p className="text-xs text-muted-foreground">
                  {config
                    ? t("config.updatedAt", {
                        time: new Date(config.updated_at).toLocaleString(),
                      })
                    : t("config.notSaved")}
                </p>
                <Button type="submit" disabled={saving}>
                  <Save data-icon="inline-start" />
                  {saving ? t("config.saving") : t("config.save")}
                </Button>
              </CardFooter>
            </form>
          )}
        </Card>

        <Card className="xl:sticky xl:top-5">
          <CardHeader>
            <CardTitle>{t("config.runtime")}</CardTitle>
            <CardDescription>{t("config.runtimeDescription")}</CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-4">
            <div className="flex flex-col gap-2 rounded-lg border bg-muted/20 p-3">
              <div className="flex items-center justify-between gap-3">
                <span className="text-sm font-medium">
                  {t("config.fields.paymentNotifyPath")}
                </span>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  disabled={!notifyURL}
                  onClick={copyNotifyURL}
                >
                  <Copy data-icon="inline-start" />
                  {t("config.copy")}
                </Button>
              </div>
              {loading ? (
                <Skeleton className="h-8 w-full" />
              ) : (
                <code className="break-all rounded-md bg-background px-2 py-2 font-mono text-xs text-foreground ring-1 ring-border">
                  {notifyURL || "-"}
                </code>
              )}
            </div>
            <div className="grid gap-3">
              <RuntimeRow
                label={t("config.fields.requestIdEnabled")}
                value={form.requestIdEnabled}
                enabledText={t("config.enabled")}
                disabledText={t("config.disabled")}
              />
              <RuntimeRow
                label={t("config.fields.maintenanceMode")}
                value={form.maintenanceMode}
                enabledText={t("config.enabled")}
                disabledText={t("config.disabled")}
              />
              <div className="flex items-center justify-between gap-3 rounded-lg border px-3 py-2">
                <span className="text-sm text-muted-foreground">
                  {t("config.fields.updatedAt")}
                </span>
                <span className="text-right text-xs">
                  {config
                    ? new Date(config.updated_at).toLocaleString()
                    : t("config.notSaved")}
                </span>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

function GatewayConfigSkeleton() {
  return (
    <CardContent>
      <div className="flex flex-col gap-5">
        <div className="grid gap-5 md:grid-cols-2">
          <Skeleton className="h-16" />
          <Skeleton className="h-16" />
          <Skeleton className="h-16" />
          <Skeleton className="h-16" />
        </div>
        <div className="grid gap-3 md:grid-cols-2">
          <Skeleton className="h-12" />
          <Skeleton className="h-12" />
        </div>
        <Skeleton className="h-48" />
      </div>
    </CardContent>
  );
}

function RuntimeRow({
  label,
  value,
  enabledText,
  disabledText,
}: {
  label: string;
  value: boolean;
  enabledText: string;
  disabledText: string;
}) {
  return (
    <div className="flex items-center justify-between gap-3 rounded-lg border px-3 py-2">
      <span className="text-sm text-muted-foreground">{label}</span>
      <Badge variant={value ? "secondary" : "outline"}>
        {value ? enabledText : disabledText}
      </Badge>
    </div>
  );
}

function toForm(config: GatewayConfig): FormState {
  return {
    siteName: config.site_name,
    gatewayBaseUrl: config.gateway_base_url,
    defaultCurrency: config.default_currency,
    defaultLocale: config.default_locale,
    requestIdEnabled: config.request_id_enabled,
    maintenanceMode: config.maintenance_mode,
    extra: JSON.stringify(config.extra ?? {}, null, 2),
  };
}

function parseExtra(value: string): Record<string, unknown> {
  const trimmed = value.trim();
  if (!trimmed) {
    return {};
  }
  const parsed = JSON.parse(trimmed) as unknown;
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new APIError(400, "invalid_extra", "扩展配置必须是 JSON 对象");
  }
  return parsed as Record<string, unknown>;
}
