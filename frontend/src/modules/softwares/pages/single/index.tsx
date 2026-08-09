import { useEffect, useRef, useState } from "react"
import { useNavigate, useParams } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { parseAsStringLiteral, useQueryState } from "nuqs"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { withError, getRequestErrorMessage } from "@/lib/network"
import { switchPackageManager } from "@/modules/brew/pages/api"

import { SoftwareDetail } from "../../components/software-detail"
import {
  cancelInstallJob,
  getLatestInstallJob,
  getSoftwareSingle,
  streamInstallJob,
  streamInstallSoftware,
  SOFTWARES_FETCH_KEY,
  type InstallJobSnapshot,
  type InstallStreamEvent,
  type InstallTerminalLine,
  type SoftwareSingleResponse,
} from "./api"
import {
  enqueueSoftwareActions,
  type ServiceStatus,
} from "../list/api"
import type { InstallTerminalStatus } from "./components/install-terminal"

let lineSeq = 0
function nextLineId() {
  lineSeq += 1
  return `line-${lineSeq}`
}

function statusFromJob(status: string): InstallTerminalStatus {
  switch (status) {
    case "running":
      return "running"
    case "success":
      return "success"
    case "cancelled":
      return "cancelled"
    case "error":
      return "error"
    default:
      return "idle"
  }
}

function linesFromJob(job: InstallJobSnapshot): InstallTerminalLine[] {
  return (job.lines ?? []).map((line) => ({
    id: nextLineId(),
    text: line.text,
    stream:
      line.stream === "stderr"
        ? "stderr"
        : line.stream === "system"
          ? "system"
          : "stdout",
    at: line.at || Date.now(),
  }))
}

