import { useEffect, useRef, useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Link } from "react-router"
import {
  CheckCircle2,
  Download,
  KeyRound,
  Loader2,
  MonitorPlay,
  Play,
  RefreshCw,
  ScrollText,
  Square,
  Wrench,
} from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { FormPasswordField } from "@/components/password"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Form } from "@/components/ui/form"
import { Separator } from "@/components/ui/separator"
import { getRequestErrorMessage, withError } from "@/lib/network"
import { useCurrentUser } from "@/store/authorization"
import { zodResolver } from "@hookform/resolvers/zod"
import { useForm } from "react-hook-form"

import {
  getUser,
  USERS_FETCH_KEY,
} from "@/modules/users/pages/list/api"
import {
  createVncProfile,
  setVncPassword,
  startVncProfile,
  vncPasswordFormSchema,
  vncPasswordRules,
  VNC_PASSWORD_MAX,
  type VncPasswordFormValues,
} from "@/modules/users/pages/single/vnc/api"

import {
  cancelRdpSetupJob,
  cancelVncSetupJob,
  detectVncSetup,
  getRdpSetupStatus,
  getVncServiceStatus,
  RDP_SETUP_FETCH_KEY,
  startVncService,
  stopVncService,
  streamRdpSetup,
  streamVncSetup,
  VNC_SERVICE_FETCH_KEY,
  VNC_SETUP_FETCH_KEY,
  type InstallTerminalLine,
} from "./api"
import { HostDetails } from "./components/host-details"
import {
  InstallTerminal,
  type InstallTerminalStatus,
} from "./components/install-terminal"
import { SoftwareQueuePanel } from "./components/software-queue-panel"
import { useVncInstallSession } from "./install-session-store"
import {
  getSoftwareQueue,
  SOFTWARES_FETCH_KEY,
} from "@/modules/softwares/pages/list/api"

let lineSeq = 0
function nextLineId() {
  lineSeq += 1
  return `line-${lineSeq}`
}

