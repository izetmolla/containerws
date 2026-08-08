import { useEffect, useRef, useState } from "react"
import { Link } from "react-router"
import {
  Loader2,
  Pause,
  Play,
  RefreshCw,
  ScrollText,
  Trash2,
} from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

import {
  streamVncServiceLogs,
  type InstallTerminalLine,
} from "./api"

let lineSeq = 0
function nextLineId() {
  lineSeq += 1
  return `log-${lineSeq}`
}

function lineClass(stream: InstallTerminalLine["stream"]) {
  switch (stream) {
    case "stderr":
      return "text-amber-300"
    case "system":
      return "text-sky-300/90"
    default:
      return "text-emerald-100/90"
  }
}

export default function VncServiceLogsPage() {
  const [lines, setLines] = useState<InstallTerminalLine[]>([])
  const [streaming, setStreaming] = useState(true)
  const [connecting, setConnecting] = useState(false)
  const abortRef = useRef<AbortController | null>(null)
  const scrollerRef = useRef<HTMLDivElement>(null)
  const streamingRef = useRef(streaming)
  const [prevStreaming, setPrevStreaming] = useState(streaming)

  useEffect(() => {
    streamingRef.current = streaming
  }, [streaming])

  if (streaming !== prevStreaming) {
    setPrevStreaming(streaming)
    if (!streaming) setConnecting(false)
  }

  useEffect(() => {
    const el = scrollerRef.current
    if (!el) return
    el.scrollTop = el.scrollHeight
  }, [lines])

  useEffect(() => {
    if (!streaming) {
      abortRef.current?.abort()
      abortRef.current = null
      return
    }

    const controller = new AbortController()
    abortRef.current = controller
    queueMicrotask(() => setConnecting(true))

    const append = (
      text: string,
      stream: InstallTerminalLine["stream"] = "stdout"
    ) => {
      setLines((prev) => {
        const next = [
          ...prev,
          { id: nextLineId(), text, stream, at: Date.now() },
        ]
        return next.length > 4000 ? next.slice(next.length - 4000) : next
      })
    }

    void (async () => {
      try {
        await streamVncServiceLogs({
          signal: controller.signal,
          onEvent: (event) => {
            setConnecting(false)
            switch (event.type) {
              case "start":
                append(event.message || "Connected to log stream", "system")
                break
              case "log": {
                const prefix = event.path ? `[${event.path}] ` : ""
                append(
                  `${prefix}${event.line ?? ""}`,
                  event.stream === "stderr" ? "stderr" : "stdout"
                )
                break
              }
              case "done":
                append(event.message || "Stream ended", "system")
                break
              case "error":
                append(event.message || "Stream error", "stderr")
                break
            }
          },
        })
        if (!controller.signal.aborted && streamingRef.current) {
          append("Stream disconnected", "system")
          setStreaming(false)
        }
      } catch (err) {
        if (controller.signal.aborted) return
        setConnecting(false)
        const message =
          err instanceof Error ? err.message : "Failed to stream logs"
        append(message, "stderr")
        toast.error(message)
        setStreaming(false)
      }
    })()

    return () => {
      controller.abort()
    }
  }, [streaming])

  return (
    <ContentLoader
      title="VNC service logs"
      description="Live terminal view of VNC service actions and per-user noVNC / TigerVNC logs."
      breadcrumb={[
        { label: "VNC / noVNC", to: "/vnc-novnc" },
        { label: "Logs" },
      ]}
      showHeaderSeparator
      rightComponent={
        <div className="flex flex-wrap items-center justify-end gap-2">
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={() => setLines([])}
            className="gap-1.5"
          >
            <Trash2 className="size-3.5" />
            Clear
          </Button>
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={() => {
              setStreaming(false)
              setTimeout(() => setStreaming(true), 50)
            }}
            className="gap-1.5"
          >
            <RefreshCw className="size-3.5" />
            Reconnect
          </Button>
          {streaming ? (
            <Button
              type="button"
              size="sm"
              variant="destructive"
              onClick={() => setStreaming(false)}
              className="gap-1.5"
            >
              <Pause className="size-3.5" />
              Pause
            </Button>
          ) : (
            <Button
              type="button"
              size="sm"
              onClick={() => setStreaming(true)}
              className="gap-1.5"
            >
              <Play className="size-3.5" />
              Resume
            </Button>
          )}
          <Button type="button" size="sm" variant="secondary" asChild>
            <Link to="/vnc-novnc">Settings</Link>
          </Button>
        </div>
      }
    >
      <div className="overflow-hidden rounded-xl border border-zinc-800 bg-zinc-950 shadow-sm">
        <div className="flex items-center gap-2 border-b border-zinc-800 px-3 py-2 text-xs text-zinc-400">
          <ScrollText className="size-3.5" />
          <span className="font-medium text-zinc-200">vnc-service</span>
          <span className="text-zinc-600">·</span>
          {connecting ? (
            <span className="inline-flex items-center gap-1 text-amber-300">
              <Loader2 className="size-3 animate-spin" />
              Connecting
            </span>
          ) : streaming ? (
            <span className="text-emerald-400">Live</span>
          ) : (
            <span className="text-zinc-500">Paused</span>
          )}
          <span className="ml-auto font-mono text-[10px] text-zinc-600">
            {lines.length} lines
          </span>
        </div>
        <div
          ref={scrollerRef}
          className="h-[min(70vh,42rem)] overflow-auto px-3 py-2 font-mono text-[12px] leading-5"
        >
          {lines.length === 0 ? (
            <p className="text-zinc-500">
              Waiting for log output… Start or stop the VNC service, or open a
              desktop session.
            </p>
          ) : (
            lines.map((line) => (
              <div
                key={line.id}
                className={cn("whitespace-pre-wrap break-all", lineClass(line.stream))}
              >
                {line.text}
              </div>
            ))
          )}
        </div>
      </div>
    </ContentLoader>
  )
}
