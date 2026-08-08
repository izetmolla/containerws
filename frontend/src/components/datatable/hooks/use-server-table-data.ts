"use client";

import { useQuery } from "@tanstack/react-query";
import type { ColumnDef } from "@tanstack/react-table";
import type { Parser } from "nuqs";
import {
  parseAsInteger,
  parseAsStringEnum,
  useQueryState,
  useQueryStates,
} from "nuqs";
import * as React from "react";

import { dataTableConfig } from "../config/data-table";
import { useOptionalDataTableLocalState } from "../context/data-table-local-state";
import { useAdvancedFiltersQueryState } from "../hooks/use-advanced-filters-query-state";
import { getColumnFilterParser } from "../lib/column-filter-parsers";
import { LIST_ALL_PER_PAGE, QUERY_KEYS } from "../lib/constants";
import { getSortingStateParser } from "../lib/parsers";
import { getValidAdvancedFilterEntries } from "../lib/data-table";
import {
  normalizeAdvancedFilterJoinOperators,
} from "../lib/advanced-filters";
import type { AdvancedFilterEntry } from "../lib/advanced-filter-types";
import { serializeServerTableParams } from "../lib/serialize-server-table-params";
import type {
  ExtendedColumnSort,
  FilterVariant,
  JoinOperator,
  ServerColumnFilter,
  ServerPaginationMeta,
  ServerTableState,
  UseServerTableDataOptions,
} from "../types/data-table";

type ColumnWithFilter = ColumnDef<unknown> & {
  id?: string;
  enableColumnFilter?: boolean;
  meta?: { options?: unknown[]; variant?: FilterVariant };
};

export function useServerTableData<TData>(
  options: UseServerTableDataOptions<TData> & {
    columns: ColumnDef<TData>[];
  }
) {
  const {
    fetch: queryFn,
    queryKey,
    initialPerPage,
    initialSorting,
    listAllRecords = false,
    enableAdvancedFilter = false,
    columns,
    enabled = true,
  } = options;

  const localState = useOptionalDataTableLocalState<TData>();
  const defaultPerPage = listAllRecords
    ? LIST_ALL_PER_PAGE
    : (initialPerPage ?? 10);

  const columnIds = React.useMemo(
    () =>
      new Set(columns.map((c) => c.id).filter(Boolean) as string[]),
    [columns]
  );

  const sortParser = React.useMemo(
    () => getSortingStateParser<TData>(columnIds),
    [columnIds]
  );

  const [urlPage] = useQueryState(QUERY_KEYS.PAGE, parseAsInteger.withDefault(1));
  const [urlPerPage] = useQueryState(
    QUERY_KEYS.PER_PAGE,
    parseAsInteger.withDefault(defaultPerPage)
  );
  const [urlSorting] = useQueryState(
    QUERY_KEYS.SORT,
    sortParser.withDefault(initialSorting ?? []),
  );
  const page = localState?.page ?? urlPage;
  const perPage = localState?.perPage ?? urlPerPage;
  const sorting = localState?.sorting ?? urlSorting;

  const filterableColumns = React.useMemo(
    () =>
      (columns as ColumnWithFilter[]).filter(
        (c) =>
          c.enableColumnFilter &&
          !(c.meta as { enableOnlyAdvancedFilters?: boolean } | undefined)
            ?.enableOnlyAdvancedFilters
      ),
    [columns]
  );

  const filterParsers = React.useMemo((): Record<string, Parser<string> | Parser<string[]>> => {
    return filterableColumns.reduce<Record<string, Parser<string> | Parser<string[]>>>(
      (acc, column) => {
        acc[column.id ?? ""] = getColumnFilterParser(column);
        return acc;
      },
      {}
    );
  }, [filterableColumns]);

  const [urlFilterValues, setUrlFilterValues] = useQueryStates(filterParsers);
  const filterValues = localState?.filterValues ?? urlFilterValues;

  const clearColumnFilters = React.useCallback(() => {
    if (localState) {
      localState.clearColumnFilters();
      return;
    }
    setUrlFilterValues(
      Object.fromEntries(
        Object.keys(filterParsers).map((k) => [k, null])
      ) as Record<string, string | string[] | null>
    );
  }, [filterParsers, localState, setUrlFilterValues]);

  const [urlAdvancedFilters] = useAdvancedFiltersQueryState<TData>();
  const [urlJoinOperator] = useQueryState(
    QUERY_KEYS.JOIN_OPERATOR,
    parseAsStringEnum([...dataTableConfig.joinOperators]).withDefault("and")
  );
  const advancedFilters = localState?.advancedFilters ?? urlAdvancedFilters;
  const joinOperator = localState?.joinOperator ?? urlJoinOperator;

  const state: ServerTableState<TData> = React.useMemo(() => {
    const columnFilters = enableAdvancedFilter
      ? []
      : Object.entries(filterValues).reduce<ServerColumnFilter[]>(
        (acc, [id, value]) => {
          if (value != null && value !== "") {
            const column = filterableColumns.find(
              (c) => (c.id ?? "") === id
            ) as ColumnWithFilter | undefined;
            acc.push({
              id,
              value: Array.isArray(value) ? value : value,
              ...(column?.meta?.variant != null && {
                variant: column.meta.variant,
              }),
            });
          }
          return acc;
        },
        []
      );

    const normalizedAdvancedFilters = enableAdvancedFilter
      ? normalizeAdvancedFilterJoinOperators(
        getValidAdvancedFilterEntries(
          (advancedFilters ?? []) as AdvancedFilterEntry<TData>[],
        ),
        (joinOperator ?? "and") as JoinOperator,
      )
      : [];

    return {
      pagination: { page, perPage },
      sorting: (sorting ?? []) as ExtendedColumnSort<TData>[],
      columnFilters,
      ...(enableAdvancedFilter && {
        filters: normalizedAdvancedFilters,
        joinOperator: joinOperator as JoinOperator | undefined,
      }),
    };
  }, [
    page,
    perPage,
    sorting,
    enableAdvancedFilter,
    filterValues,
    filterableColumns,
    advancedFilters,
    joinOperator,
  ]);

  const stateParamsKey = React.useMemo(
    () => JSON.stringify(serializeServerTableParams(state)),
    [state],
  );

  const resolvedQueryKey =
    typeof queryKey === "function"
      ? queryKey(state)
      : [...queryKey, stateParamsKey];

  const query = useQuery({
    queryKey: resolvedQueryKey,
    queryFn: () =>
      queryFn(
        serializeServerTableParams(state) as unknown as ServerTableState<TData>,
      ),
    placeholderData: (prev) => prev,
    enabled,
  });

  const data = query.data?.data ?? [];
  const pagination: ServerPaginationMeta = query.data?.pagination ?? {
    pageCount: 0,
    total: 0,
    page,
    perPage,
  };

  return {
    data,
    /** Full object returned by `fetch` (includes `data`/`pagination` plus any extra API fields). */
    fetchResult: query.data,
    isLoading: query.isLoading,
    isFetching: query.isFetching,
    isError: query.isError,
    error: query.error,
    pagination,
    refetch: query.refetch,
    clearColumnFilters,
  };
}
