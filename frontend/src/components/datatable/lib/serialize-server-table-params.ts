import type { ServerColumnFilter, ServerTableState } from "../types/data-table";
import type { FilterItemSchema } from "./parsers";
import {
  isAdvancedFilterGroup,
  normalizeAdvancedFilterJoinOperators,
} from "./advanced-filters";
import type { AdvancedFilterEntry } from "./advanced-filter-types";

/**
 * Serializes ServerTableState into query params the Go datatable ExtractQuery expects.
 */
export function serializeServerTableParams<TData>(
  state: ServerTableState<TData>,
): Record<string, string> {
  const params: Record<string, string> = {
    "pagination[page]": String(state.pagination.page),
    "pagination[perPage]": String(state.pagination.perPage),
  };

  if (state.sorting.length > 0) {
    params.sorting = JSON.stringify(
      state.sorting.map((s) => ({ id: s.id, desc: Boolean(s.desc) })),
    );
  }

  if (state.columnFilters.length > 0) {
    params.columnFilters = JSON.stringify(
      state.columnFilters.map((f) => serializeColumnFilter(f)),
    );
  }

  if (state.filters && state.filters.length > 0) {
    const filters = normalizeAdvancedFilterJoinOperators(
      state.filters,
      state.joinOperator ?? "and",
    );
    params.filters = JSON.stringify(
      filters.map((entry, index) => serializeAdvancedFilterEntry(entry, index)),
    );
    params.filterFlag = "advancedFilters";
    if (state.joinOperator) {
      params.joinOperator = state.joinOperator;
    }
  }

  return params;
}

function serializeAdvancedFilterEntry(
  entry: AdvancedFilterEntry,
  index: number,
): Record<string, unknown> {
  if (isAdvancedFilterGroup(entry)) {
    const item: Record<string, unknown> = {
      type: "group",
      filters: entry.filters.map((filter, filterIndex) =>
        serializeAdvancedFilter(filter, filterIndex),
      ),
    };
    if (index > 0 && entry.joinOperator) {
      item.joinOperator = entry.joinOperator;
    }
    return item;
  }

  return serializeAdvancedFilter(entry, index);
}

function serializeAdvancedFilter(
  filter: FilterItemSchema,
  index: number,
): Record<string, unknown> {
  const item: Record<string, unknown> = {
    id: filter.id,
    variant: filter.variant,
    operator: filter.operator,
  };

  if (index > 0 && filter.joinOperator) {
    item.joinOperator = filter.joinOperator;
  }

  if (filter.operator === "isEmpty" || filter.operator === "isNotEmpty") {
    return item;
  }

  const value = filter.value;
  if (Array.isArray(value)) {
    item.values = value.map(String);
  } else if (value != null && value !== "") {
    item.value = String(value);
  }

  return item;
}

function serializeColumnFilter(filter: ServerColumnFilter): Record<string, unknown> {
  const item: Record<string, unknown> = { id: filter.id };
  if (Array.isArray(filter.value)) {
    item.values = filter.value.map(String);
  } else if (filter.value != null && filter.value !== "") {
    item.value = String(filter.value);
  }
  if (filter.variant) {
    item.variant = filter.variant;
  }

  const operator = defaultColumnFilterOperator(filter);
  if (operator) {
    item.operator = operator;
  }

  return item;
}

function defaultColumnFilterOperator(
  filter: ServerColumnFilter,
): string | undefined {
  switch (filter.variant) {
    case "text":
      return "iLike";
    case "multiSelect":
    case "select":
      return "inArray";
    case "dateRange":
      return "isBetween";
    default:
      return undefined;
  }
}
