import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { CheckCircle2, CreditCard, RotateCcw } from "lucide-react";

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

import { completeMockPayment } from "./api";

type MockPayPageProps = {
  gatewayOrderNo: string;
};

export function MockPayPage({ gatewayOrderNo }: MockPayPageProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleComplete() {
    setSubmitting(true);
    setError(null);
    try {
      await completeMockPayment(gatewayOrderNo);
      await navigate({
        to: "/checkout/$gatewayOrderNo",
        params: { gatewayOrderNo },
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : t("mockPay.failed"));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="min-h-[100dvh] bg-background px-4 py-6 text-foreground md:px-6">
      <div className="mx-auto flex w-full max-w-md flex-col gap-5">
        <header className="flex items-center justify-between gap-4">
          <div>
            <p className="text-sm text-muted-foreground">{t("mockPay.gateway")}</p>
            <h1 className="text-xl font-semibold">{t("mockPay.title")}</h1>
          </div>
          <Badge variant="secondary">Mock</Badge>
        </header>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <CreditCard />
              {t("mockPay.cardTitle")}
            </CardTitle>
            <CardDescription>{t("mockPay.description")}</CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-4">
            <div className="rounded-md border p-3">
              <p className="text-xs text-muted-foreground">
                {t("checkout.gatewayOrderNo")}
              </p>
              <p className="mt-1 truncate font-mono text-sm">{gatewayOrderNo}</p>
            </div>
            {error ? (
              <Alert variant="destructive">
                <RotateCcw />
                <AlertTitle>{t("mockPay.failed")}</AlertTitle>
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            ) : (
              <Alert>
                <CheckCircle2 />
                <AlertTitle>{t("mockPay.ready")}</AlertTitle>
                <AlertDescription>{t("mockPay.readyDescription")}</AlertDescription>
              </Alert>
            )}
          </CardContent>
          <CardFooter>
            <Button className="w-full" disabled={submitting} onClick={handleComplete}>
              {submitting ? t("mockPay.submitting") : t("mockPay.complete")}
            </Button>
          </CardFooter>
        </Card>
      </div>
    </main>
  );
}
