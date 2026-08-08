"use client"

import { useCallback, useEffect, useRef } from "react"

import { LiveTerminal } from "@/components/cloudshell/terminal/live-terminal"
import { cn } from "@/lib/utils"

import { useCloudShellStore } from "./store"

type CloudShellTerminalProps = {
  tabId: string
  title: string
  sessionId?: string | null
  reconnectToken: number
  active: boolean
  className?: string
}

/** Only the active tab holds a WebSocket; inactive tabs detach and resume later. */
export function CloudShellTerminal({
  tabId,
  title,
  sessionId,
  reconnectToken,
  active,
  className,
}: CloudShellTerminalProps) {
  const wrapRef = useRef<HTMLDivElement>(null)
  const setTabSessionId = useCloudShellStore((s) => s.setTabSessionId)
  const clearTabSession = useCloudShellStore((s) => s.clearTabSession)

  const onReady = useCallback(
    (info: { sessionId?: string }) => {
      if (info.sessionId) {
        setTabSessionId(tabId, info.sessionId)
      }
    },
    [setTabSessionId, tabId]
  )

  // Prefer live store value so resume uses the latest persisted session id.
  const liveSessionId = useCloudShellStore(
    (s) => s.tabs.find((t) => t.id === tabId)?.sessionId ?? sessionId ?? null
  )

  const onSessionLost = useCallback(() => {
    clearTabSession(tabId)
  }, [clearTabSession, tabId])

  useEffect(() => {
    if (!active) return
    const el = wrapRef.current
    if (!el) return
    const notify = () => window.dispatchEvent(new Event("resize"))
    const ro = new ResizeObserver(() => notify())
    ro.observe(el)
    const t = window.setTimeout(notify, 40)
    return () => {
      ro.disconnect()
      window.clearTimeout(t)
    }
  }, [active, reconnectToken, sessionId])

  return (
    <div
      ref={wrapRef}
      className={cn(
        "h-full min-h-0 w-full overflow-hidden bg-black",
        !active && "hidden",
        className
      )}
      aria-hidden={!active}
    >
      {active ? (
        <LiveTerminal
          sessionId={liveSessionId}
          title={title}
          reconnectToken={reconnectToken}
          onReady={onReady}
          onSessionLost={onSessionLost}
          className="h-full min-h-0 rounded-none bg-black [&_.xterm]:h-full"
        />
      ) : null}
    </div>
  )
}
