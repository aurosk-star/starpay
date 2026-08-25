import type { Table } from "@tanstack/react-table";
import type * as React from "react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

export type DataTableToolbarProps<TData> = {
  table: Table<TData>;
  children?: React.ReactNode;
  searchColumn?: string;
  searchPlaceholder?: string;
  className?: string;
};

/** A small composable toolbar; filters and actions are supplied by the caller. */
export function DataTableToolbar<TData>({
  table,
  children,
  searchColumn,
  searchPlaceholder,
  className,
}: DataTableToolbarProps<TData>) {
  const { t } = useTranslation();
  const resolvedSearchPlaceholder =
    searchPlaceholder ?? t("dataTable.filterPlaceholder");
  const searchValue = searchColumn
    ? String(table.getColumn(searchColumn)?.getFilterValue() ?? "")
    : "";

  return (
    <div className={cn("flex flex-wrap items-center gap-2", className)}>
      {searchColumn && table.getColumn(searchColumn) ? (
        <Input
          value={searchValue}
          placeholder={resolvedSearchPlaceholder}
          onChange={(event) =>
            table.getColumn(searchColumn)?.setFilterValue(event.target.value)
          }
          className="h-8 w-full sm:w-56"
          aria-label={resolvedSearchPlaceholder}
        />
      ) : null}
      {children}
    </div>
  );
}