export default function SoftwareSinglePage() {
  const { id = "" } = useParams()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [tabParam] = useQueryState(
    "tab",
    parseAsStringLiteral(["overview", "service"] as const).withDefault(
      "overview"
    )
  )
  const initialTab = tabParam

  const [terminalOpen, setTerminalOpen] = useState(false)
  const [terminalStatus, setTerminalStatus] =
    useState<InstallTerminalStatus>("idle")
  const [terminalLines, setTerminalLines] = useState<InstallTerminalLine[]>([])
  const [jobId, setJobId] = useState<string | null>(null)
  const [failureReason, setFailureReason] = useState<string | null>(null)
  const [installing, setInstalling] = useState(false)
  const [uninstalling, setUninstalling] = useState(false)
  const [cancelling, setCancelling] = useState(false)
  const [sessionReady, setSessionReady] = useState(!id)
  const [prevId, setPrevId] = useState(id)
  if (id !== prevId) {
    setPrevId(id)
    setSessionReady(!id)
  }

  const abortRef = useRef<AbortController | null>(null)
  const jobIdRef = useRef<string | null>(null)

  useEffect(() => {
    jobIdRef.current = jobId
  }, [jobId])

  useEffect(() => {
    return () => {
      abortRef.current?.abort()
    }
  }, [])

  const { data, isLoading, error } = useQuery({
    queryKey: [SOFTWARES_FETCH_KEY, "single", id],
    queryFn: () => getSoftwareSingle(id),
    enabled: Boolean(id),
  })

  const software = data?.data?.software
  const latest = data?.data?.latest_version
  const installedVersion = data?.data?.installed_version
  const versions = data?.data?.versions ?? []
  const isInstalled = Boolean(data?.data?.is_installed)
  const uninstalled = Boolean(data?.data?.uninstalled)
  const hasUpdate = Boolean(data?.data?.has_update)
  const osMissing = Boolean(
    data?.data?.os_missing || installedVersion?.os_missing
  )
  const canInstall = Boolean(latest?.install_script) && data?.data?.package_manager !== "brew"
  const canUninstall = Boolean(
    !uninstalled &&
      data?.data?.package_manager !== "brew" &&
      (data?.data?.can_uninstall ||
        installedVersion?.can_uninstall ||
        installedVersion?.uninstall_script ||
        latest?.uninstall_script)
  )
  const canControl = Boolean(
    !uninstalled &&
      data?.data?.package_manager !== "brew" &&
      (data?.data?.can_control ||
        (software?.can_control && software?.service_units?.length))
  )
  const serviceStatus = data?.data?.service_status ?? null
  const packageManager = data?.data?.package_manager
  const canSwitchToBrew = Boolean(data?.data?.can_switch_to_brew)
  const canSwitchToLocal = Boolean(data?.data?.can_switch_to_local)

  const switchMutation = useMutation({
    mutationFn: (target: "local" | "brew") => {
      if (!id) return Promise.reject(new Error("Missing software id"))
      return switchPackageManager(id, target)
    },
    onSuccess: (res) => {
      toast.success(res.message || "Switched package manager")
      void queryClient.invalidateQueries({
        queryKey: [SOFTWARES_FETCH_KEY, "single", id],
      })
      void queryClient.invalidateQueries({ queryKey: [SOFTWARES_FETCH_KEY] })
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Switch failed"))
    },
  })

  const handleServiceStatusChange = (status: ServiceStatus) => {
    queryClient.setQueryData<SoftwareSingleResponse>(
      [SOFTWARES_FETCH_KEY, "single", id],
      (prev) => {
        if (!prev?.data) return prev
        return {
          ...prev,
          data: {
            ...prev.data,
            service_status: status,
            can_control: true,
          },
        }
      }
    )
    void queryClient.invalidateQueries({
      queryKey: [SOFTWARES_FETCH_KEY, "list"],
    })
  }

  const appendLine = (
    text: string,
    stream: InstallTerminalLine["stream"] = "stdout"
  ) => {
    setTerminalLines((prev) => [
      ...prev,
      { id: nextLineId(), text, stream, at: Date.now() },
    ])
  }

  const hideTerminalSuccess = () => {
    setTerminalOpen(false)
    setTerminalLines([])
    setTerminalStatus("idle")
    setFailureReason(null)
    setJobId(null)
  }

  const handleStreamEvent = (event: InstallStreamEvent) => {
    switch (event.type) {
      case "start":
        if (event.job_id) setJobId(event.job_id)
        // Replay already includes history; avoid duplicate "Connecting…" noise.
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
        if (event.message) {
          appendLine(event.message, ok ? "system" : "stderr")
        }
        if (ok) {
          toast.success(event.message || "Installed successfully")
          void queryClient.invalidateQueries({
            queryKey: [SOFTWARES_FETCH_KEY],
          })
          // Auto-dismiss terminal on success — not needed after a clean install.
          window.setTimeout(() => {
            hideTerminalSuccess()
          }, 1200)
        } else {
          toast.error(event.message || "Install finished with errors")
          void queryClient.invalidateQueries({
            queryKey: [SOFTWARES_FETCH_KEY],
          })
        }
        break
      }
      case "cancelled":
        setTerminalStatus("cancelled")
        appendLine(event.message || "Installation cancelled", "stderr")
        toast.message(event.message || "Installation cancelled")
        break
      case "error":
        setTerminalStatus("error")
        if (event.message) {
          setFailureReason(event.message)
        } else {
          setFailureReason((prev) => prev || "Install failed")
          appendLine("Install error", "stderr")
        }
        toast.error(event.message?.split("\n")[0] || "Install failed")
        break
    }
  }

  const attachToJobStream = async (targetJobId: string, signal: AbortSignal) => {
    await streamInstallJob(targetJobId, {
      signal,
      onEvent: handleStreamEvent,
    })
  }

  const startInstall = async () => {
    if (!id || installing) return

    abortRef.current?.abort()
    const controller = new AbortController()
    abortRef.current = controller

    setTerminalOpen(true)
    setTerminalLines([])
    setTerminalStatus("running")
    setInstalling(true)
    setCancelling(false)
    setJobId(null)
    setFailureReason(null)

    const mode = hasUpdate ? "update" : isInstalled ? "reinstall" : "install"
    appendLine(`Connecting to ${mode} stream…`, "system")

    try {
      await streamInstallSoftware(id, {
        signal: controller.signal,
        onEvent: handleStreamEvent,
      })
      setTerminalStatus((prev) => (prev === "running" ? "success" : prev))
    } catch (err) {
      if (controller.signal.aborted) {
        // Client left the page / switched stream — job keeps running server-side.
        return
      }
      setTerminalStatus("error")
      setTerminalOpen(true)
      const message = getRequestErrorMessage(err, "Install failed")
      setFailureReason(message)
      appendLine(message, "stderr")
      toast.error(message)
    } finally {
      if (abortRef.current === controller) {
        abortRef.current = null
      }
      setInstalling(false)
      setCancelling(false)
    }
  }

  // Restore install session after refresh / return (Strict Mode safe).
  useEffect(() => {
    if (!id) return

    let cancelled = false
    const controller = new AbortController()

    void (async () => {
      try {
        const res = await getLatestInstallJob(id)
        if (cancelled) return

        const job = res.data
        if (!job) {
          setSessionReady(true)
          return
        }

        if (job.status === "success") {
          // Successful installs hide the terminal — nothing to restore.
          setSessionReady(true)
          return
        }

        setJobId(job.id)
        setFailureReason(job.failure_reason || null)
        setTerminalOpen(true)
        setTerminalStatus(statusFromJob(job.status))

        if (job.status === "running") {
          // Let the SSE replay fill the terminal so we stay in sync with the live job.
          setTerminalLines([])
          setInstalling(true)
          abortRef.current = controller
          // Show the page (Installing… + terminal) while we reattach.
          setSessionReady(true)
          try {
            await attachToJobStream(job.id, controller.signal)
            if (!cancelled) {
              setInstalling(false)
              setCancelling(false)
            }
          } catch (err) {
            if (cancelled || controller.signal.aborted) return
            const message = getRequestErrorMessage(err, "Lost install stream")
            appendLine(message, "stderr")
            try {
              const again = await getLatestInstallJob(id)
              const latestJob = again.data
              if (latestJob && !cancelled) {
                setTerminalLines(linesFromJob(latestJob))
                setTerminalStatus(statusFromJob(latestJob.status))
                setFailureReason(latestJob.failure_reason || null)
                setInstalling(latestJob.status === "running")
              } else if (!cancelled) {
                setTerminalStatus("error")
                setInstalling(false)
              }
            } catch {
              if (!cancelled) {
                setTerminalStatus("error")
                setInstalling(false)
              }
            }
          } finally {
            if (abortRef.current === controller) {
              abortRef.current = null
            }
          }
          return
        }

        // Failed / cancelled — show persisted logs.
        setTerminalLines(linesFromJob(job))
        setInstalling(false)
        setSessionReady(true)
      } catch {
        // No job / unauthorized — ignore.
        if (!cancelled) setSessionReady(true)
      } finally {
        if (!cancelled) setSessionReady(true)
      }
    })()

    return () => {
      cancelled = true
      controller.abort()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- restore once per software id
  }, [id])

  const handleCancel = async () => {
    if (!installing) return
    setCancelling(true)
    appendLine("Cancel requested…", "system")

    const activeJobId = jobIdRef.current
    try {
      if (activeJobId) {
        await cancelInstallJob(activeJobId)
      }
    } catch (err) {
      appendLine(
        getRequestErrorMessage(err, "Cancel request failed"),
        "stderr"
      )
    } finally {
      // Do not abort the SSE on cancel — wait for server cancelled event.
      setCancelling(false)
    }
  }

  const handleCloseFailed = () => {
    setTerminalOpen(false)
    // Keep logs in state until retry/clear so reopen is possible via retry path.
  }

  const startUninstall = async () => {
    if (!id || !canUninstall || uninstalling || installing) return
    setUninstalling(true)
    try {
      const res = await enqueueSoftwareActions("uninstall", [id])
      toast.success(res.message || "Uninstall queued", {
        action: {
          label: "View queue",
          onClick: () => navigate("/softwares/installing"),
        },
      })
      void queryClient.invalidateQueries({
        queryKey: [SOFTWARES_FETCH_KEY, "queue"],
      })
      navigate("/softwares/installing")
    } catch (err) {
      toast.error(getRequestErrorMessage(err, "Could not queue uninstall"))
    } finally {
      setUninstalling(false)
    }
  }

  return (
    <ContentLoader
      title={software?.name || "Software"}
      description={software?.details}
      breadcrumb={[
        { label: "Softwares", to: "/softwares" },
        { label: software?.name || "Details" },
      ]}
      isLoading={isLoading || !sessionReady}
      error={withError(error, data)}
      showHeaderSeparator
      forMeta
    >
      {software ? (
        <SoftwareDetail
          software={software}
          versions={versions}
          latest={latest}
          installedVersion={installedVersion}
          isInstalled={isInstalled}
          hasUpdate={hasUpdate}
          osMissing={osMissing}
          uninstalled={uninstalled}
          canInstall={canInstall}
          canUninstall={canUninstall}
          canControl={canControl}
          serviceStatus={serviceStatus}
          packageManager={packageManager}
          canSwitchToBrew={canSwitchToBrew}
          canSwitchToLocal={canSwitchToLocal}
          switching={switchMutation.isPending}
          installing={installing}
          uninstalling={uninstalling}
          onInstall={() => void startInstall()}
          onUninstall={() => void startUninstall()}
          onSwitchManager={(target) => switchMutation.mutate(target)}
          onServiceStatusChange={handleServiceStatusChange}
          initialTab={canControl ? initialTab : "overview"}
          terminal={{
            open: terminalOpen,
            status: terminalStatus,
            lines: terminalLines,
            jobId,
            failureReason,
            cancelling,
            onCancel: () => void handleCancel(),
            onRetry: () => void startInstall(),
            onClear: () => {
              setTerminalLines([])
              setFailureReason(null)
            },
            onClose: handleCloseFailed,
          }}
        />
      ) : null}
    </ContentLoader>
  )
}
