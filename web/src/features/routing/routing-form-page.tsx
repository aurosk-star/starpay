import { useEffect, useMemo, useState, type FormEvent } from "react";
import { Link, useNavigate, useParams } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { ArrowLeft, Plus, Trash2 } from "lucide-react";
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
import {
  Field,
  FieldDescription,
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
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { listChannelAccounts } from "@/features/channels/api";
import type { ChannelAccount } from "@/features/channels/types";
import { useAuthStore } from "@/features/auth/store";
import { APIError } from "@/lib/api";

import {
  createRoutingRule,
  getRoutingRule,
  updateRoutingRule,
} from "./api";
import type {
  ManageRoutingRulePayload,
  RoutingAppScope,
  RoutingPaymentMethod,
  RoutingTerminal,
} from "./types";

type TargetForm = {
  channel_account_id: string;
  enabled: boolean;
  priority: string;
  weight: string;
};

type FormState = {
  name: string;
  enabled: boolean;
  priority: string;
  app_scope: RoutingAppScope;
  app_ids: string;
  payment_method: RoutingPaymentMethod;
  pay_modes: string;
  currency: string;
  min_amount: string;
  max_amount: string;
  terminal: RoutingTerminal;
  targets: TargetForm[];
};

const emptyTarget: TargetForm = {
  channel_account_id: "",
  enabled: true,
  priority: "100",
  weight: "100",
};

const emptyForm: FormState = {
  name: "",
  enabled: true,
  priority: "100",
  app_scope: "all",
  app_ids: "",
  payment_method: "alipay",
  pay_modes: "",
  currency: "CNY",
  min_amount: "0",
  max_amount: "0",
  terminal: "any",
  targets: [{ ...emptyTarget }],
};

export function RoutingFormPage({ mode }: { mode: "create" | "edit" }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const accessToken = useAuthStore((state) => state.accessToken);
  const params = useParams({ strict: false });
  const ruleId = "ruleId" in params ? Number(params.ruleId) : 0;
  const [form, setForm] = useState<FormState>(emptyForm);
  const [accounts, setAccounts] = useState<ChannelAccount[]>([]);
  const [loading, setLoading] = useState(mode === "edit");
  const [saving, setSaving] = useState(false);

  const availableAccounts = useMemo(
    () =>
      accounts.filter((account) => account.channel === form.payment_method),
    [accounts, form.payment_method],
  );

  useEffect(() => {
    if (!accessToken) return;
    void listChannelAccounts(accessToken)
      .then((result) => setAccounts(result.items))
      .catch((err) => {
        toast.error(
          err instanceof APIError ? err.message : t("channels.loadFailed"),
        );
      });
  }, [accessToken, t]);

  useEffect(() => {
    if (mode !== "edit" || !accessToken || !ruleId) return;
    setLoading(true);
    void getRoutingRule(accessToken, ruleId)
      .then((result) => {
        const rule = result.routing_rule;
        setForm({
          name: rule.name,
          enabled: rule.enabled,
          priority: String(rule.priority),
          app_scope: rule.app_scope,
          app_ids: rule.app_ids.join("\n"),
          payment_method: rule.payment_method,
          pay_modes: rule.pay_modes.join("\n"),
          currency: rule.currency,
          min_amount: String(rule.min_amount),
          max_amount: String(rule.max_amount),
          terminal: rule.terminal,
          targets:
            rule.targets.length > 0
              ? rule.targets.map((target) => ({
                  channel_account_id: String(target.channel_account_id),
                  enabled: target.enabled,
                  priority: String(target.priority),
                  weight: String(target.weight),
                }))
              : [{ ...emptyTarget }],
        });
      })
      .catch((err) => {
        toast.error(
          err instanceof APIError ? err.message : t("routing.loadFailed"),
        );
      })
      .finally(() => setLoading(false));
  }, [accessToken, mode, ruleId, t]);

  function setValue<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((current) => ({ ...current, [key]: value }));
  }

  function setTarget(index: number, patch: Partial<TargetForm>) {
    setForm((current) => ({
      ...current,
      targets: current.targets.map((target, targetIndex) =>
        targetIndex === index ? { ...target, ...patch } : target,
      ),
    }));
  }

  function addTarget() {
    setForm((current) => ({
      ...current,
      targets: [...current.targets, { ...emptyTarget }],
    }));
  }

  function removeTarget(index: number) {
    setForm((current) => ({
      ...current,
      targets:
        current.targets.length > 1
          ? current.targets.filter((_, targetIndex) => targetIndex !== index)
          : current.targets,
    }));
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!accessToken) return;
    setSaving(true);
    const payload: ManageRoutingRulePayload = {
      name: form.name,
      enabled: form.enabled,
      priority: numberValue(form.priority),
      app_scope: form.app_scope,
      app_ids: form.app_scope === "include" ? splitList(form.app_ids) : [],
      payment_method: form.payment_method,
      pay_modes: splitList(form.pay_modes),
      currency: form.currency.trim(),
      min_amount: numberValue(form.min_amount),
      max_amount: numberValue(form.max_amount),
      terminal: form.terminal,
      metadata: {},
      targets: form.targets.map((target) => ({
        channel_account_id: numberValue(target.channel_account_id),
        enabled: target.enabled,
        priority: numberValue(target.priority),
        weight: numberValue(target.weight),
      })),
    };
    try {
      if (mode === "edit") {
        await updateRoutingRule(accessToken, ruleId, payload);
      } else {
        await createRoutingRule(accessToken, payload);
      }
      await navigate({ to: "/routing" });
    } catch (err) {
      toast.error(err instanceof APIError ? err.message : t("routing.saveFailed"));
    } finally {
      setSaving(false);
    }
  }

  const summary = buildRuleSummary(form, t);

  return (
    <div className="flex w-full flex-col gap-5">
      <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div className="flex min-w-0 flex-col gap-2">
          <Button variant="ghost" size="sm" className="w-fit" asChild>
            <Link to="/routing">
              <ArrowLeft data-icon="inline-start" />
              {t("routing.backToList")}
            </Link>
          </Button>
          <h1 className="text-2xl font-semibold tracking-tight">
            {mode === "edit"
              ? t("routing.editTitle")
              : t("routing.createTitle")}
          </h1>
          <p className="max-w-2xl text-sm text-muted-foreground">
            {t("routing.formDescription")}
          </p>
        </div>
      </div>

      {loading ? (
        <RoutingFormSkeleton />
      ) : (
        <form className="flex flex-col gap-5" onSubmit={handleSubmit}>
          <div className="grid items-start gap-5 xl:grid-cols-[minmax(0,1fr)_360px]">
            <div className="flex min-w-0 flex-col gap-5">
              <Card>
                <CardHeader>
                  <CardTitle>{t("routing.sections.identity")}</CardTitle>
                  <CardDescription>
                    {t("routing.sections.identityDescription")}
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <div className="grid gap-4 md:grid-cols-[minmax(0,1fr)_160px]">
                    <TextField
                      label={t("routing.fields.name")}
                      description={t("routing.hints.name")}
                      value={form.name}
                      onChange={(value) => setValue("name", value)}
                    />
                    <TextField
                      label={t("routing.fields.priority")}
                      description={t("routing.hints.priority")}
                      value={form.priority}
                      type="number"
                      onChange={(value) => setValue("priority", value)}
                    />
                  </div>
                </CardContent>
                <CardFooter className="justify-between">
                  <div>
                    <div className="text-sm font-medium">
                      {t("routing.fields.enabled")}
                    </div>
                    <div className="text-xs text-muted-foreground">
                      {t("routing.hints.enabled")}
                    </div>
                  </div>
                  <Switch
                    checked={form.enabled}
                    onCheckedChange={(checked) => setValue("enabled", checked)}
                  />
                </CardFooter>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>{t("routing.sections.match")}</CardTitle>
                  <CardDescription>
                    {t("routing.sections.matchDescription")}
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <FieldGroup>
                    <div className="grid gap-4 md:grid-cols-2">
                      <Field>
                        <FieldLabel>{t("routing.fields.appScope")}</FieldLabel>
                        <Select
                          value={form.app_scope}
                          onValueChange={(value) =>
                            setValue("app_scope", value as RoutingAppScope)
                          }
                        >
                          <SelectTrigger className="w-full">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectGroup>
                              <SelectItem value="all">
                                {t("routing.appScopes.all")}
                              </SelectItem>
                              <SelectItem value="include">
                                {t("routing.appScopes.include")}
                              </SelectItem>
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                        <FieldDescription>
                          {t("routing.hints.appScope")}
                        </FieldDescription>
                      </Field>
                      <Field>
                        <FieldLabel>{t("routing.fields.terminal")}</FieldLabel>
                        <Select
                          value={form.terminal}
                          onValueChange={(value) =>
                            setValue("terminal", value as RoutingTerminal)
                          }
                        >
                          <SelectTrigger className="w-full">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectGroup>
                              <SelectItem value="any">
                                {t("routing.terminals.any")}
                              </SelectItem>
                              <SelectItem value="desktop">
                                {t("routing.terminals.desktop")}
                              </SelectItem>
                              <SelectItem value="mobile">
                                {t("routing.terminals.mobile")}
                              </SelectItem>
                              <SelectItem value="wechat_browser">
                                {t("routing.terminals.wechat_browser")}
                              </SelectItem>
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                        <FieldDescription>
                          {t("routing.hints.terminal")}
                        </FieldDescription>
                      </Field>
                    </div>
                  {form.app_scope === "include" ? (
                    <TextAreaField
                      label={t("routing.fields.appIds")}
                      description={t("routing.hints.appIds")}
                      value={form.app_ids}
                      onChange={(value) => setValue("app_ids", value)}
                    />
                  ) : null}
                    <div className="grid gap-4 md:grid-cols-3">
                      <TextField
                        label={t("routing.fields.currency")}
                        description={t("routing.hints.currency")}
                        value={form.currency}
                        onChange={(value) => setValue("currency", value)}
                      />
                      <TextField
                        label={t("routing.fields.minAmount")}
                        description={t("routing.hints.minAmount")}
                        value={form.min_amount}
                        type="number"
                        onChange={(value) => setValue("min_amount", value)}
                      />
                      <TextField
                        label={t("routing.fields.maxAmount")}
                        description={t("routing.hints.maxAmount")}
                        value={form.max_amount}
                        type="number"
                        onChange={(value) => setValue("max_amount", value)}
                      />
                    </div>
                    <div className="grid gap-4 md:grid-cols-[240px_minmax(0,1fr)]">
                    <Field>
                      <FieldLabel>
                        {t("routing.fields.paymentMethod")}
                      </FieldLabel>
                      <Select
                        value={form.payment_method}
                        onValueChange={(value) =>
                          setValue(
                            "payment_method",
                            value as RoutingPaymentMethod,
                          )
                        }
                      >
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
                        {t("routing.hints.paymentMethod")}
                      </FieldDescription>
                    </Field>
                    <TextAreaField
                      label={t("routing.fields.payModes")}
                      description={t("routing.hints.payModes")}
                      value={form.pay_modes}
                      onChange={(value) => setValue("pay_modes", value)}
                    />
                    </div>
                  </FieldGroup>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>{t("routing.targets.title")}</CardTitle>
                  <CardDescription>
                    {t("routing.targets.description")}
                  </CardDescription>
                </CardHeader>
                <CardContent className="flex flex-col gap-3">
                  {availableAccounts.length === 0 ? (
                    <Alert>
                      <AlertTitle>{t("routing.targets.emptyTitle")}</AlertTitle>
                      <AlertDescription>
                        {t("routing.targets.emptyDescription")}
                      </AlertDescription>
                    </Alert>
                  ) : null}
                  {form.targets.map((target, index) => (
                    <TargetRow
                      key={index}
                      index={index}
                      target={target}
                      accounts={availableAccounts}
                      canRemove={form.targets.length > 1}
                      t={t}
                      onPatch={(patch) => setTarget(index, patch)}
                      onRemove={() => removeTarget(index)}
                    />
                  ))}
                  <Button
                    type="button"
                    variant="outline"
                    className="w-fit"
                    onClick={addTarget}
                  >
                    <Plus data-icon="inline-start" />
                    {t("routing.targets.add")}
                  </Button>
                </CardContent>
              </Card>
            </div>

            <RuleSummaryCard
              summary={summary}
              targetCount={form.targets.length}
              enabled={form.enabled}
              t={t}
            />
          </div>

          <div className="sticky bottom-0 flex justify-end gap-2 border-t bg-background/95 py-3 backdrop-blur">
            <Button variant="outline" type="button" asChild>
              <Link to="/routing">{t("routing.cancel")}</Link>
            </Button>
            <Button type="submit" disabled={saving}>
              {t("routing.save")}
            </Button>
          </div>
        </form>
      )}
    </div>
  );
}

