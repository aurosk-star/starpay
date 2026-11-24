import {
  flexRender,
  getCoreRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  useReactTable,
  type ColumnDef,
} from "@tanstack/react-table";
import * as React from "react";

import { ScrollArea } from "@/components/ui/scroll-area";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { DataTablePagination } from "./pagination";
import { DataTableViewOptions } from "./view-options";
import type { Table as ReactTable } from "@tanstack/react-table";

type DataTableProps<TData, TValue> = {
  columns: ColumnDef<TData, TValue>[];
  data: TData[];
  emptyText?: string;
  loadingText?: string;
  loading?: boolean;
  pageSize?: number;
  toolbar?: DataTableSlot<TData>;
  pagination?: DataTableSlot<TData> | false;
  viewOptions?: DataTableSlot<TData> | boolean;
  emptyState?:
    React.ReactNode | ((table: ReactTable<TData>) => React.ReactNode);
};

export type DataTableSlot<TData> =
  React.ReactNode | React.ComponentType<{ table: ReactTable<TData> }>;

export function DataTable<TData, TValue>({
  columns,
  data,
  emptyText = "暂无数据",
  loadingText = "加载中...",
  loading = false,
  pageSize = 10,
  toolbar,
  pagination,
  viewOptions,
  emptyState,
}: DataTableProps<TData, TValue>) {
  const table = useReactTable({
    data,
    columns,
    getCoreRowModel: getCoreRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getSortedRowModel: getSortedRowModel(),
    initialState: {
      pagination: {
        pageSize,
      },
    },
  });

  const renderSlot = (
    slot: DataTableSlot<TData> | undefined,
  ): React.ReactNode =>
    typeof slot === "function" ? React.createElement(slot, { table }) : slot;
  const renderedEmptyState =
    typeof emptyState === "function" ? emptyState(table) : emptyState;

  return (
    <div className="flex min-w-0 max-w-full flex-col gap-3">
      {toolbar ? renderSlot(toolbar) : null}
      <ScrollArea className="h-[min(72vh,44rem)] max-w-full rounded-md border">
        <Table containerClassName="overflow-visible">
          <TableHeader className="sticky top-0 z-20 bg-background">
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <TableHead key={header.id} className="bg-background px-4">
                    {header.isPlaceholder
                      ? null
                      : flexRender(
                          header.column.columnDef.header,
                          header.getContext(),
                        )}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {loading ? (
              <TableRow>
                <TableCell
                  colSpan={columns.length}
                  className="h-20 px-4 text-muted-foreground"
                >
                  {loadingText}
                </TableCell>
              </TableRow>
            ) : table.getRowModel().rows.length ? (
              table.getRowModel().rows.map((row) => (
                <TableRow key={row.id}>
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id} className="px-4">
                      {flexRender(
                        cell.column.columnDef.cell,
                        cell.getContext(),
                      )}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell
                  colSpan={columns.length}
                  className="h-20 px-4 text-muted-foreground"
                >
                  {renderedEmptyState ?? emptyText}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </ScrollArea>
      {viewOptions === true ? (
        <DataTableViewOptions table={table} />
      ) : viewOptions ? (
        renderSlot(viewOptions)
      ) : null}
      {pagination !== false ? (
        pagination ? (
          renderSlot(pagination)
        ) : (
          <DataTablePagination table={table} pageSize={pageSize} />
        )
      ) : null}
    </div>
  );
}

export type DataTableColumn<TData, TValue = unknown> = ColumnDef<TData, TValue>;
