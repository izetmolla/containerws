import type { CellEditorConfig } from "../types/data-table";
import type { ColumnMeta, RowData } from "@tanstack/react-table";

const NON_EDITABLE_COLUMN_IDS = new Set(["select", "actions"]);

export function isColumnEditable(
    columnId: string,
    meta?: ColumnMeta<RowData, unknown>,
): boolean {
    if (NON_EDITABLE_COLUMN_IDS.has(columnId)) return false;
    return Boolean(meta?.editable);
}

export function resolveCellEditorConfig(
    meta?: ColumnMeta<RowData, unknown>,
): CellEditorConfig | null {
    if (!meta?.editable) return null;
    if (meta.editable === true) return { type: "text" };
    return meta.editable;
}
