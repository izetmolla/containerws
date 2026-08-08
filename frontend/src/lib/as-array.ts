/** Shared empty array — reuse so `?? EMPTY_ARRAY` stays referentially stable. */
const EMPTY_ARRAY: never[] = []

/** Return `value` or a stable empty array when nullish. */
export function asArray<T>(value: T[] | null | undefined): T[] {
  return value ?? (EMPTY_ARRAY as T[])
}
