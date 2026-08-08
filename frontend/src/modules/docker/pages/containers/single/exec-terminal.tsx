import { useEffect, useRef, useState } from "react"
import { Terminal } from "@xterm/xterm"
import { FitAddon } from "@xterm/addon-fit"
import { WebLinksAddon } from "@xterm/addon-web-links"
import "@xterm/xterm/css/xterm.css"

import { cn } from "@/lib/utils"

import { buildContainerExecWebSocketURL } from "../list/api"

type ExecTerminalProps = {
  containerId: string
  command: string
  user?: string
  className?: string
  onStatusChange?: (
    status: "connecting" | "connected" | "disconnected" | "error"
  ) => void
  onExit?: () => void
}

function fitAndRevealCursor(term: Terminal, fit: FitAddon) {
  try {
    fit.fit()
  } catch {
    return
  }
  term.scrollToBottom()
}

export function ExecTerminal({
  containerId,
  command,
  user,
  className,
  onStatusChange,
  onExit,
}: ExecTerminalProps) {
  const hostRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<Terminal | null>(null)
  const fitRef = useRef<FitAddon | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const onStatusChangeRef = useRef(onStatusChange)
  const onExitRef = useRef(onExit)
  const [status, setStatus] = useState<
    "connecting" | "connected" | "disconnected" | "error"
  >("connecting")

  useEffect(() => {
    onStatusChangeRef.current = onStatusChange
    onExitRef.current = onExit
  })

  useEffect(() => {
    onStatusChangeRef.current?.(status)
  }, [status])

  useEffect(() => {
    const host = hostRef.current
    if (!host) return

    const term = new Terminal({
      cursorBlink: true,
      cursorStyle: "block",
      fontFamily:
        "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace",
      fontSize: 13,
      lineHeight: 1.25,
      theme: {
        background: "#09090b",
        foreground: "#e4e4e7",
        cursor: "#e4e4e7",
        selectionBackground: "#3f3f46",
        black: "#09090b",
        red: "#f87171",
        green: "#4ade80",
        yellow: "#facc15",
        blue: "#60a5fa",
        magenta: "#e879f9",
        cyan: "#22d3ee",
        white: "#e4e4e7",
        brightBlack: "#71717a",
        brightRed: "#fca5a5",
        brightGreen: "#86efac",
        brightYellow: "#fde047",
        brightBlue: "#93c5fd",
        brightMagenta: "#f0abfc",
        brightCyan: "#67e8f9",
        brightWhite: "#fafafa",
      },
      allowProposedApi: true,
      scrollback: 5000,
      convertEol: false,
    })
    const fit = new FitAddon()
    const links = new WebLinksAddon()
    term.loadAddon(fit)
    term.loadAddon(links)
    term.open(host)
    fitAndRevealCursor(term, fit)
    term.focus()

    termRef.current = term
    fitRef.current = fit

    let disposed = false
    let exited = false
    let pingTimer: ReturnType<typeof setInterval> | undefined
    let fitFrame = 0

    const clearPing = () => {
      if (pingTimer) {
        clearInterval(pingTimer)
        pingTimer = undefined
      }
    }

    const sendResize = () => {
      const ws = wsRef.current
      if (!ws || ws.readyState !== WebSocket.OPEN) return
      ws.send(
        JSON.stringify({
          type: "resize",
          cols: term.cols,
          rows: term.rows,
        })
      )
    }

    const scheduleFit = () => {
      cancelAnimationFrame(fitFrame)
      fitFrame = requestAnimationFrame(() => {
        if (disposed) return
        fitAndRevealCursor(term, fit)
        sendResize()
      })
    }

      setStatus("connecting")

    const ws = new WebSocket(
      buildContainerExecWebSocketURL({
        id: containerId,
        command,
        user,
      })
    )
    ws.binaryType = "arraybuffer"
    wsRef.current = ws

    ws.onopen = () => {
      if (disposed) return
      setStatus("connected")
      sendResize()
      pingTimer = setInterval(() => {
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ type: "ping" }))
        }
      }, 30_000)
    }

    ws.onmessage = (event) => {
      if (typeof event.data === "string") {
        try {
          const msg = JSON.parse(event.data) as {
            type?: string
            message?: string
          }
            if (msg.type === "ready") {
              term.reset()
              scheduleFit()
              return
            }
          if (msg.type === "error") {
            term.write(
              `\x1b[31m${msg.message || "Console error"}\x1b[0m\r\n`
            )
            setStatus("error")
            return
          }
          if (msg.type === "exit") {
            exited = true
            term.write(
              `\x1b[90m${msg.message || "Console closed"}\x1b[0m\r\n`
            )
            setStatus("disconnected")
            onExitRef.current?.()
            return
          }
        } catch {
          term.write(event.data)
        }
        return
      }
      term.write(new Uint8Array(event.data as ArrayBuffer))
    }

    ws.onclose = () => {
      clearPing()
      if (disposed) return
      if (!exited) {
        setStatus("disconnected")
        onExitRef.current?.()
      }
    }

    const dataDisposable = term.onData((data) => {
      if (!ws || ws.readyState !== WebSocket.OPEN) return
      ws.send(new TextEncoder().encode(data))
      term.scrollToBottom()
    })

    const onWinResize = () => scheduleFit()
    window.addEventListener("resize", onWinResize)
    const fitTimer = window.setTimeout(() => scheduleFit(), 50)

    return () => {
      disposed = true
      clearPing()
      cancelAnimationFrame(fitFrame)
      window.clearTimeout(fitTimer)
      window.removeEventListener("resize", onWinResize)
      dataDisposable.dispose()
      if (ws.readyState === WebSocket.OPEN) {
        try {
          ws.send(JSON.stringify({ type: "kill" }))
        } catch {
          // ignore
        }
      }
      ws.onopen = null
      ws.onmessage = null
      ws.onerror = null
      ws.onclose = null
      ws.close()
      wsRef.current = null
      term.dispose()
      termRef.current = null
      fitRef.current = null
    }
  }, [containerId, command, user])

  return (
    <div
      className={cn(
        "h-full min-h-0 w-full overflow-hidden rounded-lg bg-zinc-950",
        className
      )}
      onClick={() => termRef.current?.focus()}
    >
      <div
        ref={hostRef}
        className="h-full min-h-[320px] w-full [&_.xterm]:h-full [&_.xterm-viewport]:overflow-auto!"
      />
    </div>
  )
}