function TextField({
  label,
  description,
  value,
  type = "text",
  onChange,
}: {
  label: string;
  description?: string;
  value: string;
  type?: string;
  onChange: (value: string) => void;
}) {
  return (
    <Field>
      <FieldLabel>{label}</FieldLabel>
      <Input
        type={type}
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
      {description ? (
        <FieldDescription>{description}</FieldDescription>
      ) : null}
    </Field>
  );
}

function TargetRow({
  index,
  target,
  accounts,
  canRemove,
  t,
  onPatch,
  onRemove,
}: {
  index: number;
  target: TargetForm;
  accounts: ChannelAccount[];
  canRemove: boolean;
  t: ReturnType<typeof useTranslation>["t"];
  onPatch: (patch: Partial<TargetForm>) => void;
  onRemove: () => void;
}) {
  return (
    <div className="grid gap-3 rounded-lg border bg-muted/20 p-3 md:grid-cols-[minmax(220px,1fr)_110px_110px_auto]">
      <Field>
        <div className="flex items-center justify-between gap-3">
          <FieldLabel>{t("routing.targets.account")}</FieldLabel>
          <Badge variant="outline">
            {t("routing.targets.index", { index: index + 1 })}
          </Badge>
        </div>
        <Select
          value={target.channel_account_id}
          onValueChange={(value) => onPatch({ channel_account_id: value })}
        >
          <SelectTrigger className="w-full">
            <SelectValue
              placeholder={t("routing.targets.accountPlaceholder")}
            />
          </SelectTrigger>
          <SelectContent>
            <SelectGroup>
              {accounts.map((account) => (
                <SelectItem key={account.id} value={String(account.id)}>
                  {account.name} · {t(`channels.${account.channel}`)}
                </SelectItem>
              ))}
            </SelectGroup>
          </SelectContent>
        </Select>
      </Field>
      <TextField
        label={t("routing.targets.priority")}
        value={target.priority}
        type="number"
        onChange={(value) => onPatch({ priority: value })}
      />
      <TextField
        label={t("routing.targets.weight")}
        value={target.weight}
        type="number"
        onChange={(value) => onPatch({ weight: value })}
      />
      <div className="flex items-end gap-2 md:justify-end">
        <Field className="min-w-20">
          <FieldLabel>{t("routing.targets.enabled")}</FieldLabel>
          <Switch
            checked={target.enabled}
            onCheckedChange={(checked) => onPatch({ enabled: checked })}
          />
        </Field>
        <Button
          type="button"
          variant="outline"
          size="icon"
          onClick={onRemove}
          disabled={!canRemove}
          aria-label={t("routing.targets.remove")}
        >
          <Trash2 />
        </Button>
      </div>
    </div>
  );
}

