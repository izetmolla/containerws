import { useMemo, useState } from "react"
import { Link, useNavigate, useOutletContext } from "react-router"
import { useMutation, useQuery } from "@tanstack/react-query"
import {
  Check,
  Copy,
  OctagonX,
  Pause,
  Play,
  RotateCcw,
  Square,
  Trash2,
} from "lucide-react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { ButtonGroup } from "@/components/ui/button-group"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { asArray } from "@/lib/as-array"
import { toastRequestError } from "@/lib/network"
import { cn } from "@/lib/utils"

import { formatBytes, stateBadgeClass } from "../../_shared/engine-format"
import { KeyValueList, MetaGrid } from "../../_shared/resource-ui"
import { listNetworks } from "../../networks/api"
import {
  commitContainer,
  connectContainerNetwork,
  disconnectContainerNetwork,
  killContainer,
  pauseContainer,
  recreateContainer,
  removeContainer,
  restartContainer,
  resumeContainer,
  startContainer,
  stopContainer,
  updateRestartPolicy,
} from "../list/api"
import type { ContainerOutletContext } from "./layout"

const RESTART_POLICIES = [
  { value: "no", label: "No" },
  { value: "always", label: "Always" },
  { value: "on-failure", label: "On failure" },
  { value: "unless-stopped", label: "Unless stopped" },
] as const

function formatTs(value?: string) {
  if (!value || value.startsWith("0001-01-01")) return "—"
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, "0")
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

function Section({
  icon,
  title,
  description,
  children,
  actions,
}: {
  icon?: React.ReactNode
  title: string
  description?: string
  children: React.ReactNode
  actions?: React.ReactNode
}) {
  return (
    <Card className="gap-0 py-0">
      <CardHeader className="flex flex-row flex-wrap items-start justify-between gap-3 border-b py-4">
        <div className="space-y-1">
          <CardTitle className="flex items-center gap-2 text-base">
            {icon}
            {title}
          </CardTitle>
          {description ? (
            <CardDescription>{description}</CardDescription>
          ) : null}
        </div>
        {actions}
      </CardHeader>
      <CardContent className="py-5">{children}</CardContent>
    </Card>
  )
}

function MetaRow({
  label,
  children,
}: {
  label: string
  children: React.ReactNode
}) {
  return (
    <div className="grid gap-1 sm:grid-cols-[9rem_1fr] sm:items-start sm:gap-4">
      <dt className="text-sm text-muted-foreground">{label}</dt>
      <dd className="min-w-0 text-sm break-all">{children}</dd>
    </div>
  )
}

type LifecycleAction =
  | "start"
  | "stop"
  | "kill"
  | "restart"
  | "pause"
  | "resume"
  | "remove"

type ConfirmState =
  | { kind: "lifecycle"; action: LifecycleAction }
  | { kind: "duplicate" }
  | { kind: "leave"; networkId: string; networkName: string }
  | null

const LIFECYCLE_COPY: Record<
  LifecycleAction,
  { title: string; description: (name: string) => string; confirm: string; destructive?: boolean }
> = {
  start: {
    title: "Start container",
    description: (name) => `Start container ${name}?`,
    confirm: "Start",
  },
  stop: {
    title: "Stop container",
    description: (name) =>
      `Stop container ${name}? Running processes will be terminated.`,
    confirm: "Stop",
  },
  kill: {
    title: "Kill container",
    description: (name) =>
      `Send SIGKILL to container ${name}? This force-stops it immediately.`,
    confirm: "Kill",
    destructive: true,
  },
  restart: {
    title: "Restart container",
    description: (name) => `Restart container ${name}?`,
    confirm: "Restart",
  },
  pause: {
    title: "Pause container",
    description: (name) => `Pause all processes in container ${name}?`,
    confirm: "Pause",
  },
  resume: {
    title: "Resume container",
    description: (name) => `Resume paused container ${name}?`,
    confirm: "Resume",
  },
  remove: {
    title: "Remove container",
    description: (name) =>
      `Remove container ${name}? This cannot be undone.`,
    confirm: "Remove",
    destructive: true,
  },
}

