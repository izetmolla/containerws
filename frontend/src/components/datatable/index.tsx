"use client";

/**
 * DataTable – server and client-side table with URL-synced state.
 *
 * Single public component: <DataTable /> with columns + source (server | client).
 * No lazy loading: all imports are direct to keep bundles predictable and avoid
 * layout shift. Exports hooks and subcomponents for advanced composition.
 */

import type { ColumnDef } from "@tanstack/react-table";
import { flexRender } from "@tanstack/react-table";
import * as React from "react";

import "./types/data-table";
import { DataTableAdvancedToolbar } from "./components/data-table-advanced-toolbar";
import { DataTableFilterList } from "./components/data-table-filter-list";
import { DataTablePagination } from "./components/data-table-pagination";
import { DataTableToolbar } from "./components/data-table-toolbar";
import { DataTableSortList } from "./components/data-table-sort-list";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Checkbox } from "@/components/ui/checkbox";
import { Loader2 } from "lucide-react";
import { getCommonPinningStyles } from "./lib/data-table";
import { useDataTable } from "./hooks/use-datatable";
import { useServerTableData } from "./hooks/use-server-table-data";
import { LIST_ALL_PER_PAGE } from "./lib/constants";
import { useTableAdvancedOptions } from "./components/data-table-advanced-options";
import {
  DataTableLocalStateProvider,
  useCreateDataTableLocalState,
} from "./context/data-table-local-state";
import type { UseServerTableDataOptions, ExtendedColumnSort } from "./types/data-table";
import type { Table as TanStackTable, TableState } from "@tanstack/react-table";
import { cn } from "@/lib/utils";
import { DataTableEditableCell } from "./components/data-table-editable-cell";
import { isColumnEditable, resolveCellEditorConfig } from "./lib/cell-editor";
import type { CellEditEvent } from "./types/data-table";

/** Server-side data source: fetch on each paginate/sort/filter */
export interface DataTableServerSource<TData> {
  type: "server";
  options: UseServerTableDataOptions<TData>;
}

/** Client-side data source: load once, filter/sort/paginate in browser */
export interface DataTableClientSource<TData> {
  type: "client";
  data: TData[];
}

export type DataTableSource<TData> =
  | DataTableServerSource<TData>
  | DataTableClientSource<TData>;

export interface DataTableProps<TData> {
  /** Column definitions */
  columns: ColumnDef<TData>[];
  /** Server (fetch on action) or client (load once) data source */
  source: DataTableSource<TData>;
  /** Initial table state */
  initialState?: Omit<Partial<TableState>, "sorting"> & {
    sorting?: ExtendedColumnSort<TData>[];
  };
  getRowId?: (row: TData) => string;
  /**
   * When the row has no `id` field, use this column key as the row id (for selection and keys).
   * Ignored if `getRowId` is provided. Example: "userId", "email".
   */
  rowIdKey?: keyof TData | string;
  /** Show built-in select (checkbox) column for row selection. When true, actionBar is shown when any row is selected. */
  enableRowSelection?: boolean;
  /** Show toolbar with sort/filters */
  enableToolbar?: boolean;
  /** Use advanced filter UI (filter list + join operator) */
  enableAdvancedFilter?: boolean;
  /** Show pagination */
  enablePagination?: boolean;
  /** When true, show total row count to the left of the rows-per-page selector */
  showTotalRows?: boolean;
  /** Action bar when rows are selected. Can be a node or a render function receiving the table instance. */
  actionBar?: React.ReactNode | ((table: TanStackTable<TData>) => React.ReactNode);
  /** Callback when selection changes. Receives the selected row records. */
  onSelectionChange?: (selectedRows: TData[]) => void;
  /**
   * When true, pagination, sorting, and filters are kept in component state
   * instead of being synced to the page URL. Use for dialogs or embedded tables.
   */
  disableParamPersistence?: boolean;
  /**
   * Stretch the table body to fill remaining vertical space (e.g. fullscreen dialogs).
   * Centers loading, error, and empty states in the table area.
   */
  fillHeight?: boolean;
  /** Keep column headers fixed at the top while scrolling the table body */
  stickyHeader?: boolean;
  /** Called when an editable cell value is committed. Requires `meta.editable` on columns. */
  onCellEdit?: (event: CellEditEvent<TData>) => void;
  className?: string;
}

