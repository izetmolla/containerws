import { createParser } from "nuqs/server";
import { z } from "zod";

import type { AdvancedFilterEntry } from "./advanced-filter-types";
import {
  filterEntrySchema,
  sortingItemSchema,
} from "./filter-schema";
import type { ExtendedColumnSort } from "../types/table-sort";

export type {
  FilterEntrySchema,
  FilterGroupSchema,
  FilterItemSchema,
} from "./filter-schema";

function resolveValidColumnKeys(
  columnIds?: string[] | Set<string>,
): Set<string> | null {
  if (!columnIds) return null;
  const set = columnIds instanceof Set ? columnIds : new Set(columnIds);
  return set.size > 0 ? set : null;
}

export const getSortingStateParser = <TData>(
  columnIds?: string[] | Set<string>,
) => {
  const validKeys = resolveValidColumnKeys(columnIds);

  return createParser({
    parse: (value) => {
      try {
        const parsed = JSON.parse(value);
        const result = z.array(sortingItemSchema).safeParse(parsed);

        if (!result.success) return null;

        if (validKeys && result.data.some((item) => !validKeys.has(item.id))) {
          return null;
        }

        return result.data as ExtendedColumnSort<TData>[];
      } catch {
        return null;
      }
    },
    serialize: (value) => JSON.stringify(value),
    eq: (a, b) =>
      a.length === b.length &&
      a.every(
        (item, index) =>
          item.id === b[index]?.id && item.desc === b[index]?.desc,
      ),
  });
};

export const getFiltersStateParser = <TData>(
  _columnIds?: string[] | Set<string>,
) => {
  return createParser({
    parse: (value) => {
      try {
        const parsed = JSON.parse(value);
        const result = z.array(filterEntrySchema).safeParse(parsed);

        if (!result.success) return null;

        return result.data as AdvancedFilterEntry<TData>[];
      } catch {
        return null;
      }
    },
    serialize: (value) => JSON.stringify(value),
    eq: (a, b) => JSON.stringify(a) === JSON.stringify(b),
  });
};
