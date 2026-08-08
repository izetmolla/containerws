import { useEffect, useRef, useState } from "react"
import { Terminal } from "@xterm/xterm"
import { FitAddon } from "@xterm/addon-fit"
import { WebLinksAddon } from "@xterm/addon-web-links"
import "@xterm/xterm/css/xterm.css"

import { cn } from "@/lib/utils"

import { buildCliWebSocketURL } from "@/components/cloudshell/api"

export type LiveTerminalReadyInfo = {
  user?: string
  home?: string
  shell?: string
  message?: string
  sessionId?: string
  resumed?: boolean
}

type LiveTerminalProps = {
  className?: string
  /** Resume an existing backend PTY session when set. */
  sessionId?: string | null
  /** Shown to the server when creating a new session. */
  title?: string | null
  /** Run the PTY as this Linux username (admin impersonation). */
  asUser?: string | null
  onStatusChange?: (
    status: "connecting" | "connected" | "disconnected" | "error"
  ) => void
  onReady?: (info: LiveTerminalReadyInfo) => void
  /** Fired when the server reports the resume token is gone (start fresh). */
  onSessionLost?: (lostId: string) => void
  /** Bump to force a fresh WebSocket attach (same sessionId still resumes). */
  reconnectToken?: number
}

const RECONNECT_BASE_MS = 800
const RECONNECT_MAX_MS = 12_000
const RECONNECT_MAX_ATTEMPTS = 40

function fitAndRevealCursor(term: Terminal, fit: FitAddon) {
  try {
    fit.fit()
  } catch {
    // Host may be display:none during tab switch.
    return
  }
  // Keep the prompt / caret in the visible area above the soft keyboard.
  term.scrollToBottom()
}

function reconnectDelay(attempt: number) {
  const exp = Math.min(
    RECONNECT_MAX_MS,
    RECONNECT_BASE_MS * 2 ** Math.min(attempt, 6)
  )
  // Small jitter so many tabs don't stampede the server.
  return exp + Math.floor(Math.random() * 250)
}