/**
 * Generic DataTable: pass columns + source (server or client). All logic lives here.
 *
 * @example Server (fetch on paginate/sort/filter):
 * ```tsx
 * <DataTable
 *   columns={getUsersTableColumns()}
 *   source={{
 *     type: "server",
 *     options: {
 *       fetch: async (state) => {
 *         const res = await getUsersList(toParams(state));
 *         return {
 *           data: res.data,
 *           pagination: res.pagination,
 *         };
 *       },
 *       queryKey: ["users"],
 *       initialPerPage: 10,
 *     },
 *   }}
 *   enableAdvancedFilter
 *   enableToolbar
 * />
 * ```
 *
 * @example Client (load once, filter/sort/paginate in browser):
 * ```tsx
 * const [rows, setRows] = useState<MyRow[]>([]);
 * useEffect(() => { fetchAll().then(setRows); }, []);
 * <DataTable
 *   columns={columns}
 *   source={{ type: "client", data: rows }}
 *   enableToolbar
 *   enablePagination
 * />
 * ```
 */
const SELECT_COLUMN_ID = "select";

function getSelectColumnDef<TData>(): ColumnDef<TData> {
  return {
    id: SELECT_COLUMN_ID,
    header: ({ table }) => (
      <Checkbox
        checked={
          table.getIsAllPageRowsSelected() ||
          (table.getIsSomePageRowsSelected() && "indeterminate")
        }
        onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
        aria-label="Select all"
        className="translate-y-0.5"
      />
    ),
    cell: ({ row }) => (
      <Checkbox
        checked={row.getIsSelected()}
        onCheckedChange={(value) => row.toggleSelected(!!value)}
        aria-label="Select row"
        className="translate-y-0.5"
      />
    ),
    enableSorting: false,
    enableHiding: false,
    size: 40,
  };
}

function DataTableWithLocalState<TData>(props: DataTableProps<TData>) {
  const listAllRecords =
    props.source.type === "server" && Boolean(props.source.options.listAllRecords);
  const initialPerPage = listAllRecords
    ? LIST_ALL_PER_PAGE
    : props.source.type === "server"
      ? (props.source.options.initialPerPage ?? 10)
      : (props.initialState?.pagination?.pageSize ?? 10);
  const localState = useCreateDataTableLocalState<TData>({
    initialSorting: props.initialState?.sorting,
    initialPerPage,
  });

  return (
    <DataTableLocalStateProvider value={localState}>
      <DataTableCore {...props} />
    </DataTableLocalStateProvider>
  );
}

export function DataTable<TData>(props: DataTableProps<TData>) {
  if (props.disableParamPersistence) {
    return <DataTableWithLocalState {...props} />;
  }

  return <DataTableCore {...props} />;
}

