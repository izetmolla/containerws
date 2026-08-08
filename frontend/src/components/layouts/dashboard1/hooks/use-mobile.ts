import * as React from "react"

const MOBILE_BREAKPOINT = 768
const TABLET_BREAKPOINT = 1200

function subscribeToMediaQuery(query: string, callback: () => void) {
  const mql = window.matchMedia(query)
  mql.addEventListener("change", callback)
  return () => mql.removeEventListener("change", callback)
}

export function useIsMobile() {
  return React.useSyncExternalStore(
    (callback) =>
      subscribeToMediaQuery(`(max-width: ${MOBILE_BREAKPOINT - 1}px)`, callback),
    () => window.matchMedia(`(max-width: ${MOBILE_BREAKPOINT - 1}px)`).matches,
    () => false
  )
}

/**
 * Returns `undefined` during SSR so consumers can avoid overwriting persisted
 * UI state before the viewport is known. On the client, returns the live match.
 */
export function useIsTablet() {
  return React.useSyncExternalStore(
    (callback) =>
      subscribeToMediaQuery(`(max-width: ${TABLET_BREAKPOINT - 1}px)`, callback),
    () => window.matchMedia(`(max-width: ${TABLET_BREAKPOINT - 1}px)`).matches,
    () => undefined
  )
}
