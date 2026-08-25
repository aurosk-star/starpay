import type { Column } from "@tanstack/react-table";
import { ArrowDown, ArrowUp, ChevronsUpDown } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export function DataTableColumnHeader<TData, TValue>({
  column,
  title,
  className,
}: {
  column: Column<TData, TValue>;
  title: string;
  className?: string;
}) {
  if (!column.getCanSort()) return <div className={cn(className)}>{title}</div>;

  const sorted = column.getIsSorted();
  const Icon =
    sorted === "asc" ? ArrowUp : sorted === "desc" ? ArrowDown : ChevronsUpDown;

  return (
    <Button
      variant="ghost"
      size="sm"
      className={cn("-ml-3 h-8 data-[state=open]:bg-accent", className)}
      onClick={() => column.toggleSorting(sorted === "asc")}
    >
      {title}
      <Icon data-icon="inline-end" />
    </Button>
  );
}
