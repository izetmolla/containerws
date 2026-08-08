import { create } from "zustand"
import { createJSONStorage, persist, type StateStorage } from "zustand/middleware"

export type CloudShellTab = {
  id: string
  title: string
  /** Backend PTY session id — used to resume after reload / reopen. */
  sessionId: string | null
  reconnectToken: number
}

type CloudShellPersisted = {
  height: number
  previousHeight: number
  tabs: Array<{
    id: string
    title: string
    sessionId: string | null
  }>
  activeTabId: string | null
  open: boolean
}

type CloudShellState = {
  /** False until localStorage rehydration finishes — do not open WS before this. */
  hasHydrated: boolean
  open: boolean
  height: number
  maximized: boolean
  previousHeight: number
  tabs: CloudShellTab[]
  activeTabId: string | null
  setHasHydrated: (value: boolean) => void
  openShell: () => void
  closeShell: () => void
  toggleShell: () => void
  setHeight: (height: number) => void
  minimize: () => void
  toggleMaximize: () => void
  addTab: () => void
  closeTab: (id: string) => CloudShellTab | null
  setActiveTab: (id: string) => void
  reconnectTab: (id: string) => void
  newShellForTab: (id: string) => string | null
  setTabSessionId: (id: string, sessionId: string | null) => void
  clearTabSession: (id: string) => void
  /** Replace tabs from durable DB-backed /cloudshell/sessions list. */
  hydrateFromServer: (
    sessions: Array<{ id: string; title?: string; alive?: boolean }>
  ) => void
  resetShellCache: () => void
}

const MIN_HEIGHT = 160
const HEADER_ONLY = 40
const DEFAULT_HEIGHT = 320
/** Bump to invalidate corrupt client caches that caused white-screen loops. */
const STORAGE_KEY = "containerws-cloudshell-v3"

function clampHeight(h: number) {
  if (typeof window === "undefined") {
    return Math.max(HEADER_ONLY, Math.round(h))
  }
  const max = Math.max(MIN_HEIGHT, Math.floor(window.innerHeight))
  return Math.min(max, Math.max(HEADER_ONLY, Math.round(h)))
}

function fullViewportHeight() {
  if (typeof window === "undefined") return DEFAULT_HEIGHT
  return Math.max(MIN_HEIGHT, Math.floor(window.innerHeight))
}

function newTab(index: number): CloudShellTab {
  const id =
    typeof crypto !== "undefined" && "randomUUID" in crypto
      ? crypto.randomUUID()
      : `tab-${Date.now()}-${index}`
  return {
    id,
    title: `session-${index}`,
    sessionId: null,
    reconnectToken: 0,
  }
}

function normalizeTabs(
  tabs: CloudShellPersisted["tabs"] | CloudShellTab[] | undefined
): CloudShellTab[] {
  if (!Array.isArray(tabs) || tabs.length === 0) {
    return [newTab(1)]
  }
  return tabs.map((t, i) => ({
    id: t.id || `tab-${i + 1}`,
    title: t.title || `session-${i + 1}`,
    sessionId: t.sessionId ?? null,
    reconnectToken: 0,
  }))
}

/** localStorage that never throws — corrupt JSON is wiped. */
const safeLocalStorage: StateStorage = {
  getItem: (name) => {
    try {
      return localStorage.getItem(name)
    } catch {
      return null
    }
  },
  setItem: (name, value) => {
    try {
      localStorage.setItem(name, value)
    } catch {
      // Quota / private mode — ignore.
    }
  },
  removeItem: (name) => {
    try {
      localStorage.removeItem(name)
    } catch {
      // ignore
    }
  },
}

function clearLegacyCloudShellKeys() {
  if (typeof window === "undefined") return
  try {
    for (const key of Object.keys(localStorage)) {
      if (
        key === "containerws-cloudshell-v2" ||
        key.startsWith("containerws-cloudshell-v2")
      ) {
        localStorage.removeItem(key)
      }
    }
  } catch {
    // ignore
  }
}

