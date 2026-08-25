import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { ArrowLeft, ShieldAlert } from "lucide-react";

import { Button } from "@/components/ui/button";

export type ErrorStatus = "401" | "403" | "404" | "500";

export function ErrorPage({ status }: { status: ErrorStatus }) {
  const { t } = useTranslation();
  return (
    <div className="flex min-h-[min(60vh,36rem)] flex-col items-center justify-center gap-4 text-center">
      <div className="flex size-14 items-center justify-center rounded-2xl bg-muted text-muted-foreground">
        <ShieldAlert className="size-7" />
      </div>
      <p className="text-6xl font-semibold tracking-tight">{status}</p>
      <div className="space-y-1">
        <h1 className="text-xl font-semibold">{t(`errors.${status}.title`)}</h1>
        <p className="text-sm text-muted-foreground">
          {t(`errors.${status}.description`)}
        </p>
      </div>
      <Button asChild variant="outline">
        <Link to="/">
          <ArrowLeft data-icon="inline-start" />
          {t("errors.backHome")}
        </Link>
      </Button>
    </div>
  );
}
