import { type Table as TanstackTable, flexRender } from "@tanstack/react-table";
import type * as React from "react";

import { DataTablePagination } from "./data-table-pagination";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { getCommonPinningStyles } from "../lib/data-table";
import { isColumnEditable, resolveCellEditorConfig } from "../lib/cell-editor";
import { DataTableEditableCell } from "./data-table-editable-cell";
import type { CellEditEvent } from "../types/data-table";
import { cn } from "@/lib/utils";

interface DataTableProps<TData> extends React.ComponentProps<"div"> {
  table: TanstackTable<TData>;
  actionBar?: React.ReactNode;
  onCellEdit?: (event: CellEditEvent<TData>) => void;
}

export function DataTableContent<TData>({
  table,
  actionBar,
  onCellEdit,
  children,
  className,
  ...props
}: DataTableProps<TData>) {
  return (
    <div
      className={cn("flex w-full flex-col gap-2.5 overflow-auto", className)}
      {...props}
    >
      {children}
      <div className="overflow-x-auto rounded-md border">
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <TableHead
                    key={header.id}
                    colSpan={header.colSpan}
                    style={header.column.getIsPinned() ? {
                      ...getCommonPinningStyles({ column: header.column }),
                    } : {}}
                    className={header.column.columnDef.meta?.className ?? ""}
                  >
                    {header.isPlaceholder
                      ? null
                      : flexRender(
                        header.column.columnDef.header,
                        header.getContext(),
                      )}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {table.getRowModel().rows?.length ? (
              table.getRowModel().rows.map((row) => (
                <TableRow
                  key={row.id}
                  data-state={row.getIsSelected() && "selected"}
                >
                  {row.getVisibleCells().map((cell) => {
                    const editorConfig =
                      onCellEdit &&
                      isColumnEditable(cell.column.id, cell.column.columnDef.meta)
                        ? resolveCellEditorConfig(cell.column.columnDef.meta)
                        : null;

                    return (
                    <TableCell
                      key={cell.id}
                      style={{
                        ...getCommonPinningStyles({ column: cell.column }),
                      }}
                      className={cn(
                        cell.column.columnDef.meta?.className ?? "",
                        editorConfig && "p-0",
                      )}
                    >
                      {editorConfig ? (
                        <DataTableEditableCell
                          cell={cell}
                          editor={editorConfig}
                          displayContent={flexRender(
                            cell.column.columnDef.cell,
                            cell.getContext(),
                          )}
                          onCommit={(value) => {
                            if (!onCellEdit) return;
                            onCellEdit({
                              rowId: row.id,
                              columnId: cell.column.id,
                              value,
                              previousValue: cell.getValue(),
                              row: row.original,
                            });
                          }}
                        />
                      ) : (
                        flexRender(
                          cell.column.columnDef.cell,
                          cell.getContext(),
                        )
                      )}
                    </TableCell>
                    );
                  })}
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell
                  colSpan={table.getAllColumns().length}
                  className="h-24 text-center"
                >
                  No results.
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>
      <div className="flex flex-col gap-2.5">
        <DataTablePagination table={table} />
        {actionBar &&
          table.getFilteredSelectedRowModel().rows.length > 0 &&
          actionBar}
      </div>
    </div>
  );
}
