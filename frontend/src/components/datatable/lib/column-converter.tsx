import type { ColumnDef } from "@tanstack/react-table";
import { DataTableColumnHeader } from "../components/data-table-column-header";
import { Badge } from "@/components/ui/badge";
import type {
  BackendColumnDefinition,
  BackendColumnMeta,
  Option,
} from "../types/data-table";
import type { FilterVariant } from "../types/data-table";
import { hasValidFilterVariant } from "./filter-variants";

function toFilterVariant(variant: string | undefined): FilterVariant | undefined {
  if (!hasValidFilterVariant(variant)) return undefined;
  return variant as FilterVariant;
}

function toOptions(meta: BackendColumnMeta | undefined): Option[] | undefined {
  if (!meta?.options?.length) return undefined;
  return meta.options.map((o) => ({ label: o.label, value: o.value }));
}

/**
 * Converts backend column definitions to TanStack Table ColumnDef.
 * Uses DataTableColumnHeader for headers and variant-based default cells.
 */
export function convertBackendColumns<TData extends Record<string, unknown>>(
  backendColumns: BackendColumnDefinition[]
): ColumnDef<TData>[] {
  return backendColumns.map((col) => {
    const meta = col.meta;
    const variant = toFilterVariant(meta?.variant);
    const options = toOptions(meta);
    const hasVariant = hasValidFilterVariant(meta?.variant);
    const canFilterSimple =
      (col.enableColumnFilter ?? false) && hasVariant;
    const onlyAdvancedFilters = col.enableOnlyAdvancedFilters ?? false;
    const canFilterAdvanced = onlyAdvancedFilters && hasVariant;
    const enableColumnFilter = canFilterSimple || canFilterAdvanced;

    const columnDef: ColumnDef<TData> = {
      id: col.id,
      accessorKey: col.accessorKey as keyof TData & string,
      header: ({ column }) => (
        <DataTableColumnHeader
          column={column}
          title={col.header}
        />
      ),
      cell: ({ row }) => {
        const value = row.getValue(col.accessorKey) as unknown;
        if (value == null || value === "") return "-";
        const str = String(value);
        if (variant === "multiSelect" || variant === "select") {
          return (
            <Badge variant="outline" className="capitalize">
              {str}
            </Badge>
          );
        }
        return str;
      },
      enableSorting: col.enableSorting ?? true,
      enableColumnFilter,
      meta: {
        label: meta?.label ?? col.header,
        placeholder: meta?.placeholder,
        ...(variant !== undefined && { variant }),
        options,
        className: "w-full",
        ...(onlyAdvancedFilters && { enableOnlyAdvancedFilters: true }),
        ...(meta?.hidden && { hidden: true }),
        ...(meta?.disabled && { disabled: true }),
        ...(meta?.filterBy && { filterBy: meta.filterBy }),
      },
    };

    if (col.size != null) {
      columnDef.size = col.size;
    }

    return columnDef;
  });
}
