import { useEffect, useState, type FormEvent } from "react";
import { Link, useNavigate, useParams } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
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
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { useAuthStore } from "@/features/auth/store";
import { APIError } from "@/lib/api";

import {
  createChannelAccount,
  getChannelAccount,
  updateChannelAccount,
} from "./api";
import {
  buildChangedConfigPayload,
  buildConfigPayload,
  emptyChannelConfig,
  normalizeConfig,
  type ChannelConfig,
} from "./config";
import type { ChannelEnv, PaymentChannel } from "./types";

type FormState = {
  channel: PaymentChannel;
  name: string;
  env: ChannelEnv;
  enabled: boolean;
  config: ChannelConfig;
};

const emptyForm: FormState = {
  channel: "wechat",
  name: "",
  env: "sandbox",
  enabled: true,
  config: emptyChannelConfig.wechat,
};

export function ChannelFormPage({ mode }: { mode: "create" | "edit" }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const accessToken = useAuthStore((state) => state.accessToken);
  const params = useParams({ strict: false });
  const channelId = "channelId" in params ? Number(params.channelId) : 0;
  const [form, setForm] = useState<FormState>(emptyForm);
  const [initialConfig, setInitialConfig] = useState<ChannelConfig>(
    emptyForm.config,
  );
  const [loading, setLoading] = useState(mode === "edit");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (mode !== "edit" || !accessToken || !channelId) return;
    setLoading(true);
    setError(null);
    void getChannelAccount(accessToken, channelId)
      .then((result) => {
        const config = normalizeConfig(
          result.channel_account.channel,
          result.channel_account.config,
        );
        setForm({
          channel: result.channel_account.channel,
          name: result.channel_account.name,
          env: result.channel_account.env,
          enabled: result.channel_account.enabled,
          config,
        });
        setInitialConfig(config);
      })
      .catch((err) => {
        setError(
          err instanceof APIError ? err.message : t("channels.loadFailed"),
        );
      })
      .finally(() => setLoading(false));
  }, [accessToken, channelId, mode, t]);

  function setConfigValue(key: keyof ChannelConfig, value: string) {
    setForm((current) => ({
      ...current,
      config: {
        ...current.config,
        [key]: value,
      },
    }));
  }

  function changeChannel(channel: PaymentChannel) {
    const config = emptyChannelConfig[channel];
    setForm((current) => ({
      ...current,
      channel,
      config,
    }));
    setInitialConfig(config);
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!accessToken) return;
    setSaving(true);
    setError(null);
    try {
      const payload = {
        channel: form.channel,
        name: form.name,
        env: form.env,
        enabled: form.enabled,
        config:
          mode === "edit"
            ? buildChangedConfigPayload(form.config, initialConfig)
            : buildConfigPayload(form.config),
      };
      if (mode === "edit") {
        await updateChannelAccount(accessToken, channelId, payload);
      } else {
        await createChannelAccount(accessToken, payload);
      }
      await navigate({ to: "/channels" });
    } catch (err) {
      setError(
        err instanceof APIError ? err.message : t("channels.saveFailed"),
      );
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="mx-auto flex w-full max-w-6xl flex-col gap-5">
      <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">
            {mode === "edit"
              ? t("channels.editTitle")
              : t("channels.createTitle")}
          </h1>
          <p className="max-w-2xl text-sm text-muted-foreground">
            {t("channels.formDescription")}
          </p>
        </div>
        <Button variant="outline" type="button" asChild>
          <Link to="/channels">{t("channels.cancel")}</Link>
        </Button>
      </div>

      {error ? (
        <Alert variant="destructive">
          <AlertTitle>{t("channels.saveFailed")}</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      ) : null}

      {loading ? (
        <Card>
          <CardContent className="py-10">
            <p className="text-sm text-muted-foreground">
              {t("channels.loading")}
            </p>
          </CardContent>
        </Card>
      ) : (
        <form className="flex flex-col gap-5" onSubmit={handleSubmit}>
          <div className="grid gap-5 lg:grid-cols-[minmax(0,0.95fr)_minmax(0,1.35fr)]">
            <Card>
              <CardHeader>
                <CardTitle>{t("channels.basicInfo")}</CardTitle>
                <CardDescription>{t("channels.basicInfoHint")}</CardDescription>
              </CardHeader>
              <CardContent>
                <FieldGroup>
                  <Field>
                    <FieldLabel>{t("channels.channel")}</FieldLabel>
                    <Select value={form.channel} onValueChange={changeChannel}>
                      <SelectTrigger className="w-full">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectGroup>
                          <SelectItem value="wechat">
                            {t("channels.wechat")}
                          </SelectItem>
                          <SelectItem value="alipay">
                            {t("channels.alipay")}
                          </SelectItem>
                          <SelectItem value="paypal">
                            {t("channels.paypal")}
                          </SelectItem>
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FieldDescription>
                      {t("channels.channelHint")}
                    </FieldDescription>
                  </Field>
                  <Field>
                    <FieldLabel htmlFor="channel_name">
                      {t("channels.name")}
                    </FieldLabel>
                    <Input
                      id="channel_name"
                      value={form.name}
                      required
                      placeholder={t("channels.placeholders.name")}
                      onChange={(event) =>
                        setForm((current) => ({
                          ...current,
                          name: event.target.value,
                        }))
                      }
                    />
                    <FieldDescription>{t("channels.nameHint")}</FieldDescription>
                  </Field>
                  <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-1 xl:grid-cols-2">
                    <Field>
                      <FieldLabel>{t("channels.env")}</FieldLabel>
                      <Select
                        value={form.env}
                        onValueChange={(value) =>
                          setForm((current) => ({
                            ...current,
                            env: value as ChannelEnv,
                          }))
                        }
                      >
                        <SelectTrigger className="w-full">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectGroup>
                            <SelectItem value="sandbox">
                              {t("channels.sandbox")}
                            </SelectItem>
                            <SelectItem value="prod">
                              {t("channels.prod")}
                            </SelectItem>
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                    </Field>
                    <Field>
                      <FieldLabel>{t("channels.status")}</FieldLabel>
                      <Select
                        value={form.enabled ? "enabled" : "disabled"}
                        onValueChange={(value) =>
                          setForm((current) => ({
                            ...current,
                            enabled: value === "enabled",
                          }))
                        }
                      >
                        <SelectTrigger className="w-full">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectGroup>
                            <SelectItem value="enabled">
                              {t("channels.enabled")}
                            </SelectItem>
                            <SelectItem value="disabled">
                              {t("channels.disabled")}
                            </SelectItem>
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                    </Field>
                  </div>
                </FieldGroup>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>{t("channels.credentials")}</CardTitle>
                <CardDescription>
                  {t(`channels.${form.channel}ConfigHint`)}
                </CardDescription>
              </CardHeader>
              <CardContent>
                <FieldSet>
                  <FieldLegend>{t("channels.credentialFields")}</FieldLegend>
                  <div className="grid gap-4 md:grid-cols-2">
                    <ChannelConfigFields
                      channel={form.channel}
                      config={form.config}
                      editing={mode === "edit"}
                      onChange={setConfigValue}
                    />
                  </div>
                </FieldSet>
              </CardContent>
            </Card>
          </div>

          <div className="sticky bottom-0 flex justify-end gap-2 border-t bg-background/95 py-3 backdrop-blur">
            <Button variant="outline" type="button" asChild>
              <Link to="/channels">{t("channels.cancel")}</Link>
            </Button>
            <Button type="submit" disabled={saving}>
              {t("channels.save")}
            </Button>
          </div>
        </form>
      )}
    </div>
  );
}