export default function VncSettingsPage() {
  const queryClient = useQueryClient()
  const currentUser = useCurrentUser()
  const currentUserId = currentUser?.id?.trim() || ""

  const {
    terminalOpen,
    terminalStatus,
    terminalLines,
    jobId,
    installing,
    cancelling,
    updateMode,
    hydratedJobId,
    setTerminalOpen,
    setTerminalStatus,
    setTerminalLines,
    appendLine: storeAppendLine,
    setJobId,
    setInstalling,
    setCancelling,
    setUpdateMode,
    setHydratedJobId,
    resetTerminal,
  } = useVncInstallSession()

  const [rdpTerminalOpen, setRdpTerminalOpen] = useState(false)
  const [rdpTerminalStatus, setRdpTerminalStatus] =
    useState<InstallTerminalStatus>("idle")
  const [rdpTerminalLines, setRdpTerminalLines] = useState<
    InstallTerminalLine[]
  >([])
  const [rdpJobId, setRdpJobId] = useState<string | null>(null)
  const [rdpInstalling, setRdpInstalling] = useState(false)
  const [rdpCancelling, setRdpCancelling] = useState(false)
  const [passwordOpen, setPasswordOpen] = useState(false)

  const abortRef = useRef<AbortController | null>(null)
  const jobIdRef = useRef<string | null>(null)
  const rdpAbortRef = useRef<AbortController | null>(null)
  const rdpJobIdRef = useRef<string | null>(null)
  const autoStartedRef = useRef(false)

  useEffect(() => {
    jobIdRef.current = jobId
  }, [jobId])

  useEffect(() => {
    rdpJobIdRef.current = rdpJobId
  }, [rdpJobId])

  useEffect(() => {
    return () => {
      // Keep VNC install stream alive across navigation; only tear down RDP.
      rdpAbortRef.current?.abort()
    }
  }, [])

  const { data, isLoading, error, refetch, isFetching } = useQuery({
    queryKey: [VNC_SETUP_FETCH_KEY, "detect"],
    queryFn: detectVncSetup,
    refetchInterval: (query) => {
      const install = query.state.data?.data?.install
      const queue = query.state.data?.data?.software_queue
      const queueBusy =
        Boolean(queue?.running) || (queue?.pending ?? 0) > 0
      if (install?.active || install?.status === "running" || queueBusy)
        return 2_000
      if (!query.state.data?.data?.softwaresync_ready) return 2_000
      return false
    },
  })

  const softwareQueueQuery = useQuery({
    queryKey: [SOFTWARES_FETCH_KEY, "queue"],
    queryFn: getSoftwareQueue,
    refetchInterval: (q) => {
      const pending = q.state.data?.data?.pending ?? 0
      const running = Boolean(q.state.data?.data?.running)
      return pending > 0 || running ? 1_500 : 4_000
    },
  })

  const serviceQuery = useQuery({
    queryKey: [VNC_SERVICE_FETCH_KEY, "status"],
    queryFn: getVncServiceStatus,
    refetchInterval: 8_000,
  })

  const rdpQuery = useQuery({
    queryKey: [RDP_SETUP_FETCH_KEY, "status"],
    queryFn: getRdpSetupStatus,
  })

  const meQuery = useQuery({
    queryKey: [USERS_FETCH_KEY, "detail", currentUserId],
    queryFn: () => getUser(currentUserId),
    enabled: Boolean(currentUserId),
    retry: false,
  })

  const plan = data?.data?.plan
  const status = data?.data?.status
  const options = data?.data?.options ?? status?.options
  const activeInstall = data?.data?.install ?? status?.install
  const softwareQueue =
    softwareQueueQuery.data?.data ??
    data?.data?.software_queue ??
    activeInstall?.software_queue
  const queuePending = softwareQueue?.pending ?? 0
  const queueBusy =
    Boolean(softwareQueue?.running) || queuePending > 0
  const waitingOnQueue =
    Boolean(activeInstall?.waiting_queue) ||
    activeInstall?.phase === "waiting_queue"
  const isReady = Boolean(status?.ready)
  const canInstall = Boolean(plan?.supported)
  const packagesMissing = Boolean(options?.missing)
  const serviceRunning = Boolean(serviceQuery.data?.data?.running)
  const rdpStatus = rdpQuery.data?.data
  const rdpReady = Boolean(rdpStatus?.ready)
  const rdpSupported = Boolean(rdpStatus?.plan?.supported ?? plan?.supported)
  const meVnc = meQuery.data?.data?.vnc
  const meHasPassword = Boolean(meVnc?.has_password)
  const meHasProfile = Boolean(meQuery.data?.data?.has_vnc || meVnc)
  const meLive = Boolean(meVnc?.live)

  // Hydrate terminal from server-side active/background install.
  useEffect(() => {
    if (!activeInstall) return
    const id = activeInstall.job_id || ""
    const running =
      activeInstall.active || activeInstall.status === "running"
    const finished =
      activeInstall.status === "success" ||
      activeInstall.status === "error" ||
      activeInstall.status === "cancelled"

    if (!running && !finished) return
    if (!id) return

    if (hydratedJobId !== id || running) {
      const lines: InstallTerminalLine[] = (activeInstall.lines || []).map(
        (l, i) => ({
          id: `srv-${id}-${i}`,
          text: l.text,
          stream:
            l.stream === "stderr" || l.stream === "system"
              ? l.stream
              : "stdout",
          at: l.at || Date.now(),
        })
      )
      setTerminalLines(lines)
      setJobId(id)
      setHydratedJobId(id)
      setTerminalOpen(true)
      setInstalling(running)
      setUpdateMode(Boolean(activeInstall.auto))
      if (running) setTerminalStatus("running")
      else if (activeInstall.status === "success") setTerminalStatus("success")
      else if (activeInstall.status === "cancelled")
        setTerminalStatus("cancelled")
      else setTerminalStatus("error")
    }
  }, [
    activeInstall,
    hydratedJobId,
    setHydratedJobId,
    setInstalling,
    setJobId,
    setTerminalLines,
    setTerminalOpen,
    setTerminalStatus,
    setUpdateMode,
  ])

  const appendLine = (
    text: string,
    stream: InstallTerminalLine["stream"] = "stdout"
  ) => {
    storeAppendLine({ id: nextLineId(), text, stream, at: Date.now() })
  }

  const startInstall = async (asUpdate = false) => {
    if (installing) return

    abortRef.current?.abort()
    const controller = new AbortController()
    abortRef.current = controller

    setUpdateMode(asUpdate)
    setTerminalOpen(true)
    setTerminalLines([])
    setTerminalStatus("running")
    setInstalling(true)
    setCancelling(false)
    setJobId(null)

    appendLine(
      asUpdate
        ? "Checking packages and applying updates…"
        : "Connecting to setup stream…",
      "system"
    )

    try {
      await streamVncSetup({
        signal: controller.signal,
        onEvent: (event) => {
          switch (event.type) {
            case "start":
              if (event.job_id) setJobId(event.job_id)
              appendLine(
                event.message ||
                  (asUpdate
                    ? "Starting package check & update"
                    : "Starting VNC/noVNC package setup"),
                "system"
              )
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
                event.message || (ok ? "Setup completed" : "Setup failed"),
                ok ? "system" : "stderr"
              )
              if (ok) {
                toast.success(
                  event.message ||
                    (asUpdate ? "Packages checked & updated" : "VNC/noVNC installed")
                )
              } else {
                toast.error(event.message || "Setup finished with errors")
              }
              void queryClient.invalidateQueries({
                queryKey: [VNC_SETUP_FETCH_KEY],
              })
              void queryClient.invalidateQueries({
                queryKey: [VNC_SERVICE_FETCH_KEY],
              })
              break
            }
            case "cancelled":
              setTerminalStatus("cancelled")
              appendLine(event.message || "Installation stopped", "stderr")
              toast.message(event.message || "Installation stopped")
              void queryClient.invalidateQueries({
                queryKey: [VNC_SETUP_FETCH_KEY],
              })
              break
            case "error":
              setTerminalStatus("error")
              appendLine(event.message || "Setup error", "stderr")
              toast.error(event.message || "Setup failed")
              break
          }
        },
      })

      setTerminalStatus(
        useVncInstallSession.getState().terminalStatus === "running"
          ? "success"
          : useVncInstallSession.getState().terminalStatus
      )
    } catch (err) {
      if (controller.signal.aborted) {
        if (useVncInstallSession.getState().terminalStatus === "running") {
          setTerminalStatus("cancelled")
        }
        return
      }
      setTerminalStatus("error")
      const message = getRequestErrorMessage(err, "Setup failed")
      appendLine(message, "stderr")
      toast.error(message)
    } finally {
      setInstalling(false)
      setCancelling(false)
      abortRef.current = null
    }
  }

  // Auto-reinstall when Option says installed-but-missing (boot may already be running).
  useEffect(() => {
    if (autoStartedRef.current) return
    if (!packagesMissing || !canInstall) return
    if (installing || activeInstall?.active || activeInstall?.status === "running")
      return
    autoStartedRef.current = true
    void startInstall(true)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [packagesMissing, canInstall, installing, activeInstall?.active, activeInstall?.status])

  const handleStop = async () => {
    if (!installing) return
    setCancelling(true)
    appendLine("Stop requested…", "system")

    const activeJobId = jobIdRef.current
    try {
      if (activeJobId) {
        await cancelVncSetupJob(activeJobId)
      }
    } catch (err) {
      appendLine(
        getRequestErrorMessage(err, "Stop request failed — aborting stream"),
        "stderr"
      )
    } finally {
      abortRef.current?.abort()
    }
  }

  const appendRdpLine = (
    text: string,
    stream: InstallTerminalLine["stream"] = "stdout"
  ) => {
    setRdpTerminalLines((prev) => [
      ...prev,
      { id: nextLineId(), text, stream, at: Date.now() },
    ])
  }

  const startRdpInstall = async () => {
    if (rdpInstalling || installing) return

    rdpAbortRef.current?.abort()
    const controller = new AbortController()
    rdpAbortRef.current = controller

    setRdpTerminalOpen(true)
    setRdpTerminalLines([])
    setRdpTerminalStatus("running")
    setRdpInstalling(true)
    setRdpCancelling(false)
    setRdpJobId(null)
    appendRdpLine("Connecting to RDP (xrdp) setup stream…", "system")

    try {
      await streamRdpSetup({
        signal: controller.signal,
        onEvent: (event) => {
          switch (event.type) {
            case "start":
              if (event.job_id) setRdpJobId(event.job_id)
              appendRdpLine(
                event.message || "Starting optional RDP (xrdp) package setup",
                "system"
              )
              break
            case "log":
              if (event.line != null) {
                appendRdpLine(
                  event.line,
                  event.stream === "stderr" ? "stderr" : "stdout"
                )
              }
              break
            case "done": {
              const ok = Boolean(event.success)
              setRdpTerminalStatus(ok ? "success" : "error")
              appendRdpLine(
                event.message || (ok ? "RDP setup completed" : "RDP setup failed"),
                ok ? "system" : "stderr"
              )
              if (ok) {
                toast.success(event.message || "RDP (xrdp) installed")
              } else {
                toast.error(event.message || "RDP setup finished with errors")
              }
              void queryClient.invalidateQueries({
                queryKey: [RDP_SETUP_FETCH_KEY],
              })
              break
            }
            case "cancelled":
              setRdpTerminalStatus("cancelled")
              appendRdpLine(event.message || "Installation stopped", "stderr")
              toast.message(event.message || "Installation stopped")
              void queryClient.invalidateQueries({
                queryKey: [RDP_SETUP_FETCH_KEY],
              })
              break
            case "error":
              setRdpTerminalStatus("error")
              appendRdpLine(event.message || "RDP setup error", "stderr")
              toast.error(event.message || "RDP setup failed")
              break
          }
        },
      })
      setRdpTerminalStatus((prev) => (prev === "running" ? "success" : prev))
    } catch (err) {
      if (controller.signal.aborted) {
        setRdpTerminalStatus((prev) =>
          prev === "running" ? "cancelled" : prev
        )
        return
      }
      setRdpTerminalStatus("error")
      const message = getRequestErrorMessage(err, "RDP setup failed")
      appendRdpLine(message, "stderr")
      toast.error(message)
    } finally {
      setRdpInstalling(false)
      setRdpCancelling(false)
      rdpAbortRef.current = null
    }
  }

  const handleRdpStop = async () => {
    if (!rdpInstalling) return
    setRdpCancelling(true)
    appendRdpLine("Stop requested…", "system")
    const activeJobId = rdpJobIdRef.current
    try {
      if (activeJobId) {
        await cancelRdpSetupJob(activeJobId)
      }
    } catch (err) {
      appendRdpLine(
        getRequestErrorMessage(err, "Stop request failed — aborting stream"),
        "stderr"
      )
    } finally {
      rdpAbortRef.current?.abort()
    }
  }

  const startMutation = useMutation({
    mutationFn: startVncService,
    onSuccess: (res) => {
      toast.success(res.message || "VNC service started")
      void queryClient.invalidateQueries({ queryKey: [VNC_SERVICE_FETCH_KEY] })
      void meQuery.refetch()
    },
    onError: (err) =>
      toast.error(getRequestErrorMessage(err, "Failed to start VNC service")),
  })

  const stopMutation = useMutation({
    mutationFn: stopVncService,
    onSuccess: (res) => {
      toast.success(res.message || "VNC service stopped")
      void queryClient.invalidateQueries({ queryKey: [VNC_SERVICE_FETCH_KEY] })
      void meQuery.refetch()
    },
    onError: (err) =>
      toast.error(getRequestErrorMessage(err, "Failed to stop VNC service")),
  })

  const startMyDesktopMutation = useMutation({
    mutationFn: async () => {
      if (!currentUserId) throw new Error("Not signed in")
      if (!meHasProfile) {
        throw new Error("Set a VNC password first to create your desktop profile")
      }
      if (!meHasPassword) {
        throw new Error("Set a VNC password before starting the desktop")
      }
      return startVncProfile(currentUserId)
    },
    onSuccess: (res) => {
      toast.success(res.message || "Desktop started")
      void meQuery.refetch()
      void queryClient.invalidateQueries({ queryKey: [VNC_SERVICE_FETCH_KEY] })
    },
    onError: (err) =>
      toast.error(getRequestErrorMessage(err, "Failed to start your desktop")),
  })

  const passwordMutation = useMutation({
    mutationFn: async (password: string) => {
      if (!currentUserId) throw new Error("Not signed in")
      if (meHasProfile) {
        return setVncPassword(currentUserId, password)
      }
      return createVncProfile(currentUserId, {
        vnc_password: password,
        start: true,
      })
    },
    onSuccess: (res) => {
      toast.success(res.message || "Password saved")
      setPasswordOpen(false)
      void meQuery.refetch()
      void queryClient.invalidateQueries({ queryKey: [VNC_SERVICE_FETCH_KEY] })
      void queryClient.invalidateQueries({ queryKey: [USERS_FETCH_KEY] })
    },
    onError: (err) =>
      toast.error(getRequestErrorMessage(err, "Failed to save VNC password")),
  })

  const serviceBusy = startMutation.isPending || stopMutation.isPending
  const showQuickActions =
    isReady && Boolean(currentUserId) && (!meHasPassword || !meLive)

  return (
    <ContentLoader
      title="VNC / noVNC"
      description="Install and manage TigerVNC, noVNC, optional RDP, and desktop packages for this host."
      breadcrumb={[
        { label: "VNC / noVNC", to: "/vnc-novnc" },
        { label: "Settings" },
      ]}
      isLoading={isLoading}
      error={withError(error, data)}
      showHeaderSeparator
      rightComponent={
        <div className="flex flex-wrap items-center gap-1.5 rounded-lg border bg-muted/30 p-1.5">
          <Button
            type="button"
            size="sm"
            variant="ghost"
            disabled={isFetching || installing || rdpInstalling}
            onClick={() => {
              void refetch()
              void serviceQuery.refetch()
              void rdpQuery.refetch()
              void softwareQueueQuery.refetch()
            }}
            className="gap-1.5"
          >
            {isFetching ||
            serviceQuery.isFetching ||
            rdpQuery.isFetching ||
            softwareQueueQuery.isFetching ? (
              <Loader2 className="size-3.5 animate-spin" />
            ) : (
              <RefreshCw className="size-3.5" />
            )}
            Refresh
          </Button>

          {(canInstall || rdpSupported) && (
            <Separator orientation="vertical" className="mx-0.5 hidden h-6 sm:block" />
          )}

          {canInstall ? (
            <Button
              type="button"
              size="sm"
              variant="outline"
              disabled={installing || rdpInstalling}
              onClick={() => void startInstall(isReady)}
              className="gap-1.5 bg-background"
            >
              {installing && updateMode ? (
                <Loader2 className="size-3.5 animate-spin" />
              ) : (
                <Wrench className="size-3.5" />
              )}
              {isReady ? "Update VNC" : "Install VNC"}
            </Button>
          ) : null}

          {rdpSupported ? (
            <Button
              type="button"
              size="sm"
              variant="outline"
              disabled={installing || rdpInstalling}
              onClick={() => void startRdpInstall()}
              className="gap-1.5 bg-background"
            >
              {rdpInstalling ? (
                <Loader2 className="size-3.5 animate-spin" />
              ) : (
                <Download className="size-3.5" />
              )}
              {rdpReady ? "Update RDP" : "Install RDP"}
            </Button>
          ) : null}

          {isReady ? (
            <>
              <Separator orientation="vertical" className="mx-0.5 hidden h-6 sm:block" />
              {serviceRunning ? (
                <Button
                  type="button"
                  size="sm"
                  variant="destructive"
                  disabled={serviceBusy || installing || rdpInstalling}
                  onClick={() => stopMutation.mutate()}
                  className="gap-1.5"
                >
                  {stopMutation.isPending ? (
                    <Loader2 className="size-3.5 animate-spin" />
                  ) : (
                    <Square className="size-3.5" />
                  )}
                  Stop
                </Button>
              ) : (
                <Button
                  type="button"
                  size="sm"
                  disabled={serviceBusy || installing || rdpInstalling}
                  onClick={() => startMutation.mutate()}
                  className="gap-1.5"
                >
                  {startMutation.isPending ? (
                    <Loader2 className="size-3.5 animate-spin" />
                  ) : (
                    <Play className="size-3.5" />
                  )}
                  Start
                </Button>
              )}
              <Button type="button" size="sm" variant="ghost" asChild>
                <Link to="/vnc-novnc/logs" className="gap-1.5">
                  <ScrollText className="size-3.5" />
                  Logs
                </Link>
              </Button>
            </>
          ) : !canInstall ? null : (
            <Button
              type="button"
              size="sm"
              disabled={installing || rdpInstalling}
              onClick={() => void startInstall(packagesMissing)}
              className="gap-1.5"
            >
              {installing && !updateMode ? (
                <Loader2 className="size-3.5 animate-spin" />
              ) : terminalStatus === "success" ? (
                <CheckCircle2 className="size-3.5" />
              ) : (
                <Download className="size-3.5" />
              )}
              {installing && !updateMode
                ? "Installing…"
                : packagesMissing
                  ? "Reinstall"
                  : "Start install"}
            </Button>
          )}
        </div>
      }
    >
      {plan && status ? (
        <div className="space-y-8 pb-8">
          <div className="flex items-start gap-4">
            <div className="flex size-14 shrink-0 items-center justify-center rounded-2xl bg-primary text-primary-foreground">
              <MonitorPlay className="size-7" />
            </div>
            <div className="min-w-0 space-y-2">
              <div className="flex flex-wrap items-center gap-2">
                {isReady ? (
                  <span className="inline-flex items-center gap-1 rounded-md bg-emerald-500/15 px-2 py-0.5 text-xs font-medium text-emerald-700 dark:text-emerald-300">
                    <CheckCircle2 className="size-3.5" />
                    Installed
                  </span>
                ) : packagesMissing ? (
                  <span className="inline-flex items-center gap-1 rounded-md bg-amber-500/15 px-2 py-0.5 text-xs font-medium text-amber-800 dark:text-amber-300">
                    Missing on OS
                  </span>
                ) : (
                  <span className="rounded-md bg-muted px-2 py-0.5 text-xs text-muted-foreground">
                    Not installed
                  </span>
                )}
                {options?.installed && !isReady ? (
                  <span className="rounded-md bg-muted px-2 py-0.5 text-xs text-muted-foreground">
                    Was installed previously
                  </span>
                ) : null}
                {isReady ? (
                  serviceRunning ? (
                    <span className="inline-flex items-center gap-1 rounded-md bg-sky-500/15 px-2 py-0.5 text-xs font-medium text-sky-700 dark:text-sky-300">
                      Service running
                      {typeof serviceQuery.data?.data?.live_sessions ===
                      "number"
                        ? ` · ${serviceQuery.data.data.live_sessions} live`
                        : null}
                    </span>
                  ) : (
                    <span className="rounded-md bg-muted px-2 py-0.5 text-xs text-muted-foreground">
                      Service stopped
                    </span>
                  )
                ) : null}
                {waitingOnQueue ? (
                  <span className="inline-flex items-center gap-1 rounded-md bg-amber-500/15 px-2 py-0.5 text-xs font-medium text-amber-800 dark:text-amber-300">
                    <Loader2 className="size-3.5 animate-spin" />
                    Waiting on software queue
                    {queuePending > 0 ? ` · ${queuePending}` : null}
                  </span>
                ) : null}
                {queueBusy && !waitingOnQueue ? (
                  <span className="inline-flex items-center gap-1 rounded-md bg-sky-500/15 px-2 py-0.5 text-xs font-medium text-sky-700 dark:text-sky-300">
                    <Loader2 className="size-3.5 animate-spin" />
                    Softwares queue · {queuePending}
                  </span>
                ) : null}
                {plan.distro || plan.os ? (
                  <span className="rounded-md bg-muted px-2 py-0.5 text-xs text-muted-foreground">
                    {[plan.distro || plan.os, plan.distro_version]
                      .filter(Boolean)
                      .join(" ")}
                  </span>
                ) : null}
                {plan.hostname ? (
                  <span className="rounded-md bg-muted px-2 py-0.5 font-mono text-xs text-muted-foreground">
                    {plan.hostname}
                  </span>
                ) : null}
              </div>
              <p className="max-w-2xl text-sm text-muted-foreground">
                {waitingOnQueue
                  ? "VNC install is queued behind Softwares. When every pending/running software job finishes or fails, package setup starts automatically."
                  : isReady
                  ? "Packages are installed. Use Check & update to refresh them, Start/Stop to control desktop sessions, or open Logs for a live terminal view."
                  : packagesMissing
                    ? "VNC was installed before but packages are missing on this host. Reinstall to restore the desktop stack."
                    : canInstall
                      ? "VNC/noVNC packages are missing on this host. Review the machine details below, then start the installation. If Softwares are installing, VNC waits for that queue first."
                      : "This operating system is not supported for automated VNC/noVNC setup."}
              </p>
              {!isReady && canInstall ? (
                <div className="pt-1">
                  <Button
                    type="button"
                    disabled={installing || rdpInstalling}
                    onClick={() => void startInstall(false)}
                    className="gap-2"
                  >
                    {installing ? (
                      <Loader2 className="size-4 animate-spin" />
                    ) : (
                      <Download className="size-4" />
                    )}
                    {installing ? "Installing…" : "Start installation"}
                  </Button>
                </div>
              ) : null}
            </div>
          </div>

          <SoftwareQueuePanel
            queue={softwareQueue}
            waiting={waitingOnQueue || (installing && queueBusy)}
            product="VNC"
          />

          <InstallTerminal
            open={terminalOpen}
            status={terminalStatus}
            lines={terminalLines}
            title={
              waitingOnQueue
                ? "Waiting for software queue"
                : updateMode
                  ? "VNC / noVNC check & update"
                  : "VNC / noVNC installation"
            }
            subtitle={
              waitingOnQueue
                ? queuePending > 0
                  ? `${queuePending} software job${queuePending === 1 ? "" : "s"} remaining`
                  : "softwaresync reconcile"
                : jobId
                  ? `job ${jobId.slice(0, 8)}${plan.package_manager ? ` · ${plan.package_manager}` : ""}`
                  : plan.package_manager || undefined
            }
            cancelling={cancelling}
            onStop={() => void handleStop()}
            onRetry={() => void startInstall(updateMode)}
            onClear={() => setTerminalLines([])}
            onClose={() => {
              resetTerminal()
            }}
          />

          {showQuickActions ? (
            <section className="grid gap-4 rounded-xl border bg-card p-5">
              <div className="space-y-1">
                <h2 className="text-base font-semibold tracking-tight">
                  Quick setup for your account
                </h2>
                <p className="text-sm text-muted-foreground">
                  Finish your personal desktop: set a VNC password if needed,
                  then start the session for{" "}
                  <code className="text-xs">
                    {currentUser?.username ||
                      currentUser?.email ||
                      "current user"}
                  </code>
                  .
                </p>
              </div>
              <div className="flex flex-wrap gap-2">
                {!meHasPassword ? (
                  <Button
                    type="button"
                    onClick={() => setPasswordOpen(true)}
                    className="gap-1.5"
                  >
                    <KeyRound className="size-3.5" />
                    Set VNC password
                  </Button>
                ) : null}
                {meHasPassword && !meLive ? (
                  <Button
                    type="button"
                    disabled={startMyDesktopMutation.isPending}
                    onClick={() => startMyDesktopMutation.mutate()}
                    className="gap-1.5"
                  >
                    {startMyDesktopMutation.isPending ? (
                      <Loader2 className="size-3.5 animate-spin" />
                    ) : (
                      <Play className="size-3.5" />
                    )}
                    {startMyDesktopMutation.isPending
                      ? "Starting…"
                      : "Start my desktop"}
                  </Button>
                ) : null}
                {meHasPassword ? (
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => setPasswordOpen(true)}
                    className="gap-1.5"
                  >
                    <KeyRound className="size-3.5" />
                    Reset password
                  </Button>
                ) : null}
              </div>
            </section>
          ) : null}

          <HostDetails plan={plan} status={status} />

          {rdpSupported ? (
            <section className="grid gap-4 rounded-xl border bg-card p-5">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="min-w-0 space-y-1">
                  <h2 className="text-base font-semibold tracking-tight">
                    Remote Desktop (RDP)
                  </h2>
                  <p className="text-sm text-muted-foreground">
                    Optional xrdp install — separate from VNC/noVNC. Enable
                    per-user from a user&apos;s VNC page after packages are
                    ready.
                  </p>
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  <span
                    className={
                      rdpReady
                        ? "inline-flex items-center gap-1 rounded-md bg-emerald-500/15 px-2 py-0.5 text-xs font-medium text-emerald-700 dark:text-emerald-300"
                        : "rounded-md bg-muted px-2 py-0.5 text-xs text-muted-foreground"
                    }
                  >
                    {rdpReady ? (
                      <>
                        <CheckCircle2 className="size-3.5" />
                        Installed
                      </>
                    ) : (
                      "Not installed"
                    )}
                  </span>
                  {rdpReady ? (
                    <span
                      className={
                        rdpStatus?.running
                          ? "inline-flex rounded-md bg-sky-500/15 px-2 py-0.5 text-xs font-medium text-sky-700 dark:text-sky-300"
                          : "rounded-md bg-muted px-2 py-0.5 text-xs text-muted-foreground"
                      }
                    >
                      {rdpStatus?.running
                        ? `Service running · :${rdpStatus.port || 3389}`
                        : `Port ${rdpStatus?.port || 3389}`}
                    </span>
                  ) : null}
                </div>
              </div>

              {rdpStatus?.missing?.length ? (
                <p className="text-xs text-muted-foreground">
                  Missing:{" "}
                  <code className="text-xs">{rdpStatus.missing.join(", ")}</code>
                </p>
              ) : null}

              <div>
                <Button
                  type="button"
                  disabled={installing || rdpInstalling}
                  onClick={() => void startRdpInstall()}
                  className="gap-2"
                >
                  {rdpInstalling ? (
                    <Loader2 className="size-4 animate-spin" />
                  ) : (
                    <Download className="size-4" />
                  )}
                  {rdpInstalling
                    ? "Installing RDP…"
                    : rdpReady
                      ? "Reinstall / update RDP"
                      : "Install RDP packages"}
                </Button>
              </div>

              <InstallTerminal
                open={rdpTerminalOpen}
                status={rdpTerminalStatus}
                lines={rdpTerminalLines}
                title="RDP (xrdp) installation"
                subtitle={
                  rdpJobId
                    ? `job ${rdpJobId.slice(0, 8)}${
                        rdpStatus?.plan?.package_manager
                          ? ` · ${rdpStatus.plan.package_manager}`
                          : plan.package_manager
                            ? ` · ${plan.package_manager}`
                            : ""
                      }`
                    : rdpStatus?.plan?.package_manager ||
                      plan.package_manager ||
                      undefined
                }
                cancelling={rdpCancelling}
                onStop={() => void handleRdpStop()}
                onRetry={() => void startRdpInstall()}
                onClear={() => setRdpTerminalLines([])}
                onClose={() => {
                  setRdpTerminalOpen(false)
                  setRdpTerminalLines([])
                  setRdpTerminalStatus("idle")
                }}
              />
            </section>
          ) : null}
        </div>
      ) : null}

      <Dialog
        open={passwordOpen}
        onOpenChange={(next) => !next && setPasswordOpen(false)}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>
              {meHasPassword ? "Reset VNC password" : "Set VNC password"}
            </DialogTitle>
            <DialogDescription>
              Required before starting your desktop. TigerVNC stores at most{" "}
              {VNC_PASSWORD_MAX} characters.
            </DialogDescription>
          </DialogHeader>
          <QuickPasswordForm
            pending={passwordMutation.isPending}
            submitLabel={meHasPassword ? "Update password" : "Save & start"}
            onCancel={() => setPasswordOpen(false)}
            onSubmit={(password) => passwordMutation.mutate(password)}
          />
        </DialogContent>
      </Dialog>
    </ContentLoader>
  )
}

