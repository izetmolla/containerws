"use client";

import {
  type ColumnFiltersState,
  type PaginationState,
  type RowSelectionState,
  type SortingState,
  type TableOptions,
  type TableState,
  type Updater,
  type VisibilityState,
  getCoreRowModel,
  getFacetedMinMaxValues,
  getFacetedRowModel,
  getFacetedUniqueValues,
  getFilteredRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  useReactTable,
} from "@tanstack/react-table";
import {
  type Parser,
  type UseQueryStateOptions,
  parseAsInteger,
  useQueryState,
  useQueryStates,
} from "nuqs";
import * as React from "react";

import { useOptionalDataTableLocalState } from "../context/data-table-local-state";
import { getColumnFilterParser } from "../lib/column-filter-parsers";
import {
  DEBOUNCE_MS_DEFAULT,
  QUERY_KEYS,
  THROTTLE_MS_DEFAULT,
} from "../lib/constants";
import { getSortingStateParser } from "../lib/parsers";
import type { ExtendedColumnSort } from "../types/data-table";
import { useDebouncedCallback } from "./use-debounced-callback";

const coreRowModel = getCoreRowModel();
const filteredRowModel = getFilteredRowModel();
const paginationRowModel = getPaginationRowModel();
const sortedRowModel = getSortedRowModel();
const facetedRowModel = getFacetedRowModel();
const facetedUniqueValues = getFacetedUniqueValues();
const facetedMinMaxValues = getFacetedMinMaxValues();

const CLIENT_ROW_MODELS = {
  getFilteredRowModel: filteredRowModel,
  getPaginationRowModel: paginationRowModel,
  getSortedRowModel: sortedRowModel,
  getFacetedRowModel: facetedRowModel,
  getFacetedUniqueValues: facetedUniqueValues,
  getFacetedMinMaxValues: facetedMinMaxValues,
} as const;

const DEFAULT_TABLE_COLUMN = { enableColumnFilter: false } as const;

interface UseDataTableProps<TData>
  extends Omit<
      TableOptions<TData>,
      | "state"
      | "pageCount"
      | "getCoreRowModel"
      | "manualFiltering"
      | "manualPagination"
      | "manualSorting"
    > {
  pageCount?: number;
  /** When false, table does client-side pagination/sort/filter (data loaded once). When true, server handles it. */
  serverSide?: boolean;
  initialState?: Omit<Partial<TableState>, "sorting"> & {
    sorting?: ExtendedColumnSort<TData>[];
  };
  history?: "push" | "replace";
  debounceMs?: number;
  throttleMs?: number;
  clearOnDefault?: boolean;
  enableAdvancedFilter?: boolean;
  /** When true, table tracks row selection (e.g. for built-in select column). */
  enableRowSelection?: boolean;
  scroll?: boolean;
  shallow?: boolean;
  startTransition?: React.TransitionStartFunction;
}

/**
 * Binds TanStack Table to URL state (pagination, sort, filters) via nuqs.
 * Use with server-side data (manual*: true) or client-side (manual*: false).
 */
