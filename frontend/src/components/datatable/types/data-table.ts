import type { AdvancedFilterEntry } from "../lib/advanced-filter-types";
import type { FilterItemSchema } from "../lib/filter-schema";
import type { JoinOperator, FilterVariant } from "./table-operators";
import type { ExtendedColumnSort } from "./table-sort";
import type { Row, RowData } from "@tanstack/react-table";

export type { AdvancedFilterEntry };
export type { FilterOperator, FilterVariant, JoinOperator } from "./table-operators";
export type { ExtendedColumnSort } from "./table-sort";

declare module "@tanstack/react-table" {
    interface ColumnMeta<_TData extends RowData, _TValue> {
        label?: string;
        placeholder?: string;
        variant?: FilterVariant;
        options?: Option[];
        range?: [number, number];
        unit?: string;
        icon?: React.FC<React.SVGProps<SVGSVGElement>>;
        /** When true, this column only appears in Advanced Filters, not in simple column filters. */
        enableOnlyAdvancedFilters?: boolean;
        /** When true, hide this column's filter input from the simple toolbar (still usable in advanced filters). */
        hidden?: boolean;
        /** When true, render the filter input in a disabled/read-only state. */
        disabled?: boolean;
        /**
         * Declares that this column's options depend on another column's filter value.
         * When the referenced column's filter changes, the columns endpoint is refetched
         * with that filter value as a query parameter so the backend can recompute Options.
         */
        filterBy?: string;
        /** Applied to header and body cells (e.g. `!whitespace-normal` for wrapping text). */
        className?: string;
        /** Optional fixed column width in px (used by legacy/simple tables). */
        width?: number;
        /** Cell text alignment hint for consumers. */
        align?: "left" | "center" | "right" | "start" | "end";
        /**
         * Enable inline cell editing. Set `true` for a text input, or pass a config object.
         * Requires `onCellEdit` on `<DataTable />`.
         */
        editable?: boolean | CellEditorConfig;
    }
}

export type CellEditorType = "text" | "number" | "select" | "checkbox";

export interface CellEditorConfig {
    type?: CellEditorType;
    /** Options for `select` editor */
    options?: Option[];
    placeholder?: string;
}

export interface CellEditEvent<TData> {
    rowId: string;
    columnId: string;
    value: unknown;
    previousValue: unknown;
    row: TData;
}

export interface Option {
    label: string;
    value: string;
    count?: number;
    icon?: React.FC<React.SVGProps<SVGSVGElement>>;
}

export interface ExtendedColumnFilter<TData> extends FilterItemSchema {
    id: Extract<keyof TData, string>;
}

export interface DataTableRowAction<TData> {
    row: Row<TData>;
    variant: "quickEdit" | "design" | "view" | "delete" | "disable" | "enable" | "archive";
}

/** Single column filter sent to server (id, value, and optional variant from column meta) */
export interface ServerColumnFilter {
    id: string;
    value: unknown;
    variant?: FilterVariant;
}

/** State passed to server fetch (pagination, sort, filters) */
export interface ServerTableState<TData = unknown> {
    /** page is 1-based; perPage is page size */
    pagination: { page: number; perPage: number };
    sorting: ExtendedColumnSort<TData>[];
    columnFilters: ServerColumnFilter[];
    /** Advanced filter mode: filters + joinOperator */
    filters?: AdvancedFilterEntry<TData>[];
    joinOperator?: JoinOperator;
}

/** Pagination info returned from server fetch */
export interface ServerPaginationMeta {
    pageCount: number;
    total?: number;
    page?: number;
    perPage?: number;
}

/** Minimum list fetch shape; APIs may include extra fields (exposed via `useServerTableData().fetchResult`). */
export type ServerFetchResult<TData> = {
    data: TData[];
    pagination: ServerPaginationMeta;
};

export interface UseServerTableDataOptions<TData> {
    /** Fetch data for the current table state. Called on state/URL change. */
    fetch: (state: ServerTableState<TData>) => Promise<ServerFetchResult<TData>>;
    /** Base key (array of strings) or function that receives state and returns a query key so refetch runs when params change. */
    queryKey: string[] | ((state: ServerTableState<TData>) => unknown[]);
    /** Default rows per page (URL key: perPage). Prefer over initialPageSize. */
    initialPerPage?: number;
    /** Default sort (URL key: sort). Applied to the first server fetch when the URL has no sort param. */
    initialSorting?: ExtendedColumnSort<TData>[];
    /**
     * When true, fetches all rows in one request (`pagination[perPage]=-1`).
     * Pagination UI is hidden unless `enablePagination` is explicitly true.
     */
    listAllRecords?: boolean;
    /** Use advanced filters (filters + joinOperator) from URL */
    enableAdvancedFilter?: boolean;
    /** Column definitions; used to build filter/sort parsers for URL state */
    columnIds?: Set<string>;
    /** When false, query is not run (e.g. client-side mode) */
    enabled?: boolean;
}

/** Column definition returned by backend GET /columns (flowtrove-compatible shape) */
export interface BackendColumnOption {
    label: string;
    value: string;
}

export interface BackendColumnMeta {
    label?: string;
    variant?: string;
    placeholder?: string;
    options?: BackendColumnOption[];
    /** When true, hide this column's filter input from the simple toolbar. */
    hidden?: boolean;
    /** When true, render the filter input in a disabled/read-only state. */
    disabled?: boolean;
    /**
     * When set, declares that this column's options depend on another column's filter value.
     * The columns endpoint is refetched (with the referenced filter value as a query param)
     * whenever that filter changes.
     */
    filterBy?: string;
}

export interface BackendColumnDefinition {
    id: string;
    accessorKey: string;
    header: string;
    enableSorting?: boolean;
    enableColumnFilter?: boolean;
    /** When true, column is filterable only in Advanced Filters, not in simple toolbar. */
    enableOnlyAdvancedFilters?: boolean;
    size?: number;
    meta?: BackendColumnMeta;
}

export interface BackendColumnsResponse {
    columns: BackendColumnDefinition[];
}

// Keep DataTableConfig available for consumers importing filter-related types from this module.
export type { DataTableConfig } from "../config/data-table";