function QuickPasswordForm({
  pending,
  submitLabel,
  onCancel,
  onSubmit,
}: {
  pending: boolean
  submitLabel: string
  onCancel: () => void
  onSubmit: (password: string) => void
}) {
  const form = useForm<VncPasswordFormValues>({
    resolver: zodResolver(vncPasswordFormSchema),
    defaultValues: { password: "", confirm_password: "" },
    mode: "onChange",
  })
  const password = form.watch("password")

  return (
    <Form {...form} schema={vncPasswordFormSchema}>
      <form
        className="grid gap-4"
        onSubmit={form.handleSubmit((values) =>
          onSubmit(values.password.trim())
        )}
      >
        <ul className="space-y-1.5 rounded-lg border bg-muted/40 p-3">
          {vncPasswordRules.map((rule) => {
            const ok = rule.test(password)
            return (
              <li
                key={rule.id}
                className={
                  ok
                    ? "flex items-start gap-2 text-xs text-emerald-700 dark:text-emerald-300"
                    : "flex items-start gap-2 text-xs text-muted-foreground"
                }
              >
                <CheckCircle2 className="mt-0.5 size-3.5 shrink-0" />
                <span>{rule.label}</span>
              </li>
            )
          })}
        </ul>
        <FormPasswordField
          control={form.control}
          name="password"
          label="VNC password"
          autoComplete="new-password"
          maxLength={VNC_PASSWORD_MAX}
        />
        <FormPasswordField
          control={form.control}
          name="confirm_password"
          label="Confirm password"
          autoComplete="new-password"
          maxLength={VNC_PASSWORD_MAX}
        />
        <DialogFooter>
          <Button type="button" variant="outline" onClick={onCancel}>
            Cancel
          </Button>
          <Button type="submit" disabled={!form.formState.isValid || pending}>
            {pending ? (
              <>
                <Loader2 className="size-3.5 animate-spin" />
                Saving…
              </>
            ) : (
              submitLabel
            )}
          </Button>
        </DialogFooter>
      </form>
    </Form>
  )
}
