import { useMemo, useSyncExternalStore } from "react"

const STORAGE_KEY = "cws.k8s.showSystem"
const EVENT = "cws.k8s.showSystem"

/** Well-known cluster system namespaces (exact match). */
const SYSTEM_NAMESPACES = new Set([
  "kube-system",
  "kube-public",
  "kube-node-lease",
  "local-path-storage",
  "cattle-system",
  "cattle-fleet-system",
  "cattle-impersonation-system",
  "fleet-system",
  "ingress-nginx",
  "cert-manager",
])

/** Prefixes treated as system (platform control-plane). */
const SYSTEM_PREFIXES = ["openshift-", "kubesphere-", "tigera-"]

export function isSystemNamespace(namespace?: string | null): boolean {
  const ns = (namespace || "").trim().toLowerCase()
  if (!ns) return false
  if (SYSTEM_NAMESPACES.has(ns)) return true
  return SYSTEM_PREFIXES.some((p) => ns.startsWith(p))
}

function readShowSystem(): boolean {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw == null) return false
    return raw === "1" || raw === "true"
  } catch {
    return false
  }
}

function writeShowSystem(value: boolean) {
  try {
    localStorage.setItem(STORAGE_KEY, value ? "1" : "0")
  } catch {
    /* ignore */
  }
  window.dispatchEvent(new Event(EVENT))
}

let cached = readShowSystem()

function subscribe(onStoreChange: () => void) {
  const onChange = () => {
    cached = readShowSystem()
    onStoreChange()
  }
  window.addEventListener(EVENT, onChange)
  window.addEventListener("storage", onChange)
  return () => {
    window.removeEventListener(EVENT, onChange)
    window.removeEventListener("storage", onChange)
  }
}

function getSnapshot() {
  return cached
}

function getServerSnapshot() {
  return false
}

/** Whether system namespaces/resources should be shown (default: hidden). */
export function useShowSystemResources() {
  const showSystem = useSyncExternalStore(
    subscribe,
    getSnapshot,
    getServerSnapshot,
  )
  const setShowSystem = (value: boolean) => {
    cached = value
    writeShowSystem(value)
  }
  return [showSystem, setShowSystem] as const
}

export function filterSystemNamespacedRows<T extends { namespace?: string }>(
  rows: T[],
  showSystem: boolean,
): T[] {
  if (showSystem) return rows
  return rows.filter((row) => !isSystemNamespace(row.namespace))
}

export function filterSystemNamespaceRows<T extends { name: string }>(
  rows: T[],
  showSystem: boolean,
): T[] {
  if (showSystem) return rows
  return rows.filter((row) => !isSystemNamespace(row.name))
}

export function useK8sNamespacedRows<T extends { namespace?: string }>(
  rows: T[],
): T[] {
  const [showSystem] = useShowSystemResources()
  return useMemo(
    () => filterSystemNamespacedRows(rows, showSystem),
    [rows, showSystem],
  )
}

export function useK8sNamespaceListRows<T extends { name: string }>(
  rows: T[],
): T[] {
  const [showSystem] = useShowSystemResources()
  return useMemo(
    () => filterSystemNamespaceRows(rows, showSystem),
    [rows, showSystem],
  )
}
