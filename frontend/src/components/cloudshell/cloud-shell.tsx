"use client"

import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type PointerEvent as ReactPointerEvent,
} from "react"

import {
  getCliSession,
  killCliSession,
  listCliSessions,
} from "@/components/cloudshell/api"
import { cn } from "@/lib/utils"

import { CloudShellHeader } from "./header"
import { SpecialKeysBar } from "./special-keys-bar"
import { CLOUDSHELL_TOGGLE_KEYS_EVENT } from "./special-keys"
import {
  CLOUD_SHELL_DEFAULT_HEIGHT,
  CLOUD_SHELL_HEADER_HEIGHT,
  ensureCloudShellHydrated,
  useCloudShellStore,
} from "./store"
import { CloudShellTerminal } from "./terminal-session"
import { useVisualViewport } from "./use-visual-viewport"

export type CloudShellVariant = "dock" | "fullscreen"

type CloudShellProps = {
  /** `dock` = bottom panel; `fullscreen` = fill the viewport (e.g. /shell). */
  variant?: CloudShellVariant
  /** Called when the user closes the shell (fullscreen usually navigates away). */
  onRequestClose?: () => void
}

async function killSessionQuiet(sessionId?: string | null) {
  if (!sessionId) return
  try {
    await killCliSession(sessionId)
  } catch {
    // Session may already be gone.
  }
}

