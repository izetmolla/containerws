"use client";

import { useQueryState, parseAsString } from "nuqs";
import { FileSpreadsheetIcon, ListFilter, RotateCcw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import * as React from "react";

import { useOptionalDataTableLocalState } from "../context/data-table-local-state";
import { QUERY_KEYS } from "../lib/constants";
import type { FilterMode } from "../lib/filter-mode";

const SIMPLE_VALUE = "simple";
const ADVANCED_VALUE: FilterMode = "advancedFilters";

const validValues: FilterMode[] = [SIMPLE_VALUE as FilterMode, ADVANCED_VALUE];

export function useTableAdvancedOptions() {
  const localState = useOptionalDataTableLocalState();
  const [rawFiltersParam] = useQueryState(QUERY_KEYS.FILTERS, parseAsString);
  const [urlFilterFlag, setUrlFilterFlag] = useQueryState<FilterMode | null>(
    "filterFlag",
    {
      parse: (value) => {
        if (!value || value === SIMPLE_VALUE) return null;
        return validValues.includes(value as FilterMode)
          ? (value as FilterMode)
          : null;
      },
      serialize: (value) => (value === ADVANCED_VALUE ? ADVANCED_VALUE : ""),
      defaultValue: null,
      clearOnDefault: true,
      shallow: false,
      eq: (a, b) => (!a && !b) || a === b,
    },
  );

  const filterFlag = localState?.filterFlag ?? urlFilterFlag;
  const setFilterFlag = localState?.setFilterFlag ?? setUrlFilterFlag;
  const hasUrlFilters = hasActiveAdvancedFilters(rawFiltersParam);

  React.useEffect(() => {
    if (localState || !hasUrlFilters || filterFlag === ADVANCED_VALUE) return;
    void setUrlFilterFlag(ADVANCED_VALUE);
  }, [filterFlag, hasUrlFilters, localState, setUrlFilterFlag]);

  return {
    enableAdvancedFilter: filterFlag === ADVANCED_VALUE || hasUrlFilters,
    filterFlag: filterFlag === ADVANCED_VALUE || hasUrlFilters ? ADVANCED_VALUE : filterFlag,
    setFilterFlag,
  };
}

interface DataTableAdvancedOptionsProps {
  clearColumnFilters?: () => void;
}

function hasActiveAdvancedFilters(rawFilters: string | null | unknown): boolean {
  if (rawFilters == null) return false;
  if (typeof rawFilters === "string") {
    const trimmed = rawFilters.trim();
    if (trimmed === "" || trimmed === "[]") return false;
    return true;
  }
  if (Array.isArray(rawFilters)) return rawFilters.length > 0;
  return false;
}

/** Reset button for advanced filters; visible only when advanced mode is on and any filter is applied. Render after the Filter button in the toolbar. Clears filters, join operator, and sortings. */
export function DataTableAdvancedResetButton() {
  const { filterFlag } = useTableAdvancedOptions();
  const localState = useOptionalDataTableLocalState();
  const [rawFilters, setUrlFilters] = useQueryState(
    QUERY_KEYS.FILTERS,
    parseAsString
  );
  const [, setUrlJoinOperator] = useQueryState(
    QUERY_KEYS.JOIN_OPERATOR,
    parseAsString
  );
  const [, setUrlSorting] = useQueryState(QUERY_KEYS.SORT, parseAsString);

  const rawFiltersValue = localState?.advancedFilters ?? rawFilters ?? null;
  const showReset =
    (filterFlag === ADVANCED_VALUE || hasActiveAdvancedFilters(rawFilters)) &&
    hasActiveAdvancedFilters(
      localState
        ? localState.advancedFilters
        : (rawFiltersValue as string | null),
    );

  const handleResetFilters = () => {
    if (localState) {
      localState.clearAdvancedFilters();
      localState.clearSorting();
      return;
    }
    setUrlFilters(null);
    setUrlJoinOperator(null);
    setUrlSorting(null);
  };

  if (!showReset) return null;

  return (
    <Button
      variant="outline"
      size="sm"
      className="h-8 gap-1.5"
      onClick={handleResetFilters}
      aria-label="Reset advanced filters"
    >
      <RotateCcw className="size-3.5" />
      Reset
    </Button>
  );
}

export function DataTableAdvancedOptions({
  clearColumnFilters,
}: DataTableAdvancedOptionsProps = {}) {
  const { filterFlag, setFilterFlag } = useTableAdvancedOptions();
  const localState = useOptionalDataTableLocalState();
  const [, setUrlFilters] = useQueryState(QUERY_KEYS.FILTERS, parseAsString);
  const [, setUrlJoinOperator] = useQueryState(
    QUERY_KEYS.JOIN_OPERATOR,
    parseAsString
  );

  const currentValue =
    filterFlag === ADVANCED_VALUE ? ADVANCED_VALUE : SIMPLE_VALUE;

  const handleModeChange = (value: string | null) => {
    if (value === SIMPLE_VALUE) {
      if (localState) {
        localState.clearAdvancedFilters();
      } else {
        setUrlFilters(null);
        setUrlJoinOperator(null);
      }
      setFilterFlag(null);
    } else if (value === ADVANCED_VALUE) {
      clearColumnFilters?.();
      setFilterFlag(ADVANCED_VALUE);
    }
  };

  return (
    <ToggleGroup
      type="single"
      value={currentValue}
      onValueChange={(value) => {
        if (value) {
          handleModeChange(value);
        }
      }}
      variant="outline"
      size="sm"
      className="h-8"
    >
      <ToggleGroupItem
        value={SIMPLE_VALUE}
        aria-label="Simple filters"
        className="size-8 p-0"
      >
        <ListFilter className="size-4" />
      </ToggleGroupItem>
      <ToggleGroupItem
        value={ADVANCED_VALUE}
        aria-label="Advanced filters"
        className="size-8 p-0"
      >
        <FileSpreadsheetIcon className="size-4" />
      </ToggleGroupItem>
    </ToggleGroup>
  );
}

export type { FilterMode } from "../lib/filter-mode";
