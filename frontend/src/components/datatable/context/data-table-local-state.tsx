"use client";

import * as React from "react";

import type { FilterMode } from "../lib/filter-mode";
import type { AdvancedFilterEntry } from "../lib/advanced-filter-types";
import type {
  ExtendedColumnSort,
  JoinOperator,
} from "../types/data-table";

export interface DataTableLocalState<TData = unknown> {
  page: number;
  perPage: number;
  sorting: ExtendedColumnSort<TData>[];
  filterValues: Record<string, string | string[] | null>;
  advancedFilters: AdvancedFilterEntry<TData>[];
  joinOperator: JoinOperator;
  filterFlag: FilterMode | null;
  setPage: (page: number) => void;
  setPerPage: (perPage: number) => void;
  setSorting: (sorting: ExtendedColumnSort<TData>[]) => void;
  setFilterValues: (
    updates:
      | Record<string, string | string[] | null>
      | ((
          prev: Record<string, string | string[] | null>,
        ) => Record<string, string | string[] | null>),
  ) => void;
  setAdvancedFilters: (
    value:
      | AdvancedFilterEntry<TData>[]
      | ((
          prev: AdvancedFilterEntry<TData>[],
        ) => AdvancedFilterEntry<TData>[]),
  ) => void;
  setJoinOperator: (op: JoinOperator) => void;
  setFilterFlag: (flag: FilterMode | null) => void;
  clearColumnFilters: () => void;
  clearSorting: () => void;
  clearAdvancedFilters: () => void;
}

const DataTableLocalStateContext =
  React.createContext<DataTableLocalState | null>(null);

export function DataTableLocalStateProvider<TData>({
  value,
  children,
}: {
  value: DataTableLocalState<TData>;
  children: React.ReactNode;
}) {
  return (
    <DataTableLocalStateContext.Provider
      value={value as DataTableLocalState}
    >
      {children}
    </DataTableLocalStateContext.Provider>
  );
}

export function useOptionalDataTableLocalState<
  TData = unknown,
>(): DataTableLocalState<TData> | null {
  return React.useContext(
    DataTableLocalStateContext,
  ) as DataTableLocalState<TData> | null;
}

export function useCreateDataTableLocalState<TData>({
  initialSorting,
  initialPerPage,
}: {
  initialSorting?: ExtendedColumnSort<TData>[];
  initialPerPage?: number;
}): DataTableLocalState<TData> {
  const [page, setPage] = React.useState(1);
  const [perPage, setPerPage] = React.useState(initialPerPage ?? 10);
  const [sorting, setSorting] = React.useState<ExtendedColumnSort<TData>[]>(
    initialSorting ?? [],
  );
  const [filterValues, setFilterValuesState] = React.useState<
    Record<string, string | string[] | null>
  >({});
  const [advancedFilters, setAdvancedFilters] = React.useState<
    AdvancedFilterEntry<TData>[]
  >([]);
  const [joinOperator, setJoinOperator] =
    React.useState<JoinOperator>("and");
  const [filterFlag, setFilterFlag] = React.useState<FilterMode | null>(null);

  const setFilterValues = React.useCallback(
    (
      updates:
        | Record<string, string | string[] | null>
        | ((
            prev: Record<string, string | string[] | null>,
          ) => Record<string, string | string[] | null>),
    ) => {
      setFilterValuesState((prev) =>
        typeof updates === "function" ? updates(prev) : { ...prev, ...updates },
      );
    },
    [],
  );

  const clearColumnFilters = React.useCallback(() => {
    setFilterValuesState({});
  }, []);

  const clearSorting = React.useCallback(() => {
    setSorting([]);
  }, []);

  const clearAdvancedFilters = React.useCallback(() => {
    setAdvancedFilters([]);
    setJoinOperator("and");
  }, []);

  return React.useMemo(
    () => ({
      page,
      perPage,
      sorting,
      filterValues,
      advancedFilters,
      joinOperator,
      filterFlag,
      setPage,
      setPerPage,
      setSorting,
      setFilterValues,
      setAdvancedFilters,
      setJoinOperator,
      setFilterFlag,
      clearColumnFilters,
      clearSorting,
      clearAdvancedFilters,
    }),
    [
      page,
      perPage,
      sorting,
      filterValues,
      advancedFilters,
      joinOperator,
      filterFlag,
      setFilterValues,
      clearColumnFilters,
      clearSorting,
      clearAdvancedFilters,
    ],
  );
}
