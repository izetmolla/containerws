import type { JoinOperator } from "../types/table-operators";
import type { FilterItemSchema } from "./filter-schema";

export interface AdvancedFilterGroup<_TData = unknown> {
  type: "group";
  groupId: string;
  joinOperator?: JoinOperator;
  filters: FilterItemSchema[];
}

export type AdvancedFilterEntry<TData = unknown> =
  | FilterItemSchema
  | AdvancedFilterGroup<TData>;

export function isAdvancedFilterGroup<TData = unknown>(
  entry: AdvancedFilterEntry<TData>,
): entry is AdvancedFilterGroup<TData> {
  return (
    typeof entry === "object" &&
    entry != null &&
    "type" in entry &&
    entry.type === "group"
  );
}

export function getAdvancedFilterEntryKey<TData>(
  entry: AdvancedFilterEntry<TData>,
): string {
  return isAdvancedFilterGroup(entry) ? entry.groupId : entry.filterId;
}

export function countAdvancedFilterEntries<TData>(
  entries: AdvancedFilterEntry<TData>[],
): number {
  return entries.reduce((count, entry) => {
    if (isAdvancedFilterGroup(entry)) {
      return count + 1 + entry.filters.length;
    }
    return count + 1;
  }, 0);
}
