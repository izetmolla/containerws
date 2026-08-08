import { useCallback, useLayoutEffect, useRef } from "react"

/**
 * Stable callback that always invokes the latest `fn` (event-handler pattern).
 * Prefer this over `useEffectEvent` when the function must be callable from
 * DOM/event handlers — eslint-plugin-react-hooks only allows Effect Events
 * inside Effects.
 */
export function useEvent<T extends (...args: never[]) => unknown>(fn: T): T {
  const ref = useRef(fn)
  useLayoutEffect(() => {
    ref.current = fn
  })
  return useCallback(((...args: Parameters<T>) => ref.current(...args)) as T, [])
}
