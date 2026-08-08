"use client";

import type { Column, Table } from "@tanstack/react-table";
import { useQueryState, parseAsString } from "nuqs";
import { X } from "lucide-react";
import * as React from "react";

import { DataTableDateFilter } from "./data-table-date-filter";
import { DataTableReactSelectFilter } from "./data-table-react-select-filter";
import { DataTableSliderFilter } from "./data-table-slider-filter";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { useOptionalDataTableLocalState } from "../context/data-table-local-state";
import { DEBOUNCE_MS_DEFAULT, QUERY_KEYS } from "../lib/constants";
import { hasValidFilterVariant } from "../lib/filter-variants";
import { useDebouncedCallback } from "../hooks/use-debounced-callback";
import { DataTableAdvancedOptions } from "./data-table-advanced-options";

interface DataTableToolbarProps<TData> extends React.ComponentProps<"div"> {
  withoutAdvancedOptions?: boolean;
  table: Table<TData>;
  clearColumnFilters?: () => void;
}

export function DataTableToolbar<TData>({
  table,
  children,
  className,
  withoutAdvancedOptions,
  clearColumnFilters,
  ...props
}: DataTableToolbarProps<TData>) {
  const isFiltered = table.getState().columnFilters.length > 0;
  const localState = useOptionalDataTableLocalState();
  const [, setUrlSorting] = useQueryState(QUERY_KEYS.SORT, parseAsString);

  const columns = React.useMemo(
    () =>
      table
        .getAllColumns()
        .filter(
          (column) =>
            column.getCanFilter() &&
            hasValidFilterVariant(column.columnDef.meta?.variant) &&
            !column.columnDef.meta?.enableOnlyAdvancedFilters &&
            !column.columnDef.meta?.hidden,
        ),
    [table],
  );

  const onReset = React.useCallback(() => {
    clearColumnFilters?.();
    table.resetColumnFilters();
    if (localState) {
      localState.clearSorting();
    } else {
      setUrlSorting(null);
    }
  }, [table, clearColumnFilters, localState, setUrlSorting]);

  return (
    <div
      role="toolbar"
      aria-orientation="horizontal"
      className={cn(
        "flex w-full items-start justify-between gap-2 p-1",
        className,
      )}
      {...props}
    >
      <div className="flex flex-1 flex-wrap items-center gap-2">
        {columns.map((column) => (
          <DataTableToolbarFilter key={column.id} column={column} />
        ))}
        {isFiltered && (
          <Button
            aria-label="Reset filters"
            variant="outline"
            size="sm"
            className="border-dashed"
            onClick={onReset}
          >
            <X />
            Reset
          </Button>
        )}
      </div>
      <div className="flex items-center gap-2">
        {children}
        {!withoutAdvancedOptions && (
          <DataTableAdvancedOptions clearColumnFilters={clearColumnFilters} />
        )}
      </div>
    </div>
  );
}

interface DataTableToolbarFilterProps<TData> {
  column: Column<TData>;
}

function formatTextFilterValue(value: unknown): string {
  if (value == null) return "";
  if (typeof value === "string") return value;
  if (Array.isArray(value)) return value.join(" ");
  return String(value);
}

function DebouncedColumnTextFilter<TData>({
  column,
  placeholder,
  disabled,
  type = "text",
  className,
  unit,
}: {
  column: Column<TData>;
  placeholder?: string;
  disabled?: boolean;
  type?: "text" | "number";
  className?: string;
  unit?: string;
}) {
  const committed = formatTextFilterValue(column.getFilterValue());
  const [draft, setDraft] = React.useState(committed);

  React.useEffect(() => {
    setDraft(committed);
  }, [committed]);

  const debouncedCommit = useDebouncedCallback((value: string) => {
    column.setFilterValue(value || undefined);
  }, DEBOUNCE_MS_DEFAULT);

  return (
    <div className={unit ? "relative" : undefined}>
      <Input
        type={type}
        inputMode={type === "number" ? "numeric" : undefined}
        placeholder={placeholder}
        value={draft}
        onChange={(event) => {
          const value = event.target.value;
          setDraft(value);
          debouncedCommit(value);
        }}
        disabled={disabled}
        className={className}
      />
      {unit ? (
        <span className="absolute top-0 right-0 bottom-0 flex items-center rounded-r-md bg-accent px-2 text-muted-foreground text-sm">
          {unit}
        </span>
      ) : null}
    </div>
  );
}

function DataTableToolbarFilter<TData>({
  column,
}: DataTableToolbarFilterProps<TData>) {
  const columnMeta = column.columnDef.meta;
  const disabled = columnMeta?.disabled === true;

  if (!columnMeta?.variant) return null;

  switch (columnMeta.variant) {
    case "text":
      return (
        <DebouncedColumnTextFilter
          column={column}
          placeholder={columnMeta.placeholder ?? columnMeta.label}
          disabled={disabled}
          className="h-8 w-40 lg:w-56"
        />
      );

    case "number":
      return (
        <DebouncedColumnTextFilter
          column={column}
          type="number"
          placeholder={columnMeta.placeholder ?? columnMeta.label}
          disabled={disabled}
          unit={columnMeta.unit}
          className={cn("h-8 w-[120px]", columnMeta.unit && "pr-8")}
        />
      );

    case "range":
      return (
        <DataTableSliderFilter
          column={column}
          title={columnMeta.label ?? column.id}
          disabled={disabled}
        />
      );

    case "date":
    case "dateRange":
      return (
        <DataTableDateFilter
          column={column}
          title={columnMeta.label ?? column.id}
          multiple={columnMeta.variant === "dateRange"}
          disabled={disabled}
        />
      );

    case "select":
    case "multiSelect":
      return (
        <DataTableReactSelectFilter
          column={column}
          title={columnMeta.label ?? column.id}
          options={columnMeta.options ?? []}
          multiple={columnMeta.variant === "multiSelect"}
          disabled={disabled}
        />
      );

    default:
      return null;
  }
}