export default function ContainerOverviewPage() {
  const navigate = useNavigate()
  const { id, container, invalidate } =
    useOutletContext<ContainerOutletContext>()
  const insp = container.inspect
  const state = (container.state || "").toLowerCase()
  const running = state === "running"
  const paused = state === "paused"
  const stopped = !running && !paused

  const [restartPolicy, setRestartPolicy] = useState(
    insp?.HostConfig?.RestartPolicy?.Name || "no"
  )
  const [joinNetworkId, setJoinNetworkId] = useState("")
  const [commitRepo, setCommitRepo] = useState(container.name || "")
  const [commitTag, setCommitTag] = useState("latest")
  const [recreateOpen, setRecreateOpen] = useState(false)
  const [recreatePull, setRecreatePull] = useState(false)
  const [confirm, setConfirm] = useState<ConfirmState>(null)

  const [prevContainerId, setPrevContainerId] = useState(container.id)
  if (container.id !== prevContainerId) {
    setPrevContainerId(container.id)
    setRestartPolicy(insp?.HostConfig?.RestartPolicy?.Name || "no")
    setCommitRepo(container.name || "")
    setJoinNetworkId("")
  }

  const networksQuery = useQuery({
    queryKey: ["docker-networks", "container-join"],
    queryFn: listNetworks,
  })
  const allNetworks = asArray(networksQuery.data?.data)

  const attached = useMemo(() => {
    const map = insp?.NetworkSettings?.Networks || {}
    return Object.entries(map)
      .filter(([, v]) => v != null)
      .map(([name, ep]) => ({
        name,
        networkId: ep?.NetworkID || name,
        ip:
          ep?.IPAddress && ep.IPPrefixLen
            ? `${ep.IPAddress}/${ep.IPPrefixLen}`
            : ep?.IPAddress || "—",
        ipv6: ep?.GlobalIPv6Address || "—",
        gateway: ep?.Gateway || "—",
        mac: ep?.MacAddress || "—",
      }))
  }, [insp?.NetworkSettings?.Networks])

  const attachedNames = new Set(attached.map((n) => n.name))
  const joinable = allNetworks.filter((n) => !attachedNames.has(n.name))

  const envRecord = useMemo(() => {
    const out: Record<string, string> = {}
    for (const line of asArray(insp?.Config?.Env)) {
      const i = line.indexOf("=")
      if (i < 0) out[line] = ""
      else out[line.slice(0, i)] = line.slice(i + 1)
    }
    return out
  }, [insp?.Config?.Env])
  const labels = insp?.Config?.Labels || {}
  const mounts = asArray(insp?.Mounts)
  const entrypoint = (insp?.Config?.Entrypoint || []).join(" ") || "—"
  const cmd = (insp?.Config?.Cmd || []).join(" ") || "—"
  const imageRef = insp?.Config?.Image || container.image
  const imageId = insp?.Image || "—"
  const ports = asArray(container.ports)
  const hc = insp?.HostConfig
  const st = insp?.State
  const nanoCpus = hc?.NanoCpus
  const cpuLabel =
    nanoCpus && nanoCpus > 0 ? `${(nanoCpus / 1e9).toFixed(2)} CPUs` : "—"
  const memLabel =
    hc?.Memory && hc.Memory > 0 ? formatBytes(hc.Memory) : "—"

  const actionMutation = useMutation({
    mutationFn: async (
      action:
        | "start"
        | "stop"
        | "kill"
        | "restart"
        | "pause"
        | "resume"
        | "remove"
        | { kind: "recreate"; pull: boolean }
    ) => {
      if (typeof action === "object") {
        return recreateContainer(id, { pull: action.pull })
      }
      switch (action) {
        case "start":
          return startContainer(id)
        case "stop":
          return stopContainer(id)
        case "kill":
          return killContainer(id)
        case "restart":
          return restartContainer(id)
        case "pause":
          return pauseContainer(id)
        case "resume":
          return resumeContainer(id)
        case "remove":
          return removeContainer(id, { force: true })
      }
    },
    onSuccess: (res, action) => {
      const kind = typeof action === "object" ? action.kind : action
      toast.success(res.message || `${kind} done`)
      if (kind === "remove") {
        navigate("/docker/containers")
        return
      }
      if (kind === "recreate") {
        setRecreateOpen(false)
        setRecreatePull(false)
        invalidate()
        if ("data" in res && res.data?.id) {
          navigate(`/docker/containers/${res.data.id}`)
        }
        return
      }
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Action failed"),
  })

  const restartMutation = useMutation({
    mutationFn: () =>
      updateRestartPolicy(id, { name: restartPolicy }),
    onSuccess: (res) => {
      toast.success(res.message || "Restart policy updated")
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Update failed"),
  })

  const joinMutation = useMutation({
    mutationFn: () => connectContainerNetwork(id, joinNetworkId),
    onSuccess: (res) => {
      toast.success(res.message || "Joined network")
      setJoinNetworkId("")
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Join failed"),
  })

  const leaveMutation = useMutation({
    mutationFn: (networkId: string) =>
      disconnectContainerNetwork(id, networkId, true),
    onSuccess: (res) => {
      toast.success(res.message || "Left network")
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Leave failed"),
  })

  const commitMutation = useMutation({
    mutationFn: () =>
      commitContainer(id, {
        repository: commitRepo.trim(),
        tag: commitTag.trim() || "latest",
      }),
    onSuccess: (res) => {
      toast.success(res.message || `Created ${res.data?.reference}`)
    },
    onError: (err) => toastRequestError(err, "Create image failed"),
  })

  const busy =
    actionMutation.isPending ||
    restartMutation.isPending ||
    joinMutation.isPending ||
    leaveMutation.isPending ||
    commitMutation.isPending

  const askLifecycle = (action: LifecycleAction) => {
    setConfirm({ kind: "lifecycle", action })
  }

  const confirmAction = () => {
    if (!confirm) return
    if (confirm.kind === "lifecycle") {
      const action = confirm.action
      setConfirm(null)
      actionMutation.mutate(action)
      return
    }
    if (confirm.kind === "duplicate") {
      setConfirm(null)
      navigate(`/docker/containers/edit?id=${encodeURIComponent(id)}`)
      return
    }
    if (confirm.kind === "leave") {
      const networkId = confirm.networkId
      setConfirm(null)
      leaveMutation.mutate(networkId)
    }
  }

  const lifecycleCopy =
    confirm?.kind === "lifecycle" ? LIFECYCLE_COPY[confirm.action] : null

  return (
    <div className="flex w-full min-w-0 flex-col gap-6">
      <Section
        title="Identity & runtime"
        description="Container facts from inspect — no extra API calls."
      >
        <MetaGrid
          items={[
            { label: "Name", value: container.name },
            { label: "Short ID", value: container.short_id || container.id.slice(0, 12) },
            {
              label: "State",
              value: (
                <span className="inline-flex items-center gap-2">
                  <span
                    className={cn(
                      "inline-flex rounded-full px-2 py-0.5 text-[11px] font-medium capitalize",
                      stateBadgeClass(container.state),
                    )}
                  >
                    {container.state}
                  </span>
                  <span className="text-xs font-normal text-muted-foreground">
                    {container.status}
                  </span>
                </span>
              ),
            },
            { label: "Image", value: imageRef },
            { label: "Image ID", value: <span className="font-mono text-xs">{imageId}</span> },
            { label: "Created", value: formatTs(container.created || insp?.Created) },
            { label: "Started", value: formatTs(st?.StartedAt) },
            { label: "Finished", value: formatTs(st?.FinishedAt) },
            { label: "Exit code", value: st?.ExitCode != null ? String(st.ExitCode) : "—" },
            { label: "OOM killed", value: st?.OOMKilled ? "Yes" : "No" },
            { label: "PID", value: st?.Pid ? String(st.Pid) : "—" },
            { label: "Error", value: st?.Error || "—" },
            { label: "Hostname", value: insp?.Config?.Hostname || "—" },
            { label: "User", value: insp?.Config?.User || "—" },
            { label: "Working dir", value: insp?.Config?.WorkingDir || "—" },
            { label: "Network mode", value: hc?.NetworkMode || "—" },
            { label: "Memory limit", value: memLabel },
            { label: "CPU limit", value: cpuLabel },
            { label: "Privileged", value: hc?.Privileged ? "Yes" : "No" },
            { label: "Read-only root", value: hc?.ReadonlyRootfs ? "Yes" : "No" },
            { label: "Auto remove", value: hc?.AutoRemove ? "Yes" : "No" },
            {
              label: "Restart policy",
              value: `${hc?.RestartPolicy?.Name || "no"}${
                hc?.RestartPolicy?.MaximumRetryCount
                  ? ` (max ${hc.RestartPolicy.MaximumRetryCount})`
                  : ""
              }`,
            },
          ]}
        />
      </Section>

      <AlertDialog
        open={confirm != null}
        onOpenChange={(open) => {
          if (!open && !busy) setConfirm(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {confirm?.kind === "lifecycle" && lifecycleCopy
                ? lifecycleCopy.title
                : confirm?.kind === "duplicate"
                  ? "Duplicate / Edit container"
                  : confirm?.kind === "leave"
                    ? "Leave network"
                    : "Confirm"}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {confirm?.kind === "lifecycle" && lifecycleCopy
                ? lifecycleCopy.description(container.name)
                : confirm?.kind === "duplicate" ? (
                <>
                  Open the editor with a copy of{" "}
                  <strong>{container.name}</strong> configuration? You can
                  change the name and settings before creating.
                </>
              ) : confirm?.kind === "leave" ? (
                <>
                  Disconnect <strong>{container.name}</strong> from network{" "}
                  <strong>{confirm.networkName}</strong>?
                </>
              ) : null}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={busy}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              variant={
                confirm?.kind === "lifecycle" && lifecycleCopy?.destructive
                  ? "destructive"
                  : confirm?.kind === "leave"
                    ? "destructive"
                    : "default"
              }
              disabled={busy}
              onClick={() => confirmAction()}
            >
              {confirm?.kind === "lifecycle" && lifecycleCopy
                ? lifecycleCopy.confirm
                : confirm?.kind === "duplicate"
                  ? "Continue"
                  : confirm?.kind === "leave"
                    ? "Leave network"
                    : "Confirm"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <Dialog
        open={recreateOpen}
        onOpenChange={(open) => {
          if (!actionMutation.isPending) {
            setRecreateOpen(open)
            if (!open) setRecreatePull(false)
          }
        }}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Recreate container</DialogTitle>
            <DialogDescription>
              Recreate <strong>{container.name}</strong> with the same
              configuration. The current container will be stopped and replaced.
            </DialogDescription>
          </DialogHeader>
          <div className="flex items-center justify-between gap-3 rounded-lg border px-4 py-3">
            <div className="space-y-0.5">
              <p className="text-sm font-medium">Re-pull image</p>
              <p className="text-xs text-muted-foreground">
                Pull the latest{" "}
                <code className="text-[11px]">{imageRef}</code> before
                recreating.
              </p>
            </div>
            <Switch
              checked={recreatePull}
              onCheckedChange={setRecreatePull}
              disabled={actionMutation.isPending}
            />
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              disabled={actionMutation.isPending}
              onClick={() => setRecreateOpen(false)}
            >
              Cancel
            </Button>
            <Button
              disabled={actionMutation.isPending}
              onClick={() =>
                actionMutation.mutate({
                  kind: "recreate",
                  pull: recreatePull,
                })
              }
            >
              {actionMutation.isPending
                ? recreatePull
                  ? "Pulling & recreating…"
                  : "Recreating…"
                : "Recreate"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>


      <Section title="Published ports">
        {ports.length ? (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[520px] border-collapse text-sm">
              <thead>
                <tr className="border-b bg-muted/30 text-left text-[11px] font-medium tracking-wide text-muted-foreground uppercase">
                  <th className="px-3 py-2">Host</th>
                  <th className="px-3 py-2">Container</th>
                  <th className="px-3 py-2">Protocol</th>
                  <th className="px-3 py-2">IP</th>
                </tr>
              </thead>
              <tbody>
                {ports.map((p, i) => (
                  <tr
                    key={`${p.public_port}-${p.private_port}-${i}`}
                    className="border-b border-border/50 last:border-0"
                  >
                    <td className="px-3 py-2 font-mono text-xs">
                      {p.public_port ?? "—"}
                    </td>
                    <td className="px-3 py-2 font-mono text-xs">{p.private_port}</td>
                    <td className="px-3 py-2 font-mono text-xs uppercase text-muted-foreground">
                      {p.type || "tcp"}
                    </td>
                    <td className="px-3 py-2 font-mono text-xs text-muted-foreground">
                      {p.ip || "—"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">No published ports.</p>
        )}
      </Section>

      <Section title="Configuration">
        <dl className="grid gap-4">
          <MetaRow label="Entrypoint">
            <code className="font-mono text-xs">{entrypoint}</code>
          </MetaRow>
          <MetaRow label="CMD">
            <code className="font-mono text-xs">{cmd}</code>
          </MetaRow>
          <MetaRow label="ENV">
            <KeyValueList data={envRecord} searchable />
          </MetaRow>
          <MetaRow label="Labels">
            <KeyValueList data={labels || {}} searchable />
          </MetaRow>
          <MetaRow label="Restart policies">
            <div className="flex flex-wrap items-center gap-2">
              <Select
                value={restartPolicy}
                onValueChange={setRestartPolicy}
                disabled={busy}
              >
                <SelectTrigger className="w-[180px]">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {RESTART_POLICIES.map((p) => (
                    <SelectItem key={p.value} value={p.value}>
                      {p.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Button
                size="sm"
                variant="outline"
                disabled={
                  busy ||
                  restartPolicy ===
                    (insp?.HostConfig?.RestartPolicy?.Name || "no")
                }
                onClick={() => restartMutation.mutate()}
              >
                Update
              </Button>
            </div>
          </MetaRow>
        </dl>
      </Section>

      <Section title="Volumes">
        {mounts.length ? (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[640px] border-collapse text-sm">
              <thead>
                <tr className="border-b bg-muted/30 text-left text-[11px] font-medium tracking-wide text-muted-foreground uppercase">
                  <th className="px-3 py-2">Type</th>
                  <th className="px-3 py-2">Source</th>
                  <th className="px-3 py-2">Destination</th>
                  <th className="px-3 py-2">Mode</th>
                  <th className="px-3 py-2">Driver</th>
                </tr>
              </thead>
              <tbody>
                {mounts.map((m, i) => (
                  <tr
                    key={`${m.Source}-${m.Destination}-${i}`}
                    className="border-b border-border/50 last:border-0"
                  >
                    <td className="px-3 py-2 text-xs capitalize text-muted-foreground">
                      {m.Type || "—"}
                    </td>
                    <td className="px-3 py-2 font-mono text-xs break-all">
                      {m.Name || m.Source || "—"}
                    </td>
                    <td className="px-3 py-2 font-mono text-xs break-all text-muted-foreground">
                      {m.Destination || "—"}
                    </td>
                    <td className="px-3 py-2 font-mono text-xs text-muted-foreground">
                      {m.Mode || (m.RW === false ? "ro" : "rw")}
                      {m.Propagation ? ` · ${m.Propagation}` : ""}
                    </td>
                    <td className="px-3 py-2 font-mono text-xs text-muted-foreground">
                      {m.Driver || "—"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">No volumes mounted.</p>
        )}
      </Section>

      <Section title="Connected networks">
        <div className="mb-4 flex flex-wrap items-end gap-2">
          <div className="grid min-w-[220px] flex-1 gap-1.5">
            <Label>Select a network</Label>
            <Select
              value={joinNetworkId || undefined}
              onValueChange={setJoinNetworkId}
              disabled={busy || joinable.length === 0}
            >
              <SelectTrigger className="w-full">
                <SelectValue
                  placeholder={
                    joinable.length ? "Choose network" : "No networks available"
                  }
                />
              </SelectTrigger>
              <SelectContent>
                {joinable.map((n) => (
                  <SelectItem key={n.id} value={n.id}>
                    {n.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <Button
            disabled={busy || !joinNetworkId}
            onClick={() => joinMutation.mutate()}
          >
            Join network
          </Button>
        </div>

        {attached.length ? (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[720px] border-collapse text-sm">
              <thead>
                <tr className="border-b bg-muted/30 text-left text-[11px] font-medium tracking-wide text-muted-foreground uppercase">
                  <th className="px-3 py-2">Network</th>
                  <th className="px-3 py-2">IPv4</th>
                  <th className="px-3 py-2">IPv6</th>
                  <th className="px-3 py-2">Gateway</th>
                  <th className="px-3 py-2">MAC</th>
                  <th className="px-3 py-2">Actions</th>
                </tr>
              </thead>
              <tbody>
                {attached.map((n) => (
                  <tr
                    key={n.name}
                    className="border-b border-border/50 last:border-0"
                  >
                    <td className="px-3 py-2">
                      <Link
                        to={`/docker/networks/edit?id=${encodeURIComponent(n.networkId)}`}
                        className="font-medium text-sky-600 hover:underline dark:text-sky-400"
                      >
                        {n.name}
                      </Link>
                    </td>
                    <td className="px-3 py-2 font-mono text-xs">{n.ip}</td>
                    <td className="px-3 py-2 font-mono text-xs text-muted-foreground">
                      {n.ipv6}
                    </td>
                    <td className="px-3 py-2 font-mono text-xs text-muted-foreground">
                      {n.gateway}
                    </td>
                    <td className="px-3 py-2 font-mono text-xs text-muted-foreground">
                      {n.mac}
                    </td>
                    <td className="px-3 py-2">
                      <Button
                        size="sm"
                        variant="destructive"
                        disabled={busy}
                        onClick={() =>
                          setConfirm({
                            kind: "leave",
                            networkId: n.networkId,
                            networkName: n.name,
                          })
                        }
                      >
                        Leave network
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">
            Not connected to any networks.
          </p>
        )}
      </Section>

      <Section
        title="Actions"
        description="Lifecycle controls, recreate, and duplicate."
      >
        <div className="flex flex-wrap items-center gap-3">
          <ButtonGroup className="flex-wrap" aria-label="Lifecycle actions">
            <Button
              size="sm"
              variant="outline"
              disabled={busy || running || paused}
              onClick={() => askLifecycle("start")}
            >
              <Play data-icon="inline-start" />
              Start
            </Button>
            <Button
              size="sm"
              variant="outline"
              disabled={busy || stopped}
              onClick={() => askLifecycle("stop")}
            >
              <Square data-icon="inline-start" />
              Stop
            </Button>
            <Button
              size="sm"
              variant="outline"
              disabled={busy || stopped}
              onClick={() => askLifecycle("kill")}
            >
              <OctagonX data-icon="inline-start" />
              Kill
            </Button>
            <Button
              size="sm"
              variant="outline"
              disabled={busy}
              onClick={() => askLifecycle("restart")}
            >
              <RotateCcw data-icon="inline-start" />
              Restart
            </Button>
            <Button
              size="sm"
              variant="outline"
              disabled={busy || !running}
              onClick={() => askLifecycle("pause")}
            >
              <Pause data-icon="inline-start" />
              Pause
            </Button>
            <Button
              size="sm"
              variant="outline"
              disabled={busy || !paused}
              onClick={() => askLifecycle("resume")}
            >
              <Play data-icon="inline-start" />
              Resume
            </Button>
            <Button
              size="sm"
              variant="destructive"
              disabled={busy}
              onClick={() => askLifecycle("remove")}
            >
              <Trash2 data-icon="inline-start" />
              Remove
            </Button>
          </ButtonGroup>

          <ButtonGroup className="flex-wrap" aria-label="Edit actions">
            <Button
              size="sm"
              variant="outline"
              disabled={busy}
              onClick={() => {
                setRecreatePull(false)
                setRecreateOpen(true)
              }}
            >
              <RotateCcw data-icon="inline-start" />
              Recreate
            </Button>
            <Button
              size="sm"
              variant="outline"
              disabled={busy}
              onClick={() => setConfirm({ kind: "duplicate" })}
            >
              <Copy data-icon="inline-start" />
              Duplicate/Edit
            </Button>
          </ButtonGroup>
        </div>
      </Section>

      <Section
        title="Create image"
        description="Commit the container filesystem into a new local image."
      >
        <div className="grid gap-4 sm:grid-cols-[1fr_1fr_auto] sm:items-end">
          <div className="grid gap-1.5">
            <Label>Image</Label>
            <Input
              value={commitRepo}
              onChange={(e) => setCommitRepo(e.target.value)}
              placeholder="repository name"
              disabled={busy}
            />
          </div>
          <div className="grid gap-1.5">
            <Label>Tag</Label>
            <Input
              value={commitTag}
              onChange={(e) => setCommitTag(e.target.value)}
              placeholder="latest"
              disabled={busy}
            />
          </div>
          <Button
            disabled={busy || !commitRepo.trim()}
            onClick={() => commitMutation.mutate()}
          >
            <Check data-icon="inline-start" />
            {commitMutation.isPending ? "Creating…" : "Create"}
          </Button>
        </div>
        <p className="mt-3 text-xs text-muted-foreground">
          Creates a local image only. Push to a registry separately if needed.
        </p>
      </Section>

    </div>
  )
}
