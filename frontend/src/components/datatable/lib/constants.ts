export const QUERY_KEYS = {
  PAGE: "page",
  PER_PAGE: "perPage",
  SORT: "sort",
  FILTERS: "filters",
  JOIN_OPERATOR: "joinOperator",
} as const;

/** Default debounce (ms) for filter URL updates. */
export const DEBOUNCE_MS_DEFAULT = 300;

/** Default throttle (ms) for nuqs. */
export const THROTTLE_MS_DEFAULT = 50;

/** Sentinel perPage value: server returns all rows (no LIMIT). */
export const LIST_ALL_PER_PAGE = -1;

/** Separator for array values in URL (e.g. multi-select filters). */
export const ARRAY_SEPARATOR = ",";

/** z-index for selects/popovers opened inside the filter panel (above PopoverContent z-[100]). */
export const NESTED_OVERLAY_Z_CLASS = "z-[110]";