export function useDataTable<TData>(props: UseDataTableProps<TData>) {
  const localState = useOptionalDataTableLocalState<TData>();
  const {
    columns,
    pageCount = -1,
    serverSide = true,
    initialState,
    history = "replace",
    debounceMs = DEBOUNCE_MS_DEFAULT,
    throttleMs = THROTTLE_MS_DEFAULT,
    clearOnDefault = false,
    enableAdvancedFilter = false,
    enableRowSelection = true,
    scroll = false,
    shallow = true,
    startTransition,
    ...tableProps
  } = props;

  const queryStateOptions = React.useMemo<
    Omit<UseQueryStateOptions<string>, "parse">
  >(
    () => ({
      history,
      scroll,
      shallow,
      throttleMs,
      debounceMs,
      clearOnDefault,
      startTransition,
    }),
    [
      history,
      scroll,
      shallow,
      throttleMs,
      debounceMs,
      clearOnDefault,
      startTransition,
    ],
  );

  const [rowSelection, setRowSelection] = React.useState<RowSelectionState>(
    initialState?.rowSelection ?? {},
  );
  const [columnVisibility, setColumnVisibility] =
    React.useState<VisibilityState>(initialState?.columnVisibility ?? {});

  const [urlPage, setUrlPage] = useQueryState(
    QUERY_KEYS.PAGE,
    parseAsInteger.withOptions(queryStateOptions).withDefault(1),
  );
  const [urlPerPage, setUrlPerPage] = useQueryState(
    QUERY_KEYS.PER_PAGE,
    parseAsInteger
      .withOptions(queryStateOptions)
      .withDefault(initialState?.pagination?.pageSize ?? 10),
  );
  const page = localState?.page ?? urlPage;
  const setPage = localState?.setPage ?? setUrlPage;
  const perPage = localState?.perPage ?? urlPerPage;
  const setPerPage = localState?.setPerPage ?? setUrlPerPage;

  const pagination: PaginationState = React.useMemo(() => {
    return {
      pageIndex: page - 1, // zero-based index -> one-based index
      pageSize: perPage,
    };
  }, [page, perPage]);

  const onPaginationChange = React.useCallback(
    (updaterOrValue: Updater<PaginationState>) => {
      if (typeof updaterOrValue === "function") {
        const newPagination = updaterOrValue(pagination);
        void setPage(newPagination.pageIndex + 1);
        void setPerPage(newPagination.pageSize);
      } else {
        void setPage(updaterOrValue.pageIndex + 1);
        void setPerPage(updaterOrValue.pageSize);
      }
    },
    [pagination, setPage, setPerPage],
  );

  const columnIds = React.useMemo(() => {
    return new Set(
      columns.map((column) => column.id).filter(Boolean) as string[],
    );
  }, [columns]);

  const [urlSorting, setUrlSorting] = useQueryState(
    QUERY_KEYS.SORT,
    getSortingStateParser<TData>(columnIds)
      .withOptions(queryStateOptions)
      .withDefault(initialState?.sorting ?? []),
  );
  const sorting = localState?.sorting ?? urlSorting;
  const setSorting = localState?.setSorting ?? setUrlSorting;

  const onSortingChange = React.useCallback(
    (updaterOrValue: Updater<SortingState>) => {
      if (typeof updaterOrValue === "function") {
        const newSorting = updaterOrValue(sorting);
        setSorting(newSorting as ExtendedColumnSort<TData>[]);
      } else {
        setSorting(updaterOrValue as ExtendedColumnSort<TData>[]);
      }
    },
    [sorting, setSorting],
  );

  const filterableColumns = React.useMemo(() => {
    if (enableAdvancedFilter) return [];

    return columns.filter(
      (column) =>
        column.enableColumnFilter &&
        !column.meta?.enableOnlyAdvancedFilters
    );
  }, [columns, enableAdvancedFilter]);

  const filterParsers = React.useMemo(() => {
    if (enableAdvancedFilter) return {};

    return filterableColumns.reduce<
      Record<string, Parser<string> | Parser<string[]>>
    >((acc, column) => {
      acc[column.id ?? ""] = getColumnFilterParser(column, queryStateOptions);
      return acc;
    }, {});
  }, [filterableColumns, queryStateOptions, enableAdvancedFilter]);

  const [urlFilterValues, setUrlFilterValues] = useQueryStates(filterParsers);
  const filterValues = localState?.filterValues ?? urlFilterValues;
  const setFilterValues = localState?.setFilterValues ?? setUrlFilterValues;

  const debouncedSetFilterValues = useDebouncedCallback(
    (values: Record<string, string | string[] | null>) => {
      setPage(1);
      setFilterValues(values);
    },
    debounceMs,
  );

  const initialColumnFilters: ColumnFiltersState = React.useMemo(() => {
    if (enableAdvancedFilter) return [];

    return Object.entries(filterValues).reduce<ColumnFiltersState>(
      (filters, [key, value]) => {
        if (value == null || value === "") return filters;

        const column = filterableColumns.find((col) => col.id === key);
        const processedValue = Array.isArray(value)
          ? value
          : column?.meta?.options
            ? [value]
            : value;

        filters.push({
          id: key,
          value: processedValue,
        });
        return filters;
      },
      [],
    );
  }, [filterValues, enableAdvancedFilter, filterableColumns]);

  const [columnFilters, setColumnFilters] =
    React.useState<ColumnFiltersState>(initialColumnFilters);

  const onColumnFiltersChange = React.useCallback(
    (updaterOrValue: Updater<ColumnFiltersState>) => {
      if (enableAdvancedFilter) return;

      setColumnFilters((prev) => {
        const next =
          typeof updaterOrValue === "function"
            ? updaterOrValue(prev)
            : updaterOrValue;

        const filterUpdates = next.reduce<
          Record<string, string | string[] | null>
        >((acc, filter) => {
          if (filterableColumns.find((column) => column.id === filter.id)) {
            acc[filter.id] = filter.value as string | string[];
          }
          return acc;
        }, {});

        for (const prevFilter of prev) {
          if (!next.some((filter) => filter.id === prevFilter.id)) {
            filterUpdates[prevFilter.id] = null;
          }
        }

        debouncedSetFilterValues(filterUpdates);
        return next;
      });
    },
    [debouncedSetFilterValues, filterableColumns, enableAdvancedFilter],
  );

  const table = useReactTable({
    ...tableProps,
    columns,
    initialState,
    pageCount: serverSide ? pageCount : undefined,
    state: {
      pagination,
      sorting,
      columnVisibility,
      rowSelection,
      columnFilters,
    },
    defaultColumn: {
      ...tableProps.defaultColumn,
      ...DEFAULT_TABLE_COLUMN,
    },
    enableRowSelection,
    onRowSelectionChange: setRowSelection,
    onPaginationChange,
    onSortingChange,
    onColumnFiltersChange,
    onColumnVisibilityChange: setColumnVisibility,
    getCoreRowModel: coreRowModel,
    ...(serverSide ? {} : (CLIENT_ROW_MODELS as Partial<TableOptions<TData>>)),
    manualPagination: serverSide,
    manualSorting: serverSide,
    manualFiltering: serverSide,
  });

  return { table, shallow, debounceMs, throttleMs };
}
