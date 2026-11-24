import { useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { APIError } from "@/lib/api";

import { updateOrder } from "./api";
import type { PaymentOrder } from "./types";

type OrderEditDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  order: PaymentOrder | null;
  accessToken: string;
  onUpdated: (order: PaymentOrder) => void;
};

type EditForm = {
  business_type: string;
  subject: string;
  description: string;
  channel: string;
  pay_method: string;
};

const emptyForm: EditForm = {
  business_type: "",
  subject: "",
  description: "",
  channel: "",
  pay_method: "",
};

export function OrderEditDialog({
  open,
  onOpenChange,
  order,
  accessToken,
  onUpdated,
}: OrderEditDialogProps) {
  const { t } = useTranslation();
  const [form, setForm] = useState<EditForm>(emptyForm);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open || !order) return;
    setForm({
      business_type: order.business_type ?? "",
      subject: order.subject ?? "",
      description: order.description ?? "",
      channel: order.channel ?? "",
      pay_method: order.pay_method ?? "",
    });
  }, [open, order]);

  function setValue<K extends keyof EditForm>(key: K, value: EditForm[K]) {
    setForm((current) => ({ ...current, [key]: value }));
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!order || !accessToken) return;
    setSaving(true);
    try {
      const result = await updateOrder(accessToken, order.id, {
        business_type: form.business_type.trim() || undefined,
        subject: form.subject.trim(),
        description: form.description.trim() || undefined,
        channel: form.channel.trim() || undefined,
        pay_method: form.pay_method.trim() || undefined,
      });
      onUpdated(result.order);
      toast.success(t("common.feedback.updated"));
      onOpenChange(false);
    } catch (err) {
      toast.error(err instanceof APIError ? err.message : t("orders.edit.failed"));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t("orders.edit.title")}</DialogTitle>
          <DialogDescription>{t("orders.edit.description")}</DialogDescription>
        </DialogHeader>
        <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="order_subject">
                {t("orders.edit.subject")}
              </FieldLabel>
              <Input
                id="order_subject"
                value={form.subject}
                required
                onChange={(event) => setValue("subject", event.target.value)}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="order_business_type">
                {t("orders.edit.businessType")}
              </FieldLabel>
              <Input
                id="order_business_type"
                value={form.business_type}
                onChange={(event) =>
                  setValue("business_type", event.target.value)
                }
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="order_description">
                {t("orders.edit.orderDescription")}
              </FieldLabel>
              <Textarea
                id="order_description"
                value={form.description}
                onChange={(event) =>
                  setValue("description", event.target.value)
                }
              />
            </Field>
            <div className="grid gap-4 sm:grid-cols-2">
              <Field>
                <FieldLabel htmlFor="order_channel">
                  {t("orders.edit.channel")}
                </FieldLabel>
                <Input
                  id="order_channel"
                  value={form.channel}
                  onChange={(event) => setValue("channel", event.target.value)}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="order_pay_method">
                  {t("orders.edit.payMethod")}
                </FieldLabel>
                <Input
                  id="order_pay_method"
                  value={form.pay_method}
                  onChange={(event) =>
                    setValue("pay_method", event.target.value)
                  }
                />
              </Field>
            </div>
          </FieldGroup>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
            >
              {t("orders.cancel")}
            </Button>
            <Button type="submit" disabled={saving}>
              {saving ? t("orders.edit.saving") : t("orders.edit.save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
