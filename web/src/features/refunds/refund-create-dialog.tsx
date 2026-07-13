import { useState, type FormEvent } from "react";
import { Plus } from "lucide-react";
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
  DialogTrigger,
} from "@/components/ui/dialog";
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { useAuthStore } from "@/features/auth/store";
import { APIError } from "@/lib/api";
import { createRefund } from "./api";
import type { Refund } from "./types";

export function RefundCreateDialog({
  onCreated,
}: {
  onCreated: (refund: Refund) => void;
}) {
  const { t } = useTranslation();
  const token = useAuthStore((s) => s.accessToken);
  const [open, setOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [form, setForm] = useState({
    app_id: "",
    gateway_order_no: "",
    merchant_refund_no: "",
    amount: "",
    currency: "CNY",
    reason: "",
  });
  async function submit(event: FormEvent) {
    event.preventDefault();
    if (!token) return;
    setSubmitting(true);
    try {
      const result = await createRefund(token, {
        ...form,
        amount: Number(form.amount),
      });
      onCreated(result.refund);
      setOpen(false);
      setForm({
        app_id: "",
        gateway_order_no: "",
        merchant_refund_no: "",
        amount: "",
        currency: "CNY",
        reason: "",
      });
      toast.success(t("refunds.created"));
    } catch (err) {
      toast.error(
        err instanceof APIError ? err.message : t("refunds.createFailed"),
      );
    } finally {
      setSubmitting(false);
    }
  }
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button>
          <Plus data-icon="inline-start" />
          {t("refunds.create")}
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("refunds.createTitle")}</DialogTitle>
          <DialogDescription>
            {t("refunds.createDescription")}
          </DialogDescription>
        </DialogHeader>
        <form className="flex flex-col gap-4" onSubmit={submit}>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="refund-app">
                {t("refunds.fields.app")}
              </FieldLabel>
              <Input
                id="refund-app"
                required
                value={form.app_id}
                onChange={(e) =>
                  setForm((v) => ({ ...v, app_id: e.target.value }))
                }
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="refund-order">
                {t("refunds.fields.order")}
              </FieldLabel>
              <Input
                id="refund-order"
                required
                value={form.gateway_order_no}
                onChange={(e) =>
                  setForm((v) => ({ ...v, gateway_order_no: e.target.value }))
                }
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="refund-merchant">
                {t("refunds.fields.merchantRefund")}
              </FieldLabel>
              <Input
                id="refund-merchant"
                required
                value={form.merchant_refund_no}
                onChange={(e) =>
                  setForm((v) => ({ ...v, merchant_refund_no: e.target.value }))
                }
              />
            </Field>
            <div className="grid grid-cols-2 gap-3">
              <Field>
                <FieldLabel htmlFor="refund-amount">
                  {t("refunds.fields.amount")}
                </FieldLabel>
                <Input
                  id="refund-amount"
                  type="number"
                  min="1"
                  required
                  value={form.amount}
                  onChange={(e) =>
                    setForm((v) => ({ ...v, amount: e.target.value }))
                  }
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="refund-currency">
                  {t("refunds.fields.currency")}
                </FieldLabel>
                <Input
                  id="refund-currency"
                  required
                  value={form.currency}
                  onChange={(e) =>
                    setForm((v) => ({
                      ...v,
                      currency: e.target.value.toUpperCase(),
                    }))
                  }
                />
              </Field>
            </div>
            <Field>
              <FieldLabel htmlFor="refund-reason">
                {t("refunds.fields.reason")}
              </FieldLabel>
              <Textarea
                id="refund-reason"
                value={form.reason}
                onChange={(e) =>
                  setForm((v) => ({ ...v, reason: e.target.value }))
                }
              />
            </Field>
          </FieldGroup>
          <DialogFooter>
            <Button type="submit" disabled={submitting}>
              {submitting ? t("refunds.creating") : t("refunds.create")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
