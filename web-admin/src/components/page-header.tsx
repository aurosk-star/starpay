import type { ReactNode } from "react";
import { Link } from "@tanstack/react-router";
import { ArrowLeft } from "lucide-react";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export type PageHeaderProps = {
  title: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  /** 标题右侧的状态徽标等 */
  badge?: ReactNode;
  /** 返回列表页的链接 */
  back?: { to: string; label: string };
  className?: string;
  descriptionClassName?: string;
};

/** Shared shadcn-admin page heading with responsive action placement. */
export function PageHeader({
  title,
  description,
  actions,
  badge,
  back,
  className,
  descriptionClassName,
}: PageHeaderProps) {
  return (
    <div
      className={cn(
        "flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between",
        className,
      )}
    >
      <div className="min-w-0">
        {back ? (
          <Button
            variant="ghost"
            size="sm"
            className="mb-2 w-fit -ms-3"
            asChild
          >
            <Link to={back.to as never}>
              <ArrowLeft data-icon="inline-start" />
              {back.label}
            </Link>
          </Button>
        ) : null}
        <div className="flex flex-wrap items-center gap-2">
          <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
          {badge}
        </div>
        {description ? (
          <p
            className={cn(
              "mt-1 text-sm text-muted-foreground",
              descriptionClassName,
            )}
          >
            {description}
          </p>
        ) : null}
      </div>
      {actions ? (
        <div className="flex shrink-0 flex-wrap items-center gap-2">
          {actions}
        </div>
      ) : null}
    </div>
  );
}
