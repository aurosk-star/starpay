import {
  flexRender,
  getCoreRowModel,
  getPaginationRowModel,
  useReactTable,
  type ColumnDef,
} from "@tanstack/react-table";
import type * as React from "react";

import {
  Pagination,
  PaginationContent,
  PaginationEllipsis,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from "@/components/ui/pagination";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { cn } from "@/lib/utils";

type DataTableProps<TData, TValue> = {
  columns: ColumnDef<TData, TValue>[];
  data: TData[];
  emptyText?: string;
  loadingText?: string;
  loading?: boolean;
  pageSize?: number;
};

export function DataTable<TData, TValue>({
  columns,
  data,
  emptyText = "暂无数据",
  loadingText = "加载中...",
  loading = false,
  pageSize = 10,
}: DataTableProps<TData, TValue>) {
  const table = useReactTable({
    data,
    columns,
    getCoreRowModel: getCoreRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    initialState: {
      pagination: {
        pageSize,
      },
    },
  });
  const currentPage = table.getState().pagination.pageIndex + 1;
  const pageCount = table.getPageCount();
  const canPreviousPage = table.getCanPreviousPage();
  const canNextPage = table.getCanNextPage();
  const paginationItems = getPaginationItems(currentPage, pageCount);

  function goToPreviousPage(event: React.MouseEvent<HTMLAnchorElement>) {
    event.preventDefault();
    if (canPreviousPage) table.previousPage();
  }

  function goToNextPage(event: React.MouseEvent<HTMLAnchorElement>) {
    event.preventDefault();
    if (canNextPage) table.nextPage();
  }

  return (
    <div className="flex min-w-0 max-w-full flex-col gap-3">
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
                  {emptyText}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </ScrollArea>
      {data.length > pageSize ? (
        <Pagination className="w-full justify-start overflow-x-auto pb-1 sm:justify-end">
          <PaginationContent className="min-w-max">
            <PaginationItem>
              <PaginationPrevious
                href="#"
                aria-disabled={!canPreviousPage}
                tabIndex={canPreviousPage ? undefined : -1}
                className={cn(
                  !canPreviousPage && "pointer-events-none opacity-50",
                )}
                onClick={goToPreviousPage}
              >
                上一页
              </PaginationPrevious>
            </PaginationItem>
            {paginationItems.map((item, index) =>
              item === "ellipsis" ? (
                <PaginationItem key={`ellipsis-${index}`}>
                  <PaginationEllipsis />
                </PaginationItem>
              ) : (
                <PaginationItem key={item}>
                  <PaginationLink
                    href="#"
                    isActive={item === currentPage}
                    onClick={(event) => {
                      event.preventDefault();
                      table.setPageIndex(item - 1);
                    }}
                  >
                    {item}
                  </PaginationLink>
                </PaginationItem>
              ),
            )}
            <PaginationItem>
              <PaginationNext
                href="#"
                aria-disabled={!canNextPage}
                tabIndex={canNextPage ? undefined : -1}
                className={cn(!canNextPage && "pointer-events-none opacity-50")}
                onClick={goToNextPage}
              >
                下一页
              </PaginationNext>
            </PaginationItem>
          </PaginationContent>
        </Pagination>
      ) : null}
    </div>
  );
}

export type DataTableColumn<TData, TValue = unknown> = ColumnDef<TData, TValue>;

function getPaginationItems(
  currentPage: number,
  pageCount: number,
): Array<number | "ellipsis"> {
  if (pageCount <= 7) {
    return Array.from({ length: pageCount }, (_, index) => index + 1);
  }

  if (currentPage <= 4) {
    return [1, 2, 3, 4, 5, "ellipsis", pageCount];
  }

  if (currentPage >= pageCount - 3) {
    return [
      1,
      "ellipsis",
      pageCount - 4,
      pageCount - 3,
      pageCount - 2,
      pageCount - 1,
      pageCount,
    ];
  }

  return [
    1,
    "ellipsis",
    currentPage - 1,
    currentPage,
    currentPage + 1,
    "ellipsis",
    pageCount,
  ];
}
