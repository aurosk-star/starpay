import { DataTable, type DataTableColumn } from "./data-table";

export function createDataTable<TData>() {
  return function TypedDataTable<TValue>({
    columns,
    data,
    emptyText,
    loadingText,
    loading,
    pageSize,
  }: {
    columns: DataTableColumn<TData, TValue>[];
    data: TData[];
    emptyText?: string;
    loadingText?: string;
    loading?: boolean;
    pageSize?: number;
  }) {
    return (
      <DataTable
        columns={columns}
        data={data}
        emptyText={emptyText}
        loadingText={loadingText}
        loading={loading}
        pageSize={pageSize}
      />
    );
  };
}

export type { DataTableColumn };
