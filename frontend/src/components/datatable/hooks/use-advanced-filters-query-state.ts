"use client";

import {
  type UseQueryStateOptions,
  useQueryState,
} from "nuqs";
import * as React from "react";

import { QUERY_KEYS } from "../lib/constants";
import { getFiltersStateParser } from "../lib/parsers";
import type { AdvancedFilterEntry } from "../lib/advanced-filter-types";

function readFiltersSearchParam(): string | null {
  if (typeof window === "undefined") return null;
  return new URLSearchParams(window.location.search).get(QUERY_KEYS.FILTERS);
}

/**
 * URL-backed advanced filter state. Column validation is deferred to the backend;
 * the parser accepts the full filter tree (including groups) from the URL.
 */
export function useAdvancedFiltersQueryState<TData>(
  options?: Omit<UseQueryStateOptions<string>, "parse">,
) {
  const filtersParser = React.useMemo(
    () => getFiltersStateParser<TData>(),
    [],
  );

  const [filters, setFilters] = useQueryState(
    QUERY_KEYS.FILTERS,
    filtersParser.withDefault([]).withOptions({
      clearOnDefault: true,
      shallow: true,
      ...options,
    }),
  );

  const restoredFromUrlRef = React.useRef(false);

  React.useEffect(() => {
    if (restoredFromUrlRef.current) return;

    const raw = readFiltersSearchParam();
    if (!raw) return;

    const reparsed = filtersParser.parse(raw);
    if (!reparsed || reparsed.length === 0) return;

    restoredFromUrlRef.current = true;

    const current = (filters ?? []) as AdvancedFilterEntry<TData>[];
    if (JSON.stringify(reparsed) !== JSON.stringify(current)) {
      void setFilters(reparsed as AdvancedFilterEntry<TData>[]);
    }
  }, [filters, filtersParser, setFilters]);

  return [filters, setFilters] as const;
}
