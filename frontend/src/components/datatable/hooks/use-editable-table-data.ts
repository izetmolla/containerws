"use client";

import * as React from "react";

import type { CellEditEvent } from "../types/data-table";

export interface UseEditableTableDataOptions<TData> {
    getRowId?: (row: TData) => string;
    onCellEdit?: (event: CellEditEvent<TData>) => void;
}

/**
 * Manages client-side table data with inline cell edits.
 * Updates local row state and optionally forwards edits via `onCellEdit`.
 */
export function useEditableTableData<TData extends Record<string, unknown>>(
    initialData: TData[],
    options: UseEditableTableDataOptions<TData> = {},
) {
    const { getRowId, onCellEdit } = options;
    const [data, setData] = React.useState(initialData);

    React.useEffect(() => {
        setData(initialData);
    }, [initialData]);

    const resolveRowId = React.useCallback(
        (row: TData) => {
            if (getRowId) return getRowId(row);
            const id = row.id;
            return id != null ? String(id) : "";
        },
        [getRowId],
    );

    const handleCellEdit = React.useCallback(
        (event: CellEditEvent<TData>) => {
            setData((prev) =>
                prev.map((row) => {
                    if (resolveRowId(row) !== event.rowId) return row;
                    return { ...row, [event.columnId]: event.value };
                }),
            );
            onCellEdit?.(event);
        },
        [onCellEdit, resolveRowId],
    );

    return { data, setData, onCellEdit: handleCellEdit };
}
