import { useEffect, useRef, useState } from "react"
import {
  Loader2,
  Pause,
  Play,
  RotateCcw,
  ScrollText,
  Square,
} from "lucide-react"
import { toast } from "sonner"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { getRequestErrorMessage } from "@/lib/network"

import {
  controlSoftwareService,
  type ServiceStatus,
  type SoftwareServiceAction,
} from "../pages/list/api"
import {
  streamSoftwareServiceLogs,
  type ServiceLogStreamEvent,
} from "../pages/single/api"

type LogRow = { id: string; text: string; unit?: string }

let logSeq = 0
function nextLogId() {
  logSeq += 1
  return `svc-log-${logSeq}`
}

function overallClass(overall?: string) {
  switch (overall) {
    case "running":
      return "bg-emerald-500/15 text-emerald-700 dark:text-emerald-400"
    case "failed":
      return "bg-red-500/15 text-red-700 dark:text-red-400"
    case "partial":
      return "bg-amber-500/15 text-amber-700 dark:text-amber-400"
    case "stopped":
      return "bg-muted text-muted-foreground"
    default:
      return "bg-muted text-muted-foreground"
  }
}

export function SoftwareServicePanel({
  softwareId,
  softwareName,
  units,
  initialStatus,
  isInstalled,
  onStatusChange,
}: {
  softwareId: string
  softwareName: string
  units: string[]
  initialStatus?: ServiceStatus | null
  isInstalled: boolean
  onStatusChange?: (status: ServiceStatus) => void
}) {
  const [status, setStatus] = useState<ServiceStatus | null>(
    initialStatus ?? null
  )
  const [prevInitialStatus, setPrevInitialStatus] = useState(initialStatus)
  if (initialStatus !== prevInitialStatus) {
    setPrevInitialStatus(initialStatus)
    setStatus(initialStatus ?? null)
  }
  const [busy, setBusy] = useState<SoftwareServiceAction | null>(null)
  const [logsOpen, setLogsOpen] = useState(false)
  const [logsLive, setLogsLive] = useState(false)
  const [logRows, setLogRows] = useState<LogRow[]>([])
  const [logError, setLogError] = useState<string | null>(null)
  const abortRef = useRef<AbortController | null>(null)
  const scrollerRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    return () => {
      abortRef.current?.abort()
    }
  }, [])

  useEffect(() => {
    const el = scrollerRef.current
    if (!el) return
    el.scrollTop = el.scrollHeight
  }, [logRows])

  const overall = status?.overall
  const canAct = isInstalled && !busy

  const runAction = async (action: SoftwareServiceAction) => {
    setBusy(action)
    try {
      const res = await controlSoftwareService(softwareId, action)
      setStatus(res.status)
      onStatusChange?.(res.status)
      toast.success(`${action} · ${res.status.overall}`)
    } catch (err) {
      toast.error(getRequestErrorMessage(err, `Could not ${action} service`))
    } finally {
      setBusy(null)
    }
  }

  const stopLogs = () => {
    abortRef.current?.abort()
    abortRef.current = null
    setLogsLive(false)
  }

  const startLogs = async () => {
    stopLogs()
    setLogsOpen(true)
    setLogError(null)
    setLogRows([])
    setLogsLive(true)
    const ac = new AbortController()
    abortRef.current = ac
    try {
      await streamSoftwareServiceLogs(softwareId, {
        lines: 150,
        signal: ac.signal,
        onEvent: (ev: ServiceLogStreamEvent) => {
          if (ev.type === "start" && ev.message) {
            setLogRows((prev) => [
              ...prev,
              { id: nextLogId(), text: ev.message || "" },
            ])
          } else if (ev.type === "log" && ev.line) {
            setLogRows((prev) => [
              ...prev,
              { id: nextLogId(), text: ev.line || "", unit: ev.unit },
            ])
          } else if (ev.type === "error") {
            setLogError(ev.message || "Log stream error")
          } else if (ev.type === "done") {
            setLogsLive(false)
          }
        },
      })
    } catch (err) {
      if (!ac.signal.aborted) {
        setLogError(
          getRequestErrorMessage(err, "Could not stream service logs")
        )
      }
    } finally {
      if (abortRef.current === ac) {
        setLogsLive(false)
        abortRef.current = null
      }
    }
  }

  return (
    <div className="rounded-lg border border-border/70 bg-card/40 p-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="min-w-0 space-y-1">
          <h2 className="text-sm font-semibold tracking-wide text-muted-foreground uppercase">
            Service
          </h2>
          <p className="text-sm text-muted-foreground">
            {softwareName} exposes systemd units
            {units.length ? (
              <>
                :{" "}
                <span className="font-mono text-foreground">
                  {units.join(", ")}
                </span>
              </>
            ) : null}
          </p>
        </div>
        <Badge className={cn("capitalize", overallClass(overall))}>
          {overall || "unknown"}
        </Badge>
      </div>

      {!isInstalled ? (
        <p className="mt-3 text-xs text-amber-700 dark:text-amber-400">
          Install this software first to start, stop, or restart the service.
        </p>
      ) : (
        <div className="mt-4 flex flex-wrap gap-2">
          <Button
            size="sm"
            variant="outline"
            disabled={!canAct || overall === "running"}
            onClick={() => void runAction("start")}
          >
            {busy === "start" ? (
              <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
            ) : (
              <Play className="mr-1.5 h-3.5 w-3.5" />
            )}
            Start
          </Button>
          <Button
            size="sm"
            variant="outline"
            disabled={!canAct || overall === "stopped"}
            onClick={() => void runAction("stop")}
          >
            {busy === "stop" ? (
              <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
            ) : (
              <Square className="mr-1.5 h-3.5 w-3.5" />
            )}
            Stop
          </Button>
          <Button
            size="sm"
            variant="outline"
            disabled={!canAct}
            onClick={() => void runAction("restart")}
          >
            {busy === "restart" ? (
              <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
            ) : (
              <RotateCcw className="mr-1.5 h-3.5 w-3.5" />
            )}
            Restart
          </Button>
          <Button
            size="sm"
            variant={logsLive ? "secondary" : "outline"}
            onClick={() => {
              if (logsLive) stopLogs()
              else void startLogs()
            }}
          >
            {logsLive ? (
              <Pause className="mr-1.5 h-3.5 w-3.5" />
            ) : (
              <ScrollText className="mr-1.5 h-3.5 w-3.5" />
            )}
            {logsLive ? "Stop live logs" : "Live logs"}
          </Button>
        </div>
      )}

      {status?.units?.length ? (
        <ul className="mt-3 space-y-1 text-xs text-muted-foreground">
          {status.units.map((u) => (
            <li key={u.unit} className="flex flex-wrap gap-x-2">
              <span className="font-mono text-foreground">{u.unit}</span>
              <span>
                {u.active}
                {u.sub ? `/${u.sub}` : ""}
              </span>
              {u.error ? (
                <span className="text-destructive">{u.error}</span>
              ) : null}
            </li>
          ))}
        </ul>
      ) : null}

      {logsOpen ? (
        <div className="mt-4 overflow-hidden rounded-md border bg-zinc-950 text-zinc-100">
          <div className="flex items-center justify-between border-b border-zinc-800 px-3 py-2 text-xs text-zinc-400">
            <span>journalctl · live</span>
            {logsLive ? (
              <span className="inline-flex items-center gap-1 text-emerald-400">
                <span className="size-1.5 animate-pulse rounded-full bg-emerald-400" />
                streaming
              </span>
            ) : (
              <span>paused</span>
            )}
          </div>
          <div
            ref={scrollerRef}
            className="max-h-72 overflow-auto p-3 font-mono text-[11px] leading-relaxed"
          >
            {logRows.length === 0 && !logError ? (
              <p className="text-zinc-500">Waiting for log lines…</p>
            ) : null}
            {logRows.map((row) => (
              <div key={row.id} className="whitespace-pre-wrap break-all">
                {row.unit ? (
                  <span className="text-sky-400">{row.unit} </span>
                ) : null}
                {row.text}
              </div>
            ))}
            {logError ? (
              <p className="mt-2 text-red-400">{logError}</p>
            ) : null}
          </div>
        </div>
      ) : null}
    </div>
  )
}
