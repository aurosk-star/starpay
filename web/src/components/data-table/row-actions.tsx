import type { ComponentType, ReactNode, SVGProps } from "react";
import { MoreHorizontal } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";

export type DataTableRowAction = {
  label: ReactNode;
  icon?: ComponentType<SVGProps<SVGSVGElement>>;
  onClick?: () => void;
  disabled?: boolean;
  variant?: "outline" | "ghost" | "destructive" | "secondary" | "default";
  asChild?: boolean;
  child?: ReactNode;
};

export function DataTableRowActions({
  actions,
  className,
}: {
  actions: DataTableRowAction[];
  className?: string;
}) {
  const visibleActions = actions.slice(0, 2);
  const overflowActions = actions.slice(2);

  return (
    <div className={cn("flex justify-end gap-2", className)}>
      {visibleActions.map((action, index) => (
        <RowActionButton key={index} action={action} />
      ))}
      {overflowActions.length > 0 ? (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="outline" size="icon-sm">
              <MoreHorizontal />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            {overflowActions.map((action, index) => {
              const Icon = action.icon;
              return (
                <DropdownMenuItem
                  key={index}
                  disabled={action.disabled}
                  onClick={action.onClick}
                  asChild={action.asChild}
                >
                  {action.asChild && action.child ? (
                    action.child
                  ) : (
                    <>
                      {Icon ? <Icon data-icon="inline-start" /> : null}
                      {action.label}
                    </>
                  )}
                </DropdownMenuItem>
              );
            })}
          </DropdownMenuContent>
        </DropdownMenu>
      ) : null}
    </div>
  );
}

function RowActionButton({ action }: { action: DataTableRowAction }) {
  const Icon = action.icon;
  const content =
    action.asChild && action.child ? (
      action.child
    ) : (
      <>
        {Icon ? <Icon data-icon="inline-start" /> : null}
        {action.label}
      </>
    );

  return (
    <Button
      variant={action.variant ?? "outline"}
      size="sm"
      disabled={action.disabled}
      onClick={action.onClick}
      asChild={action.asChild}
    >
      {content}
    </Button>
  );
}