function DataTableCore<TData>({
  columns,
  source,
  initialState,
  getRowId: getRowIdProp,
  rowIdKey,
  enableRowSelection = false,
  enableToolbar = true,
  enableAdvancedFilter = false,
  enablePagination = true,
  showTotalRows = false,
  fillHeight = false,
  stickyHeader = false,
  actionBar,
  onSelectionChange,
  onCellEdit,
  className,
}: DataTableProps<TData>) {
  const isServer = source.type === "server";
  const serverOptions = isServer ? source.options : null;
  const listAllRecords = Boolean(isServer && serverOptions?.listAllRecords);
  const effectiveEnablePagination = enablePagination && !listAllRecords;

  const { enableAdvancedFilter: enableAdvancedFilterFlag } =
    useTableAdvancedOptions();

  const effectiveAdvancedFilter =
    enableAdvancedFilter && enableAdvancedFilterFlag;

  const getRowId = React.useMemo(() => {
    if (getRowIdProp) return getRowIdProp;
    if (rowIdKey != null) {
      const key = rowIdKey as string;
      return (row: TData) =>
        String((row as Record<string, unknown>)[key] ?? "");
    }
    return undefined;
  }, [getRowIdProp, rowIdKey]);

  const tableColumns = React.useMemo(() => {
    if (!enableRowSelection) return columns;
    const selectCol = getSelectColumnDef<TData>();
    return [selectCol, ...columns] as ColumnDef<TData>[];
  }, [enableRowSelection, columns]);

  const mergedInitialState = React.useMemo(
    () =>
      ({
        columnPinning: { right: ["actions"] },
        ...(listAllRecords
          ? { pagination: { pageSize: LIST_ALL_PER_PAGE } }
          : {}),
        ...initialState,
      }) as typeof initialState & { columnPinning: { right: string[] } },
    [initialState, listAllRecords],
  );

  const serverData = useServerTableData(
    isServer && serverOptions
      ? {
        ...serverOptions,
        columns: tableColumns,
        enableAdvancedFilter: effectiveAdvancedFilter,
        initialPerPage: listAllRecords
          ? LIST_ALL_PER_PAGE
          : (serverOptions.initialPerPage ?? 10),
        initialSorting:
          serverOptions.initialSorting ?? mergedInitialState?.sorting,
        listAllRecords,
        enabled: serverOptions.enabled ?? true,
      }
      : {
        fetch: async () => ({ data: [], pagination: { pageCount: 0 } }),
        queryKey: [],
        columns: tableColumns,
        enabled: false,
      }
  );

  const serverDataSource = isServer ? serverData.data : undefined;
  const clientData = source.type === "client" ? source.data : undefined;
  const data = isServer ? serverDataSource : clientData;
  const pageCount: number =
    isServer && serverData.pagination
      ? serverData.pagination.pageCount ?? 0
      : -1;

  const { table, shallow, debounceMs, throttleMs } = useDataTable({
    data: data ?? [],
    columns: tableColumns,
    pageCount: isServer ? pageCount : -1,
    serverSide: isServer,
    enableAdvancedFilter: effectiveAdvancedFilter,
    enableRowSelection,
    initialState: mergedInitialState,
    getRowId,
    clearOnDefault: true,
  });

  const isLoading = isServer && serverData.isLoading;
  const isError = isServer && serverData.isError;
  const fetchError = isServer ? serverData.error : null;
  const isFetching = isServer && serverData.isFetching;
  const columnCount = table.getAllColumns().length;

  const totalRows = isServer
    ? (serverData.pagination.total ?? 0)
    : table.getFilteredRowModel().rows.length;

  const errorMessage = React.useMemo(() => {
    if (!fetchError) return null;
    if (fetchError instanceof Error) return fetchError.message;
    const err = fetchError as { response?: { data?: { message?: string } }; message?: string };
    return err.response?.data?.message ?? err.message ?? "Failed to load data";
  }, [fetchError]);

  const retryingFromErrorRef = React.useRef(false);
  const lastErrorMessageRef = React.useRef<string | null>(null);
  const prevFetchingRef = React.useRef(isFetching);
  if (errorMessage) lastErrorMessageRef.current = errorMessage;
  if (isError) retryingFromErrorRef.current = true;
  React.useEffect(() => {
    const finishedFetching = prevFetchingRef.current && !isFetching;
    prevFetchingRef.current = isFetching;
    if (finishedFetching && !isError) {
      retryingFromErrorRef.current = false;
      lastErrorMessageRef.current = null;
    }
  }, [isError, isFetching]);
  const showErrorRow =
    (isError && !!errorMessage) ||
    (retryingFromErrorRef.current && isFetching);
  const displayedErrorMessage =
    errorMessage || lastErrorMessageRef.current || "Failed to load data";

  const rowSelection = table.getState().rowSelection;
  React.useEffect(() => {
    if (!onSelectionChange) return;
    const selectedRows = table.getSelectedRowModel().rows.map(
      (row) => row.original
    );
    onSelectionChange(selectedRows);
  }, [rowSelection, onSelectionChange, table]);

  const clearColumnFilters = isServer ? serverData.clearColumnFilters : undefined;

  const isStickyHeader = stickyHeader || fillHeight;
  const rowCount = table.getRowModel().rows.length;
  const showEmptyOverlay =
    fillHeight && !isLoading && !showErrorRow && rowCount === 0;

  const toolbar = enableToolbar && (
    effectiveAdvancedFilter ? (
      <DataTableAdvancedToolbar
        table={table}
        clearColumnFilters={clearColumnFilters}
        withoutAdvancedOptions={!enableAdvancedFilter}
      >
        <DataTableSortList table={table} align="start" />
        <DataTableFilterList
          table={table}
          shallow={shallow}
          debounceMs={debounceMs}
          throttleMs={throttleMs}
          align="start"
        />
      </DataTableAdvancedToolbar>
    ) : (
      <DataTableToolbar
        table={table}
        clearColumnFilters={clearColumnFilters}
        withoutAdvancedOptions={!enableAdvancedFilter}
      >
        <DataTableSortList table={table} align="end" />
      </DataTableToolbar>
    )
  );

  return (
    <div
      className={cn(
        "flex w-full flex-col gap-2.5",
        fillHeight ? "min-h-0 flex-1 overflow-hidden" : "overflow-auto",
        className,
      )}
    >
      {toolbar ? (
        <div className={cn(fillHeight && "shrink-0")}>{toolbar}</div>
      ) : null}
      <div
        className={cn(
          "rounded-md border",
          fillHeight
            ? "relative flex min-h-0 flex-1 flex-col overflow-hidden"
            : "overflow-x-auto",
        )}
      >
        {fillHeight && isLoading ? (
          <div className="pointer-events-none absolute inset-0 z-[1] flex items-center justify-center bg-background/80 text-muted-foreground text-sm">
            <Loader2 className="mr-2 size-4 animate-spin" aria-hidden />
            Loading...
          </div>
        ) : null}
        {fillHeight && showErrorRow ? (
          <div className="absolute inset-0 z-[1] flex items-center justify-center bg-background/80 px-4 text-center text-destructive text-sm">
            <span className="font-medium">{displayedErrorMessage}</span>
            {isServer && serverData.refetch ? (
              <span className="ml-1 inline-flex items-center gap-1.5">
                <span className="text-muted-foreground">·</span>
                {isFetching ? (
                  <Loader2
                    className="size-4 shrink-0 animate-spin text-muted-foreground"
                    aria-hidden
                  />
                ) : null}
                <button
                  type="button"
                  disabled={isFetching}
                  onClick={() => serverData.refetch()}
                  className="underline underline-offset-2 hover:no-underline disabled:opacity-50 disabled:no-underline"
                >
                  Retry
                </button>
              </span>
            ) : null}
          </div>
        ) : null}
        {showEmptyOverlay ? (
          <div className="pointer-events-none absolute inset-0 z-[1] flex items-center justify-center text-muted-foreground text-sm">
            No results.
          </div>
        ) : null}
        <div
          className={cn(
            (fillHeight || stickyHeader) && "min-h-0 flex-1 overflow-auto",
            !fillHeight && stickyHeader && "max-h-[min(70vh,48rem)]",
          )}
        >
          <Table containerClassName={isStickyHeader ? "overflow-visible" : undefined}>
            <TableHeader>
              {table.getHeaderGroups().map((headerGroup) => (
                <TableRow key={headerGroup.id} className={isStickyHeader ? "hover:bg-transparent" : undefined}>
                  {headerGroup.headers.map((header) => {
                    const isPinned = header.column.getIsPinned();
                    const pinningStyle = isPinned
                      ? getCommonPinningStyles({
                        column: header.column,
                        withBorder: true,
                        layer: "header",
                      })
                      : {};
                    return (
                      <TableHead
                        key={header.id}
                        colSpan={header.colSpan}
                        style={{
                          ...pinningStyle,
                          ...(isStickyHeader && !isPinned
                            ? {
                              top: 0,
                              zIndex: 20,
                            }
                            : isStickyHeader && isPinned
                              ? {
                                top: 0,
                              }
                              : {}),
                        }}
                        className={cn(
                          header.column.columnDef.meta?.className,
                          isStickyHeader &&
                          "sticky bg-background shadow-[inset_0_-1px_0_hsl(var(--border))]",
                        )}
                      >
                        {header.isPlaceholder
                          ? null
                          : flexRender(
                            header.column.columnDef.header,
                            header.getContext(),
                          )}
                      </TableHead>
                    );
                  })}
                </TableRow>
              ))}
            </TableHeader>
            <TableBody>
              {!fillHeight && showErrorRow ? (
                <TableRow>
                  <TableCell
                    colSpan={columnCount}
                    className="h-24 text-center text-destructive"
                  >
                    <span className="font-medium">{displayedErrorMessage}</span>
                    {isServer && serverData.refetch && (
                      <span className="ml-1 inline-flex items-center gap-1.5">
                        <span className="text-muted-foreground">·</span>
                        {isFetching && (
                          <Loader2
                            className="size-4 shrink-0 animate-spin text-muted-foreground"
                            aria-hidden
                          />
                        )}
                        <button
                          type="button"
                          disabled={isFetching}
                          onClick={() => serverData.refetch()}
                          className="underline underline-offset-2 hover:no-underline disabled:opacity-50 disabled:no-underline"
                        >
                          Retry
                        </button>
                      </span>
                    )}
                  </TableCell>
                </TableRow>
              ) : !fillHeight && isLoading ? (
                <TableRow>
                  <TableCell
                    colSpan={columnCount}
                    className="h-24 text-center text-muted-foreground"
                  >
                    Loading...
                  </TableCell>
                </TableRow>
              ) : rowCount > 0 ? (
                table.getRowModel().rows.map((row) => (
                  <TableRow
                    key={row.id}
                    data-state={row.getIsSelected() && "selected"}
                  >
                    {row.getVisibleCells().map((cell) => {
                      const editorConfig =
                        onCellEdit &&
                          isColumnEditable(cell.column.id, cell.column.columnDef.meta)
                          ? resolveCellEditorConfig(cell.column.columnDef.meta)
                          : null;

                      return (
                        <TableCell
                          key={cell.id}
                          style={getCommonPinningStyles({
                            column: cell.column,
                            withBorder:
                              cell.column.getIsPinned() === "right" ||
                              cell.column.getIsPinned() === "left",
                          })}
                          className={cn(
                            cell.column.columnDef.meta?.className,
                            cell.column.id === "actions" && "relative bg-background",
                            editorConfig && "p-0",
                          )}
                        >
                          {editorConfig ? (
                            <DataTableEditableCell
                              cell={cell}
                              editor={editorConfig}
                              displayContent={flexRender(
                                cell.column.columnDef.cell,
                                cell.getContext(),
                              )}
                              onCommit={(value) =>
                                onCellEdit?.({
                                  rowId: row.id,
                                  columnId: cell.column.id,
                                  value,
                                  previousValue: cell.getValue(),
                                  row: row.original,
                                })
                              }
                            />
                          ) : (
                            flexRender(
                              cell.column.columnDef.cell,
                              cell.getContext(),
                            )
                          )}
                        </TableCell>
                      );
                    })}
                  </TableRow>
                ))
              ) : !fillHeight ? (
                <TableRow>
                  <TableCell
                    colSpan={columnCount}
                    className="h-24 text-center"
                  >
                    No results.
                  </TableCell>
                </TableRow>
              ) : (
                <TableRow className="hover:bg-transparent">
                  <TableCell
                    colSpan={columnCount}
                    className="h-px border-0 p-0"
                    aria-hidden
                  />
                </TableRow>
              )}
            </TableBody>
          </Table>
        </div>
      </div>
      <div className={cn("flex flex-col gap-2.5", fillHeight && "shrink-0")}>
        {effectiveEnablePagination && (
          <DataTablePagination
            table={table}
            isFetching={isServer && serverData.isFetching}
            serverSide={isServer}
            showTotalRows={showTotalRows}
            totalRows={totalRows}
          />
        )}
        {actionBar &&
          table.getSelectedRowModel().rows.length > 0 &&
          (typeof actionBar === "function" ? actionBar(table) : actionBar)}
      </div>
    </div>
  );
}

