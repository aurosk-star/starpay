import { useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Save, Settings2 } from "lucide-react";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
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
  paymentNotifyPath: string;
  defaultCurrency: string;
  defaultLocale: string;
  requestIdEnabled: boolean;
  maintenanceMode: boolean;
  extra: string;
};

const emptyForm: FormState = {
  siteName: "",
  gatewayBaseUrl: "",
  paymentNotifyPath: "",
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
        payment_notify_path: form.paymentNotifyPath,
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

      <Card>
        <CardHeader>
          <CardTitle>{t("config.gateway")}</CardTitle>
          <CardDescription>{t("config.gatewayDescription")}</CardDescription>
        </CardHeader>
        <CardContent>
          {loading ? (
            <p className="text-sm text-muted-foreground">
              {t("config.loading")}
            </p>
          ) : (
            <form className="flex flex-col gap-5" onSubmit={handleSubmit}>
              <FieldGroup>
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
                  <FieldLabel htmlFor="payment_notify_path">
                    {t("config.fields.paymentNotifyPath")}
                  </FieldLabel>
                  <Input
                    id="payment_notify_path"
                    value={form.paymentNotifyPath}
                    onChange={(event) =>
                      setForm((current) => ({
                        ...current,
                        paymentNotifyPath: event.target.value,
                      }))
                    }
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
                <Field orientation="horizontal">
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
                <Field orientation="horizontal">
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
                <Field>
                  <FieldLabel htmlFor="extra">
                    {t("config.fields.extra")}
                  </FieldLabel>
                  <Textarea
                    id="extra"
                    value={form.extra}
                    rows={8}
                    onChange={(event) =>
                      setForm((current) => ({
                        ...current,
                        extra: event.target.value,
                      }))
                    }
                  />
                </Field>
              </FieldGroup>
              <div className="flex items-center justify-between gap-3">
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
              </div>
            </form>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function toForm(config: GatewayConfig): FormState {
  return {
    siteName: config.site_name,
    gatewayBaseUrl: config.gateway_base_url,
    paymentNotifyPath: config.payment_notify_path,
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
