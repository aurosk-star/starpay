import {
  DataTable,
  type DataTableColumn,
  type DataTableSlot,
} from "./data-table";
import type * as React from "react";

export function createDataTable<TData>() {
  return function TypedDataTable<TValue>({
    columns,
    data,
    emptyText,
    loadingText,
    loading,
    pageSize,
    toolbar,
    pagination,
    viewOptions,
    emptyState,
  }: {
    columns: DataTableColumn<TData, TValue>[];
    data: TData[];
    emptyText?: string;
    loadingText?: string;
    loading?: boolean;
    pageSize?: number;
    toolbar?: DataTableSlot<TData>;
    pagination?: DataTableSlot<TData> | false;
    viewOptions?: DataTableSlot<TData> | boolean;
    emptyState?:
      | React.ReactNode
      | ((
          table: import("@tanstack/react-table").Table<TData>,
        ) => React.ReactNode);
  }) {
    return (
      <DataTable
        columns={columns}
        data={data}
        emptyText={emptyText}
        loadingText={loadingText}
        loading={loading}
        pageSize={pageSize}
        toolbar={toolbar}
        pagination={pagination}
        viewOptions={viewOptions}
        emptyState={emptyState}
      />
    );
  };
}

export type { DataTableColumn };
