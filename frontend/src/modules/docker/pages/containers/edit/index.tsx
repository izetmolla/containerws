import { useMemo, useState, type ReactNode } from "react"
import { Link, useNavigate, useSearchParams } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Plus, RefreshCw, Settings2, Trash2 } from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { ReactSelect } from "@/components/ui/reactselect"
import { Switch } from "@/components/ui/switch"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { toastRequestError } from "@/lib/network"
import { cn } from "@/lib/utils"

import { EngineBanner } from "../../_shared/engine-status"
import { EnvironmentSelector } from "../../_shared/environment-selector"
import { listNetworks } from "../../networks/api"
import {
  createContainer,
  DOCKER_CONTAINERS_KEY,
  getContainer,
  updateContainer,
  type CreateContainerInput,
  type DockerInspect,
} from "../list/api"

type Option = { value: string; label: string }

type PortRow = {
  host_ip: string
  host_port: string
  container_port: string
  protocol: string
}

type VolumeRow = {
  host: string
  container: string
  read_only: boolean
}

type KvRow = { key: string; value: string }

type ConsoleMode = "both" | "interactive" | "tty" | "none"

const REGISTRY_OPTIONS: Option[] = [
  { value: "dockerhub", label: "Docker Hub (anonymous)" },
  { value: "other", label: "Other / full image name" },
]

const RESTART_OPTIONS: Option[] = [
  { value: "no", label: "No" },
  { value: "always", label: "Always" },
  { value: "unless-stopped", label: "Unless stopped" },
  { value: "on-failure", label: "On failure" },
]

const LOG_DRIVER_OPTIONS: Option[] = [
  { value: "", label: "Default logging driver" },
  { value: "json-file", label: "json-file" },
  { value: "local", label: "local" },
  { value: "syslog", label: "syslog" },
  { value: "journald", label: "journald" },
  { value: "none", label: "none" },
]

const PROTOCOL_OPTIONS: Option[] = [
  { value: "tcp", label: "TCP" },
  { value: "udp", label: "UDP" },
]

const ADVANCED_TABS = [
  { id: "commands", label: "Commands & logging" },
  { id: "volumes", label: "Volumes" },
  { id: "network", label: "Network" },
  { id: "env", label: "Env" },
  { id: "labels", label: "Labels" },
  { id: "restart", label: "Restart policy" },
  { id: "runtime", label: "Runtime & resources" },
  { id: "capabilities", label: "Capabilities" },
] as const

function emptyPort(): PortRow {
  return { host_ip: "", host_port: "", container_port: "", protocol: "tcp" }
}

function emptyVolume(): VolumeRow {
  return { host: "", container: "", read_only: false }
}

function emptyKv(): KvRow {
  return { key: "", value: "" }
}

function shellJoin(parts?: string[] | null) {
  if (!parts?.length) return ""
  return parts.join(" ")
}

function shellSplit(text: string) {
  const t = text.trim()
  if (!t) return undefined
  return t.split(/\s+/).filter(Boolean)
}

function portsFromInspect(insp?: DockerInspect): PortRow[] {
  const bindings = insp?.HostConfig?.PortBindings
  if (!bindings || !Object.keys(bindings).length) return []
  const rows: PortRow[] = []
  for (const [key, binds] of Object.entries(bindings)) {
    const [portProto, protoRaw] = key.split("/")
    const protocol = (protoRaw || "tcp").toLowerCase()
    if (!binds?.length) {
      rows.push({
        host_ip: "",
        host_port: "",
        container_port: portProto,
        protocol,
      })
      continue
    }
    for (const b of binds) {
      rows.push({
        host_ip: b.HostIp || "",
        host_port: b.HostPort || "",
        container_port: portProto,
        protocol,
      })
    }
  }
  return rows
}

function volumesFromInspect(insp?: DockerInspect): VolumeRow[] {
  const binds = insp?.HostConfig?.Binds
  if (binds?.length) {
    return binds.map((b) => {
      const parts = b.split(":")
      if (parts.length >= 3) {
        return {
          host: parts[0],
          container: parts[1],
          read_only: parts[2] === "ro",
        }
      }
      return {
        host: parts[0] || "",
        container: parts[1] || "",
        read_only: false,
      }
    })
  }
  return (insp?.Mounts || [])
    .filter((m) => m.Type === "bind" || m.Source)
    .map((m) => ({
      host: m.Source || "",
      container: m.Destination || "",
      read_only: m.RW === false,
    }))
}