export function LiveTerminal({
  className,
  sessionId,
  title,
  asUser,
  onStatusChange,
  onReady,
  onSessionLost,
  reconnectToken = 0,
}: LiveTerminalProps) {
  const hostRef = useRef<HTMLDivElement>(null)
  const wrapRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<Terminal | null>(null)
  const fitRef = useRef<FitAddon | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const onReadyRef = useRef(onReady)
  const onSessionLostRef = useRef(onSessionLost)
  const onStatusChangeRef = useRef(onStatusChange)
  const [status, setStatus] = useState<
    "connecting" | "connected" | "disconnected" | "error"
  >("connecting")

  useEffect(() => {
    onReadyRef.current = onReady
    onSessionLostRef.current = onSessionLost
    onStatusChangeRef.current = onStatusChange
  })

  useEffect(() => {
    onStatusChangeRef.current?.(status)
  }, [status])

  // Header "Show keyboard" / external focus requests.
  useEffect(() => {
    const onFocusRequest = () => {
      const term = termRef.current
      const fit = fitRef.current
      if (!term) return
      term.focus()
      if (fit) fitAndRevealCursor(term, fit)
    }
    window.addEventListener("cloudshell:focus", onFocusRequest)
    return () => window.removeEventListener("cloudshell:focus", onFocusRequest)
  }, [])

  // Special-keys bar (Ctrl+C, Esc, arrows, …) injects raw PTY bytes.
  useEffect(() => {
    const onInject = (event: Event) => {
      const detail = (event as CustomEvent<{ data?: string; delivered?: boolean }>)
        .detail
      const data = detail?.data
      if (!data) return
      const ws = wsRef.current
      if (!ws || ws.readyState !== WebSocket.OPEN) return
      ws.send(new TextEncoder().encode(data))
      termRef.current?.scrollToBottom()
      detail.delivered = true
    }
    window.addEventListener("cloudshell:inject", onInject)
    return () => window.removeEventListener("cloudshell:inject", onInject)
  }, [])

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
      // PTY already sends CRLF — converting again creates blank rows.
      convertEol: false,
    })
    const fit = new FitAddon()
    const links = new WebLinksAddon()
    term.loadAddon(fit)
    term.loadAddon(links)
    term.open(host)
    fitAndRevealCursor(term, fit)
    term.focus()

    // Mobile soft-keyboard: keep the helper textarea from scrolling the page away.
    const helper = host.querySelector(
      ".xterm-helper-textarea"
    ) as HTMLTextAreaElement | null
    if (helper) {
      helper.setAttribute("autocomplete", "off")
      helper.setAttribute("autocorrect", "off")
      helper.setAttribute("autocapitalize", "off")
      helper.setAttribute("spellcheck", "false")
      helper.setAttribute("enterkeyhint", "enter")
      helper.style.fontSize = "16px" // iOS: avoid focus zoom
    }

    termRef.current = term
    fitRef.current = fit

    let disposed = false
    let pingTimer: ReturnType<typeof setInterval> | undefined
    let reconnectTimer: ReturnType<typeof setTimeout> | undefined
    let sawReady = false
    let shellExited = false
    let reconnectAttempt = 0
    let fitFrame = 0
    // Prefer the live session id from `ready` so auto-resume works after the
    // first connect even when the prop started null.
    let liveSessionId = sessionId?.trim() || null

    const clearReconnectTimer = () => {
      if (reconnectTimer) {
        clearTimeout(reconnectTimer)
        reconnectTimer = undefined
      }
    }

    const clearPingTimer = () => {
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

    const scheduleReconnect = (reason: string) => {
      if (disposed || shellExited) return
      if (reconnectAttempt >= RECONNECT_MAX_ATTEMPTS) {
        setStatus("disconnected")
        term.write(
          "\x1b[31mCould not reconnect — use Reconnect in the menu.\x1b[0m\r\n"
        )
        return
      }
      clearReconnectTimer()
      const delay = reconnectDelay(reconnectAttempt)
      reconnectAttempt += 1
      setStatus("connecting")
      term.write(
        `\x1b[90m${reason} — retrying in ${Math.round(delay / 1000)}s…\x1b[0m\r\n`
      )
      reconnectTimer = setTimeout(() => {
        reconnectTimer = undefined
        connect({ auto: true })
      }, delay)
    }

    const connect = (opts?: { auto?: boolean }) => {
      if (disposed || shellExited) return
      clearReconnectTimer()
      clearPingTimer()

      // Drop any half-open socket before opening another.
      const prev = wsRef.current
      if (prev) {
        prev.onopen = null
        prev.onmessage = null
        prev.onerror = null
        prev.onclose = null
        try {
          prev.close()
        } catch {
          // ignore
        }
        wsRef.current = null
      }

      setStatus("connecting")
      const resumeId = liveSessionId
      const auto = Boolean(opts?.auto)
      if (!auto) {
        term.write(
          resumeId
            ? "\x1b[90mReconnecting…\x1b[0m\r\n"
            : "\x1b[90mConnecting…\x1b[0m\r\n"
        )
      }

      const ws = new WebSocket(
        buildCliWebSocketURL({
          sessionId: resumeId,
          title,
          asUser,
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
              code?: string
              message?: string
              user?: string
              home?: string
              shell?: string
              session_id?: string
              resumed?: boolean
            }
            if (msg.type === "ready") {
              sawReady = true
              reconnectAttempt = 0
              if (msg.session_id) {
                liveSessionId = msg.session_id
              }
              onReadyRef.current?.({
                user: msg.user,
                home: msg.home,
                shell: msg.shell,
                message: msg.message,
                sessionId: msg.session_id,
                resumed: Boolean(msg.resumed),
              })
              // Clear connecting / reconnect banners; replay / prompt follows.
              term.reset()
              if (msg.message && !msg.resumed) {
                term.write(`\x1b[36m${msg.message}\x1b[0m\r\n`)
              } else if (auto && msg.resumed) {
                term.write("\x1b[90mReconnected.\x1b[0m\r\n")
              }
              scheduleFit()
              return
            }
            if (msg.type === "error") {
              if (msg.code === "SESSION_NOT_FOUND" && resumeId) {
                // Parent clears the stale id and bumps reconnectToken → fresh mount.
                liveSessionId = null
                shellExited = true
                clearReconnectTimer()
                onSessionLostRef.current?.(resumeId)
                term.write(
                  `\x1b[33mSession expired — starting a new shell…\x1b[0m\r\n`
                )
                setStatus("disconnected")
                return
              }
              term.write(
                `\x1b[31m${msg.message || "Terminal error"}\x1b[0m\r\n`
              )
              setStatus("error")
              // Socket will close next; onclose schedules reconnect.
              return
            }
            if (msg.type === "exit") {
              shellExited = true
              clearReconnectTimer()
              term.write(
                `\x1b[90m${msg.message || "Shell closed"}\x1b[0m\r\n`
              )
              setStatus("disconnected")
              return
            }
          } catch {
            term.write(event.data)
          }
          return
        }

        const bytes = new Uint8Array(event.data as ArrayBuffer)
        term.write(bytes)
      }

      ws.onerror = () => {
        // onclose always follows; avoid double messaging here.
      }

      ws.onclose = () => {
        clearPingTimer()
        if (disposed || shellExited) {
          if (!disposed && shellExited) setStatus("disconnected")
          return
        }
        scheduleReconnect(
          sawReady ? "Connection lost" : "Disconnected"
        )
      }
    }

    const dataDisposable = term.onData((data) => {
      const ws = wsRef.current
      if (!ws || ws.readyState !== WebSocket.OPEN) return
      ws.send(new TextEncoder().encode(data))
      // Keep caret visible without a full refit on every key.
      term.scrollToBottom()
    })

    const onWinResize = () => scheduleFit()
    window.addEventListener("resize", onWinResize)
    window.addEventListener("orientationchange", onWinResize)
    const vp = window.visualViewport
    vp?.addEventListener("resize", onWinResize)
    vp?.addEventListener("scroll", onWinResize)

    // Resume immediately when the network or tab comes back.
    const onOnline = () => {
      if (disposed || shellExited) return
      const ws = wsRef.current
      if (ws && ws.readyState === WebSocket.OPEN) return
      clearReconnectTimer()
      reconnectAttempt = Math.min(reconnectAttempt, 1)
      connect({ auto: true })
    }
    const onVisible = () => {
      if (document.visibilityState !== "visible") return
      onOnline()
    }
    window.addEventListener("online", onOnline)
    document.addEventListener("visibilitychange", onVisible)

    const fitTimer = window.setTimeout(() => scheduleFit(), 50)

    connect()

    return () => {
      disposed = true
      clearReconnectTimer()
      clearPingTimer()
      cancelAnimationFrame(fitFrame)
      window.clearTimeout(fitTimer)
      window.removeEventListener("resize", onWinResize)
      window.removeEventListener("orientationchange", onWinResize)
      window.removeEventListener("online", onOnline)
      document.removeEventListener("visibilitychange", onVisible)
      vp?.removeEventListener("resize", onWinResize)
      vp?.removeEventListener("scroll", onWinResize)
      dataDisposable.dispose()
      // Closing the socket detaches on the server; PTY stays alive for resume.
      const ws = wsRef.current
      if (ws) {
        ws.onopen = null
        ws.onmessage = null
        ws.onerror = null
        ws.onclose = null
        ws.close()
      }
      wsRef.current = null
      term.dispose()
      termRef.current = null
      fitRef.current = null
    }
  }, [reconnectToken, sessionId, title, asUser])

  const focusTerminal = () => {
    const term = termRef.current
    const fit = fitRef.current
    if (!term) return
    term.focus()
    if (fit) fitAndRevealCursor(term, fit)
  }

  return (
    <div
      ref={wrapRef}
      className={cn(
        "h-full min-h-0 w-full overflow-hidden rounded-lg bg-zinc-950",
        "touch-manipulation [-webkit-tap-highlight-color:transparent]",
        className
      )}
      onPointerDown={() => {
        // Open the soft keyboard and keep the caret in view on first tap.
        focusTerminal()
      }}
      onClick={focusTerminal}
    >
      <div
        ref={hostRef}
        className="h-full w-full [&_.xterm]:h-full [&_.xterm-viewport]:overflow-auto! [&_.xterm-screen]:touch-manipulation"
      />
    </div>
  )
}
