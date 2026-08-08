import type { JoinOperator } from "../types/data-table";
import type { FilterItemSchema } from "./parsers";
import {
  type AdvancedFilterEntry,
  type AdvancedFilterGroup,
  isAdvancedFilterGroup,
} from "./advanced-filter-types";

export function normalizeAdvancedFilterJoinOperators<
  T extends AdvancedFilterEntry,
>(entries: T[], fallbackJoin: JoinOperator = "and"): T[] {
  return entries.map((entry, index) => {
    if (isAdvancedFilterGroup(entry)) {
      const group: AdvancedFilterGroup = {
        ...entry,
        filters: normalizeConditionJoinOperators(
          entry.filters,
          fallbackJoin,
        ),
      };
      if (index === 0) {
        const { joinOperator: _removed, ...rest } = group;
        return rest as T;
      }
      return {
        ...group,
        joinOperator: entry.joinOperator ?? fallbackJoin,
      } as T;
    }

    if (index === 0) {
      const { joinOperator: _removed, ...rest } = entry as FilterItemSchema;
      return rest as T;
    }

    return {
      ...(entry as FilterItemSchema),
      joinOperator:
        (entry as FilterItemSchema).joinOperator ?? fallbackJoin,
    } as T;
  });
}

function normalizeConditionJoinOperators(
  filters: FilterItemSchema[],
  fallbackJoin: JoinOperator,
): FilterItemSchema[] {
  return filters.map((filter, index) => {
    if (index === 0) {
      const { joinOperator: _removed, ...rest } = filter;
      return rest;
    }
    return {
      ...filter,
      joinOperator: filter.joinOperator ?? fallbackJoin,
    };
  });
}

export function updateFilterInEntries<T extends AdvancedFilterEntry>(
  entries: T[],
  filterId: string,
  updates: Partial<Omit<FilterItemSchema, "filterId">>,
): T[] {
  return entries.map((entry) => {
    if (isAdvancedFilterGroup(entry)) {
      return {
        ...entry,
        filters: entry.filters.map((filter) =>
          filter.filterId === filterId ? { ...filter, ...updates } : filter,
        ),
      } as T;
    }
    if (entry.filterId === filterId) {
      return { ...entry, ...updates } as T;
    }
    return entry;
  });
}

export function removeFilterFromEntries<T extends AdvancedFilterEntry>(
  entries: T[],
  filterId: string,
): T[] {
  const result: T[] = [];

  for (const entry of entries) {
    if (isAdvancedFilterGroup(entry)) {
      const filters = entry.filters.filter((filter) => filter.filterId !== filterId);
      if (filters.length === 0) {
        continue;
      }
      result.push({ ...entry, filters } as T);
      continue;
    }

    if (entry.filterId === filterId) {
      continue;
    }

    result.push(entry);
  }

  return result;
}

export function updateGroupInEntries<T extends AdvancedFilterEntry>(
  entries: T[],
  groupId: string,
  updates: Partial<Omit<AdvancedFilterGroup, "groupId" | "type">>,
): T[] {
  return entries.map((entry) => {
    if (isAdvancedFilterGroup(entry) && entry.groupId === groupId) {
      return { ...entry, ...updates } as T;
    }
    return entry;
  });
}

export function removeGroupFromEntries<T extends AdvancedFilterEntry>(
  entries: T[],
  groupId: string,
): T[] {
  return entries.filter(
    (entry) => !(isAdvancedFilterGroup(entry) && entry.groupId === groupId),
  );
}

export function addFilterToGroup<T extends AdvancedFilterEntry>(
  entries: T[],
  groupId: string,
  filter: FilterItemSchema,
): T[] {
  return entries.map((entry) => {
    if (!isAdvancedFilterGroup(entry) || entry.groupId !== groupId) {
      return entry;
    }
    return {
      ...entry,
      filters: [...entry.filters, filter],
    } as T;
  });
}

export function createDefaultGroupFilter(
  column: { id: string; columnDef: { meta?: { variant?: FilterItemSchema["variant"] } } },
  generateId: () => string,
  getDefaultFilterOperator: (variant: FilterItemSchema["variant"]) => FilterItemSchema["operator"],
): FilterItemSchema {
  const variant = column.columnDef.meta?.variant ?? "text";
  return {
    id: column.id,
    value: "",
    variant,
    operator: getDefaultFilterOperator(variant),
    filterId: generateId(),
  };
}

export function createDefaultAdvancedFilterGroup(
  column: { id: string; columnDef: { meta?: { variant?: FilterItemSchema["variant"] } } },
  generateId: () => string,
  getDefaultFilterOperator: (variant: FilterItemSchema["variant"]) => FilterItemSchema["operator"],
  options?: { joinOperator?: JoinOperator },
): AdvancedFilterGroup {
  return {
    type: "group",
    groupId: generateId(),
    ...(options?.joinOperator && { joinOperator: options.joinOperator }),
    filters: [createDefaultGroupFilter(column, generateId, getDefaultFilterOperator)],
  };
}

export function getLastRemovableFilterId<TData>(
  entries: AdvancedFilterEntry<TData>[],
): string | null {
  if (entries.length === 0) return null;

  const last = entries[entries.length - 1];
  if (isAdvancedFilterGroup(last)) {
    const lastFilter = last.filters[last.filters.length - 1];
    return lastFilter?.filterId ?? last.groupId;
  }

  return last.filterId;
}

export function flattenAdvancedFilterConditions(
  entries: AdvancedFilterEntry[],
): FilterItemSchema[] {
  const result: FilterItemSchema[] = [];

  for (const entry of entries) {
    if (isAdvancedFilterGroup(entry)) {
      result.push(...entry.filters);
      continue;
    }
    result.push(entry);
  }

  return result;
}
