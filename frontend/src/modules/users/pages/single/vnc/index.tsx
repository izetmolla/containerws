import {
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react"
import { Link, useOutletContext } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { zodResolver } from "@hookform/resolvers/zod"
import { useForm } from "react-hook-form"
import {
  ArrowRight,
  CheckCircle2,
  Circle,
  ExternalLink,
  ImageIcon,
  KeyRound,
  Loader2,
  Monitor,
  RefreshCw,
  Save,
  Upload,
  Wrench,
} from "lucide-react"
import { toast } from "sonner"

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
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { ReactSelect } from "@/components/ui/reactselect"
import { ReactSelectCreatable } from "@/components/ui/reactselectcreatable"
import {
  REQUEST_HEADER_AUTH_KEY,
  TOKEN_TYPE,
  toastRequestError,
} from "@/lib/network"
import { cn } from "@/lib/utils"
import { getCurrentTokens } from "@/store/authorization"
import useAuthorizationStore from "@/store/authorization"

import {
  getVncSetupStatus,
  VNC_SETUP_FETCH_KEY,
} from "@/modules/vnc-novnc/pages/settings/api"
import { openNovnc, type VncProfile } from "../../list/api"
import type { UserSingleOutletContext } from "../types"
import { InstallTerminal } from "@/modules/vnc-novnc/pages/settings/components/install-terminal"
import type { InstallTerminalStatus } from "@/modules/vnc-novnc/pages/settings/components/install-terminal"
import {
  createVncProfile,
  listVncBindAddresses,
  resetVncWallpaper,
  setVncPassword,
  startVncProfile,
  stopVncProfile,
  updateVncProfile,
  uploadVncWallpaper,
  vncCreateFormSchema,
  vncPasswordFormSchema,
  vncPasswordRules,
  VNC_PASSWORD_MAX,
  vncWallpaperSrc,
  type VncCreateFormValues,
  type VncPasswordFormValues,
  type VncSettingsInput,
} from "./api"
import {
  cancelRdpSetupJob,
  disableUserRdp,
  enableUserRdp,
  getUserRdp,
  startUserRdp,
  stopUserRdp,
  streamRdpSetup,
  updateUserRdp,
  USER_RDP_FETCH_KEY,
  type InstallTerminalLine,
  type RdpBindAddress,
} from "./rdp-api"

const GEOMETRY_PRESETS = [
  "3840x2160",
  "2560x1440",
  "1920x1080",
  "1680x1050",
  "1600x900",
  "1440x900",
  "1366x768",
  "1280x800",
  "1280x720",
  "1024x768",
  "800x600",
]

type SelectOption = { value: string; label: string }

const GEOMETRY_OPTIONS: SelectOption[] = GEOMETRY_PRESETS.map((g) => ({
  value: g,
  label: g,
}))
const DEPTH_OPTIONS: SelectOption[] = [16, 24, 32].map((d) => ({
  value: String(d),
  label: `${d}-bit`,
}))
const DPI_OPTIONS: SelectOption[] = [72, 96, 120, 144, 192].map((d) => ({
  value: String(d),
  label: String(d),
}))
const FRAMERATE_OPTIONS: SelectOption[] = [15, 24, 30, 45, 60].map((f) => ({
  value: String(f),
  label: `${f} fps`,
}))
const RESIZE_OPTIONS: SelectOption[] = [
  { value: "off", label: "Off (fixed)" },
  { value: "scale", label: "Scale locally" },
  { value: "remote", label: "Remote resize" },
]
const BELL_OPTIONS: SelectOption[] = [
  { value: "on", label: "On" },
  { value: "off", label: "Off" },
]
const LOGGING_OPTIONS: SelectOption[] = ["error", "warn", "info", "debug"].map(
  (level) => ({ value: level, label: level })
)
const DESKTOP_OPTIONS: SelectOption[] = [
  { value: "xfce", label: "XFCE" },
  { value: "lxde", label: "LXDE" },
  { value: "mate", label: "MATE" },
  { value: "gnome", label: "GNOME" },
]
const SECURITY_OPTIONS: SelectOption[] = [
  { value: "VncAuth", label: "VncAuth" },
  { value: "None", label: "None (insecure)" },
  { value: "VncAuth,TLSVnc", label: "VncAuth + TLSVnc" },
]
const COMPARE_FB_OPTIONS: SelectOption[] = [
  { value: "0", label: "Off" },
  { value: "1", label: "Auto" },
  { value: "2", label: "Always" },
]

type FormState = {
  address: string
  geometry: string
  depth: number
  dpi: number
  framerate: number
  always_shared: boolean
  accept_set_desktop_size: boolean
  security_types: string
  compare_fb: number
  improved_hextile: boolean
  desktop_session: string
  quality: number
  compression: number
  autoconnect: boolean
  reconnect: boolean
  reconnect_delay: number
  resize: string
  view_only: boolean
  show_dot: boolean
  view_clip: boolean
  shared: boolean
  bell: string
  logging: string
}

function profileToForm(v: VncProfile): FormState {
  return {
    address: v.address || "127.0.0.1",
    geometry: v.geometry || "1920x1080",
    depth: v.depth ?? 24,
    dpi: v.dpi ?? 96,
    framerate: v.framerate ?? 60,
    always_shared: v.always_shared ?? true,
    accept_set_desktop_size: v.accept_set_desktop_size ?? true,
    security_types: v.security_types || "VncAuth",
    compare_fb: v.compare_fb ?? 0,
    improved_hextile: v.improved_hextile ?? true,
    desktop_session: v.desktop_session || "xfce",
    quality: v.quality ?? 9,
    compression: v.compression ?? 0,
    autoconnect: v.autoconnect ?? true,
    reconnect: v.reconnect ?? true,
    reconnect_delay: v.reconnect_delay ?? 2000,
    resize: v.resize || "remote",
    view_only: v.view_only ?? false,
    show_dot: v.show_dot ?? false,
    view_clip: v.view_clip ?? false,
    shared: v.shared ?? true,
    bell: v.bell || "on",
    logging: v.logging || "warn",
  }
}

function defaultFormState(): FormState {
  return {
    address: "127.0.0.1",
    geometry: "1920x1080",
    depth: 24,
    dpi: 96,
    framerate: 60,
    always_shared: true,
    accept_set_desktop_size: true,
    security_types: "VncAuth",
    compare_fb: 0,
    improved_hextile: true,
    desktop_session: "xfce",
    quality: 9,
    compression: 0,
    autoconnect: true,
    reconnect: true,
    reconnect_delay: 2000,
    resize: "remote",
    view_only: false,
    show_dot: false,
    view_clip: false,
    shared: true,
    bell: "on",
    logging: "warn",
  }
}

function formsEqual(a: FormState, b: FormState) {
  return (Object.keys(a) as (keyof FormState)[]).every((key) => a[key] === b[key])
}

function CheckboxRow({
  checked,
  onChange,
  label,
  hint,
}: {
  checked: boolean
  onChange: (v: boolean) => void
  label: string
  hint?: string
}) {
  return (
    <label className="flex cursor-pointer items-start gap-3 rounded-lg border border-transparent px-1 py-1.5 hover:border-border/60 hover:bg-muted/30">
      <input
        type="checkbox"
        className="mt-1 size-4 shrink-0 accent-foreground"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
      />
      <span className="min-w-0">
        <span className="block text-sm font-medium leading-none">{label}</span>
        {hint ? (
          <span className="mt-1 block text-xs text-muted-foreground">{hint}</span>
        ) : null}
      </span>
    </label>
  )
}

function Field({
  label,
  hint,
  children,
}: {
  label: string
  hint?: string
  children: ReactNode
}) {
  return (
    <div className="grid gap-1.5">
      <Label className="text-sm">{label}</Label>
      {children}
      {hint ? <p className="text-xs text-muted-foreground">{hint}</p> : null}
    </div>
  )
}

function OptionGroup({
  title,
  description,
  actions,
  children,
}: {
  title: string
  description?: string
  actions?: ReactNode
  children: ReactNode
}) {
  return (
    <div className="rounded-lg border bg-muted/15 p-4">
      <div className="mb-3 flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h3 className="text-sm font-medium tracking-tight">{title}</h3>
          {description ? (
            <p className="mt-0.5 text-xs text-muted-foreground">{description}</p>
          ) : null}
        </div>
        {actions ? (
          <div className="flex shrink-0 flex-wrap items-center justify-end gap-2">
            {actions}
          </div>
        ) : null}
      </div>
      {children}
    </div>
  )
}

function Section({
  title,
  description,
  children,
  className,
}: {
  title: string
  description?: string
  children: ReactNode
  className?: string
}) {
  return (
    <section
      className={cn(
        "overflow-hidden rounded-xl border bg-card shadow-sm",
        className
      )}
    >
      <div className="border-b bg-gradient-to-br from-muted/40 via-muted/20 to-transparent px-5 py-4 md:px-6">
        <h2 className="text-base font-semibold tracking-tight">{title}</h2>
        {description ? (
          <p className="mt-1 max-w-2xl text-sm text-muted-foreground">
            {description}
          </p>
        ) : null}
      </div>
      <div className="grid gap-4 p-5 md:p-6">{children}</div>
    </section>
  )
}

export default function UserVncPage() {
  const { user, id, invalidate } = useOutletContext<UserSingleOutletContext>()
  const queryClient = useQueryClient()
  const [vncOpen, setVncOpen] = useState(false)
  const [passwordOpen, setPasswordOpen] = useState(false)
  const [form, setForm] = useState<FormState | null>(null)
  const [savedForm, setSavedForm] = useState<FormState | null>(null)
  const [, setCustomGeometry] = useState(false)

  const [wallpaperBust, setWallpaperBust] = useState(() => Date.now())
  const [wallpaperPreview, setWallpaperPreview] = useState<string | null>(null)
  const [rdpJobId, setRdpJobId] = useState<string | null>(null)
  const [rdpTermOpen, setRdpTermOpen] = useState(false)
  const [rdpTermStatus, setRdpTermStatus] =
    useState<InstallTerminalStatus>("idle")
  const [rdpLines, setRdpLines] = useState<InstallTerminalLine[]>([])
  const [rdpCancelling, setRdpCancelling] = useState(false)
  const rdpAbortRef = useRef<AbortController | null>(null)

  const [rdpAddress, setRdpAddress] = useState("127.0.0.1")
  const [shellEl, setShellEl] = useState<HTMLDivElement | null>(null)
  const [barBox, setBarBox] = useState<{
    left: number
    width: number
    bottom: number
  } | null>(null)

  useLayoutEffect(() => {
    if (!shellEl) return
    // Lock to the dashboard content column, not a nested layout wrapper.
    const main = shellEl.closest(
      '[class*="@container/main"]'
    ) as HTMLElement | null
    if (!main) return
    const panel = main.parentElement

    const sync = () => {
      const r = main.getBoundingClientRect()
      const panelBottom =
        panel?.getBoundingClientRect().bottom ?? window.innerHeight
      const next = {
        left: Math.round(r.left),
        width: Math.round(r.width),
        bottom: Math.max(0, Math.round(window.innerHeight - panelBottom)),
      }
      setBarBox((prev) =>
        prev &&
        prev.left === next.left &&
        prev.width === next.width &&
        prev.bottom === next.bottom
          ? prev
          : next
      )
    }
    sync()
    const ro = new ResizeObserver(sync)
    ro.observe(main)
    if (panel) ro.observe(panel)
    window.addEventListener("resize", sync)
    window.addEventListener("scroll", sync, true)
    return () => {
      ro.disconnect()
      window.removeEventListener("resize", sync)
      window.removeEventListener("scroll", sync, true)
    }
  }, [shellEl])

  const setupQuery = useQuery({
    queryKey: [VNC_SETUP_FETCH_KEY, "status"],
    queryFn: getVncSetupStatus,
  })

  const packagesReady = Boolean(setupQuery.data?.data?.ready)

  const addressesQuery = useQuery({
    queryKey: ["user-vnc", "addresses", id],
    queryFn: () => listVncBindAddresses(id),
    enabled: !!id && packagesReady,
  })

  const rdpQuery = useQuery({
    queryKey: [USER_RDP_FETCH_KEY, id],
    queryFn: () => getUserRdp(id),
    enabled: !!id && packagesReady,
  })

  const rdpAddrFromQuery = rdpQuery.data?.data?.rdp_address
  const [prevRdpAddrFromQuery, setPrevRdpAddrFromQuery] =
    useState(rdpAddrFromQuery)
  if (rdpAddrFromQuery !== prevRdpAddrFromQuery) {
    setPrevRdpAddrFromQuery(rdpAddrFromQuery)
    if (rdpAddrFromQuery) setRdpAddress(rdpAddrFromQuery)
  }

  const vnc = user.vnc
  const vncSyncKey = vnc
    ? `${vnc.id}:${vnc.updated_at ?? ""}:${[
        vnc.address,
        vnc.geometry,
        vnc.depth,
        vnc.dpi,
        vnc.framerate,
        vnc.always_shared,
        vnc.accept_set_desktop_size,
        vnc.security_types,
        vnc.compare_fb,
        vnc.improved_hextile,
        vnc.desktop_session,
        vnc.quality,
        vnc.compression,
        vnc.autoconnect,
        vnc.reconnect,
        vnc.reconnect_delay,
        vnc.resize,
        vnc.view_only,
        vnc.show_dot,
        vnc.view_clip,
        vnc.shared,
        vnc.bell,
        vnc.logging,
      ].join("|")}`
    : ""

  const [prevVncSyncKey, setPrevVncSyncKey] = useState(vncSyncKey)
  if (vncSyncKey !== prevVncSyncKey) {
    setPrevVncSyncKey(vncSyncKey)
    if (!vnc) {
      setForm(null)
      setSavedForm(null)
    } else {
      const next = profileToForm(vnc)
      setForm((prev) => {
        // Keep local edits while dirty; adopt server values only when clean.
        if (!prev || !savedForm || formsEqual(prev, savedForm)) {
          return next
        }
        return prev
      })
      setSavedForm(next)
      setCustomGeometry(!GEOMETRY_PRESETS.includes(next.geometry))
    }
  }

  const wallpaperSyncKey =
    vnc && id ? `${id}:${wallpaperBust}` : ""
  const [prevWallpaperSyncKey, setPrevWallpaperSyncKey] =
    useState(wallpaperSyncKey)
  if (wallpaperSyncKey !== prevWallpaperSyncKey) {
    setPrevWallpaperSyncKey(wallpaperSyncKey)
    if (!wallpaperSyncKey) setWallpaperPreview(null)
  }

  useEffect(() => {
    if (!vnc || !id) {
      return
    }
    let revoked = false
    let objectUrl: string | null = null
    const ctrl = new AbortController()
    ;(async () => {
      try {
        const tokens = getCurrentTokens(useAuthorizationStore.getState())
        const headers: Record<string, string> = {}
        if (tokens?.access_token) {
          headers[REQUEST_HEADER_AUTH_KEY] =
            `${TOKEN_TYPE}${tokens.access_token}`
        }
        const res = await fetch(vncWallpaperSrc(id, wallpaperBust), {
          headers,
          signal: ctrl.signal,
        })
        if (!res.ok) return
        const blob = await res.blob()
        if (revoked) return
        objectUrl = URL.createObjectURL(blob)
        setWallpaperPreview(objectUrl)
      } catch {
        // ignore abort / preview failures
      }
    })()
    return () => {
      revoked = true
      ctrl.abort()
      if (objectUrl) URL.revokeObjectURL(objectUrl)
    }
  }, [vnc, id, wallpaperBust])

  const patch = <K extends keyof FormState>(key: K, value: FormState[K]) => {
    setForm((prev) => (prev ? { ...prev, [key]: value } : prev))
  }

  const vncCreateMutation = useMutation({
    mutationFn: (body: { vnc_password: string; start: boolean }) =>
      createVncProfile(id, body),
    onSuccess: (res) => {
      toast.success(res.message || "VNC profile created")
      setVncOpen(false)
      invalidate()
      if (res.novnc_url) {
        openNovnc(res.novnc_url)
      }
    },
    onError: (err) => toastRequestError(err, "Failed to create VNC profile"),
  })

  const vncStartMutation = useMutation({
    mutationFn: () => startVncProfile(id),
    onSuccess: (res) => {
      toast.success(res.message || "Started")
      invalidate()
    },
    onError: (err) => {
      toastRequestError(err, "Failed to start VNC")
      const text = String(
        (err as { response?: { data?: { message?: string } } })?.response?.data
          ?.message || ""
      ).toLowerCase()
      if (text.includes("password")) {
        setPasswordOpen(true)
      }
    },
  })

  const vncStopMutation = useMutation({
    mutationFn: () => stopVncProfile(id),
    onSuccess: (res) => {
      toast.success(res.message || "Stopped")
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Failed to stop VNC"),
  })

  const saveMutation = useMutation({
    mutationFn: (body: VncSettingsInput) => updateVncProfile(id, body),
    onSuccess: (res) => {
      toast.success(res.message || "Settings saved")
      if (res.warning) toast.warning(res.warning)
      invalidate()
      void rdpQuery.refetch()
    },
    onError: (err) => toastRequestError(err, "Failed to save VNC settings"),
  })

  const passwordMutation = useMutation({
    mutationFn: (password: string) => setVncPassword(id, password),
    onSuccess: (res) => {
      toast.success(res.message || "Password updated")
      setPasswordOpen(false)
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Failed to update password"),
  })

  const wallpaperUploadMutation = useMutation({
    mutationFn: (file: File) => uploadVncWallpaper(id, file),
    onSuccess: (res) => {
      toast.success(res.message || "Wallpaper updated")
      if (res.warning) toast.warning(res.warning)
      setWallpaperBust(Date.now())
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Failed to upload wallpaper"),
  })

  const wallpaperResetMutation = useMutation({
    mutationFn: () => resetVncWallpaper(id),
    onSuccess: (res) => {
      toast.success(res.message || "Wallpaper reset")
      if (res.warning) toast.warning(res.warning)
      setWallpaperBust(Date.now())
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Failed to reset wallpaper"),
  })

  const rdpEnableMutation = useMutation({
    mutationFn: () => enableUserRdp(id, { rdp_address: rdpAddress }),
    onSuccess: (res) => {
      toast.success(res.message || "RDP enabled")
      if (res.data) {
        queryClient.setQueryData([USER_RDP_FETCH_KEY, id], res)
        if (res.data.rdp_address) setRdpAddress(res.data.rdp_address)
      }
      void rdpQuery.refetch()
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Failed to enable RDP"),
  })

  const rdpDisableMutation = useMutation({
    mutationFn: () => disableUserRdp(id),
    onSuccess: (res) => {
      toast.success(res.message || "RDP disabled")
      if (res.data) {
        queryClient.setQueryData([USER_RDP_FETCH_KEY, id], res)
      }
      void rdpQuery.refetch()
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Failed to disable RDP"),
  })

  const rdpAddressMutation = useMutation({
    mutationFn: (address: string) =>
      updateUserRdp(id, { rdp_address: address }),
    onSuccess: (res) => {
      toast.success(res.message || "RDP address updated")
      if (res.data?.rdp_address) setRdpAddress(res.data.rdp_address)
      void rdpQuery.refetch()
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Failed to update RDP address"),
  })

  const rdpStartMutation = useMutation({
    mutationFn: () => startUserRdp(id),
    onSuccess: (res) => {
      toast.success(res.message || "RDP service started")
      if (res.data) {
        queryClient.setQueryData([USER_RDP_FETCH_KEY, id], res)
      }
      void rdpQuery.refetch()
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Failed to start RDP service"),
  })

  const rdpStopMutation = useMutation({
    mutationFn: () => stopUserRdp(id),
    onSuccess: (res) => {
      toast.success(res.message || "RDP service stopped")
      if (res.data) {
        queryClient.setQueryData([USER_RDP_FETCH_KEY, id], res)
      }
      void rdpQuery.refetch()
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Failed to stop RDP service"),
  })

  const startRdpInstall = async (opts?: { reinstall?: boolean }) => {
    const reinstall = Boolean(opts?.reinstall)
    rdpAbortRef.current?.abort()
    const ac = new AbortController()
    rdpAbortRef.current = ac

    setRdpTermOpen(true)
    setRdpTermStatus("running")
    setRdpCancelling(false)
    setRdpJobId(null)
    let lineN = 0
    const pushLine = (
      text: string,
      stream: InstallTerminalLine["stream"] = "stdout"
    ) => {
      lineN += 1
      setRdpLines((prev) => [
        ...prev,
        { id: `${lineN}`, text, stream, at: Date.now() },
      ])
    }
    setRdpLines([])
    pushLine(
      reinstall
        ? "Connecting to RDP force-reinstall stream…"
        : "Connecting to RDP setup stream…",
      "system"
    )

    try {
      await streamRdpSetup({
        signal: ac.signal,
        reinstall,
        onEvent: (ev) => {
          if (ev.job_id) setRdpJobId(ev.job_id)
          switch (ev.type) {
            case "start":
              pushLine(
                ev.message ||
                  (reinstall
                    ? "Starting RDP force reinstall"
                    : "Starting RDP package setup"),
                "system"
              )
              break
            case "log":
              if (ev.line) {
                pushLine(
                  ev.line,
                  ev.stream === "stderr" ? "stderr" : "stdout"
                )
              }
              break
            case "error":
              setRdpTermStatus("error")
              pushLine(ev.message || "RDP install failed", "stderr")
              toast.error("RDP install failed", {
                description: ev.message || "Setup stream reported an error",
              })
              break
            case "cancelled":
              setRdpTermStatus("cancelled")
              pushLine(ev.message || "Installation stopped", "stderr")
              break
            case "done": {
              const ok = Boolean(ev.success)
              setRdpTermStatus(ok ? "success" : "error")
              pushLine(
                ev.message ||
                  (ok ? "RDP setup completed" : "RDP setup failed"),
                ok ? "system" : "stderr"
              )
              if (ok) {
                toast.success(ev.message || "RDP installed")
                void rdpQuery.refetch()
              } else {
                toast.error("RDP install failed", {
                  description: ev.message || "Setup finished with errors",
                })
              }
              break
            }
          }
        },
      })
      setRdpTermStatus((prev) => (prev === "running" ? "success" : prev))
    } catch (err) {
      if (ac.signal.aborted) {
        setRdpTermStatus((prev) =>
          prev === "running" ? "cancelled" : prev
        )
        return
      }
      setRdpTermStatus("error")
      pushLine(
        err instanceof Error ? err.message : "RDP install failed",
        "stderr"
      )
      toastRequestError(err, "RDP install failed")
    }
  }

  const dirty = useMemo(() => {
    if (!form || !savedForm) return false
    return !formsEqual(form, savedForm)
  }, [form, savedForm])

  const defaultsForReset = useMemo((): FormState | null => {
    if (!form) return null
    return {
      ...defaultFormState(),
      // Keep the current bind address — Reset targets desktop/client settings.
      address: form.address,
    }
  }, [form])

  const canResetSettings = useMemo(() => {
    if (!form || !defaultsForReset) return false
    return !formsEqual(form, defaultsForReset)
  }, [form, defaultsForReset])

  const resetSettings = () => {
    if (!defaultsForReset) return
    setForm(defaultsForReset)
    setCustomGeometry(false)
    saveMutation.mutate({ ...defaultsForReset, restart: true })
  }

  const discardChanges = () => {
    if (!savedForm || !vnc) return
    setForm(savedForm)
    setCustomGeometry(!GEOMETRY_PRESETS.includes(savedForm.geometry))
  }

  const actionPending =
    vncStartMutation.isPending ||
    vncStopMutation.isPending ||
    saveMutation.isPending

  if (setupQuery.isLoading) {
    return (
      <div className="w-full">
        <section className="rounded-xl border bg-card p-5 text-sm text-muted-foreground">
          Checking VNC / noVNC installation…
        </section>
      </div>
    )
  }

  if (!packagesReady) {
    const missing = setupQuery.data?.data?.missing?.filter(Boolean) ?? []
    return (
      <div className="w-full">
        <section className="grid gap-4 rounded-xl border border-dashed bg-muted/20 p-6">
          <div className="flex items-start gap-3">
            <div className="grid size-10 shrink-0 place-items-center rounded-xl bg-amber-500/15 text-amber-700 dark:text-amber-300">
              <Wrench className="size-4" />
            </div>
            <div className="min-w-0 space-y-1">
              <h2 className="font-semibold tracking-tight">
                VNC / noVNC is not installed
              </h2>
              <p className="text-sm text-muted-foreground">
                Install TigerVNC and noVNC on this host before creating a
                desktop profile for this user.
                {missing.length ? (
                  <>
                    {" "}
                    Missing:{" "}
                    <code className="text-xs">{missing.join(", ")}</code>.
                  </>
                ) : null}
              </p>
            </div>
          </div>
          <div>
            <Button asChild>
              <Link to="/vnc-novnc">
                Go to VNC &amp; noVNC settings
                <ArrowRight data-icon="inline-end" />
              </Link>
            </Button>
          </div>
        </section>
      </div>
    )
  }

  if (!vnc || !form) {
    return (
      <div className="w-full">
        <section className="grid gap-3 rounded-xl border bg-card p-5">
          <div className="flex items-center gap-2">
            <Monitor className="size-4" />
            <h2 className="font-semibold">VNC / noVNC</h2>
          </div>
          <p className="text-sm text-muted-foreground">
            No desktop profile yet. Create one to allocate localhost ports and
            configure resolution, quality, and client options.
          </p>
          <div>
            <Button onClick={() => setVncOpen(true)}>Create VNC profile</Button>
          </div>
        </section>

        <Dialog open={vncOpen} onOpenChange={(next) => !next && setVncOpen(false)}>
          <DialogContent className="sm:max-w-md">
            <DialogHeader>
              <DialogTitle>Create VNC profile</DialogTitle>
              <DialogDescription>
                Allocates free localhost ports and stores the session for this
                user. Choose a VNC password that meets the requirements below.
              </DialogDescription>
            </DialogHeader>
            <VncCreateForm
              pending={vncCreateMutation.isPending}
              onCancel={() => setVncOpen(false)}
              onSubmit={(body) => vncCreateMutation.mutate(body)}
            />
          </DialogContent>
        </Dialog>
      </div>
    )
  }

  const openDesktop = () => {
    const url = vnc.novnc_url || user.novnc_url
    if (url) openNovnc(url)
  }

  const onSave = () => {
    saveMutation.mutate({ ...form, restart: true })
  }

  return (
    <div ref={setShellEl} className="flex w-full flex-col gap-5">
      <section className="overflow-hidden rounded-xl border bg-card shadow-sm">
        <div className="flex flex-col gap-4 border-b bg-gradient-to-br from-muted/50 via-muted/30 to-transparent p-5 md:flex-row md:items-center md:justify-between md:p-6">
          <div className="flex min-w-0 items-start gap-3.5">
            <div className="grid size-12 shrink-0 place-items-center rounded-2xl bg-background shadow-sm ring-1 ring-border">
              <Monitor className="size-5 text-foreground/80" />
            </div>
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <h1 className="text-lg font-semibold tracking-tight">
                  Desktop session
                </h1>
                <span
                  className={cn(
                    "inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium capitalize ring-1 ring-inset",
                    vnc.live
                      ? "bg-emerald-500/12 text-emerald-700 ring-emerald-500/20 dark:text-emerald-300"
                      : vnc.status === "active"
                        ? "bg-amber-500/12 text-amber-800 ring-amber-500/20 dark:text-amber-300"
                        : "bg-muted text-muted-foreground ring-border"
                  )}
                >
                  <span
                    className={cn(
                      "size-1.5 rounded-full",
                      vnc.live
                        ? "bg-emerald-500"
                        : vnc.status === "active"
                          ? "bg-amber-500"
                          : "bg-muted-foreground/50"
                    )}
                    aria-hidden
                  />
                  {vnc.live ? "running" : vnc.status}
                </span>
              </div>
              <p className="mt-1.5 truncate text-sm text-muted-foreground">
                {form.geometry} · quality {form.quality}
                {form.address !== "127.0.0.1"
                  ? ` · RFB ${form.address}:${vnc.vnc_port || "…"}`
                  : " · panel proxy"}
              </p>
            </div>
          </div>

          <div className="flex flex-wrap gap-2">
            {vnc.live ? (
              <Button onClick={openDesktop}>
                Open desktop
                <ExternalLink data-icon="inline-end" />
              </Button>
            ) : null}
            {vnc.live ? (
              <Button
                variant="outline"
                disabled={actionPending}
                onClick={() => vncStopMutation.mutate()}
              >
                {vncStopMutation.isPending ? (
                  <>
                    <Loader2 className="size-3.5 animate-spin" />
                    Stopping…
                  </>
                ) : (
                  "Stop"
                )}
              </Button>
            ) : (
              <Button
                variant="outline"
                disabled={actionPending}
                onClick={() => {
                  if (!vnc.has_password) {
                    toast.message("Set a VNC password before starting the desktop")
                    setPasswordOpen(true)
                    return
                  }
                  vncStartMutation.mutate()
                }}
              >
                {vncStartMutation.isPending ? (
                  <>
                    <Loader2 className="size-3.5 animate-spin" />
                    Starting…
                  </>
                ) : (
                  "Start"
                )}
              </Button>
            )}
            <Button
              variant="outline"
              disabled={actionPending || passwordMutation.isPending}
              onClick={() => setPasswordOpen(true)}
            >
              {passwordMutation.isPending ? (
                <Loader2 className="size-3.5 animate-spin" />
              ) : (
                <KeyRound data-icon="inline-start" />
              )}
              {vnc.has_password ? "Reset password" : "Set password"}
            </Button>
            <Button
              variant="outline"
              disabled={actionPending || !canResetSettings}
              onClick={resetSettings}
              title="Restore default screen, encoding, desktop, security, and connection settings, then restart"
            >
              {saveMutation.isPending ? (
                <>
                  <Loader2 className="size-3.5 animate-spin" />
                  Resetting…
                </>
              ) : (
                "Reset"
              )}
            </Button>
          </div>
        </div>

        <dl
          className={cn(
            "grid gap-px bg-border sm:grid-cols-2",
            (rdpQuery.data?.data?.enabled || user.vnc?.rdp_enabled) &&
              "lg:grid-cols-4"
          )}
        >
          {(
            [
              [
                "Listen mode",
                form.address === "127.0.0.1" || form.address === "localhost"
                  ? "Localhost (proxied)"
                  : `Interface ${form.address}`,
              ],
              [
                "VNC port",
                vnc.vnc_port
                  ? `${form.address}:${vnc.vnc_port} (auto)`
                  : "Assigned on start",
              ],
              ...(rdpQuery.data?.data?.enabled || user.vnc?.rdp_enabled
                ? [
                    [
                      "RDP address",
                      rdpQuery.data?.data?.rdp_address ||
                        user.vnc?.rdp_address ||
                        rdpAddress ||
                        "—",
                    ],
                    [
                      "RDP port",
                      String(
                        rdpQuery.data?.data?.rdp_port ||
                          rdpQuery.data?.data?.port ||
                          user.vnc?.rdp_port ||
                          "—"
                      ),
                    ],
                  ]
                : []),
            ] as [string, string][]
          ).map(([label, value]) => (
            <div key={label} className="bg-card px-5 py-3">
              <dt className="text-xs text-muted-foreground">{label}</dt>
              <dd className="mt-0.5 font-mono text-sm">{value}</dd>
            </div>
          ))}
        </dl>
      </section>

      <Section
        title="Network"
        description="Listen addresses for TigerVNC (RFB) and optional RDP. Ports are assigned automatically."
      >
        {(() => {
          const rdp = rdpQuery.data?.data
          const ready = Boolean(rdp?.packages_ready)
          const enabled = Boolean(rdp?.enabled)
          const pending =
            rdpEnableMutation.isPending ||
            rdpDisableMutation.isPending ||
            rdpAddressMutation.isPending ||
            rdpStartMutation.isPending ||
            rdpStopMutation.isPending
          const addressOptions = (rdp?.addresses ?? []).map(
            (opt: RdpBindAddress) => ({
              value: opt.address,
              label: opt.label,
            })
          )
          const selectedAddress =
            addressOptions.find((o) => o.value === rdpAddress) ||
            (rdpAddress ? { value: rdpAddress, label: rdpAddress } : null)
          const port = rdp?.rdp_port || rdp?.port || user.vnc?.rdp_port || 0
          const serviceRunning = Boolean(rdp?.service_running)

          return (
            <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
              <OptionGroup
                title="VNC listen address"
                description="Where the RFB server binds on this host."
              >
                <Field
                  label="Interface"
                  hint={
                    form.address === "127.0.0.1"
                      ? "Localhost — desktop only via /novnc."
                      : "LAN/public bind — native VNC clients can connect to address:port."
                  }
                >
                  <ReactSelect<SelectOption, false>
                    size="sm"
                    options={[
                      ...(addressesQuery.data?.data ?? []).map((opt) => ({
                        value: opt.address,
                        label: opt.label,
                      })),
                      ...(!addressesQuery.data?.data?.some(
                        (o) => o.address === form.address
                      ) && form.address
                        ? [{ value: form.address, label: form.address }]
                        : []),
                    ]}
                    value={form.address}
                    onValueChange={(v) => v && patch("address", v)}
                    placeholder="Select address"
                    isSearchable
                  />
                </Field>
                {vnc.vnc_port ? (
                  <p className="mt-3 font-mono text-xs text-muted-foreground">
                    {form.address}:{vnc.vnc_port}
                  </p>
                ) : null}
              </OptionGroup>

              <OptionGroup
                title="RDP listen address"
                description="Xvnc attaches to this user’s TigerVNC desktop (same session as noVNC)."
                actions={
                  ready ? (
                    <>
                      {enabled ? (
                        <>
                          {serviceRunning ? (
                            <Button
                              size="sm"
                              variant="outline"
                              disabled={pending}
                              onClick={() => rdpStopMutation.mutate()}
                            >
                              {rdpStopMutation.isPending
                                ? "Stopping…"
                                : "Stop"}
                            </Button>
                          ) : (
                            <Button
                              size="sm"
                              variant="outline"
                              disabled={pending}
                              onClick={() => rdpStartMutation.mutate()}
                            >
                              {rdpStartMutation.isPending
                                ? "Starting…"
                                : "Start"}
                            </Button>
                          )}
                          <Button
                            size="sm"
                            variant="outline"
                            disabled={pending}
                            onClick={() => rdpDisableMutation.mutate()}
                          >
                            {rdpDisableMutation.isPending
                              ? "Disabling…"
                              : "Disable RDP"}
                          </Button>
                        </>
                      ) : (
                        <Button
                          size="sm"
                          disabled={pending}
                          onClick={() => rdpEnableMutation.mutate()}
                        >
                          {rdpEnableMutation.isPending
                            ? "Enabling…"
                            : "Enable RDP"}
                        </Button>
                      )}
                      <Button
                        size="sm"
                        variant="ghost"
                        disabled={rdpTermStatus === "running"}
                        onClick={() => void startRdpInstall({ reinstall: true })}
                      >
                        Reinstall
                      </Button>
                    </>
                  ) : (
                    <Button
                      size="sm"
                      disabled={rdpTermStatus === "running"}
                      onClick={() => void startRdpInstall()}
                    >
                      <Wrench data-icon="inline-start" />
                      Install
                    </Button>
                  )
                }
              >
                <div className="mb-3 flex flex-wrap items-center gap-1.5">
                  <span
                    className={cn(
                      "inline-flex rounded-full px-2 py-0.5 text-[11px] font-medium",
                      ready
                        ? "bg-emerald-500/15 text-emerald-700 dark:text-emerald-300"
                        : "bg-amber-500/15 text-amber-800 dark:text-amber-300"
                    )}
                  >
                    {ready ? "xrdp installed" : "xrdp missing"}
                  </span>
                  {ready ? (
                    <span
                      className={cn(
                        "inline-flex rounded-full px-2 py-0.5 text-[11px] font-medium",
                        rdp?.service_running
                          ? "bg-emerald-500/15 text-emerald-700 dark:text-emerald-300"
                          : "bg-muted text-muted-foreground"
                      )}
                    >
                      {rdp?.service_running
                        ? "service running"
                        : "service stopped"}
                    </span>
                  ) : null}
                  {enabled ? (
                    <span className="inline-flex rounded-full bg-sky-500/15 px-2 py-0.5 text-[11px] font-medium text-sky-800 dark:text-sky-300">
                      enabled
                    </span>
                  ) : null}
                </div>

                {!ready ? (
                  <p className="text-xs text-muted-foreground">
                    Install xrdp for this host. This does not affect VNC packages.
                    {rdp?.missing?.length ? (
                      <>
                        {" "}
                        Missing:{" "}
                        <code className="text-[11px]">
                          {rdp.missing.join(", ")}
                        </code>
                      </>
                    ) : null}
                  </p>
                ) : (
                  <div className="grid gap-4">
                    <div className="grid grid-cols-[minmax(0,1fr)_5.5rem] items-start gap-3">
                      <Field
                        label="Interface"
                        hint="Localhost = host only."
                      >
                        <ReactSelect<SelectOption, false>
                          size="sm"
                          options={addressOptions}
                          value={selectedAddress}
                          isDisabled={pending}
                          onValueChange={(v) => {
                            const next = v || "127.0.0.1"
                            setRdpAddress(next)
                            if (enabled) {
                              rdpAddressMutation.mutate(next)
                            }
                          }}
                          placeholder="Select host address"
                          isSearchable
                        />
                      </Field>
                      <Field label="Port">
                        <div className="flex h-8 items-center justify-center rounded-md border bg-muted/40 px-2 font-mono text-sm tabular-nums">
                          {port > 0 ? port : "—"}
                        </div>
                      </Field>
                    </div>
                    {rdp?.connect_hint || port > 0 ? (
                      <p className="text-xs text-muted-foreground">
                        {rdp?.connect_hint ||
                          `${rdpAddress}:${port} · ${user.username || "user"} · VNC password`}
                      </p>
                    ) : null}
                  </div>
                )}
              </OptionGroup>
            </div>
          )
        })()}
      </Section>

      <Section
        title="Desktop wallpaper"
        description="Custom XFCE wallpaper for this user. Applied live when the desktop is running."
      >
        <div className="grid gap-4 md:grid-cols-[220px_1fr]">
          <div className="overflow-hidden rounded-lg border bg-muted/30">
            <img
              src={wallpaperPreview || undefined}
              alt="Desktop wallpaper preview"
              className="aspect-video h-full w-full bg-muted object-cover"
            />
          </div>
          <div className="flex flex-col justify-center gap-3">
            <p className="text-sm text-muted-foreground">
              JPG, PNG, or WebP up to 12MB.
              {vnc.has_wallpaper
                ? " Custom wallpaper is active."
                : " Using the system default wallpaper."}
            </p>
            <div className="flex flex-wrap gap-2">
              <Button
                type="button"
                variant="outline"
                disabled={wallpaperUploadMutation.isPending}
                onClick={() => {
                  const input = document.createElement("input")
                  input.type = "file"
                  input.accept =
                    "image/jpeg,image/png,image/webp,.jpg,.jpeg,.png,.webp"
                  input.onchange = () => {
                    const file = input.files?.[0]
                    if (file) wallpaperUploadMutation.mutate(file)
                  }
                  input.click()
                }}
              >
                <Upload data-icon="inline-start" />
                {wallpaperUploadMutation.isPending
                  ? "Uploading…"
                  : "Upload image"}
              </Button>
              <Button
                type="button"
                variant="ghost"
                disabled={
                  !vnc.has_wallpaper || wallpaperResetMutation.isPending
                }
                onClick={() => wallpaperResetMutation.mutate()}
              >
                <ImageIcon data-icon="inline-start" />
                Reset default
              </Button>
            </div>
          </div>
        </div>
      </Section>

      <div className="grid gap-5 xl:grid-cols-2">
        <Section
          title="Display"
          description="Framebuffer and desktop environment applied when the session starts."
        >
          <OptionGroup
            title="Screen"
            description="Resolution, color depth, DPI, and refresh rate."
          >
            <div className="grid gap-4 sm:grid-cols-2">
              <Field
                label="Resolution"
                hint="Preset or type a custom WxH value"
              >
                <ReactSelectCreatable<SelectOption, false>
                  size="sm"
                  options={GEOMETRY_OPTIONS}
                  value={form.geometry}
                  onValueChange={(v) => {
                    if (!v) return
                    patch("geometry", v)
                    setCustomGeometry(!GEOMETRY_PRESETS.includes(v))
                  }}
                  formatCreateLabel={(input) => `Use “${input}”`}
                  placeholder="1920x1080"
                  isSearchable
                />
              </Field>
              <Field label="Color depth">
                <ReactSelect<SelectOption, false>
                  size="sm"
                  options={DEPTH_OPTIONS}
                  value={String(form.depth)}
                  onValueChange={(v) => v && patch("depth", Number(v))}
                />
              </Field>
              <Field label="DPI" hint="Font / Xft scaling">
                <ReactSelect<SelectOption, false>
                  size="sm"
                  options={DPI_OPTIONS}
                  value={String(form.dpi)}
                  onValueChange={(v) => v && patch("dpi", Number(v))}
                />
              </Field>
              <Field label="Frame rate" hint="Max RFB updates / sec">
                <ReactSelect<SelectOption, false>
                  size="sm"
                  options={FRAMERATE_OPTIONS}
                  value={String(form.framerate)}
                  onValueChange={(v) => v && patch("framerate", Number(v))}
                />
              </Field>
            </div>
          </OptionGroup>

          <OptionGroup
            title="Desktop environment"
            description="Session type for VNC and RDP when the client does not choose one."
          >
            <Field label="Session">
              <ReactSelect<SelectOption, false>
                size="sm"
                options={DESKTOP_OPTIONS}
                value={form.desktop_session}
                onValueChange={(v) => v && patch("desktop_session", v)}
              />
            </Field>
          </OptionGroup>
        </Section>

        <Section
          title="Streaming"
          description="Encoding quality and how the browser adapts to the remote desktop size."
        >
          <OptionGroup title="Tight encoding">
            <div className="grid gap-5">
              <Field
                label={`JPEG quality · ${form.quality}`}
                hint="0 = lowest / fastest · 9 = highest fidelity"
              >
                <input
                  type="range"
                  min={0}
                  max={9}
                  step={1}
                  value={form.quality}
                  onChange={(e) => patch("quality", Number(e.target.value))}
                  className="w-full accent-foreground"
                />
              </Field>
              <Field
                label={`Compression · ${form.compression}`}
                hint="0 = none · 9 = max (slower encode, smaller stream)"
              >
                <input
                  type="range"
                  min={0}
                  max={9}
                  step={1}
                  value={form.compression}
                  onChange={(e) =>
                    patch("compression", Number(e.target.value))
                  }
                  className="w-full accent-foreground"
                />
              </Field>
            </div>
          </OptionGroup>
          <OptionGroup title="Viewport">
            <Field
              label="Resize mode"
              hint="How the desktop adapts to the browser window"
            >
              <ReactSelect<SelectOption, false>
                size="sm"
                options={RESIZE_OPTIONS}
                value={form.resize}
                onValueChange={(v) => v && patch("resize", v)}
              />
            </Field>
          </OptionGroup>
        </Section>

        <Section
          title="Server"
          description="TigerVNC security, sharing, and encoding flags written on start."
        >
          <OptionGroup title="Security">
            <div className="grid gap-4 sm:grid-cols-2">
              <Field label="Security types">
                <ReactSelect<SelectOption, false>
                  size="sm"
                  options={SECURITY_OPTIONS}
                  value={form.security_types}
                  onValueChange={(v) => v && patch("security_types", v)}
                />
              </Field>
              <Field
                label="CompareFB"
                hint="Skip unchanged framebuffer tiles"
              >
                <ReactSelect<SelectOption, false>
                  size="sm"
                  options={COMPARE_FB_OPTIONS}
                  value={String(form.compare_fb)}
                  onValueChange={(v) => v && patch("compare_fb", Number(v))}
                />
              </Field>
            </div>
          </OptionGroup>
          <OptionGroup title="Sharing & encoding">
            <div className="grid gap-1 sm:grid-cols-1">
              <CheckboxRow
                checked={form.always_shared}
                onChange={(v) => patch("always_shared", v)}
                label="Always shared"
                hint="Allow multiple simultaneous viewers"
              />
              <CheckboxRow
                checked={form.accept_set_desktop_size}
                onChange={(v) => patch("accept_set_desktop_size", v)}
                label="Accept SetDesktopSize"
                hint="Allow clients to change remote resolution"
              />
              <CheckboxRow
                checked={form.improved_hextile}
                onChange={(v) => patch("improved_hextile", v)}
                label="Improved Hextile"
                hint="Better Hextile encoding on the server"
              />
            </div>
          </OptionGroup>
        </Section>

        <Section
          title="Browser client"
          description="noVNC connection behaviour passed as vnc.html query parameters."
        >
          <OptionGroup title="Connection">
            <div className="grid gap-4 sm:grid-cols-2">
              <Field label="Reconnect delay (ms)">
                <Input
                  type="number"
                  min={0}
                  step={100}
                  value={form.reconnect_delay}
                  onChange={(e) =>
                    patch("reconnect_delay", Number(e.target.value) || 0)
                  }
                />
              </Field>
              <Field label="Bell">
                <ReactSelect<SelectOption, false>
                  size="sm"
                  options={BELL_OPTIONS}
                  value={form.bell}
                  onValueChange={(v) => v && patch("bell", v)}
                />
              </Field>
              <Field label="Logging level" hint="Browser console verbosity">
                <ReactSelect<SelectOption, false>
                  size="sm"
                  options={LOGGING_OPTIONS}
                  value={form.logging}
                  onValueChange={(v) => v && patch("logging", v)}
                />
              </Field>
            </div>
          </OptionGroup>
          <OptionGroup title="Client behaviour">
            <div className="grid gap-1">
              <CheckboxRow
                checked={form.autoconnect}
                onChange={(v) => patch("autoconnect", v)}
                label="Autoconnect"
                hint="Connect as soon as vnc.html loads"
              />
              <CheckboxRow
                checked={form.reconnect}
                onChange={(v) => patch("reconnect", v)}
                label="Auto reconnect"
                hint="Retry after disconnect"
              />
              <CheckboxRow
                checked={form.shared}
                onChange={(v) => patch("shared", v)}
                label="Shared session"
                hint="Request a shared RFB connection"
              />
              <CheckboxRow
                checked={form.view_only}
                onChange={(v) => patch("view_only", v)}
                label="View only"
                hint="Disable keyboard and pointer input"
              />
              <CheckboxRow
                checked={form.show_dot}
                onChange={(v) => patch("show_dot", v)}
                label="Show local cursor dot"
                hint="Draw a local cursor marker"
              />
              <CheckboxRow
                checked={form.view_clip}
                onChange={(v) => patch("view_clip", v)}
                label="Clip to viewport"
                hint="Clip oversized remote desktop to the window"
              />
            </div>
          </OptionGroup>
        </Section>
      </div>

      {rdpTermOpen ? (
        <div className="fixed inset-x-3 bottom-3 z-50 mx-auto max-w-4xl md:inset-x-auto md:right-6 md:left-auto md:w-[min(42rem,calc(100vw-2rem))]">
          <InstallTerminal
            open={rdpTermOpen}
            status={rdpTermStatus}
            lines={rdpLines}
            title="RDP package install"
            subtitle="xrdp setup stream"
            cancelling={rdpCancelling}
            onStop={async () => {
              setRdpCancelling(true)
              try {
                if (rdpJobId) await cancelRdpSetupJob(rdpJobId)
                rdpAbortRef.current?.abort()
                setRdpTermStatus("cancelled")
              } finally {
                setRdpCancelling(false)
              }
            }}
            onClose={() => setRdpTermOpen(false)}
            onClear={() => setRdpLines([])}
            onRetry={() => void startRdpInstall({ reinstall: true })}
          />
        </div>
      ) : null}

      {/* Pinned to bottom of @container/main (not full viewport / sidebar) */}
      <div className="h-14 shrink-0" aria-hidden />
      {barBox ? (
        <section
          className="fixed z-30 border-t bg-card/95 px-(--content-padding) py-2.5 shadow-[0_-6px_20px_rgba(0,0,0,0.06)] backdrop-blur supports-[backdrop-filter]:bg-card/90 dark:shadow-[0_-6px_20px_rgba(0,0,0,0.35)]"
          style={{
            left: barBox.left,
            width: barBox.width,
            bottom: barBox.bottom,
          }}
        >
          <div className="flex w-full flex-wrap items-center justify-between gap-2">
            <div className="flex min-w-0 flex-wrap items-center gap-2">
              <p className="px-2 text-xs text-muted-foreground">
                Save restarts the desktop
                {rdpQuery.data?.data?.enabled ? " and RDP" : ""} so new settings
                apply.
              </p>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setPasswordOpen(true)}
              >
                Password
              </Button>
              <Button variant="ghost" size="sm" asChild>
                <Link to="/vnc-novnc">Packages</Link>
              </Button>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={actionPending || !dirty}
                onClick={discardChanges}
              >
                Discard
              </Button>
              <Button
                size="sm"
                disabled={actionPending || !dirty}
                onClick={onSave}
              >
                {saveMutation.isPending ? (
                  <>
                    <RefreshCw className="size-3.5 animate-spin" />
                    Saving…
                  </>
                ) : (
                  <>
                    <Save data-icon="inline-start" />
                    Save
                  </>
                )}
              </Button>
            </div>
          </div>
        </section>
      ) : null}

      <Dialog
        open={passwordOpen}
        onOpenChange={(next) => !next && setPasswordOpen(false)}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>
              {vnc.has_password ? "Reset VNC password" : "Set VNC password"}
            </DialogTitle>
            <DialogDescription>
              TigerVNC VncAuth stores at most {VNC_PASSWORD_MAX} characters. The
              desktop restarts automatically so the new password applies
              immediately.
            </DialogDescription>
          </DialogHeader>
          <PasswordForm
            pending={passwordMutation.isPending}
            submitLabel={
              vnc.has_password ? "Update password" : "Set password"
            }
            onCancel={() => setPasswordOpen(false)}
            onSubmit={(password) => passwordMutation.mutate(password)}
          />
        </DialogContent>
      </Dialog>
    </div>
  )
}

function VncPasswordRulesList({ password }: { password: string }) {
  return (
    <ul className="space-y-1.5 rounded-lg border bg-muted/40 p-3">
      {vncPasswordRules.map((rule) => {
        const ok = rule.test(password)
        return (
          <li
            key={rule.id}
            className={cn(
              "flex items-start gap-2 text-xs leading-snug",
              ok ? "text-emerald-700 dark:text-emerald-300" : "text-muted-foreground"
            )}
          >
            {ok ? (
              <CheckCircle2 className="mt-0.5 size-3.5 shrink-0" />
            ) : (
              <Circle className="mt-0.5 size-3.5 shrink-0" />
            )}
            <span>{rule.label}</span>
          </li>
        )
      })}
    </ul>
  )
}

function VncCreateForm({
  pending,
  onCancel,
  onSubmit,
}: {
  pending: boolean
  onCancel: () => void
  onSubmit: (body: { vnc_password: string; start: boolean }) => void
}) {
  const form = useForm<VncCreateFormValues>({
    resolver: zodResolver(vncCreateFormSchema),
    defaultValues: {
      password: "",
      confirm_password: "",
      start: true,
    },
    mode: "onChange",
  })
  const password = form.watch("password")

  return (
    <Form {...form} schema={vncCreateFormSchema}>
      <form
        className="grid gap-4"
        onSubmit={form.handleSubmit((values) =>
          onSubmit({
            vnc_password: values.password.trim(),
            start: values.start,
          })
        )}
      >
        <div className="space-y-2">
          <p className="text-xs font-medium text-foreground">
            Password requirements
          </p>
          <VncPasswordRulesList password={password} />
        </div>

        <FormPasswordField
          control={form.control}
          name="password"
          label="VNC password"
          autoComplete="new-password"
          maxLength={VNC_PASSWORD_MAX}
          placeholder={`e.g. Desk7op`}
        />
        <FormPasswordField
          control={form.control}
          name="confirm_password"
          label="Confirm password"
          autoComplete="new-password"
          maxLength={VNC_PASSWORD_MAX}
        />

        <label className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            className="size-4 accent-foreground"
            checked={form.watch("start")}
            onChange={(e) => form.setValue("start", e.target.checked)}
          />
          Start desktop after create
        </label>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={onCancel}>
            Cancel
          </Button>
          <Button type="submit" disabled={!form.formState.isValid || pending}>
            {pending ? "Creating…" : "Create"}
          </Button>
        </DialogFooter>
      </form>
    </Form>
  )
}

function PasswordForm({
  pending,
  submitLabel = "Update password",
  onCancel,
  onSubmit,
}: {
  pending: boolean
  submitLabel?: string
  onCancel: () => void
  onSubmit: (password: string) => void
}) {
  const form = useForm<VncPasswordFormValues>({
    resolver: zodResolver(vncPasswordFormSchema),
    defaultValues: {
      password: "",
      confirm_password: "",
    },
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
        <div className="space-y-2">
          <p className="text-xs font-medium text-foreground">
            Password requirements
          </p>
          <VncPasswordRulesList password={password} />
        </div>

        <FormPasswordField
          control={form.control}
          name="password"
          label="New VNC password"
          autoComplete="new-password"
          maxLength={VNC_PASSWORD_MAX}
          placeholder={`Up to ${VNC_PASSWORD_MAX} characters`}
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
            {pending ? "Saving…" : submitLabel}
          </Button>
        </DialogFooter>
      </form>
    </Form>
  )
}
