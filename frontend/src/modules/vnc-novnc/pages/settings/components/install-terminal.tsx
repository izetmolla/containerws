import { useEffect, useRef, useState } from "react"
import {
  Ban,
  CheckCircle2,
  Copy,
  Loader2,
  RefreshCw,
  Square,
  Terminal,
  XCircle,
} from "lucide-react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

import type { InstallTerminalLine } from "../api"

export type InstallTerminalStatus =
  | "idle"
  | "running"
  | "success"
  | "error"
  | "cancelled"

type InstallTerminalProps = {
  open: boolean
  status: InstallTerminalStatus
  lines: InstallTerminalLine[]
  title?: string
  subtitle?: string
  cancelling?: boolean
  onStop?: () => void
  onClose?: () => void
  onClear?: () => void
  onRetry?: () => void
}

const statusLabel: Record<InstallTerminalStatus, string> = {
  idle: "Ready",
  running: "Installing…",
  success: "Completed",
  error: "Failed",
  cancelled: "Stopped",
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

function formatLogs(lines: InstallTerminalLine[]) {
  return lines
    .map((line) => {
      const prefix =
        line.stream === "stderr" ? "!" : line.stream === "system" ? "#" : ">"
      return `${prefix} ${line.text}`
    })
    .join("\n")
}

export function InstallTerminal({
  open,
  status,
  lines,
  title = "Installation terminal",
  subtitle,
  cancelling,
  onStop,
  onClose,
  onClear,
  onRetry,
}: InstallTerminalProps) {
  const scrollerRef = useRef<HTMLDivElement>(null)
  const [copying, setCopying] = useState(false)

  useEffect(() => {
    const el = scrollerRef.current
    if (!el) return
    el.scrollTop = el.scrollHeight
  }, [lines, open, status])

  if (!open) return null

  const running = status === "running"
  const finished =
    status === "success" || status === "error" || status === "cancelled"

  const handleCopy = async () => {
    const text = formatLogs(lines)
    if (!text.trim()) {
      toast.message("Nothing to copy yet")
      return
    }
    setCopying(true)
    try {
      await navigator.clipboard.writeText(text)
      toast.success("Logs copied")
    } catch {
      toast.error("Could not copy logs")
    } finally {
      setCopying(false)
    }
  }

  return (
    <section className="overflow-hidden rounded-xl border border-zinc-800 bg-zinc-950 text-zinc-100 shadow-lg">
      <div className="flex flex-wrap items-center justify-between gap-2 border-b border-zinc-800 px-3 py-2">
        <div className="flex min-w-0 items-center gap-2">
          <Terminal className="size-4 shrink-0 text-zinc-400" />
          <div className="min-w-0">
            <p className="truncate text-sm font-medium">{title}</p>
            {subtitle ? (
              <p className="truncate font-mono text-[11px] text-zinc-500">
                {subtitle}
              </p>
            ) : null}
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <StatusBadge status={status} />
          <Button
            type="button"
            size="sm"
            variant="ghost"
            disabled={lines.length === 0 || copying}
            onClick={() => void handleCopy()}
            className="gap-1.5 text-zinc-300 hover:text-zinc-50"
          >
            {copying ? (
              <Loader2 className="size-3.5 animate-spin" />
            ) : (
              <Copy className="size-3.5" />
            )}
            Copy
          </Button>
          {running ? (
            <Button
              type="button"
              size="sm"
              variant="destructive"
              disabled={cancelling}
              onClick={onStop}
              className="gap-1.5"
            >
              {cancelling ? (
                <Loader2 className="size-3.5 animate-spin" />
              ) : (
                <Ban className="size-3.5" />
              )}
              {cancelling ? "Stopping…" : "Stop"}
            </Button>
          ) : (
            <>
              {finished && onRetry ? (
                <Button
                  type="button"
                  size="sm"
                  variant="secondary"
                  onClick={onRetry}
                  className="gap-1.5"
                >
                  <RefreshCw className="size-3.5" />
                  Retry
                </Button>
              ) : null}
              {onClear ? (
                <Button
                  type="button"
                  size="sm"
                  variant="ghost"
                  onClick={onClear}
                >
                  Clear
                </Button>
              ) : null}
              {onClose ? (
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  onClick={onClose}
                >
                  <Square className="size-3.5" />
                  Close
                </Button>
              ) : null}
            </>
          )}
        </div>
      </div>

      <div
        ref={scrollerRef}
        className="h-72 overflow-y-auto bg-[radial-gradient(ellipse_at_top,_#18181b_0%,_#09090b_55%)] px-3 py-3 font-mono text-[12px] leading-5"
        aria-live="polite"
        aria-relevant="additions"
      >
        {lines.length === 0 ? (
          <p className="text-zinc-500">Waiting for install output…</p>
        ) : (
          <ul className="space-y-0.5">
            {lines.map((line) => (
              <li
                key={line.id}
                className={cn(
                  "whitespace-pre-wrap break-all",
                  lineClass(line.stream)
                )}
              >
                <span className="select-none text-zinc-600">
                  {line.stream === "stderr"
                    ? "!"
                    : line.stream === "system"
                      ? "#"
                      : "›"}{" "}
                </span>
                {line.text}
              </li>
            ))}
          </ul>
        )}
        {running ? (
          <div className="mt-2 flex items-center gap-2 text-zinc-500">
            <span className="inline-block size-2 animate-pulse rounded-full bg-emerald-400" />
            live
          </div>
        ) : null}
      </div>

      <div className="flex items-center justify-between border-t border-zinc-800 px-3 py-1.5 text-[11px] text-zinc-500">
        <span>View only — no shell input</span>
        <span>{statusLabel[status]}</span>
      </div>
    </section>
  )
}

function StatusBadge({ status }: { status: InstallTerminalStatus }) {
  const base =
    "inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-[11px] font-medium"
  switch (status) {
    case "running":
      return (
        <span className={cn(base, "bg-emerald-500/15 text-emerald-300")}>
          <Loader2 className="size-3 animate-spin" />
          {statusLabel[status]}
        </span>
      )
    case "success":
      return (
        <span className={cn(base, "bg-emerald-500/15 text-emerald-300")}>
          <CheckCircle2 className="size-3" />
          {statusLabel[status]}
        </span>
      )
    case "error":
      return (
        <span className={cn(base, "bg-red-500/15 text-red-300")}>
          <XCircle className="size-3" />
          {statusLabel[status]}
        </span>
      )
    case "cancelled":
      return (
        <span className={cn(base, "bg-amber-500/15 text-amber-300")}>
          <Ban className="size-3" />
          {statusLabel[status]}
        </span>
      )
    default:
      return (
        <span className={cn(base, "bg-zinc-500/15 text-zinc-400")}>
          {statusLabel[status]}
        </span>
      )
  }
}