function RuleSummaryCard({
  summary,
  targetCount,
  enabled,
  t,
}: {
  summary: Array<[string, string]>;
  targetCount: number;
  enabled: boolean;
  t: ReturnType<typeof useTranslation>["t"];
}) {
  return (
    <Card className="xl:sticky xl:top-5">
      <CardHeader>
        <CardTitle>{t("routing.summary.title")}</CardTitle>
        <CardDescription>{t("routing.summary.description")}</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        <div className="flex items-center justify-between rounded-lg border px-3 py-2">
          <span className="text-sm text-muted-foreground">
            {t("routing.fields.enabled")}
          </span>
          <Badge variant={enabled ? "secondary" : "outline"}>
            {enabled ? t("routing.enabled") : t("routing.disabled")}
          </Badge>
        </div>
        {summary.map(([label, value]) => (
          <div
            key={label}
            className="flex items-start justify-between gap-3 rounded-lg border px-3 py-2"
          >
            <span className="text-sm text-muted-foreground">{label}</span>
            <span className="break-all text-right text-xs font-medium">
              {value || "-"}
            </span>
          </div>
        ))}
        <div className="flex items-center justify-between rounded-lg border px-3 py-2">
          <span className="text-sm text-muted-foreground">
            {t("routing.targets.title")}
          </span>
          <Badge variant="secondary">
            {t("routing.table.targetCount", { count: targetCount })}
          </Badge>
        </div>
      </CardContent>
    </Card>
  );
}