export function CloudShell({
  variant = "dock",
  onRequestClose,
}: CloudShellProps) {
  const hasHydrated = useCloudShellStore((s) => s.hasHydrated)
  const open = useCloudShellStore((s) => s.open)
  const openShell = useCloudShellStore((s) => s.openShell)
  const height = useCloudShellStore((s) => s.height)
  const maximized = useCloudShellStore((s) => s.maximized)
  const tabs = useCloudShellStore((s) => s.tabs)
  const activeTabId = useCloudShellStore((s) => s.activeTabId)
  const activePtySessionId = useCloudShellStore((s) => {
    const tab = s.tabs.find((t) => t.id === s.activeTabId)
    return tab?.sessionId ?? null
  })
  const closeShell = useCloudShellStore((s) => s.closeShell)
  const setHeight = useCloudShellStore((s) => s.setHeight)
  const minimize = useCloudShellStore((s) => s.minimize)
  const toggleMaximize = useCloudShellStore((s) => s.toggleMaximize)
  const addTab = useCloudShellStore((s) => s.addTab)
  const closeTab = useCloudShellStore((s) => s.closeTab)
  const setActiveTab = useCloudShellStore((s) => s.setActiveTab)
  const reconnectTab = useCloudShellStore((s) => s.reconnectTab)
  const newShellForTab = useCloudShellStore((s) => s.newShellForTab)
  const previousHeight = useCloudShellStore((s) => s.previousHeight)

  const fullscreen = variant === "fullscreen"
  const dragRef = useRef<{ startY: number; startHeight: number } | null>(null)
  const minimized =
    !fullscreen && height <= CLOUD_SHELL_HEADER_HEIGHT + 4
  const [editorPath, setEditorPath] = useState<string | null>(null)
  const [specialKeysOpen, setSpecialKeysOpen] = useState(false)
  const vv = useVisualViewport()

  useEffect(() => {
    const onToggleKeys = () => setSpecialKeysOpen((v) => !v)
    window.addEventListener(CLOUDSHELL_TOGGLE_KEYS_EVENT, onToggleKeys)
    return () =>
      window.removeEventListener(CLOUDSHELL_TOGGLE_KEYS_EVENT, onToggleKeys)
  }, [])
  // On phones, when the soft keyboard opens, fill the visual viewport so the
  // cursor stays visible above the keyboard (iOS does not shrink fixed bottoms).
  const mobileKeyboardMode =
    !fullscreen && open && !minimized && vv.isCompact && vv.keyboardOpen

  // Persist rehydration can race with first paint / HMR — never stay stuck.
  useEffect(() => ensureCloudShellHydrated(), [])

  // Lock document scroll while the docked shell is open on compact devices.
  useEffect(() => {
    if (fullscreen || !open || !vv.isCompact) return
    const prevOverflow = document.body.style.overflow
    const prevTouch = document.body.style.touchAction
    document.body.style.overflow = "hidden"
    document.body.style.touchAction = "none"
    return () => {
      document.body.style.overflow = prevOverflow
      document.body.style.touchAction = prevTouch
    }
  }, [fullscreen, open, vv.isCompact])

  // Refit xterm after keyboard / visualViewport changes.
  useEffect(() => {
    if (!open && !fullscreen) return
    const id = window.setTimeout(() => {
      window.dispatchEvent(new Event("resize"))
    }, 50)
    return () => window.clearTimeout(id)
  }, [
    open,
    fullscreen,
    mobileKeyboardMode,
    vv.height,
    vv.bottomInset,
    vv.offsetTop,
    height,
  ])

  // Fallback folder for Open Editor when live PTY cwd is unavailable.
  useEffect(() => {
    if (!hasHydrated) return
    let cancelled = false
    void (async () => {
      try {
        const [sessionsRes, sessionRes] = await Promise.all([
          listCliSessions().catch(() => null),
          getCliSession().catch(() => null),
        ])
        if (cancelled) return
        const rows = Array.isArray(sessionsRes?.data) ? sessionsRes.data : []
        const ptyId = activePtySessionId || activeTabId
        const active = rows.find((s) => s.id === ptyId)
        const cwd =
          active?.cwd?.trim() ||
          sessionRes?.data?.user?.cwd?.trim() ||
          sessionRes?.data?.user?.home_dir?.trim() ||
          "/workspace"
        setEditorPath(cwd)
      } catch {
        if (!cancelled) setEditorPath("/workspace")
      }
    })()
    return () => {
      cancelled = true
    }
  }, [hasHydrated, activeTabId, activePtySessionId, open])

  // Durable sessions from DB (owned by logged-in user).
  useEffect(() => {
    if (!hasHydrated) return
    let cancelled = false
    void listCliSessions()
      .then((res) => {
        if (cancelled) return
        const rows = Array.isArray(res?.data) ? res.data : []
        useCloudShellStore.getState().hydrateFromServer(
          rows.map((s) => ({
            id: s.id,
            title: s.title,
            alive: s.alive,
          }))
        )
      })
      .catch(() => {
        // API unavailable — keep local tabs; still show the shell.
        if (!cancelled) {
          useCloudShellStore.getState().setHasHydrated(true)
        }
      })
    return () => {
      cancelled = true
    }
  }, [hasHydrated])

  useEffect(() => {
    if (fullscreen && hasHydrated) openShell()
  }, [fullscreen, hasHydrated, openShell])

  const onPointerMove = useCallback(
    (e: PointerEvent) => {
      const drag = dragRef.current
      if (!drag) return
      const next = drag.startHeight + (drag.startY - e.clientY)
      setHeight(next)
    },
    [setHeight]
  )

  const endDragListenerRef = useRef<(() => void) | null>(null)

  const endDrag = useCallback(() => {
    dragRef.current = null
    document.body.style.cursor = ""
    document.body.style.userSelect = ""
    window.removeEventListener("pointermove", onPointerMove)
    const onUp = endDragListenerRef.current
    if (onUp) window.removeEventListener("pointerup", onUp)
    endDragListenerRef.current = null
    window.dispatchEvent(new Event("resize"))
  }, [onPointerMove])

  const startDrag = (e: ReactPointerEvent<HTMLDivElement>) => {
    if (fullscreen) return
    e.preventDefault()
    dragRef.current = { startY: e.clientY, startHeight: height }
    document.body.style.cursor = "ns-resize"
    document.body.style.userSelect = "none"
    const onUp = () => endDrag()
    endDragListenerRef.current = onUp
    window.addEventListener("pointermove", onPointerMove)
    window.addEventListener("pointerup", onUp)
  }

  useEffect(() => {
    return () => {
      window.removeEventListener("pointermove", onPointerMove)
      const onUp = endDragListenerRef.current
      if (onUp) window.removeEventListener("pointerup", onUp)
      endDragListenerRef.current = null
    }
  }, [onPointerMove])

  useEffect(() => {
    if (fullscreen) return
    const onKey = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key.toLowerCase() === "b") {
        e.preventDefault()
        const next = !useCloudShellStore.getState().open
        if (next) {
          useCloudShellStore.getState().openShell()
          return
        }
        useCloudShellStore.getState().closeShell()
      }
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [fullscreen])

  const handleCloseTab = useCallback(
    (id: string) => {
      const closed = closeTab(id)
      void killSessionQuiet(closed?.sessionId)
    },
    [closeTab]
  )

  const handleNewShell = useCallback(() => {
    if (!activeTabId) return
    const prev = newShellForTab(activeTabId)
    void killSessionQuiet(prev)
  }, [activeTabId, newShellForTab])

  const handleClose = useCallback(() => {
    if (onRequestClose) {
      onRequestClose()
      return
    }
    closeShell()
  }, [closeShell, onRequestClose])

  if (!hasHydrated) {
    if (!fullscreen && !open) return null
    return (
      <div
        className={cn(
          "z-50 flex items-center justify-center bg-black text-sm text-zinc-400",
          fullscreen
            ? "fixed inset-0"
            : "fixed inset-x-0 bottom-0 border-t border-white/10"
        )}
        style={
          fullscreen
            ? undefined
            : {
                height: Math.max(height, CLOUD_SHELL_DEFAULT_HEIGHT),
                bottom: 0,
              }
        }
      >
        Restoring terminal sessions…
      </div>
    )
  }

  if (!fullscreen && !open) {
    return null
  }

  const dockStyle = (() => {
    if (fullscreen) {
      if (vv.isCompact && vv.keyboardOpen) {
        return {
          top: vv.offsetTop,
          left: 0,
          right: 0,
          bottom: "auto" as const,
          height: Math.max(vv.height, CLOUD_SHELL_HEADER_HEIGHT + 80),
          maxHeight: vv.height,
        }
      }
      return undefined
    }
    if (mobileKeyboardMode) {
      // Pin exactly to the visible viewport above the soft keyboard.
      return {
        top: vv.offsetTop,
        bottom: "auto" as const,
        height: Math.max(vv.height, CLOUD_SHELL_HEADER_HEIGHT + 80),
        maxHeight: vv.height,
      }
    }
    // Keep the panel above any residual visualViewport inset (Android / some iOS).
    // Maximized: stretch bottom → top (full visual viewport).
    if (maximized) {
      return {
        top: vv.offsetTop,
        bottom: "auto" as const,
        height: Math.max(vv.height, CLOUD_SHELL_HEADER_HEIGHT + 80),
        maxHeight: vv.height,
      }
    }
    const maxVisible = Math.max(
      CLOUD_SHELL_HEADER_HEIGHT + 80,
      vv.height - 8
    )
    return {
      bottom: vv.bottomInset,
      height: Math.min(height, maxVisible),
      maxHeight: maxVisible,
    }
  })()

  return (
    <div
      className={cn(
        "z-50 flex flex-col border-white/10 bg-black",
        fullscreen && !(vv.isCompact && vv.keyboardOpen)
          ? "fixed inset-0 border-0"
          : fullscreen
            ? "fixed inset-x-0 border-0"
            : "fixed inset-x-0 border-t shadow-[0_-8px_32px_rgba(0,0,0,0.45)]",
        (mobileKeyboardMode || (fullscreen && vv.keyboardOpen)) &&
          "overscroll-none"
      )}
      style={
        fullscreen && !(vv.isCompact && vv.keyboardOpen)
          ? undefined
          : dockStyle
      }
      role="region"
      aria-label="Cloud Shell"
    >
      {!fullscreen && !mobileKeyboardMode ? (
        <div
          className="absolute inset-x-0 top-0 z-20 h-2 cursor-ns-resize touch-none"
          onPointerDown={startDrag}
          onDoubleClick={() => {
            if (maximized) {
              setHeight(previousHeight || CLOUD_SHELL_DEFAULT_HEIGHT)
            } else {
              toggleMaximize()
            }
          }}
          title="Drag to resize"
        />
      ) : null}

      <CloudShellHeader
        tabs={tabs}
        activeTabId={activeTabId}
        maximized={fullscreen || maximized}
        minimized={minimized}
        fullscreen={fullscreen}
        editorPath={editorPath}
        specialKeysOpen={specialKeysOpen}
        onToggleSpecialKeys={() => setSpecialKeysOpen((v) => !v)}
        onSelectTab={setActiveTab}
        onCloseTab={handleCloseTab}
        onAddTab={addTab}
        onMinimize={() => {
          if (fullscreen) return
          if (minimized) {
            setHeight(previousHeight || CLOUD_SHELL_DEFAULT_HEIGHT)
          } else {
            minimize()
          }
        }}
        onToggleMaximize={() => {
          if (fullscreen) return
          toggleMaximize()
        }}
        onClose={handleClose}
        onPopOut={() => window.open("/shell", "_blank", "noopener,noreferrer")}
        onReconnect={() => {
          if (activeTabId) reconnectTab(activeTabId)
        }}
        onNewShell={handleNewShell}
      />

      <div
        className={cn(
          "min-h-0 flex-1 overflow-hidden bg-black",
          minimized && "hidden"
        )}
      >
        {tabs.map((tab) => (
          <CloudShellTerminal
            key={tab.id}
            tabId={tab.id}
            title={tab.title}
            sessionId={tab.sessionId}
            reconnectToken={tab.reconnectToken}
            active={tab.id === activeTabId}
          />
        ))}
      </div>

      {!minimized ? (
        <SpecialKeysBar
          open={specialKeysOpen}
          onClose={() => setSpecialKeysOpen(false)}
        />
      ) : null}
    </div>
  )
}