function ChannelConfigFields({
  channel,
  config,
  editing,
  onChange,
}: {
  channel: PaymentChannel;
  config: ChannelConfig;
  editing: boolean;
  onChange: (key: keyof ChannelConfig, value: string) => void;
}) {
  const { t } = useTranslation();

  if (channel === "wechat") {
    return (
      <>
        <ConfigInput name="app_id" value={config.app_id} onChange={onChange} />
        <ConfigInput name="mch_id" value={config.mch_id} onChange={onChange} />
        <ConfigInput
          name="api_v3_key"
          value={config.api_v3_key}
          secret
          editing={editing}
          onChange={onChange}
        />
        <ConfigInput
          name="serial_no"
          value={config.serial_no}
          onChange={onChange}
        />
        <ConfigTextarea
          name="private_key"
          value={config.private_key}
          secret
          editing={editing}
          onChange={onChange}
        />
        <ConfigTextarea
          name="cert"
          value={config.cert}
          secret
          editing={editing}
          onChange={onChange}
        />
        <ConfigTextarea
          name="wechat_pay_public_key"
          value={config.wechat_pay_public_key}
          secret
          editing={editing}
          onChange={onChange}
        />
        <ConfigInput name="mode" value={config.mode} onChange={onChange} />
      </>
    );
  }

  if (channel === "alipay") {
    return (
      <>
        <ConfigInput name="app_id" value={config.app_id} onChange={onChange} />
        <ConfigTextarea
          name="private_key"
          value={config.private_key}
          secret
          editing={editing}
          onChange={onChange}
        />
        <ConfigTextarea
          name="alipay_public_key"
          value={config.alipay_public_key}
          secret
          editing={editing}
          onChange={onChange}
        />
        <ConfigInput
          name="server_url"
          value={config.server_url}
          onChange={onChange}
        />
        <ConfigInput
          name="product_code"
          value={config.product_code}
          onChange={onChange}
        />
        <CapabilitySwitch
          name="enable_page_pay"
          checked={config.enable_page_pay !== "false"}
          onChange={onChange}
        />
        <CapabilitySwitch
          name="enable_wap_pay"
          checked={config.enable_wap_pay !== "false"}
          onChange={onChange}
        />
        <CapabilitySwitch
          name="enable_qr_pay"
          checked={config.enable_qr_pay !== "false"}
          onChange={onChange}
        />
      </>
    );
  }

  return (
    <>
      <ConfigInput
        name="client_id"
        value={config.client_id}
        onChange={onChange}
      />
      <ConfigInput
        name="client_secret"
        value={config.client_secret}
        secret
        editing={editing}
        onChange={onChange}
      />
      <ConfigInput
        name="webhook_id"
        value={config.webhook_id}
        secret
        editing={editing}
        onChange={onChange}
      />
      <ConfigInput
        name="brand_name"
        value={config.brand_name}
        onChange={onChange}
      />
      <ConfigInput name="intent" value={config.intent} onChange={onChange} />
      {editing ? (
        <p className="text-sm text-muted-foreground">
          {t("channels.secretEditHint")}
        </p>
      ) : null}
    </>
  );
}

