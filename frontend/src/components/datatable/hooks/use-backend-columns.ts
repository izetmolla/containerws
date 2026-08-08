"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { useQuery } from "@tanstack/react-query";
import {
  parseAsArrayOf,
  parseAsString,
  useQueryStates,
} from "nuqs";
import * as React from "react";

import { ARRAY_SEPARATOR } from "../lib/constants";
import { convertBackendColumns } from "../lib/column-converter";
import { useOptionalDataTableLocalState } from "../context/data-table-local-state";
import type {
  BackendColumnDefinition,
  BackendColumnsResponse,
} from "../types/data-table";

const EMPTY_COLUMN_VISIBILITY: Record<string, boolean> = {};
const EMPTY_PARSERS: Record<
  string,
  ReturnType<typeof parseAsArrayOf<string>>
> = {};
const EMPTY_BACKEND_COLUMNS: BackendColumnDefinition[] = [];

/**
 * Filter values forwarded to fetchColumns when columns declare `meta.filterBy`.
 * Each value is a comma-joined string so it travels through axios's default param
 * serializer as a single query string entry (e.g. `?study_level=master,bachelor`),
 * which is what the Go-side `fiberv3query.GetFilter` expects.
 */
export type BackendColumnFilterValues = Record<string, string>;

export interface UseBackendColumnsOptions<TData = Record<string, unknown>> {
  /**
   * Fetch column definitions from the backend.
   *
   * `filters` carries the current URL filter state for any column ids that another
   * column references via `meta.filterBy` (e.g. `{ study_level: ["master"] }`). Forward
   * these as query params so the backend can recompute dependent column options.
   */
  fetchColumns: (
    filters?: BackendColumnFilterValues
  ) => Promise<
    BackendColumnsResponse | { columns: BackendColumnDefinition[] }
  >;
  queryKey: unknown[];
  /** Columns to prepend (e.g. select checkbox). */
  prependColumns?: ColumnDef<TData>[];
  /** Columns to append (e.g. actions). */
  appendColumns?: ColumnDef<TData>[];
  /**
   * Override backend column properties by column id.
   * Array of objects with required `id` (backend column id) and any column properties to merge.
   */
  overrideColumns?: Array<{ id: string } & Partial<ColumnDef<TData>>>;
  /** When false, the query is not run. */
  enabled?: boolean;
}

/**
 * Fetches column definitions from the backend and converts them to TanStack ColumnDef.
 * Use with DataTable: columns={columns} while isLoading show a skeleton.
 *
 * If any returned column declares `meta.filterBy` (a column id whose filter value the
 * column's options depend on), this hook subscribes to that column's URL filter state
 * and refetches the columns endpoint whenever it changes, forwarding the value as
 * `filters` to `fetchColumns`. Backends should pass these through as query params and
 * use them to compute the dependent column's `Options`.
 */