function RoutingFormSkeleton() {
  return (
    <div className="grid items-start gap-5 xl:grid-cols-[minmax(0,1fr)_360px]">
      <div className="flex flex-col gap-5">
        <Card>
          <CardHeader>
            <Skeleton className="h-5 w-36" />
            <Skeleton className="h-4 w-72" />
          </CardHeader>
          <CardContent>
            <div className="grid gap-4 md:grid-cols-2">
              <Skeleton className="h-16" />
              <Skeleton className="h-16" />
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <Skeleton className="h-5 w-32" />
            <Skeleton className="h-4 w-80" />
          </CardHeader>
          <CardContent className="flex flex-col gap-4">
            <Skeleton className="h-16" />
            <Skeleton className="h-16" />
            <Skeleton className="h-24" />
          </CardContent>
        </Card>
      </div>
      <Card>
        <CardHeader>
          <Skeleton className="h-5 w-28" />
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <Skeleton className="h-10" />
          <Skeleton className="h-10" />
          <Skeleton className="h-10" />
        </CardContent>
      </Card>
    </div>
  );
}

function TextAreaField({
  label,
  description,
  value,
  onChange,
}: {
  label: string;
  description?: string;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <Field>
      <FieldLabel>{label}</FieldLabel>
      <Textarea
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
      {description ? (
        <FieldDescription>{description}</FieldDescription>
      ) : null}
    </Field>
  );
}

function numberValue(value: string) {
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) ? parsed : 0;
}

function splitList(value: string) {
  return value
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function buildRuleSummary(
  form: FormState,
  t: ReturnType<typeof useTranslation>["t"],
): Array<[string, string]> {
  return [
    [t("routing.fields.priority"), form.priority],
    [
      t("routing.fields.appScope"),
      form.app_scope === "all"
        ? t("routing.appScopes.all")
        : splitList(form.app_ids).join(", "),
    ],
    [
      t("routing.table.amount"),
      formatAmountSummary(form.min_amount, form.max_amount),
    ],
    [t("routing.fields.terminal"), t(`routing.terminals.${form.terminal}`)],
    [t("routing.fields.paymentMethod"), t(`channels.${form.payment_method}`)],
    [t("routing.fields.payModes"), splitList(form.pay_modes).join(", ") || "*"],
  ];
}

function formatAmountSummary(min: string, max: string) {
  const minValue = numberValue(min);
  const maxValue = numberValue(max);
  if (!minValue && !maxValue) return "*";
  if (!minValue) return `<= ${maxValue}`;
  if (!maxValue) return `>= ${minValue}`;
  return `${minValue} - ${maxValue}`;
}
