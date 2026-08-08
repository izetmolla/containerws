import { useEffect, useMemo, useRef, useState } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import {
  CheckCircle2,
  CloudDownload,
  Copy,
  Loader2,
  Terminal,
  Wrench,
} from "lucide-react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { getRequestErrorMessage } from "@/lib/network"
import { cn } from "@/lib/utils"
import { SoftwareQueuePanel } from "@/modules/softwares/components/software-queue-panel"
import {
  getSoftwareQueue,
  SOFTWARES_FETCH_KEY,
  type SoftwareQueueItem,
  type SoftwareQueueSnapshot,
} from "@/modules/softwares/pages/list/api"
import {
  cancelInstallJob,
  streamInstallJob,
  streamInstallSoftware,
  type InstallStreamEvent,
  type InstallTerminalLine,
} from "@/modules/softwares/pages/single/api"
import {
  InstallTerminal,
  type InstallTerminalStatus,
} from "@/modules/softwares/pages/single/components/install-terminal"

import {
  VSCODE_SESSIONS_FETCH_KEY,
  type CodeserverRuntimeStatus,
} from "./api"

let lineSeq = 0
function nextLineId() {
  lineSeq += 1
  return `vscode-install-${lineSeq}`
}

function matchesVsCodeSoftware(
  item: SoftwareQueueItem,
  softwareId: string,
  softwareName: string
) {
  if (softwareId && item.software_id === softwareId) return true
  const name = (item.software_name || "").toLowerCase()
  const target = softwareName.toLowerCase()
  if (target && name === target) return true
  return name.includes("vs code server") || name.includes("code server")
}

type VsCodeInstallGateProps = {
  status: CodeserverRuntimeStatus
  queue?: SoftwareQueueSnapshot | null
  onInstalled: () => void
}