export function useBackendColumns<TData = Record<string, unknown>>(
  options: UseBackendColumnsOptions<TData>
): {
  columns: ColumnDef<TData>[];
  isLoading: boolean;
  isError: boolean;
  error: Error | null;
  refetch: () => void;
  columnVisibility: Record<string, boolean>;
} {
  const {
    fetchColumns,
    queryKey,
    prependColumns,
    appendColumns,
    overrideColumns,
    enabled = true,
  } = options;

  // ---- Pass 1: fetch columns ignoring filterBy values. The result tells us
  // which column ids are referenced via `meta.filterBy`. We deliberately keep
  // this query keyed only by the caller's queryKey so its identity is rock
  // solid and doesn't churn on every render.
  const baseQuery = useQuery({
    queryKey: [...queryKey, "__base"],
    queryFn: async () => {
      const result = await fetchColumns(undefined);
      const columns =
        "columns" in result
          ? result.columns
          : (result as BackendColumnsResponse).columns;
      const list = Array.isArray(columns) ? columns : [];
      const columnVisibility =
        "columnVisibility" in result
          ? result.columnVisibility
          : list.reduce<Record<string, boolean>>((acc, c) => {
              acc[c.id] = true;
              return acc;
            }, {});
      return { columns: list, columnVisibility };
    },
    enabled,
  });

  const baseColumns = (baseQuery.data?.columns ??
    EMPTY_BACKEND_COLUMNS) as BackendColumnDefinition[];

  // ---- Discover filterBy keys purely from the base response. Reduced to a
  // primitive string token so downstream memos depend on a stable value.
  const filterByKeysToken = React.useMemo(() => {
    const seen = new Set<string>();
    for (const c of baseColumns) {
      const k = c.meta?.filterBy;
      if (typeof k === "string" && k.length > 0) seen.add(k);
    }
    return Array.from(seen).sort().join(",");
  }, [baseColumns]);

  const filterByKeys = React.useMemo(
    () => (filterByKeysToken ? filterByKeysToken.split(",") : []),
    [filterByKeysToken]
  );

  const filterByParsers = React.useMemo(() => {
    if (filterByKeys.length === 0) return EMPTY_PARSERS;
    const out: Record<
      string,
      ReturnType<typeof parseAsArrayOf<string>>
    > = {};
    for (const k of filterByKeys) {
      out[k] = parseAsArrayOf(parseAsString, ARRAY_SEPARATOR);
    }
    return out;
  }, [filterByKeys]);

  const [urlFilterByValues] = useQueryStates(filterByParsers);
  const localState = useOptionalDataTableLocalState();
  const filterByValues = localState?.filterValues ?? urlFilterByValues;

  // Stable string signature of the *active* filterBy values, so the dependent
  // query only re-runs when something meaningful actually changed.
  const filterBySignature = React.useMemo(() => {
    if (filterByKeys.length === 0) return "";
    const parts: string[] = [];
    for (const key of filterByKeys) {
      const v = (filterByValues as Record<
        string,
        string[] | null | undefined
      >)[key];
      if (Array.isArray(v) && v.length > 0) {
        parts.push(`${key}=${v.join(ARRAY_SEPARATOR)}`);
      }
    }
    return parts.join("&");
  }, [filterByKeys, filterByValues]);

  // ---- Pass 2: only fires when there ARE filterBy keys AND there is at least
  // one active value. Otherwise we fall back to baseQuery's result.
  const dependentEnabled =
    enabled && filterByKeys.length > 0 && filterBySignature.length > 0;

  const dependentQuery = useQuery({
    queryKey: [...queryKey, "__dep", filterBySignature],
    queryFn: async () => {
      const filtersArg: BackendColumnFilterValues = {};
      for (const key of filterByKeys) {
        const v = (filterByValues as Record<
          string,
          string[] | null | undefined
        >)[key];
        if (Array.isArray(v) && v.length > 0) {
          filtersArg[key] = v.join(ARRAY_SEPARATOR);
        }
      }
      const result = await fetchColumns(
        Object.keys(filtersArg).length > 0 ? filtersArg : undefined
      );
      const columns =
        "columns" in result
          ? result.columns
          : (result as BackendColumnsResponse).columns;
      const list = Array.isArray(columns) ? columns : [];
      const columnVisibility =
        "columnVisibility" in result
          ? result.columnVisibility
          : list.reduce<Record<string, boolean>>((acc, c) => {
              acc[c.id] = true;
              return acc;
            }, {});
      return { columns: list, columnVisibility };
    },
    enabled: dependentEnabled,
  });

  // Prefer dependent data when it has resolved at least once, otherwise fall
  // back to the base data so the table doesn't blank out while a refetch runs.
  const effectiveData =
    dependentEnabled && dependentQuery.data
      ? dependentQuery.data
      : baseQuery.data;
  const backendColumns = (effectiveData?.columns ??
    EMPTY_BACKEND_COLUMNS) as BackendColumnDefinition[];

  const converted = React.useMemo(
    () => convertBackendColumns(backendColumns) as ColumnDef<TData>[],
    [backendColumns]
  );

  const overrideById = React.useMemo(() => {
    if (!overrideColumns?.length) return null;
    return overrideColumns.reduce<
      Record<string, { id: string } & Partial<ColumnDef<TData>>>
    >((acc, item) => {
      acc[item.id] = item;
      return acc;
    }, {});
  }, [overrideColumns]);

  const columns = React.useMemo<ColumnDef<TData>[]>(() => {
    const merged: ColumnDef<TData>[] = overrideById
      ? converted.map((col) => {
          const id =
            col.id ??
            (col as { accessorKey?: string }).accessorKey ??
            "";
          const override = id ? overrideById[id] : undefined;
          if (!override) return col;
          const { id: _overrideId, meta: overrideMeta, ...restOverride } =
            override;
          const mergedMeta = overrideMeta
            ? { ...col.meta, ...overrideMeta }
            : col.meta;
          return {
            ...col,
            ...restOverride,
            meta: mergedMeta,
          } as ColumnDef<TData>;
        })
      : converted;
    return [
      ...(prependColumns ?? []),
      ...merged,
      ...(appendColumns ?? []),
    ];
  }, [prependColumns, converted, appendColumns, overrideById]);

  const columnVisibility = React.useMemo(
    () =>
      (effectiveData?.columnVisibility ?? EMPTY_COLUMN_VISIBILITY) as Record<
        string,
        boolean
      >,
    [effectiveData?.columnVisibility]
  );

  // Treat the hook as loading only when the *base* fetch hasn't resolved yet.
  // A dependent refetch happens in the background and shouldn't block the page.
  return {
    columns,
    isLoading: baseQuery.isLoading,
    isError: baseQuery.isError || dependentQuery.isError,
    error: (baseQuery.error ?? dependentQuery.error) as Error | null,
    refetch: dependentEnabled ? dependentQuery.refetch : baseQuery.refetch,
    columnVisibility,
  };
}