function envFromInspect(insp?: DockerInspect): KvRow[] {
  return (insp?.Config?.Env || []).map((line) => {
    const i = line.indexOf("=")
    if (i < 0) return { key: line, value: "" }
    return { key: line.slice(0, i), value: line.slice(i + 1) }
  })
}

function labelsFromInspect(insp?: DockerInspect): KvRow[] {
  const labels = insp?.Config?.Labels || {}
  return Object.entries(labels).map(([key, value]) => ({ key, value }))
}

function memoryFromBytes(n?: number) {
  if (!n) return ""
  if (n % (1024 * 1024 * 1024) === 0) return `${n / (1024 * 1024 * 1024)}g`
  if (n % (1024 * 1024) === 0) return `${n / (1024 * 1024)}m`
  return String(n)
}

function consoleFromInspect(insp?: DockerInspect): ConsoleMode {
  const tty = Boolean(insp?.Config?.Tty)
  const stdin = Boolean(insp?.Config?.OpenStdin)
  if (tty && stdin) return "both"
  if (stdin) return "interactive"
  if (tty) return "tty"
  return "none"
}

export default function ContainerEditPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [params] = useSearchParams()
  const editId = params.get("id")?.trim() || ""
  const isEdit = Boolean(editId)

  const [name, setName] = useState("")
  const [registry, setRegistry] = useState("dockerhub")
  const [image, setImage] = useState("")
  const [alwaysPull, setAlwaysPull] = useState(true)
  const [publishAll, setPublishAll] = useState(false)
  const [ports, setPorts] = useState<PortRow[]>([])
  const [autoRemove, setAutoRemove] = useState(false)
  const [startAfter, setStartAfter] = useState(true)
  const [imageError, setImageError] = useState(false)

  const [cmdMode, setCmdMode] = useState<"default" | "override">("default")
  const [cmd, setCmd] = useState("")
  const [entrypointMode, setEntrypointMode] = useState<"default" | "override">(
    "default"
  )
  const [entrypoint, setEntrypoint] = useState("")
  const [workdir, setWorkdir] = useState("")
  const [user, setUser] = useState("")
  const [consoleMode, setConsoleMode] = useState<ConsoleMode>("none")
  const [logDriver, setLogDriver] = useState("")
  const [logOpts, setLogOpts] = useState<KvRow[]>([])

  const [volumes, setVolumes] = useState<VolumeRow[]>([])
  const [networks, setNetworks] = useState<string[]>([])
  const [hostname, setHostname] = useState("")
  const [extraHosts, setExtraHosts] = useState("")
  const [dns, setDns] = useState("")

  const [envRows, setEnvRows] = useState<KvRow[]>([])
  const [labelRows, setLabelRows] = useState<KvRow[]>([])
  const [restart, setRestart] = useState("no")
  const [restartRetries, setRestartRetries] = useState("0")

  const [privileged, setPrivileged] = useState(false)
  const [readonlyRoot, setReadonlyRoot] = useState(false)
  const [memory, setMemory] = useState("")
  const [cpus, setCpus] = useState("")
  const [devices, setDevices] = useState("")
  const [capAdd, setCapAdd] = useState("")
  const [capDrop, setCapDrop] = useState("")

  const detailQuery = useQuery({
    queryKey: [DOCKER_CONTAINERS_KEY, "edit", editId],
    queryFn: () => getContainer(editId),
    enabled: isEdit,
  })

  const networksQuery = useQuery({
    queryKey: ["docker-networks-opts"],
    queryFn: listNetworks,
    staleTime: 30_000,
  })

  const detailData = isEdit ? detailQuery.data?.data : undefined
  const [prevDetailData, setPrevDetailData] = useState(detailData)
  if (detailData !== prevDetailData) {
    setPrevDetailData(detailData)
    if (detailData) {
      const insp = detailData.inspect
      setName(detailData.name || "")
      setImage(detailData.image || insp?.Config?.Image || "")
      setRegistry(
        (detailData.image || "").includes(".") &&
          (detailData.image || "").includes("/")
          ? "other"
          : "dockerhub"
      )
      setAlwaysPull(false)
      setPublishAll(Boolean(insp?.HostConfig?.PublishAllPorts))
      setPorts(portsFromInspect(insp))
      setAutoRemove(Boolean(insp?.HostConfig?.AutoRemove))
      setStartAfter(detailData.state === "running")

      const cmdVal = shellJoin(insp?.Config?.Cmd)
      setCmd(cmdVal)
      setCmdMode(cmdVal ? "override" : "default")
      const epVal = shellJoin(insp?.Config?.Entrypoint)
      setEntrypoint(epVal)
      setEntrypointMode(epVal ? "override" : "default")
      setWorkdir(insp?.Config?.WorkingDir || "")
      setUser(insp?.Config?.User || "")
      setHostname(insp?.Config?.Hostname || "")
      setConsoleMode(consoleFromInspect(insp))
      setLogDriver(insp?.HostConfig?.LogConfig?.Type || "")
      setLogOpts(
        Object.entries(insp?.HostConfig?.LogConfig?.Config || {}).map(
          ([key, value]) => ({ key, value })
        )
      )
      setVolumes(volumesFromInspect(insp))
      const nets = Object.keys(insp?.NetworkSettings?.Networks || {})
      setNetworks(nets)
      setExtraHosts((insp?.HostConfig?.ExtraHosts || []).join("\n"))
      setDns((insp?.HostConfig?.Dns || []).join("\n"))
      setEnvRows(envFromInspect(insp))
      setLabelRows(labelsFromInspect(insp))
      setRestart(insp?.HostConfig?.RestartPolicy?.Name || "no")
      setRestartRetries(
        String(insp?.HostConfig?.RestartPolicy?.MaximumRetryCount || 0)
      )
      setPrivileged(Boolean(insp?.HostConfig?.Privileged))
      setReadonlyRoot(Boolean(insp?.HostConfig?.ReadonlyRootfs))
      setMemory(memoryFromBytes(insp?.HostConfig?.Memory))
      setCpus(
        insp?.HostConfig?.NanoCpus
          ? String(insp.HostConfig.NanoCpus / 1e9)
          : ""
      )
      setDevices(
        (insp?.HostConfig?.Devices || [])
          .map((d) =>
            [d.PathOnHost, d.PathInContainer, d.CgroupPermissions]
              .filter(Boolean)
              .join(":")
          )
          .join("\n")
      )
      setCapAdd((insp?.HostConfig?.CapAdd || []).join("\n"))
      setCapDrop((insp?.HostConfig?.CapDrop || []).join("\n"))
    }
  }

  const networkOptions = useMemo(
    () =>
      (networksQuery.data?.data ?? []).map((n) => ({
        value: n.name,
        label: n.name,
      })),
    [networksQuery.data?.data]
  )
  const saveMutation = useMutation({
    mutationFn: async (body: CreateContainerInput) => {
      if (isEdit) return updateContainer(editId, body)
      return createContainer(body)
    },
    onSuccess: (res) => {
      toast.success(res.message || (isEdit ? "Container updated" : "Container created"))
      void queryClient.invalidateQueries({ queryKey: [DOCKER_CONTAINERS_KEY] })
      const id = res.data?.id
      if (id) navigate(`/docker/containers/${id}`)
      else navigate("/docker/containers")
    },
    onError: (err) =>
      toastRequestError(err, isEdit ? "Update failed" : "Create failed"),
  })

  const buildBody = (): CreateContainerInput | null => {
    const img = image.trim()
    if (!img) {
      setImageError(true)
      toast.message("Image is required")
      return null
    }
    setImageError(false)

    const portStrings = ports
      .filter((p) => p.container_port.trim())
      .map((p) => {
        const proto = p.protocol || "tcp"
        const cport = p.container_port.trim()
        const hport = p.host_port.trim()
        const hip = p.host_ip.trim()
        let base: string
        if (hip && hport) base = `${hip}:${hport}:${cport}`
        else if (hport) base = `${hport}:${cport}`
        else base = cport
        return `${base}/${proto}`
      })

    const binds = volumes
      .filter((v) => v.host.trim() && v.container.trim())
      .map((v) =>
        `${v.host.trim()}:${v.container.trim()}${v.read_only ? ":ro" : ""}`
      )

    const env = envRows
      .filter((r) => r.key.trim())
      .map((r) => `${r.key.trim()}=${r.value}`)

    const labels: Record<string, string> = {}
    for (const r of labelRows) {
      if (!r.key.trim()) continue
      labels[r.key.trim()] = r.value
    }

    const log_opts: Record<string, string> = {}
    for (const r of logOpts) {
      if (!r.key.trim()) continue
      log_opts[r.key.trim()] = r.value
    }

    const tty = consoleMode === "both" || consoleMode === "tty"
    const open_stdin = consoleMode === "both" || consoleMode === "interactive"

    return {
      image: img,
      name: name.trim() || undefined,
      ports: portStrings,
      publish_all: publishAll,
      always_pull: alwaysPull,
      auto_remove: autoRemove,
      start: startAfter,
      binds,
      networks: networks.length ? networks : undefined,
      env,
      labels,
      cmd: cmdMode === "override" ? shellSplit(cmd) : undefined,
      entrypoint:
        entrypointMode === "override" ? shellSplit(entrypoint) : undefined,
      working_dir: workdir.trim() || undefined,
      user: user.trim() || undefined,
      hostname: hostname.trim() || undefined,
      extra_hosts: extraHosts
        .split("\n")
        .map((s) => s.trim())
        .filter(Boolean),
      dns: dns
        .split(/[\n,]/)
        .map((s) => s.trim())
        .filter(Boolean),
      restart_policy: restart,
      restart_retries: Number(restartRetries) || 0,
      privileged,
      readonly_rootfs: readonlyRoot,
      memory: memory.trim() || undefined,
      cpus: cpus.trim() ? Number(cpus) : undefined,
      devices: devices
        .split("\n")
        .map((s) => s.trim())
        .filter(Boolean),
      cap_add: capAdd
        .split(/[\n,]/)
        .map((s) => s.trim())
        .filter(Boolean),
      cap_drop: capDrop
        .split(/[\n,]/)
        .map((s) => s.trim())
        .filter(Boolean),
      tty,
      open_stdin,
      log_driver: logDriver || undefined,
      log_opts: Object.keys(log_opts).length ? log_opts : undefined,
    }
  }

  const onDeploy = () => {
    const body = buildBody()
    if (!body) return
    saveMutation.mutate(body)
  }

  return (
    <ContentLoader
      title={isEdit ? "Edit container" : "Create container"}
      breadcrumb={[
        { label: "Docker", to: "/docker" },
        { label: "Containers", to: "/docker/containers" },
        { label: isEdit ? "Edit" : "Add container" },
      ]}
      isLoading={isEdit && detailQuery.isLoading}
      error={isEdit ? detailQuery.error : undefined}
      rightComponent={
        <div className="flex flex-wrap items-center gap-2">
          <EnvironmentSelector />
          <Button
            size="sm"
            variant="outline"
            onClick={() =>
              isEdit ? detailQuery.refetch() : window.location.reload()
            }
          >
            <RefreshCw className="size-3.5" />
          </Button>
        </div>
      }
    >
      <div className="flex w-full min-w-0 flex-col gap-6">
        <EngineBanner />

        <Card className="gap-0 py-0">
          <CardHeader className="border-b py-5">
            <CardTitle>{isEdit ? "Container configuration" : "Create container"}</CardTitle>
            <CardDescription>
              {isEdit
                ? "Updating replaces the existing container (stop → remove → create) with the same name by default."
                : "Configure the image, networking, and runtime options, then deploy."}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6 py-6">
            <Field label="Name">
              <Input
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. myContainer"
              />
            </Field>

            <div className="space-y-3 rounded-lg border p-4">
              <p className="text-sm font-medium">Image configuration</p>
              <div className="grid gap-4 sm:grid-cols-[14rem_1fr]">
                <Field label="Registry">
                  <ReactSelect<Option, false>
                    size="sm"
                    options={REGISTRY_OPTIONS}
                    value={registry}
                    onValueChange={(v) => v && setRegistry(v)}
                  />
                </Field>
                <Field
                  label="Image"
                  hint={
                    registry === "dockerhub"
                      ? "Docker Hub anonymous pulls are limited (100 / 6h)."
                      : "Use a full reference including registry host."
                  }
                >
                  <div className="flex gap-2">
                    <div
                      className={cn(
                        "flex min-w-0 flex-1 overflow-hidden rounded-md border bg-background",
                        imageError && "border-destructive"
                      )}
                    >
                      {registry === "dockerhub" ? (
                        <span className="flex shrink-0 items-center border-r bg-muted/50 px-2.5 font-mono text-xs text-muted-foreground">
                          docker.io
                        </span>
                      ) : null}
                      <Input
                        value={image}
                        onChange={(e) => {
                          setImage(e.target.value)
                          if (e.target.value.trim()) setImageError(false)
                        }}
                        placeholder="e.g. nginx:alpine"
                        className="border-0 shadow-none focus-visible:ring-0"
                      />
                    </div>
                    <Button variant="outline" asChild>
                      <Link to="/docker/images">Search</Link>
                    </Button>
                  </div>
                  {imageError ? (
                    <p className="text-xs text-destructive">Image is required</p>
                  ) : null}
                </Field>
              </div>

              <div className="flex items-center justify-between gap-4 rounded-md border px-3 py-2.5">
                <div>
                  <Label htmlFor="always-pull" className="cursor-pointer">
                    Always pull the image
                  </Label>
                  <p className="text-xs text-muted-foreground">
                    Re-pull from the registry before creating the container.
                  </p>
                </div>
                <Switch
                  id="always-pull"
                  checked={alwaysPull}
                  onCheckedChange={setAlwaysPull}
                />
              </div>
            </div>

            <div className="space-y-3">
              <p className="text-sm font-medium">Network ports configuration</p>
              <div className="flex items-center justify-between gap-4 rounded-md border px-3 py-2.5">
                <Label htmlFor="publish-all" className="cursor-pointer">
                  Publish all exposed ports to random host ports
                </Label>
                <Switch
                  id="publish-all"
                  checked={publishAll}
                  onCheckedChange={setPublishAll}
                />
              </div>

              <div className="space-y-2">
                <p className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
                  Port mapping
                </p>
                {ports.map((row, idx) => (
                  <div
                    key={idx}
                    className="grid grid-cols-[1fr_1fr_1fr_6.5rem_auto] gap-2"
                  >
                    <Input
                      placeholder="host IP"
                      value={row.host_ip}
                      onChange={(e) =>
                        setPorts((prev) =>
                          prev.map((p, i) =>
                            i === idx ? { ...p, host_ip: e.target.value } : p
                          )
                        )
                      }
                    />
                    <Input
                      placeholder="host port"
                      value={row.host_port}
                      onChange={(e) =>
                        setPorts((prev) =>
                          prev.map((p, i) =>
                            i === idx ? { ...p, host_port: e.target.value } : p
                          )
                        )
                      }
                    />
                    <Input
                      placeholder="container port"
                      value={row.container_port}
                      onChange={(e) =>
                        setPorts((prev) =>
                          prev.map((p, i) =>
                            i === idx
                              ? { ...p, container_port: e.target.value }
                              : p
                          )
                        )
                      }
                    />
                    <ReactSelect<Option, false>
                      size="sm"
                      options={PROTOCOL_OPTIONS}
                      value={row.protocol}
                      onValueChange={(v) =>
                        v &&
                        setPorts((prev) =>
                          prev.map((p, i) =>
                            i === idx ? { ...p, protocol: v } : p
                          )
                        )
                      }
                    />
                    <Button
                      size="icon-sm"
                      variant="ghost"
                      onClick={() =>
                        setPorts((prev) => prev.filter((_, i) => i !== idx))
                      }
                    >
                      <Trash2 className="size-3.5" />
                    </Button>
                  </div>
                ))}
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => setPorts((prev) => [...prev, emptyPort()])}
                >
                  <Plus data-icon="inline-start" />
                  Map additional port
                </Button>
              </div>
            </div>

            <div className="flex items-center justify-between gap-4 rounded-md border px-3 py-2.5">
              <div>
                <Label htmlFor="auto-remove" className="cursor-pointer">
                  Auto remove
                </Label>
                <p className="text-xs text-muted-foreground">
                  Automatically remove the container when it exits.
                </p>
              </div>
              <Switch
                id="auto-remove"
                checked={autoRemove}
                onCheckedChange={setAutoRemove}
              />
            </div>

            <div className="flex items-center justify-between gap-4 rounded-md border px-3 py-2.5">
              <Label htmlFor="start-after" className="cursor-pointer">
                Start container after deploy
              </Label>
              <Switch
                id="start-after"
                checked={startAfter}
                onCheckedChange={setStartAfter}
              />
            </div>
          </CardContent>
          <CardFooter className="border-t py-4">
            <Button
              size="lg"
              className="w-full sm:w-auto"
              disabled={saveMutation.isPending || !image.trim()}
              onClick={onDeploy}
            >
              {saveMutation.isPending
                ? isEdit
                  ? "Updating…"
                  : "Deploying…"
                : isEdit
                  ? "Update the container"
                  : "Deploy the container"}
            </Button>
          </CardFooter>
        </Card>

        <Card className="gap-0 py-0">
          <CardHeader className="border-b py-5">
            <CardTitle className="flex items-center gap-2">
              <Settings2 className="size-4" />
              Advanced container settings
            </CardTitle>
          </CardHeader>
          <CardContent className="py-0">
            <Tabs defaultValue="commands" className="gap-0">
              <div className="overflow-x-auto border-b">
                <TabsList
                  variant="line"
                  className="h-auto w-max min-w-full justify-start rounded-none bg-transparent p-0"
                >
                  {ADVANCED_TABS.map((t) => (
                    <TabsTrigger
                      key={t.id}
                      value={t.id}
                      className="rounded-none px-3 py-3"
                    >
                      {t.label}
                    </TabsTrigger>
                  ))}
                </TabsList>
              </div>

              <TabsContent value="commands" className="space-y-5 p-6">
                <ModeField
                  label="Command"
                  mode={cmdMode}
                  onMode={setCmdMode}
                  value={cmd}
                  onValue={setCmd}
                  placeholder="e.g. nginx -g 'daemon off;'"
                />
                <ModeField
                  label="Entrypoint"
                  mode={entrypointMode}
                  onMode={setEntrypointMode}
                  value={entrypoint}
                  onValue={setEntrypoint}
                  placeholder="e.g. /docker-entrypoint.sh"
                />
                <div className="grid gap-4 sm:grid-cols-2">
                  <Field label="Working Dir">
                    <Input
                      value={workdir}
                      onChange={(e) => setWorkdir(e.target.value)}
                      placeholder="e.g. /myapp"
                    />
                  </Field>
                  <Field label="User">
                    <Input
                      value={user}
                      onChange={(e) => setUser(e.target.value)}
                      placeholder="e.g. nginx"
                    />
                  </Field>
                </div>
                <Field label="Console">
                  <div className="flex flex-wrap gap-3">
                    {(
                      [
                        ["both", "Interactive & TTY (-i -t)"],
                        ["interactive", "Interactive (-i)"],
                        ["tty", "TTY (-t)"],
                        ["none", "None"],
                      ] as const
                    ).map(([value, label]) => (
                      <label
                        key={value}
                        className="flex cursor-pointer items-center gap-2 text-sm"
                      >
                        <input
                          type="radio"
                          name="console"
                          className="size-4 accent-foreground"
                          checked={consoleMode === value}
                          onChange={() => setConsoleMode(value)}
                        />
                        {label}
                      </label>
                    ))}
                  </div>
                </Field>
                <div className="space-y-3 rounded-lg border p-4">
                  <p className="text-sm font-medium">Logging</p>
                  <Field label="Driver">
                    <ReactSelect<Option, false>
                      size="sm"
                      options={LOG_DRIVER_OPTIONS}
                      value={logDriver}
                      onValueChange={(v) => setLogDriver(v || "")}
                    />
                  </Field>
                  <KvEditor
                    label="Options"
                    rows={logOpts}
                    onChange={setLogOpts}
                    addLabel="Add item"
                  />
                </div>
              </TabsContent>

              <TabsContent value="volumes" className="space-y-4 p-6">
                {volumes.map((row, idx) => (
                  <div
                    key={idx}
                    className="grid grid-cols-[1fr_1fr_auto_auto] items-center gap-2"
                  >
                    <Input
                      placeholder="host path or volume"
                      value={row.host}
                      onChange={(e) =>
                        setVolumes((prev) =>
                          prev.map((v, i) =>
                            i === idx ? { ...v, host: e.target.value } : v
                          )
                        )
                      }
                    />
                    <Input
                      placeholder="container path"
                      value={row.container}
                      onChange={(e) =>
                        setVolumes((prev) =>
                          prev.map((v, i) =>
                            i === idx ? { ...v, container: e.target.value } : v
                          )
                        )
                      }
                    />
                    <label className="flex items-center gap-2 text-xs whitespace-nowrap">
                      <input
                        type="checkbox"
                        className="size-4 accent-foreground"
                        checked={row.read_only}
                        onChange={(e) =>
                          setVolumes((prev) =>
                            prev.map((v, i) =>
                              i === idx
                                ? { ...v, read_only: e.target.checked }
                                : v
                            )
                          )
                        }
                      />
                      Read-only
                    </label>
                    <Button
                      size="icon-sm"
                      variant="ghost"
                      onClick={() =>
                        setVolumes((prev) => prev.filter((_, i) => i !== idx))
                      }
                    >
                      <Trash2 className="size-3.5" />
                    </Button>
                  </div>
                ))}
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => setVolumes((prev) => [...prev, emptyVolume()])}
                >
                  <Plus data-icon="inline-start" />
                  Map additional volume
                </Button>
              </TabsContent>

              <TabsContent value="network" className="space-y-4 p-6">
                <Field label="Networks">
                  <ReactSelect<Option, true>
                    size="sm"
                    isMulti
                    options={networkOptions}
                    value={networks}
                    onValueChange={(v) => setNetworks([...(v || [])])}
                    placeholder="Select networks…"
                  />
                </Field>
                <Field label="Hostname">
                  <Input
                    value={hostname}
                    onChange={(e) => setHostname(e.target.value)}
                    placeholder="container hostname"
                  />
                </Field>
                <Field label="Extra hosts" hint="host:ip — one per line">
                  <textarea
                    className="min-h-24 w-full rounded-md border bg-background px-3 py-2 font-mono text-sm"
                    value={extraHosts}
                    onChange={(e) => setExtraHosts(e.target.value)}
                    placeholder="myhost:10.0.0.1"
                  />
                </Field>
                <Field label="DNS" hint="Comma or newline separated">
                  <Input
                    value={dns}
                    onChange={(e) => setDns(e.target.value)}
                    placeholder="1.1.1.1, 8.8.8.8"
                  />
                </Field>
              </TabsContent>

              <TabsContent value="env" className="space-y-4 p-6">
                <KvEditor
                  label="Environment variables"
                  rows={envRows}
                  onChange={setEnvRows}
                  addLabel="Add environment variable"
                  keyPlaceholder="NAME"
                  valuePlaceholder="value"
                />
              </TabsContent>

              <TabsContent value="labels" className="space-y-4 p-6">
                <KvEditor
                  label="Labels"
                  rows={labelRows}
                  onChange={setLabelRows}
                  addLabel="Add label"
                  keyPlaceholder="com.example.key"
                  valuePlaceholder="value"
                />
              </TabsContent>

              <TabsContent value="restart" className="space-y-4 p-6">
                <Field label="Restart policy">
                  <ReactSelect<Option, false>
                    size="sm"
                    options={RESTART_OPTIONS}
                    value={restart}
                    onValueChange={(v) => v && setRestart(v)}
                  />
                </Field>
                {restart === "on-failure" ? (
                  <Field label="Maximum retry count">
                    <Input
                      type="number"
                      value={restartRetries}
                      onChange={(e) => setRestartRetries(e.target.value)}
                    />
                  </Field>
                ) : null}
              </TabsContent>

              <TabsContent value="runtime" className="space-y-4 p-6">
                <div className="flex items-center justify-between gap-4 rounded-md border px-3 py-2.5">
                  <Label htmlFor="privileged">Privileged mode</Label>
                  <Switch
                    id="privileged"
                    checked={privileged}
                    onCheckedChange={setPrivileged}
                  />
                </div>
                <div className="flex items-center justify-between gap-4 rounded-md border px-3 py-2.5">
                  <Label htmlFor="readonly-root">Read-only root filesystem</Label>
                  <Switch
                    id="readonly-root"
                    checked={readonlyRoot}
                    onCheckedChange={setReadonlyRoot}
                  />
                </div>
                <div className="grid gap-4 sm:grid-cols-2">
                  <Field label="Memory limit" hint="e.g. 512m, 1g">
                    <Input
                      value={memory}
                      onChange={(e) => setMemory(e.target.value)}
                      placeholder="512m"
                    />
                  </Field>
                  <Field label="CPU limit" hint="Number of CPUs, e.g. 1.5">
                    <Input
                      value={cpus}
                      onChange={(e) => setCpus(e.target.value)}
                      placeholder="1"
                    />
                  </Field>
                </div>
                <Field
                  label="Devices"
                  hint="host:container[:perms] — one per line"
                >
                  <textarea
                    className="min-h-24 w-full rounded-md border bg-background px-3 py-2 font-mono text-sm"
                    value={devices}
                    onChange={(e) => setDevices(e.target.value)}
                    placeholder="/dev/ttyUSB0:/dev/ttyUSB0"
                  />
                </Field>
              </TabsContent>

              <TabsContent value="capabilities" className="space-y-4 p-6">
                <Field label="Cap add" hint="One capability per line">
                  <textarea
                    className="min-h-24 w-full rounded-md border bg-background px-3 py-2 font-mono text-sm"
                    value={capAdd}
                    onChange={(e) => setCapAdd(e.target.value)}
                    placeholder="NET_ADMIN"
                  />
                </Field>
                <Field label="Cap drop" hint="One capability per line">
                  <textarea
                    className="min-h-24 w-full rounded-md border bg-background px-3 py-2 font-mono text-sm"
                    value={capDrop}
                    onChange={(e) => setCapDrop(e.target.value)}
                    placeholder="ALL"
                  />
                </Field>
              </TabsContent>
            </Tabs>
          </CardContent>
        </Card>
      </div>
    </ContentLoader>
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

