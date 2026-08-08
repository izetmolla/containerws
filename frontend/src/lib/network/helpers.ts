/**
 * Small URL/route helpers used by feature-specific API modules.
 *
 * They live here (rather than in `api-service.ts`) because they're
 * pure: no axios, no auth state. Treating them as standalone makes
 * the call sites easier to read - `withAPI("/users")` is obviously
 * URL composition, while `ApiService.fetchData(...)` is obviously
 * a network call.
 */

import { getGlobalOptions, mergeInitialData } from "../globalOptions"

import { exceptedPaths } from "./env"

/**
 * Stamps a `module` segment onto a request payload so the same
 * call site can be reused across multiple module deployments. The
 * value is derived from the current pathname, falling back to the
 * caller's hint when present, and is suppressed for paths the host
 * application has registered in `exceptedPaths`.
 */
export function withModule<T>(
  params?: T & { module?: string }
): T & { module: string } {
  const module =
    window?.location?.pathname?.split("/")?.length > 1
      ? window?.location?.pathname?.split("/")?.[1]
      : (params?.module ?? "")
  if (exceptedPaths.includes(module)) {
    return { module: "", ...params } as T & { module: string }
  }
  return { module, ...params } as T & { module: string }
}

/** Generic paginated payload shape used by list endpoints. */
export interface WithPagination<T> {
  data: T[]
  pagination: {
    page: number
    limit: number
    total: number
    total_pages: number
  }
}

/**
 * Prefixes a route with `api`. Accepts both leading-slash and
 * leading-segment input so call sites can stay readable.
 */
export function withAPI(path: string): string {
  if (path.startsWith("/")) {
    return `api${path}`
  }
  return `api/${path}`
}

/**
 * Convenience for react-query's `useQuery`: returns `enabled` and
 * `initialData` already wired up from the global SSR-injected data
 * store, so a query can short-circuit when the page already has
 * the answer.
 */
export function withInitialData<T>(contentKey: string = "data", data?: T) {
  const initialData = getGlobalOptions<T>(contentKey)
  return {
    enabled: !initialData,
    initialData: mergeInitialData(initialData, data),
  }
}
