/**
 * Runtime environment helpers. Kept apart from the axios client so
 * tests can import them without booting the full network stack and
 * so they can be tweaked in one place when the embedding strategy
 * changes (e.g. when an SSR shell starts injecting `__GLOBAL_DATA__`).
 */

/**
 * Path prefixes that should never have the auto-derived `module`
 * segment inserted by `withModule`. Currently empty; exported so
 * callers can mutate it during bootstrap without a bigger API.
 */
export const exceptedPaths: string[] = []

/**
 * The module identifier the SSR shell stamps onto the page. Apps
 * served at `/` (the default) report `"app"`; nested deployments
 * report their mount path. Falls back to `"/"` when the marker is
 * missing so calls during pre-render don't crash.
 */
export function getModuleName(): string {
  return document.getElementById("__GLOBAL_DATA__")?.dataset?.app ?? "/"
}

/**
 * Builds the absolute base URL the API client should use. The "app"
 * module lives at the origin root; everything else is sandboxed
 * under `/<module>`.
 */
export function baseApiURL(): string {
  return `${window.location.protocol}//${window.location.host}/api`
}

/**
 * `import.meta.env.DEV` access wrapped so it can't blow up in a CJS
 * test runner that doesn't expose `import.meta`. The function form
 * also lets tests stub it out via `vi.spyOn` if they need to.
 */
export function isDev(): boolean {
  try {
    return Boolean(
      (import.meta as unknown as { env?: { DEV?: boolean } }).env?.DEV
    )
  } catch {
    return false
  }
}
