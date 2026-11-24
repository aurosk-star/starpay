import { AlertCircle, Inbox, LoaderCircle } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";

type PageStateProps =
  | { kind: "loading"; message?: string }
  | { kind: "empty"; title?: string; description?: string }
  | {
      kind: "error";
      title?: string;
      description?: string;
      onRetry?: () => void;
    };

export function PageState(props: PageStateProps) {
  const { t } = useTranslation();
  if (props.kind === "loading") {
    return (
      <Card>
        <CardContent className="flex min-h-32 items-center justify-center gap-2 text-sm text-muted-foreground">
          <LoaderCircle className="size-4 animate-spin" />
          {props.message ?? t("common.loading")}
        </CardContent>
      </Card>
    );
  }
  if (props.kind === "empty") {
    return (
      <Card>
        <CardContent className="flex min-h-32 flex-col items-center justify-center gap-2 text-center">
          <Inbox className="size-6 text-muted-foreground" />
          <p className="font-medium">
            {props.title ?? t("pageState.emptyTitle")}
          </p>
          <p className="text-sm text-muted-foreground">
            {props.description ?? t("pageState.emptyDescription")}
          </p>
        </CardContent>
      </Card>
    );
  }
  return (
    <Alert variant="destructive">
      <AlertCircle />
      <AlertTitle>{props.title ?? t("pageState.errorTitle")}</AlertTitle>
      <AlertDescription className="flex items-center justify-between gap-4">
        <span>{props.description ?? t("pageState.errorDescription")}</span>
        {props.onRetry ? (
          <Button variant="outline" size="sm" onClick={props.onRetry}>
            {t("common.retry")}
          </Button>
        ) : null}
      </AlertDescription>
    </Alert>
  );
}
