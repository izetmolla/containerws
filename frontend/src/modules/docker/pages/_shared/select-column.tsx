import { type ColumnHelper } from "@tanstack/react-table"

import { Checkbox } from "@/components/ui/checkbox"

/** Checkbox column for DataTable multi-select (requires enableRowSelection + getRowId). */
export function selectColumn<T>(columnHelper: ColumnHelper<T>, label = "row") {
  return columnHelper.display({
    id: "select",
    enableSorting: false,
    header: ({ table }) => (
      <Checkbox
        checked={
          table.getIsAllPageRowsSelected()
            ? true
            : table.getIsSomePageRowsSelected()
              ? "indeterminate"
              : false
        }
        onCheckedChange={(v) => table.toggleAllPageRowsSelected(!!v)}
        aria-label="Select all"
      />
    ),
    cell: ({ row }) => (
      <Checkbox
        checked={row.getIsSelected()}
        onCheckedChange={(v) => row.toggleSelected(!!v)}
        aria-label={`Select ${label}`}
        onClick={(e) => e.stopPropagation()}
      />
    ),
    meta: { width: 40, className: "w-10 px-2" },
  })
}
