import { dataTableConfig } from "../config/data-table";

/** Set of valid filter variant strings (text, number, range, date, etc.). */
export const VALID_FILTER_VARIANTS = new Set<string>(
  dataTableConfig.filterVariants,
);

/**
 * Returns true if the value is a supported filter variant.
 * Use this to avoid rendering a filter for columns with missing or invalid variant.
 */
export function hasValidFilterVariant(variant: unknown): boolean {
  return typeof variant === "string" && VALID_FILTER_VARIANTS.has(variant);
}
