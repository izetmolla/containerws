import { useEffect, useState, type ReactNode } from "react"
import {
  flexRender,
  getCoreRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  useReactTable,
  type ColumnDef,
  type OnChangeFn,
  type PaginationState,
  type RowSelectionState,
  type SortingState,
} from "@tanstack/react-table"
import { ArrowDown, ArrowUp, ArrowUpDown, ChevronLeft, ChevronRight } from "lucide-react"

import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

const DEFAULT_PAGE_SIZES = [10, 25, 50, 100]

type DataTableProps<TData> = {
  columns: ColumnDef<TData, unknown>[]
  data: TData[]
  emptyMessage?: string
  className?: string
  toolbar?: ReactNode
  toolbarStart?: ReactNode
  /** Tighter rows and softer chrome for denser admin lists. */
  dense?: boolean
  /** Header label alignment. Default `start`. */
  headerAlign?: "start" | "center"
  /** Enable multi-row selection (pass with getRowId). */
  enableRowSelection?: boolean
  rowSelection?: RowSelectionState
  onRowSelectionChange?: OnChangeFn<RowSelectionState>
  getRowId?: (originalRow: TData, index: number) => string
  /** Client-side column sorting. */
  enableSorting?: boolean
  sorting?: SortingState
  onSortingChange?: OnChangeFn<SortingState>
  initialSorting?: SortingState
  /** Client-side pagination. */
  enablePagination?: boolean
  pageSize?: number
  pageSizeOptions?: number[]
  /** When this value changes, pagination returns to page 1 (e.g. search query). */
  paginationResetKey?: string | number
}

