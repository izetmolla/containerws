import type { Table } from "@tanstack/react-table";
import type * as React from "react";

import { cn } from "@/lib/utils";
import {
  DataTableAdvancedOptions,
  DataTableAdvancedResetButton,
} from "./data-table-advanced-options";

interface DataTableAdvancedToolbarProps<TData>
  extends React.ComponentProps<"div"> {
  withoutAdvancedOptions?: boolean;
  table: Table<TData>;
  clearColumnFilters?: () => void;
}

export function DataTableAdvancedToolbar<TData>({
  table: _table,
  children,
  className,
  withoutAdvancedOptions,
  clearColumnFilters,
  ...props
}: DataTableAdvancedToolbarProps<TData>) {
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
        {children}
        {!withoutAdvancedOptions && <DataTableAdvancedResetButton />}
      </div>
      <div className="flex items-center gap-2">
        {!withoutAdvancedOptions && (
          <DataTableAdvancedOptions clearColumnFilters={clearColumnFilters} />
        )}
      </div>
    </div>
  );
}
