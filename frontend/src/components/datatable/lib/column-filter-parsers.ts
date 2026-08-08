import type { Parser, UseQueryStateOptions } from "nuqs";
import { parseAsArrayOf, parseAsString } from "nuqs";

import { ARRAY_SEPARATOR } from "./constants";
import type { FilterVariant } from "../types/data-table";

type FilterableColumnMeta = {
  options?: unknown[];
  variant?: FilterVariant;
};

type QueryStateOptions = Omit<UseQueryStateOptions<string>, "parse">;

export function getColumnFilterParser(
  column: { meta?: FilterableColumnMeta },
  queryStateOptions?: QueryStateOptions,
): Parser<string> | Parser<string[]> {
  const variant = column.meta?.variant;

  if (column.meta?.options?.length || variant === "multiSelect") {
    const parser = parseAsArrayOf(parseAsString, ARRAY_SEPARATOR);
    return queryStateOptions ? parser.withOptions(queryStateOptions) : parser;
  }

  if (variant === "dateRange") {
    const parser = parseAsArrayOf(parseAsString, ARRAY_SEPARATOR);
    return queryStateOptions ? parser.withOptions(queryStateOptions) : parser;
  }

  return queryStateOptions
    ? parseAsString.withOptions(queryStateOptions)
    : parseAsString;
}
