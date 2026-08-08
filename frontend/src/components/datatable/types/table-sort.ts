import type { ColumnSort } from "@tanstack/react-table";

export interface ExtendedColumnSort<TData> extends Omit<ColumnSort, "id"> {
    id: Extract<keyof TData, string>;
}