export function DataTable<TData>({
  columns,
  data,
  emptyMessage = "No results.",
  className,
  toolbar,
  toolbarStart,
  dense = false,
  headerAlign = "start",
  enableRowSelection = false,
  rowSelection,
  onRowSelectionChange,
  getRowId,
  enableSorting = false,
  sorting: controlledSorting,
  onSortingChange,
  initialSorting = [],
  enablePagination = false,
  pageSize: initialPageSize = 10,
  pageSizeOptions = DEFAULT_PAGE_SIZES,
  paginationResetKey,
}: DataTableProps<TData>) {
  const [uncontrolledSorting, setUncontrolledSorting] =
    useState<SortingState>(initialSorting)
  const [pagination, setPagination] = useState<PaginationState>({
    pageIndex: 0,
    pageSize: initialPageSize,
  })

  useEffect(() => {
    if (!enablePagination) return
    setPagination((p) => ({ ...p, pageIndex: 0 }))
  }, [paginationResetKey, enablePagination])

  const sorting = controlledSorting ?? uncontrolledSorting
  const handleSortingChange: OnChangeFn<SortingState> =
    onSortingChange ?? setUncontrolledSorting

  const table = useReactTable({
    data,
    columns,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: enableSorting ? getSortedRowModel() : undefined,
    getPaginationRowModel: enablePagination
      ? getPaginationRowModel()
      : undefined,
    enableRowSelection,
    enableSorting,
    autoResetPageIndex: false,
    onRowSelectionChange,
    onSortingChange: enableSorting ? handleSortingChange : undefined,
    onPaginationChange: enablePagination ? setPagination : undefined,
    getRowId,
    // Always pass concrete pagination — `undefined` overrides the library
    // default and makes getPageCount() crash on `.pageSize`.
    state: {
      rowSelection: rowSelection ?? {},
      ...(enableSorting ? { sorting } : {}),
      pagination,
    },
  })

  const showToolbar = Boolean(toolbar || toolbarStart)
  const { pageIndex, pageSize } = table.getState().pagination
  const pageCount = enablePagination ? table.getPageCount() : 0
  const filteredCount = table.getFilteredRowModel().rows.length
  const from = filteredCount === 0 ? 0 : pageIndex * pageSize + 1
  const to = Math.min(filteredCount, (pageIndex + 1) * pageSize)

  return (
    <div className={cn("flex w-full flex-col", dense ? "gap-2" : "gap-3", className)}>
      {showToolbar ? (
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="min-w-0 flex-1">{toolbarStart}</div>
          {toolbar ? (
            <div className="flex shrink-0 flex-wrap items-center gap-2">{toolbar}</div>
          ) : null}
        </div>
      ) : null}
      <div
        className={cn(
          "overflow-hidden border border-border/70 bg-card/40",
          dense ? "rounded-md" : "rounded-xl shadow-sm"
        )}
      >
        <div className="overflow-x-auto">
          <table className="w-full min-w-[720px] border-collapse text-sm">
            <thead>
              {table.getHeaderGroups().map((headerGroup) => (
                <tr
                  key={headerGroup.id}
                  className="border-b border-border/70 bg-muted/30"
                >
                  {headerGroup.headers.map((header) => {
                    const meta = header.column.columnDef.meta as
                      | { className?: string; width?: number }
                      | undefined
                    const canSort = enableSorting && header.column.getCanSort()
                    const sorted = header.column.getIsSorted()
                    return (
                      <th
                        key={header.id}
                        className={cn(
                          "align-middle text-[11px] font-medium tracking-[0.08em] text-muted-foreground uppercase",
                          headerAlign === "center" ? "text-center" : "text-start",
                          dense ? "h-9 px-3" : "h-11 px-4",
                          meta?.className
                        )}
                        style={
                          meta?.width
                            ? { width: meta.width, minWidth: meta.width }
                            : undefined
                        }
                      >
                        {header.isPlaceholder ? null : canSort ? (
                          <button
                            type="button"
                            className={cn(
                              "inline-flex items-center gap-1 uppercase transition-colors hover:text-foreground",
                              sorted && "text-foreground"
                            )}
                            onClick={header.column.getToggleSortingHandler()}
                          >
                            {flexRender(
                              header.column.columnDef.header,
                              header.getContext()
                            )}
                            {sorted === "asc" ? (
                              <ArrowUp className="size-3.5" />
                            ) : sorted === "desc" ? (
                              <ArrowDown className="size-3.5" />
                            ) : (
                              <ArrowUpDown className="size-3.5 opacity-40" />
                            )}
                          </button>
                        ) : (
                          flexRender(
                            header.column.columnDef.header,
                            header.getContext()
                          )
                        )}
                      </th>
                    )
                  })}
                </tr>
              ))}
            </thead>
            <tbody>
              {table.getRowModel().rows.length ? (
                table.getRowModel().rows.map((row) => (
                  <tr
                    key={row.id}
                    data-state={row.getIsSelected() ? "selected" : undefined}
                    className={cn(
                      "border-b border-border/50 last:border-0 transition-colors hover:bg-muted/25",
                      row.getIsSelected() && "bg-primary/5 hover:bg-primary/10"
                    )}
                  >
                    {row.getVisibleCells().map((cell) => {
                      const meta = cell.column.columnDef.meta as
                        | { className?: string; width?: number }
                        | undefined
                      return (
                        <td
                          key={cell.id}
                          className={cn(
                            "align-middle",
                            dense ? "px-3 py-2" : "px-4 py-3.5",
                            meta?.className
                          )}
                          style={
                            meta?.width
                              ? { width: meta.width, minWidth: meta.width }
                              : undefined
                          }
                        >
                          {flexRender(
                            cell.column.columnDef.cell,
                            cell.getContext()
                          )}
                        </td>
                      )
                    })}
                  </tr>
                ))
              ) : (
                <tr>
                  <td
                    colSpan={columns.length}
                    className={cn(
                      "px-3 text-center text-sm text-muted-foreground",
                      dense ? "h-20" : "h-28 px-4"
                    )}
                  >
                    {emptyMessage}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      {enablePagination ? (
        <div className="flex flex-wrap items-center justify-between gap-3 px-1">
          <p className="text-xs text-muted-foreground">
            {filteredCount === 0
              ? "No results"
              : `Showing ${from}–${to} of ${filteredCount}`}
          </p>
          <div className="flex flex-wrap items-center gap-3">
            <div className="flex items-center gap-2">
              <span className="text-xs text-muted-foreground">Items per page</span>
              <select
                aria-label="Items per page"
                className="h-7 appearance-none rounded-[min(var(--radius-md),10px)] border border-input bg-background px-2.5 pe-7 text-xs font-medium outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 dark:bg-input/30"
                style={{
                  backgroundImage: `url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 24 24' fill='none' stroke='%23888' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='m6 9 6 6 6-6'/%3E%3C/svg%3E")`,
                  backgroundRepeat: "no-repeat",
                  backgroundPosition: "right 0.45rem center",
                }}
                value={pageSize}
                onChange={(e) => table.setPageSize(Number(e.target.value))}
              >
                {pageSizeOptions.map((n) => (
                  <option key={n} value={n}>
                    {n}
                  </option>
                ))}
              </select>
            </div>
            <div className="flex items-center gap-1">
              <Button
                size="icon-sm"
                variant="outline"
                disabled={!table.getCanPreviousPage()}
                onClick={() => table.previousPage()}
                aria-label="Previous page"
              >
                <ChevronLeft className="size-3.5" />
              </Button>
              <span className="min-w-[4.5rem] text-center text-xs text-muted-foreground">
                {pageCount === 0 ? 0 : pageIndex + 1} / {pageCount}
              </span>
              <Button
                size="icon-sm"
                variant="outline"
                disabled={!table.getCanNextPage()}
                onClick={() => table.nextPage()}
                aria-label="Next page"
              >
                <ChevronRight className="size-3.5" />
              </Button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  )
}