function ConfigInput({
  name,
  value,
  secret,
  editing,
  onChange,
}: {
  name: keyof ChannelConfig;
  value?: string;
  secret?: boolean;
  editing?: boolean;
  onChange: (key: keyof ChannelConfig, value: string) => void;
}) {
  const { t } = useTranslation();

  return (
    <Field>
      <FieldLabel htmlFor={`config_${name}`}>
        {t(`channels.fields.${name}`)}
      </FieldLabel>
      <Input
        id={`config_${name}`}
        type={secret ? "password" : "text"}
        value={value === "********" ? "" : value || ""}
        placeholder={
          secret && editing
            ? t("channels.keepSecretPlaceholder")
            : t(`channels.placeholders.${name}`)
        }
        autoComplete="off"
        spellCheck={false}
        className="font-mono"
        onChange={(event) => onChange(name, event.target.value)}
      />
      <FieldDescription>
        {secret && editing
          ? t("channels.secretEditHint")
          : t(`channels.hints.${name}`)}
      </FieldDescription>
    </Field>
  );
}

function ConfigTextarea({
  name,
  value,
  secret,
  editing,
  onChange,
}: {
  name: keyof ChannelConfig;
  value?: string;
  secret?: boolean;
  editing?: boolean;
  onChange: (key: keyof ChannelConfig, value: string) => void;
}) {
  const { t } = useTranslation();

  return (
    <Field className="md:col-span-2">
      <FieldLabel htmlFor={`config_${name}`}>
        {t(`channels.fields.${name}`)}
      </FieldLabel>
      <Textarea
        id={`config_${name}`}
        className="min-h-36 font-mono text-xs leading-relaxed"
        value={value === "********" ? "" : value || ""}
        placeholder={
          secret && editing
            ? t("channels.keepSecretPlaceholder")
            : t(`channels.placeholders.${name}`)
        }
        spellCheck={false}
        onChange={(event) => onChange(name, event.target.value)}
      />
      <FieldDescription>
        {secret && editing
          ? t("channels.secretEditHint")
          : t(`channels.hints.${name}`)}
      </FieldDescription>
    </Field>
  );
}

function CapabilitySwitch({
  name,
  checked,
  onChange,
}: {
  name: keyof ChannelConfig;
  checked: boolean;
  onChange: (key: keyof ChannelConfig, value: string) => void;
}) {
  const { t } = useTranslation();

  return (
    <Field orientation="horizontal" className="rounded-md border p-3">
      <div className="space-y-1">
        <FieldLabel htmlFor={`config_${name}`}>
          {t(`channels.fields.${name}`)}
        </FieldLabel>
        <FieldDescription>{t(`channels.hints.${name}`)}</FieldDescription>
      </div>
      <Switch
        id={`config_${name}`}
        checked={checked}
        onCheckedChange={(nextChecked) =>
          onChange(name, nextChecked ? "true" : "false")
        }
      />
    </Field>
  );
}