export function VsCodeInstallGate({
  status,
  queue: queueFromStatus,
  onInstalled,
}: VsCodeInstallGateProps) {
  const queryClient = useQueryClient()
  const [terminalOpen, setTerminalOpen] = useState(false)
  const [terminalStatus, setTerminalStatus] =
    useState<InstallTerminalStatus>("idle")
  const [terminalLines, setTerminalLines] = useState<InstallTerminalLine[]>([])
  const [jobId, setJobId] = useState<string | null>(null)
  const [installing, setInstalling] = useState(false)
  const [cancelling, setCancelling] = useState(false)
  const [copied, setCopied] = useState(false)

  const abortRef = useRef<AbortController | null>(null)
  const jobIdRef = useRef<string | null>(null)
  const attachedJobRef = useRef<string | null>(null)
  const onInstalledRef = useRef(onInstalled)

  useEffect(() => {
    onInstalledRef.current = onInstalled
  }, [onInstalled])

  const softwareId = status.software_id?.trim() || ""
  const softwareName = status.software_name?.trim() || "VS Code Server"
  const cliCommand =
    status.cli_command?.trim() || `cws software install "${softwareName}"`
  const canInstall = Boolean(softwareId)

  const queueQuery = useQuery({
    queryKey: [SOFTWARES_FETCH_KEY, "queue"],
    queryFn: getSoftwareQueue,
    refetchInterval: (q) => {
      const pending = q.state.data?.data?.pending ?? 0
      const running = Boolean(q.state.data?.data?.running)
      return pending > 0 || running ? 1_500 : 4_000
    },
  })

  const softwareQueue =
    queueQuery.data?.data ?? queueFromStatus ?? status.software_queue ?? null
  const queuePending = softwareQueue?.pending ?? 0
  const queueBusy =
    Boolean(softwareQueue?.running) || queuePending > 0

  const vscodeQueueItem = useMemo(() => {
    const items = softwareQueue?.items ?? []
    return (
      items.find(
        (it) =>
          (it.status === "pending" || it.status === "running") &&
          matchesVsCodeSoftware(it, softwareId, softwareName)
      ) ?? null
    )
  }, [softwareQueue?.items, softwareId, softwareName])

  const waitingOnQueue =
    Boolean(vscodeQueueItem) ||
    (installing && queueBusy) ||
    (!status.softwaresync_ready && !status.installed)

  useEffect(() => {
    jobIdRef.current = jobId
  }, [jobId])

  useEffect(() => {
    return () => {
      abortRef.current?.abort()
    }
  }, [])

  const appendLine = (
    text: string,
    stream: InstallTerminalLine["stream"] = "stdout"
  ) => {
    setTerminalLines((prev) => [
      ...prev,
      { id: nextLineId(), text, stream, at: Date.now() },
    ])
  }

  const handleStreamEvent = (event: InstallStreamEvent) => {
    switch (event.type) {
      case "start":
        if (event.job_id) setJobId(event.job_id)
        appendLine(
          event.message ||
            `Starting ${event.name ?? softwareName}${event.version ? ` v${event.version}` : ""}`,
          "system"
        )
        break
      case "system":
        if (event.line != null) appendLine(event.line, "system")
        break
      case "log":
        if (event.line != null) {
          appendLine(
            event.line,
            event.stream === "stderr" ? "stderr" : "stdout"
          )
        }
        break
      case "done": {
        const ok = Boolean(event.success)
        setTerminalStatus(ok ? "success" : "error")
        appendLine(
          event.message || (ok ? "Install completed" : "Install failed"),
          ok ? "system" : "stderr"
        )
        if (ok) {
          toast.success(event.message || "VS Code Server installed")
          void queryClient.invalidateQueries({
            queryKey: [SOFTWARES_FETCH_KEY],
          })
          void queryClient.invalidateQueries({
            queryKey: [VSCODE_SESSIONS_FETCH_KEY],
          })
          onInstalledRef.current()
        } else {
          toast.error(event.message || "Install finished with errors")
        }
        break
      }
      case "cancelled":
        setTerminalStatus("cancelled")
        appendLine(event.message || "Install cancelled", "system")
        break
      case "error":
        setTerminalStatus("error")
        appendLine(event.message || "Install stream error", "stderr")
        toast.error(event.message || "Install failed")
        break
    }
  }

  const attachToJob = async (targetJobId: string) => {
    if (!targetJobId || attachedJobRef.current === targetJobId) return
    if (installing) return

    abortRef.current?.abort()
    const controller = new AbortController()
    abortRef.current = controller
    attachedJobRef.current = targetJobId

    setTerminalOpen(true)
    setTerminalLines([])
    setTerminalStatus("running")
    setInstalling(true)
    setCancelling(false)
    setJobId(targetJobId)
    appendLine(`Attached to install job ${targetJobId.slice(0, 8)}…`, "system")

    try {
      await streamInstallJob(targetJobId, {
        signal: controller.signal,
        onEvent: handleStreamEvent,
      })
    } catch (err) {
      if (controller.signal.aborted) {
        setTerminalStatus((prev) =>
          prev === "running" ? "cancelled" : prev
        )
        return
      }
      setTerminalStatus("error")
      const message = getRequestErrorMessage(err, "Could not follow install")
      appendLine(message, "stderr")
      toast.error(message)
    } finally {
      setInstalling(false)
      setCancelling(false)
      if (attachedJobRef.current === targetJobId) {
        attachedJobRef.current = null
      }
    }
  }

  // Auto-follow when VS Code Server is already queued / installing.
  const pendingQueueKey =
    vscodeQueueItem?.status === "pending" ? vscodeQueueItem.id : ""
  const [prevPendingQueueKey, setPrevPendingQueueKey] = useState("")
  if (!pendingQueueKey && prevPendingQueueKey) {
    setPrevPendingQueueKey("")
  } else if (pendingQueueKey && pendingQueueKey !== prevPendingQueueKey) {
    setPrevPendingQueueKey(pendingQueueKey)
    setTerminalOpen(true)
    setTerminalStatus("running")
    setTerminalLines((prev) => {
      if (prev.some((l) => l.text.includes("Waiting for software queue"))) {
        return prev
      }
      return [
        ...prev,
        {
          id: nextLineId(),
          text: `Waiting for software queue — ${softwareName} is queued behind other installs.`,
          stream: "system",
          at: Date.now(),
        },
      ]
    })
  }

  useEffect(() => {
    if (!vscodeQueueItem) return
    if (vscodeQueueItem.status === "running" && vscodeQueueItem.job_id) {
      const jobId = vscodeQueueItem.job_id
      queueMicrotask(() => {
        void attachToJob(jobId)
      })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- attach once per queue job
  }, [vscodeQueueItem?.id, vscodeQueueItem?.status, vscodeQueueItem?.job_id])

  // When status flips to installed (e.g. CLI finished while we were waiting).
  useEffect(() => {
    if (!status.installed) return
    onInstalledRef.current()
  }, [status.installed])

  const startInstall = async () => {
    if (!softwareId || installing) return

    abortRef.current?.abort()
    const controller = new AbortController()
    abortRef.current = controller
    attachedJobRef.current = null

    setTerminalOpen(true)
    setTerminalLines([])
    setTerminalStatus("running")
    setInstalling(true)
    setCancelling(false)
    setJobId(null)
    appendLine(`Connecting to install stream for ${softwareName}…`, "system")
    appendLine(`CLI equivalent: ${cliCommand}`, "system")
    if (queueBusy) {
      appendLine(
        "Softwares queue is busy — install will start when the queue is clear.",
        "system"
      )
    }

    try {
      await streamInstallSoftware(softwareId, {
        signal: controller.signal,
        onEvent: handleStreamEvent,
      })
    } catch (err) {
      if (controller.signal.aborted) {
        setTerminalStatus((prev) =>
          prev === "running" ? "cancelled" : prev
        )
        return
      }
      setTerminalStatus("error")
      const message = getRequestErrorMessage(err, "Install failed")
      appendLine(message, "stderr")
      toast.error(message)
    } finally {
      setInstalling(false)
      setCancelling(false)
    }
  }

  const cancelInstall = async () => {
    const id = jobIdRef.current
    setCancelling(true)
    if (id) {
      try {
        await cancelInstallJob(id)
      } catch (err) {
        toast.error(getRequestErrorMessage(err, "Could not cancel install"))
      }
    }
    abortRef.current?.abort()
  }

  const copyCli = async () => {
    try {
      await navigator.clipboard.writeText(cliCommand)
      setCopied(true)
      toast.success("CLI command copied")
      window.setTimeout(() => setCopied(false), 1500)
    } catch {
      toast.error("Could not copy command")
    }
  }

  const headline = vscodeQueueItem
    ? vscodeQueueItem.status === "running"
      ? "Installing VS Code Server…"
      : "VS Code Server is queued"
    : "VS Code Server is not installed"

  const description = vscodeQueueItem
    ? vscodeQueueItem.status === "running"
      ? "Installation is in progress. When it finishes, your workspaces list will appear here."
      : "Another software job is ahead in the queue. VS Code install starts automatically when it reaches the front."
    : "Install Microsoft's VS Code CLI on this host before managing serve-web sessions. If Softwares are installing, wait for the queue — then install from the panel or CLI."

  return (
    <div className="flex w-full flex-col gap-4">
      <section className="grid gap-4 rounded-xl border border-dashed bg-muted/20 p-6">
        <div className="flex items-start gap-3">
          <div className="grid size-10 shrink-0 place-items-center rounded-xl bg-sky-500/15 text-sky-700 dark:text-sky-300">
            {waitingOnQueue || installing ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              <Wrench className="size-4" />
            )}
          </div>
          <div className="min-w-0 space-y-1">
            <div className="flex flex-wrap items-center gap-2">
              <h2 className="font-semibold tracking-tight">{headline}</h2>
              {waitingOnQueue ? (
                <span className="inline-flex items-center gap-1 rounded-md bg-amber-500/15 px-2 py-0.5 text-xs font-medium text-amber-800 dark:text-amber-300">
                  <Loader2 className="size-3 animate-spin" />
                  Waiting on queue
                  {queuePending > 0 ? ` · ${queuePending}` : null}
                </span>
              ) : null}
              {queueBusy && !waitingOnQueue ? (
                <span className="inline-flex items-center gap-1 rounded-md bg-sky-500/15 px-2 py-0.5 text-xs font-medium text-sky-700 dark:text-sky-300">
                  <Loader2 className="size-3 animate-spin" />
                  Softwares queue · {queuePending}
                </span>
              ) : null}
            </div>
            <p className="text-sm text-muted-foreground">{description}</p>
            {status.detail ? (
              <p className="font-mono text-xs text-muted-foreground">
                Probe: {status.detail}
              </p>
            ) : null}
          </div>
        </div>

        {!vscodeQueueItem ? (
          <>
            <div className="space-y-2">
              <div className="flex items-center gap-2 text-xs font-medium text-muted-foreground">
                <Terminal className="size-3.5" />
                CLI command
              </div>
              <div className="flex flex-col gap-2 sm:flex-row sm:items-stretch">
                <code
                  className={cn(
                    "flex-1 overflow-x-auto rounded-lg border bg-zinc-950 px-3 py-2.5",
                    "font-mono text-[12px] leading-5 text-emerald-100/90"
                  )}
                >
                  {cliCommand}
                </code>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="shrink-0 gap-1.5"
                  onClick={() => void copyCli()}
                >
                  {copied ? (
                    <CheckCircle2 className="size-3.5" />
                  ) : (
                    <Copy className="size-3.5" />
                  )}
                  {copied ? "Copied" : "Copy"}
                </Button>
              </div>
            </div>

            <div className="flex flex-wrap items-center gap-2">
              <Button
                type="button"
                disabled={!canInstall || installing}
                onClick={() => void startInstall()}
              >
                {installing ? (
                  <Loader2 data-icon="inline-start" className="animate-spin" />
                ) : (
                  <CloudDownload data-icon="inline-start" />
                )}
                {installing ? "Installing…" : `Install ${softwareName}`}
              </Button>
              {!canInstall ? (
                <p className="text-xs text-amber-700 dark:text-amber-300">
                  Catalog entry not found. Seed softwares or run the CLI command
                  above.
                </p>
              ) : null}
            </div>
          </>
        ) : null}
      </section>

      <SoftwareQueuePanel
        queue={softwareQueue}
        waiting={waitingOnQueue}
        product="VS Code"
      />

      <InstallTerminal
        open={terminalOpen}
        status={terminalStatus}
        lines={terminalLines}
        title={
          waitingOnQueue && !jobId
            ? "Waiting for software queue"
            : `${softwareName} install`
        }
        subtitle={
          waitingOnQueue && !jobId
            ? queuePending > 0
              ? `${queuePending} software job${queuePending === 1 ? "" : "s"} remaining`
              : "softwaresync reconcile"
            : jobId
              ? `job ${jobId.slice(0, 8)}`
              : cliCommand
        }
        cancelling={cancelling}
        onCancel={jobId ? () => void cancelInstall() : undefined}
        onRetry={() => void startInstall()}
        onClear={() => setTerminalLines([])}
        onClose={() => setTerminalOpen(false)}
      />
    </div>
  )
}
