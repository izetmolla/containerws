import { useSyncExternalStore } from "react"

export const AUTO_REFRESH_STORAGE_KEY = "containerws-auto-refresh-ms"
export const AUTO_REFRESH_EVENT = "containerws:auto-refresh-ms"

export const AUTO_REFRESH_OPTIONS = [
  { value: 2000, label: "2 seconds" },
  { value: 3000, label: "3 seconds" },
  { value: 5000, label: "5 seconds" },
  { value: 10000, label: "10 seconds" },
  { value: 30000, label: "30 seconds" },
  { value: 0, label: "Off" },
] as const

export const DEFAULT_AUTO_REFRESH_MS = 3000

const ALLOWED = new Set(AUTO_REFRESH_OPTIONS.map((o) => o.value))

export function readAutoRefreshMs(): number {
  if (typeof window === "undefined") return DEFAULT_AUTO_REFRESH_MS
  try {
    const raw = localStorage.getItem(AUTO_REFRESH_STORAGE_KEY)
    if (raw == null || raw === "") return DEFAULT_AUTO_REFRESH_MS
    const n = Number(raw)
    if (!Number.isFinite(n) || !ALLOWED.has(n as (typeof AUTO_REFRESH_OPTIONS)[number]["value"])) {
      return DEFAULT_AUTO_REFRESH_MS
    }
    return n
  } catch {
    return DEFAULT_AUTO_REFRESH_MS
  }
}

export function writeAutoRefreshMs(ms: number): number {
  const next = ALLOWED.has(ms as (typeof AUTO_REFRESH_OPTIONS)[number]["value"])
    ? ms
    : DEFAULT_AUTO_REFRESH_MS
  try {
    localStorage.setItem(AUTO_REFRESH_STORAGE_KEY, String(next))
  } catch {
    // ignore quota / private mode
  }
  if (typeof window !== "undefined") {
    window.dispatchEvent(
      new CustomEvent(AUTO_REFRESH_EVENT, { detail: next })
    )
  }
  return next
}

/** Live auto-refresh interval (ms). 0 = off. Synced via localStorage + events. */
export function useAutoRefreshMs(): number {
  return useSyncExternalStore(
    (onStoreChange) => {
      const onCustom = () => onStoreChange()
      const onStorage = (e: StorageEvent) => {
        if (e.key === AUTO_REFRESH_STORAGE_KEY) onStoreChange()
      }
      window.addEventListener(AUTO_REFRESH_EVENT, onCustom)
      window.addEventListener("storage", onStorage)
      return () => {
        window.removeEventListener(AUTO_REFRESH_EVENT, onCustom)
        window.removeEventListener("storage", onStorage)
      }
    },
    readAutoRefreshMs,
    () => DEFAULT_AUTO_REFRESH_MS
  )
}

export function autoRefreshInterval(ms: number): number | false {
  return ms > 0 ? ms : false
}
