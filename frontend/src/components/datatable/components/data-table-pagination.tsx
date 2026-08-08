import type { Table } from "@tanstack/react-table";
import {
  ChevronLeft,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  Loader2,
} from "lucide-react";
import * as React from "react";

import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";

type PendingDirection = "first" | "prev" | "next" | "last";

interface DataTablePaginationProps<TData> extends React.ComponentProps<"div"> {
  table: Table<TData>;
  /** Options for rows per page (perPage) */
  perPageOptions?: number[];
  /** When true (e.g. server fetch in progress), pagination buttons are disabled and show loading on the clicked one */
  isFetching?: boolean;
  /** When true, skip client filtered row models (server already filters data). */
  serverSide?: boolean;
  /** When true, show total row count to the left of the rows-per-page selector */
  showTotalRows?: boolean;
  /** Total rows (server total or client filtered count). Required when showTotalRows is true. */
  totalRows?: number;
}

export function DataTablePagination<TData>({
  table,
  perPageOptions = [10, 20, 30, 40, 50, 100, 200, 500, 1000, 1500, 2000, 5000, 10000, 100000, 500000],
  isFetching = false,
  serverSide = false,
  showTotalRows = false,
  totalRows,
  className,
  ...props
}: DataTablePaginationProps<TData>) {
  const [pendingDirection, setPendingDirection] =
    React.useState<PendingDirection | null>(null);

  React.useEffect(() => {
    if (!isFetching) setPendingDirection(null);
  }, [isFetching]);

  const pagination = table.getState().pagination;
  const page = pagination.pageIndex + 1;
  const perPage = pagination.pageSize;

  const canPrev = table.getCanPreviousPage();
  const canNext = table.getCanNextPage();
  const pageCount = table.getPageCount();
  const isDisabled = isFetching;

  const goFirst = React.useCallback(() => {
    if (!canPrev || isDisabled) return;
    setPendingDirection("first");
    table.setPageIndex(0);
  }, [table, canPrev, isDisabled]);

  const goPrev = React.useCallback(() => {
    if (!canPrev || isDisabled) return;
    setPendingDirection("prev");
    table.previousPage();
  }, [table, canPrev, isDisabled]);

  const goNext = React.useCallback(() => {
    if (!canNext || isDisabled) return;
    setPendingDirection("next");
    table.nextPage();
  }, [table, canNext, isDisabled]);

  const goLast = React.useCallback(() => {
    if (!canNext || isDisabled) return;
    setPendingDirection("last");
    table.setPageIndex(pageCount - 1);
  }, [table, canNext, isDisabled, pageCount]);

  const icon = (direction: PendingDirection) =>
    isFetching && pendingDirection === direction ? (
      <Loader2 className="size-4 animate-spin" />
    ) : null;

  const selectedCount = table.getSelectedRowModel().rows.length;
  const visibleCount = serverSide
    ? table.getRowModel().rows.length
    : table.getFilteredRowModel().rows.length;

  return (
    <div
      className={cn(
        "flex w-full flex-col-reverse items-center justify-between gap-4 overflow-auto p-1 sm:flex-row sm:gap-8",
        className,
      )}
      {...props}
    >
      <div className="flex-1 whitespace-nowrap text-muted-foreground text-sm">
        {selectedCount} of {visibleCount} row(s) selected.
      </div>
      <div className="flex flex-col-reverse items-center gap-4 sm:flex-row sm:gap-6 lg:gap-8">
        <div className="flex items-center space-x-2">
          {showTotalRows ? (
            <p className="whitespace-nowrap font-medium text-sm text-muted-foreground bold">
              ({totalRows != null ? totalRows.toLocaleString() : "—"} rows)
            </p>
          ) : null}
          <div className="flex items-center space-x-2">
            <p className="whitespace-nowrap font-medium text-sm">Rows per page</p>
            <Select
              value={`${perPage}`}
              disabled={isDisabled}
              onValueChange={(value) => {
                table.setPageSize(Number(value));
              }}
            >
              <SelectTrigger className="h-8 w-[4.5rem] [&[data-size]]:h-8">
                <SelectValue placeholder={perPage} />
              </SelectTrigger>
              <SelectContent side="top">
                {perPageOptions.map((size) => (
                  <SelectItem key={size} value={`${size}`}>
                    {size}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>
        <div className="flex items-center justify-center font-medium text-sm">
          Page {page} of{" "}
          {table.getPageCount()}
        </div>
        <div className="flex items-center space-x-2">
          <Button
            aria-label="Go to first page"
            variant="outline"
            size="icon"
            className="hidden size-10 lg:flex"
            onClick={goFirst}
            disabled={!canPrev || isDisabled}
          >
            {icon("first") ?? <ChevronsLeft />}
          </Button>
          <Button
            aria-label="Go to previous page"
            variant="outline"
            size="icon"
            className="size-8"
            onClick={goPrev}
            disabled={!canPrev || isDisabled}
          >
            {icon("prev") ?? <ChevronLeft />}
          </Button>
          <Button
            aria-label="Go to next page"
            variant="outline"
            size="icon"
            className="size-8"
            onClick={goNext}
            disabled={!canNext || isDisabled}
          >
            {icon("next") ?? <ChevronRight />}
          </Button>
          <Button
            aria-label="Go to last page"
            variant="outline"
            size="icon"
            className="hidden size-8 lg:flex"
            onClick={goLast}
            disabled={!canNext || isDisabled}
          >
            {icon("last") ?? <ChevronsRight />}
          </Button>
        </div>
      </div>
    </div>
  );
}