function ModeField({
  label,
  mode,
  onMode,
  value,
  onValue,
  placeholder,
}: {
  label: string
  mode: "default" | "override"
  onMode: (m: "default" | "override") => void
  value: string
  onValue: (v: string) => void
  placeholder?: string
}) {
  return (
    <Field label={label}>
      <div className="mb-2 inline-flex rounded-md border p-0.5">
        {(["default", "override"] as const).map((m) => (
          <button
            key={m}
            type="button"
            className={cn(
              "rounded-sm px-3 py-1 text-xs font-medium capitalize",
              mode === m
                ? "bg-muted text-foreground"
                : "text-muted-foreground hover:text-foreground"
            )}
            onClick={() => onMode(m)}
          >
            {m}
          </button>
        ))}
      </div>
      {mode === "override" ? (
        <Input
          value={value}
          onChange={(e) => onValue(e.target.value)}
          placeholder={placeholder}
          className="font-mono text-sm"
        />
      ) : null}
    </Field>
  )
}

function KvEditor({
  label,
  rows,
  onChange,
  addLabel,
  keyPlaceholder = "name",
  valuePlaceholder = "value",
}: {
  label: string
  rows: KvRow[]
  onChange: (rows: KvRow[]) => void
  addLabel: string
  keyPlaceholder?: string
  valuePlaceholder?: string
}) {
  return (
    <div className="space-y-2">
      <p className="text-sm font-medium">{label}</p>
      {rows.map((row, idx) => (
        <div key={idx} className="grid grid-cols-[1fr_1fr_auto] gap-2">
          <Input
            placeholder={keyPlaceholder}
            value={row.key}
            onChange={(e) =>
              onChange(
                rows.map((r, i) =>
                  i === idx ? { ...r, key: e.target.value } : r
                )
              )
            }
          />
          <Input
            placeholder={valuePlaceholder}
            value={row.value}
            onChange={(e) =>
              onChange(
                rows.map((r, i) =>
                  i === idx ? { ...r, value: e.target.value } : r
                )
              )
            }
          />
          <Button
            size="icon-sm"
            variant="ghost"
            onClick={() => onChange(rows.filter((_, i) => i !== idx))}
          >
            <Trash2 className="size-3.5" />
          </Button>
        </div>
      ))}
      <Button
        size="sm"
        variant="outline"
        onClick={() => onChange([...rows, emptyKv()])}
      >
        <Plus data-icon="inline-start" />
        {addLabel}
      </Button>
    </div>
  )
}
