import type { Table } from "@tanstack/react-table";
import {
  Pagination,
  PaginationContent,
  PaginationEllipsis,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from "@/components/ui/pagination";
import { cn } from "@/lib/utils";

export type DataTablePaginationProps<TData> = {
  table: Table<TData>;
  pageSize?: number;
  previousLabel?: string;
  nextLabel?: string;
};

/** Responsive pagination wired to TanStack Table's pagination state. */
export function DataTablePagination<TData>({
  table,
  pageSize,
  previousLabel = "上一页",
  nextLabel = "下一页",
}: DataTablePaginationProps<TData>) {
  const pageCount = table.getPageCount();
  const currentPage = table.getState().pagination.pageIndex + 1;
  const canPreviousPage = table.getCanPreviousPage();
  const canNextPage = table.getCanNextPage();

  // Keep the legacy default: do not show controls for a single page.
  if (
    pageCount <= 1 ||
    (pageSize !== undefined &&
      table.getPrePaginationRowModel().rows.length <= pageSize)
  ) {
    return null;
  }

  return (
    <Pagination className="w-full justify-start overflow-x-auto pb-1 sm:justify-end">
      <PaginationContent className="min-w-max">
        <PaginationItem>
          <PaginationPrevious
            href="#"
            aria-disabled={!canPreviousPage}
            tabIndex={canPreviousPage ? undefined : -1}
            className={cn(!canPreviousPage && "pointer-events-none opacity-50")}
            onClick={(event) => {
              event.preventDefault();
              if (canPreviousPage) table.previousPage();
            }}
          >
            {previousLabel}
          </PaginationPrevious>
        </PaginationItem>
        {getPaginationItems(currentPage, pageCount).map((item, index) =>
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
            onClick={(event) => {
              event.preventDefault();
              if (canNextPage) table.nextPage();
            }}
          >
            {nextLabel}
          </PaginationNext>
        </PaginationItem>
      </PaginationContent>
    </Pagination>
  );
}

export function getPaginationItems(
  currentPage: number,
  pageCount: number,
): Array<number | "ellipsis"> {
  if (pageCount <= 7) {
    return Array.from({ length: pageCount }, (_, index) => index + 1);
  }
  if (currentPage <= 4) return [1, 2, 3, 4, 5, "ellipsis", pageCount];
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