// --- Subcomponents (for composition)
export { DataTableContent } from "./components/data-table";
export { DataTableAdvancedToolbar } from "./components/data-table-advanced-toolbar";
export { DataTableToolbar } from "./components/data-table-toolbar";
export { DataTableSortList } from "./components/data-table-sort-list";
export { DataTableFilterList } from "./components/data-table-filter-list";
export { DataTableFilterMenu } from "./components/data-table-filter-menu";
export { DataTablePagination } from "./components/data-table-pagination";
export { DataTableColumnHeader } from "./components/data-table-column-header";
export { DataTableEditableCell } from "./components/data-table-editable-cell";
export { DataTableSkeleton } from "./components/data-table-skeleton";
export {
  DataTableActionBar,
  DataTableActionBarAction,
  DataTableActionBarSelection,
} from "./components/data-table-action-bar";

// --- Hooks
export { useDataTable } from "./hooks/use-datatable";
export { useServerTableData } from "./hooks/use-server-table-data";
export { useEditableTableData } from "./hooks/use-editable-table-data";
export { useBackendColumns } from "./hooks/use-backend-columns";
export type { UseBackendColumnsOptions } from "./hooks/use-backend-columns";

// --- Lib (filter helpers, converter)
export { getValidFilters } from "./lib/data-table";
export { convertBackendColumns } from "./lib/column-converter";
export { hasValidFilterVariant } from "./lib/filter-variants";

// --- Types
export type {
  ServerTableState,
  ServerPaginationMeta,
  ServerFetchResult,
  UseServerTableDataOptions,
  ExtendedColumnSort,
  ExtendedColumnFilter,
  BackendColumnDefinition,
  BackendColumnsResponse,
  BackendColumnMeta,
  BackendColumnOption,
  CellEditEvent,
  CellEditorConfig,
  CellEditorType,
} from "./types/data-table";
