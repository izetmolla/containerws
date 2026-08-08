import type {
    ExtendedColumnFilter,
    FilterOperator,
    FilterVariant,
} from "../types/data-table";
import type { Column } from "@tanstack/react-table";

import { dataTableConfig } from "../config/data-table";
import {
    type AdvancedFilterEntry,
    isAdvancedFilterGroup,
} from "./advanced-filter-types";

export function getCommonPinningStyles<TData>({
    column,
    withBorder = false,
    layer = "body",
}: {
    column: Column<TData>;
    withBorder?: boolean;
    /** Header cells need a higher z-index than body cells when using sticky headers. */
    layer?: "header" | "body";
}): React.CSSProperties {
    const isPinned = column.getIsPinned();
    if (!isPinned) {
        return {};
    }

    const isLastLeftPinnedColumn =
        isPinned === "left" && column.getIsLastColumn("left");
    const isFirstRightPinnedColumn =
        isPinned === "right" && column.getIsFirstColumn("right");

    const zIndex =
        layer === "header"
            ? 30
            : column.id === "actions"
              ? 25
              : 15;

    return {
        boxShadow: withBorder
            ? isLastLeftPinnedColumn
                ? "-4px 0 4px -4px hsl(var(--border)) inset"
                : isFirstRightPinnedColumn
                    ? "4px 0 4px -4px hsl(var(--border)) inset"
                    : undefined
            : undefined,
        left: isPinned === "left" ? `${column.getStart("left")}px` : undefined,
        right: isPinned === "right" ? `${column.getAfter("right")}px` : undefined,
        opacity: 0.97,
        position: "sticky",
        background: "hsl(var(--background) / 0.85)",
        backdropFilter: "blur(8px)",
        WebkitBackdropFilter: "blur(8px)",
        width: column.getSize(),
        zIndex,
    };
}

export function getFilterOperators(filterVariant: FilterVariant) {
    const operatorMap: Record<
        FilterVariant,
        { label: string; value: FilterOperator }[]
    > = {
        text: dataTableConfig.textOperators,
        number: dataTableConfig.numericOperators,
        range: dataTableConfig.numericOperators,
        date: dataTableConfig.dateOperators,
        dateRange: dataTableConfig.dateOperators,
        boolean: dataTableConfig.booleanOperators,
        select: dataTableConfig.selectOperators,
        multiSelect: dataTableConfig.multiSelectOperators,
    };

    return operatorMap[filterVariant] ?? dataTableConfig.textOperators;
}

export function getDefaultFilterOperator(filterVariant: FilterVariant) {
    const operators = getFilterOperators(filterVariant);

    return operators[0]?.value ?? (filterVariant === "text" ? "iLike" : "eq");
}

export function getValidFilters<TData>(
    filters: ExtendedColumnFilter<TData>[],
): ExtendedColumnFilter<TData>[] {
    return filters.filter(
        (filter) =>
            filter.operator === "isEmpty" ||
            filter.operator === "isNotEmpty" ||
            (Array.isArray(filter.value)
                ? filter.value.length > 0
                : filter.value !== "" &&
                filter.value !== null &&
                filter.value !== undefined),
    );
}

export function getValidAdvancedFilterEntries<TData>(
    entries: AdvancedFilterEntry<TData>[],
): AdvancedFilterEntry<TData>[] {
    return entries.reduce<AdvancedFilterEntry<TData>[]>((acc, entry) => {
        if (isAdvancedFilterGroup(entry)) {
            const validFilters = getValidFilters(
                entry.filters as ExtendedColumnFilter<TData>[],
            );
            if (validFilters.length === 0) {
                return acc;
            }
            acc.push({ ...entry, filters: validFilters });
            return acc;
        }

        if (
            entry.operator === "isEmpty" ||
            entry.operator === "isNotEmpty" ||
            (Array.isArray(entry.value)
                ? entry.value.length > 0
                : entry.value !== "" &&
                entry.value !== null &&
                entry.value !== undefined)
        ) {
            acc.push(entry);
        }

        return acc;
    }, []);
}