clearLegacyCloudShellKeys()

export const useCloudShellStore = create<CloudShellState>()(
  persist(
    (set, get) => ({
      hasHydrated: false,
      open: false,
      height: DEFAULT_HEIGHT,
      maximized: false,
      previousHeight: DEFAULT_HEIGHT,
      tabs: [newTab(1)],
      activeTabId: null,

      setHasHydrated: (value) => set({ hasHydrated: value }),

      openShell: () =>
        set((s) => ({
          open: true,
          height: s.maximized
            ? fullViewportHeight()
            : Math.max(s.height, DEFAULT_HEIGHT),
        })),

      closeShell: () => set({ open: false, maximized: false }),

      toggleShell: () => {
        const { open, openShell, closeShell } = get()
        if (open) closeShell()
        else openShell()
      },

      setHeight: (height) =>
        set({
          height: clampHeight(height),
          maximized: false,
        }),

      minimize: () =>
        set((s) => ({
          previousHeight: s.height > HEADER_ONLY ? s.height : s.previousHeight,
          height: HEADER_ONLY,
          maximized: false,
        })),

      toggleMaximize: () =>
        set((s) => {
          if (s.maximized) {
            return {
              maximized: false,
              height: clampHeight(
                Math.max(s.previousHeight, DEFAULT_HEIGHT)
              ),
            }
          }
          const current =
            s.height > HEADER_ONLY ? s.height : s.previousHeight || DEFAULT_HEIGHT
          return {
            maximized: true,
            previousHeight: current,
            height: fullViewportHeight(),
          }
        }),

      addTab: () =>
        set((s) => {
          const tab = newTab(s.tabs.length + 1)
          return {
            open: true,
            tabs: [...s.tabs, tab],
            activeTabId: tab.id,
            height: Math.max(s.height, DEFAULT_HEIGHT),
          }
        }),

      closeTab: (id) => {
        const state = get()
        const closing = state.tabs.find((t) => t.id === id) ?? null
        if (state.tabs.length <= 1) {
          const replacement = newTab(1)
          set({
            open: false,
            tabs: [replacement],
            activeTabId: replacement.id,
          })
          return closing
        }
        const tabs = state.tabs.filter((t) => t.id !== id)
        const activeTabId =
          state.activeTabId === id
            ? (tabs[tabs.length - 1]?.id ?? null)
            : state.activeTabId
        set({ tabs, activeTabId })
        return closing
      },

      setActiveTab: (id) => set({ activeTabId: id, open: true }),

      reconnectTab: (id) =>
        set((s) => ({
          tabs: s.tabs.map((t) =>
            t.id === id ? { ...t, reconnectToken: t.reconnectToken + 1 } : t
          ),
        })),

      newShellForTab: (id) => {
        const prev = get().tabs.find((t) => t.id === id)?.sessionId ?? null
        set((s) => ({
          tabs: s.tabs.map((t) =>
            t.id === id
              ? {
                  ...t,
                  sessionId: null,
                  reconnectToken: t.reconnectToken + 1,
                }
              : t
          ),
        }))
        return prev
      },

      setTabSessionId: (id, sessionId) =>
        set((s) => ({
          tabs: s.tabs.map((t) =>
            t.id === id
              ? {
                  ...t,
                  sessionId: sessionId ?? t.sessionId ?? null,
                }
              : t
          ),
        })),

      clearTabSession: (id) =>
        set((s) => ({
          tabs: s.tabs.map((t) =>
            t.id === id
              ? {
                  ...t,
                  sessionId: null,
                  reconnectToken: t.reconnectToken + 1,
                }
              : t
          ),
        })),

      hydrateFromServer: (sessions) => {
        const alive = (sessions ?? []).filter((s) => s?.id && s.alive !== false)
        if (alive.length === 0) {
          // Keep UI chrome; clear stale resume ids so WS opens a fresh PTY.
          set((s) => ({
            tabs: s.tabs.map((t) => ({
              ...t,
              sessionId: null,
            })),
            hasHydrated: true,
          }))
          return
        }
        const tabs: CloudShellTab[] = alive.map((s, i) => ({
          id: s.id,
          title: s.title?.trim() || `session-${i + 1}`,
          sessionId: s.id,
          reconnectToken: 0,
        }))
        set((s) => ({
          tabs,
          activeTabId:
            s.activeTabId && tabs.some((t) => t.id === s.activeTabId)
              ? s.activeTabId
              : tabs[0]?.id ?? null,
          hasHydrated: true,
        }))
      },

      resetShellCache: () => {
        try {
          localStorage.removeItem(STORAGE_KEY)
        } catch {
          // ignore
        }
        const tab = newTab(1)
        set({
          hasHydrated: true,
          open: false,
          height: DEFAULT_HEIGHT,
          maximized: false,
          previousHeight: DEFAULT_HEIGHT,
          tabs: [tab],
          activeTabId: tab.id,
        })
      },
    }),
    {
      name: STORAGE_KEY,
      storage: createJSONStorage(() => safeLocalStorage),
      version: 3,
      partialize: (s): CloudShellPersisted => ({
        height: s.height,
        previousHeight: s.previousHeight,
        open: s.open,
        activeTabId: s.activeTabId,
        tabs: s.tabs.map((t) => ({
          id: t.id,
          title: t.title,
          // Prefer server as source of truth for session ids — still cache for offline resume.
          sessionId: t.sessionId ?? null,
        })),
      }),
      merge: (persisted, current) => {
        try {
          const p = (persisted ?? {}) as Partial<CloudShellPersisted>
          const tabs = normalizeTabs(p.tabs)
          let activeTabId = p.activeTabId ?? tabs[0]?.id ?? null
          if (activeTabId && !tabs.some((t) => t.id === activeTabId)) {
            activeTabId = tabs[0]?.id ?? null
          }
          return {
            ...current,
            height: typeof p.height === "number" ? p.height : current.height,
            previousHeight:
              typeof p.previousHeight === "number"
                ? p.previousHeight
                : current.previousHeight,
            open: Boolean(p.open),
            tabs,
            activeTabId,
            maximized: false,
            hasHydrated: false,
          }
        } catch {
          return { ...current, hasHydrated: false }
        }
      },
      onRehydrateStorage: () => (state, error) => {
        if (error) {
          console.warn("cloudshell persist rehydrate failed", error)
          try {
            localStorage.removeItem(STORAGE_KEY)
          } catch {
            // ignore
          }
        }
        // Use the rehydrated state's setter — do not touch `useCloudShellStore`
        // here or create() hits a TDZ (Cannot access before initialization).
        if (state) {
          state.setHasHydrated(true)
          return
        }
        queueMicrotask(() => {
          useCloudShellStore.setState({ hasHydrated: true })
        })
      },
    }
  )
)

/** Call from React — marks hydrated quickly without aggressive rehydrate races. */
export function ensureCloudShellHydrated() {
  if (useCloudShellStore.getState().hasHydrated) {
    return () => {}
  }
  const mark = () => {
    if (!useCloudShellStore.getState().hasHydrated) {
      useCloudShellStore.setState({ hasHydrated: true })
    }
  }
  const unsub = useCloudShellStore.persist.onFinishHydration(mark)
  if (useCloudShellStore.persist.hasHydrated()) {
    mark()
  }
  const timeout = window.setTimeout(mark, 100)
  return () => {
    unsub()
    window.clearTimeout(timeout)
  }
}

export const CLOUD_SHELL_MIN_HEIGHT = MIN_HEIGHT
export const CLOUD_SHELL_HEADER_HEIGHT = HEADER_ONLY
export const CLOUD_SHELL_DEFAULT_HEIGHT = DEFAULT_HEIGHT
export const CLOUD_SHELL_STORAGE_KEY = STORAGE_KEY
